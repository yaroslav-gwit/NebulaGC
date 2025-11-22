package sdk

import (
	"strings"
	"testing"
	"time"
)

func TestClientConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com:8080"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
				NodeID:    "node-789",
				NodeToken: "token-abc",
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple base URLs",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com", "https://cp2.example.com"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: false,
		},
		{
			name: "missing base URLs",
			config: ClientConfig{
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: true,
			errMsg:  "at least one base URL is required",
		},
		{
			name: "empty base URL",
			config: ClientConfig{
				BaseURLs:  []string{""},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: true,
			errMsg:  "base URL at index 0 is empty",
		},
		{
			name: "invalid URL format",
			config: ClientConfig{
				BaseURLs:  []string{"invalid-url"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: true,
			errMsg:  "base URL must start with http:// or https://",
		},
		{
			name: "missing tenant ID",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com"},
				ClusterID: "cluster-456",
			},
			wantErr: true,
			errMsg:  "tenant_id is required",
		},
		{
			name: "missing cluster ID",
			config: ClientConfig{
				BaseURLs: []string{"https://cp1.example.com"},
				TenantID: "tenant-123",
			},
			wantErr: true,
			errMsg:  "cluster_id is required",
		},
		{
			name: "trailing slash removed from URLs",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com/"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}

				// Verify defaults were set
				if tt.config.RetryAttempts == 0 {
					t.Errorf("RetryAttempts not set to default")
				}
				if tt.config.RetryWaitMin == 0 {
					t.Errorf("RetryWaitMin not set to default")
				}
				if tt.config.RetryWaitMax == 0 {
					t.Errorf("RetryWaitMax not set to default")
				}
				if tt.config.Timeout == 0 {
					t.Errorf("Timeout not set to default")
				}
				if tt.config.HTTPClient == nil {
					t.Errorf("HTTPClient not created")
				}

				// Verify URL normalization
				for _, url := range tt.config.BaseURLs {
					if strings.HasSuffix(url, "/") {
						t.Errorf("Base URL still has trailing slash: %s", url)
					}
				}
			}
		})
	}
}

func TestClientConfig_HasNodeAuth(t *testing.T) {
	tests := []struct {
		name      string
		nodeToken string
		want      bool
	}{
		{
			name:      "has node token",
			nodeToken: "node-token-123",
			want:      true,
		},
		{
			name:      "empty node token",
			nodeToken: "",
			want:      false,
		},
		{
			name:      "whitespace only token",
			nodeToken: "   ",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				NodeToken: tt.nodeToken,
			}
			if got := config.HasNodeAuth(); got != tt.want {
				t.Errorf("HasNodeAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientConfig_HasClusterAuth(t *testing.T) {
	tests := []struct {
		name         string
		clusterToken string
		want         bool
	}{
		{
			name:         "has cluster token",
			clusterToken: "cluster-token-123",
			want:         true,
		},
		{
			name:         "empty cluster token",
			clusterToken: "",
			want:         false,
		},
		{
			name:         "whitespace only token",
			clusterToken: "   ",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				ClusterToken: tt.clusterToken,
			}
			if got := config.HasClusterAuth(); got != tt.want {
				t.Errorf("HasClusterAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientConfig_Defaults(t *testing.T) {
	config := ClientConfig{
		BaseURLs:  []string{"https://cp1.example.com"},
		TenantID:  "tenant-123",
		ClusterID: "cluster-456",
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error = %v", err)
	}

	// Check defaults
	if config.RetryAttempts != 3 {
		t.Errorf("RetryAttempts = %d, want 3", config.RetryAttempts)
	}
	if config.RetryWaitMin != 1*time.Second {
		t.Errorf("RetryWaitMin = %v, want 1s", config.RetryWaitMin)
	}
	if config.RetryWaitMax != 30*time.Second {
		t.Errorf("RetryWaitMax = %v, want 30s", config.RetryWaitMax)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
	if config.HTTPClient == nil {
		t.Error("HTTPClient should be created")
	}
}
