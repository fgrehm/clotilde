package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
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
