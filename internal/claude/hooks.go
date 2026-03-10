package claude

import (
	"encoding/json"
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
	Stop         []HookMatcher `json:"Stop,omitempty"`
	Notification []HookMatcher `json:"Notification,omitempty"`
	PreToolUse   []HookMatcher `json:"PreToolUse,omitempty"`
	PostToolUse  []HookMatcher `json:"PostToolUse,omitempty"`
	SessionEnd   []HookMatcher `json:"SessionEnd,omitempty"`
}

// GenerateHookConfig generates the hook configuration for clotilde.
// Returns a HookConfig that should be merged into .claude/settings.json
func GenerateHookConfig(clotildeBinaryPath string) HookConfig {
	sessionStartCommand := fmt.Sprintf("%s hook sessionstart", clotildeBinaryPath)
	notifyCommand := fmt.Sprintf("%s hook notify", clotildeBinaryPath)

	notifyHook := func(matcher string) HookMatcher {
		m := HookMatcher{
			Hooks: []Hook{{Type: "command", Command: notifyCommand}},
		}
		if matcher != "" {
			m.Matcher = matcher
		}
		return m
	}

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
		Stop:         []HookMatcher{notifyHook("")},
		Notification: []HookMatcher{notifyHook("")},
		PreToolUse:   []HookMatcher{notifyHook(".*")},
		PostToolUse:  []HookMatcher{notifyHook(".*")},
		SessionEnd:   []HookMatcher{notifyHook("")},
	}
}

// HookConfigString returns the hooks as a formatted string for display.
func HookConfigString(config HookConfig) string {
	wrapper := struct {
		Hooks HookConfig `json:"hooks"`
	}{Hooks: config}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Sprintf("{error: %v}", err)
	}
	return string(data)
}
