package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yaroslav/nebulagc/sdk"
	"go.uber.org/zap"
)

func TestHealthChecker_Start_Stop(t *testing.T) {
	var serverURL string

	// Create test server that responds as master
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/tenants/tenant-1/clusters/cluster-1/replicas" {
			resp := struct {
				Replicas []sdk.ReplicaInfo `json:"replicas"`
			}{
				Replicas: []sdk.ReplicaInfo{
					{
						InstanceID:    "replica-1",
						URL:           serverURL,
						IsMaster:      true,
						LastHeartbeat: time.Now(),
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	serverURL = server.URL

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hc.Start(ctx)

	// Wait briefly for initial health check
	time.Sleep(100 * time.Millisecond)

	// Check that it's not degraded
	if hc.IsDegraded() {
		t.Error("Health checker should not be degraded with healthy master")
	}

	// Stop health checker
	hc.Stop()
}

func TestHealthChecker_DegradedMode(t *testing.T) {
	// Create test server that fails master discovery
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return errors
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hc.Start(ctx)

	// Wait for health check to complete
	time.Sleep(200 * time.Millisecond)

	// Should be in degraded mode
	if !hc.IsDegraded() {
		t.Error("Health checker should be degraded when master unreachable")
	}

	// Stop health checker
	hc.Stop()
}

func TestHealthChecker_Recovery(t *testing.T) {
	failMasterDiscovery := true
	var serverURL string

	// Create test server with controllable failures
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			if failMasterDiscovery {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/tenants/tenant-1/clusters/cluster-1/replicas" {
			if failMasterDiscovery {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			resp := struct {
				Replicas []sdk.ReplicaInfo `json:"replicas"`
			}{
				Replicas: []sdk.ReplicaInfo{
					{
						InstanceID:    "replica-1",
						URL:           serverURL,
						IsMaster:      true,
						LastHeartbeat: time.Now(),
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	serverURL = server.URL

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Start health checker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hc.Start(ctx)

	// Wait for initial health check showing degraded
	time.Sleep(200 * time.Millisecond)

	if !hc.IsDegraded() {
		t.Error("Health checker should be degraded initially")
	}

	// Fix the server
	failMasterDiscovery = false

	// Wait for next health check cycle
	// The initial check happens immediately, then uses HealthCheckInterval
	// For testing, we perform a manual health check
	hc.performHealthCheck(ctx)

	// Should recover from degraded mode
	if hc.IsDegraded() {
		t.Error("Health checker should recover when master becomes available")
	}

	// Stop health checker
	hc.Stop()
}

func TestHealthChecker_GetHealthStatus(t *testing.T) {
	var serverURL string

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/tenants/tenant-1/clusters/cluster-1/replicas" {
			resp := struct {
				Replicas []sdk.ReplicaInfo `json:"replicas"`
			}{
				Replicas: []sdk.ReplicaInfo{
					{
						InstanceID:    "replica-1",
						URL:           serverURL,
						IsMaster:      true,
						LastHeartbeat: time.Now(),
					},
					{
						InstanceID:    "replica-2",
						URL:           "https://cp2.example.com",
						IsMaster:      false,
						LastHeartbeat: time.Now(),
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	serverURL = server.URL

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Perform health check
	ctx := context.Background()
	hc.performHealthCheck(ctx)

	// Check status
	healthy, total, lastCheck := hc.GetHealthStatus()

	if healthy != 2 {
		t.Errorf("Expected 2 healthy replicas, got %d", healthy)
	}

	if total != 2 {
		t.Errorf("Expected 2 total replicas, got %d", total)
	}

	if lastCheck.IsZero() {
		t.Error("Last check time should not be zero")
	}

	if time.Since(lastCheck) > time.Second {
		t.Error("Last check should be recent")
	}
}

func TestHealthChecker_RefreshReplicas(t *testing.T) {
	var serverURL string

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/tenants/tenant-1/clusters/cluster-1/replicas" {
			resp := struct {
				Replicas []sdk.ReplicaInfo `json:"replicas"`
			}{
				Replicas: []sdk.ReplicaInfo{
					{
						InstanceID:    "replica-1",
						URL:           serverURL,
						IsMaster:      true,
						LastHeartbeat: time.Now(),
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	serverURL = server.URL

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Refresh replicas
	ctx := context.Background()
	err = hc.RefreshReplicas(ctx)

	if err != nil {
		t.Errorf("RefreshReplicas() unexpected error = %v", err)
	}
}

func TestHealthChecker_NoHealthyReplicas(t *testing.T) {
	var serverURL string

	// Create test server with stale replicas
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/check-master" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/tenants/tenant-1/clusters/cluster-1/replicas" {
			// Return replicas with old heartbeats (stale)
			oldTime := time.Now().Add(-10 * time.Minute)
			resp := struct {
				Replicas []sdk.ReplicaInfo `json:"replicas"`
			}{
				Replicas: []sdk.ReplicaInfo{
					{
						InstanceID:    "replica-1",
						URL:           serverURL,
						IsMaster:      true,
						LastHeartbeat: oldTime,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	serverURL = server.URL

	// Create client
	client, err := sdk.NewClient(sdk.ClientConfig{
		BaseURLs:      []string{server.URL},
		TenantID:      "tenant-1",
		ClusterID:     "cluster-1",
		ClusterToken:  "test-token",
		RetryAttempts: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create health checker
	logger := zap.NewNop()
	hc := NewHealthChecker(client, logger)

	// Perform health check
	ctx := context.Background()
	hc.performHealthCheck(ctx)

	// Should be degraded due to stale replicas
	if !hc.IsDegraded() {
		t.Error("Health checker should be degraded with stale replicas")
	}

	// Check status shows 0 healthy replicas
	healthy, total, _ := hc.GetHealthStatus()
	if healthy != 0 {
		t.Errorf("Expected 0 healthy replicas, got %d", healthy)
	}
	if total != 1 {
		t.Errorf("Expected 1 total replica, got %d", total)
	}
}
