package lighthouse

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateConfig(t *testing.T) {
	clusterConfig := &ClusterConfig{
		ClusterID:      "test-cluster-123",
		ClusterName:    "Test Cluster",
		LighthousePort: 4242,
		ConfigVersion:  1,
	}

	basePath := "/var/lib/nebulagc/lighthouse"
	config := GenerateConfig(clusterConfig, basePath)

	// Verify basic structure
	if config.Lighthouse.AmLighthouse != true {
		t.Error("Expected am_lighthouse to be true")
	}

	if config.Listen.Port != 4242 {
		t.Errorf("Expected port 4242, got %d", config.Listen.Port)
	}

	if config.Tun.MTU != 1300 {
		t.Errorf("Expected MTU 1300, got %d", config.Tun.MTU)
	}

	// Verify PKI paths
	expectedCAPath := filepath.Join(basePath, clusterConfig.ClusterID, "ca.crt")
	if config.PKI.CA != expectedCAPath {
		t.Errorf("Expected CA path %s, got %s", expectedCAPath, config.PKI.CA)
	}
}

func TestGenerateConfigYAMLOutput(t *testing.T) {
	clusterConfig := &ClusterConfig{
		ClusterID:      "test-cluster-123",
		ClusterName:    "Test Cluster",
		LighthousePort: 4242,
		ConfigVersion:  1,
	}

	basePath := "/var/lib/nebulagc/lighthouse"
	config := GenerateConfig(clusterConfig, basePath)

	// Marshal to YAML to verify it's valid
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config to YAML: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty YAML output")
	}

	// Verify we can unmarshal it back
	var parsed NebulaConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if parsed.Lighthouse.AmLighthouse != true {
		t.Error("Parsed config lost am_lighthouse value")
	}
}

func TestWriteConfigFiles(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	clusterConfig := &ClusterConfig{
		ClusterID:      "test-cluster-123",
		ClusterName:    "Test Cluster",
		CACert:         "-----BEGIN CERTIFICATE-----\nCA CERT\n-----END CERTIFICATE-----",
		CRL:            "-----BEGIN X509 CRL-----\nCRL\n-----END X509 CRL-----",
		HostCert:       "-----BEGIN CERTIFICATE-----\nHOST CERT\n-----END CERTIFICATE-----",
		HostKey:        "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----",
		LighthousePort: 4242,
		ConfigVersion:  1,
	}

	configPath, err := WriteConfigFiles(clusterConfig, tmpDir)
	if err != nil {
		t.Fatalf("WriteConfigFiles failed: %v", err)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify PKI files were created
	clusterDir := filepath.Join(tmpDir, clusterConfig.ClusterID)
	expectedFiles := []string{"ca.crt", "crl.pem", "host.crt", "host.key", "config.yml"}

	for _, filename := range expectedFiles {
		path := filepath.Join(clusterDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", filename)
		}
	}

	// Verify file contents
	caCert, err := os.ReadFile(filepath.Join(clusterDir, "ca.crt"))
	if err != nil {
		t.Fatalf("Failed to read ca.crt: %v", err)
	}

	if string(caCert) != clusterConfig.CACert {
		t.Error("CA cert content does not match")
	}

	// Verify config.yml is valid YAML
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config.yml: %v", err)
	}

	var parsedConfig NebulaConfig
	if err := yaml.Unmarshal(configData, &parsedConfig); err != nil {
		t.Fatalf("Config.yml contains invalid YAML: %v", err)
	}

	if parsedConfig.Listen.Port != 4242 {
		t.Errorf("Config port mismatch: expected 4242, got %d", parsedConfig.Listen.Port)
	}
}

func TestRemoveConfigFiles(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	clusterConfig := &ClusterConfig{
		ClusterID:      "test-cluster-456",
		ClusterName:    "Test Cluster",
		CACert:         "CA CERT",
		CRL:            "CRL",
		HostCert:       "HOST CERT",
		HostKey:        "KEY",
		LighthousePort: 4242,
		ConfigVersion:  1,
	}

	// Write files
	_, err = WriteConfigFiles(clusterConfig, tmpDir)
	if err != nil {
		t.Fatalf("WriteConfigFiles failed: %v", err)
	}

	// Verify directory exists
	clusterDir := filepath.Join(tmpDir, clusterConfig.ClusterID)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		t.Fatal("Cluster directory was not created")
	}

	// Remove files
	if err := RemoveConfigFiles(clusterConfig.ClusterID, tmpDir); err != nil {
		t.Fatalf("RemoveConfigFiles failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		t.Error("Cluster directory still exists after removal")
	}
}

func TestConfigFilePermissions(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "lighthouse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	clusterConfig := &ClusterConfig{
		ClusterID:      "test-cluster-789",
		ClusterName:    "Test Cluster",
		CACert:         "CA CERT",
		CRL:            "CRL",
		HostCert:       "HOST CERT",
		HostKey:        "KEY",
		LighthousePort: 4242,
		ConfigVersion:  1,
	}

	_, err = WriteConfigFiles(clusterConfig, tmpDir)
	if err != nil {
		t.Fatalf("WriteConfigFiles failed: %v", err)
	}

	// Check directory permissions (should be 0700)
	clusterDir := filepath.Join(tmpDir, clusterConfig.ClusterID)
	dirInfo, err := os.Stat(clusterDir)
	if err != nil {
		t.Fatalf("Failed to stat cluster directory: %v", err)
	}

	expectedDirPerm := os.FileMode(0700)
	if dirInfo.Mode().Perm() != expectedDirPerm {
		t.Errorf("Expected directory permissions %o, got %o", expectedDirPerm, dirInfo.Mode().Perm())
	}

	// Check file permissions (should be 0600)
	keyPath := filepath.Join(clusterDir, "host.key")
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat host.key: %v", err)
	}

	expectedFilePerm := os.FileMode(0600)
	if keyInfo.Mode().Perm() != expectedFilePerm {
		t.Errorf("Expected file permissions %o, got %o", expectedFilePerm, keyInfo.Mode().Perm())
	}
}
