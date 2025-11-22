package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaroslav/nebulagc/sdk"
	"go.uber.org/zap"
)

// HealthCheckInterval is the duration between health checks.
const HealthCheckInterval = 60 * time.Second

// HealthChecker performs periodic health checks on control plane instances
// and manages degraded mode state for a cluster.
type HealthChecker struct {
	client  *sdk.Client
	logger  *zap.Logger
	closeCh chan struct{}
	wg      sync.WaitGroup

	mu              sync.RWMutex
	isDegraded      bool
	lastHealthCheck time.Time
	healthyReplicas int
	totalReplicas   int
}

// NewHealthChecker creates a new health checker for a control plane client.
func NewHealthChecker(client *sdk.Client, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		client:  client,
		logger:  logger,
		closeCh: make(chan struct{}),
	}
}

// Start begins periodic health checks in a background goroutine.
// It will check control plane health every HealthCheckInterval.
func (h *HealthChecker) Start(ctx context.Context) {
	h.wg.Add(1)
	go h.run(ctx)
}

// Stop gracefully stops the health checker and waits for cleanup.
func (h *HealthChecker) Stop() {
	close(h.closeCh)
	h.wg.Wait()
}

// IsDegraded returns true if the cluster is currently in degraded mode
// (master unreachable or all replicas unhealthy).
func (h *HealthChecker) IsDegraded() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.isDegraded
}

// GetHealthStatus returns the current health status of the cluster.
func (h *HealthChecker) GetHealthStatus() (healthy, total int, lastCheck time.Time) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthyReplicas, h.totalReplicas, h.lastHealthCheck
}

// run is the main health check loop.
func (h *HealthChecker) run(ctx context.Context) {
	defer h.wg.Done()

	// Perform initial health check immediately
	h.performHealthCheck(ctx)

	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.performHealthCheck(ctx)
		case <-h.closeCh:
			h.logger.Info("Health checker stopping")
			return
		case <-ctx.Done():
			h.logger.Info("Health checker context cancelled")
			return
		}
	}
}

// performHealthCheck executes a health check cycle.
func (h *HealthChecker) performHealthCheck(ctx context.Context) {
	h.logger.Debug("Performing health check")

	// Try to discover master first
	if err := h.client.DiscoverMaster(ctx); err != nil {
		h.logger.Warn("Failed to discover master during health check",
			zap.Error(err))
		h.setDegraded(true, 0, 0)
		return
	}

	// Try to get replica list
	replicas, err := h.client.GetClusterReplicas(ctx)
	if err != nil {
		h.logger.Warn("Failed to get replica list during health check",
			zap.Error(err))
		// We still have master, so not fully degraded
		h.setDegraded(false, 1, 1)
		return
	}

	// Count healthy replicas
	healthy := 0
	total := len(replicas)
	var masterFound bool

	for _, replica := range replicas {
		if replica.IsMaster {
			masterFound = true
		}
		// Check if replica is healthy (recent heartbeat)
		if time.Since(replica.LastHeartbeat) < 2*HealthCheckInterval {
			healthy++
		}
	}

	if !masterFound {
		h.logger.Warn("No master found in replica list")
		h.setDegraded(true, healthy, total)
		return
	}

	if healthy == 0 {
		h.logger.Warn("No healthy replicas found")
		h.setDegraded(true, 0, total)
		return
	}

	// All checks passed
	h.setDegraded(false, healthy, total)

	h.logger.Debug("Health check complete",
		zap.Int("healthy_replicas", healthy),
		zap.Int("total_replicas", total))
}

// setDegraded updates the degraded mode state and logs state changes.
func (h *HealthChecker) setDegraded(degraded bool, healthy, total int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update state
	wasDegraded := h.isDegraded
	h.isDegraded = degraded
	h.healthyReplicas = healthy
	h.totalReplicas = total
	h.lastHealthCheck = time.Now()

	// Log state changes
	if degraded && !wasDegraded {
		h.logger.Warn("Cluster entered DEGRADED mode",
			zap.Int("healthy_replicas", healthy),
			zap.Int("total_replicas", total),
			zap.String("impact", "continuing with existing configuration"))
	} else if !degraded && wasDegraded {
		h.logger.Info("Cluster recovered from DEGRADED mode",
			zap.Int("healthy_replicas", healthy),
			zap.Int("total_replicas", total))
	}
}

// RefreshReplicas forces an immediate refresh of the replica list.
// This can be called when connection errors occur to try to find healthy instances.
func (h *HealthChecker) RefreshReplicas(ctx context.Context) error {
	h.logger.Info("Forcing replica list refresh")

	replicas, err := h.client.GetClusterReplicas(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh replica list: %w", err)
	}

	// Extract URLs from replicas
	var urls []string
	for _, replica := range replicas {
		// Only include healthy replicas
		if time.Since(replica.LastHeartbeat) < 2*HealthCheckInterval {
			urls = append(urls, replica.URL)
		}
	}

	if len(urls) == 0 {
		return fmt.Errorf("no healthy replicas found")
	}

	h.logger.Info("Replica list refreshed",
		zap.Int("healthy_count", len(urls)),
		zap.Int("total_count", len(replicas)))

	// Note: We don't update client.BaseURLs here because the client
	// is immutable after creation. The daemon should recreate the client
	// if it needs to use a different set of URLs.

	return nil
}
