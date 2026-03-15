package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		Use:   "stats [name]",
		Short: "Show session activity statistics",
		Long: `Display activity statistics for a session including turn count, timing,
tokens, models, and tool usage.

With --all, reads from daily JSONL stats files recorded by the SessionEnd hook
(enable with 'clotilde setup --stats'). Falls back to parsing transcripts if
no stats files exist. The JSONL files at $XDG_DATA_HOME/clotilde/stats/ are
also designed for consumption by other tools (dashboards, scripts, etc.).`,
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
	cmd.AddCommand(newStatsBackfillCmd())

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
	now := time.Now()

	// Resolve project path for filtering (empty if not in a project)
	var projectPath string
	if root, err := config.FindClotildeRoot(); err == nil {
		projectPath = filepath.Dir(filepath.Dir(root))
	}

	// Try reading from daily JSONL stats files first
	records, err := readStatsForPeriod(now, 7)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to read stats files: %v\n", err)
	}

	// Filter to current project (keep records with matching or empty project path)
	if projectPath != "" && len(records) > 0 {
		var filtered []claude.SessionStatsRecord
		for _, rec := range records {
			if rec.ProjectPath == "" || rec.ProjectPath == projectPath {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	if len(records) > 0 {
		return showAggregateFromRecords(cmd, records)
	}

	// Fall back to transcript parsing
	return showAggregateFromTranscripts(cmd)
}

// showAggregateFromRecords aggregates stats from daily JSONL records.
func showAggregateFromRecords(cmd *cobra.Command, records []claude.SessionStatsRecord) error {
	// Deduplicate: keep last record per session_id (most recent cumulative totals)
	seen := make(map[string]int)
	var deduped []claude.SessionStatsRecord
	for _, rec := range records {
		if idx, ok := seen[rec.SessionID]; ok {
			deduped[idx] = rec
		} else {
			seen[rec.SessionID] = len(deduped)
			deduped = append(deduped, rec)
		}
	}

	merged := aggregateRecords(deduped)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Aggregate stats (%d sessions, last 7 days)\n", len(deduped))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "─────────────────────────────────\n")

	printAggregateStats(cmd, merged)
	printRecordBreakdown(cmd, deduped)
	return nil
}

// showAggregateFromTranscripts falls back to parsing transcripts when no stats files exist.
func showAggregateFromTranscripts(cmd *cobra.Command) error {
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

	var rows []sessionBreakdownRow
	var allStats []*claude.TranscriptStats
	for _, sess := range recent {
		s, err := collectSessionStats(sess, clotildeRoot)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping session '%s': %v\n", sess.Name, err)
			continue
		}
		if s != nil {
			rows = append(rows, sessionBreakdownRow{
				name:        sess.Name,
				turns:       s.Turns,
				activeTimeS: int(s.ActiveTime.Seconds()),
				tokens:      s.InputTokens + s.OutputTokens,
			})
			allStats = append(allStats, s)
		}
	}

	merged := claude.MergeTranscriptStats(allStats)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Aggregate stats (%d sessions, last 7 days)\n", len(rows))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "─────────────────────────────────\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "(from transcripts, enable stats tracking with 'clotilde setup --stats')\n\n")

	printStats(cmd, merged)
	printBreakdownTable(cmd, rows)
	return nil
}

