package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/outputstyle"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

var rootCmd = &cobra.Command{
	Use:     "clotilde",
	Short:   "Named sessions, profiles, and context management for Claude Code",
	Long:    `Clotilde wraps Claude Code with human-friendly session names, profiles, and context management, enabling easy switching between multiple parallel conversations.`,
	Version: version,
	Run:     runDashboard,
}

// runDashboard shows the interactive dashboard when no subcommand is provided
func runDashboard(cmd *cobra.Command, args []string) {
	// Check if in TTY (interactive terminal)
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	if !isTTY {
		// Non-interactive mode: show help
		_ = cmd.Help()
		return
	}

	// Find or create clotilde root (dashboard always works)
	clotildeRoot, err := config.FindOrCreateClotildeRoot()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize session storage: %v\n", err)
		os.Exit(1)
	}

	// Load sessions
	store := session.NewFileStore(clotildeRoot)
	sessions, err := store.List()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to load sessions: %v\n", err)
		os.Exit(1)
	}

	// Sort by last accessed (most recent first)
	sortSessionsByLastAccessed(sessions)

	// Dashboard loop - keep showing dashboard until quit or session launched
	for {
		// Reload sessions each loop iteration (in case they were modified)
		sessions, err = store.List()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to load sessions: %v\n", err)
			os.Exit(1)
		}
		sortSessionsByLastAccessed(sessions)

		// Show dashboard
		dashboard := ui.NewDashboard(sessions)
		selectedAction, err := ui.RunDashboard(dashboard)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Dashboard error: %v\n", err)
			os.Exit(1)
		}

		// Handle cancelled (user pressed q/esc)
		if selectedAction == "" {
			return
		}

		// Dispatch to appropriate command based on selection
		shouldReturn := handleDashboardAction(selectedAction, sessions, clotildeRoot, store)
		if shouldReturn {
			return
		}
		// Otherwise loop back to dashboard
	}
}

// handleDashboardAction handles a dashboard action and returns true if we should exit
func handleDashboardAction(selectedAction string, sessions []*session.Session, clotildeRoot string, store session.Store) bool {
	switch selectedAction {
	case "start":
		// Auto-generate a session name and start immediately
		existingNames := make([]string, len(sessions))
		for i, sess := range sessions {
			existingNames[i] = sess.Name
		}
		name := util.GenerateUniqueRandomName(existingNames)

		result, err := createSession(SessionCreateParams{Name: name})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Created session '%s' (%s)", result.Session.Name, result.Session.Metadata.SessionID)))
		fmt.Println("\nStarting Claude Code...")

		if err := claude.Start(result.ClotildeRoot, result.Session, result.SettingsFile, result.SystemPromptFile, nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to start session: %v\n", err)
			os.Exit(1)
		}
		return true

	case "resume":
		// Show picker to select session
		if len(sessions) == 0 {
			fmt.Println("No sessions available to resume.")
			return false // Stay in dashboard
		}

		picker := ui.NewPicker(sessions, "Select session to resume").WithPreview()
		selected, err := ui.RunPicker(picker)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Picker failed: %v\n", err)
			os.Exit(1)
		}

		if selected == nil {
			// Cancelled - go back to dashboard
			return false
		}

		// Update last accessed
		selected.UpdateLastAccessed()
		if err := store.Update(selected); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to update session: %v\n", err)
			os.Exit(1)
		}

		// Resume the session (reuse logic from resume command)
		if err := resumeSession(clotildeRoot, selected, store); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to resume session: %v\n", err)
			os.Exit(1)
		}

		// After resuming (launching Claude), exit dashboard
		return true

	case "fork":
		if len(sessions) == 0 {
			fmt.Println("No sessions available to fork.")
			return false
		}

		// Filter out incognito sessions (can't fork from them)
		var forkable []*session.Session
		for _, s := range sessions {
			if !s.Metadata.IsIncognito {
				forkable = append(forkable, s)
			}
		}
		if len(forkable) == 0 {
			fmt.Println("No non-incognito sessions available to fork.")
			return false
		}

		picker := ui.NewPicker(forkable, "Select session to fork").WithPreview()
		parent, err := ui.RunPicker(picker)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Picker failed: %v\n", err)
			os.Exit(1)
		}
		if parent == nil {
			return false
		}

		if err := forkFromDashboard(clotildeRoot, parent, sessions, store); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to fork session: %v\n", err)
			os.Exit(1)
		}
		return true

	case "list":
		// Show interactive table
		selected, err := showInteractiveTable(sessions, store)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to show table: %v\n", err)
			os.Exit(1)
		}

		// If no session selected (cancelled), go back to dashboard
		if selected == nil {
			return false
		}

		// Update last accessed
		selected.UpdateLastAccessed()
		if err := store.Update(selected); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to update session: %v\n", err)
			os.Exit(1)
		}

		// Resume the selected session
		if err := resumeSession(clotildeRoot, selected, store); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to resume session: %v\n", err)
			os.Exit(1)
		}

		// After resuming (launching Claude), exit dashboard
		return true

	case "delete":
		// Show picker to select session
		if len(sessions) == 0 {
			fmt.Println("No sessions available to delete.")
			return false // Stay in dashboard
		}

		picker := ui.NewPicker(sessions, "Select session to delete").WithPreview()
		selected, err := ui.RunPicker(picker)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Picker failed: %v\n", err)
			os.Exit(1)
		}

		if selected == nil {
			// Cancelled - go back to dashboard
			return false
		}

		// Show confirmation with details
		details := buildDeletionDetails(clotildeRoot, selected)
		confirmModel := ui.NewConfirm(
			fmt.Sprintf("Delete session '%s'?", selected.Name),
			"This will permanently delete:",
		).WithDetails(details).WithDestructive()

		confirmed, err := ui.RunConfirm(confirmModel)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Confirmation dialog failed: %v\n", err)
			os.Exit(1)
		}

		if !confirmed {
			// Cancelled - go back to dashboard
			return false
		}

		// Delete the session (reuse logic from delete command)
		if err := deleteSession(clotildeRoot, selected, store); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to delete session: %v\n", err)
			os.Exit(1)
		}

		// After deleting, go back to dashboard
		return false

	case "quit":
		// User explicitly selected quit - exit dashboard
		return true

	default:
		// Unknown action - stay in dashboard
		return false
	}
}

