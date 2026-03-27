package claude

import (
	"fmt"
	"os/exec"
)

// IsInstalled checks if the claude CLI is available in PATH.
// Returns an error with helpful message if not found.
func IsInstalled() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH\n\n" +
			"Please install Claude Code first:\n" +
			"  Visit: https://code.claude.com/\n" +
			"  Or run: npm install -g @anthropic-ai/claude-code")
	}
	return nil
}

