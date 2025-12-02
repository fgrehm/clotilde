package claude

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fgrehm/clotilde/internal/util"
)

// DeletedFiles contains information about files deleted during cleanup.
type DeletedFiles struct {
	Transcript []string // Transcript file paths that were deleted
	AgentLogs  []string // Agent log file paths that were deleted
}

// DeleteSessionData removes Claude Code transcript and agent logs for a session.
// If transcriptPath is provided, uses it directly. Otherwise computes it from clotildeRoot.
// Returns DeletedFiles with info about what was deleted.
func DeleteSessionData(clotildeRoot, sessionID, transcriptPath string) (*DeletedFiles, error) {
	deleted := &DeletedFiles{
		Transcript: []string{},
		AgentLogs:  []string{},
	}

	var claudeProjectDir string

	// If transcript path is provided, use it directly
	if transcriptPath != "" {
		// Delete transcript file
		if util.FileExists(transcriptPath) {
			if err := os.Remove(transcriptPath); err != nil {
				return deleted, fmt.Errorf("failed to delete transcript: %w", err)
			}
			deleted.Transcript = append(deleted.Transcript, transcriptPath)
		}
		// Get project directory from transcript path
		claudeProjectDir = filepath.Dir(transcriptPath)
	} else {
		// Fall back to computing the path
		projectDir := ProjectDir(clotildeRoot)
		homeDir, err := util.HomeDir()
		if err != nil {
			return deleted, fmt.Errorf("failed to get home directory: %w", err)
		}

		claudeProjectDir = filepath.Join(homeDir, ".claude", "projects", projectDir)

		// Delete transcript file
		transcriptPath := filepath.Join(claudeProjectDir, sessionID+".jsonl")
		if util.FileExists(transcriptPath) {
			if err := os.Remove(transcriptPath); err != nil {
				return deleted, fmt.Errorf("failed to delete transcript: %w", err)
			}
			deleted.Transcript = append(deleted.Transcript, transcriptPath)
		}
	}

	// Delete agent logs that reference this session
	agentLogs, err := deleteAgentLogs(claudeProjectDir, sessionID)
	if err != nil {
		return deleted, err
	}
	deleted.AgentLogs = agentLogs

	return deleted, nil
}

// deleteAgentLogs finds and deletes agent log files that reference the given sessionId.
// Returns a list of deleted agent log file paths.
func deleteAgentLogs(claudeProjectDir, sessionID string) ([]string, error) {
	deletedLogs := []string{}

	// Check if project directory exists
	if !util.DirExists(claudeProjectDir) {
		return deletedLogs, nil // Nothing to delete
	}

	// Find all agent-*.jsonl files
	pattern := filepath.Join(claudeProjectDir, "agent-*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return deletedLogs, fmt.Errorf("failed to find agent logs: %w", err)
	}

	// Check each file for references to sessionId
	for _, logPath := range matches {
		containsSession, err := fileContainsSessionID(logPath, sessionID)
		if err != nil {
			// Log warning but continue with other files
			fmt.Fprintf(os.Stderr, "Warning: failed to check %s: %v\n", logPath, err)
			continue
		}

		if containsSession {
			if err := os.Remove(logPath); err != nil {
				return deletedLogs, fmt.Errorf("failed to delete agent log %s: %w", logPath, err)
			}
			deletedLogs = append(deletedLogs, logPath)
		}
	}

	return deletedLogs, nil
}

// fileContainsSessionID checks if a file contains a reference to the given sessionId.
func fileContainsSessionID(path, sessionID string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), sessionID) {
			return true, nil
		}
	}

	return false, scanner.Err()
}
