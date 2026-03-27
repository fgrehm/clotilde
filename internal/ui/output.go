package ui

import "fmt"

// Success renders a success message with a green checkmark
func Success(msg string) string {
	checkmark := SuccessStyle.Render("✓")
	return fmt.Sprintf("%s %s", checkmark, msg)
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

