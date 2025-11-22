package sdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ClientConfig{
				BaseURLs:  []string{"https://cp1.example.com"},
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing base URL",
			config: ClientConfig{
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error = %v", err)
				}
				if client == nil {
					t.Error("NewClient() returned nil client")
				}
			}
		})
	}
}

func TestClient_DiscoverMaster(t *testing.T) {
	// Create test servers
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer masterServer.Close()

	replicaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer replicaServer.Close()

	tests := []struct {
		name     string
		baseURLs []string
		wantErr  bool
	}{
		{
			name:     "master found - first URL",
			baseURLs: []string{masterServer.URL, replicaServer.URL},
			wantErr:  false,
		},
		{
			name:     "master found - second URL",
			baseURLs: []string{replicaServer.URL, masterServer.URL},
			wantErr:  false,
		},
		{
			name:     "no master found",
			baseURLs: []string{replicaServer.URL},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ClientConfig{
				BaseURLs:  tt.baseURLs,
				TenantID:  "tenant-123",
				ClusterID: "cluster-456",
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			err = client.DiscoverMaster(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DiscoverMaster() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("DiscoverMaster() unexpected error = %v", err)
				}

				// Verify master URL was cached
				masterURL := client.getMasterURL()
				if masterURL == "" {
					t.Error("Master URL not cached after successful discovery")
				}
			}
		})
	}
}

func TestClient_ClearMasterCache(t *testing.T) {
	client, err := NewClient(ClientConfig{
		BaseURLs:  []string{"https://cp1.example.com"},
		TenantID:  "tenant-123",
		ClusterID: "cluster-456",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Set master URL
	client.mu.Lock()
	client.masterURL = "https://cp1.example.com"
	client.mu.Unlock()

	// Verify it's set
	if client.getMasterURL() == "" {
		t.Error("Master URL should be set")
	}

	// Clear cache
	client.clearMasterCache()

	// Verify it's cleared
	if client.getMasterURL() != "" {
		t.Error("Master URL should be cleared")
	}
}

func TestClient_BuildURLList(t *testing.T) {
	client, err := NewClient(ClientConfig{
		BaseURLs:  []string{"https://cp1.example.com", "https://cp2.example.com", "https://cp3.example.com"},
		TenantID:  "tenant-123",
		ClusterID: "cluster-456",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name         string
		masterURL    string
		preferMaster bool
		wantFirst    string
		wantLen      int
	}{
		{
			name:         "no master cached, prefer master",
			masterURL:    "",
			preferMaster: true,
			wantFirst:    "https://cp1.example.com",
			wantLen:      3,
		},
		{
			name:         "no master cached, don't prefer master",
			masterURL:    "",
			preferMaster: false,
			wantFirst:    "https://cp1.example.com",
			wantLen:      3,
		},
		{
			name:         "master cached, prefer master",
			masterURL:    "https://cp2.example.com",
			preferMaster: true,
			wantFirst:    "https://cp2.example.com",
			wantLen:      3,
		},
		{
			name:         "master cached, don't prefer master",
			masterURL:    "https://cp2.example.com",
			preferMaster: false,
			wantFirst:    "https://cp1.example.com",
			wantLen:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set master URL if specified
			client.mu.Lock()
			client.masterURL = tt.masterURL
			client.mu.Unlock()

			urls := client.buildURLList(tt.preferMaster)

			if len(urls) != tt.wantLen {
				t.Errorf("buildURLList() returned %d URLs, want %d", len(urls), tt.wantLen)
			}

			if len(urls) > 0 && urls[0] != tt.wantFirst {
				t.Errorf("buildURLList() first URL = %s, want %s", urls[0], tt.wantFirst)
			}
		})
	}
}

func TestClient_DoRequest_Authentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for node token
		if nodeToken := r.Header.Get(HeaderNodeToken); nodeToken != "" {
			if nodeToken == "valid-node-token" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check for cluster token
		if clusterToken := r.Header.Get(HeaderClusterToken); clusterToken != "" {
			if clusterToken == "valid-cluster-token" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// No auth
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name         string
		nodeToken    string
		clusterToken string
		authType     AuthType
		wantErr      bool
	}{
		{
			name:      "valid node token",
			nodeToken: "valid-node-token",
			authType:  AuthTypeNode,
			wantErr:   false,
		},
		{
			name:         "valid cluster token",
			clusterToken: "valid-cluster-token",
			authType:     AuthTypeCluster,
			wantErr:      false,
		},
		{
			name:     "no authentication",
			authType: AuthTypeNone,
			wantErr:  false,
		},
		{
			name:      "invalid node token",
			nodeToken: "invalid-token",
			authType:  AuthTypeNode,
			wantErr:   true,
		},
		{
			name:     "missing node token",
			authType: AuthTypeNode,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeToken:     tt.nodeToken,
				ClusterToken:  tt.clusterToken,
				RetryAttempts: 0, // Disable retries for faster tests
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			resp, err := client.doRequest(ctx, http.MethodGet, "/test", nil, tt.authType, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("doRequest() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("doRequest() unexpected error = %v", err)
				}
				if resp == nil {
					t.Error("doRequest() returned nil response")
				} else {
					resp.Body.Close()
				}
			}
		})
	}
}

func TestClient_CalculateBackoff(t *testing.T) {
	client, err := NewClient(ClientConfig{
		BaseURLs:     []string{"https://cp1.example.com"},
		TenantID:     "tenant-123",
		ClusterID:    "cluster-456",
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name    string
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "first retry",
			attempt: 0,
			wantMin: 0,
			wantMax: 2 * time.Second, // 2^0 = 1 second * 2 for jitter
		},
		{
			name:    "second retry",
			attempt: 1,
			wantMin: 0,
			wantMax: 4 * time.Second, // 2^1 = 2 seconds * 2 for jitter
		},
		{
			name:    "third retry",
			attempt: 2,
			wantMin: 0,
			wantMax: 8 * time.Second, // 2^2 = 4 seconds * 2 for jitter
		},
		{
			name:    "capped at max",
			attempt: 10,
			wantMin: 0,
			wantMax: 10 * time.Second, // Capped at RetryWaitMax
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := client.calculateBackoff(tt.attempt)

			if backoff < tt.wantMin {
				t.Errorf("calculateBackoff() = %v, want >= %v", backoff, tt.wantMin)
			}
			if backoff > tt.wantMax {
				t.Errorf("calculateBackoff() = %v, want <= %v", backoff, tt.wantMax)
			}
		})
	}
}

