package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

// newResumeCmd creates a fresh resume command instance (avoids flag pollution in tests)
func newResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume [name] [-- <claude-flags>...]",
		Short: "Resume an existing session by name",
		Long: `Resume a Claude Code session by its human-friendly name.

If no session name is provided, an interactive picker will be shown
(in TTY environments).

Pass additional flags to Claude Code after '--':
  clotilde resume my-session -- --debug api,hooks`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: sessionNameCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
		}

		// Create store
		store := session.NewFileStore(clotildeRoot)

		// Determine session name
		var name string
		if len(args) == 0 {
			// No session name provided - show picker if in TTY
			isTTY := isatty.IsTerminal(os.Stdout.Fd())
			if !isTTY {
				return fmt.Errorf("session name required in non-interactive mode")
			}

			// Load all sessions
			sessions, err := store.List()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			if len(sessions) == 0 {
				return fmt.Errorf("no sessions available")
			}

			// Sort by last accessed (most recent first)
			sortSessionsByLastAccessed(sessions)

			// Show picker with preview pane
			picker := ui.NewPicker(sessions, "Select session to resume").WithPreview()
			selected, err := ui.RunPicker(picker)
			if err != nil {
				return fmt.Errorf("picker failed: %w", err)
			}

			if selected == nil {
				// User cancelled
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
				return nil
			}

			name = selected.Name
		} else {
			name = args[0]
		}

		// Extract additional args after '--'
		var additionalArgs []string
		argsLenAtDash := cmd.Flags().ArgsLenAtDash()
		if argsLenAtDash > 0 && len(args) > argsLenAtDash {
			additionalArgs = args[argsLenAtDash:]
		}

		// Resolve shorthand flags (resume doesn't create sessions, pass to claude CLI)
		permMode, err := resolvePermissionMode(cmd)
		if err != nil {
			return err
		}
		if permMode != "" {
			additionalArgs = append(additionalArgs, "--permission-mode", permMode)
		}

		fastEnabled, err := resolveFastMode(cmd)
		if err != nil {
			return err
		}
		if fastEnabled {
			additionalArgs = append(additionalArgs, "--model", "haiku", "--effort", "low")
		}

		// Load session
		sess, err := store.Get(name)
		if err != nil {
			return fmt.Errorf("session '%s' not found", name)
		}

		// Update lastAccessed timestamp
		sess.UpdateLastAccessed()
		if err := store.Update(sess); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		sessionDir := config.GetSessionDir(clotildeRoot, name)

		// Check for settings file
		var settingsFile string
		settingsPath := filepath.Join(sessionDir, "settings.json")
		if util.FileExists(settingsPath) {
			settingsFile = settingsPath
		}

		// Check for system prompt file
		var systemPromptFile string
		promptPath := filepath.Join(sessionDir, "system-prompt.md")
		if util.FileExists(promptPath) {
			systemPromptFile = promptPath
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resuming session '%s' (%s)\n\n", name, sess.Metadata.SessionID)

		// Invoke claude
		return claude.Resume(clotildeRoot, sess, settingsFile, systemPromptFile, additionalArgs)
	},
	}
	registerShorthandFlags(cmd)
	return cmd
}

// sortSessionsByLastAccessed sorts sessions by last accessed time (most recent first)
func sortSessionsByLastAccessed(sessions []*session.Session) {
	// Simple bubble sort - good enough for typical session counts
	for i := 0; i < len(sessions)-1; i++ {
		for j := 0; j < len(sessions)-i-1; j++ {
			if sessions[j].Metadata.LastAccessed.Before(sessions[j+1].Metadata.LastAccessed) {
				sessions[j], sessions[j+1] = sessions[j+1], sessions[j]
			}
		}
	}
}
