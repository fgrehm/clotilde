package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var statsCmd = &cobra.Command{
	Use:               "stats <name>",
	Short:             "Show session activity statistics",
	Long:              `Display activity statistics for a session including turn count, timing, and response times.`,
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

		// Load session
		sess, err := store.Get(name)
		if err != nil {
			return fmt.Errorf("session '%s' not found", name)
		}

		// Determine transcript path
		transcriptPath := sess.Metadata.TranscriptPath
		if transcriptPath == "" {
			// Fall back to computing the path
			homeDir, err := util.HomeDir()
			if err == nil {
				projectDir := claude.ProjectDir(clotildeRoot)
				claudeProjectDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
				transcriptPath = filepath.Join(claudeProjectDir, sess.Metadata.SessionID+".jsonl")
			}
		}

		// Parse transcript stats
		var stats *claude.TranscriptStats
		if transcriptPath != "" {
			stats, _ = claude.ParseTranscriptStats(transcriptPath)
		}

		// Print header
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session stats: %s\n", name)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "─────────────────────────────────\n")

		if stats == nil || (stats.Turns == 0 && stats.FirstMessage.IsZero()) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No transcript found.\n")
			return nil
		}

		// Print turns
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Turns         %d\n", stats.Turns)

		// Print started time if available
		if !stats.FirstMessage.IsZero() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Started       %s\n", formatSessionDate(stats.FirstMessage))
		}

		// Print last active time if available
		if !stats.LastMessage.IsZero() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last active   %s\n", formatSessionDate(stats.LastMessage))
		}

		// Print total time if available
		if !stats.FirstMessage.IsZero() && !stats.LastMessage.IsZero() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Total time    %s\n", util.FormatDuration(stats.TotalTime))
		}

		// Print active and average times only if there are turns
		if stats.Turns > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Active time   %s     (approx)\n", util.FormatDuration(stats.ActiveTime))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Avg response  %s      (approx)\n", util.FormatDuration(stats.AvgResponseTime))
		}

		return nil
	},
}

// formatSessionDate formats a time as "Month Day, Year HH:MM"
// Example: "Feb 17, 2025 20:35"
func formatSessionDate(t time.Time) string {
	return t.Format("Jan 2, 2006 15:04")
}