// ============================================================================
// Node Management Methods Tests
// ============================================================================

func TestClient_CreateNode(t *testing.T) {
	tests := []struct {
		name         string
		nodeName     string
		isAdmin      bool
		mtu          int
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "successful creation",
			nodeName:     "test-node",
			isAdmin:      false,
			mtu:          1300,
			serverStatus: http.StatusOK,
			serverBody:   `{"node_id":"node-123","node_token":"token-abc123","nebula_ip":"10.0.0.5"}`,
			wantErr:      false,
		},
		{
			name:         "admin node creation",
			nodeName:     "admin-node",
			isAdmin:      true,
			mtu:          1300,
			serverStatus: http.StatusOK,
			serverBody:   `{"node_id":"node-456","node_token":"token-def456","nebula_ip":"10.0.0.6"}`,
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			nodeName:     "test-node",
			isAdmin:      false,
			mtu:          1300,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
		{
			name:         "invalid MTU",
			nodeName:     "test-node",
			isAdmin:      false,
			mtu:          100,
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"MTU must be between 576 and 9000"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method and path
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				// Verify cluster token header
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			credentials, err := client.CreateNode(ctx, tt.nodeName, tt.isAdmin, tt.mtu)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateNode() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CreateNode() unexpected error = %v", err)
				}
				if credentials == nil {
					t.Error("CreateNode() returned nil credentials")
				} else {
					if credentials.NodeID == "" {
						t.Error("NodeID is empty")
					}
					if credentials.NodeToken == "" {
						t.Error("NodeToken is empty")
					}
					if credentials.NebulaIP == "" {
						t.Error("NebulaIP is empty")
					}
				}
			}
		})
	}
}

