package helpers

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// TestDB wraps a test database connection with cleanup.
type TestDB struct {
	DB   *sql.DB
	Path string
	t    *testing.T
}

// NewTestDB creates a new temporary SQLite database for testing.
// The database is pre-populated with schema migrations.
// The database file is automatically cleaned up when the test ends.
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Create temp file for database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	testDB := &TestDB{
		DB:   db,
		Path: dbPath,
		t:    t,
	}

	// Register cleanup
	t.Cleanup(func() {
		db.Close()
	})

	return testDB
}

// runMigrations applies all schema migrations to the database.
func runMigrations(db *sql.DB) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_create_tenants",
			sql: `
				CREATE TABLE IF NOT EXISTS tenants (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL UNIQUE,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				);
			`,
		},
		{
			name: "002_create_clusters",
			sql: `
				CREATE TABLE IF NOT EXISTS clusters (
					id TEXT PRIMARY KEY,
					tenant_id TEXT NOT NULL,
					name TEXT NOT NULL,
					lighthouse_port INTEGER NOT NULL DEFAULT 4242,
					cluster_token_hash TEXT NOT NULL,
					config_version INTEGER NOT NULL DEFAULT 0,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					UNIQUE (tenant_id, name),
					FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
				);
			`,
		},
		{
			name: "003_create_cluster_state",
			sql: `
				CREATE TABLE IF NOT EXISTS cluster_state (
					cluster_id TEXT NOT NULL,
					instance_id TEXT NOT NULL,
					replica_discovery_url TEXT NOT NULL,
					is_master INTEGER NOT NULL DEFAULT 0,
					last_heartbeat DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (cluster_id, instance_id),
					FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
				);
			`,
		},
		{
			name: "004_create_replicas",
			sql: `
				CREATE TABLE IF NOT EXISTS replicas (
					id TEXT PRIMARY KEY,
					cluster_id TEXT NOT NULL,
					instance_id TEXT NOT NULL,
					replica_url TEXT NOT NULL,
					is_active INTEGER NOT NULL DEFAULT 1,
					priority INTEGER NOT NULL DEFAULT 100,
					last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
				);
			`,
		},
		{
			name: "005_create_nodes",
			sql: `
				CREATE TABLE IF NOT EXISTS nodes (
					id TEXT PRIMARY KEY,
					tenant_id TEXT NOT NULL,
					cluster_id TEXT NOT NULL,
					name TEXT NOT NULL,
					is_admin INTEGER NOT NULL DEFAULT 0,
					token_hash TEXT NOT NULL,
					mtu INTEGER NOT NULL DEFAULT 1300,
					is_lighthouse INTEGER NOT NULL DEFAULT 0,
					is_relay INTEGER NOT NULL DEFAULT 0,
					lighthouse_public_ip TEXT,
					lighthouse_public_port INTEGER,
					routes TEXT,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					UNIQUE (tenant_id, cluster_id, name),
					FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
					FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
				);
			`,
		},
		{
			name: "006_create_config_bundles",
			sql: `
				CREATE TABLE IF NOT EXISTS config_bundles (
					id TEXT PRIMARY KEY,
					tenant_id TEXT NOT NULL,
					cluster_id TEXT NOT NULL,
					version INTEGER NOT NULL,
					bundle_data BLOB NOT NULL,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					UNIQUE (tenant_id, cluster_id, version),
					FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
					FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
				);
			`,
		},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
	}

	return nil
}

// CleanupTables removes all data from all tables (preserving schema).
func CleanupTables(t *testing.T, db *sql.DB) {
	t.Helper()

	tables := []string{
		"config_bundles",
		"nodes",
		"replicas",
		"cluster_state",
		"clusters",
		"tenants",
	}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Fatalf("failed to clean table %s: %v", table, err)
		}
	}
}

// MustExec executes a SQL statement and fails the test if it errors.
func MustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	_, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("MustExec failed: %v\nQuery: %s", err, query)
	}
}

// MustQuery executes a SQL query and fails the test if it errors.
func MustQuery(t *testing.T, db *sql.DB, query string, args ...interface{}) *sql.Rows {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("MustQuery failed: %v\nQuery: %s", err, query)
	}
	return rows
}

// TestLogger creates a zap logger suitable for tests.
func TestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	return logger
}
