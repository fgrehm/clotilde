package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "stats [name]",
		Short:             "Show session activity statistics",
		Long:              `Display activity statistics for a session including turn count, timing, tokens, models, and tool usage.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: sessionNameCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")

			if all {
				return showAllStats(cmd)
			}

			if len(args) == 0 {
				return fmt.Errorf("session name required (or use --all for aggregate stats)")
			}

			return showSessionStats(cmd, args[0])
		},
	}

	cmd.Flags().Bool("all", false, "Show aggregate stats across sessions active in the last 7 days")

	return cmd
}

func showSessionStats(cmd *cobra.Command, name string) error {
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		return fmt.Errorf("no sessions found (create one with 'clotilde start <name>')")
	}

	store := session.NewFileStore(clotildeRoot)

	sess, err := store.Get(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found", name)
	}

	stats, err := collectSessionStats(sess, clotildeRoot)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session stats: %s\n", name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "─────────────────────────────────\n")

	printStats(cmd, stats)
	return nil
}

func showAllStats(cmd *cobra.Command) error {
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		return fmt.Errorf("no sessions found (create one with 'clotilde start <name>')")
	}

	store := session.NewFileStore(clotildeRoot)
	sessions, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
		return nil
	}

	// Filter to sessions active in the last 7 days
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var recent []*session.Session
	for _, sess := range sessions {
		if sess.Metadata.LastAccessed.After(cutoff) {
			recent = append(recent, sess)
		}
	}

	if len(recent) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions active in the last 7 days.")
		return nil
	}

	var allStats []*claude.TranscriptStats
	for _, sess := range recent {
		s, err := collectSessionStats(sess, clotildeRoot)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping session '%s': %v\n", sess.Name, err)
			continue
		}
		if s != nil {
			allStats = append(allStats, s)
		}
	}

	merged := claude.MergeTranscriptStats(allStats)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Aggregate stats (%d sessions, last 7 days)\n", len(recent))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "─────────────────────────────────\n")

	printStats(cmd, merged)
	return nil
}

func collectSessionStats(sess *session.Session, clotildeRoot string) (*claude.TranscriptStats, error) {
	homeDir, err := util.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	paths := allTranscriptPaths(sess, clotildeRoot, homeDir)

	var parsed []*claude.TranscriptStats
	for _, path := range paths {
		s, err := claude.ParseTranscriptStats(path)
		if err != nil {
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && os.IsNotExist(pathErr) {
				continue
			}
			return nil, fmt.Errorf("reading transcript %s: %w", path, err)
		}
		parsed = append(parsed, s)
	}

	if len(parsed) == 0 {
		return nil, nil
	}

	return claude.MergeTranscriptStats(parsed), nil
}

func printStats(cmd *cobra.Command, stats *claude.TranscriptStats) {
	w := cmd.OutOrStdout()

	if stats == nil || (stats.Turns == 0 && stats.FirstMessage.IsZero()) {
		_, _ = fmt.Fprintln(w, "No transcript found.")
		return
	}

	// Activity
	_, _ = fmt.Fprintf(w, "Turns         %d\n", stats.Turns)

	if !stats.FirstMessage.IsZero() {
		_, _ = fmt.Fprintf(w, "Started       %s\n", formatSessionDate(stats.FirstMessage))
	}

	if !stats.LastMessage.IsZero() {
		_, _ = fmt.Fprintf(w, "Last active   %s\n", formatSessionDate(stats.LastMessage))
	}

	if !stats.FirstMessage.IsZero() && !stats.LastMessage.IsZero() {
		_, _ = fmt.Fprintf(w, "Total time    %s\n", util.FormatDuration(stats.TotalTime))
	}

	if stats.Turns > 0 {
		_, _ = fmt.Fprintf(w, "Active time   %s     (approx)\n", util.FormatDuration(stats.ActiveTime))
		_, _ = fmt.Fprintf(w, "Avg response  %s      (approx)\n", util.FormatDuration(stats.AvgResponseTime))
	}

	// Tokens
	if stats.InputTokens > 0 || stats.OutputTokens > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "Input tokens  %s\n", formatTokenCount(stats.InputTokens))
		_, _ = fmt.Fprintf(w, "Output tokens %s\n", formatTokenCount(stats.OutputTokens))
		if stats.CacheReadTokens > 0 {
			_, _ = fmt.Fprintf(w, "Cache read    %s\n", formatTokenCount(stats.CacheReadTokens))
		}
		if stats.CacheCreationTokens > 0 {
			_, _ = fmt.Fprintf(w, "Cache write   %s\n", formatTokenCount(stats.CacheCreationTokens))
		}
	}

	// Models
	if len(stats.Models) > 0 {
		_, _ = fmt.Fprintln(w)
		families := make([]string, len(stats.Models))
		for i, m := range stats.Models {
			families[i] = claude.FormatModelFamily(m)
		}
		_, _ = fmt.Fprintf(w, "Models        %s\n", strings.Join(families, ", "))
	}

	// Tool usage
	if len(stats.ToolUses) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Tool usage:")
		printToolUses(w, stats.ToolUses)
	}
}

// printToolUses prints tool usage sorted by count (descending), then name.
func printToolUses(w interface{ Write([]byte) (int, error) }, toolUses map[string]int) {
	type toolCount struct {
		name  string
		count int
	}
	sorted := make([]toolCount, 0, len(toolUses))
	for name, count := range toolUses {
		sorted = append(sorted, toolCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].name < sorted[j].name
	})
	for _, tc := range sorted {
		_, _ = fmt.Fprintf(w, "  %-14s %d\n", tc.name, tc.count)
	}
}

// formatTokenCount formats a token count with "k" suffix for readability.
func formatTokenCount(count int) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	if count < 10000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	return fmt.Sprintf("%dk", count/1000)
}

// formatSessionDate formats a time as "Month Day, Year HH:MM"
func formatSessionDate(t time.Time) string {
	return t.Format("Jan 2, 2006 15:04")
}
