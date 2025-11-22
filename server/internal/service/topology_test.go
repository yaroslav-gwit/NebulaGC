package service

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

// setupTopologyTestDB creates an in-memory database for topology testing.
func setupTopologyTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create schema
	schema := `
	CREATE TABLE tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE clusters (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		config_version INTEGER NOT NULL DEFAULT 1,
		cluster_token_hash TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE nodes (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		cluster_id TEXT NOT NULL,
		name TEXT NOT NULL,
		is_admin INTEGER NOT NULL DEFAULT 0,
		token_hash TEXT NOT NULL,
		mtu INTEGER NOT NULL DEFAULT 1300,
		routes TEXT,
		routes_updated_at INTEGER,
		is_lighthouse INTEGER NOT NULL DEFAULT 0,
		lighthouse_public_ip TEXT,
		lighthouse_port INTEGER,
		is_relay INTEGER NOT NULL DEFAULT 0,
		lighthouse_relay_updated_at INTEGER,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO tenants (id, name, created_at) VALUES ('tenant1', 'Test Tenant', 1000000000);
		INSERT INTO clusters (id, tenant_id, name, config_version, cluster_token_hash, created_at)
		VALUES ('cluster1', 'tenant1', 'Test Cluster', 1, 'hash', 1000000000);
		INSERT INTO nodes (id, tenant_id, cluster_id, name, token_hash, created_at)
		VALUES
			('node1', 'tenant1', 'cluster1', 'node-1', 'hash1', 1000000000),
			('node2', 'tenant1', 'cluster1', 'node-2', 'hash2', 1000000000),
			('node3', 'tenant1', 'cluster1', 'node-3', 'hash3', 1000000000);
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	return db
}

func TestTopologyService_UpdateRoutes(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Update routes with valid CIDRs
	routes := []string{"10.0.1.0/24", "10.0.2.0/24"}
	err := service.UpdateRoutes("node1", routes)
	if err != nil {
		t.Fatalf("UpdateRoutes failed: %v", err)
	}

	// Verify routes were stored
	storedRoutes, err := service.GetNodeRoutes("node1")
	if err != nil {
		t.Fatalf("GetNodeRoutes failed: %v", err)
	}

	if len(storedRoutes) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(storedRoutes))
	}

	// Verify config version was bumped
	var version int64
	err = db.QueryRow(`SELECT config_version FROM clusters WHERE id = 'cluster1'`).Scan(&version)
	if err != nil {
		t.Fatalf("Failed to get config version: %v", err)
	}

	if version != 2 {
		t.Errorf("Expected config version 2, got %d", version)
	}
}

func TestTopologyService_UpdateRoutesInvalidCIDR(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Try to update with invalid CIDR
	routes := []string{"invalid-cidr"}
	err := service.UpdateRoutes("node1", routes)

	if err == nil {
		t.Error("Expected error for invalid CIDR, got nil")
	}
}

func TestTopologyService_UpdateRoutesClearAll(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// First add some routes
	routes := []string{"10.0.1.0/24"}
	service.UpdateRoutes("node1", routes)

	// Clear routes with empty array
	err := service.UpdateRoutes("node1", []string{})
	if err != nil {
		t.Fatalf("Failed to clear routes: %v", err)
	}

	// Verify routes were cleared
	storedRoutes, err := service.GetNodeRoutes("node1")
	if err != nil {
		t.Fatalf("GetNodeRoutes failed: %v", err)
	}

	if len(storedRoutes) != 0 {
		t.Errorf("Expected 0 routes after clear, got %d", len(storedRoutes))
	}
}

func TestTopologyService_GetClusterRoutes(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Add routes to multiple nodes
	service.UpdateRoutes("node1", []string{"10.0.1.0/24"})
	service.UpdateRoutes("node2", []string{"10.0.2.0/24", "10.0.3.0/24"})

	// Get all cluster routes
	clusterRoutes, err := service.GetClusterRoutes("cluster1")
	if err != nil {
		t.Fatalf("GetClusterRoutes failed: %v", err)
	}

	if len(clusterRoutes) != 2 {
		t.Errorf("Expected routes from 2 nodes, got %d", len(clusterRoutes))
	}

	if len(clusterRoutes["node1"]) != 1 {
		t.Errorf("Expected 1 route for node1, got %d", len(clusterRoutes["node1"]))
	}

	if len(clusterRoutes["node2"]) != 2 {
		t.Errorf("Expected 2 routes for node2, got %d", len(clusterRoutes["node2"]))
	}
}

func TestTopologyService_SetLighthouse(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Set lighthouse status
	err := service.SetLighthouse("cluster1", "node1", "203.0.113.1", 4242)
	if err != nil {
		t.Fatalf("SetLighthouse failed: %v", err)
	}

	// Verify lighthouse status
	var isLighthouse int
	var publicIP string
	var port int
	err = db.QueryRow(`
		SELECT is_lighthouse, lighthouse_public_ip, lighthouse_port
		FROM nodes WHERE id = 'node1'
	`).Scan(&isLighthouse, &publicIP, &port)
	if err != nil {
		t.Fatalf("Failed to query lighthouse status: %v", err)
	}

	if isLighthouse != 1 {
		t.Error("Expected is_lighthouse = 1")
	}

	if publicIP != "203.0.113.1" {
		t.Errorf("Expected public IP 203.0.113.1, got %s", publicIP)
	}

	if port != 4242 {
		t.Errorf("Expected port 4242, got %d", port)
	}

	// Verify config version was bumped
	var version int64
	db.QueryRow(`SELECT config_version FROM clusters WHERE id = 'cluster1'`).Scan(&version)
	if version != 2 {
		t.Errorf("Expected config version 2, got %d", version)
	}
}

func TestTopologyService_UnsetLighthouse(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// First set lighthouse status
	service.SetLighthouse("cluster1", "node1", "203.0.113.1", 4242)

	// Now unset it
	err := service.UnsetLighthouse("cluster1", "node1")
	if err != nil {
		t.Fatalf("UnsetLighthouse failed: %v", err)
	}

	// Verify lighthouse status cleared
	var isLighthouse int
	err = db.QueryRow(`SELECT is_lighthouse FROM nodes WHERE id = 'node1'`).Scan(&isLighthouse)
	if err != nil {
		t.Fatalf("Failed to query lighthouse status: %v", err)
	}

	if isLighthouse != 0 {
		t.Error("Expected is_lighthouse = 0")
	}
}

func TestTopologyService_SetRelay(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Set relay status
	err := service.SetRelay("cluster1", "node1")
	if err != nil {
		t.Fatalf("SetRelay failed: %v", err)
	}

	// Verify relay status
	var isRelay int
	err = db.QueryRow(`SELECT is_relay FROM nodes WHERE id = 'node1'`).Scan(&isRelay)
	if err != nil {
		t.Fatalf("Failed to query relay status: %v", err)
	}

	if isRelay != 1 {
		t.Error("Expected is_relay = 1")
	}

	// Verify config version was bumped
	var version int64
	db.QueryRow(`SELECT config_version FROM clusters WHERE id = 'cluster1'`).Scan(&version)
	if version != 2 {
		t.Errorf("Expected config version 2, got %d", version)
	}
}

func TestTopologyService_UnsetRelay(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// First set relay status
	service.SetRelay("cluster1", "node1")

	// Now unset it
	err := service.UnsetRelay("cluster1", "node1")
	if err != nil {
		t.Fatalf("UnsetRelay failed: %v", err)
	}

	// Verify relay status cleared
	var isRelay int
	err = db.QueryRow(`SELECT is_relay FROM nodes WHERE id = 'node1'`).Scan(&isRelay)
	if err != nil {
		t.Fatalf("Failed to query relay status: %v", err)
	}

	if isRelay != 0 {
		t.Error("Expected is_relay = 0")
	}
}

func TestTopologyService_GetTopology(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Set up topology
	service.SetLighthouse("cluster1", "node1", "203.0.113.1", 4242)
	service.SetRelay("cluster1", "node2")
	service.UpdateRoutes("node3", []string{"10.0.1.0/24"})

	// Get topology
	topology, err := service.GetTopology("cluster1")
	if err != nil {
		t.Fatalf("GetTopology failed: %v", err)
	}

	// Verify lighthouses
	if len(topology.Lighthouses) != 1 {
		t.Errorf("Expected 1 lighthouse, got %d", len(topology.Lighthouses))
	}

	if topology.Lighthouses[0].NodeID != "node1" {
		t.Errorf("Expected lighthouse node1, got %s", topology.Lighthouses[0].NodeID)
	}

	// Verify relays
	if len(topology.Relays) != 1 {
		t.Errorf("Expected 1 relay, got %d", len(topology.Relays))
	}

	if topology.Relays[0].NodeID != "node2" {
		t.Errorf("Expected relay node2, got %s", topology.Relays[0].NodeID)
	}

	// Verify routes
	if len(topology.Routes["node3"]) != 1 {
		t.Errorf("Expected 1 route for node3, got %d", len(topology.Routes["node3"]))
	}
}

func TestTopologyService_RotateClusterToken(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Rotate token
	newToken, err := service.RotateClusterToken("cluster1")
	if err != nil {
		t.Fatalf("RotateClusterToken failed: %v", err)
	}

	if len(newToken) < 41 {
		t.Errorf("Expected token >= 41 characters, got %d", len(newToken))
	}

	// Verify hash was updated in database
	var hash string
	err = db.QueryRow(`SELECT cluster_token_hash FROM clusters WHERE id = 'cluster1'`).Scan(&hash)
	if err != nil {
		t.Fatalf("Failed to query token hash: %v", err)
	}

	if hash == "hash" {
		t.Error("Token hash was not updated")
	}
}

func TestTopologyService_MultipleLighthouses(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Set multiple lighthouses
	service.SetLighthouse("cluster1", "node1", "203.0.113.1", 4242)
	service.SetLighthouse("cluster1", "node2", "203.0.113.2", 4242)

	// Get topology
	topology, err := service.GetTopology("cluster1")
	if err != nil {
		t.Fatalf("GetTopology failed: %v", err)
	}

	if len(topology.Lighthouses) != 2 {
		t.Errorf("Expected 2 lighthouses, got %d", len(topology.Lighthouses))
	}
}

func TestTopologyService_MultipleRelays(t *testing.T) {
	db := setupTopologyTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewTopologyService(db, logger, "secret")

	// Set multiple relays
	service.SetRelay("cluster1", "node1")
	service.SetRelay("cluster1", "node2")

	// Get topology
	topology, err := service.GetTopology("cluster1")
	if err != nil {
		t.Fatalf("GetTopology failed: %v", err)
	}

	if len(topology.Relays) != 2 {
		t.Errorf("Expected 2 relays, got %d", len(topology.Relays))
	}
}
