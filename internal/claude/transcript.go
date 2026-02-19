package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"time"
)

// transcriptEntry represents a single line in the Claude Code transcript JSONL.
type transcriptEntry struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
	} `json:"message"`
}

var modelFamilyRegex = regexp.MustCompile(`claude-(?:\d+-)*(\w+)-\d+`)

// ExtractLastModel reads the transcript and returns the last model used.
// Returns the model family name (e.g. "sonnet", "opus", "haiku") or empty string if not found.
func ExtractLastModel(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	// Read the entire file and find the last assistant message with a model
	// For large files, we could optimize by seeking to end and reading backwards,
	// but for now we'll read forward and keep the last match
	var lastModel string
	scanner := bufio.NewScanner(file)

	// Increase buffer size to handle large lines in transcripts
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed lines
			continue
		}

		// Look for assistant messages with model field
		if entry.Type == "assistant" && entry.Message.Model != "" {
			lastModel = entry.Message.Model
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return ""
	}

	// Extract model family name using regex
	return formatModelFamily(lastModel)
}

// formatModelFamily extracts the model family name from the full model ID.
// e.g. "claude-sonnet-4-5-20250929" -> "sonnet"
func formatModelFamily(fullModel string) string {
	if fullModel == "" {
		return ""
	}

	matches := modelFamilyRegex.FindStringSubmatch(fullModel)
	if len(matches) > 1 {
		return matches[1] // Return the captured family name
	}

	// Fallback: return full model if regex doesn't match
	return fullModel
}

// TranscriptStats contains statistics about a session transcript.
type TranscriptStats struct {
	Turns           int
	FirstMessage    time.Time
	LastMessage     time.Time
	TotalTime       time.Duration
	ActiveTime      time.Duration
	AvgResponseTime time.Duration
}

// transcriptEntryForStats is used for parsing transcript entries for stats.
// Uses json.RawMessage to handle polymorphic message.content field.
type transcriptEntryForStats struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// isHumanTurn checks if a message content is a human turn (string) vs tool result (array).
// Single byte check: if first non-whitespace byte is '[', it's a tool result array.
func isHumanTurn(content json.RawMessage) bool {
	for _, b := range content {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b != '['
	}
	return false
}

// ParseTranscriptStats reads a transcript JSONL file and returns statistics.
// Returns an error if the file can't be opened or read.
func ParseTranscriptStats(transcriptPath string) (*TranscriptStats, error) {
	if transcriptPath == "" {
		return &TranscriptStats{}, nil
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	stats := &TranscriptStats{}

	scanner := bufio.NewScanner(file)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var turnStart time.Time
	var lastAssistantTime time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry transcriptEntryForStats
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed lines
			continue
		}

		// Track the overall max timestamp as LastMessage
		if !entry.Timestamp.IsZero() {
			if stats.LastMessage.IsZero() || entry.Timestamp.After(stats.LastMessage) {
				stats.LastMessage = entry.Timestamp
			}
		}

		switch entry.Type {
		case "progress":
			// First progress event marks session start
			if stats.FirstMessage.IsZero() && !entry.Timestamp.IsZero() {
				stats.FirstMessage = entry.Timestamp
			}

		case "user":
			// Check if this is a human turn (string content) vs tool result (array content)
			if len(entry.Message.Content) > 0 && isHumanTurn(entry.Message.Content) {
				// Finalize previous turn if any
				if !turnStart.IsZero() && !lastAssistantTime.IsZero() {
					stats.ActiveTime += lastAssistantTime.Sub(turnStart)
				}

				// Start new turn
				turnStart = entry.Timestamp
				lastAssistantTime = time.Time{}
				stats.Turns++
			}
			// If it's a tool result (array), skip it

		case "assistant":
			// Update last assistant time for this turn
			if !entry.Timestamp.IsZero() {
				lastAssistantTime = entry.Timestamp
			}
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	// Finalize last open turn
	if !turnStart.IsZero() && !lastAssistantTime.IsZero() {
		stats.ActiveTime += lastAssistantTime.Sub(turnStart)
	}

	// Calculate total time and average response time
	if !stats.FirstMessage.IsZero() && !stats.LastMessage.IsZero() {
		stats.TotalTime = stats.LastMessage.Sub(stats.FirstMessage)
	}

	if stats.Turns > 0 {
		stats.AvgResponseTime = stats.ActiveTime / time.Duration(stats.Turns)
	}

	return stats, nil
}
