package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yaroslav/nebulagc/cmd/nebulagc/daemon"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	configPath string
	devMode    bool
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the NebulaGC node daemon",
	Long: `Start the daemon to manage Nebula instances on this node.

The daemon will:
  - Load configuration from the specified file
  - Connect to the control plane
  - Poll for configuration updates every 5 seconds
  - Manage Nebula processes for each configured cluster
  - Automatically restart processes on crashes
  - Handle graceful shutdown on SIGTERM/SIGINT

Configuration file should be in JSON format and specify:
  - Control plane URLs
  - Cluster credentials (tenant ID, cluster ID, node ID, tokens)
  - Local config directories`,
	RunE: runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)

	daemonCmd.Flags().StringVarP(&configPath, "config", "c", "/etc/nebulagc/config.json",
		"Path to daemon configuration file")
	daemonCmd.Flags().BoolVar(&devMode, "dev", false,
		"Enable development mode (console logging instead of JSON)")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger, err := initLogger(devMode)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("NebulaGC daemon starting",
		zap.String("version", Version),
		zap.String("commit", Commit),
		zap.String("config", configPath))

	// Create daemon manager
	manager, err := daemon.NewManager(daemon.ManagerConfig{
		ConfigPath: configPath,
		Logger:     logger,
	})
	if err != nil {
		logger.Error("Failed to create daemon manager", zap.Error(err))
		return fmt.Errorf("failed to create manager: %w", err)
	}

	logger.Info("Daemon manager created successfully")

	// Run the manager (blocks until shutdown)
	if err := manager.Run(); err != nil {
		logger.Error("Manager error", zap.Error(err))
		return err
	}

	logger.Info("Daemon stopped")
	return nil
}

func initLogger(devMode bool) (*zap.Logger, error) {
	var config zap.Config

	if devMode {
		// Development mode: human-readable console output
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		// Production mode: JSON structured logging
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	return config.Build()
}
