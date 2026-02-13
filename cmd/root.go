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
	Short:   "Named session management for Claude Code",
	Long:    `Clotilde wraps Claude Code with human-friendly session names, enabling easy switching between multiple parallel conversations.`,
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

	// Try to find clotilde root
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		// Not in a clotilde project - show help
		_, _ = fmt.Fprintln(os.Stderr, "Not in a clotilde project. Initialize with:")
		_, _ = fmt.Fprintln(os.Stderr, "  clotilde init")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_ = cmd.Help()
		return
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
		// TODO: Interactive prompt for session name (commit 24 in plan)
		fmt.Println("\nStarting new session...")
		fmt.Println("Run: clotilde start <session-name>")
		return true // Exit dashboard

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
		// TODO: Interactive prompts for parent and new session names (commit 24 in plan)
		fmt.Println("\nForking session...")
		fmt.Println("Run: clotilde fork <parent-session> <new-session>")
		return true // Exit dashboard

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
	// Create fresh init command with flag
	freshInitCmd := &cobra.Command{
		Use:   initCmd.Use,
		Short: initCmd.Short,
		Long:  initCmd.Long,
		RunE:  initCmd.RunE,
	}
	freshInitCmd.Flags().Bool("global", false, "Install hooks in .claude/settings.json (project-wide) instead of settings.local.json (local)")

	// Add all subcommands
	rootCmd.AddCommand(freshInitCmd)
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newIncognitoCmd())
	rootCmd.AddCommand(newResumeCmd())
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(newForkCmd())
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newCompletionCmd())

	// Add global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&claudeBinaryPath, "claude-bin", "", "Path to claude binary (hidden, for testing)")
	_ = rootCmd.PersistentFlags().MarkHidden("claude-bin")
}

// claudeBinaryPath is set via the --claude-bin flag (hidden, for testing)
var claudeBinaryPath string

// verbose is set via the --verbose/-v flag
var verbose bool

// NewRootCmd returns a new root command instance (useful for testing)
// This creates a fresh command tree to avoid flag pollution between tests
func NewRootCmd() *cobra.Command {
	// Create new root with same config
	root := &cobra.Command{
		Use:     rootCmd.Use,
		Short:   rootCmd.Short,
		Long:    rootCmd.Long,
		Version: rootCmd.Version,
	}

	// Disable Cobra's auto-generated completion command so we can use our custom one
	root.CompletionOptions.DisableDefaultCmd = true

	// Create fresh init command with flag
	freshInitCmd := &cobra.Command{
		Use:   initCmd.Use,
		Short: initCmd.Short,
		Long:  initCmd.Long,
		RunE:  initCmd.RunE,
	}
	freshInitCmd.Flags().Bool("global", false, "Install hooks in .claude/settings.json (project-wide) instead of settings.local.json (local)")

	// Add all subcommands (use factory functions to avoid flag pollution)
	root.AddCommand(freshInitCmd)
	root.AddCommand(newStartCmd())
	root.AddCommand(newIncognitoCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(listCmd)
	root.AddCommand(inspectCmd)
	root.AddCommand(newForkCmd())
	root.AddCommand(deleteCmd)
	root.AddCommand(hookCmd)
	root.AddCommand(versionCmd)
	root.AddCommand(newCompletionCmd())

	// Add global flags
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&claudeBinaryPath, "claude-bin", "", "Path to claude binary (hidden, for testing)")
	_ = root.PersistentFlags().MarkHidden("claude-bin")

	return root
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
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resumeSession resumes a session (extracted from resume command)
func resumeSession(clotildeRoot string, sess *session.Session, store session.Store) error {
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
