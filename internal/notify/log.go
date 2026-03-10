package notify

import (
	"fmt"
	"os"
	"path/filepath"
)

// LogDir is the directory where event JSONL files are written.
// Overridable in tests.
var LogDir = "/tmp/clotilde"

// LogEvent appends rawJSON as a line to <LogDir>/<sessionID>.events.jsonl.
// Creates the log directory if it doesn't exist. No-op if sessionID is empty.
func LogEvent(rawJSON []byte, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	if err := os.MkdirAll(LogDir, 0o755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(LogDir, sessionID+".events.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(rawJSON); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}

	return nil
}
