package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
//
// This handler provides liveness, readiness, and master status checks
// for Kubernetes and load balancer health monitoring.
type HealthHandler struct {
	db         *sql.DB
	instanceID string
	isMaster   func() (bool, string, error) // Function to check if this instance is master
}

// NewHealthHandler creates a new health check handler.
//
// Parameters:
//   - db: Database connection for readiness checks
//   - instanceID: This control plane instance's UUID
//   - isMaster: Function that returns (isMaster bool, masterURL string, err error)
func NewHealthHandler(db *sql.DB, instanceID string, isMaster func() (bool, string, error)) *HealthHandler {
	return &HealthHandler{
		db:         db,
		instanceID: instanceID,
		isMaster:   isMaster,
	}
}

// LivenessResponse represents the liveness probe response.
type LivenessResponse struct {
	Status     string `json:"status"`
	InstanceID string `json:"instance_id"`
}

// ReadinessResponse represents the readiness probe response.
type ReadinessResponse struct {
	Status     string `json:"status"`
	InstanceID string `json:"instance_id"`
	Database   string `json:"database"`
}

// MasterResponse represents the master status response.
type MasterResponse struct {
	IsMaster   bool   `json:"is_master"`
	InstanceID string `json:"instance_id"`
	MasterURL  string `json:"master_url,omitempty"`
}

// Liveness handles GET /health/live for Kubernetes liveness probes.
//
// This endpoint always returns 200 OK as long as the HTTP server is running.
// It indicates that the process is alive and can accept requests.
//
// Response: 200 OK with JSON body {"status": "ok", "instance_id": "uuid"}
func (h *HealthHandler) Liveness(c *gin.Context) {
	respondSuccess(c, http.StatusOK, LivenessResponse{
		Status:     "ok",
		InstanceID: h.instanceID,
	})
}

// Readiness handles GET /health/ready for Kubernetes readiness probes.
//
// This endpoint checks if the instance is ready to serve traffic by:
// - Verifying database connectivity with a simple ping
//
// Returns:
//   - 200 OK if ready to serve traffic
//   - 503 Service Unavailable if not ready (database unreachable)
func (h *HealthHandler) Readiness(c *gin.Context) {
	// Check database connectivity
	if err := h.db.Ping(); err != nil {
		respondError(c, http.StatusServiceUnavailable, "unhealthy", "Database unavailable")
		return
	}

	respondSuccess(c, http.StatusOK, ReadinessResponse{
		Status:     "ready",
		InstanceID: h.instanceID,
		Database:   "connected",
	})
}

// Master handles GET /health/master for master status checks.
//
// This endpoint returns information about whether this instance is the master
// and provides the master URL for client failover.
//
// Returns:
//   - 200 OK with master status information
//   - 503 Service Unavailable if unable to determine master status
//
// Response includes:
//   - is_master: true if this instance is the master
//   - instance_id: this instance's UUID
//   - master_url: URL of the master instance (if this is not the master)
func (h *HealthHandler) Master(c *gin.Context) {
	isMaster, masterURL, err := h.isMaster()
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, "master_check_failed", "Unable to determine master status")
		return
	}

	response := MasterResponse{
		IsMaster:   isMaster,
		InstanceID: h.instanceID,
	}

	// Include master URL if we're not the master
	if !isMaster {
		response.MasterURL = masterURL
	}

	respondSuccess(c, http.StatusOK, response)
}
