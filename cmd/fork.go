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
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: sessionNameCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			parentName := args[0]

			// Get incognito flag early to determine if we need a name
			incognito, _ := cmd.Flags().GetBool("incognito")

			var forkName string
			if len(args) >= 2 {
				forkName = args[1]
			} else {
				// Only allow missing name for incognito forks
				if !incognito {
					return fmt.Errorf("fork name required (or use --incognito for random name)")
				}

				// Generate unique random name
				clotildeRoot, err := config.FindClotildeRoot()
				if err != nil {
					return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
				}
				store := session.NewFileStore(clotildeRoot)
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

			// Resolve shorthand flags (fork doesn't create sessions, pass to claude CLI)
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

			// Find clotilde root
			clotildeRoot, err := config.FindClotildeRoot()
			if err != nil {
				return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
			}

			// Validate fork name
			if err := session.ValidateName(forkName); err != nil {
				return err
			}

			// Create store
			store := session.NewFileStore(clotildeRoot)

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

			// Create fork session with empty sessionId (will be filled by hook)
			var fork *session.Session
			if incognito {
				fork = session.NewIncognitoSession(forkName, "")
			} else {
				fork = session.NewSession(forkName, "")
			}
			fork.Metadata.IsForkedSession = true
			fork.Metadata.ParentSession = parentName
			fork.Metadata.SystemPromptMode = parentSess.Metadata.SystemPromptMode // Inherit from parent

			if err := store.Create(fork); err != nil {
				return fmt.Errorf("failed to create fork: %w", err)
			}

			forkDir := config.GetSessionDir(clotildeRoot, forkName)
			parentDir := config.GetSessionDir(clotildeRoot, parentName)

			// Copy settings.json if exists
			parentSettings := filepath.Join(parentDir, "settings.json")
			if util.FileExists(parentSettings) {
				forkSettings := filepath.Join(forkDir, "settings.json")
				if err := util.CopyFile(parentSettings, forkSettings); err != nil {
					return fmt.Errorf("failed to copy settings: %w", err)
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

			// Handle custom output style inheritance
			parentSettingsPath := filepath.Join(parentDir, "settings.json")
			if util.FileExists(parentSettingsPath) {
				parentSettingsData, err := os.ReadFile(parentSettingsPath)
				if err == nil {
					var parentSettings session.Settings
					if err := json.Unmarshal(parentSettingsData, &parentSettings); err == nil {
						// Check if parent has custom output style (starts with "clotilde/")
						if parentSettings.OutputStyle != "" && strings.HasPrefix(parentSettings.OutputStyle, "clotilde/") {
							// Extract parent session name from style reference
							parentStyleName := strings.TrimPrefix(parentSettings.OutputStyle, "clotilde/")

							// Read parent's custom style file
							parentStylePath := outputstyle.GetCustomStylePath(clotildeRoot, parentStyleName)
							if util.FileExists(parentStylePath) {
								styleContent, err := os.ReadFile(parentStylePath)
								if err == nil {
									// Extract just the content (skip frontmatter from parent)
									content := string(styleContent)
									// Simple frontmatter skip (find second "---" and take everything after)
									parts := strings.SplitN(content, "---", 3)
									if len(parts) == 3 {
										content = strings.TrimSpace(parts[2])
									}

									// Create custom style for fork
									if err := outputstyle.CreateCustomStyleFile(clotildeRoot, forkName, content); err != nil {
										return fmt.Errorf("failed to copy custom output style: %w", err)
									}

									// Update fork's settings.json to reference new style
									forkSettingsPath := filepath.Join(forkDir, "settings.json")
									forkSettingsData, _ := os.ReadFile(forkSettingsPath)
									var forkSettings session.Settings
									if err := json.Unmarshal(forkSettingsData, &forkSettings); err == nil {
										forkSettings.OutputStyle = outputstyle.GetCustomStyleReference(forkName)
										updatedData, _ := json.MarshalIndent(forkSettings, "", "  ")
										_ = os.WriteFile(forkSettingsPath, updatedData, 0o644)
									}

									// Update fork metadata
									fork.Metadata.HasCustomOutputStyle = true
									_ = store.Update(fork)
								}
							}
						}
					}
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
	registerShorthandFlags(cmd)
	return cmd
}
