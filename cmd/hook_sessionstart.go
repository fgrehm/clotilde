package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/notify"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

// hookInput represents the JSON structure passed to SessionStart hooks.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Source         string `json:"source"` // startup, resume, compact, clear
}

var sessionStartCmd = &cobra.Command{
	Use:   "sessionstart",
	Short: "Unified SessionStart hook handler",
	Long: `Called by Claude Code's SessionStart hook for all sources (startup, resume, compact, clear).
Handles fork registration, session ID updates, and context injection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read hook input from stdin (must happen before guard check,
		// since we need session_id and source to scope the guard)
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read hook input: %w", err)
		}

		var hookData hookInput
		if err := json.Unmarshal(input, &hookData); err != nil {
			return fmt.Errorf("failed to parse hook input: %w", err)
		}

		// Log raw event for debugging (before any other processing)
		if err := notify.LogEvent(input, hookData.SessionID); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to log event: %v\n", err)
		}

		// Guard against double execution (global + per-project hooks).
		// Scoped to session_id:source so that different events (e.g. startup
		// vs clear) are not blocked by a previous invocation's marker.
		marker := hookData.SessionID + ":" + hookData.Source
		if isHookExecuted(marker) {
			return nil
		}

		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			// Not in a clotilde project, silently exit
			return nil
		}

		// Mark as executed to prevent double-run from global + project hooks
		markHookExecuted(marker)

		store := session.NewFileStore(clotildeRoot)

		// Dispatch based on source field
		switch hookData.Source {
		case "startup":
			return handleStartup(clotildeRoot, hookData, store)
		case "resume":
			return handleResume(clotildeRoot, hookData, store)
		case "compact":
			return handleCompact(clotildeRoot, hookData, store)
		case "clear":
			return handleClear(clotildeRoot, hookData, store)
		default:
			// Fallback to startup for backward compatibility or unknown sources
			return handleStartup(clotildeRoot, hookData, store)
		}
	},
}

// handleStartup handles new session startup.
func handleStartup(clotildeRoot string, hookData hookInput, store session.Store) error {
	// Get session name from environment
	sessionName := os.Getenv("CLOTILDE_SESSION_NAME")

	// Persist session name for statusline and future operations
	if sessionName != "" {
		if err := writeSessionNameToEnv(sessionName); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to write session name to env: %v\n", err)
		}

		// Save transcript path to metadata
		if hookData.TranscriptPath != "" {
			if err := saveTranscriptPath(store, sessionName, hookData.TranscriptPath); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to save transcript path: %v\n", err)
			}
		}
	}

	// Output session name, context, and global context
	outputContexts(clotildeRoot, store, sessionName)

	return nil
}

// handleResume handles session resumption and fork registration.
func handleResume(clotildeRoot string, hookData hookInput, store session.Store) error {
	sessionName := os.Getenv("CLOTILDE_SESSION_NAME")

	// Check if this is a fork registration
	forkName := os.Getenv("CLOTILDE_FORK_NAME")
	if forkName != "" {
		if err := registerFork(store, forkName, hookData.SessionID); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to register fork: %v\n", err)
		}
		// Use fork name for context output
		sessionName = forkName
	}

	// Persist session name
	if sessionName != "" {
		if err := writeSessionNameToEnv(sessionName); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to write session name to env: %v\n", err)
		}

		// Save transcript path to metadata
		if hookData.TranscriptPath != "" {
			if err := saveTranscriptPath(store, sessionName, hookData.TranscriptPath); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to save transcript path: %v\n", err)
			}
		}
	}

	// Crash recovery: check if the prior invocation's stats were recorded
	// Only run when stats tracking is enabled (opt-in)
	if sessionName != "" {
		globalCfg, cfgErr := config.LoadGlobalOrDefault()
		if cfgErr == nil && globalCfg.StatsTracking != nil && *globalCfg.StatsTracking {
			attemptCrashRecovery(clotildeRoot, sessionName, store)
		}
	}

	// Output session name, context, and global context
	outputContexts(clotildeRoot, store, sessionName)

	return nil
}

// attemptCrashRecovery checks if the previous invocation of this session ended
// without a SessionEnd hook firing (crash, SIGKILL, power loss). If so, writes
// a recovery stats record using the transcript data available now.
func attemptCrashRecovery(clotildeRoot, sessionName string, store session.Store) {
	sess, err := store.Get(sessionName)
	if err != nil {
		return
	}

	// Fast-path: if lastAccessed is within 30 seconds, skip (double-fire or immediate resume)
	if time.Since(sess.Metadata.LastAccessed) < 30*time.Second {
		return
	}

	// Check if a stats record already exists for the last invocation.
	// Only skip recovery if the record's EndedAt is after LastAccessed,
	// meaning the prior run exited cleanly after the session was last opened.
	prev, err := claude.FindLastRecord(sess.Metadata.SessionID, time.Now().UTC())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "clotilde: crash recovery lookup failed: %v\n", err)
		return
	}
	if prev != nil && prev.EndedAt.After(sess.Metadata.LastAccessed) {
		return // Normal exit, stats already recorded for this invocation
	}

	// No prior record found: write a recovery record
	homeDir, err := util.HomeDir()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "clotilde: crash recovery skipped: %v\n", err)
		return
	}
	paths := allTranscriptPaths(sess, clotildeRoot, homeDir)
	if len(paths) == 0 {
		return
	}

	var statsList []*claude.TranscriptStats
	for _, p := range paths {
		s, parseErr := claude.ParseTranscriptStats(p)
		if parseErr != nil {
			continue
		}
		statsList = append(statsList, s)
	}
	if len(statsList) == 0 {
		return
	}

	merged := claude.MergeTranscriptStats(statsList)
	if merged.Turns == 0 {
		return // Empty transcript, nothing to recover
	}

	endedAt := sess.Metadata.LastAccessed.UTC()
	if !merged.LastMessage.IsZero() {
		endedAt = merged.LastMessage.UTC()
	}
	record := claude.SessionStatsRecord{
		SessionName:         sessionName,
		SessionID:           sess.Metadata.SessionID,
		ProjectPath:         config.ProjectRoot(clotildeRoot),
		Turns:               merged.Turns,
		ActiveTimeS:         int(merged.ActiveTime.Seconds()),
		TotalTimeS:          int(merged.TotalTime.Seconds()),
		InputTokens:         merged.InputTokens,
		OutputTokens:        merged.OutputTokens,
		CacheCreationTokens: merged.CacheCreationTokens,
		CacheReadTokens:     merged.CacheReadTokens,
		Models:              merged.Models,
		ToolUses:            merged.ToolUses,
		EndedAt:             endedAt,
	}
	if record.Models == nil {
		record.Models = []string{}
	}
	if record.ToolUses == nil {
		record.ToolUses = make(map[string]int)
	}

	if writeErr := claude.AppendStatsRecord(record); writeErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "clotilde: crash recovery write failed: %v\n", writeErr)
	}
}

// handleCompact handles session compaction, updating session ID and preserving history.
// NOTE: Currently Claude Code does NOT create a new UUID for /compact (only /clear does).
// This handler is defensive programming in case Claude Code's behavior changes in the future.
func handleCompact(clotildeRoot string, hookData hookInput, store session.Store) error {
	// Resolve session name using three-level fallback
	sessionName, err := resolveSessionName(hookData, store, true)
	if err != nil {
		// If we can't resolve the session name, silently continue
		// This might be a non-clotilde session or first compact without env
		_, _ = fmt.Fprintf(os.Stderr, "Warning: unable to resolve session name for compact: %v\n", err)
		return nil
	}

	if sessionName == "" {
		// No session name available, nothing to update
		return nil
	}

	// Load existing session
	sess, err := store.Get(sessionName)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: session '%s' not found: %v\n", sessionName, err)
		return nil
	}

	// Update session ID, preserving old ID in history
	sess.AddPreviousSessionID(hookData.SessionID)
	sess.Metadata.TranscriptPath = hookData.TranscriptPath
	sess.UpdateLastAccessed()

	if err := store.Update(sess); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to update session metadata: %v\n", err)
	}

	// Persist session name for next operation
	if err := writeSessionNameToEnv(sessionName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to write session name to env: %v\n", err)
	}

	// Output session name, context, and global context
	outputContexts(clotildeRoot, store, sessionName)

	return nil
}

// handleClear handles session clear - identical to compact.
// Unlike /compact, /clear DOES create a new session UUID in Claude Code.
func handleClear(clotildeRoot string, hookData hookInput, store session.Store) error {
	return handleCompact(clotildeRoot, hookData, store)
}

// registerFork updates the fork's metadata.json with the actual session UUID.
// This is idempotent - won't overwrite existing UUIDs.
func registerFork(store session.Store, forkName, sessionID string) error {
	// Load fork session
	fork, err := store.Get(forkName)
	if err != nil {
		return fmt.Errorf("fork '%s' not found: %w", forkName, err)
	}

	// Only update if sessionId is empty (idempotent)
	if fork.Metadata.SessionID == "" {
		fork.Metadata.SessionID = sessionID
		fork.UpdateLastAccessed()
		if err := store.Update(fork); err != nil {
			return fmt.Errorf("failed to update fork metadata: %w", err)
		}
	}

	return nil
}

// saveTranscriptPath saves the transcript path and updates lastAccessed in a single write.
func saveTranscriptPath(store session.Store, sessionName, transcriptPath string) error {
	sess, err := store.Get(sessionName)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", sessionName, err)
	}

	sess.Metadata.TranscriptPath = transcriptPath
	sess.UpdateLastAccessed()

	if err := store.Update(sess); err != nil {
		return fmt.Errorf("failed to update session metadata: %w", err)
	}

	return nil
}

// isHookExecuted checks if a hook with this marker has already run.
// It checks both the env var (set by Claude Code after sourcing CLAUDE_ENV_FILE)
// and the file contents directly (in case Claude Code hasn't re-sourced yet).
// The marker is scoped to "session_id:source" so different events don't block each other.
func isHookExecuted(marker string) bool {
	if os.Getenv("CLOTILDE_HOOK_EXECUTED") == marker {
		return true
	}
	return readLastEnvFileValue("CLOTILDE_HOOK_EXECUTED") == marker
}

// markHookExecuted writes CLOTILDE_HOOK_EXECUTED=<marker> to CLAUDE_ENV_FILE
// so that a second hook invocation (from global + project hooks) for the same
// event is skipped.
func markHookExecuted(marker string) {
	_ = appendToEnvFile("CLOTILDE_HOOK_EXECUTED", marker)
}

// writeSessionNameToEnv writes the session name to Claude's env file for statusline use.
func writeSessionNameToEnv(sessionName string) error {
	return appendToEnvFile("CLOTILDE_SESSION", sessionName)
}

// readLastEnvFileValue reads CLAUDE_ENV_FILE and returns the last value
// assigned to the given key (KEY=value lines). Returns "" if not found.
// Uses last-wins semantics to match shell sourcing behavior.
func readLastEnvFileValue(key string) string {
	claudeEnvFile := os.Getenv("CLAUDE_ENV_FILE")
	if claudeEnvFile == "" {
		return ""
	}

	content, err := os.ReadFile(claudeEnvFile)
	if err != nil {
		return ""
	}

	prefix := key + "="
	var lastValue string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			lastValue = strings.TrimPrefix(line, prefix)
		}
	}
	return lastValue
}

// appendToEnvFile appends a KEY=value line to CLAUDE_ENV_FILE.
// Returns nil if CLAUDE_ENV_FILE is not set.
func appendToEnvFile(key, value string) error {
	claudeEnvFile := os.Getenv("CLAUDE_ENV_FILE")
	if claudeEnvFile == "" {
		return nil
	}

	f, err := os.OpenFile(claudeEnvFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open CLAUDE_ENV_FILE: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintf(f, "%s=%s\n", key, value); err != nil {
		return fmt.Errorf("failed to write to CLAUDE_ENV_FILE: %w", err)
	}
	return nil
}

// outputContexts loads and outputs session name and session context.
func outputContexts(_ string, store session.Store, sessionName string) {
	// Output session name
	if sessionName != "" {
		fmt.Printf("\nSession name: %s\n", sessionName)
	}

	// Output session context from metadata
	if sessionName != "" {
		sess, err := store.Get(sessionName)
		if err == nil && sess.Metadata.Context != "" {
			fmt.Printf("Context: %s\n", sess.Metadata.Context)
		}
	}
}

func init() {
	hookCmd.AddCommand(sessionStartCmd)
}
