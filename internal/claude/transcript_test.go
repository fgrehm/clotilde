package claude_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fgrehm/clotilde/internal/claude"
)

func TestExtractLastModel(t *testing.T) {
	tests := []struct {
		name           string
		transcriptData string
		expectedModel  string
	}{
		{
			name: "sonnet model",
			transcriptData: `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":"hi"}}`,
			expectedModel: "sonnet",
		},
		{
			name: "opus model",
			transcriptData: `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-opus-4-20250514","role":"assistant","content":"hi"}}`,
			expectedModel: "opus",
		},
		{
			name: "haiku model",
			transcriptData: `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-3-5-haiku-20250219","role":"assistant","content":"hi"}}`,
			expectedModel: "haiku",
		},
		{
			name: "multiple messages - returns last",
			transcriptData: `{"type":"assistant","message":{"model":"claude-opus-4-20250514","role":"assistant","content":"first"}}
{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":"second"}}`,
			expectedModel: "sonnet",
		},
		{
			name:           "no assistant messages",
			transcriptData: `{"type":"user","message":{"role":"user","content":"hello"}}`,
			expectedModel:  "",
		},
		{
			name:           "empty transcript",
			transcriptData: "",
			expectedModel:  "",
		},
		{
			name: "mixed entries with non-assistant types",
			transcriptData: `{"type":"file-history-snapshot","messageId":"abc"}
{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":"hi"}}
{"type":"summary","summary":"test"}`,
			expectedModel: "sonnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with transcript data
			tmpDir := t.TempDir()
			transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

			if err := os.WriteFile(transcriptPath, []byte(tt.transcriptData), 0o644); err != nil {
				t.Fatalf("Failed to write test transcript: %v", err)
			}

			result := claude.ExtractLastModel(transcriptPath)
			if result != tt.expectedModel {
				t.Errorf("Expected model %q, got %q", tt.expectedModel, result)
			}
		})
	}
}

func TestExtractLastModel_LargeFile(t *testing.T) {
	// Verify that the tail-read optimization finds the last model in a file > 128KB.
	// An early "opus" message is buried before the 128KB tail;
	// a later "sonnet" message sits in the tail.
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "large.jsonl")

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	writeLine := func(line string) {
		t.Helper()
		if _, err := fmt.Fprintln(f, line); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	writeLine(`{"type":"assistant","message":{"model":"claude-opus-4-20250514","role":"assistant","content":"early"}}`)

	// 2500 lines × ~63 bytes ≈ 157KB – pushes the opus entry before the 128KB tail window.
	userLine := `{"type":"user","message":{"role":"user","content":"padding"}}`
	for range 2500 {
		writeLine(userLine)
	}

	writeLine(`{"type":"assistant","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":"last"}}`)

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	result := claude.ExtractLastModel(transcriptPath)
	if result != "sonnet" {
		t.Errorf("got %q, want %q", result, "sonnet")
	}
}

func TestExtractLastModel_NonExistentFile(t *testing.T) {
	result := claude.ExtractLastModel("/non/existent/path")
	if result != "" {
		t.Errorf("Expected empty string for non-existent file, got %q", result)
	}
}

func TestExtractLastModel_EmptyPath(t *testing.T) {
	result := claude.ExtractLastModel("")
	if result != "" {
		t.Errorf("Expected empty string for empty path, got %q", result)
	}
}

