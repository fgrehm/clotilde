package notify

// EmojiForEvent returns an emoji prefix for the given Claude Code hook event.
func EmojiForEvent(hookEventName string, data map[string]interface{}) string {
	switch hookEventName {
	case "Stop":
		return "\u2705" // check mark
	case "Notification":
		notifType, _ := data["notification_type"].(string)
		switch notifType {
		case "permission_prompt":
			return "\u26a0\ufe0f" // warning
		case "idle_prompt":
			return "\U0001f4a4" // zzz
		default:
			return "\u2753" // question mark
		}
	case "PreToolUse":
		return emojiForTool(data)
	case "PostToolUse":
		return "\U0001f914" // thinking face
	case "SessionEnd":
		return ""
	default:
		return "\U0001f535" // blue circle
	}
}

func emojiForTool(data map[string]interface{}) string {
	toolName, _ := data["tool_name"].(string)
	switch toolName {
	case "Bash":
		return "\u26a1" // lightning
	case "Read", "Grep", "Glob", "ToolSearch":
		return "\U0001f4d6" // open book
	case "Write", "Edit", "MultiEdit":
		return "\u270f\ufe0f" // pencil
	case "Task":
		return "\U0001f500" // shuffle
	case "WebSearch", "WebFetch":
		return "\U0001f310" // globe
	default:
		return "\u2699\ufe0f" // gear
	}
}
