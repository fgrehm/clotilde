package claude

import "fmt"

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
	Stop         []HookMatcher `json:"Stop,omitempty"`
	Notification []HookMatcher `json:"Notification,omitempty"`
	PreToolUse   []HookMatcher `json:"PreToolUse,omitempty"`
	PostToolUse  []HookMatcher `json:"PostToolUse,omitempty"`
	SessionEnd   []HookMatcher `json:"SessionEnd,omitempty"`
}

// HookConfigOptions controls which optional hooks are generated.
type HookConfigOptions struct {
	StatsEnabled bool
}

// GenerateHookConfig generates the hook configuration for clotilde.
// Returns a HookConfig that should be merged into .claude/settings.json.
// When opts.StatsEnabled is true, includes a SessionEnd hook for stats recording.
func GenerateHookConfig(clotildeBinaryPath string, opts HookConfigOptions) HookConfig {
	sessionStartCommand := fmt.Sprintf("%s hook sessionstart", clotildeBinaryPath)

	config := HookConfig{
		SessionStart: []HookMatcher{
			{
				Hooks: []Hook{
					{
						Type:    "command",
						Command: sessionStartCommand,
					},
				},
			},
		},
	}

	if opts.StatsEnabled {
		sessionEndCommand := fmt.Sprintf("%s hook sessionend", clotildeBinaryPath)
		config.SessionEnd = []HookMatcher{
			{
				Hooks: []Hook{
					{
						Type:    "command",
						Command: sessionEndCommand,
					},
				},
			},
		}
	}

	return config
}

