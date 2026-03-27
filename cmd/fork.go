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
	"github.com/fgrehm/clotilde/internal/outputstyle"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

// newForkCmd creates a fresh fork command instance (avoids flag pollution in tests)
func newForkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fork <parent-name> [fork-name] [-- <claude-flags>...]",
		Short: "Fork an existing session",
		Long: `Create a new session that branches from an existing one.
The fork inherits settings and system prompt from the parent.

If fork-name is not provided for incognito forks, a random name will be generated.

Pass additional flags to Claude Code after '--':
  clotilde fork my-session experiment -- --debug api,hooks
  clotilde fork my-session --incognito  # Random name like "happy-fox"`,
		Args:              rangePositionalArgs(1, 2),
		ValidArgsFunction: sessionNameCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			parentName := args[0]

			// Get incognito flag early to determine if we need a name
			incognito, _ := cmd.Flags().GetBool("incognito")

			// Find or create clotilde root
			clotildeRoot, err := config.FindOrCreateClotildeRoot()
			if err != nil {
				return fmt.Errorf("failed to initialize session storage: %w", err)
			}

			store := session.NewFileStore(clotildeRoot)

			var forkName string
			if len(args) >= 2 {
				forkName = args[1]
			} else {
				// Only allow missing name for incognito forks
				if !incognito {
					return fmt.Errorf("fork name required (or use --incognito for random name)")
				}

				sessions, err := store.List()
				if err != nil {
					return fmt.Errorf("failed to list sessions: %w", err)
				}

				existingNames := make([]string, len(sessions))
				for i, sess := range sessions {
					existingNames[i] = sess.Name
				}

				forkName = util.GenerateUniqueRandomName(existingNames)
			}

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
				additionalArgs = append(additionalArgs, "--permission-mode", permMode)
			}

			fastEnabled, err := resolveFastMode(cmd)
			if err != nil {
				return err
			}

			// Resolve model/effort from flags (persisted to settings.json below, not CLI args)
			var forkModel, forkEffort string
			if fastEnabled {
				forkModel = "haiku"
				forkEffort = "low"
			} else {
				effort, _ := cmd.Flags().GetString("effort")
				forkEffort = effort
			}

			if err := session.ValidateName(forkName); err != nil {
				return err
			}

			// Check if fork already exists
			if store.Exists(forkName) {
				return fmt.Errorf("session '%s' already exists", forkName)
			}

			// Load parent session
			parentSess, err := store.Get(parentName)
			if err != nil {
				return fmt.Errorf("parent session '%s' not found", parentName)
			}

			// Prevent forking FROM incognito sessions
			if parentSess.Metadata.IsIncognito {
				return fmt.Errorf("cannot fork from incognito session '%s' (it will auto-delete when you exit)", parentName)
			}

			// Create fork session with a pre-assigned UUID passed via --session-id
			forkUUID := util.GenerateUUID()
			var fork *session.Session
			if incognito {
				fork = session.NewIncognitoSession(forkName, forkUUID)
			} else {
				fork = session.NewSession(forkName, forkUUID)
			}
			fork.Metadata.IsForkedSession = true
			fork.Metadata.ParentSession = parentName
			fork.Metadata.SystemPromptMode = parentSess.Metadata.SystemPromptMode // Inherit from parent

			// Set context: use --context flag if provided, otherwise inherit from parent
			forkContext, _ := cmd.Flags().GetString("context")
			if forkContext != "" {
				fork.Metadata.Context = forkContext
			} else if parentSess.Metadata.Context != "" {
				fork.Metadata.Context = parentSess.Metadata.Context
			}

			if err := store.Create(fork); err != nil {
				return fmt.Errorf("failed to create fork: %w", err)
			}

			forkDir := config.GetSessionDir(clotildeRoot, forkName)
			parentDir := config.GetSessionDir(clotildeRoot, parentName)

			// Copy settings.json and handle custom output style inheritance
			parentSettingsPath := filepath.Join(parentDir, "settings.json")
			if util.FileExists(parentSettingsPath) {
				forkSettingsPath := filepath.Join(forkDir, "settings.json")
				if err := util.CopyFile(parentSettingsPath, forkSettingsPath); err != nil {
					return fmt.Errorf("failed to copy settings: %w", err)
				}

				// Check for custom output style that needs its own copy
				parentSettingsData, err := os.ReadFile(parentSettingsPath)
				if err == nil {
					var parsedSettings session.Settings
					if err := json.Unmarshal(parentSettingsData, &parsedSettings); err == nil {
						if parsedSettings.OutputStyle != "" && strings.HasPrefix(parsedSettings.OutputStyle, "clotilde/") {
							parentStyleName := strings.TrimPrefix(parsedSettings.OutputStyle, "clotilde/")
							parentStylePath := outputstyle.GetCustomStylePath(clotildeRoot, parentStyleName)
							if util.FileExists(parentStylePath) {
								styleContent, err := os.ReadFile(parentStylePath)
								if err == nil {
									content := string(styleContent)
									parts := strings.SplitN(content, "---", 3)
									if len(parts) == 3 {
										content = strings.TrimSpace(parts[2])
									}

									if err := outputstyle.CreateCustomStyleFile(clotildeRoot, forkName, content); err != nil {
										return fmt.Errorf("failed to copy custom output style: %w", err)
									}

									// Update the already-copied settings to reference the fork's style
									parsedSettings.OutputStyle = outputstyle.GetCustomStyleReference(forkName)
									updatedData, err := json.MarshalIndent(parsedSettings, "", "  ")
									if err != nil {
										return fmt.Errorf("failed to marshal fork settings: %w", err)
									}
									if err := os.WriteFile(forkSettingsPath, updatedData, 0o644); err != nil {
										return fmt.Errorf("failed to write fork settings: %w", err)
									}

									fork.Metadata.HasCustomOutputStyle = true
									if err := store.Update(fork); err != nil {
										return fmt.Errorf("failed to update fork metadata: %w", err)
									}
								}
							}
						}
					}
				}
			}

			// Apply model/effort overrides to fork settings.json (sticky, not CLI args)
			if forkModel != "" || forkEffort != "" {
				settings, err := store.LoadSettings(forkName)
				if err != nil {
					return fmt.Errorf("failed to load fork settings: %w", err)
				}
				if settings == nil {
					settings = &session.Settings{}
				}
				if forkModel != "" {
					settings.Model = forkModel
				}
				if forkEffort != "" {
					settings.EffortLevel = forkEffort
				}
				if err := store.SaveSettings(forkName, settings); err != nil {
					return fmt.Errorf("failed to save fork settings: %w", err)
				}
			}

			// Copy system-prompt.md if exists
			parentPrompt := filepath.Join(parentDir, "system-prompt.md")
			if util.FileExists(parentPrompt) {
				forkPrompt := filepath.Join(forkDir, "system-prompt.md")
				if err := util.CopyFile(parentPrompt, forkPrompt); err != nil {
					return fmt.Errorf("failed to copy system prompt: %w", err)
				}
			}

			if incognito {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info(fmt.Sprintf("👻 Created incognito fork '%s' from '%s'", forkName, parentName)))
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Info("👻 This fork will auto-delete when you exit Claude"))
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success(fmt.Sprintf("Created fork '%s' from '%s'", forkName, parentName)))
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nStarting Claude Code with fork...")

			// Build file paths for claude invocation
			var settingsFile, systemPromptFile string
			if util.FileExists(filepath.Join(forkDir, "settings.json")) {
				settingsFile = filepath.Join(forkDir, "settings.json")
			}
			if util.FileExists(filepath.Join(forkDir, "system-prompt.md")) {
				systemPromptFile = filepath.Join(forkDir, "system-prompt.md")
			}

			// Invoke claude with fork (pass fork session for cleanup handling)
			return claude.Fork(clotildeRoot, parentSess, forkName, settingsFile, systemPromptFile, additionalArgs, fork)
		},
	}
	cmd.Flags().Bool("incognito", false, "Create fork as incognito session (auto-deletes on exit)")
	cmd.Flags().String("context", "", "Session context (e.g. \"working on ticket GH-123\")")
	registerShorthandFlags(cmd)
	return cmd
}
