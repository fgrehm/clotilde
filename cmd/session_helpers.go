package cmd

import (
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
