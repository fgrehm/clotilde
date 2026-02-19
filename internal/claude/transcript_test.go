package claude_test

import (
	"os"
	"path/filepath"
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
