package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
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
		// Read hook input from stdin
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read hook input: %w", err)
		}

		var hookData hookInput
		if err := json.Unmarshal(input, &hookData); err != nil {
			return fmt.Errorf("failed to parse hook input: %w", err)
		}

		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			// Not in a clotilde project, silently exit
			return nil
		}

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

	// Output only global context for new sessions
	if err := outputContexts(clotildeRoot, ""); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to output contexts: %v\n", err)
	}

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

	// Output contexts (global + fork context if forking)
	contextSessionName := ""
	if forkName != "" {
		contextSessionName = forkName
	}
	if err := outputContexts(clotildeRoot, contextSessionName); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to output contexts: %v\n", err)
	}

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

	// Output global context
	if err := outputContexts(clotildeRoot, ""); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to output contexts: %v\n", err)
	}

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
	claudeEnvFile := os.Getenv("CLAUDE_ENV_FILE")
	if claudeEnvFile == "" {
		return ""
	}

	content, err := os.ReadFile(claudeEnvFile)
	if err != nil {
		return ""
	}

	// Parse for CLOTILDE_SESSION=value
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CLOTILDE_SESSION=") {
			return strings.TrimPrefix(line, "CLOTILDE_SESSION=")
		}
	}

	return ""
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

// writeSessionNameToEnv writes the session name to Claude's env file for statusline use.
func writeSessionNameToEnv(sessionName string) error {
	claudeEnvFile := os.Getenv("CLAUDE_ENV_FILE")
	if claudeEnvFile == "" {
		// CLAUDE_ENV_FILE not available (only available in SessionStart hooks)
		return nil
	}

	// Open file in append mode, create if doesn't exist
	f, err := os.OpenFile(claudeEnvFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CLAUDE_ENV_FILE: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Write session name as environment variable
	if _, err := fmt.Fprintf(f, "CLOTILDE_SESSION=%s\n", sessionName); err != nil {
		return fmt.Errorf("failed to write to CLAUDE_ENV_FILE: %w", err)
	}

	return nil
}

// outputContexts loads and outputs global and session-specific context.
func outputContexts(clotildeRoot, sessionName string) error {
	// Output global context if exists
	globalContext := filepath.Join(clotildeRoot, config.GlobalContextFile)
	if util.FileExists(globalContext) {
		content, err := os.ReadFile(globalContext)
		if err == nil {
			fmt.Printf("\n--- Global Context ---\n%s\n", string(content))
		}
	}

	// Output session-specific context if sessionName is provided
	if sessionName != "" {
		sessionContext := filepath.Join(config.GetSessionDir(clotildeRoot, sessionName), "context.md")
		if util.FileExists(sessionContext) {
			content, err := os.ReadFile(sessionContext)
			if err == nil {
				fmt.Printf("\n--- Session Context ---\n%s\n", string(content))
			}
		}
	}

	return nil
}

func init() {
	hookCmd.AddCommand(sessionStartCmd)
}
