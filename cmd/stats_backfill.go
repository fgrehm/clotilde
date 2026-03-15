package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
)

func newStatsBackfillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backfill",
		Short: "Generate stats records from existing session transcripts",
		Long: `Parse transcripts for all sessions in the current project and write
stats records to the daily JSONL files. Skips sessions that already have
a record. Useful for populating stats after enabling tracking on an
existing project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			now := time.Now()
			var wrote, skipped, errored int

			for _, sess := range sessions {
				name := sess.Name
				sid := sess.Metadata.SessionID
				if sid == "" {
					skipped++
					continue
				}

				// Skip if a record already exists for this session
				existing, err := claude.FindLastRecord(sid, now)
				if err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  warning: %s: failed to check existing records: %v\n", name, err)
					errored++
					continue
				}
				if existing != nil {
					skipped++
					continue
				}

				stats, err := collectSessionStats(sess, clotildeRoot)
				if err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  warning: %s: %v\n", name, err)
					errored++
					continue
				}
				if stats == nil || stats.Turns == 0 {
					skipped++
					continue
				}

				projectPath := config.ProjectRoot(clotildeRoot)

				record := claude.SessionStatsRecord{
					SessionName:         name,
					SessionID:           sid,
					ProjectPath:         projectPath,
					Turns:               stats.Turns,
					ActiveTimeS:         int(stats.ActiveTime.Seconds()),
					TotalTimeS:          int(stats.TotalTime.Seconds()),
					InputTokens:         stats.InputTokens,
					OutputTokens:        stats.OutputTokens,
					CacheCreationTokens: stats.CacheCreationTokens,
					CacheReadTokens:     stats.CacheReadTokens,
					Models:              stats.Models,
					ToolUses:            stats.ToolUses,
					EndedAt:             sess.Metadata.LastAccessed,
				}

				if err := claude.AppendStatsRecord(record); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  warning: %s: failed to write record: %v\n", name, err)
					errored++
					continue
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d turns, %s tokens\n",
					name, stats.Turns, formatTokenCount(stats.InputTokens+stats.OutputTokens))
				wrote++
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backfill complete: %d written, %d skipped", wrote, skipped)
			if errored > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), ", %d errors", errored)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())

			return nil
		},
	}
}