func init() {
	// Disable Cobra's auto-generated completion command so we can use our custom one
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Initialize global rootCmd with all subcommands
	initRootCmd()
}

// initRootCmd initializes the global rootCmd with all subcommands
func initRootCmd() {
	registerSubcommands(rootCmd)
}

// claudeBinaryPath is set via the --claude-bin flag (hidden, for testing)
var claudeBinaryPath string

// verbose is set via the --verbose/-v flag
var verbose bool

// NewRootCmd returns a new root command instance (useful for testing).
// Creates a fresh command tree to avoid flag pollution between tests.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     rootCmd.Use,
		Short:   rootCmd.Short,
		Long:    rootCmd.Long,
		Version: rootCmd.Version,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	registerSubcommands(root)
	return root
}

// registerSubcommands adds all subcommands and global flags to the given root command.
func registerSubcommands(root *cobra.Command) {
	freshInitCmd := &cobra.Command{
		Use:   initCmd.Use,
		Short: initCmd.Short,
		Long:  initCmd.Long,
		RunE:  initCmd.RunE,
	}
	freshInitCmd.Flags().Bool("global", false, "Install hooks in .claude/settings.json (project-wide) instead of settings.local.json (local)")

	root.AddCommand(freshInitCmd)
	root.AddCommand(newSetupCmd())
	root.AddCommand(newStartCmd())
	root.AddCommand(newIncognitoCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(listCmd)
	root.AddCommand(inspectCmd)
	root.AddCommand(newStatsCmd())
	root.AddCommand(newForkCmd())
	root.AddCommand(deleteCmd)
	root.AddCommand(newExportCmd())
	root.AddCommand(hookCmd)
	root.AddCommand(newTourCmd())
	root.AddCommand(versionCmd)
	root.AddCommand(newCompletionCmd())

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&claudeBinaryPath, "claude-bin", "", "Path to claude binary (hidden, for testing)")
	_ = root.PersistentFlags().MarkHidden("claude-bin")
}

