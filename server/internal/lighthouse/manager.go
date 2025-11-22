package lighthouse

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Manager manages Nebula lighthouse processes.
type Manager struct {
	config    *Config
	db        *sql.DB
	logger    *zap.Logger
	processes map[string]*ProcessInfo // clusterID -> ProcessInfo
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewManager creates a new lighthouse manager.
//
// Parameters:
//   - config: Manager configuration
//   - db: Database connection
//   - logger: Zap logger
//
// Returns:
//   - Configured Manager
func NewManager(config *Config, db *sql.DB, logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:    config,
		db:        db,
		logger:    logger,
		processes: make(map[string]*ProcessInfo),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the lighthouse manager.
//
// This starts the background watcher goroutine that checks for
// config version updates and manages lighthouse processes.
//
// Returns:
//   - Error if manager cannot start
func (m *Manager) Start() error {
	if !m.config.Enabled {
		m.logger.Info("lighthouse management disabled")
		return nil
	}

	m.logger.Info("starting lighthouse manager",
		zap.String("instance_id", m.config.InstanceID),
		zap.String("base_path", m.config.BasePath),
		zap.Duration("check_interval", m.config.CheckInterval))

	// Ensure base directory exists
	if err := os.MkdirAll(m.config.BasePath, 0700); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Start background watcher
	m.wg.Add(1)
	go m.watchLoop()

	return nil
}

// Stop stops the lighthouse manager and all managed processes.
//
// Returns:
//   - Error if shutdown fails
func (m *Manager) Stop() error {
	m.logger.Info("stopping lighthouse manager")

	// Cancel context to stop watcher
	m.cancel()

	// Wait for watcher to stop
	m.wg.Wait()

	// Stop all processes
	m.mu.Lock()
	defer m.mu.Unlock()

	for clusterID, info := range m.processes {
		if err := m.stopProcessLocked(clusterID, info); err != nil {
			m.logger.Error("failed to stop process",
				zap.String("cluster_id", clusterID),
				zap.Error(err))
		}
	}

	return nil
}

// watchLoop is the background goroutine that checks for config updates.
func (m *Manager) watchLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	// Run once immediately
	m.checkClusters()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkClusters()
		}
	}
}

// checkClusters checks all lighthouse clusters for config updates.
func (m *Manager) checkClusters() {
	// Query lighthouse clusters
	rows, err := m.db.Query(`
		SELECT c.id, c.config_version,
		       COALESCE(cs.running_config_version, 0) as running_version
		FROM clusters c
		LEFT JOIN cluster_state cs ON c.id = cs.cluster_id AND cs.instance_id = ?
		WHERE c.provide_lighthouse = 1
	`, m.config.InstanceID)
	if err != nil {
		m.logger.Error("failed to query lighthouse clusters", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var clusterID string
		var configVersion, runningVersion int64

		if err := rows.Scan(&clusterID, &configVersion, &runningVersion); err != nil {
			m.logger.Error("failed to scan cluster row", zap.Error(err))
			continue
		}

		// Check if update needed
		if configVersion > runningVersion {
			m.logger.Info("config version mismatch, updating lighthouse",
				zap.String("cluster_id", clusterID),
				zap.Int64("current_version", configVersion),
				zap.Int64("running_version", runningVersion))

			if err := m.updateLighthouse(clusterID); err != nil {
				m.logger.Error("failed to update lighthouse",
					zap.String("cluster_id", clusterID),
					zap.Error(err))
			}
		}
	}

	// Check for crashed processes
	m.checkProcesses()
}

// updateLighthouse updates the configuration and restarts the lighthouse for a cluster.
func (m *Manager) updateLighthouse(clusterID string) error {
	// Load cluster config from database
	clusterConfig, err := m.loadClusterConfig(clusterID)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Write config files
	configPath, err := WriteConfigFiles(clusterConfig, m.config.BasePath)
	if err != nil {
		return fmt.Errorf("failed to write config files: %w", err)
	}

	// Stop existing process if running
	m.mu.Lock()
	if info, exists := m.processes[clusterID]; exists {
		if err := m.stopProcessLocked(clusterID, info); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("failed to stop existing process: %w", err)
		}
	}
	m.mu.Unlock()

	// Start new process
	if err := m.startProcess(clusterID, configPath, clusterConfig.ConfigVersion); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Update cluster_state
	if err := m.updateClusterState(clusterID, clusterConfig.ConfigVersion); err != nil {
		return fmt.Errorf("failed to update cluster state: %w", err)
	}

	return nil
}

