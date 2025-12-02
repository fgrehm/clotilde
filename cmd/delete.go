package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/outputstyle"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a session and its Claude Code data",
	Long: `Delete a session folder and associated Claude Code transcripts and logs.
This operation cannot be undone.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: sessionNameCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
		}

		// Create store
		store := session.NewFileStore(clotildeRoot)

		// Load session to verify it exists
		sess, err := store.Get(name)
		if err != nil {
			return fmt.Errorf("session '%s' not found", name)
		}

		// Get --force flag
		force, _ := cmd.Flags().GetBool("force")

		// Confirmation prompt unless --force
		if !force {
			// Check if we're in a TTY (interactive terminal)
			isTTY := isatty.IsTerminal(os.Stdout.Fd())

			if isTTY {
				// Use TUI confirmation dialog
				details := buildDeletionDetails(clotildeRoot, sess)

				confirmModel := ui.NewConfirm(
					fmt.Sprintf("Delete session '%s'?", name),
					"This will permanently delete:",
				).WithDetails(details).WithDestructive()

				confirmed, err := ui.RunConfirm(confirmModel)
				if err != nil {
					return fmt.Errorf("confirmation dialog failed: %w", err)
				}

				if !confirmed {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			} else {
				// Fallback to text prompt for non-TTY (scripts, pipes)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Delete session '%s' (%s)?\n", name, sess.Metadata.SessionID)
				_, _ = fmt.Fprint(cmd.OutOrStdout(), "This will delete the session folder and all Claude Code data. [y/N]: ")

				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}
		}

		// Track all deleted files for verbose output
		allDeletedFiles := &claude.DeletedFiles{
			Transcript: []string{},
			AgentLogs:  []string{},
		}

		// Delete Claude data for current session (transcript and agent logs)
		deleted, err := claude.DeleteSessionData(clotildeRoot, sess.Metadata.SessionID, sess.Metadata.TranscriptPath)
		if err != nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Warning(fmt.Sprintf("Failed to delete Claude data for current session: %v", err)))
		} else {
			allDeletedFiles.Transcript = append(allDeletedFiles.Transcript, deleted.Transcript...)
			allDeletedFiles.AgentLogs = append(allDeletedFiles.AgentLogs, deleted.AgentLogs...)
		}

		// Delete Claude data for previous sessions (from /clear operations, and defensively from /compact)
		for _, prevSessionID := range sess.Metadata.PreviousSessionIDs {
			deleted, err := claude.DeleteSessionData(clotildeRoot, prevSessionID, "")
			if err != nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Warning(fmt.Sprintf("Failed to delete Claude data for previous session %s: %v", prevSessionID, err)))
			} else {
				allDeletedFiles.Transcript = append(allDeletedFiles.Transcript, deleted.Transcript...)
				allDeletedFiles.AgentLogs = append(allDeletedFiles.AgentLogs, deleted.AgentLogs...)
			}
		}

		// Delete session folder
		if err := store.Delete(name); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		// Delete custom output style if it exists
		if sess.Metadata.HasCustomOutputStyle {
			if err := outputstyle.DeleteCustomStyleFile(clotildeRoot, name); err != nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Warning(fmt.Sprintf("Failed to delete output style file: %v", err)))
			}
		}

		// Show summary of what was deleted
		transcriptCount := len(allDeletedFiles.Transcript)
		agentLogCount := len(allDeletedFiles.AgentLogs)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Success(fmt.Sprintf("Deleted session '%s'", name)))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Session folder, %d transcript(s), %d agent log(s)\n", transcriptCount, agentLogCount)

		// Show detailed file paths in verbose mode
		if verbose {
			if transcriptCount > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n  Deleted transcripts:")
				for _, path := range allDeletedFiles.Transcript {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", path)
				}
			}
			if agentLogCount > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\n  Deleted agent logs:")
				for _, path := range allDeletedFiles.AgentLogs {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", path)
				}
			}
		}
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

// buildDeletionDetails builds a list of items that will be deleted
func buildDeletionDetails(clotildeRoot string, sess *session.Session) []string {
	var details []string

	// Session folder
	sessionDir := config.GetSessionDir(clotildeRoot, sess.Name)
	details = append(details, fmt.Sprintf("Session folder: %s", sessionDir))

	// Claude transcript
	if sess.Metadata.TranscriptPath != "" && util.FileExists(sess.Metadata.TranscriptPath) {
		fileInfo, err := os.Stat(sess.Metadata.TranscriptPath)
		if err == nil {
			size := fileInfo.Size()
			details = append(details, fmt.Sprintf("Claude transcript (%d KB)", size/1024))
		} else {
			details = append(details, "Claude transcript")
		}
	}

	// Previous session UUIDs (from /clear operations)
	if len(sess.Metadata.PreviousSessionIDs) > 0 {
		details = append(details, fmt.Sprintf("%d previous session transcript(s) (from /clear)", len(sess.Metadata.PreviousSessionIDs)))
	}

	// Agent logs
	if sess.Metadata.SessionID != "" {
		// Note: We can't easily count agent logs here without reading them,
		// so just mention they'll be cleaned up
		details = append(details, "Agent logs (if any)")
	}

	// Fork safety note
	if sess.Metadata.IsForkedSession {
		details = append(details, fmt.Sprintf("Note: Won't affect parent '%s'", sess.Metadata.ParentSession))
	}

	// Custom output style
	if sess.Metadata.HasCustomOutputStyle {
		details = append(details, "Custom output style file")
	}

	return details
}