func TestExtractModelAndLastTime(t *testing.T) {
	tests := []struct {
		name          string
		transcript    string
		expectedModel string
		expectTime    bool // whether we expect a non-zero timestamp
	}{
		{
			name: "returns model and timestamp from assistant entry",
			transcript: `{"type":"user","timestamp":"2025-01-01T10:00:00Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-01-01T10:00:05Z","message":{"model":"claude-sonnet-4-5-20250929","content":"hi"}}`,
			expectedModel: "sonnet",
			expectTime:    true,
		},
		{
			name: "last timestamp wins even if not on assistant entry",
			transcript: `{"type":"assistant","timestamp":"2025-01-01T10:00:05Z","message":{"model":"claude-sonnet-4-5-20250929","content":"hi"}}
{"type":"user","timestamp":"2025-01-01T10:01:00Z","message":{"content":"follow-up"}}`,
			expectedModel: "sonnet",
			expectTime:    true,
		},
		{
			name: "last assistant model wins",
			transcript: `{"type":"assistant","timestamp":"2025-01-01T10:00:00Z","message":{"model":"claude-opus-4-20250514","content":"first"}}
{"type":"user","timestamp":"2025-01-01T10:00:10Z","message":{"content":"ok"}}
{"type":"assistant","timestamp":"2025-01-01T10:00:20Z","message":{"model":"claude-sonnet-4-5-20250929","content":"second"}}`,
			expectedModel: "sonnet",
			expectTime:    true,
		},
		{
			name:          "empty transcript",
			transcript:    "",
			expectedModel: "",
			expectTime:    false,
		},
		{
			name:          "no assistant entries",
			transcript:    `{"type":"user","timestamp":"2025-01-01T10:00:00Z","message":{"content":"hello"}}`,
			expectedModel: "",
			expectTime:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "transcript.jsonl")
			if err := os.WriteFile(path, []byte(tt.transcript), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}

			model, ts := claude.ExtractModelAndLastTime(path)
			if model != tt.expectedModel {
				t.Errorf("model: got %q, want %q", model, tt.expectedModel)
			}
			if tt.expectTime && ts.IsZero() {
				t.Error("expected non-zero timestamp, got zero")
			}
			if !tt.expectTime && !ts.IsZero() {
				t.Errorf("expected zero timestamp, got %v", ts)
			}
		})
	}
}

func TestExtractModelAndLastTime_NonExistentFile(t *testing.T) {
	model, ts := claude.ExtractModelAndLastTime("/non/existent/path")
	if model != "" {
		t.Errorf("expected empty model, got %q", model)
	}
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}
}

func TestExtractModelAndLastTime_EmptyPath(t *testing.T) {
	model, ts := claude.ExtractModelAndLastTime("")
	if model != "" {
		t.Errorf("expected empty model, got %q", model)
	}
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}
}

