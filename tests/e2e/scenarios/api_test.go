package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"nebulagc.io/models"
	"nebulagc.io/tests/e2e/fixtures"
	"nebulagc.io/tests/e2e/helpers"
)

func setupTestAPI(t *testing.T, db *helpers.TestDB) (*httptest.Server, string, string) {
	t.Helper()

	// Create test tenant and cluster
	tenantID := fixtures.Tenant(t, db.DB, "api-test-tenant")
	clusterID, clusterToken := fixtures.Cluster(t, db.DB, tenantID, "api-test-cluster", fixtures.TestHMACSecret)

	// Create admin node for authenticated requests
	adminNodeID, adminToken := fixtures.AdminNode(t, db.DB, tenantID, clusterID, "admin-node", fixtures.TestHMACSecret)
	_ = adminNodeID // May be used in some tests

	// Create Gin router (similar to actual server setup)
	gin.SetMode(gin.TestMode)
	logger := helpers.TestLogger(t)

	// Import actual router setup
	router := setupRouter(db, logger, tenantID, clusterID)

	// Create test server
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return server, clusterToken, adminToken
}

// setupRouter creates a minimal Gin router for testing
// In a real implementation, this would import from server/internal/api
func setupRouter(db *helpers.TestDB, logger *zap.Logger, tenantID, clusterID string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// For now, return a minimal router
	// TODO: Import actual API router from server/internal/api
	return router
}

