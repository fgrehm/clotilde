package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize clotilde in the current project",
	Long: `Initialize clotilde by creating the .claude/clotilde directory structure
and setting up SessionStart hooks in .claude/settings.local.json (local to your machine).

Use --global to install hooks in .claude/settings.json instead (shared with team).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		// Check if claude is installed
		if err := claude.IsInstalled(); err != nil {
			return err
		}

		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Check if already initialized
		alreadyInitialized := config.IsInitialized()
		if alreadyInitialized {
			fmt.Println("Clotilde is already initialized. Updating hooks...")
		} else {
			// Create .claude/clotilde structure
			fmt.Println("Creating .claude/clotilde structure...")
			if err := config.EnsureClotildeStructure(cwd); err != nil {
				return fmt.Errorf("failed to create clotilde structure: %w", err)
			}
		}

		// Get path to clotilde binary (for hooks)
		clotildeBinary, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine clotilde binary path: %w", err)
		}

		// Setup hooks
		settingsFile := "settings.local.json"
		if global {
			settingsFile = "settings.json"
		}

		if !alreadyInitialized {
			if global {
				fmt.Println("Setting up SessionStart hooks in .claude/settings.json (project-wide)...")
			} else {
				fmt.Println("Setting up SessionStart hooks in .claude/settings.local.json (local to this machine)...")
			}
		}
		if err := setupHooks(cwd, clotildeBinary, settingsFile); err != nil {
			return fmt.Errorf("failed to setup hooks: %w", err)
		}

		clotildeRoot := filepath.Join(cwd, config.ClotildeDir)
		if alreadyInitialized {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success("Hooks updated successfully!"))
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success("Clotilde initialized successfully!"))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Root: %s\n", clotildeRoot)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nYou can now create sessions with:\n")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  clotilde start <session-name>\n")
		}

		return nil
	},
}

func setupHooks(projectRoot, clotildeBinary, settingsFile string) error {
	claudeDir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(claudeDir, settingsFile)

	// Ensure .claude directory exists
	if !util.FileExists(claudeDir) {
		if err := os.MkdirAll(claudeDir, 0o755); err != nil {
			return fmt.Errorf("failed to create .claude directory: %w", err)
		}
	}

	// Read existing settings if they exist
	var settings map[string]interface{}
	if util.FileExists(settingsPath) {
		if err := util.ReadJSON(settingsPath, &settings); err != nil {
			return fmt.Errorf("failed to read existing settings: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Generate hook config
	hookConfig := claude.GenerateHookConfig(clotildeBinary)

	// Merge hooks into settings
	var hooks map[string]interface{}
	if existingHooks, ok := settings["hooks"].(map[string]interface{}); ok {
		// Start with existing hooks
		hooks = existingHooks
	} else {
		hooks = make(map[string]interface{})
	}

	// Add/update clotilde hooks (SessionStart only)
	hooks["SessionStart"] = hookConfig.SessionStart
	settings["hooks"] = hooks

	// Write settings back
	if err := util.WriteJSON(settingsPath, settings); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	// Pretty print the hooks for user confirmation
	hooksJSON, _ := json.MarshalIndent(hooks, "  ", "  ")
	fmt.Printf("  Added hooks to .claude/%s:\n  %s\n", settingsFile, string(hooksJSON))

	return nil
}
