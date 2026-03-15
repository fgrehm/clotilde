package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Register clotilde hooks globally for Claude Code",
		Long: `Register SessionStart hooks in ~/.claude/settings.json so clotilde
works automatically in all projects. Run this once after installing clotilde.

Use --local to install hooks in ~/.claude/settings.local.json instead.
Use --stats to enable session statistics tracking (opt-in).`,
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

			// Resolve stats preference
			statsEnabled := resolveStatsPreference(cmd)

			// Persist stats preference in global config
			globalCfg, err := config.LoadGlobalOrDefault()
			if err != nil {
				return fmt.Errorf("failed to load global config: %w", err)
			}
			globalCfg.StatsTracking = &statsEnabled
			if err := config.SaveGlobal(globalCfg); err != nil {
				return fmt.Errorf("failed to save global config: %w", err)
			}

			opts := claude.HookConfigOptions{
				StatsEnabled: statsEnabled,
			}
			hooks, err := mergeHooksIntoSettings(settingsPath, clotildeBinary, opts)
			if err != nil {
				return err
			}

			hooksJSON, _ := json.MarshalIndent(hooks, "  ", "  ")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added hooks to ~/%s:\n  %s\n", filepath.Join(".claude", settingsFile), string(hooksJSON))
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success("Clotilde setup complete!"))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  Sessions will be created automatically when you run:")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  clotilde start <session-name>")

			if statsEnabled {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  Session statistics tracking is enabled.")
			}

			return nil
		},
	}

	cmd.Flags().Bool("local", false, "Install hooks in ~/.claude/settings.local.json instead of settings.json")
	cmd.Flags().Bool("stats", false, "Enable session statistics tracking")
	cmd.Flags().Bool("no-stats", false, "Disable session statistics tracking")
	cmd.MarkFlagsMutuallyExclusive("stats", "no-stats")

	return cmd
}

// resolveStatsPreference determines whether stats should be enabled.
// --stats flag takes precedence, then --no-stats, then existing config, then interactive prompt.
func resolveStatsPreference(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("stats") {
		return true
	}
	if cmd.Flags().Changed("no-stats") {
		return false
	}

	// Preserve existing preference from global config (both true and false)
	globalCfg, err := config.LoadGlobalOrDefault()
	if err == nil && globalCfg.StatsTracking != nil {
		return *globalCfg.StatsTracking
	}

	// Only prompt interactively when stdin is a TTY
	stdin := cmd.InOrStdin()
	if f, ok := stdin.(*os.File); !ok || !isatty.IsTerminal(f.Fd()) {
		return false
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Track session statistics (turns, tokens, tool usage)? [y/N] ")
	reader := bufio.NewReader(stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes"
}
