package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
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

	// Output session name, context, and global context
	outputContexts(clotildeRoot, store, sessionName)

	return nil
}

// handleCompact handles session compaction, updating session ID and preserving history.
// NOTE: Currently Claude Code does NOT create a new UUID for /compact (only /clear does).
// This handler is defensive programming in case Claude Code's behavior changes in the future.
func handleCompact(clotildeRoot string, hookData hookInput, store session.Store) error {
	// Resolve session name using three-level fallback
	sessionName, err := getSessionName("compact", hookData, store)
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

// getSessionName resolves the session name using a three-level fallback strategy.
func getSessionName(source string, hookData hookInput, store session.Store) (string, error) {
	// Priority 1: CLOTILDE_SESSION_NAME env var (set by clotilde start/resume)
	if name := os.Getenv("CLOTILDE_SESSION_NAME"); name != "" {
		return name, nil
	}

	// For compact/clear operations, try additional fallback methods
	if source == "compact" || source == "clear" {
		// Priority 2: Read from CLAUDE_ENV_FILE (persisted by previous hook)
		if name := readSessionNameFromEnvFile(); name != "" {
			return name, nil
		}

		// Priority 3: Reverse UUID lookup (last resort)
		// Note: This uses the OLD session ID before compact/clear
		// We need to search for sessions that might have this as current or previous ID
		return findSessionByUUID(store, hookData.SessionID)
	}

	return "", nil
}

// readSessionNameFromEnvFile reads the session name from CLAUDE_ENV_FILE.
func readSessionNameFromEnvFile() string {
	return readLastEnvFileValue("CLOTILDE_SESSION")
}

// findSessionByUUID searches for a session with the given UUID.
// Checks both current sessionId and previousSessionIds.
func findSessionByUUID(store session.Store, uuid string) (string, error) {
	sessions, err := store.List()
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	// First check current sessionId
	for _, sess := range sessions {
		if sess.Metadata.SessionID == uuid {
			return sess.Name, nil
		}
	}

	// Then check previousSessionIds
	for _, sess := range sessions {
		for _, prevID := range sess.Metadata.PreviousSessionIDs {
			if prevID == uuid {
				return sess.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no session found with UUID %s", uuid)
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

// saveTranscriptPath saves the transcript path to the session's metadata.
func saveTranscriptPath(store session.Store, sessionName, transcriptPath string) error {
	// Load session
	sess, err := store.Get(sessionName)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", sessionName, err)
	}

	// Update transcript path
	sess.Metadata.TranscriptPath = transcriptPath
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
