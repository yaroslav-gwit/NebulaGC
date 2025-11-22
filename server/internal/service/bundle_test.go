package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
	"go.uber.org/zap"
	"nebulagc.io/pkg/bundle"
)

// createTestBundle creates a valid tar.gz bundle for testing.
func createTestBundle() []byte {
	validYAML := `pki:
  ca: /etc/nebula/ca.crt
  cert: /etc/nebula/host.crt
  key: /etc/nebula/host.key
`

	files := map[string]string{
		bundle.RequiredFileConfig:   validYAML,
		bundle.RequiredFileCACert:   "-----BEGIN CERTIFICATE-----\nca cert\n-----END CERTIFICATE-----",
		bundle.RequiredFileCRL:      "-----BEGIN X509 CRL-----\ncrl\n-----END X509 CRL-----",
		bundle.RequiredFileHostCert: "-----BEGIN CERTIFICATE-----\nhost cert\n-----END CERTIFICATE-----",
		bundle.RequiredFileHostKey:  "-----BEGIN NEBULA PRIVATE KEY-----\nkey\n-----END NEBULA PRIVATE KEY-----",
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			panic(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			panic(err)
		}
	}

	tw.Close()
	gzw.Close()

	return buf.Bytes()
}

// setupTestDB creates an in-memory database for testing.
func setupBundleTestDB(t *testing.T) *sql.DB {
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
		created_at INTEGER NOT NULL,
		UNIQUE(tenant_id, name)
	);

	CREATE TABLE config_bundles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
		version INTEGER NOT NULL,
		data BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		UNIQUE(cluster_id, version)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert test tenant and cluster
	_, err = db.Exec(`
		INSERT INTO tenants (id, name, created_at) VALUES ('tenant1', 'Test Tenant', 1000000000);
		INSERT INTO clusters (id, tenant_id, name, config_version, cluster_token_hash, created_at)
		VALUES ('cluster1', 'tenant1', 'Test Cluster', 1, 'hash', 1000000000);
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	return db
}

func TestBundleService_UploadAndGetVersion(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)
	bundleData := createTestBundle()

	// Upload bundle
	version, err := service.Upload("cluster1", bundleData)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if version != 2 {
		t.Errorf("Expected version 2, got %d", version)
	}

	// Check current version
	currentVersion, err := service.GetCurrentVersion("cluster1")
	if err != nil {
		t.Fatalf("GetCurrentVersion failed: %v", err)
	}

	if currentVersion != 2 {
		t.Errorf("Expected current version 2, got %d", currentVersion)
	}
}

func TestBundleService_UploadInvalidBundle(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)

	// Upload invalid bundle (too large)
	largeData := make([]byte, bundle.MaxBundleSize+1)
	_, err := service.Upload("cluster1", largeData)

	if err != bundle.ErrBundleTooLarge {
		t.Errorf("Expected ErrBundleTooLarge, got %v", err)
	}
}

func TestBundleService_UploadMissingRequiredFile(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)

	// Create bundle missing required file
	files := map[string]string{
		bundle.RequiredFileConfig: "pki:\n  ca: /etc/nebula/ca.crt\n",
		bundle.RequiredFileCACert: "ca cert",
		// Missing other required files
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}

	tw.Close()
	gzw.Close()

	_, err := service.Upload("cluster1", buf.Bytes())

	if !errors.Is(err, bundle.ErrMissingRequiredFile) {
		t.Errorf("Expected ErrMissingRequiredFile, got %v", err)
	}
}

func TestBundleService_Download(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)
	bundleData := createTestBundle()

	// Upload bundle
	version, err := service.Upload("cluster1", bundleData)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Download latest bundle
	downloadedData, downloadedVersion, err := service.Download("cluster1", 0)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if downloadedVersion != version {
		t.Errorf("Expected version %d, got %d", version, downloadedVersion)
	}

	if !bytes.Equal(downloadedData, bundleData) {
		t.Error("Downloaded data doesn't match uploaded data")
	}
}

func TestBundleService_DownloadSpecificVersion(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)
	bundle1 := createTestBundle()
	bundle2 := createTestBundle()

	// Upload two versions
	v1, _ := service.Upload("cluster1", bundle1)
	v2, _ := service.Upload("cluster1", bundle2)

	// Download version 1
	data, version, err := service.Download("cluster1", v1)
	if err != nil {
		t.Fatalf("Download v1 failed: %v", err)
	}

	if version != v1 {
		t.Errorf("Expected version %d, got %d", v1, version)
	}

	if !bytes.Equal(data, bundle1) {
		t.Error("Downloaded v1 data doesn't match")
	}

	// Download version 2
	data, version, err = service.Download("cluster1", v2)
	if err != nil {
		t.Fatalf("Download v2 failed: %v", err)
	}

	if version != v2 {
		t.Errorf("Expected version %d, got %d", v2, version)
	}
}

func TestBundleService_CheckVersion(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)
	bundleData := createTestBundle()

	// Upload bundle (will be version 2)
	_, err := service.Upload("cluster1", bundleData)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Check if client version 1 is current (should be false)
	isCurrent, currentVersion, err := service.CheckVersion("cluster1", 1)
	if err != nil {
		t.Fatalf("CheckVersion failed: %v", err)
	}

	if isCurrent {
		t.Error("Expected version 1 to not be current")
	}

	if currentVersion != 2 {
		t.Errorf("Expected current version 2, got %d", currentVersion)
	}

	// Check if client version 2 is current (should be true)
	isCurrent, currentVersion, err = service.CheckVersion("cluster1", 2)
	if err != nil {
		t.Fatalf("CheckVersion failed: %v", err)
	}

	if !isCurrent {
		t.Error("Expected version 2 to be current")
	}

	if currentVersion != 2 {
		t.Errorf("Expected current version 2, got %d", currentVersion)
	}
}

func TestBundleService_MultipleUploads(t *testing.T) {
	db := setupBundleTestDB(t)
	defer db.Close()

	logger := zap.NewNop()
	service := NewBundleService(db, logger)
	bundleData := createTestBundle()

	// Upload multiple bundles
	for i := 2; i <= 5; i++ {
		version, err := service.Upload("cluster1", bundleData)
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}

		if version != int64(i) {
			t.Errorf("Expected version %d, got %d", i, version)
		}
	}

	// Check final version
	currentVersion, err := service.GetCurrentVersion("cluster1")
	if err != nil {
		t.Fatalf("GetCurrentVersion failed: %v", err)
	}

	if currentVersion != 5 {
		t.Errorf("Expected final version 5, got %d", currentVersion)
	}
}
