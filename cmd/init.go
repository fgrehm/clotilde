package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

var initCmd = &cobra.Command{
	Use:        "init",
	Short:      "Initialize clotilde in the current project",
	Deprecated: "use 'clotilde setup' for global hook installation. Sessions are now created automatically.",
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

// mergeHooksIntoSettings reads a Claude settings file, merges clotilde's
// hooks, and writes it back. Returns the merged hooks map for display purposes.
// The caller is responsible for ensuring the parent directory exists.
func mergeHooksIntoSettings(settingsPath, clotildeBinary string, opts claude.HookConfigOptions) (map[string]interface{}, error) {
	// Read existing settings if they exist
	var settings map[string]interface{}
	if util.FileExists(settingsPath) {
		if err := util.ReadJSON(settingsPath, &settings); err != nil {
			return nil, fmt.Errorf("failed to read existing settings: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Generate hook config
	hookConfig := claude.GenerateHookConfig(clotildeBinary, opts)

	// Merge hooks into settings, preserving non-clotilde hooks
	var hooks map[string]interface{}
	if existingHooks, ok := settings["hooks"].(map[string]interface{}); ok {
		hooks = existingHooks
	} else {
		hooks = make(map[string]interface{})
	}

	mergeHookType := func(key string, matchers []claude.HookMatcher) {
		existing := hooks[key]
		merged := stripAndAppendHooks(existing, matchers, clotildeBinary)
		if len(merged) > 0 {
			hooks[key] = merged
		} else {
			delete(hooks, key)
		}
	}

	mergeHookType("SessionStart", hookConfig.SessionStart)
	mergeHookType("Stop", hookConfig.Stop)
	mergeHookType("Notification", hookConfig.Notification)
	mergeHookType("PreToolUse", hookConfig.PreToolUse)
	mergeHookType("PostToolUse", hookConfig.PostToolUse)
	mergeHookType("SessionEnd", hookConfig.SessionEnd)
	settings["hooks"] = hooks

	if err := util.WriteJSON(settingsPath, settings); err != nil {
		return nil, fmt.Errorf("failed to write settings: %w", err)
	}

	return hooks, nil
}

// stripAndAppendHooks removes clotilde hooks from existing matchers, then
// appends the new clotilde matchers. Preserves non-clotilde hooks. A hook is
// considered "clotilde" if its command starts with the clotilde binary path.
// Returns nil if both existing and new are empty.
func stripAndAppendHooks(existing interface{}, newMatchers []claude.HookMatcher, clotildeBinary string) []interface{} {
	prefix := clotildeBinary + " "
	var result []interface{}

	// Process existing matchers: remove clotilde hooks, keep others
	if arr, ok := existing.([]interface{}); ok {
		for _, item := range arr {
			matcher, ok := item.(map[string]interface{})
			if !ok {
				result = append(result, item)
				continue
			}

			hooksRaw, ok := matcher["hooks"].([]interface{})
			if !ok {
				result = append(result, item)
				continue
			}

			// Filter out clotilde hooks
			var kept []interface{}
			for _, h := range hooksRaw {
				hook, ok := h.(map[string]interface{})
				if !ok {
					kept = append(kept, h)
					continue
				}
				cmd, _ := hook["command"].(string)
				if isClotildeHookCmd(cmd, prefix, clotildeBinary) {
					continue // strip clotilde hook
				}
				kept = append(kept, h)
			}

			if len(kept) > 0 {
				stripped := make(map[string]interface{})
				for k, v := range matcher {
					stripped[k] = v
				}
				stripped["hooks"] = kept
				result = append(result, stripped)
			}
		}
	}

	// Append new clotilde matchers
	for _, m := range newMatchers {
		result = append(result, m)
	}

	return result
}

// isClotildeHookCmd returns true if a hook command belongs to clotilde,
// either matching the current binary path or any binary named "clotilde".
func isClotildeHookCmd(cmd, currentPrefix, currentBinary string) bool {
	if strings.HasPrefix(cmd, currentPrefix) || cmd == currentBinary {
		return true
	}
	// Detect old clotilde hooks installed from a different path
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return filepath.Base(parts[0]) == "clotilde"
	}
	return false
}

func setupHooks(projectRoot, clotildeBinary, settingsFile string) error {
	claudeDir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(claudeDir, settingsFile)

	// Ensure .claude directory exists
	if err := util.EnsureDir(claudeDir); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	hooks, err := mergeHooksIntoSettings(settingsPath, clotildeBinary, claude.HookConfigOptions{})
	if err != nil {
		return err
	}

	// Pretty print the hooks for user confirmation
	hooksJSON, _ := json.MarshalIndent(hooks, "  ", "  ")
	fmt.Printf("  Added hooks to .claude/%s:\n  %s\n", settingsFile, string(hooksJSON))

	return nil
}
