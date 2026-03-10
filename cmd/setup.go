package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Register clotilde hooks globally for Claude Code",
		Long: `Register SessionStart hooks in ~/.claude/settings.json so clotilde
works automatically in all projects. Run this once after installing clotilde.

Use --local to install hooks in ~/.claude/settings.local.json instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			local, _ := cmd.Flags().GetBool("local")

			if err := claude.IsInstalled(); err != nil {
				return err
			}

			clotildeBinary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to determine clotilde binary path: %w", err)
			}

			homeDir, err := util.HomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}

			settingsFile := "settings.json"
			if local {
				settingsFile = "settings.local.json"
			}

			claudeDir := filepath.Join(homeDir, ".claude")
			settingsPath := filepath.Join(claudeDir, settingsFile)

			// Ensure ~/.claude directory exists
			if err := util.EnsureDir(claudeDir); err != nil {
				return fmt.Errorf("failed to create ~/.claude directory: %w", err)
			}

			hooks, err := mergeHooksIntoSettings(settingsPath, clotildeBinary)
			if err != nil {
				return err
			}

			hooksJSON, _ := json.MarshalIndent(hooks, "  ", "  ")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added hooks to ~/%s:\n  %s\n", filepath.Join(".claude", settingsFile), string(hooksJSON))
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success("Clotilde setup complete!"))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  Sessions will be created automatically when you run:")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  clotilde start <session-name>")

			return nil
		},
	}

	cmd.Flags().Bool("local", false, "Install hooks in ~/.claude/settings.local.json instead of settings.json")

	return cmd
}
