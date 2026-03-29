package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/session"
)

// allTranscriptPaths returns paths for all transcripts associated with a session,
// in chronological order: previous UUIDs first (oldest to newest), then the current one.
// The current path comes from metadata when available; otherwise it is computed from the UUID.
// Callers should skip paths that do not exist on disk (missing transcripts are not an error).
func allTranscriptPaths(sess *session.Session, clotildeRoot, homeDir string) []string {
	var paths []string

	for _, prevID := range sess.Metadata.PreviousSessionIDs {
		if prevID == "" {
			continue
		}
		paths = append(paths, claude.TranscriptPath(homeDir, clotildeRoot, prevID))
	}

	current := sess.Metadata.TranscriptPath
	if current == "" && sess.Metadata.SessionID != "" {
		current = claude.TranscriptPath(homeDir, clotildeRoot, sess.Metadata.SessionID)
	}
	if current != "" {
		paths = append(paths, current)
	}

	return paths
}

// resolveSessionName resolves the session name using a multi-level fallback strategy.
// Priority 1: CLOTILDE_SESSION_NAME env var (always checked).
// When fullFallback is true, also tries:
// Priority 2: Read from CLAUDE_ENV_FILE (persisted by previous hook).
// Priority 3: Reverse UUID lookup in session store.
func resolveSessionName(hookData hookInput, store session.Store, fullFallback bool) (string, error) {
	if name := os.Getenv("CLOTILDE_SESSION_NAME"); name != "" {
		return name, nil
	}

	if !fullFallback {
		return "", nil
	}

	if name := readLastEnvFileValue("CLOTILDE_SESSION"); name != "" {
		return name, nil
	}

	return findSessionByUUID(store, hookData.SessionID)
}

// findSessionByUUID searches for a session with the given UUID.
// Checks both current sessionId and previousSessionIds.
func findSessionByUUID(store session.Store, uuid string) (string, error) {
	sessions, err := store.List()
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, sess := range sessions {
		if sess.Metadata.SessionID == uuid {
			return sess.Name, nil
		}
	}

	for _, sess := range sessions {
		if slices.Contains(sess.Metadata.PreviousSessionIDs, uuid) {
			return sess.Name, nil
		}
	}

	return "", fmt.Errorf("no session found with UUID %s", uuid)
}
