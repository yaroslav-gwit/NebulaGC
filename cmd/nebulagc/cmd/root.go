package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version information (set at build time via ldflags)
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "nebulagc",
	Short: "NebulaGC - Nebula VPN Control Plane & Node Daemon",
	Long: `NebulaGC provides centralized management for Nebula overlay networks.

It consists of:
  - Control Plane: Manages clusters, nodes, and configurations
  - Node Daemon: Runs on each node to manage local Nebula instances

The daemon automatically:
  - Polls for configuration updates
  - Manages Nebula process lifecycle
  - Handles automatic restarts on crashes
  - Supports multiple clusters per node`,
	SilenceUsage: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here if needed
}

// versionString returns formatted version information
func versionString() string {
	return fmt.Sprintf("NebulaGC %s (commit: %s, built: %s)",
		Version, Commit, BuildDate)
}
