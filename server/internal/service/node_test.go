package service

import (
	"context"
	"database/sql"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	_ "modernc.org/sqlite"
	"nebulagc.io/models"
)

func newNodeTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	schema := `
CREATE TABLE clusters (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    config_version INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    cluster_id TEXT NOT NULL,
    name TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    token_hash TEXT NOT NULL,
    mtu INTEGER NOT NULL DEFAULT 1300 CHECK(mtu >= 1280 AND mtu <= 9000),
    routes TEXT,
    routes_updated_at DATETIME,
    is_lighthouse INTEGER NOT NULL DEFAULT 0,
    lighthouse_public_ip TEXT,
    lighthouse_port INTEGER,
    is_relay INTEGER NOT NULL DEFAULT 0,
    lighthouse_relay_updated_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, cluster_id, name)
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func newNodeService(t *testing.T) (*NodeService, *sql.DB) {
	t.Helper()
	core, _ := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	db := newNodeTestDB(t)
	return NewNodeService(db, logger, "secret-should-be-long-enough-123456"), db
}

func seedCluster(t *testing.T, db *sql.DB, tenantID, clusterID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO clusters (id, tenant_id) VALUES (?, ?)`, clusterID, tenantID); err != nil {
		t.Fatalf("seed cluster: %v", err)
	}
}

func TestCreateNodeAndList(t *testing.T) {
	svc, db := newNodeService(t)
	defer db.Close()

	const tenantID = "tenant-1"
	const clusterID = "cluster-1"
	seedCluster(t, db, tenantID, clusterID)

	req := &models.NodeCreateRequest{Name: "node-a", IsAdmin: true, MTU: 1400}
	creds, err := svc.CreateNode(context.Background(), tenantID, clusterID, "cluster-token", req)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	if creds.NodeID == "" || creds.NodeToken == "" {
		t.Fatalf("expected node id and token")
	}
	if creds.ClusterToken != "cluster-token" {
		t.Fatalf("expected cluster token echoed, got %q", creds.ClusterToken)
	}

	resp, err := svc.ListNodes(context.Background(), tenantID, clusterID, 1, 10)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}
	if resp.Total != 1 || len(resp.Nodes) != 1 {
		t.Fatalf("expected one node, got total=%d len=%d", resp.Total, len(resp.Nodes))
	}
	if resp.Nodes[0].Name != "node-a" || resp.Nodes[0].MTU != 1400 || !resp.Nodes[0].IsAdmin {
		t.Fatalf("unexpected node summary: %+v", resp.Nodes[0])
	}
}

func TestUpdateMTUAndRotateToken(t *testing.T) {
	svc, db := newNodeService(t)
	defer db.Close()
	tenantID := "tenant-2"
	clusterID := "cluster-2"
	seedCluster(t, db, tenantID, clusterID)

	req := &models.NodeCreateRequest{Name: "node-b"}
	creds, err := svc.CreateNode(context.Background(), tenantID, clusterID, "", req)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	summary, err := svc.UpdateMTU(context.Background(), tenantID, clusterID, creds.NodeID, 1500)
	if err != nil {
		t.Fatalf("UpdateMTU failed: %v", err)
	}
	if summary.MTU != 1500 {
		t.Fatalf("expected mtu 1500, got %d", summary.MTU)
	}

	rotated, err := svc.RotateNodeToken(context.Background(), tenantID, clusterID, creds.NodeID)
	if err != nil {
		t.Fatalf("RotateNodeToken failed: %v", err)
	}
	if rotated.NodeToken == "" {
		t.Fatal("expected new node token")
	}
}

func TestDeleteNodeAndConfigBump(t *testing.T) {
	svc, db := newNodeService(t)
	defer db.Close()
	tenantID := "tenant-3"
	clusterID := "cluster-3"
	seedCluster(t, db, tenantID, clusterID)

	req := &models.NodeCreateRequest{Name: "node-c"}
	creds, err := svc.CreateNode(context.Background(), tenantID, clusterID, "", req)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	if err := svc.DeleteNode(context.Background(), tenantID, clusterID, creds.NodeID); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	var version int
	if err := db.QueryRow(`SELECT config_version FROM clusters WHERE id = ?`, clusterID).Scan(&version); err != nil {
		t.Fatalf("check config_version: %v", err)
	}
	if version != 3 { // initial 1 + create + delete
		t.Fatalf("expected config_version 3, got %d", version)
	}
}

func TestValidationErrors(t *testing.T) {
	svc, db := newNodeService(t)
	defer db.Close()
	tenantID := "tenant-4"
	clusterID := "cluster-4"
	seedCluster(t, db, tenantID, clusterID)

	_, err := svc.CreateNode(context.Background(), tenantID, clusterID, "", &models.NodeCreateRequest{Name: "", MTU: 1200})
	if err == nil {
		t.Fatal("expected error for invalid name/mtu")
	}

	if _, err := svc.UpdateMTU(context.Background(), tenantID, clusterID, "missing", 1500); err != models.ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}

	if _, err := svc.UpdateMTU(context.Background(), tenantID, clusterID, "missing", 9001); err != models.ErrInvalidMTU {
		t.Fatalf("expected ErrInvalidMTU, got %v", err)
	}
}