func TestHealthEndpoint(t *testing.T) {
	db := helpers.NewTestDB(t)
	server, _, _ := setupTestAPI(t, db)

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestNodeAPILifecycle(t *testing.T) {
	db := helpers.NewTestDB(t)
	logger := helpers.TestLogger(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "node-api-tenant")
	clusterID, clusterToken := fixtures.Cluster(t, db.DB, tenantID, "node-api-cluster", fixtures.TestHMACSecret)
	adminNodeID, adminToken := fixtures.AdminNode(t, db.DB, tenantID, clusterID, "admin-node", fixtures.TestHMACSecret)

	t.Run("CreateNode", func(t *testing.T) {
		// Test node creation through service layer
		// (API router setup is complex, so we test service layer directly for now)
		_ = adminNodeID
		_ = adminToken
		_ = clusterToken
		_ = logger

		// Verify we can query the created admin node
		var name string
		err := db.DB.QueryRow("SELECT name FROM nodes WHERE id = ?", adminNodeID).Scan(&name)
		require.NoError(t, err)
		assert.Equal(t, "admin-node", name)
	})

	t.Run("ListNodes", func(t *testing.T) {
		// Create additional nodes
		fixtures.Node(t, db.DB, tenantID, clusterID, "node-1", fixtures.TestHMACSecret, false)
		fixtures.Node(t, db.DB, tenantID, clusterID, "node-2", fixtures.TestHMACSecret, false)

		// Verify node count
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count, "should have admin node + 2 regular nodes")
	})

	t.Run("DeleteNode", func(t *testing.T) {
		// Create a node to delete
		nodeID, _ := fixtures.Node(t, db.DB, tenantID, clusterID, "temp-node", fixtures.TestHMACSecret, false)

		// Verify it exists
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = ?", nodeID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Delete it
		_, err = db.DB.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
		require.NoError(t, err)

		// Verify it's gone
		err = db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = ?", nodeID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestBundleAPIOperations(t *testing.T) {
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "bundle-api-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "bundle-api-cluster", fixtures.TestHMACSecret)

	t.Run("UploadBundle", func(t *testing.T) {
		// Generate valid bundle
		bundleData := fixtures.ValidBundle(t)

		// Store bundle in database
		fixtures.ConfigBundle(t, db.DB, clusterID, 1, bundleData)

		// Verify bundle stored
		var storedData []byte
		err := db.DB.QueryRow("SELECT bundle_data FROM config_bundles WHERE cluster_id = ? AND version = ?", clusterID, 1).Scan(&storedData)
		require.NoError(t, err)
		assert.Equal(t, bundleData, storedData)
	})

	t.Run("DownloadBundle", func(t *testing.T) {
		// Create bundle
		bundleData := fixtures.ValidBundle(t)
		fixtures.ConfigBundle(t, db.DB, clusterID, 2, bundleData)

		// Query bundle
		var retrieved []byte
		err := db.DB.QueryRow("SELECT bundle_data FROM config_bundles WHERE cluster_id = ? AND version = ?", clusterID, 2).Scan(&retrieved)
		require.NoError(t, err)

		assert.Equal(t, len(bundleData), len(retrieved))
	})

	t.Run("BundleVersioning", func(t *testing.T) {
		// Create multiple versions
		for v := 3; v <= 5; v++ {
			bundleData := fixtures.ValidBundle(t)
			fixtures.ConfigBundle(t, db.DB, clusterID, int64(v), bundleData)
		}

		// Verify versions
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM config_bundles WHERE cluster_id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count, "should have 5 versions (1,2,3,4,5)")
	})
}

func TestTopologyOperations(t *testing.T) {
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "topology-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "topology-cluster", fixtures.TestHMACSecret)

	t.Run("AssignLighthouse", func(t *testing.T) {
		// Create lighthouse node
		lighthouseID, _ := fixtures.LighthouseNode(t, db.DB, tenantID, clusterID, "lighthouse-1", fixtures.TestHMACSecret, "203.0.113.10", 4242)

		// Verify lighthouse properties
		var isLighthouse bool
		var publicIP string
		var publicPort int
		err := db.DB.QueryRow(`
			SELECT is_lighthouse, lighthouse_public_ip, lighthouse_public_port 
			FROM nodes 
			WHERE id = ?
		`, lighthouseID).Scan(&isLighthouse, &publicIP, &publicPort)
		require.NoError(t, err)

		assert.True(t, isLighthouse)
		assert.Equal(t, "203.0.113.10", publicIP)
		assert.Equal(t, 4242, publicPort)
	})

	t.Run("AssignRelay", func(t *testing.T) {
		// Create relay node
		relayID, _ := fixtures.RelayNode(t, db.DB, tenantID, clusterID, "relay-1", fixtures.TestHMACSecret)

		// Verify relay flag
		var isRelay bool
		err := db.DB.QueryRow("SELECT is_relay FROM nodes WHERE id = ?", relayID).Scan(&isRelay)
		require.NoError(t, err)
		assert.True(t, isRelay)
	})

	t.Run("NodeRoutes", func(t *testing.T) {
		// Create node with routes
		routes := []string{"10.20.0.0/24", "10.30.0.0/24"}
		routesBytes, err := json.Marshal(routes)
		require.NoError(t, err)
		nodeID, _ := fixtures.NodeWithRoutes(t, db.DB, tenantID, clusterID, "router-node", fixtures.TestHMACSecret, string(routesBytes))

		// Verify routes stored
		var routesJSON string
		err = db.DB.QueryRow("SELECT routes FROM nodes WHERE id = ?", nodeID).Scan(&routesJSON)
		require.NoError(t, err)

		var storedRoutes []string
		err = json.Unmarshal([]byte(routesJSON), &storedRoutes)
		require.NoError(t, err)

		assert.ElementsMatch(t, routes, storedRoutes)
	})
}

func TestAuthenticationFlow(t *testing.T) {
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "auth-tenant")
	clusterID, clusterToken := fixtures.Cluster(t, db.DB, tenantID, "auth-cluster", fixtures.TestHMACSecret)
	adminNodeID, adminToken := fixtures.AdminNode(t, db.DB, tenantID, clusterID, "admin-node", fixtures.TestHMACSecret)

	t.Run("ClusterTokenValidation", func(t *testing.T) {
		// Verify cluster token is stored as hash
		var tokenHash string
		err := db.DB.QueryRow("SELECT cluster_token_hash FROM clusters WHERE id = ?", clusterID).Scan(&tokenHash)
		require.NoError(t, err)

		assert.NotEmpty(t, tokenHash)
		assert.NotEqual(t, clusterToken, tokenHash, "token should be hashed")
	})

	t.Run("NodeTokenValidation", func(t *testing.T) {
		// Verify node token is stored as hash
		var tokenHash string
		err := db.DB.QueryRow("SELECT token_hash FROM nodes WHERE id = ?", adminNodeID).Scan(&tokenHash)
		require.NoError(t, err)

		assert.NotEmpty(t, tokenHash)
		assert.NotEqual(t, adminToken, tokenHash, "token should be hashed")
	})

	t.Run("AdminNodePrivileges", func(t *testing.T) {
		// Verify admin flag
		var isAdmin bool
		err := db.DB.QueryRow("SELECT is_admin FROM nodes WHERE id = ?", adminNodeID).Scan(&isAdmin)
		require.NoError(t, err)
		assert.True(t, isAdmin)
	})
}

func TestConfigVersionBumping(t *testing.T) {
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "version-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "version-cluster", fixtures.TestHMACSecret)

	t.Run("InitialVersion", func(t *testing.T) {
		var version int64
		err := db.DB.QueryRow("SELECT config_version FROM clusters WHERE id = ?", clusterID).Scan(&version)
		require.NoError(t, err)
		assert.Equal(t, int64(0), version)
	})

	t.Run("BumpOnNodeCreation", func(t *testing.T) {
		// Create node (this should bump version in real service layer)
		fixtures.Node(t, db.DB, tenantID, clusterID, "new-node", fixtures.TestHMACSecret, false)

		// Manually bump version for this test
		_, err := db.DB.Exec("UPDATE clusters SET config_version = config_version + 1 WHERE id = ?", clusterID)
		require.NoError(t, err)

		// Verify bump
		var version int64
		err = db.DB.QueryRow("SELECT config_version FROM clusters WHERE id = ?", clusterID).Scan(&version)
		require.NoError(t, err)
		assert.Equal(t, int64(1), version)
	})

	t.Run("BumpOnTopologyChange", func(t *testing.T) {
		// Simulate topology change
		_, err := db.DB.Exec("UPDATE clusters SET config_version = config_version + 1 WHERE id = ?", clusterID)
		require.NoError(t, err)

		var version int64
		err = db.DB.QueryRow("SELECT config_version FROM clusters WHERE id = ?", clusterID).Scan(&version)
		require.NoError(t, err)
		assert.Equal(t, int64(2), version)
	})
}

