package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/notify"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Log Claude Code hook events for debugging",
	Long: `Called by Claude Code hooks (Stop, Notification, PreToolUse, PostToolUse, SessionEnd).
Appends the raw hook payload to /tmp/clotilde/<session-id>.events.jsonl for debugging.`,
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

		return notify.LogEvent(input, sessionID)
	},
}

func init() {
	hookCmd.AddCommand(notifyCmd)
}
