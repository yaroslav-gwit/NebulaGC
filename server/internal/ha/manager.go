package ha

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"nebulagc.io/server/internal/service"
)

// Manager manages high availability operations for a control plane instance.
//
// The manager handles:
// - Initial replica registration
// - Periodic heartbeat sending
// - Periodic stale replica pruning
// - Graceful shutdown and cleanup
type Manager struct {
	config  *Config
	service *service.ReplicaService
	logger  *zap.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// For testing - allow overriding time functions
	now func() time.Time
}

// NewManager creates a new HA manager.
//
// Parameters:
//   - config: HA configuration
//   - service: Replica service for database operations
//   - logger: Zap logger for structured logging
//
// Returns:
//   - Configured Manager
func NewManager(config *Config, service *service.ReplicaService, logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:  config,
		service: service,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		now:     time.Now,
	}
}

// Start initializes the HA manager and starts background goroutines.
//
// This function:
// 1. Registers this instance in the replicas table
// 2. Starts the heartbeat goroutine
// 3. Starts the pruning goroutine (if enabled)
//
// Returns:
//   - error: Any error that occurred during startup
func (m *Manager) Start() error {
	// Register this instance
	if err := m.service.Register(m.config.InstanceID, m.config.URL); err != nil {
		return fmt.Errorf("failed to register replica: %w", err)
	}

	m.logger.Info("HA manager started",
		zap.String("instance_id", m.config.InstanceID),
		zap.String("url", m.config.URL),
		zap.Duration("heartbeat_interval", m.config.HeartbeatInterval),
		zap.Bool("pruning_enabled", m.config.EnablePruning),
	)

	// Start heartbeat goroutine
	m.wg.Add(1)
	go m.heartbeatLoop()

	// Start pruning goroutine if enabled
	if m.config.EnablePruning {
		m.wg.Add(1)
		go m.pruningLoop()
	}

	return nil
}

// Stop gracefully shuts down the HA manager.
//
// This function:
// 1. Cancels the context to stop background goroutines
// 2. Waits for goroutines to finish
// 3. Unregisters this instance from the replicas table
func (m *Manager) Stop() error {
	m.logger.Info("stopping HA manager", zap.String("instance_id", m.config.InstanceID))

	// Cancel context to stop goroutines
	m.cancel()

	// Wait for goroutines to finish
	m.wg.Wait()

	// Unregister this instance
	if err := m.service.Unregister(m.config.InstanceID); err != nil {
		m.logger.Error("failed to unregister replica",
			zap.String("instance_id", m.config.InstanceID),
			zap.Error(err),
		)
		return err
	}

	m.logger.Info("HA manager stopped", zap.String("instance_id", m.config.InstanceID))
	return nil
}

// GetMaster returns information about the current master replica.
//
// Returns:
//   - *MasterInfo: Information about the current master
//   - error: Any error that occurred
func (m *Manager) GetMaster() (*MasterInfo, error) {
	return m.service.GetMaster(m.config.HeartbeatThreshold, m.config.InstanceID)
}

// IsMaster returns whether this instance is currently the master.
//
// Returns:
//   - bool: true if this instance is the master
//   - string: URL of the master (empty if we are master)
//   - error: Any error that occurred
func (m *Manager) IsMaster() (bool, string, error) {
	master, err := m.GetMaster()
	if err != nil {
		return false, "", err
	}

	if master.IsSelf {
		return true, "", nil
	}

	return false, master.URL, nil
}

// ListReplicas returns all healthy replicas.
//
// Returns:
//   - []*ReplicaInfo: List of healthy replicas
//   - error: Any error that occurred
func (m *Manager) ListReplicas() ([]*ReplicaInfo, error) {
	return m.service.ListReplicas(m.config.HeartbeatThreshold, m.config.InstanceID)
}

// heartbeatLoop runs the periodic heartbeat sender.
//
// This goroutine sends a heartbeat at the configured interval until
// the context is cancelled.
func (m *Manager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	m.logger.Info("heartbeat loop started",
		zap.String("instance_id", m.config.InstanceID),
		zap.Duration("interval", m.config.HeartbeatInterval),
	)

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("heartbeat loop stopped", zap.String("instance_id", m.config.InstanceID))
			return

		case <-ticker.C:
			if err := m.service.SendHeartbeat(m.config.InstanceID); err != nil {
				m.logger.Error("failed to send heartbeat",
					zap.String("instance_id", m.config.InstanceID),
					zap.Error(err),
				)
			}
		}
	}
}

// pruningLoop runs the periodic stale replica pruner.
//
// This goroutine prunes stale replicas at the configured interval until
// the context is cancelled.
func (m *Manager) pruningLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.PruneInterval)
	defer ticker.Stop()

	m.logger.Info("pruning loop started",
		zap.Duration("interval", m.config.PruneInterval),
		zap.Duration("threshold", m.config.HeartbeatThreshold*PruneThresholdMultiplier),
	)

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("pruning loop stopped")
			return

		case <-ticker.C:
			count, err := m.service.PruneStale(m.config.HeartbeatThreshold, PruneThresholdMultiplier)
			if err != nil {
				m.logger.Error("failed to prune stale replicas", zap.Error(err))
			} else if count > 0 {
				m.logger.Info("pruned stale replicas", zap.Int("count", count))
			}
		}
	}
}
