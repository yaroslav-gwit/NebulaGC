// Package bench provides performance benchmarking utilities for the NebulaGC server.
//
// This package includes helpers for:
//   - Setting up test databases with realistic data
//   - Creating HTTP clients for API benchmarking
//   - Measuring latency percentiles (p50, p95, p99)
//   - Generating load test data
//
// Note: Database benchmarks test SQLite directly.
// API benchmarks require a running server (set BENCH_SERVER_URL, BENCH_ADMIN_TOKEN, BENCH_CLUSTER_ID).
package bench

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nebulagc.io/models"
)

// BenchmarkConfig holds configuration for API benchmarks.
type BenchmarkConfig struct {
	ServerURL  string
	AdminToken string
	ClusterID  string
}

// GetBenchConfig returns configuration from environment or defaults.
func GetBenchConfig(tb testing.TB) *BenchmarkConfig {
	tb.Helper()

	serverURL := os.Getenv("BENCH_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	adminToken := os.Getenv("BENCH_ADMIN_TOKEN")
	if adminToken == "" {
		tb.Skip("BENCH_ADMIN_TOKEN not set - skipping API benchmark")
	}

	clusterID := os.Getenv("BENCH_CLUSTER_ID")
	if clusterID == "" {
		tb.Skip("BENCH_CLUSTER_ID not set - skipping API benchmark")
	}

	return &BenchmarkConfig{
		ServerURL:  serverURL,
		AdminToken: adminToken,
		ClusterID:  clusterID,
	}
}

// SetupTestDB creates a temporary database for benchmarking.
func SetupTestDB(tb testing.TB) (*sql.DB, string) {
	tb.Helper()

	dbPath := fmt.Sprintf("/tmp/bench-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		tb.Fatalf("Failed to open database: %v", err)
	}

	tb.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})

	// Run migrations
	if err := runMigrations(db); err != nil {
		tb.Fatalf("Failed to run migrations: %v", err)
	}

	// Create test tenant and cluster
	tenantID, clusterID := seedTestData(tb, db)

	tb.Logf("Test database: %s (tenant=%s, cluster=%s)", dbPath, tenantID, clusterID)

	return db, clusterID
}

