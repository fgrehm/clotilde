package claude

import (
	"bufio"
	"bytes"
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

// forEachTailLine opens a transcript file, seeks to the last tailSize bytes,
// and calls fn for each complete JSONL line in the tail. Uses bufio.Reader with
// ReadSlice so that oversized lines are drained and skipped rather than halting
// the scan (unlike bufio.Scanner which stops permanently on ErrTooLong).
// Returns a non-nil error only for unexpected I/O failures.
func forEachTailLine(transcriptPath string, tailSize int, fn func(line []byte)) error {
	if transcriptPath == "" {
		return nil
	}

	file, err := os.Open(transcriptPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	skipFirstLine := false
	if info.Size() > int64(tailSize) {
		if _, err := file.Seek(info.Size()-int64(tailSize), io.SeekStart); err != nil {
			return err
		}
		check := make([]byte, 1)
		if _, err := file.ReadAt(check, info.Size()-int64(tailSize)-1); err == nil {
			skipFirstLine = check[0] != '\n'
		} else {
			skipFirstLine = true
		}
	}

	reader := bufio.NewReaderSize(file, tailSize)

	if skipFirstLine {
		// Drain partial first line (may span multiple ReadSlice calls).
		var drainErr error
		for {
			_, drainErr = reader.ReadSlice('\n')
			if !errors.Is(drainErr, bufio.ErrBufferFull) {
				break
			}
		}
		if drainErr == io.EOF {
			return nil
		}
		if drainErr != nil {
			return drainErr
		}
	}

	for {
		line, readErr := reader.ReadSlice('\n')
		if errors.Is(readErr, bufio.ErrBufferFull) {
			for errors.Is(readErr, bufio.ErrBufferFull) {
				_, readErr = reader.ReadSlice('\n')
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
			continue
		}
		line = bytes.TrimRight(line, "\r\n")
		if len(line) > 0 {
			fn(line)
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// ExtractLastModel reads the transcript and returns the last model used.
// Returns the model family name (e.g. "sonnet", "opus", "haiku") or empty string if not found.
//
// For large transcripts, only the last 128KB is read. Assistant entries that
// record message.model are typically small, so the most recent one will almost
// always be within the tail. A single assistant response larger than 128KB would
// be missed, but that is an accepted tradeoff for the performance benefit.
func ExtractLastModel(transcriptPath string) string {
	var lastModel string
	err := forEachTailLine(transcriptPath, 128*1024, func(line []byte) {
		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			if entry.Type == "assistant" && entry.Message.Model != "" {
				lastModel = entry.Message.Model
			}
		}
	})
	if err != nil {
		return ""
	}
	return FormatModelFamily(lastModel)
}

// FormatModelFamily extracts the model family name from the full model ID.
// e.g. "claude-sonnet-4-5-20250929" -> "sonnet"
func FormatModelFamily(fullModel string) string {
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
	type tsEntry struct {
		Timestamp time.Time `json:"timestamp"`
	}
	var last time.Time
	err := forEachTailLine(transcriptPath, 128*1024, func(line []byte) {
		var e tsEntry
		if err := json.Unmarshal(line, &e); err == nil {
			if !e.Timestamp.IsZero() {
				last = e.Timestamp
			}
		}
	})
	if err != nil {
		return time.Time{}
	}
	return last
}

// ExtractModelAndLastTime reads the transcript tail once and returns both the
// last model family name and the timestamp of the last entry. More efficient
// than calling ExtractLastModel and LastTranscriptTime separately.
// Returns empty string and zero time if the transcript is missing or unreadable.
func ExtractModelAndLastTime(transcriptPath string) (string, time.Time) {
	type entry struct {
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
		Message   struct {
			Model string `json:"model"`
		} `json:"message"`
	}
	var lastModel string
	var lastTime time.Time
	err := forEachTailLine(transcriptPath, 128*1024, func(line []byte) {
		var e entry
		if err := json.Unmarshal(line, &e); err == nil {
			if !e.Timestamp.IsZero() {
				lastTime = e.Timestamp
			}
			if e.Type == "assistant" && e.Message.Model != "" {
				lastModel = e.Message.Model
			}
		}
	})
	if err != nil {
		return "", time.Time{}
	}
	return FormatModelFamily(lastModel), lastTime
}

// TranscriptStats contains statistics about a session transcript.
type TranscriptStats struct {
	Turns           int
	FirstMessage    time.Time
	LastMessage     time.Time
	TotalTime       time.Duration
	ActiveTime      time.Duration
	AvgResponseTime time.Duration

	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	Models              []string       // deduplicated, first-appearance order
	ToolUses            map[string]int // tool name -> invocation count
}

// transcriptEntryForStats is used for parsing transcript entries for stats.
// Uses json.RawMessage to handle polymorphic message.content field.
type transcriptEntryForStats struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// contentBlock is used to extract tool_use blocks from message.content arrays.
type contentBlock struct {
	Type string `json:"type"`
	Name string `json:"name"`
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

	stats := &TranscriptStats{
		ToolUses: make(map[string]int),
	}

	modelSeen := make(map[string]bool)

	// Use bufio.Reader instead of bufio.Scanner so that oversized lines (e.g. large
	// tool outputs >1MB) are consumed and skipped rather than halting the scan entirely.
	// ReadSlice avoids allocating for lines that fit in the buffer; oversized lines
	// (ErrBufferFull) are drained and skipped so we never hold a huge []byte.
	// 1MB matches the old scanner max token size so entries up to 1MB are still parsed.
	reader := bufio.NewReaderSize(file, 1024*1024)

	var turnStart time.Time
	var lastAssistantTime time.Time

	for {
		line, readErr := reader.ReadSlice('\n')
		if errors.Is(readErr, bufio.ErrBufferFull) {
			// Line exceeds buffer size; discard the remainder and skip it.
			for errors.Is(readErr, bufio.ErrBufferFull) {
				_, readErr = reader.ReadSlice('\n')
			}
			// readErr is now nil (newline found) or io.EOF / other error.
			if readErr != nil && readErr != io.EOF {
				return nil, readErr
			}
			if readErr == io.EOF {
				break
			}
			continue
		}
		line = bytes.TrimRight(line, "\r\n")

		if len(line) > 0 {
			var entry transcriptEntryForStats
			if err := json.Unmarshal(line, &entry); err == nil {
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

				case "assistant":
					// Update last assistant time for this turn
					if !entry.Timestamp.IsZero() {
						lastAssistantTime = entry.Timestamp
					}

					// Token usage
					stats.InputTokens += entry.Message.Usage.InputTokens
					stats.OutputTokens += entry.Message.Usage.OutputTokens
					stats.CacheCreationTokens += entry.Message.Usage.CacheCreationInputTokens
					stats.CacheReadTokens += entry.Message.Usage.CacheReadInputTokens

					// Model tracking (deduplicated, first-appearance order)
					if m := entry.Message.Model; m != "" && !modelSeen[m] {
						modelSeen[m] = true
						stats.Models = append(stats.Models, m)
					}

					// Tool usage from content blocks
					if len(entry.Message.Content) > 0 && !isHumanTurn(entry.Message.Content) {
						var blocks []contentBlock
						if json.Unmarshal(entry.Message.Content, &blocks) == nil {
							for _, b := range blocks {
								if b.Type == "tool_use" && b.Name != "" {
									stats.ToolUses[b.Name]++
								}
							}
						}
					}
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
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

// MergeTranscriptStats merges multiple transcript stats into one.
// Sums counts (turns, tokens, tool uses, active time), takes earliest first
// message and latest last message, unions model lists preserving first-appearance
// order, and recomputes total time and average response time.
// Nil entries in the slice are skipped.
func MergeTranscriptStats(stats []*TranscriptStats) *TranscriptStats {
	merged := &TranscriptStats{
		ToolUses: make(map[string]int),
	}
	modelSeen := make(map[string]bool)

	for _, s := range stats {
		if s == nil {
			continue
		}

		merged.Turns += s.Turns
		merged.ActiveTime += s.ActiveTime
		merged.InputTokens += s.InputTokens
		merged.OutputTokens += s.OutputTokens
		merged.CacheCreationTokens += s.CacheCreationTokens
		merged.CacheReadTokens += s.CacheReadTokens

		// Earliest first message
		if !s.FirstMessage.IsZero() {
			if merged.FirstMessage.IsZero() || s.FirstMessage.Before(merged.FirstMessage) {
				merged.FirstMessage = s.FirstMessage
			}
		}

		// Latest last message
		if !s.LastMessage.IsZero() {
			if merged.LastMessage.IsZero() || s.LastMessage.After(merged.LastMessage) {
				merged.LastMessage = s.LastMessage
			}
		}

		// Union models preserving first-appearance order
		for _, m := range s.Models {
			if !modelSeen[m] {
				modelSeen[m] = true
				merged.Models = append(merged.Models, m)
			}
		}

		// Sum tool uses
		for tool, count := range s.ToolUses {
			merged.ToolUses[tool] += count
		}
	}

	// Recompute derived fields
	if !merged.FirstMessage.IsZero() && !merged.LastMessage.IsZero() {
		merged.TotalTime = merged.LastMessage.Sub(merged.FirstMessage)
	}
	if merged.Turns > 0 {
		merged.AvgResponseTime = merged.ActiveTime / time.Duration(merged.Turns)
	}

	return merged
}
