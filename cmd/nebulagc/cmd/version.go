package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, commit, build date, and Go version.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("NebulaGC %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Built: %s\n", BuildDate)
		fmt.Printf("Go: %s\n", runtime.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
