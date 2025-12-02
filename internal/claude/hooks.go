package claude

import (
	"fmt"
)

// Hook represents a single hook command configuration.
type Hook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// HookMatcher represents a hook matcher with associated hooks.
type HookMatcher struct {
	Matcher string `json:"matcher,omitempty"`
	Hooks   []Hook `json:"hooks"`
}

// HookConfig represents the hook configuration for Claude Code settings.
type HookConfig struct {
	SessionStart []HookMatcher `json:"SessionStart,omitempty"`
}

// GenerateHookConfig generates the hook configuration for clotilde.
// Returns a HookConfig that should be merged into .claude/settings.json
func GenerateHookConfig(clotildeBinaryPath string) HookConfig {
	sessionStartCommand := fmt.Sprintf("%s hook sessionstart", clotildeBinaryPath)

	return HookConfig{
		SessionStart: []HookMatcher{
			{
				// No matcher field - handles all sources (startup, resume, compact, clear) internally
				Hooks: []Hook{
					{
						Type:    "command",
						Command: sessionStartCommand,
					},
				},
			},
		},
	}
}

// HookConfigString returns the hooks as a formatted string for display.
func HookConfigString(config HookConfig) string {
	return fmt.Sprintf(`{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "%s"
          }
        ]
      }
    ]
  }
}`, config.SessionStart[0].Hooks[0].Command)
}