// runMigrations applies database migrations.
func runMigrations(db *sql.DB) error {
	migrations := []string{
		// Tenants table
		`CREATE TABLE IF NOT EXISTS tenants (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Clusters table
		`CREATE TABLE IF NOT EXISTS clusters (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			UNIQUE(tenant_id, name)
		)`,
		// Cluster state table
		`CREATE TABLE IF NOT EXISTS cluster_state (
			cluster_id TEXT PRIMARY KEY,
			config_version INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
		)`,
		// Nodes table
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			cluster_id TEXT NOT NULL,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			mtu INTEGER NOT NULL DEFAULT 1300,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
			UNIQUE(cluster_id, name)
		)`,
		// Config bundles table
		`CREATE TABLE IF NOT EXISTS config_bundles (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			cluster_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			bundle_data BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
			UNIQUE(cluster_id, version)
		)`,
		// Replicas table
		`CREATE TABLE IF NOT EXISTS replicas (
			id TEXT PRIMARY KEY,
			cluster_id TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			replica_url TEXT NOT NULL,
			last_heartbeat DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
			UNIQUE(cluster_id, instance_id)
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// seedTestData creates a tenant and cluster for testing.
func seedTestData(tb testing.TB, db *sql.DB) (tenantID, clusterID string) {
	tb.Helper()

	tenantID = "bench-tenant-id"
	clusterID = "bench-cluster-id"

	// Insert tenant
	_, err := db.Exec(`INSERT INTO tenants (id, name) VALUES (?, ?)`, tenantID, "Benchmark Tenant")
	if err != nil {
		tb.Fatalf("Failed to insert tenant: %v", err)
	}

	// Insert cluster
	clusterTokenHash := "cluster-token-hash"
	_, err = db.Exec(`INSERT INTO clusters (id, tenant_id, name, token_hash) VALUES (?, ?, ?, ?)`,
		clusterID, tenantID, "Benchmark Cluster", clusterTokenHash)
	if err != nil {
		tb.Fatalf("Failed to insert cluster: %v", err)
	}

	// Insert cluster state
	_, err = db.Exec(`INSERT INTO cluster_state (cluster_id, config_version) VALUES (?, 0)`, clusterID)
	if err != nil {
		tb.Fatalf("Failed to insert cluster state: %v", err)
	}

	return tenantID, clusterID
}

// SeedNodes creates N nodes in the database for testing.
func SeedNodes(tb testing.TB, db *sql.DB, clusterID string, count int) []string {
	tb.Helper()

	nodeIDs := make([]string, count)
	for i := 0; i < count; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		tokenHash := fmt.Sprintf("token-hash-%d", i)
		_, err := db.Exec(`INSERT INTO nodes (id, cluster_id, name, token_hash, is_admin) VALUES (?, ?, ?, ?, 0)`,
			nodeID, clusterID, fmt.Sprintf("node-%d", i), tokenHash)
		if err != nil {
			tb.Fatalf("Failed to seed node: %v", err)
		}
		nodeIDs[i] = nodeID
	}

	return nodeIDs
}

// SeedBundle creates a config bundle in the database.
func SeedBundle(tb testing.TB, db *sql.DB, tenantID, clusterID string, version int, sizeBytes int) string {
	tb.Helper()

	bundleID := fmt.Sprintf("bundle-%d", version)
	bundleData := generateRandomData(sizeBytes)

	_, err := db.Exec(`INSERT INTO config_bundles (id, tenant_id, cluster_id, version, bundle_data) VALUES (?, ?, ?, ?, ?)`,
		bundleID, tenantID, clusterID, version, bundleData)
	if err != nil {
		tb.Fatalf("Failed to seed bundle: %v", err)
	}

	// Update cluster state version
	_, err = db.Exec(`UPDATE cluster_state SET config_version = ?, updated_at = CURRENT_TIMESTAMP WHERE cluster_id = ?`,
		version, clusterID)
	if err != nil {
		tb.Fatalf("Failed to update cluster state: %v", err)
	}

	return bundleID
}

// generateRandomData creates random bytes for bundle data.
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return data
}

// HTTPClient wraps http.Client with helper methods for benchmarking.
type HTTPClient struct {
	Client  *http.Client
	BaseURL string
}

// NewHTTPClient creates a new HTTP client for benchmarking.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		Client:  &http.Client{Timeout: 30 * time.Second},
		BaseURL: baseURL,
	}
}

// Get performs a GET request.
func (c *HTTPClient) Get(path string, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("X-NebulaGC-Node-Token", token)
	}
	return c.Client.Do(req)
}

// Post performs a POST request with JSON body.
func (c *HTTPClient) Post(path string, token string, body interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-NebulaGC-Node-Token", token)
	}
	return c.Client.Do(req)
}

// PostGzip performs a POST request with gzip body.
func (c *HTTPClient) PostGzip(path string, token string, data []byte) (*http.Response, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(data); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/gzip")
	if token != "" {
		req.Header.Set("X-NebulaGC-Node-Token", token)
	}
	return c.Client.Do(req)
}

// LatencyStats holds latency statistics.
type LatencyStats struct {
	Min    time.Duration
	Max    time.Duration
	Mean   time.Duration
	P50    time.Duration
	P95    time.Duration
	P99    time.Duration
	Stddev time.Duration
}

// CalculateLatencyStats computes latency percentiles from a slice of durations.
func CalculateLatencyStats(latencies []time.Duration) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var sum time.Duration
	for _, lat := range sorted {
		sum += lat
	}
	mean := sum / time.Duration(len(sorted))

	// Calculate standard deviation
	var variance float64
	for _, lat := range sorted {
		diff := float64(lat - mean)
		variance += diff * diff
	}
	variance /= float64(len(sorted))
	stddev := time.Duration(variance)

	return LatencyStats{
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Mean:   mean,
		P50:    sorted[len(sorted)/2],
		P95:    sorted[int(float64(len(sorted))*0.95)],
		P99:    sorted[int(float64(len(sorted))*0.99)],
		Stddev: stddev,
	}
}

// ReadAll reads the response body and closes it.
func ReadAll(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CreateTestNode creates a node for testing.
func CreateTestNode(name string, isAdmin bool) *models.Node {
	return &models.Node{
		ID:        fmt.Sprintf("node-%s", name),
		ClusterID: "test-cluster",
		Name:      name,
		TokenHash: "test-hash",
		IsAdmin:   isAdmin,
		MTU:       1300,
		CreatedAt: time.Now(),
	}
}
