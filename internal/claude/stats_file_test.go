package claude_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fgrehm/clotilde/internal/claude"
)

func TestStatsDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/custom/data")
		dir, err := claude.StatsDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dir != "/custom/data/clotilde/stats" {
			t.Errorf("got %q, want /custom/data/clotilde/stats", dir)
		}
	})

	t.Run("falls back to ~/.local/share when XDG_DATA_HOME not set", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		dir, err := claude.StatsDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".local", "share", "clotilde", "stats")
		if dir != want {
			t.Errorf("got %q, want %q", dir, want)
		}
	})
}

func TestDailyStatsFilePath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	ts := time.Date(2026, 3, 13, 18, 0, 0, 0, time.UTC)
	path, err := claude.DailyStatsFilePath(ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/data/clotilde/stats/2026-03-13.jsonl" {
		t.Errorf("got %q, want /data/clotilde/stats/2026-03-13.jsonl", path)
	}
}

func TestAppendStatsRecord(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	now := time.Date(2026, 3, 13, 18, 0, 0, 0, time.UTC)

	t.Run("creates file and parent dirs if missing", func(t *testing.T) {
		rec := claude.SessionStatsRecord{
			SessionName: "test-session",
			SessionID:   "uuid-1",
			Turns:       5,
			EndedAt:     now,
		}
		if err := claude.AppendStatsRecord(rec); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		path, _ := claude.DailyStatsFilePath(now)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file should exist: %v", err)
		}
	})

	t.Run("appends second record to existing file", func(t *testing.T) {
		rec := claude.SessionStatsRecord{
			SessionName: "second-session",
			SessionID:   "uuid-2",
			Turns:       3,
			EndedAt:     now,
		}
		if err := claude.AppendStatsRecord(rec); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		path, _ := claude.DailyStatsFilePath(now)
		records, err := claude.ReadStatsFile(path)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("got %d records, want 2", len(records))
		}
		if records[1].SessionName != "second-session" {
			t.Errorf("got session_name %q, want second-session", records[1].SessionName)
		}
	})

	t.Run("writes valid JSON that round-trips", func(t *testing.T) {
		rec := claude.SessionStatsRecord{
			SessionName:         "round-trip",
			SessionID:           "uuid-rt",
			ProjectPath:         "/home/user/project",
			Turns:               10,
			ActiveTimeS:         600,
			TotalTimeS:          1200,
			InputTokens:         5000,
			OutputTokens:        2000,
			CacheCreationTokens: 100,
			CacheReadTokens:     3000,
			Models:              []string{"claude-sonnet-4-5-20250929"},
			ToolUses:            map[string]int{"Read": 5, "Bash": 3},
			PrevTurns:           3,
			PrevActiveTimeS:     200,
			PrevTotalTimeS:      400,
			PrevInputTokens:     1000,
			PrevOutputTokens:    500,
			EndedAt:             time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
		}
		if err := claude.AppendStatsRecord(rec); err != nil {
			t.Fatalf("write error: %v", err)
		}

		path, _ := claude.DailyStatsFilePath(rec.EndedAt)
		records, err := claude.ReadStatsFile(path)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("got %d records, want 1", len(records))
		}

		got := records[0]
		if got.SessionName != rec.SessionName {
			t.Errorf("SessionName: got %q, want %q", got.SessionName, rec.SessionName)
		}
		if got.Turns != rec.Turns {
			t.Errorf("Turns: got %d, want %d", got.Turns, rec.Turns)
		}
		if got.InputTokens != rec.InputTokens {
			t.Errorf("InputTokens: got %d, want %d", got.InputTokens, rec.InputTokens)
		}
		if got.ToolUses["Read"] != 5 {
			t.Errorf("ToolUses[Read]: got %d, want 5", got.ToolUses["Read"])
		}
		if got.PrevTurns != rec.PrevTurns {
			t.Errorf("PrevTurns: got %d, want %d", got.PrevTurns, rec.PrevTurns)
		}
	})
}

func TestReadStatsFile(t *testing.T) {
	t.Run("returns nil for non-existent file", func(t *testing.T) {
		records, err := claude.ReadStatsFile("/non/existent/file.jsonl")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if records != nil {
			t.Errorf("expected nil, got %v", records)
		}
	})

	t.Run("skips lines that fail JSON unmarshal", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.jsonl")

		content := `{"session_id":"good-1","session_name":"a","turns":1,"ended_at":"2026-03-13T18:00:00Z"}
this is not json
{"session_id":"good-2","session_name":"b","turns":2,"ended_at":"2026-03-13T18:00:00Z"}
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		records, err := claude.ReadStatsFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("got %d records, want 2", len(records))
		}
		if records[0].SessionID != "good-1" {
			t.Errorf("first record session_id: got %q, want good-1", records[0].SessionID)
		}
		if records[1].SessionID != "good-2" {
			t.Errorf("second record session_id: got %q, want good-2", records[1].SessionID)
		}
	})
}

func TestFindLastRecord(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	t.Run("returns latest record for session from today", func(t *testing.T) {
		rec1 := claude.SessionStatsRecord{SessionID: "uuid-a", Turns: 5, EndedAt: now}
		rec2 := claude.SessionStatsRecord{SessionID: "uuid-a", Turns: 10, EndedAt: now}
		_ = claude.AppendStatsRecord(rec1)
		_ = claude.AppendStatsRecord(rec2)

		found, err := claude.FindLastRecord("uuid-a", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found == nil {
			t.Fatal("expected record, got nil")
		}
		if found.Turns != 10 {
			t.Errorf("Turns: got %d, want 10", found.Turns)
		}
	})

	t.Run("falls back to previous days", func(t *testing.T) {
		yesterday := now.AddDate(0, 0, -1)
		rec := claude.SessionStatsRecord{SessionID: "uuid-b", Turns: 7, EndedAt: yesterday}
		_ = claude.AppendStatsRecord(rec)

		found, err := claude.FindLastRecord("uuid-b", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found == nil {
			t.Fatal("expected record, got nil")
		}
		if found.Turns != 7 {
			t.Errorf("Turns: got %d, want 7", found.Turns)
		}
	})

	t.Run("returns nil when no record found within 30 days", func(t *testing.T) {
		found, err := claude.FindLastRecord("uuid-nonexistent", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})
}