func TestClient_DeleteNode(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "successful deletion",
			nodeID:       "node-123",
			serverStatus: http.StatusNoContent,
			serverBody:   "",
			wantErr:      false,
		},
		{
			name:         "node not found",
			nodeID:       "node-999",
			serverStatus: http.StatusNotFound,
			serverBody:   `{"error":"node not found"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			nodeID:       "node-123",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE request, got %s", r.Method)
				}

				// Verify cluster token header
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverBody != "" {
					w.Write([]byte(tt.serverBody))
				}
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			err = client.DeleteNode(ctx, tt.nodeID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteNode() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("DeleteNode() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestClient_ListNodes(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		serverStatus int
		serverBody   string
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "successful list with nodes",
			page:         1,
			pageSize:     10,
			serverStatus: http.StatusOK,
			serverBody:   `[{"id":"node-1","name":"node1","nebula_ip":"10.0.0.1","is_admin":true,"mtu":1300,"created_at":"2025-01-01T00:00:00Z"},{"id":"node-2","name":"node2","nebula_ip":"10.0.0.2","is_admin":false,"mtu":1300,"created_at":"2025-01-01T00:00:00Z"}]`,
			wantCount:    2,
			wantErr:      false,
		},
		{
			name:         "empty list",
			page:         1,
			pageSize:     10,
			serverStatus: http.StatusOK,
			serverBody:   `[]`,
			wantCount:    0,
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			page:         1,
			pageSize:     10,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				// Verify cluster token header
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}

				// Verify query parameters
				page := r.URL.Query().Get("page")
				pageSize := r.URL.Query().Get("page_size")
				if page == "" || pageSize == "" {
					t.Error("Page or page_size query parameter missing")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			nodes, err := client.ListNodes(ctx, tt.page, tt.pageSize)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ListNodes() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ListNodes() unexpected error = %v", err)
				}
				if len(nodes) != tt.wantCount {
					t.Errorf("ListNodes() returned %d nodes, want %d", len(nodes), tt.wantCount)
				}
			}
		})
	}
}

func TestClient_UpdateMTU(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		mtu          int
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "successful update",
			nodeID:       "node-123",
			mtu:          1400,
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"MTU updated successfully"}`,
			wantErr:      false,
		},
		{
			name:         "invalid MTU - too low",
			nodeID:       "node-123",
			mtu:          500,
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"MTU must be between 576 and 9000"}`,
			wantErr:      true,
		},
		{
			name:         "invalid MTU - too high",
			nodeID:       "node-123",
			mtu:          10000,
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"MTU must be between 576 and 9000"}`,
			wantErr:      true,
		},
		{
			name:         "node not found",
			nodeID:       "node-999",
			mtu:          1400,
			serverStatus: http.StatusNotFound,
			serverBody:   `{"error":"node not found"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			nodeID:       "node-123",
			mtu:          1400,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}

				// Verify cluster token header
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			err = client.UpdateMTU(ctx, tt.nodeID, tt.mtu)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateMTU() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("UpdateMTU() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestClient_RotateNodeToken(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		serverStatus int
		serverBody   string
		wantToken    string
		wantErr      bool
	}{
		{
			name:         "successful rotation",
			nodeID:       "node-123",
			serverStatus: http.StatusOK,
			serverBody:   `{"token":"new-token-abc123xyz","message":"Token rotated successfully"}`,
			wantToken:    "new-token-abc123xyz",
			wantErr:      false,
		},
		{
			name:         "node not found",
			nodeID:       "node-999",
			serverStatus: http.StatusNotFound,
			serverBody:   `{"error":"node not found"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			nodeID:       "node-123",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				// Verify cluster token header
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			token, err := client.RotateNodeToken(ctx, tt.nodeID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RotateNodeToken() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("RotateNodeToken() unexpected error = %v", err)
				}
				if token != tt.wantToken {
					t.Errorf("RotateNodeToken() returned token %s, want %s", token, tt.wantToken)
				}
			}
		})
	}
}

// ============================================================================
// Config Bundle Methods Tests
// ============================================================================

