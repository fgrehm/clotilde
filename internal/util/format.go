package util

import (
	"fmt"
	"strings"
	"time"
)

// FormatSize converts bytes to human-readable format.
// Examples: 512 -> "512 B", 1536 -> "1.5 KB", 1048576 -> "1 MB"
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	divisor := int64(unit)
	for i := 0; i < 4; i++ {
		if bytes < divisor*unit {
			switch i {
			case 0:
				return fmt.Sprintf("%.1f KB", float64(bytes)/float64(divisor))
			case 1:
				return fmt.Sprintf("%.1f MB", float64(bytes)/float64(divisor))
			case 2:
				return fmt.Sprintf("%.1f GB", float64(bytes)/float64(divisor))
			case 3:
				return fmt.Sprintf("%.1f TB", float64(bytes)/float64(divisor))
			}
		}
		divisor *= unit
	}

	return fmt.Sprintf("%.1f TB", float64(bytes)/float64(divisor/unit))
}

// TruncateText truncates text to maxChars, replacing newlines with spaces.
// If text is longer than maxChars, appends "..." to indicate truncation.
// Examples: TruncateText("Hello\nWorld", 10) -> "Hello World"
//
//	TruncateText("Very long text here", 10) -> "Very long..."
func TruncateText(text string, maxChars int) string {
	// Replace newlines and collapse multiple spaces
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.Join(strings.Fields(text), " ")

	if len(text) <= maxChars {
		return text
	}

	return text[:maxChars-3] + "..."
}

// FormatRelativeTime formats a time as a human-readable relative string.
// Examples: "just now", "5 minutes ago", "2 hours ago", "3 days ago", "2024-01-15"
func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}

// FormatDuration formats a duration as a compact human-readable string.
// Examples: "0s", "15s", "2m 8s", "1h", "2h 5m"
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}

	if m > 0 {
		if s > 0 {
			return fmt.Sprintf("%dm %ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	}

	return fmt.Sprintf("%ds", s)
}
