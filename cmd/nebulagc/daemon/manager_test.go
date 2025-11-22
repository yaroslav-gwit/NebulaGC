package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	// Create temporary directory for test config
	tempDir := t.TempDir()

	// Create valid config
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

	// Write config to file
	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.MarshalIndent(validConfig, "", "  ")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test creating manager
	t.Run("valid config", func(t *testing.T) {
		logger := zap.NewNop()
		manager, err := NewManager(ManagerConfig{
			ConfigPath:      configPath,
			Logger:          logger,
			ShutdownTimeout: 5 * time.Second,
		})

		if err != nil {
			t.Errorf("NewManager() error = %v", err)
		}
		if manager == nil {
			t.Error("NewManager() returned nil manager")
		}
		if len(manager.clusters) != 1 {
			t.Errorf("Expected 1 cluster, got %d", len(manager.clusters))
		}
		if manager.shutdownTimeout != 5*time.Second {
			t.Errorf("Expected shutdown timeout 5s, got %v", manager.shutdownTimeout)
		}
	})

	// Test with multiple clusters
	t.Run("multiple clusters", func(t *testing.T) {
		multiConfig := DaemonConfig{
			ControlPlaneURLs: []string{"https://control1.example.com"},
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
		}

		multiConfigPath := filepath.Join(tempDir, "multi_config.json")
		multiConfigData, _ := json.MarshalIndent(multiConfig, "", "  ")
		if err := os.WriteFile(multiConfigPath, multiConfigData, 0644); err != nil {
			t.Fatalf("Failed to write multi config: %v", err)
		}

		logger := zap.NewNop()
		manager, err := NewManager(ManagerConfig{
			ConfigPath: multiConfigPath,
			Logger:     logger,
		})

		if err != nil {
			t.Errorf("NewManager() error = %v", err)
		}
		if len(manager.clusters) != 2 {
			t.Errorf("Expected 2 clusters, got %d", len(manager.clusters))
		}
	})

	// Test with invalid config
	t.Run("invalid config", func(t *testing.T) {
		invalidConfigPath := filepath.Join(tempDir, "invalid.json")
		if err := os.WriteFile(invalidConfigPath, []byte("{invalid}"), 0644); err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		logger := zap.NewNop()
		_, err := NewManager(ManagerConfig{
			ConfigPath: invalidConfigPath,
			Logger:     logger,
		})

		if err == nil {
			t.Error("NewManager() expected error for invalid config")
		}
	})

	// Test default shutdown timeout
	t.Run("default shutdown timeout", func(t *testing.T) {
		logger := zap.NewNop()
		manager, err := NewManager(ManagerConfig{
			ConfigPath: configPath,
			Logger:     logger,
		})

		if err != nil {
			t.Errorf("NewManager() error = %v", err)
		}
		if manager.shutdownTimeout != 30*time.Second {
			t.Errorf("Expected default timeout 30s, got %v", manager.shutdownTimeout)
		}
	})
}

func TestManager_Shutdown(t *testing.T) {
	// Create temporary directory for test config
	tempDir := t.TempDir()

	// Create valid config
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

	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.MarshalIndent(validConfig, "", "  ")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Run("graceful shutdown", func(t *testing.T) {
		logger := zap.NewNop()
		manager, err := NewManager(ManagerConfig{
			ConfigPath:      configPath,
			Logger:          logger,
			ShutdownTimeout: 2 * time.Second,
		})

		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		// Start cluster managers
		ctx, cancel := context.WithCancel(context.Background())
		manager.cancel = cancel

		for _, clusterMgr := range manager.clusters {
			manager.wg.Add(1)
			go func(mgr *ClusterManager) {
				defer manager.wg.Done()
				mgr.Run(ctx)
			}(clusterMgr)
		}

		// Give them a moment to start
		time.Sleep(100 * time.Millisecond)

		// Trigger shutdown
		start := time.Now()
		err = manager.Shutdown()
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}

		// Should complete quickly (well before timeout)
		if duration > 1*time.Second {
			t.Errorf("Shutdown took too long: %v", duration)
		}
	})
}

func TestClusterManager_Run(t *testing.T) {
	// Create temporary directory for test config
	tempDir := t.TempDir()

	// Create valid config
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

	configPath := filepath.Join(tempDir, "config.json")
	configData, _ := json.MarshalIndent(validConfig, "", "  ")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	t.Run("cluster manager lifecycle", func(t *testing.T) {
		logger := zap.NewNop()
		manager, err := NewManager(ManagerConfig{
			ConfigPath: configPath,
			Logger:     logger,
		})

		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		clusterMgr := manager.clusters["test-cluster"]
		if clusterMgr == nil {
			t.Fatal("Cluster manager not found")
		}

		// Run cluster manager with cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			clusterMgr.Run(ctx)
			close(done)
		}()

		// Let it run briefly
		time.Sleep(100 * time.Millisecond)

		// Cancel and wait for shutdown
		cancel()

		select {
		case <-done:
			// Shutdown completed
		case <-time.After(1 * time.Second):
			t.Error("ClusterManager.Run() did not stop within timeout")
		}
	})
}

func TestClusterManager_VersionTracking(t *testing.T) {
	logger := zap.NewNop()
	config := &ClusterConfig{
		Name:      "test-cluster",
		TenantID:  "12345678-1234-1234-1234-123456789012",
		ClusterID: "87654321-4321-4321-4321-210987654321",
		NodeID:    "abcdef12-3456-7890-abcd-ef1234567890",
		NodeToken: "12345678901234567890123456789012345678901",
		ConfigDir: "/etc/nebula/test",
	}

	cm := &ClusterManager{
		name:   config.Name,
		config: config,
		logger: logger,
	}

	// Test initial version
	if v := cm.GetCurrentVersion(); v != 0 {
		t.Errorf("Initial version should be 0, got %d", v)
	}

	// Test setting version
	cm.SetCurrentVersion(42)
	if v := cm.GetCurrentVersion(); v != 42 {
		t.Errorf("Expected version 42, got %d", v)
	}
}
