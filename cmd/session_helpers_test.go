package cmd

import (
	"testing"

	"github.com/fgrehm/clotilde/internal/session"
)

func TestAllTranscriptPaths_EmptyFork(t *testing.T) {
	// A newly-created fork has empty SessionID and no TranscriptPath (the hook
	// hasn't run yet). allTranscriptPaths must not append a path ending in ".jsonl"
	// for such sessions.
	sess := &session.Session{}
	sess.Metadata.SessionID = ""
	sess.Metadata.TranscriptPath = ""

	paths := allTranscriptPaths(sess, "/tmp/clotilde", "/home/user")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty fork, got %d: %v", len(paths), paths)
	}
}

func TestAllTranscriptPaths_WithSessionID(t *testing.T) {
	sess := &session.Session{}
	sess.Metadata.SessionID = "abc-123"
	sess.Metadata.TranscriptPath = ""

	paths := allTranscriptPaths(sess, "/tmp/.claude/clotilde", "/home/user")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] == "" {
		t.Error("expected non-empty path for session with UUID")
	}
}

func TestAllTranscriptPaths_WithExplicitTranscriptPath(t *testing.T) {
	sess := &session.Session{}
	sess.Metadata.SessionID = ""
	sess.Metadata.TranscriptPath = "/home/user/.claude/projects/foo/abc.jsonl"

	paths := allTranscriptPaths(sess, "/tmp/.claude/clotilde", "/home/user")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/home/user/.claude/projects/foo/abc.jsonl" {
		t.Errorf("unexpected path: %s", paths[0])
	}
}

func TestAllTranscriptPaths_WithPreviousIDs(t *testing.T) {
	sess := &session.Session{}
	sess.Metadata.SessionID = "current-uuid"
	sess.Metadata.TranscriptPath = ""
	sess.Metadata.PreviousSessionIDs = []string{"old-uuid-1", "old-uuid-2"}

	paths := allTranscriptPaths(sess, "/tmp/.claude/clotilde", "/home/user")
	// previous (2) + current (1)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d: %v", len(paths), paths)
	}
}
