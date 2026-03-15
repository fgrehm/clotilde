package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionStatsRecord represents a single stats entry in the daily JSONL file.
type SessionStatsRecord struct {
	SessionName  string `json:"session_name"`
	SessionID    string `json:"session_id"`
	ProjectPath  string `json:"project_path"`
	Turns        int    `json:"turns"`
	ActiveTimeS  int    `json:"active_time_s"`
	TotalTimeS   int    `json:"total_time_s"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`

	CacheCreationTokens int            `json:"cache_creation_tokens"`
	CacheReadTokens     int            `json:"cache_read_tokens"`
	Models              []string       `json:"models"`
	ToolUses            map[string]int `json:"tool_uses"`

	PrevTurns               int            `json:"prev_turns"`
	PrevActiveTimeS         int            `json:"prev_active_time_s"`
	PrevTotalTimeS          int            `json:"prev_total_time_s"`
	PrevInputTokens         int            `json:"prev_input_tokens"`
	PrevOutputTokens        int            `json:"prev_output_tokens"`
	PrevCacheCreationTokens int            `json:"prev_cache_creation_tokens"`
	PrevCacheReadTokens     int            `json:"prev_cache_read_tokens"`
	PrevToolUses            map[string]int `json:"prev_tool_uses"`

	EndedAt time.Time `json:"ended_at"`
}

// StatsDir returns the directory for stats files.
// Uses $XDG_DATA_HOME/clotilde/stats or ~/.local/share/clotilde/stats.
func StatsDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine home directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "clotilde", "stats"), nil
}

// DailyStatsFileName returns the filename for the daily stats file for the given date.
// Always normalizes to UTC to ensure consistent naming across timezones.
func DailyStatsFileName(t time.Time) string {
	return t.UTC().Format("2006-01-02") + ".jsonl"
}

// DailyStatsFilePath returns the full path for the daily stats file for the given date.
// Always normalizes to UTC to ensure consistent file naming across timezones.
func DailyStatsFilePath(t time.Time) (string, error) {
	dir, err := StatsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DailyStatsFileName(t)), nil
}

// AppendStatsRecord marshals the record to JSON and appends it to the daily stats file.
// Creates the directory and file if they don't exist.
func AppendStatsRecord(record SessionStatsRecord) error {
	path, err := DailyStatsFilePath(record.EndedAt)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create stats directory: %w", err)
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal stats record: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open stats file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write stats record: %w", err)
	}
	return nil
}

// ReadStatsFile reads all records from a daily stats file.
// Skips lines that fail JSON unmarshal (handles truncated writes).
// Returns nil, nil if the file doesn't exist.
func ReadStatsFile(path string) ([]SessionStatsRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var records []SessionStatsRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line length
	for scanner.Scan() {
		var rec SessionStatsRecord
		if json.Unmarshal(scanner.Bytes(), &rec) == nil {
			records = append(records, rec)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// FindLastRecord scans daily stats files from today back up to 30 days,
// returning the most recent record matching sessionID.
// Returns nil (no error) if no prior record is found.
func FindLastRecord(sessionID string, now time.Time) (*SessionStatsRecord, error) {
	return FindLastRecordDays(sessionID, now, 30)
}

// FindLastRecordDays scans daily stats files from today back the given number
// of days, returning the most recent record matching sessionID.
// Returns nil (no error) if no prior record is found.
func FindLastRecordDays(sessionID string, now time.Time, days int) (*SessionStatsRecord, error) {
	now = now.UTC()
	for daysBack := range days + 1 {
		date := now.AddDate(0, 0, -daysBack)
		path, err := DailyStatsFilePath(date)
		if err != nil {
			return nil, err
		}

		records, err := ReadStatsFile(path)
		if err != nil {
			return nil, err
		}

		// Scan backwards to find the latest match
		for i := len(records) - 1; i >= 0; i-- {
			if records[i].SessionID == sessionID {
				return &records[i], nil
			}
		}
	}
	return nil, nil
}

// ConsolidateStatsFile deduplicates a daily file by session_id, keeping only
// the last record per session. Writes to a temp file then atomically renames.
// Returns the deduplicated records.
func ConsolidateStatsFile(path string) ([]SessionStatsRecord, error) {
	records, err := ReadStatsFile(path)
	if err != nil {
		return nil, err
	}

	// Deduplicate: keep last occurrence per session_id
	seen := make(map[string]int) // session_id -> index in deduped
	var deduped []SessionStatsRecord
	for _, rec := range records {
		if idx, ok := seen[rec.SessionID]; ok {
			deduped[idx] = rec
		} else {
			seen[rec.SessionID] = len(deduped)
			deduped = append(deduped, rec)
		}
	}

	// Write to temp file in same directory for atomic rename
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmpPath := filepath.Join(dir, "."+base+".tmp")

	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	for _, rec := range deduped {
		data, err := json.Marshal(rec)
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("failed to marshal record: %w", err)
		}
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("failed to write record: %w", err)
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return deduped, nil
}
