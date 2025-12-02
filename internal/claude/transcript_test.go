package claude_test

import (
	"os"
	"path/filepath"
	"testing"

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

			if err := os.WriteFile(transcriptPath, []byte(tt.transcriptData), 0644); err != nil {
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
