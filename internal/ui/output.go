package ui

import (
	"fmt"
	"strings"
)

// Success renders a success message with a green checkmark
func Success(msg string) string {
	checkmark := SuccessStyle.Render("✓")
	return fmt.Sprintf("%s %s", checkmark, msg)
}

// Error renders an error message in a red bordered box
func Error(msg string) string {
	// Format the message with the error icon
	icon := ErrorStyle.Render("✗")
	content := fmt.Sprintf("%s %s", icon, msg)

	// Wrap in error box
	return ErrorBoxStyle.Render(content)
}

// Warning renders a warning message with a yellow warning icon
func Warning(msg string) string {
	icon := WarningStyle.Render("⚠")
	return fmt.Sprintf("%s %s", icon, msg)
}

// Info renders an info message with a blue info icon
func Info(msg string) string {
	icon := InfoStyle.Render("ℹ")
	return fmt.Sprintf("%s %s", icon, msg)
}

// ErrorWithDetails renders an error message with optional detail lines
func ErrorWithDetails(msg string, details []string) string {
	icon := ErrorStyle.Render("✗")
	lines := []string{fmt.Sprintf("%s %s", icon, msg)}

	if len(details) > 0 {
		lines = append(lines, "")
		for _, detail := range details {
			lines = append(lines, "  "+DimStyle.Render(detail))
		}
	}

	return ErrorBoxStyle.Render(strings.Join(lines, "\n"))
}
