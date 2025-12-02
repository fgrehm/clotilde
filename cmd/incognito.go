package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

// newIncognitoCmd creates a fresh incognito command instance (avoids flag pollution in tests)
func newIncognitoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "incognito [name] [-- <claude-flags>...]",
		Short: "Start an incognito session (auto-deletes on exit)",
		Long: `Start a new incognito Claude Code session that automatically deletes
itself when you exit. Useful for quick queries, testing, or sensitive work.

If no name is provided, a random name like "happy-fox" will be generated.

The session and all associated data (transcripts, logs) will be permanently
deleted when you exit Claude.

Pass additional flags to Claude Code after '--':
  clotilde incognito              # Random name like "brave-wolf"
  clotilde incognito quick-test   # Explicit name
  clotilde incognito -- --debug api,hooks

Note: Incognito sessions are deleted on normal exit (Ctrl+D, /exit). If the
process crashes or is killed (SIGKILL), the session may persist. Use
'clotilde delete <name>' to clean up manually if needed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract additional args after '--'
			var additionalArgs []string
			argsLenAtDash := cmd.Flags().ArgsLenAtDash()
			if argsLenAtDash > 0 && len(args) > argsLenAtDash {
				additionalArgs = args[argsLenAtDash:]
			}

			// Generate or use provided name
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				// Generate a unique random name
				clotildeRoot, err := config.FindClotildeRoot()
				if err != nil {
					return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
				}
				store := session.NewFileStore(clotildeRoot)
				sessions, err := store.List()
				if err != nil {
					return fmt.Errorf("failed to list sessions: %w", err)
				}

				// Get existing names
				existingNames := make([]string, len(sessions))
				for i, sess := range sessions {
					existingNames[i] = sess.Name
				}

				name = util.GenerateUniqueRandomName(existingNames)
			}

			// Build params from flags (incognito doesn't have --incognito flag, so build manually)
			params := buildIncognitoParams(cmd, name)

			// Create the session
			result, err := createSession(params)
			if err != nil {
				return err
			}

			// Print output
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info(fmt.Sprintf("👻 Created incognito session '%s' (%s)", result.Session.Name, result.Session.Metadata.SessionID)))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info("👻 This session will auto-delete when you exit Claude"))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nStarting Claude Code...")

			// Invoke claude
			return claude.Start(result.ClotildeRoot, result.Session, result.SettingsFile, result.SystemPromptFile, additionalArgs)
		},
	}
	cmd.Flags().String("model", "", "Claude model to use (haiku, sonnet, opus)")
	cmd.Flags().String("append-system-prompt", "", "System prompt text to append")
	cmd.Flags().String("append-system-prompt-file", "", "Path to system prompt file to append")
	cmd.Flags().String("replace-system-prompt", "", "System prompt text to replace default (use instead of append)")
	cmd.Flags().String("replace-system-prompt-file", "", "Path to system prompt file to replace default (use instead of append)")

	// Permission flags
	cmd.Flags().String("permission-mode", "", "Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)")
	cmd.Flags().StringSlice("allowed-tools", nil, "Comma-separated list of allowed tools (e.g. 'Bash(npm:*),Read')")
	cmd.Flags().StringSlice("disallowed-tools", nil, "Comma-separated list of disallowed tools (e.g. 'Write,Bash(git:*)')")
	cmd.Flags().StringSlice("add-dir", nil, "Additional directories to allow tool access to")

	// Output style flags
	cmd.Flags().String("output-style", "", "Output style: 'default', 'Explanatory', 'Learning', or custom content")
	cmd.Flags().String("output-style-file", "", "Path to custom output style file")

	// Register flag completions
	_ = cmd.RegisterFlagCompletionFunc("model", modelCompletion)
	_ = cmd.RegisterFlagCompletionFunc("output-style", outputStyleCompletion)
	return cmd
}
