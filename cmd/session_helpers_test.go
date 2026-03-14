package cmd

import (
	"testing"

	"github.com/fgrehm/clotilde/internal/session"
)

func TestAllTranscriptPaths(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		transcriptPath string
		previousIDs    []string
		wantCount      int
	}{
		{
			name:      "empty fork — hook hasn't filled in UUID yet",
			wantCount: 0,
		},
		{
			name:      "session with UUID only — path computed from UUID",
			sessionID: "abc-123",
			wantCount: 1,
		},
		{
			name:           "explicit TranscriptPath takes precedence over UUID",
			transcriptPath: "/home/user/.claude/projects/foo/abc.jsonl",
			wantCount:      1,
		},
		{
			name:        "previous IDs included before current",
			sessionID:   "current-uuid",
			previousIDs: []string{"old-uuid-1", "old-uuid-2"},
			wantCount:   3, // 2 previous + 1 current
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := &session.Session{}
			sess.Metadata.SessionID = tt.sessionID
			sess.Metadata.TranscriptPath = tt.transcriptPath
			sess.Metadata.PreviousSessionIDs = tt.previousIDs

			paths := allTranscriptPaths(sess, "/tmp/.claude/clotilde", "/home/user")

			if len(paths) != tt.wantCount {
				t.Errorf("got %d paths %v, want %d", len(paths), paths, tt.wantCount)
			}
			// Verify no path is empty — a path ending in ".jsonl" would be a sign
			// that an empty UUID slipped through.
			for _, p := range paths {
				if p == "" {
					t.Errorf("paths contains an empty entry: %v", paths)
				}
			}
			// Explicit path should be preserved verbatim.
			if tt.transcriptPath != "" && len(paths) > 0 {
				last := paths[len(paths)-1]
				if last != tt.transcriptPath {
					t.Errorf("last path = %q, want %q", last, tt.transcriptPath)
				}
			}
		})
	}
}