// readStatsForPeriod reads all stats records from the last N days of JSONL files.
// Returns records in chronological order (oldest first) so dedup keeps the latest.
func readStatsForPeriod(now time.Time, days int) ([]claude.SessionStatsRecord, error) {
	var all []claude.SessionStatsRecord
	for daysBack := days - 1; daysBack >= 0; daysBack-- {
		date := now.AddDate(0, 0, -daysBack)
		path, err := claude.DailyStatsFilePath(date)
		if err != nil {
			return nil, err
		}
		records, err := claude.ReadStatsFile(path)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
}

// aggregateStats holds aggregated data from SessionStatsRecords.
type aggregateStats struct {
	Turns               int
	ActiveTimeS         int
	TotalTimeS          int
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	Models              []string
	ToolUses            map[string]int
	Earliest            time.Time
	Latest              time.Time
}

// aggregateRecords sums deduplicated stats records into a single aggregate.
// Uses delta fields (current - prev_*) for all counters to compute per-period activity.
func aggregateRecords(records []claude.SessionStatsRecord) aggregateStats {
	agg := aggregateStats{
		ToolUses: make(map[string]int),
	}
	modelSeen := make(map[string]bool)

	for _, rec := range records {
		// Use delta (current - prev) for per-period stats
		agg.Turns += rec.Turns - rec.PrevTurns
		agg.ActiveTimeS += rec.ActiveTimeS - rec.PrevActiveTimeS
		agg.TotalTimeS += rec.TotalTimeS - rec.PrevTotalTimeS
		agg.InputTokens += rec.InputTokens - rec.PrevInputTokens
		agg.OutputTokens += rec.OutputTokens - rec.PrevOutputTokens

		agg.CacheCreationTokens += rec.CacheCreationTokens - rec.PrevCacheCreationTokens
		agg.CacheReadTokens += rec.CacheReadTokens - rec.PrevCacheReadTokens

		for _, m := range rec.Models {
			if !modelSeen[m] {
				modelSeen[m] = true
				agg.Models = append(agg.Models, m)
			}
		}
		for tool, count := range rec.ToolUses {
			prevCount := rec.PrevToolUses[tool]
			agg.ToolUses[tool] += count - prevCount
		}

		if !rec.EndedAt.IsZero() {
			if agg.Earliest.IsZero() || rec.EndedAt.Before(agg.Earliest) {
				agg.Earliest = rec.EndedAt
			}
			if rec.EndedAt.After(agg.Latest) {
				agg.Latest = rec.EndedAt
			}
		}
	}
	return agg
}

func printAggregateStats(cmd *cobra.Command, agg aggregateStats) {
	w := cmd.OutOrStdout()

	if agg.Turns == 0 {
		_, _ = fmt.Fprintln(w, "No activity recorded.")
		return
	}

	_, _ = fmt.Fprintf(w, "Turns         %d\n", agg.Turns)

	if agg.TotalTimeS > 0 {
		_, _ = fmt.Fprintf(w, "Total time    %s\n", util.FormatDuration(time.Duration(agg.TotalTimeS)*time.Second))
	}
	if agg.ActiveTimeS > 0 {
		_, _ = fmt.Fprintf(w, "Active time   %s     (approx)\n", util.FormatDuration(time.Duration(agg.ActiveTimeS)*time.Second))
	}

	if agg.InputTokens > 0 || agg.OutputTokens > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "Input tokens  %s\n", formatTokenCount(agg.InputTokens))
		_, _ = fmt.Fprintf(w, "Output tokens %s\n", formatTokenCount(agg.OutputTokens))
		if agg.CacheReadTokens > 0 {
			_, _ = fmt.Fprintf(w, "Cache read    %s\n", formatTokenCount(agg.CacheReadTokens))
		}
		if agg.CacheCreationTokens > 0 {
			_, _ = fmt.Fprintf(w, "Cache write   %s\n", formatTokenCount(agg.CacheCreationTokens))
		}
	}

	if len(agg.Models) > 0 {
		_, _ = fmt.Fprintln(w)
		families := make([]string, len(agg.Models))
		for i, m := range agg.Models {
			families[i] = claude.FormatModelFamily(m)
		}
		_, _ = fmt.Fprintf(w, "Models        %s\n", strings.Join(families, ", "))
	}

	if len(agg.ToolUses) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Tool usage:")
		printToolUses(w, agg.ToolUses)
	}
}

// sessionBreakdownRow holds per-session data for the breakdown table.
type sessionBreakdownRow struct {
	name        string
	turns       int
	activeTimeS int
	tokens      int
}

// printRecordBreakdown prints a per-session table from JSONL records, sorted by active time descending.
func printRecordBreakdown(cmd *cobra.Command, records []claude.SessionStatsRecord) {
	if len(records) < 2 {
		return
	}

	rows := make([]sessionBreakdownRow, 0, len(records))
	for _, rec := range records {
		rows = append(rows, sessionBreakdownRow{
			name:        rec.SessionName,
			turns:       rec.Turns - rec.PrevTurns,
			activeTimeS: rec.ActiveTimeS - rec.PrevActiveTimeS,
			tokens:      (rec.InputTokens - rec.PrevInputTokens) + (rec.OutputTokens - rec.PrevOutputTokens),
		})
	}

	printBreakdownTable(cmd, rows)
}

// printBreakdownTable prints a sorted per-session breakdown table.
func printBreakdownTable(cmd *cobra.Command, rows []sessionBreakdownRow) {
	// Sort by active time descending, then by name
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].activeTimeS != rows[j].activeTimeS {
			return rows[i].activeTimeS > rows[j].activeTimeS
		}
		return rows[i].name < rows[j].name
	})

	// Find max name length for alignment
	maxName := 7 // minimum "Session" header width
	for _, r := range rows {
		if len(r.name) > maxName {
			maxName = len(r.name)
		}
	}

	w := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "  %-*s  %5s  %8s  %8s\n", maxName, "Session", "Turns", "Active", "Tokens")
	for _, r := range rows {
		active := util.FormatDuration(time.Duration(r.activeTimeS) * time.Second)
		_, _ = fmt.Fprintf(w, "  %-*s  %5d  %8s  %8s\n", maxName, r.name, r.turns, active, formatTokenCount(r.tokens))
	}
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

// internalTools are Claude Code orchestration/meta tools that aren't
// user-visible work actions. Excluded from stats display.
var internalTools = map[string]bool{
	"AskUserQuestion": true,
	"EnterPlanMode":   true,
	"ExitPlanMode":    true,
	"EnterWorktree":   true,
	"ExitWorktree":    true,
	"TaskCreate":      true,
	"TaskGet":         true,
	"TaskList":        true,
	"TaskOutput":      true,
	"TaskStop":        true,
	"TaskUpdate":      true,
	"ToolSearch":      true,
}

// printToolUses prints tool usage sorted by count (descending), then name.
// Internal/orchestration tools are excluded from display.
func printToolUses(w interface{ Write([]byte) (int, error) }, toolUses map[string]int) {
	type toolCount struct {
		name  string
		count int
	}
	sorted := make([]toolCount, 0, len(toolUses))
	for name, count := range toolUses {
		if internalTools[name] {
			continue
		}
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
