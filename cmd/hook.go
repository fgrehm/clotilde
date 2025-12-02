package cmd

import (
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Internal commands for Claude Code hooks",
	Hidden: true,
	Long: `Internal commands called by Claude Code's session hooks.
These commands are not intended for direct user invocation.`,
}