func TestClient_GetLatestVersion(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantVersion  int64
		wantErr      bool
	}{
		{
			name:         "successful version retrieval",
			serverStatus: http.StatusOK,
			serverBody:   `{"version":42}`,
			wantVersion:  42,
			wantErr:      false,
		},
		{
			name:         "version zero",
			serverStatus: http.StatusOK,
			serverBody:   `{"version":0}`,
			wantVersion:  0,
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid node token"}`,
			wantErr:      true,
		},
		{
			name:         "invalid response format",
			serverStatus: http.StatusOK,
			serverBody:   `{"invalid":"response"}`,
			wantVersion:  0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				// Verify node token header
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			version, err := client.GetLatestVersion(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetLatestVersion() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetLatestVersion() unexpected error = %v", err)
				}
				if version != tt.wantVersion {
					t.Errorf("GetLatestVersion() returned version %d, want %d", version, tt.wantVersion)
				}
			}
		})
	}
}

func TestClient_DownloadBundle(t *testing.T) {
	bundleData := []byte("mock-bundle-data-tar-gz")

	tests := []struct {
		name           string
		currentVersion int64
		serverStatus   int
		serverBody     []byte
		versionHeader  string
		wantData       []byte
		wantVersion    int64
		wantErr        bool
	}{
		{
			name:           "successful download - new version",
			currentVersion: 1,
			serverStatus:   http.StatusOK,
			serverBody:     bundleData,
			versionHeader:  "2",
			wantData:       bundleData,
			wantVersion:    2,
			wantErr:        false,
		},
		{
			name:           "304 not modified - no update",
			currentVersion: 5,
			serverStatus:   http.StatusNotModified,
			serverBody:     nil,
			versionHeader:  "",
			wantData:       nil,
			wantVersion:    5,
			wantErr:        false,
		},
		{
			name:           "unauthorized",
			currentVersion: 1,
			serverStatus:   http.StatusUnauthorized,
			serverBody:     []byte(`{"error":"invalid node token"}`),
			versionHeader:  "",
			wantErr:        true,
		},
		{
			name:           "rate limited",
			currentVersion: 1,
			serverStatus:   http.StatusTooManyRequests,
			serverBody:     []byte(`{"error":"rate limit exceeded"}`),
			versionHeader:  "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				// Verify node token header
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}

				// Verify current_version query parameter
				currentVersion := r.URL.Query().Get("current_version")
				if currentVersion == "" {
					t.Error("current_version query parameter missing")
				}

				// Set version header if provided
				if tt.versionHeader != "" {
					w.Header().Set("X-Config-Version", tt.versionHeader)
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverBody != nil {
					w.Write(tt.serverBody)
				}
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			data, version, err := client.DownloadBundle(ctx, tt.currentVersion)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DownloadBundle() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("DownloadBundle() unexpected error = %v", err)
				}
				if version != tt.wantVersion {
					t.Errorf("DownloadBundle() returned version %d, want %d", version, tt.wantVersion)
				}
				if tt.wantData == nil && data != nil {
					t.Errorf("DownloadBundle() returned data, want nil")
				}
				if tt.wantData != nil {
					if data == nil {
						t.Error("DownloadBundle() returned nil data, want bundle data")
					} else if string(data) != string(tt.wantData) {
						t.Errorf("DownloadBundle() returned wrong data")
					}
				}
			}
		})
	}
}

