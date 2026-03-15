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

// GenerateNotifyHookConfig returns hook matchers for Stop, Notification, PreToolUse,
// PostToolUse, and SessionEnd, all pointing to `clotilde hook notify`.
// These are opt-in and registered only when a feature requires them (e.g. Zellij tab status).
func GenerateNotifyHookConfig(clotildeBinaryPath string) HookConfig {
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
