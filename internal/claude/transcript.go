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

// newTranscriptScanner returns a bufio.Scanner configured for transcript JSONL reading.
// It starts with a 64KB buffer and allows lines up to 1MB before returning ErrTooLong.
func newTranscriptScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	return scanner
}

// ExtractLastModel reads the transcript and returns the last model used.
// Returns the model family name (e.g. "sonnet", "opus", "haiku") or empty string if not found.
//
// For large transcripts, only the last 128KB is read. Assistant entries that
// record message.model are typically small, so the most recent one will almost
// always be within the tail. A single assistant response larger than 128KB would
// be missed, but that is an accepted tradeoff for the performance benefit.
func ExtractLastModel(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return ""
	}

	const tailSize = 128 * 1024 // 128KB
	skipFirstLine := false
	if info.Size() > tailSize {
		if _, err := file.Seek(info.Size()-tailSize, io.SeekStart); err != nil {
			return ""
		}
		// If the byte just before the seek point is not '\n', the first scanned
		// line will be partial and must be discarded. Check before creating the
		// scanner so all file I/O is sequenced clearly.
		check := make([]byte, 1)
		if _, err := file.ReadAt(check, info.Size()-tailSize-1); err == nil {
			skipFirstLine = check[0] != '\n'
		} else {
			skipFirstLine = true // can't verify boundary; assume partial
		}
	}

	scanner := newTranscriptScanner(file)

	if skipFirstLine {
		scanner.Scan() // discard partial first line
	}

	var lastModel string
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Type == "assistant" && entry.Message.Model != "" {
			lastModel = entry.Message.Model
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrTooLong) {
		return ""
	}

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

// LastTranscriptTime returns the timestamp of the last entry in a transcript file.
// Only the tail of the file is read for efficiency (same technique as ExtractLastModel).
// Returns zero time if the file can't be opened or contains no timestamped entries.
func LastTranscriptTime(transcriptPath string) time.Time {
	if transcriptPath == "" {
		return time.Time{}
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return time.Time{}
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return time.Time{}
	}

	const tailSize = 128 * 1024 // 128KB — matches ExtractLastModel; lines larger than this are an accepted tradeoff
	skipFirstLine := false
	if info.Size() > tailSize {
		if _, err := file.Seek(info.Size()-tailSize, io.SeekStart); err != nil {
			return time.Time{}
		}
		check := make([]byte, 1)
		if _, err := file.ReadAt(check, info.Size()-tailSize-1); err == nil {
			skipFirstLine = check[0] != '\n'
		} else {
			skipFirstLine = true
		}
	}

	scanner := newTranscriptScanner(file)

	if skipFirstLine {
		scanner.Scan() // discard partial first line
	}

	type entry struct {
		Timestamp time.Time `json:"timestamp"`
	}

	var last time.Time
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if !e.Timestamp.IsZero() {
			last = e.Timestamp
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrTooLong) {
		return time.Time{}
	}

	return last
}

// ExtractModelAndLastTime reads the transcript tail once and returns both the
// last model family name and the timestamp of the last entry. More efficient
// than calling ExtractLastModel and LastTranscriptTime separately.
// Returns empty string and zero time if the transcript is missing or unreadable.
func ExtractModelAndLastTime(transcriptPath string) (string, time.Time) {
	if transcriptPath == "" {
		return "", time.Time{}
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", time.Time{}
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", time.Time{}
	}

	const tailSize = 128 * 1024 // 128KB
	skipFirstLine := false
	if info.Size() > tailSize {
		if _, err := file.Seek(info.Size()-tailSize, io.SeekStart); err != nil {
			return "", time.Time{}
		}
		check := make([]byte, 1)
		if _, err := file.ReadAt(check, info.Size()-tailSize-1); err == nil {
			skipFirstLine = check[0] != '\n'
		} else {
			skipFirstLine = true
		}
	}

	scanner := newTranscriptScanner(file)

	if skipFirstLine {
		scanner.Scan() // discard partial first line
	}

	type entry struct {
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
		Message   struct {
			Model string `json:"model"`
		} `json:"message"`
	}

	var lastModel string
	var lastTime time.Time
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if !e.Timestamp.IsZero() {
			lastTime = e.Timestamp
		}
		if e.Type == "assistant" && e.Message.Model != "" {
			lastModel = e.Message.Model
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrTooLong) {
		return "", time.Time{}
	}

	return formatModelFamily(lastModel), lastTime
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

	scanner := newTranscriptScanner(file)

	var turnStart time.Time
	var lastAssistantTime time.Time

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntryForStats
		if err := json.Unmarshal(line, &entry); err != nil {
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

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrTooLong) {
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
