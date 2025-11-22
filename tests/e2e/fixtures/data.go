package fixtures

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"nebulagc.io/models"
	"nebulagc.io/pkg/token"
)

// Tenant creates a test tenant in the database.
func Tenant(t *testing.T, db *sql.DB, name string) string {
	t.Helper()

	tenantID := uuid.New().String()

	_, err := db.Exec(`
		INSERT INTO tenants (id, name, created_at)
		VALUES (?, ?, ?)
	`, tenantID, name, time.Now())

	if err != nil {
		t.Fatalf("failed to create test tenant: %v", err)
	}

	return tenantID
}

// Cluster creates a test cluster in the database and returns cluster ID and token.
func Cluster(t *testing.T, db *sql.DB, tenantID, name, hmacSecret string) (string, string) {
	t.Helper()

	clusterID := uuid.New().String()
	clusterToken, err := token.Generate()
	if err != nil {
		t.Fatalf("failed to generate cluster token: %v", err)
	}

	tokenHash := token.Hash(clusterToken, hmacSecret)

	_, err = db.Exec(`
		INSERT INTO clusters (id, tenant_id, name, lighthouse_port, cluster_token_hash, config_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, clusterID, tenantID, name, 4242, tokenHash, 0, time.Now())

	if err != nil {
		t.Fatalf("failed to create test cluster: %v", err)
	}

	return clusterID, clusterToken
}

// ClusterWithVersion creates a test cluster with specific config version.
func ClusterWithVersion(t *testing.T, db *sql.DB, tenantID, name, hmacSecret string, version int64) (string, string) {
	t.Helper()

	clusterID, clusterToken := Cluster(t, db, tenantID, name, hmacSecret)

	_, err := db.Exec(`UPDATE clusters SET config_version = ? WHERE id = ?`, version, clusterID)
	if err != nil {
		t.Fatalf("failed to update cluster version: %v", err)
	}

	return clusterID, clusterToken
}

// Node creates a test node in the database and returns node ID and token.
func Node(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret string, isAdmin bool) (string, string) {
	t.Helper()

	nodeID := uuid.New().String()
	nodeToken, err := token.Generate()
	if err != nil {
		t.Fatalf("failed to generate node token: %v", err)
	}

	tokenHash := token.Hash(nodeToken, hmacSecret)

	_, err = db.Exec(`
		INSERT INTO nodes (id, tenant_id, cluster_id, name, is_admin, token_hash, mtu, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, nodeID, tenantID, clusterID, name, isAdmin, tokenHash, 1300, time.Now())

	if err != nil {
		t.Fatalf("failed to create test node: %v", err)
	}

	return nodeID, nodeToken
}

// AdminNode creates an admin node.
func AdminNode(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret string) (string, string) {
	return Node(t, db, tenantID, clusterID, name, hmacSecret, true)
}

// RegularNode creates a regular (non-admin) node.
func RegularNode(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret string) (string, string) {
	return Node(t, db, tenantID, clusterID, name, hmacSecret, false)
}

// NodeWithRoutes creates a node with specific routes.
func NodeWithRoutes(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret string, routes string) (string, string) {
	t.Helper()

	nodeID, nodeToken := Node(t, db, tenantID, clusterID, name, hmacSecret, false)

	_, err := db.Exec(`
		UPDATE nodes 
		SET routes = ?
		WHERE id = ?
	`, routes, nodeID)

	if err != nil {
		t.Fatalf("failed to update node routes: %v", err)
	}

	return nodeID, nodeToken
}

// LighthouseNode creates a lighthouse node.
func LighthouseNode(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret, publicIP string, port int) (string, string) {
	t.Helper()

	nodeID, nodeToken := Node(t, db, tenantID, clusterID, name, hmacSecret, false)

	_, err := db.Exec(`
		UPDATE nodes 
		SET is_lighthouse = 1, lighthouse_public_ip = ?, lighthouse_public_port = ?
		WHERE id = ?
	`, publicIP, port, nodeID)

	if err != nil {
		t.Fatalf("failed to update lighthouse status: %v", err)
	}

	return nodeID, nodeToken
}

// RelayNode creates a relay node.
func RelayNode(t *testing.T, db *sql.DB, tenantID, clusterID, name, hmacSecret string) (string, string) {
	t.Helper()

	nodeID, nodeToken := Node(t, db, tenantID, clusterID, name, hmacSecret, false)

	_, err := db.Exec(`
		UPDATE nodes 
		SET is_relay = 1
		WHERE id = ?
	`, nodeID)

	if err != nil {
		t.Fatalf("failed to update relay status: %v", err)
	}

	return nodeID, nodeToken
}

// ConfigBundle inserts a config bundle into the database.
func ConfigBundle(t *testing.T, db *sql.DB, clusterID string, version int64, data []byte) string {
	t.Helper()

	bundleID := uuid.New().String()

	// Get tenant_id from cluster
	var tenantID string
	err := db.QueryRow("SELECT tenant_id FROM clusters WHERE id = ?", clusterID).Scan(&tenantID)
	if err != nil {
		t.Fatalf("failed to get tenant_id for cluster: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO config_bundles (id, tenant_id, cluster_id, version, bundle_data, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, bundleID, tenantID, clusterID, version, data, time.Now())

	if err != nil {
		t.Fatalf("failed to create config bundle: %v", err)
	}

	return bundleID
}

// Replica creates a replica entry in the database.
func Replica(t *testing.T, db *sql.DB, clusterID, instanceID, replicaURL string) string {
	t.Helper()

	replicaID := uuid.New().String()

	_, err := db.Exec(`
		INSERT INTO replicas (id, cluster_id, instance_id, replica_url, is_active, priority, last_seen, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, replicaID, clusterID, instanceID, replicaURL, 1, 100, time.Now(), time.Now())

	if err != nil {
		t.Fatalf("failed to create replica: %v", err)
	}

	return replicaID
}

// ClusterState creates a cluster state entry.
func ClusterState(t *testing.T, db *sql.DB, clusterID, instanceID string, isMaster bool) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO cluster_state (cluster_id, instance_id, is_master, last_heartbeat)
		VALUES (?, ?, ?, ?)
	`, clusterID, instanceID, isMaster, time.Now())

	if err != nil {
		t.Fatalf("failed to create cluster state: %v", err)
	}
}

// AuthHeaders returns HTTP headers for cluster token authentication.
func AuthHeaders(clusterToken string) map[string]string {
	return map[string]string{
		"X-NebulaGC-Cluster-Token": clusterToken,
	}
}

// NodeAuthHeaders returns HTTP headers for node token authentication.
func NodeAuthHeaders(nodeToken string) map[string]string {
	return map[string]string{
		"X-NebulaGC-Node-Token": nodeToken,
	}
}

// RandomName generates a random name for testing.
func RandomName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}

// TestHMACSecret returns a test HMAC secret.
const TestHMACSecret = "test-hmac-secret-32-bytes-long!!"

// CreateTestClusterRequest creates a models.ClusterCreateRequest with test data.
func CreateTestClusterRequest(name string) *models.ClusterCreateRequest {
	return &models.ClusterCreateRequest{
		TenantID:       uuid.New().String(),
		Name:           name,
		LighthousePort: 4242,
	}
}

// CreateTestNodeRequest creates a models.NodeCreateRequest with test data.
func CreateTestNodeRequest(name string, isAdmin bool) *models.NodeCreateRequest {
	return &models.NodeCreateRequest{
		Name:    name,
		IsAdmin: isAdmin,
		MTU:     1300,
	}
}

// CreateTestRouteRequest creates a models.NodeRoutesRequest with test data.
func CreateTestRouteRequest(routes []string) *models.NodeRoutesRequest {
	return &models.NodeRoutesRequest{
		Routes: routes,
	}
}