func TestParseTranscriptStats(t *testing.T) {
	tests := []struct {
		name              string
		transcriptData    string
		expectedTurns     int
		expectedZeroTimes bool // True if FirstMessage/LastMessage should be zero
		validateTimes     func(t *testing.T, stats *claude.TranscriptStats)
	}{
		{
			name: "single human turn with assistant response",
			transcriptData: `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:15Z","message":{"content":"hi"}}`,
			expectedTurns:     1,
			expectedZeroTimes: false,
		},
		{
			name: "multiple human turns",
			transcriptData: `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"first"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:15Z","message":{"content":"response1"}}
{"type":"user","timestamp":"2025-02-17T20:36:00Z","message":{"content":"second"}}
{"type":"assistant","timestamp":"2025-02-17T20:36:10Z","message":{"content":"response2"}}
{"type":"user","timestamp":"2025-02-17T20:37:00Z","message":{"content":"third"}}
{"type":"assistant","timestamp":"2025-02-17T20:37:20Z","message":{"content":"response3"}}`,
			expectedTurns:     3,
			expectedZeroTimes: false,
		},
		{
			name: "tool result should not count as turn",
			transcriptData: `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:15Z","message":{"content":"hi"}}
{"type":"user","timestamp":"2025-02-17T20:35:20Z","message":{"content":[{"type": "tool_result"}]}}
{"type":"assistant","timestamp":"2025-02-17T20:35:25Z","message":{"content":"understood"}}`,
			expectedTurns:     1,
			expectedZeroTimes: false,
		},
		{
			name:              "empty transcript",
			transcriptData:    "",
			expectedTurns:     0,
			expectedZeroTimes: true,
		},
		{
			name:              "progress only",
			transcriptData:    `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}`,
			expectedTurns:     0,
			expectedZeroTimes: false,
		},
		{
			name: "correct active time calculation",
			transcriptData: `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"first"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:20Z","message":{"content":"response1"}}
{"type":"user","timestamp":"2025-02-17T20:36:00Z","message":{"content":"second"}}
{"type":"assistant","timestamp":"2025-02-17T20:36:15Z","message":{"content":"response2"}}`,
			expectedTurns:     2,
			expectedZeroTimes: false,
			validateTimes: func(t *testing.T, stats *claude.TranscriptStats) {
				// Turn 1: 20:35:20 - 20:35:10 = 10 seconds
				// Turn 2: 20:36:15 - 20:36:00 = 15 seconds
				// Total active: 25 seconds
				expectedActive := 25 * time.Second
				if stats.ActiveTime != expectedActive {
					t.Errorf("Expected active time %v, got %v", expectedActive, stats.ActiveTime)
				}
				// Average: 25s / 2 = 12.5s
				expectedAvg := 12500 * time.Millisecond
				if stats.AvgResponseTime != expectedAvg {
					t.Errorf("Expected avg response time %v, got %v", expectedAvg, stats.AvgResponseTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with transcript data
			tmpDir := t.TempDir()
			transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

			if err := os.WriteFile(transcriptPath, []byte(tt.transcriptData), 0o644); err != nil {
				t.Fatalf("Failed to write test transcript: %v", err)
			}

			stats, err := claude.ParseTranscriptStats(transcriptPath)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if stats.Turns != tt.expectedTurns {
				t.Errorf("Expected %d turns, got %d", tt.expectedTurns, stats.Turns)
			}

			if tt.expectedZeroTimes {
				if !stats.FirstMessage.IsZero() {
					t.Errorf("Expected zero FirstMessage, got %v", stats.FirstMessage)
				}
				if !stats.LastMessage.IsZero() {
					t.Errorf("Expected zero LastMessage, got %v", stats.LastMessage)
				}
			} else {
				if stats.FirstMessage.IsZero() {
					t.Errorf("Expected non-zero FirstMessage")
				}
				if stats.LastMessage.IsZero() {
					t.Errorf("Expected non-zero LastMessage")
				}
			}

			if tt.validateTimes != nil {
				tt.validateTimes(t, stats)
			}
		})
	}
}

func TestParseTranscriptStats_NonExistentFile(t *testing.T) {
	_, err := claude.ParseTranscriptStats("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestParseTranscriptStats_EmptyPath(t *testing.T) {
	stats, err := claude.ParseTranscriptStats("")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if stats.Turns != 0 {
		t.Errorf("Expected 0 turns, got %d", stats.Turns)
	}
}

func TestParseTranscriptStats_OversizedLine(t *testing.T) {
	// Verify that a line exceeding the 1MB scanner limit doesn't cause an error —
	// ParseTranscriptStats should return partial results from lines processed before
	// the oversized one rather than failing entirely.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	// Write a valid turn before the giant line so we can verify partial results.
	prefix := `{"type":"progress","timestamp":"2025-01-01T10:00:00Z"}
{"type":"user","timestamp":"2025-01-01T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-01-01T10:00:15Z","message":{"content":"hi"}}
`
	// A single JSON line with >1MB content field — triggers bufio.ErrTooLong.
	hugeLine := fmt.Sprintf(`{"type":"user","message":{"content":"%s"}}`, strings.Repeat("x", 1024*1024+1))
	data := prefix + hugeLine + "\n"

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	stats, err := claude.ParseTranscriptStats(path)
	if err != nil {
		t.Fatalf("expected no error on oversized line, got: %v", err)
	}
	// We should have captured the one turn that precedes the giant line.
	if stats.Turns != 1 {
		t.Errorf("expected 1 turn, got %d", stats.Turns)
	}
}
