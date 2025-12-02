package claude

import (
	"path/filepath"
	"strings"
)

// ProjectDir converts a clotilde root path to Claude Code's project directory format.
// Claude Code stores project data in ~/.claude/projects/<encoded-path>/
// where the path is encoded by replacing / and . with -
//
// Example:
//
//	/home/user/project/.claude/clotilde -> ~/.claude/projects/-home-user-project
func ProjectDir(clotildeRoot string) string {
	// Get the project root (parent of .claude/clotilde)
	projectRoot := filepath.Dir(filepath.Dir(clotildeRoot))

	// Convert path: replace / and . with -
	encoded := strings.ReplaceAll(projectRoot, "/", "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")

	return encoded
}

// TranscriptPath returns the path to a session's transcript file in Claude's storage.
// Format: ~/.claude/projects/<project-dir>/<session-id>.jsonl
func TranscriptPath(homeDir, clotildeRoot, sessionID string) string {
	projectDir := ProjectDir(clotildeRoot)
	return filepath.Join(homeDir, ".claude", "projects", projectDir, sessionID+".jsonl")
}

// AgentLogPattern returns a glob pattern for finding agent logs for a session.
// Format: ~/.claude/projects/<project-dir>/agent-*.jsonl
func AgentLogPattern(homeDir, clotildeRoot string) string {
	projectDir := ProjectDir(clotildeRoot)
	return filepath.Join(homeDir, ".claude", "projects", projectDir, "agent-*.jsonl")
}
