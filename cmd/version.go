package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version, Commit, and BuildDate are set at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ecsctl version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ecsctl version %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
