package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/notify"
	"github.com/fgrehm/clotilde/internal/session"
)

// NotifyTabRenamer is the TabRenamer used by the notify hook.
// Overridable in tests.
var NotifyTabRenamer notify.TabRenamer = &notify.ZellijTabRenamer{}

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Handle Claude Code hook events (tab status, logging)",
	Long: `Called by Claude Code hooks (Stop, Notification, PreToolUse, PostToolUse, SessionEnd).
When running inside Zellij, updates the tab name with an emoji prefix reflecting session status.
Always appends the raw hook payload to /tmp/clotilde/<session-id>.events.jsonl for debugging.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read hook input: %w", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(input, &payload); err != nil {
			return fmt.Errorf("failed to parse hook input: %w", err)
		}

		sessionID, _ := payload["session_id"].(string)

		if err := notify.LogEvent(input, sessionID); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to log event: %v\n", err)
		}

		// Tab renaming: only when running inside Zellij
		if os.Getenv("ZELLIJ") == "" {
			return nil
		}

		hookEventName, _ := payload["hook_event_name"].(string)
		sessionName := resolveNotifySessionName(sessionID)
		if sessionName == "" {
			return nil
		}

		emoji := notify.EmojiForEvent(hookEventName, payload)
		if emoji == "" {
			// SessionEnd: skip rename (tab restore is a future task)
			return nil
		}

		tabName := emoji + " " + sessionName
		if err := NotifyTabRenamer.RenameTab(tabName); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to rename tab: %v\n", err)
		}

		return nil
	},
}

// resolveNotifySessionName resolves the clotilde session name for display.
// Uses a three-level fallback: env var, env file, reverse UUID lookup.
func resolveNotifySessionName(sessionID string) string {
	// Priority 1: CLOTILDE_SESSION_NAME env var
	if name := os.Getenv("CLOTILDE_SESSION_NAME"); name != "" {
		return name
	}

	// Priority 2: Read from CLAUDE_ENV_FILE
	if name := readSessionNameFromEnvFile(); name != "" {
		return name
	}

	// Priority 3: Reverse UUID lookup
	if sessionID != "" {
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			return ""
		}
		store := session.NewFileStore(clotildeRoot)
		name, err := findSessionByUUID(store, sessionID)
		if err != nil {
			return ""
		}
		return name
	}

	return ""
}

func init() {
	hookCmd.AddCommand(notifyCmd)
}
