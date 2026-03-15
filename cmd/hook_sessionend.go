package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/notify"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var sessionEndCmd = &cobra.Command{
	Use:   "sessionend",
	Short: "SessionEnd hook handler for stats recording",
	Long:  `Called by Claude Code's SessionEnd hook. Records session statistics to a daily JSONL file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprint(os.Stderr, "clotilde: saving session stats...\n")

		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read hook input: %w", err)
		}

		var hookData hookInput
		if err := json.Unmarshal(input, &hookData); err != nil {
			return fmt.Errorf("failed to parse hook input: %w", err)
		}

		if err := notify.LogEvent(input, hookData.SessionID); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to log event: %v\n", err)
		}

		// Double-execution guard (checked before work, marked after successful write)
		marker := hookData.SessionID + ":sessionend"
		if isHookExecuted(marker) {
			return nil
		}

		now := time.Now().UTC()

		record, err := buildStatsRecord(hookData, now)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "clotilde: stats skipped: %v\n", err)
			return nil
		}

		// Critical section: mask signals for the file write only
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
		err = claude.AppendStatsRecord(record)
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)

		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "clotilde: failed to write stats: %v\n", err)
		} else {
			markHookExecuted(marker)
		}

		return nil
	},
}

// buildStatsRecord assembles a SessionStatsRecord from the hook payload and
// transcript data. Returns an error if transcripts can't be read.
func buildStatsRecord(hookData hookInput, now time.Time) (claude.SessionStatsRecord, error) {
	var sessionName, projectPath string
	var transcriptPaths []string

	clotildeRoot, rootErr := config.FindClotildeRoot()

	if rootErr == nil {
		// Clotilde project found: resolve session and build full transcript list
		store := session.NewFileStore(clotildeRoot)
		name, _ := resolveSessionName(hookData, store, true)
		sessionName = name

		projectPath = config.ProjectRoot(clotildeRoot)

		if sessionName != "" {
			if sess, err := store.Get(sessionName); err == nil {
				homeDir, homeDirErr := util.HomeDir()
				if homeDirErr == nil {
					transcriptPaths = allTranscriptPaths(sess, clotildeRoot, homeDir)
				}
			}
		}
	}

	// Fallback: if no transcript paths resolved, use payload's transcript_path
	if len(transcriptPaths) == 0 && hookData.TranscriptPath != "" {
		transcriptPaths = []string{hookData.TranscriptPath}
	}

	if len(transcriptPaths) == 0 {
		return claude.SessionStatsRecord{}, fmt.Errorf("no transcript paths available")
	}

	// Parse and merge all transcripts
	var statsList []*claude.TranscriptStats
	for _, path := range transcriptPaths {
		s, err := claude.ParseTranscriptStats(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "clotilde: skipping unreadable transcript %s: %v\n", path, err)
			continue
		}
		statsList = append(statsList, s)
	}

	if len(statsList) == 0 {
		return claude.SessionStatsRecord{}, fmt.Errorf("no readable transcripts found")
	}

	merged := claude.MergeTranscriptStats(statsList)

	// Look up prior record for prev_* fields
	prev, err := claude.FindLastRecord(hookData.SessionID, now)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "clotilde: warning: FindLastRecord: %v\n", err)
	}

	record := claude.SessionStatsRecord{
		SessionName:         sessionName,
		SessionID:           hookData.SessionID,
		ProjectPath:         projectPath,
		Turns:               merged.Turns,
		ActiveTimeS:         int(merged.ActiveTime.Seconds()),
		TotalTimeS:          int(merged.TotalTime.Seconds()),
		InputTokens:         merged.InputTokens,
		OutputTokens:        merged.OutputTokens,
		CacheCreationTokens: merged.CacheCreationTokens,
		CacheReadTokens:     merged.CacheReadTokens,
		Models:              merged.Models,
		ToolUses:            merged.ToolUses,
		EndedAt:             now,
	}

	if record.Models == nil {
		record.Models = []string{}
	}
	if record.ToolUses == nil {
		record.ToolUses = make(map[string]int)
	}

	if prev != nil {
		record.PrevTurns = prev.Turns
		record.PrevActiveTimeS = prev.ActiveTimeS
		record.PrevTotalTimeS = prev.TotalTimeS
		record.PrevInputTokens = prev.InputTokens
		record.PrevOutputTokens = prev.OutputTokens
		record.PrevCacheCreationTokens = prev.CacheCreationTokens
		record.PrevCacheReadTokens = prev.CacheReadTokens
		record.PrevToolUses = prev.ToolUses
	}

	return record, nil
}

func init() {
	hookCmd.AddCommand(sessionEndCmd)
}
