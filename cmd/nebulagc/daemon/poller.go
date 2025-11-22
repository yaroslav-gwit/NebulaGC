package daemon

import (
	"context"
	"time"

	"github.com/yaroslav/nebulagc/sdk"
	"go.uber.org/zap"
)

// Poller manages config version polling and updates for a cluster.
type Poller struct {
	// client is the SDK client for API calls
	client *sdk.Client

	// logger is the structured logger with cluster context
	logger *zap.Logger

	// interval is the time between polling attempts
	interval time.Duration

	// onUpdate is called when a new config version is available
	// Returns the new version number and any error
	onUpdate func(ctx context.Context, data []byte, version int64) error

	// getCurrentVersion returns the currently deployed version
	getCurrentVersion func() int64

	// setCurrentVersion updates the tracked version
	setCurrentVersion func(int64)
}

// PollerConfig holds configuration for creating a Poller.
type PollerConfig struct {
	// Client is the SDK client
	Client *sdk.Client

	// Logger is the structured logger
	Logger *zap.Logger

	// Interval is the polling interval (default: 5 seconds)
	Interval time.Duration

	// OnUpdate is called when new config is available
	OnUpdate func(ctx context.Context, data []byte, version int64) error

	// GetCurrentVersion returns the current version
	GetCurrentVersion func() int64

	// SetCurrentVersion updates the version
	SetCurrentVersion func(int64)
}

// NewPoller creates a new config poller.
func NewPoller(config PollerConfig) *Poller {
	interval := config.Interval
	if interval == 0 {
		interval = 5 * time.Second
	}

	return &Poller{
		client:            config.Client,
		logger:            config.Logger,
		interval:          interval,
		onUpdate:          config.OnUpdate,
		getCurrentVersion: config.GetCurrentVersion,
		setCurrentVersion: config.SetCurrentVersion,
	}
}

// Run starts the polling loop and blocks until context is cancelled.
//
// The loop:
// 1. Queries the latest config version from control plane
// 2. Compares with current version
// 3. Downloads and applies new config if available
// 4. Waits for next interval or context cancellation
//
// Parameters:
//   - ctx: Context for cancellation
func (p *Poller) Run(ctx context.Context) {
	p.logger.Info("Config poller started", zap.Duration("interval", p.interval))

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run initial check immediately
	p.checkForUpdate(ctx)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Config poller stopped")
			return
		case <-ticker.C:
			p.checkForUpdate(ctx)
		}
	}
}

// checkForUpdate checks if a new config version is available and applies it.
func (p *Poller) checkForUpdate(ctx context.Context) {
	currentVersion := p.getCurrentVersion()

	// Query latest version from control plane
	latestVersion, err := p.client.GetLatestVersion(ctx)
	if err != nil {
		p.logger.Error("Failed to get latest version", zap.Error(err))
		return
	}

	// Check if update is needed
	if latestVersion <= currentVersion {
		p.logger.Debug("No update available",
			zap.Int64("current", currentVersion),
			zap.Int64("latest", latestVersion),
		)
		return
	}

	p.logger.Info("New config version available",
		zap.Int64("current", currentVersion),
		zap.Int64("latest", latestVersion),
	)

	// Download bundle
	data, newVersion, err := p.client.DownloadBundle(ctx, currentVersion)
	if err != nil {
		p.logger.Error("Failed to download bundle", zap.Error(err))
		return
	}

	// Handle 304 Not Modified (no data returned)
	if data == nil {
		p.logger.Debug("Bundle not modified (304)",
			zap.Int64("version", newVersion),
		)
		return
	}

	p.logger.Info("Downloaded config bundle",
		zap.Int64("version", newVersion),
		zap.Int("size_bytes", len(data)),
	)

	// Apply the update
	if err := p.onUpdate(ctx, data, newVersion); err != nil {
		p.logger.Error("Failed to apply config update",
			zap.Error(err),
			zap.Int64("version", newVersion),
		)
		return
	}

	// Update tracked version
	p.setCurrentVersion(newVersion)

	p.logger.Info("Config update applied successfully",
		zap.Int64("version", newVersion),
	)
}