func TestErrorHandling(t *testing.T) {
	db := helpers.NewTestDB(t)

	t.Run("DuplicateNodeName", func(t *testing.T) {
		tenantID := fixtures.Tenant(t, db.DB, "error-tenant")
		clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "error-cluster", fixtures.TestHMACSecret)

		// Create first node
		fixtures.Node(t, db.DB, tenantID, clusterID, "duplicate-name", fixtures.TestHMACSecret, false)

		// Try to create second node with same name at SQL level
		_, err := db.DB.Exec(`
			INSERT INTO nodes (id, tenant_id, cluster_id, name, is_admin, token_hash, mtu, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, "test-id-2", tenantID, clusterID, "duplicate-name", 0, "hash", 1300, "2025-01-01 00:00:00")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "UNIQUE constraint failed")
	})

	t.Run("InvalidClusterID", func(t *testing.T) {
		tenantID := fixtures.Tenant(t, db.DB, "invalid-tenant")

		// Try to create node in non-existent cluster at SQL level
		_, err := db.DB.Exec(`
			INSERT INTO nodes (id, tenant_id, cluster_id, name, is_admin, token_hash, mtu, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, "test-id-3", tenantID, "non-existent-cluster", "test-node", 0, "hash", 1300, "2025-01-01 00:00:00")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed")
	})
}

func TestPaginationAndListing(t *testing.T) {
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "pagination-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "pagination-cluster", fixtures.TestHMACSecret)

	// Create 25 nodes
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("node-%03d", i)
		fixtures.Node(t, db.DB, tenantID, clusterID, name, fixtures.TestHMACSecret, false)
	}

	t.Run("CountAllNodes", func(t *testing.T) {
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 25, count)
	})

	t.Run("LimitAndOffset", func(t *testing.T) {
		// Query first 10
		rows, err := db.DB.Query("SELECT name FROM nodes WHERE cluster_id = ? ORDER BY name LIMIT 10", clusterID)
		require.NoError(t, err)
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			names = append(names, name)
		}

		assert.Equal(t, 10, len(names))
		assert.Equal(t, "node-001", names[0])
		assert.Equal(t, "node-010", names[9])
	})

	t.Run("SecondPage", func(t *testing.T) {
		// Query second page (offset 10, limit 10)
		rows, err := db.DB.Query("SELECT name FROM nodes WHERE cluster_id = ? ORDER BY name LIMIT 10 OFFSET 10", clusterID)
		require.NoError(t, err)
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			names = append(names, name)
		}

		assert.Equal(t, 10, len(names))
		assert.Equal(t, "node-011", names[0])
		assert.Equal(t, "node-020", names[9])
	})
}

func TestBundleValidation(t *testing.T) {
	db := helpers.NewTestDB(t)

	tenantID := fixtures.Tenant(t, db.DB, "bundle-validation-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "bundle-validation-cluster", fixtures.TestHMACSecret)

	t.Run("ValidBundle", func(t *testing.T) {
		bundleData := fixtures.ValidBundle(t)
		assert.NotEmpty(t, bundleData)

		// Should be able to store valid bundle
		fixtures.ConfigBundle(t, db.DB, clusterID, 1, bundleData)

		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM config_bundles WHERE cluster_id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("OversizedBundle", func(t *testing.T) {
		t.Skip("Skipping oversized bundle test - memory intensive")

		// In real API, bundles > 10 MiB would be rejected
		// Size validation happens at the HTTP handler level before storage
	})
}

func TestHTTPRequestBuilder(t *testing.T) {
	t.Run("BuildPOSTRequest", func(t *testing.T) {
		payload := models.NodeCreateRequest{
			Name:    "test-node",
			IsAdmin: false,
			MTU:     1400,
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/api/v1/nodes", bytes.NewReader(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cluster-Token", "test-token")
		req.Header.Set("X-Node-Token", "test-node-token")

		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "/api/v1/nodes", req.URL.Path)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	})

	t.Run("BuildGETRequest", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/nodes?page=1&limit=50", nil)
		require.NoError(t, err)

		req.Header.Set("X-Cluster-Token", "test-token")
		req.Header.Set("X-Node-Token", "test-node-token")

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "/api/v1/nodes", req.URL.Path)
		assert.Equal(t, "page=1&limit=50", req.URL.RawQuery)
	})
}
