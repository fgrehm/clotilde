package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/ui"
)

func init() {
	// Wire up the claude binary path function and verbose flag
	claude.ClaudeBinaryPathFunc = GetClaudeBinaryPath
	claude.VerboseFunc = IsVerbose
}

// newStartCmd creates a fresh start command instance (avoids flag pollution in tests)
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <name> [-- <claude-flags>...]",
		Short: "Start a new named session",
		Long: `Start a new Claude Code session with a human-friendly name.
Optionally specify a model and system prompt.

Pass additional flags to Claude Code after '--':
  clotilde start my-session -- --debug api,hooks
  clotilde start test --model haiku -- --verbose`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract additional args after '--'
			var additionalArgs []string
			argsLenAtDash := cmd.Flags().ArgsLenAtDash()
			if argsLenAtDash > 0 && len(args) > argsLenAtDash {
				additionalArgs = args[argsLenAtDash:]
			}

			// Resolve shorthand flags
			permMode, err := resolvePermissionMode(cmd)
			if err != nil {
				return err
			}
			if permMode != "" {
				_ = cmd.Flags().Set("permission-mode", permMode)
			}

			fastEnabled, err := resolveFastMode(cmd)
			if err != nil {
				return err
			}
			if fastEnabled {
				_ = cmd.Flags().Set("model", "haiku")
				additionalArgs = append(additionalArgs, "--effort", "low")
			}

			// Build params from flags
			params, err := buildSessionCreateParams(cmd, args[0])
			if err != nil {
				return err
			}

			// Create the session
			result, err := createSession(params)
			if err != nil {
				return err
			}

			// Print output
			if params.Incognito {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info(fmt.Sprintf("👻 Created incognito session '%s' (%s)", result.Session.Name, result.Session.Metadata.SessionID)))
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info("👻 This session will auto-delete when you exit Claude"))
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success(fmt.Sprintf("Created session '%s' (%s)", result.Session.Name, result.Session.Metadata.SessionID)))
			}
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
	cmd.Flags().Bool("incognito", false, "Create incognito session (auto-deletes on exit)")

	// Permission flags
	cmd.Flags().String("permission-mode", "", "Permission mode (acceptEdits, bypassPermissions, default, dontAsk, plan)")
	cmd.Flags().StringSlice("allowed-tools", nil, "Comma-separated list of allowed tools (e.g. 'Bash(npm:*),Read')")
	cmd.Flags().StringSlice("disallowed-tools", nil, "Comma-separated list of disallowed tools (e.g. 'Write,Bash(git:*)')")
	cmd.Flags().StringSlice("add-dir", nil, "Additional directories to allow tool access to")

	// Output style flags
	cmd.Flags().String("output-style", "", "Output style: 'default', 'Explanatory', 'Learning', or custom content")
	cmd.Flags().String("output-style-file", "", "Path to custom output style file")

	// Shorthand flags
	registerShorthandFlags(cmd)

	// Register flag completions
	_ = cmd.RegisterFlagCompletionFunc("model", modelCompletion)
	_ = cmd.RegisterFlagCompletionFunc("output-style", outputStyleCompletion)
	return cmd
}