// GetClaudeBinaryPath returns the path to the claude binary.
// If --claude-bin flag is set, returns that path. Otherwise returns "claude".
func GetClaudeBinaryPath() string {
	if claudeBinaryPath != "" {
		return claudeBinaryPath
	}
	return "claude"
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resumeSession resumes a session (extracted from resume command)
func resumeSession(clotildeRoot string, sess *session.Session, _ session.Store) error {
	sessionDir := config.GetSessionDir(clotildeRoot, sess.Name)

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

	fmt.Printf("Resuming session '%s' (%s)\n\n", sess.Name, sess.Metadata.SessionID)

	// Invoke claude
	return claude.Resume(clotildeRoot, sess, settingsFile, systemPromptFile, nil)
}

// deleteSession deletes a session (extracted from delete command)
func deleteSession(clotildeRoot string, sess *session.Session, store session.Store) error {
	// Track all deleted files for verbose output
	allDeletedFiles := &claude.DeletedFiles{
		Transcript: []string{},
		AgentLogs:  []string{},
	}

	// Delete Claude data for current session (transcript and agent logs)
	deleted, err := claude.DeleteSessionData(clotildeRoot, sess.Metadata.SessionID, sess.Metadata.TranscriptPath)
	if err != nil {
		fmt.Println(ui.Warning(fmt.Sprintf("Failed to delete Claude data for current session: %v", err)))
	} else {
		allDeletedFiles.Transcript = append(allDeletedFiles.Transcript, deleted.Transcript...)
		allDeletedFiles.AgentLogs = append(allDeletedFiles.AgentLogs, deleted.AgentLogs...)
	}

	// Delete Claude data for previous sessions (from /clear operations, and defensively from /compact)
	for _, prevSessionID := range sess.Metadata.PreviousSessionIDs {
		deleted, err := claude.DeleteSessionData(clotildeRoot, prevSessionID, "")
		if err != nil {
			fmt.Println(ui.Warning(fmt.Sprintf("Failed to delete Claude data for previous session %s: %v", prevSessionID, err)))
		} else {
			allDeletedFiles.Transcript = append(allDeletedFiles.Transcript, deleted.Transcript...)
			allDeletedFiles.AgentLogs = append(allDeletedFiles.AgentLogs, deleted.AgentLogs...)
		}
	}

	// Delete session folder
	if err := store.Delete(sess.Name); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Delete custom output style if it exists
	if sess.Metadata.HasCustomOutputStyle {
		if err := outputstyle.DeleteCustomStyleFile(clotildeRoot, sess.Name); err != nil {
			fmt.Println(ui.Warning(fmt.Sprintf("Failed to delete output style file: %v", err)))
		}
	}

	// Show summary of what was deleted
	transcriptCount := len(allDeletedFiles.Transcript)
	agentLogCount := len(allDeletedFiles.AgentLogs)
	fmt.Println(ui.Success(fmt.Sprintf("Deleted session '%s'", sess.Name)))
	fmt.Printf("  Session folder, %d transcript(s), %d agent log(s)\n", transcriptCount, agentLogCount)

	// Show detailed file paths in verbose mode
	if verbose {
		if transcriptCount > 0 {
			fmt.Println("\n  Deleted transcripts:")
			for _, path := range allDeletedFiles.Transcript {
				fmt.Printf("    %s\n", path)
			}
		}
		if agentLogCount > 0 {
			fmt.Println("\n  Deleted agent logs:")
			for _, path := range allDeletedFiles.AgentLogs {
				fmt.Printf("    %s\n", path)
			}
		}
	}

	return nil
}

// forkFromDashboard creates a fork with an auto-generated name and launches Claude
func forkFromDashboard(clotildeRoot string, parent *session.Session, sessions []*session.Session, store session.Store) error {
	existingNames := make([]string, len(sessions))
	for i, s := range sessions {
		existingNames[i] = s.Name
	}
	forkName := util.GenerateUniqueRandomName(existingNames)

	// Create fork session with empty sessionId (filled by hook)
	fork := session.NewSession(forkName, "")
	fork.Metadata.IsForkedSession = true
	fork.Metadata.ParentSession = parent.Name
	fork.Metadata.SystemPromptMode = parent.Metadata.SystemPromptMode
	fork.Metadata.Context = parent.Metadata.Context

	if err := store.Create(fork); err != nil {
		return fmt.Errorf("failed to create fork: %w", err)
	}

	forkDir := config.GetSessionDir(clotildeRoot, forkName)
	parentDir := config.GetSessionDir(clotildeRoot, parent.Name)

	// Copy settings.json if exists
	parentSettings := filepath.Join(parentDir, "settings.json")
	if util.FileExists(parentSettings) {
		if err := util.CopyFile(parentSettings, filepath.Join(forkDir, "settings.json")); err != nil {
			return fmt.Errorf("failed to copy settings: %w", err)
		}
	}

	// Copy system-prompt.md if exists
	parentPrompt := filepath.Join(parentDir, "system-prompt.md")
	if util.FileExists(parentPrompt) {
		if err := util.CopyFile(parentPrompt, filepath.Join(forkDir, "system-prompt.md")); err != nil {
			return fmt.Errorf("failed to copy system prompt: %w", err)
		}
	}

	fmt.Println(ui.Success(fmt.Sprintf("Created fork '%s' from '%s'", forkName, parent.Name)))
	fmt.Println("\nStarting Claude Code with fork...")

	var settingsFile, systemPromptFile string
	if util.FileExists(filepath.Join(forkDir, "settings.json")) {
		settingsFile = filepath.Join(forkDir, "settings.json")
	}
	if util.FileExists(filepath.Join(forkDir, "system-prompt.md")) {
		systemPromptFile = filepath.Join(forkDir, "system-prompt.md")
	}

	return claude.Fork(clotildeRoot, parent, forkName, settingsFile, systemPromptFile, nil, fork)
}