func TestClient_UploadBundle(t *testing.T) {
	bundleData := []byte("test-bundle-data")

	tests := []struct {
		name         string
		bundleData   []byte
		serverStatus int
		serverBody   string
		wantVersion  int64
		wantErr      bool
	}{
		{
			name:         "successful upload",
			bundleData:   bundleData,
			serverStatus: http.StatusCreated,
			serverBody:   `{"version":10}`,
			wantVersion:  10,
			wantErr:      false,
		},
		{
			name:         "successful upload - 200 OK",
			bundleData:   bundleData,
			serverStatus: http.StatusOK,
			serverBody:   `{"version":11}`,
			wantVersion:  11,
			wantErr:      false,
		},
		{
			name:         "unauthorized - not admin",
			bundleData:   bundleData,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"admin privileges required"}`,
			wantErr:      true,
		},
		{
			name:         "invalid bundle format",
			bundleData:   bundleData,
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"invalid bundle format"}`,
			wantErr:      true,
		},
		{
			name:         "rate limited",
			bundleData:   bundleData,
			serverStatus: http.StatusTooManyRequests,
			serverBody:   `{"error":"rate limit exceeded"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				// Verify node token header
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/octet-stream" {
					t.Errorf("Expected Content-Type application/octet-stream, got %s", r.Header.Get("Content-Type"))
				}

				// Read and verify body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
				}
				if string(body) != string(tt.bundleData) {
					t.Error("Request body doesn't match bundle data")
				}

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			ctx := context.Background()
			version, err := client.UploadBundle(ctx, tt.bundleData)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UploadBundle() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("UploadBundle() unexpected error = %v", err)
				}
				if version != tt.wantVersion {
					t.Errorf("UploadBundle() returned version %d, want %d", version, tt.wantVersion)
				}
			}
		})
	}
}

// ============================================================================
// Topology Management Methods Tests
// ============================================================================

func TestClient_RegisterRoutes(t *testing.T) {
	tests := []struct {
		name         string
		routes       []string
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "successful registration",
			routes:       []string{"10.100.0.0/24", "10.101.0.0/24"},
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"routes updated"}`,
			wantErr:      false,
		},
		{
			name:         "empty routes",
			routes:       []string{},
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"routes cleared"}`,
			wantErr:      false,
		},
		{
			name:         "invalid CIDR format",
			routes:       []string{"invalid-cidr"},
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"invalid CIDR format"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			routes:       []string{"10.100.0.0/24"},
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid node token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeID:        "node-789",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			err := client.RegisterRoutes(ctx, tt.routes)

			if tt.wantErr && err == nil {
				t.Error("RegisterRoutes() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("RegisterRoutes() unexpected error = %v", err)
			}
		})
	}
}

func TestClient_GetRoutes(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantRoutes   []string
		wantErr      bool
	}{
		{
			name:         "successful retrieval",
			serverStatus: http.StatusOK,
			serverBody:   `{"routes":["10.100.0.0/24","10.101.0.0/24"]}`,
			wantRoutes:   []string{"10.100.0.0/24", "10.101.0.0/24"},
			wantErr:      false,
		},
		{
			name:         "no routes",
			serverStatus: http.StatusOK,
			serverBody:   `{"routes":[]}`,
			wantRoutes:   []string{},
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid node token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeID:        "node-789",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			routes, err := client.GetRoutes(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetRoutes() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetRoutes() unexpected error = %v", err)
				}
				if len(routes) != len(tt.wantRoutes) {
					t.Errorf("GetRoutes() returned %d routes, want %d", len(routes), len(tt.wantRoutes))
				}
			}
		})
	}
}

func TestClient_ListClusterRoutes(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "successful listing",
			serverStatus: http.StatusOK,
			serverBody:   `[{"node_id":"node-1","routes":["10.100.0.0/24"]},{"node_id":"node-2","routes":["10.101.0.0/24"]}]`,
			wantCount:    2,
			wantErr:      false,
		},
		{
			name:         "no routes in cluster",
			serverStatus: http.StatusOK,
			serverBody:   `[]`,
			wantCount:    0,
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-cluster-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			routes, err := client.ListClusterRoutes(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("ListClusterRoutes() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ListClusterRoutes() unexpected error = %v", err)
				}
				if len(routes) != tt.wantCount {
					t.Errorf("ListClusterRoutes() returned %d routes, want %d", len(routes), tt.wantCount)
				}
			}
		})
	}
}

func TestClient_SetLighthouse(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		enabled      bool
		publicIP     string
		port         int
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "enable lighthouse",
			nodeID:       "node-123",
			enabled:      true,
			publicIP:     "203.0.113.10",
			port:         4242,
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"lighthouse enabled"}`,
			wantErr:      false,
		},
		{
			name:         "disable lighthouse",
			nodeID:       "node-123",
			enabled:      false,
			publicIP:     "",
			port:         0,
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"lighthouse disabled"}`,
			wantErr:      false,
		},
		{
			name:         "invalid public IP",
			nodeID:       "node-123",
			enabled:      true,
			publicIP:     "invalid-ip",
			port:         4242,
			serverStatus: http.StatusBadRequest,
			serverBody:   `{"error":"invalid public IP"}`,
			wantErr:      true,
		},
		{
			name:         "node not found",
			nodeID:       "node-999",
			enabled:      true,
			publicIP:     "203.0.113.10",
			port:         4242,
			serverStatus: http.StatusNotFound,
			serverBody:   `{"error":"node not found"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			nodeID:       "node-123",
			enabled:      true,
			publicIP:     "203.0.113.10",
			port:         4242,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-cluster-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			err := client.SetLighthouse(ctx, tt.nodeID, tt.enabled, tt.publicIP, tt.port)

			if tt.wantErr && err == nil {
				t.Error("SetLighthouse() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("SetLighthouse() unexpected error = %v", err)
			}
		})
	}
}

