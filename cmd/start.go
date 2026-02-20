package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
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

			// Check if session already exists - offer to resume instead
			if clotildeRoot, err := config.FindClotildeRoot(); err == nil {
				store := session.NewFileStore(clotildeRoot)
				if store.Exists(args[0]) {
					return handleExistingSession(cmd, args[0], clotildeRoot, store, additionalArgs)
				}
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
	cmd.Flags().String("context", "", "Session context (e.g. \"working on ticket GH-123\")")
	cmd.Flags().String("profile", "", "Named profile from config (model, permissions, output style)")

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
	_ = cmd.RegisterFlagCompletionFunc("profile", profileNameCompletion)
	_ = cmd.RegisterFlagCompletionFunc("output-style", outputStyleCompletion)
	return cmd
}

// handleExistingSession prompts the user to resume an existing session instead of
// creating a duplicate. In non-TTY mode, returns an error suggesting the resume command.
func handleExistingSession(cmd *cobra.Command, name, clotildeRoot string, store *session.FileStore, additionalArgs []string) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("session '%s' already exists, use 'clotilde resume %s' to resume it", name, name)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", ui.Warning(fmt.Sprintf("Session '%s' already exists.", name)))
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Would you like to resume it? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		return nil
	}

	// Resolve shorthand flags for resume (pass as additional args, not baked into settings)
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

	// Load and resume session
	sess, err := store.Get(name)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	sess.UpdateLastAccessed()
	if err := store.Update(sess); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	sessionDir := config.GetSessionDir(clotildeRoot, name)

	var settingsFile string
	if util.FileExists(filepath.Join(sessionDir, "settings.json")) {
		settingsFile = filepath.Join(sessionDir, "settings.json")
	}

	var systemPromptFile string
	if util.FileExists(filepath.Join(sessionDir, "system-prompt.md")) {
		systemPromptFile = filepath.Join(sessionDir, "system-prompt.md")
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nResuming session '%s' (%s)\n\n", sess.Name, sess.Metadata.SessionID)
	return claude.Resume(clotildeRoot, sess, settingsFile, systemPromptFile, additionalArgs)
}