// loadClusterConfig loads cluster configuration from the database.
func (m *Manager) loadClusterConfig(clusterID string) (*ClusterConfig, error) {
	var config ClusterConfig
	var lighthousePort sql.NullInt64

	err := m.db.QueryRow(`
		SELECT id, name, config_version, ca_cert, crl, lighthouse_cert, lighthouse_key, lighthouse_port
		FROM clusters
		WHERE id = ?
	`, clusterID).Scan(
		&config.ClusterID,
		&config.ClusterName,
		&config.ConfigVersion,
		&config.CACert,
		&config.CRL,
		&config.HostCert,
		&config.HostKey,
		&lighthousePort,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query cluster: %w", err)
	}

	if lighthousePort.Valid {
		config.LighthousePort = int(lighthousePort.Int64)
	} else {
		config.LighthousePort = 4242 // Default port
	}

	// Load replicas (not used in config yet, but available for static host map)
	rows, err := m.db.Query(`SELECT instance_id, address FROM replicas`)
	if err != nil {
		return nil, fmt.Errorf("failed to query replicas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var replica ReplicaInfo
		if err := rows.Scan(&replica.InstanceID, &replica.Address); err != nil {
			continue
		}
		config.Replicas = append(config.Replicas, replica)
	}

	return &config, nil
}

// startProcess starts a Nebula process for a cluster.
func (m *Manager) startProcess(clusterID, configPath string, version int64) error {
	cmd := exec.Command(m.config.NebulaBinary, "-config", configPath)
	cmd.Stdout = nil // TODO: Pipe to logger
	cmd.Stderr = nil // TODO: Pipe to logger

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start nebula: %w", err)
	}

	m.mu.Lock()
	m.processes[clusterID] = &ProcessInfo{
		ClusterID:     clusterID,
		PID:           cmd.Process.Pid,
		ConfigVersion: version,
		StartedAt:     time.Now(),
	}
	m.mu.Unlock()

	m.logger.Info("started lighthouse process",
		zap.String("cluster_id", clusterID),
		zap.Int("pid", cmd.Process.Pid),
		zap.Int64("version", version))

	return nil
}

// stopProcessLocked stops a running process (caller must hold lock).
func (m *Manager) stopProcessLocked(clusterID string, info *ProcessInfo) error {
	process, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-time.After(5 * time.Second):
		// Force kill
		process.Kill()
	case <-done:
	}

	delete(m.processes, clusterID)

	m.logger.Info("stopped lighthouse process",
		zap.String("cluster_id", clusterID),
		zap.Int("pid", info.PID))

	return nil
}

// checkProcesses checks if managed processes are still running and restarts if needed.
func (m *Manager) checkProcesses() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for clusterID, info := range m.processes {
		process, err := os.FindProcess(info.PID)
		if err != nil {
			m.logger.Warn("process not found, needs restart",
				zap.String("cluster_id", clusterID),
				zap.Int("pid", info.PID))
			// Trigger update on next check
			continue
		}

		// Check if process is still alive by sending signal 0
		if err := process.Signal(syscall.Signal(0)); err != nil {
			m.logger.Warn("process crashed, will restart on next check",
				zap.String("cluster_id", clusterID),
				zap.Int("pid", info.PID),
				zap.Error(err))
		}
	}
}

// updateClusterState updates the running config version in the database.
func (m *Manager) updateClusterState(clusterID string, version int64) error {
	now := time.Now().Unix()

	_, err := m.db.Exec(`
		INSERT INTO cluster_state (cluster_id, instance_id, running_config_version, last_updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (cluster_id, instance_id) DO UPDATE SET
			running_config_version = excluded.running_config_version,
			last_updated_at = excluded.last_updated_at
	`, clusterID, m.config.InstanceID, version, now)

	if err != nil {
		return fmt.Errorf("failed to update cluster_state: %w", err)
	}

	return nil
}
