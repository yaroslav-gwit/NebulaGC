package service

import (
	"database/sql"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	_ "modernc.org/sqlite"
)

// createTestDB builds an in-memory SQLite database with the replicas schema.
func createTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	schema := `
CREATE TABLE replicas (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK(role IN ('master','replica')),
    created_at DATETIME NOT NULL,
    last_seen_at DATETIME
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create replicas table: %v", err)
	}

	return db
}

func newTestLogger() *zap.Logger {
	core, _ := observer.New(zap.InfoLevel)
	return zap.New(core)
}

func TestRegisterAndHeartbeat(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	svc := NewReplicaService(db, newTestLogger())

	if err := svc.Register("id-1", "https://one.example.com", "master"); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	var address, role string
	var lastSeen time.Time
	if err := db.QueryRow(`SELECT address, role, last_seen_at FROM replicas WHERE id = ?`, "id-1").Scan(&address, &role, &lastSeen); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if address != "https://one.example.com" || role != "master" {
		t.Fatalf("unexpected row: address=%s role=%s", address, role)
	}

	// Update existing replica
	if err := svc.Register("id-1", "https://new.example.com", "replica"); err != nil {
		t.Fatalf("register update failed: %v", err)
	}

	if err := db.QueryRow(`SELECT address, role FROM replicas WHERE id = ?`, "id-1").Scan(&address, &role); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if address != "https://new.example.com" || role != "replica" {
		t.Fatalf("unexpected updated row: address=%s role=%s", address, role)
	}

	// Heartbeat should succeed
	if err := svc.SendHeartbeat("id-1"); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	// Heartbeat on missing replica should error
	if err := svc.SendHeartbeat("missing"); err == nil {
		t.Fatal("expected heartbeat error for missing replica")
	}
}

func TestMasterSelectionAndListing(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	now := time.Now()

	entries := []struct {
		id    string
		addr  string
		role  string
		age   time.Duration
		alive bool
	}{
		{"id-1", "https://one.example.com", "master", 30 * time.Minute, true},
		{"id-2", "https://two.example.com", "replica", 20 * time.Minute, true},
		{"id-3", "https://three.example.com", "replica", 10 * time.Minute, false},
	}

	for _, e := range entries {
		lastSeen := now
		if !e.alive {
			lastSeen = now.Add(-1 * time.Hour)
		}
		createdAt := now.Add(-e.age)
		if _, err := db.Exec(
			`INSERT INTO replicas (id, address, role, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
			e.id, e.addr, e.role, createdAt, lastSeen,
		); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	svc := NewReplicaService(db, newTestLogger())

	master, err := svc.GetMaster(30*time.Second, "id-1")
	if err != nil {
		t.Fatalf("GetMaster failed: %v", err)
	}
	if master.InstanceID != "id-1" || !master.IsSelf {
		t.Fatalf("expected id-1 as master and self when current id is master, got %+v", master)
	}

	// Ask from replica perspective
	master, err = svc.GetMaster(30*time.Second, "id-2")
	if err != nil {
		t.Fatalf("GetMaster failed: %v", err)
	}
	if master.InstanceID != "id-1" || master.Address != "https://one.example.com" || master.IsSelf {
		t.Fatalf("unexpected master info: %+v", master)
	}

	replicas, err := svc.ListReplicas(30*time.Second, "id-2")
	if err != nil {
		t.Fatalf("ListReplicas failed: %v", err)
	}
	if len(replicas) != 2 { // third is stale
		t.Fatalf("expected 2 replicas, got %d", len(replicas))
	}
	if !replicas[0].IsMaster || replicas[0].InstanceID != "id-1" {
		t.Fatalf("first replica should be master, got %+v", replicas[0])
	}
}

func TestPruneAndValidateMasters(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	now := time.Now()

	_, err := db.Exec(
		`INSERT INTO replicas (id, address, role, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
		"id-1", "https://one.example.com", "master", now.Add(-time.Hour), now.Add(-2*time.Hour),
	)
	if err != nil {
		t.Fatalf("insert master failed: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO replicas (id, address, role, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
		"id-2", "https://two.example.com", "master", now.Add(-30*time.Minute), now,
	)
	if err != nil {
		t.Fatalf("insert second master failed: %v", err)
	}

	svc := NewReplicaService(db, newTestLogger())

	if err := svc.ValidateSingleMaster(); err == nil {
		t.Fatal("expected validation error for multiple masters")
	}

	pruned, err := svc.PruneStale(30*time.Minute, 2)
	if err != nil {
		t.Fatalf("PruneStale failed: %v", err)
	}
	if pruned == 0 {
		t.Fatal("expected stale replicas to be pruned")
	}
}
