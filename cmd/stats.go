package cmd

import (
	"errors"
	"fmt"
	"os"
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
			return fmt.Errorf("no sessions found (create one with 'clotilde start <name>')")
		}

		// Create store
		store := session.NewFileStore(clotildeRoot)

		// Load session
		sess, err := store.Get(name)
		if err != nil {
			return fmt.Errorf("session '%s' not found", name)
		}

		// Collect stats across all transcripts (previous + current)
		homeDir, err := util.HomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		paths := allTranscriptPaths(sess, clotildeRoot, homeDir)

		var stats *claude.TranscriptStats
		for _, path := range paths {
			s, err := claude.ParseTranscriptStats(path)
			if err != nil {
				var pathErr *os.PathError
				if errors.As(err, &pathErr) && os.IsNotExist(pathErr) {
					continue // previous transcript deleted or not yet written
				}
				return fmt.Errorf("reading transcript %s: %w", path, err)
			}
			stats = mergeTranscriptStats(stats, s)
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

// mergeTranscriptStats merges b into a, accumulating turns/times across multiple transcripts.
// Either argument may be nil (treated as empty). Returns a new merged value.
func mergeTranscriptStats(a, b *claude.TranscriptStats) *claude.TranscriptStats {
	merged := &claude.TranscriptStats{}

	for _, s := range []*claude.TranscriptStats{a, b} {
		if s == nil {
			continue
		}
		merged.Turns += s.Turns
		merged.ActiveTime += s.ActiveTime

		if !s.FirstMessage.IsZero() && (merged.FirstMessage.IsZero() || s.FirstMessage.Before(merged.FirstMessage)) {
			merged.FirstMessage = s.FirstMessage
		}
		if s.LastMessage.After(merged.LastMessage) {
			merged.LastMessage = s.LastMessage
		}
	}

	if !merged.FirstMessage.IsZero() && !merged.LastMessage.IsZero() {
		merged.TotalTime = merged.LastMessage.Sub(merged.FirstMessage)
	}
	if merged.Turns > 0 {
		merged.AvgResponseTime = merged.ActiveTime / time.Duration(merged.Turns)
	}

	return merged
}

// formatSessionDate formats a time as "Month Day, Year HH:MM"
// Example: "Feb 17, 2025 20:35"
func formatSessionDate(t time.Time) string {
	return t.Format("Jan 2, 2006 15:04")
}
