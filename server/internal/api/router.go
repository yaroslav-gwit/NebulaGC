package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"nebulagc.io/server/internal/api/handlers"
	"nebulagc.io/server/internal/api/middleware"
	"nebulagc.io/server/internal/ha"
	"nebulagc.io/server/internal/metrics"
	"nebulagc.io/server/internal/service"
)

// RouterConfig holds configuration for setting up the HTTP router.
type RouterConfig struct {
	// DB is the database connection.
	DB *sql.DB

	// Logger is the Zap logger for request logging.
	Logger *zap.Logger

	// HMACSecret is the secret key for token validation.
	HMACSecret string

	// InstanceID is this control plane instance's UUID.
	InstanceID string

	// AllowOrigins is the list of allowed CORS origins.
	// Use []string{"*"} to allow all origins (not recommended for production).
	AllowOrigins []string

	// DisableWriteGuard disables the replica write guard (for single-instance deployments).
	DisableWriteGuard bool

	// HAManager provides master detection for write-guard and health endpoints.
	HAManager *ha.Manager
}

// SetupRouter creates and configures the Gin HTTP router with all routes and middleware.
//
// This function sets up:
// - Global middleware (logging, CORS, rate limiting)
// - Health check endpoints (no auth required)
// - Node management endpoints (node token auth)
// - Config distribution endpoints (node token auth)
// - Topology management endpoints (cluster token auth)
// - Route management endpoints (node token auth)
// - Token rotation endpoints (various auth)
//
// Parameters:
//   - config: Router configuration
//
// Returns:
//   - Configured Gin engine ready to serve requests
func SetupRouter(config *RouterConfig) *gin.Engine {
	// Create router
	router := gin.New()

	// Recovery middleware (recover from panics)
	router.Use(gin.Recovery())

	// Metrics middleware (should be early to capture all requests)
	router.Use(middleware.MetricsMiddleware())

	// Request logging middleware
	router.Use(middleware.RequestLogger(config.Logger))

	// CORS middleware
	if len(config.AllowOrigins) > 0 {
		router.Use(middleware.CORS(config.AllowOrigins))
	}

	// Global rate limiting by IP (applies to all endpoints)
	router.Use(middleware.RateLimitByIP(100.0, 200)) // 100 req/s per IP

	// Replica write guard (if enabled)
	if !config.DisableWriteGuard && config.HAManager != nil {
		router.Use(middleware.WriteGuard(config.HAManager.IsMaster))
	}

	// Authentication config for middleware
	authConfig := &middleware.AuthConfig{
		DB:     config.DB,
		Secret: config.HMACSecret,
	}

	// Services
	nodeService := service.NewNodeService(config.DB, config.Logger, config.HMACSecret)
	nodeHandler := handlers.NewNodeHandler(nodeService)

	bundleService := service.NewBundleService(config.DB, config.Logger)
	bundleHandler := handlers.NewBundleHandler(bundleService)

	topologyService := service.NewTopologyService(config.DB, config.Logger, config.HMACSecret)
	topologyHandler := handlers.NewTopologyHandler(topologyService)

	// Health check handler
	healthHandler := handlers.NewHealthHandler(
		config.DB,
		config.InstanceID,
		selectMasterChecker(config),
	)

	// Metrics endpoint (no authentication required)
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(
		metrics.Registry,
		promhttp.HandlerOpts{},
	)))

	// Health check routes (no authentication required)
	health := router.Group("/health")
	{
		health.GET("/live", healthHandler.Liveness)
		health.GET("/ready", healthHandler.Readiness)
		health.GET("/master", healthHandler.Master)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Node management endpoints (requires node token authentication)
	nodes := v1.Group("/nodes")
	nodes.Use(middleware.RequireNodeToken(authConfig))
	nodes.Use(middleware.RateLimitByNode(50.0, 100)) // 50 req/s per node
	{
		// POST /api/v1/nodes - Create new node (requires admin node)
		nodes.POST("", middleware.RequireAdminNode(), nodeHandler.CreateNode)

		// GET /api/v1/nodes - List nodes in cluster (requires admin node)
		nodes.GET("", middleware.RequireAdminNode(), nodeHandler.ListNodes)

		// PATCH /api/v1/nodes/:id/mtu - Update MTU (requires admin node)
		nodes.PATCH("/:id/mtu", middleware.RequireAdminNode(), nodeHandler.UpdateMTU)

		// POST /api/v1/nodes/:id/token - Rotate node token (requires admin node)
		nodes.POST("/:id/token", middleware.RequireAdminNode(), nodeHandler.RotateNodeToken)

		// DELETE /api/v1/nodes/:id - Delete node (requires admin node)
		nodes.DELETE("/:id", middleware.RequireAdminNode(), nodeHandler.DeleteNode)
	}

	// Config distribution endpoints (requires node token authentication)
	config_endpoints := v1.Group("/config")
	config_endpoints.Use(middleware.RequireNodeToken(authConfig))
	config_endpoints.Use(middleware.RateLimitByNode(10.0, 20)) // Lower limit for config downloads
	{
		// GET /api/v1/config/version - Check current config version
		config_endpoints.GET("/version", bundleHandler.GetVersion)

		// GET /api/v1/config/bundle - Download config bundle
		config_endpoints.GET("/bundle", bundleHandler.DownloadBundle)

		// POST /api/v1/config/bundle - Upload config bundle (requires admin node)
		config_endpoints.POST("/bundle", middleware.RequireAdminNode(), bundleHandler.UploadBundle)
	}

	// Topology management endpoints (requires cluster token authentication)
	topology := v1.Group("/topology")
	topology.Use(middleware.RequireClusterToken(authConfig))
	topology.Use(middleware.RateLimitByCluster(100.0, 200)) // 100 req/s per cluster
	{
		// GET /api/v1/topology - Get cluster topology
		topology.GET("", topologyHandler.GetTopology)

		// POST /api/v1/topology/lighthouse - Assign lighthouse
		topology.POST("/lighthouse", topologyHandler.AssignLighthouse)

		// DELETE /api/v1/topology/lighthouse/:node_id - Unassign lighthouse
		topology.DELETE("/lighthouse/:node_id", topologyHandler.UnassignLighthouse)

		// POST /api/v1/topology/relay - Assign relay
		topology.POST("/relay", topologyHandler.AssignRelay)

		// DELETE /api/v1/topology/relay/:node_id - Unassign relay
		topology.DELETE("/relay/:node_id", topologyHandler.UnassignRelay)
	}

	// Route management endpoints (requires node token authentication)
	routes := v1.Group("/routes")
	routes.Use(middleware.RequireNodeToken(authConfig))
	routes.Use(middleware.RateLimitByNode(20.0, 40)) // 20 req/s per node for route updates
	{
		// GET /api/v1/routes - Get node's advertised routes
		routes.GET("", topologyHandler.GetRoutes)

		// PUT /api/v1/routes - Update node's advertised routes
		routes.PUT("", topologyHandler.UpdateRoutes)

		// GET /api/v1/routes/cluster - Get all routes in cluster
		routes.GET("/cluster", topologyHandler.GetClusterRoutes)
	}

	// Token rotation endpoints
	tokens := v1.Group("/tokens")
	{
		// POST /api/v1/tokens/cluster/rotate - Rotate cluster token (requires cluster token)
		tokens.POST("/cluster/rotate",
			middleware.RequireClusterToken(authConfig),
			topologyHandler.RotateClusterToken)
	}

	return router
}

// selectMasterChecker returns the appropriate master-check function.
// Defaults to always-master in single-instance mode.
func selectMasterChecker(config *RouterConfig) func() (bool, string, error) {
	if config.HAManager != nil {
		return config.HAManager.IsMaster
	}

	return func() (bool, string, error) {
		return true, "", nil
	}
}
