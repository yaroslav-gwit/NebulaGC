package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Manager coordinates the lifecycle of multiple Nebula cluster instances.
// It spawns a ClusterManager for each configured cluster and handles
// graceful shutdown on signals or errors.
type Manager struct {
	// daemon holds the daemon configuration and SDK clients
	daemon *Daemon

	// logger is the structured logger for the manager
	logger *zap.Logger

	// shutdownTimeout is the maximum time to wait for graceful shutdown
	shutdownTimeout time.Duration

	// clusters maps cluster names to their managers
	clusters map[string]*ClusterManager

	// wg tracks running cluster manager goroutines
	wg sync.WaitGroup

	// cancel is called to signal shutdown to all cluster managers
	cancel context.CancelFunc
}

// ManagerConfig holds configuration for the Manager.
type ManagerConfig struct {
	// ConfigPath is the optional path to the config file
	ConfigPath string

	// Logger is the structured logger (optional, will create default if nil)
	Logger *zap.Logger

	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	// Default: 30 seconds
	ShutdownTimeout time.Duration
}

// NewManager creates a new daemon manager.
//
// Parameters:
//   - config: Manager configuration
//
// Returns:
//   - *Manager: The initialized manager
//   - error: Initialization error
func NewManager(config ManagerConfig) (*Manager, error) {
	// Initialize daemon (loads config, creates SDK clients)
	daemon, err := Initialize(config.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize daemon: %w", err)
	}

	// Create logger if not provided
	logger := config.Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	// Set default shutdown timeout
	shutdownTimeout := config.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	manager := &Manager{
		daemon:          daemon,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
		clusters:        make(map[string]*ClusterManager),
	}

	// Create cluster managers
	for _, clusterName := range daemon.ClusterNames() {
		clusterConfig, _ := daemon.GetClusterConfig(clusterName)
		client, _ := daemon.GetClient(clusterName)

		clusterManager := &ClusterManager{
			name:   clusterName,
			config: clusterConfig,
			client: client,
			logger: logger.With(zap.String("cluster", clusterName)),
		}

		manager.clusters[clusterName] = clusterManager
	}

	return manager, nil
}

// Run starts the daemon manager and blocks until shutdown is signaled.
// It spawns goroutines for each cluster manager and handles OS signals.
//
// Returns:
//   - error: Error during startup or shutdown
func (m *Manager) Run() error {
	m.logger.Info("Starting NebulaGC daemon",
		zap.Int("clusters", len(m.clusters)),
		zap.Strings("cluster_names", m.daemon.ClusterNames()),
	)

	// Create cancellable context for all cluster managers
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// Start each cluster manager in a goroutine
	for name, clusterMgr := range m.clusters {
		m.wg.Add(1)
		go func(name string, mgr *ClusterManager) {
			defer m.wg.Done()
			mgr.Run(ctx)
		}(name, clusterMgr)
		m.logger.Info("Started cluster manager", zap.String("cluster", name))
	}

	// Wait for shutdown signal
	m.waitForSignal()

	// Shutdown gracefully
	return m.Shutdown()
}

// Shutdown gracefully stops all cluster managers.
//
// Returns:
//   - error: Shutdown error
func (m *Manager) Shutdown() error {
	m.logger.Info("Shutting down daemon", zap.Duration("timeout", m.shutdownTimeout))

	// Cancel context to signal all cluster managers to stop
	if m.cancel != nil {
		m.cancel()
	}

	// Wait for all cluster managers to finish (with timeout)
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("All cluster managers stopped gracefully")
		return nil
	case <-time.After(m.shutdownTimeout):
		m.logger.Warn("Shutdown timeout exceeded, forcing exit")
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// waitForSignal blocks until SIGTERM or SIGINT is received.
func (m *Manager) waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigChan
	m.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
}

// Stop triggers a graceful shutdown (alias for Shutdown).
func (m *Manager) Stop() error {
	return m.Shutdown()
}
