package daemon

import (
	"context"
	"time"

	"github.com/yaroslav/nebulagc/sdk"
	"go.uber.org/zap"
)

// ClusterManager manages the lifecycle of a single Nebula cluster instance.
// It coordinates polling for config updates and process supervision.
type ClusterManager struct {
	// name is the human-readable cluster identifier
	name string

	// config is the cluster configuration
	config *ClusterConfig

	// client is the SDK client for this cluster
	client *sdk.Client

	// logger is the structured logger with cluster context
	logger *zap.Logger

	// currentVersion tracks the last known config bundle version
	currentVersion int64

	// poller manages config version polling and updates
	poller *Poller

	// bundleManager handles bundle extraction and atomic replacement
	bundleManager *BundleManager

	// supervisor manages the Nebula process lifecycle
	supervisor *Supervisor

	// healthChecker performs periodic health checks on the control plane
	healthChecker *HealthChecker
}

// Run starts the cluster manager and blocks until context is cancelled.
// This method manages config polling and will supervise the Nebula process (Task 00020).
//
// Parameters:
//   - ctx: Context for cancellation and shutdown
func (cm *ClusterManager) Run(ctx context.Context) {
	cm.logger.Info("Cluster manager started",
		zap.String("tenant_id", cm.config.TenantID),
		zap.String("cluster_id", cm.config.ClusterID),
		zap.String("node_id", cm.config.NodeID),
		zap.String("config_dir", cm.config.ConfigDir),
	)

	// Initialize: discover control plane master
	if err := cm.discoverMaster(ctx); err != nil {
		cm.logger.Error("Failed to discover control plane master", zap.Error(err))
		// Continue anyway - SDK will use round-robin if no master cached
	}

	// Initialize bundle manager
	cm.bundleManager = NewBundleManager(cm.config.ConfigDir)

	// Initialize supervisor
	configPath := cm.config.ConfigDir + "/config.yml"
	cm.supervisor = NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       1 * time.Second,
		MaxBackoff:       60 * time.Second,
		SuccessThreshold: 5 * time.Minute,
		Logger:           cm.logger,
	})

	// onUpdate callback that applies bundle and restarts Nebula
	onUpdate := func(ctx context.Context, data []byte, version int64) error {
		// First apply the bundle
		if err := cm.bundleManager.ApplyBundle(ctx, data, version); err != nil {
			return err
		}

		// Then restart Nebula to pick up new config
		cm.logger.Info("Restarting Nebula after config update",
			zap.Int64("version", version))
		cm.supervisor.Restart()

		return nil
	}

	// Initialize poller
	cm.poller = NewPoller(PollerConfig{
		Client:            cm.client,
		Logger:            cm.logger,
		Interval:          5 * time.Second,
		OnUpdate:          onUpdate,
		GetCurrentVersion: cm.GetCurrentVersion,
		SetCurrentVersion: cm.SetCurrentVersion,
	})

	// Initialize health checker
	cm.healthChecker = NewHealthChecker(cm.client, cm.logger)

	// Start config poller in goroutine
	go cm.poller.Run(ctx)

	// Start health checker in goroutine
	cm.healthChecker.Start(ctx)

	// Start Nebula process supervisor in goroutine
	go func() {
		if err := cm.supervisor.Run(); err != nil {
			cm.logger.Error("Supervisor error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	cm.logger.Info("Cluster manager shutting down")

	// Stop health checker
	cm.healthChecker.Stop()

	// Gracefully stop Nebula process
	if err := cm.supervisor.Stop(); err != nil {
		cm.logger.Error("Error stopping supervisor", zap.Error(err))
	}
}

// discoverMaster attempts to discover and cache the control plane master.
func (cm *ClusterManager) discoverMaster(ctx context.Context) error {
	cm.logger.Info("Discovering control plane master")

	// Try to discover master with timeout
	discoverCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := cm.client.DiscoverMaster(discoverCtx); err != nil {
		return err
	}

	cm.logger.Info("Control plane master discovered")
	return nil
}

// GetCurrentVersion returns the currently deployed config bundle version.
func (cm *ClusterManager) GetCurrentVersion() int64 {
	return cm.currentVersion
}

// SetCurrentVersion updates the tracked config bundle version.
func (cm *ClusterManager) SetCurrentVersion(version int64) {
	cm.currentVersion = version
	cm.logger.Info("Updated config version", zap.Int64("version", version))
}

// IsDegraded returns true if the cluster is in degraded mode.
func (cm *ClusterManager) IsDegraded() bool {
	if cm.healthChecker == nil {
		return false
	}
	return cm.healthChecker.IsDegraded()
}

// GetHealthStatus returns the current health status of the cluster.
func (cm *ClusterManager) GetHealthStatus() (healthy, total int, lastCheck time.Time) {
	if cm.healthChecker == nil {
		return 0, 0, time.Time{}
	}
	return cm.healthChecker.GetHealthStatus()
}