func TestClient_SetRelay(t *testing.T) {
	tests := []struct {
		name         string
		nodeID       string
		enabled      bool
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "enable relay",
			nodeID:       "node-123",
			enabled:      true,
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"relay enabled"}`,
			wantErr:      false,
		},
		{
			name:         "disable relay",
			nodeID:       "node-123",
			enabled:      false,
			serverStatus: http.StatusOK,
			serverBody:   `{"message":"relay disabled"}`,
			wantErr:      false,
		},
		{
			name:         "node not found",
			nodeID:       "node-999",
			enabled:      true,
			serverStatus: http.StatusNotFound,
			serverBody:   `{"error":"node not found"}`,
			wantErr:      true,
		},
		{
			name:         "unauthorized",
			nodeID:       "node-123",
			enabled:      true,
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-cluster-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			err := client.SetRelay(ctx, tt.nodeID, tt.enabled)

			if tt.wantErr && err == nil {
				t.Error("SetRelay() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("SetRelay() unexpected error = %v", err)
			}
		})
	}
}

func TestClient_GetTopology(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantErr      bool
	}{
		{
			name:         "successful retrieval",
			serverStatus: http.StatusOK,
			serverBody:   `{"lighthouses":[{"node_id":"node-1","name":"lighthouse-1","public_ip":"203.0.113.10","port":4242}],"relays":[{"node_id":"node-2","name":"relay-1"}],"routes":{"node-3":["10.100.0.0/24"]}}`,
			wantErr:      false,
		},
		{
			name:         "empty topology",
			serverStatus: http.StatusOK,
			serverBody:   `{"lighthouses":[],"relays":[],"routes":{}}`,
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid node token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get(HeaderNodeToken) == "" {
					t.Error("Node token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				NodeToken:     "valid-node-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			topology, err := client.GetTopology(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetTopology() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetTopology() unexpected error = %v", err)
				}
				if topology == nil {
					t.Error("GetTopology() returned nil topology")
				}
			}
		})
	}
}

func TestClient_RotateClusterToken(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantToken    string
		wantErr      bool
	}{
		{
			name:         "successful rotation",
			serverStatus: http.StatusOK,
			serverBody:   `{"token":"new-cluster-token-abc123xyz","message":"Token rotated"}`,
			wantToken:    "new-cluster-token-abc123xyz",
			wantErr:      false,
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			serverBody:   `{"error":"invalid cluster token"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.Header.Get(HeaderClusterToken) == "" {
					t.Error("Cluster token header missing")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				ClusterToken:  "valid-cluster-token",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			token, err := client.RotateClusterToken(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("RotateClusterToken() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("RotateClusterToken() unexpected error = %v", err)
				}
				if token != tt.wantToken {
					t.Errorf("RotateClusterToken() returned token %s, want %s", token, tt.wantToken)
				}
			}
		})
	}
}

