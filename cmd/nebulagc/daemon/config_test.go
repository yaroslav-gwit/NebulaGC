package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  DaemonConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: DaemonConfig{
				ControlPlaneURLs: []string{"https://control1.example.com"},
				Clusters: []ClusterConfig{
					{
						Name:      "test-cluster",
						TenantID:  "12345678-1234-1234-1234-123456789012",
						ClusterID: "87654321-4321-4321-4321-210987654321",
						NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "12345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with cluster token",
			config: DaemonConfig{
				ControlPlaneURLs: []string{"https://control1.example.com"},
				Clusters: []ClusterConfig{
					{
						Name:         "test-cluster",
						TenantID:     "12345678-1234-1234-1234-123456789012",
						ClusterID:    "87654321-4321-4321-4321-210987654321",
						NodeID:       "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken:    "12345678901234567890123456789012345678901",
						ClusterToken: "98765432109876543210987654321098765432109",
						ConfigDir:    "/etc/nebula/test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple clusters",
			config: DaemonConfig{
				ControlPlaneURLs: []string{"https://control1.example.com", "https://control2.example.com"},
				Clusters: []ClusterConfig{
					{
						Name:      "cluster-1",
						TenantID:  "12345678-1234-1234-1234-123456789012",
						ClusterID: "87654321-4321-4321-4321-210987654321",
						NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "12345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/cluster1",
					},
					{
						Name:      "cluster-2",
						TenantID:  "22345678-1234-1234-1234-123456789012",
						ClusterID: "97654321-4321-4321-4321-210987654321",
						NodeID:    "bbcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "22345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/cluster2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing control plane URLs",
			config: DaemonConfig{
				ControlPlaneURLs: []string{},
				Clusters: []ClusterConfig{
					{
						Name:      "test-cluster",
						TenantID:  "12345678-1234-1234-1234-123456789012",
						ClusterID: "87654321-4321-4321-4321-210987654321",
						NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "12345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty control plane URL",
			config: DaemonConfig{
				ControlPlaneURLs: []string{""},
				Clusters: []ClusterConfig{
					{
						Name:      "test-cluster",
						TenantID:  "12345678-1234-1234-1234-123456789012",
						ClusterID: "87654321-4321-4321-4321-210987654321",
						NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "12345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid URL format",
			config: DaemonConfig{
				ControlPlaneURLs: []string{"not-a-url"},
				Clusters: []ClusterConfig{
					{
						Name:      "test-cluster",
						TenantID:  "12345678-1234-1234-1234-123456789012",
						ClusterID: "87654321-4321-4321-4321-210987654321",
						NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
						NodeToken: "12345678901234567890123456789012345678901",
						ConfigDir: "/etc/nebula/test",
					},
				},
			},
			wantErr: false, // url.Parse accepts relative URLs
		},
		{
			name: "missing clusters",
			config: DaemonConfig{
				ControlPlaneURLs: []string{"https://control1.example.com"},
				Clusters:         []ClusterConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DaemonConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClusterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ClusterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: ClusterConfig{
				Name:      "",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "invalid tenant ID",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "not-a-uuid",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "invalid cluster ID",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "invalid",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "invalid node ID",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "12345678",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "short node token",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "short",
				ConfigDir: "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "short cluster token",
			config: ClusterConfig{
				Name:         "test-cluster",
				TenantID:     "12345678-1234-1234-1234-123456789012",
				ClusterID:    "87654321-4321-4321-4321-210987654321",
				NodeID:       "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken:    "12345678901234567890123456789012345678901",
				ClusterToken: "short",
				ConfigDir:    "/etc/nebula/test",
			},
			wantErr: true,
		},
		{
			name: "missing config dir",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "",
			},
			wantErr: true,
		},
		{
			name: "relative config dir path",
			config: ClusterConfig{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "relative/path",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temporary directory for test configs
	tempDir := t.TempDir()

	// Valid config
	validConfig := DaemonConfig{
		ControlPlaneURLs: []string{"https://control1.example.com"},
		Clusters: []ClusterConfig{
			{
				Name:      "test-cluster",
				TenantID:  "12345678-1234-1234-1234-123456789012",
				ClusterID: "87654321-4321-4321-4321-210987654321",
				NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
				NodeToken: "12345678901234567890123456789012345678901",
				ConfigDir: "/etc/nebula/test",
			},
		},
	}

	// Write valid config to file
	validConfigPath := filepath.Join(tempDir, "valid.json")
	validData, _ := json.MarshalIndent(validConfig, "", "  ")
	if err := os.WriteFile(validConfigPath, validData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading valid config
	t.Run("load valid config", func(t *testing.T) {
		config, err := LoadConfigFromPath(validConfigPath)
		if err != nil {
			t.Errorf("LoadConfigFromPath() error = %v", err)
		}
		if config == nil {
			t.Error("LoadConfigFromPath() returned nil config")
		}
		if len(config.Clusters) != 1 {
			t.Errorf("Expected 1 cluster, got %d", len(config.Clusters))
		}
	})

	// Test loading non-existent file
	t.Run("load non-existent file", func(t *testing.T) {
		_, err := LoadConfigFromPath(filepath.Join(tempDir, "nonexistent.json"))
		if err == nil {
			t.Error("LoadConfigFromPath() expected error for non-existent file")
		}
	})

	// Test loading invalid JSON
	t.Run("load invalid JSON", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid.json")
		if err := os.WriteFile(invalidPath, []byte("{invalid json}"), 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		_, err := LoadConfigFromPath(invalidPath)
		if err == nil {
			t.Error("LoadConfigFromPath() expected error for invalid JSON")
		}
	})

	// Test loading config with validation errors
	t.Run("load config with validation errors", func(t *testing.T) {
		invalidConfig := DaemonConfig{
			ControlPlaneURLs: []string{}, // Empty URLs - validation error
			Clusters: []ClusterConfig{
				{
					Name:      "test-cluster",
					TenantID:  "12345678-1234-1234-1234-123456789012",
					ClusterID: "87654321-4321-4321-4321-210987654321",
					NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
					NodeToken: "12345678901234567890123456789012345678901",
					ConfigDir: "/etc/nebula/test",
				},
			},
		}

		invalidPath := filepath.Join(tempDir, "validation_error.json")
		invalidData, _ := json.MarshalIndent(invalidConfig, "", "  ")
		if err := os.WriteFile(invalidPath, invalidData, 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		_, err := LoadConfigFromPath(invalidPath)
		if err == nil {
			t.Error("LoadConfigFromPath() expected validation error")
		}
	})
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		uuid  string
		valid bool
	}{
		{"valid UUID", "12345678-1234-1234-1234-123456789012", true},
		{"valid UUID uppercase", "12345678-1234-1234-1234-123456789ABC", true},
		{"valid UUID lowercase", "abcdef12-3456-7890-abcd-ef1234567890", true},
		{"invalid - too short", "12345678-1234-1234-1234-12345678901", false},
		{"invalid - too long", "12345678-1234-1234-1234-1234567890123", false},
		{"invalid - wrong format", "1234567812341234123412345678901234", false},
		{"invalid - missing hyphens", "12345678123412341234123456789012", false},
		{"invalid - wrong positions", "123456789-1234-1234-1234-23456789012", false},
		{"empty string", "", false},
		{"not a UUID", "not-a-uuid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidUUID(tt.uuid); got != tt.valid {
				t.Errorf("isValidUUID(%s) = %v, want %v", tt.uuid, got, tt.valid)
			}
		})
	}
}
