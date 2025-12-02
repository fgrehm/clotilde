package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version information, set via ldflags at build time
	version   = "DEVELOPMENT"
	commit    = "none"
	date      = "unknown"
	goVersion = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, commit hash, build date, and Go version.`,
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "clotilde version %s\n", version)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", commit)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", date)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  go:     %s\n", goVersion)
	},
}