func TestClient_CheckMaster(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		serverBody   string
		wantMaster   bool
		wantErr      bool
	}{
		{
			name:         "instance is master",
			serverStatus: http.StatusOK,
			serverBody:   `{"is_master":true,"instance_id":"instance-123"}`,
			wantMaster:   true,
			wantErr:      false,
		},
		{
			name:         "instance is not master",
			serverStatus: http.StatusOK,
			serverBody:   `{"is_master":false,"instance_id":"instance-456","master_url":"https://master.example.com"}`,
			wantMaster:   false,
			wantErr:      false,
		},
		{
			name:         "service unavailable",
			serverStatus: http.StatusServiceUnavailable,
			serverBody:   `{"error":"unable to determine master status"}`,
			wantMaster:   false,
			wantErr:      true,
		},
		{
			name:         "invalid JSON response",
			serverStatus: http.StatusOK,
			serverBody:   `{invalid json}`,
			wantMaster:   false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.URL.Path != "/health/master" {
					t.Errorf("Expected path /health/master, got %s", r.URL.Path)
				}
				// Health check should not require authentication
				if r.Header.Get(HeaderNodeToken) != "" || r.Header.Get(HeaderClusterToken) != "" {
					t.Error("CheckMaster should not send authentication headers")
				}
				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:      []string{server.URL},
				TenantID:      "tenant-123",
				ClusterID:     "cluster-456",
				RetryAttempts: 0,
			})

			ctx := context.Background()
			isMaster, err := client.CheckMaster(ctx, server.URL)

			if tt.wantErr {
				if err == nil {
					t.Error("CheckMaster() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CheckMaster() unexpected error = %v", err)
				}
				if isMaster != tt.wantMaster {
					t.Errorf("CheckMaster() returned %v, want %v", isMaster, tt.wantMaster)
				}
			}
		})
	}

	// Test connection failure
	t.Run("connection failure", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{
			BaseURLs:      []string{"https://unreachable.example.com"},
			TenantID:      "tenant-123",
			ClusterID:     "cluster-456",
			RetryAttempts: 0,
		})

		ctx := context.Background()
		_, err := client.CheckMaster(ctx, "https://unreachable.example.com")

		if err == nil {
			t.Error("CheckMaster() expected error for unreachable host but got nil")
		}
	})
}

func TestClient_GetClusterReplicas(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantCount  int
		wantErr    bool
	}{
		{
			name: "successful request with two replicas",
			response: `{
				"replicas": [
					{
						"instance_id": "replica-1",
						"url": "https://cp1.example.com",
						"is_master": true,
						"last_heartbeat": "2025-01-26T10:00:00Z"
					},
					{
						"instance_id": "replica-2",
						"url": "https://cp2.example.com",
						"is_master": false,
						"last_heartbeat": "2025-01-26T10:00:05Z"
					}
				]
			}`,
			statusCode: http.StatusOK,
			wantCount:  2,
			wantErr:    false,
		},
		{
			name: "empty replica list",
			response: `{
				"replicas": []
			}`,
			statusCode: http.StatusOK,
			wantCount:  0,
			wantErr:    false,
		},
		{
			name:       "unauthorized",
			response:   `{"error": "unauthorized"}`,
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "server error",
			response:   `{"error": "internal server error"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				expectedPath := "/api/v1/tenants/tenant-123/clusters/cluster-456/replicas"
				if r.URL.Path != expectedPath {
					t.Errorf("Request path = %s, want %s", r.URL.Path, expectedPath)
				}

				// Verify method
				if r.Method != http.MethodGet {
					t.Errorf("Request method = %s, want %s", r.Method, http.MethodGet)
				}

				// Verify authentication header
				if r.Header.Get("X-NebulaGC-Cluster-Token") == "" {
					t.Error("Expected X-NebulaGC-Cluster-Token header to be present")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client, _ := NewClient(ClientConfig{
				BaseURLs:     []string{server.URL},
				TenantID:     "tenant-123",
				ClusterID:    "cluster-456",
				ClusterToken: "test-cluster-token",
			})

			ctx := context.Background()
			replicas, err := client.GetClusterReplicas(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetClusterReplicas() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetClusterReplicas() unexpected error = %v", err)
				}
				if len(replicas) != tt.wantCount {
					t.Errorf("GetClusterReplicas() returned %d replicas, want %d", len(replicas), tt.wantCount)
				}
				if tt.wantCount > 0 {
					// Verify first replica
					if replicas[0].InstanceID == "" {
						t.Error("First replica has empty InstanceID")
					}
					if replicas[0].URL == "" {
						t.Error("First replica has empty URL")
					}
				}
			}
		})
	}
}
