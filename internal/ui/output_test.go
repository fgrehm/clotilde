package ui

import (
	"strings"
	"testing"
)

func TestSuccess(t *testing.T) {
	result := Success("Operation completed")
	if result == "" {
		t.Error("Success() returned empty string")
	}
	if !strings.Contains(result, "Operation completed") {
		t.Error("Success() did not include the message")
	}
	if !strings.Contains(result, "✓") {
		t.Error("Success() did not include checkmark")
	}
}

func TestError(t *testing.T) {
	result := Error("Something went wrong")
	if result == "" {
		t.Error("Error() returned empty string")
	}
	if !strings.Contains(result, "Something went wrong") {
		t.Error("Error() did not include the message")
	}
	if !strings.Contains(result, "✗") {
		t.Error("Error() did not include error icon")
	}
	// Error messages should be in a box (multi-line)
	if len(strings.Split(result, "\n")) < 3 {
		t.Error("Error() should render a bordered box (multi-line output)")
	}
}

func TestWarning(t *testing.T) {
	result := Warning("This is a warning")
	if result == "" {
		t.Error("Warning() returned empty string")
	}
	if !strings.Contains(result, "This is a warning") {
		t.Error("Warning() did not include the message")
	}
	if !strings.Contains(result, "⚠") {
		t.Error("Warning() did not include warning icon")
	}
}

func TestInfo(t *testing.T) {
	result := Info("Informational message")
	if result == "" {
		t.Error("Info() returned empty string")
	}
	if !strings.Contains(result, "Informational message") {
		t.Error("Info() did not include the message")
	}
	if !strings.Contains(result, "ℹ") {
		t.Error("Info() did not include info icon")
	}
}

func TestErrorWithDetails(t *testing.T) {
	details := []string{"Detail 1", "Detail 2", "Detail 3"}
	result := ErrorWithDetails("Main error message", details)

	if result == "" {
		t.Error("ErrorWithDetails() returned empty string")
	}
	if !strings.Contains(result, "Main error message") {
		t.Error("ErrorWithDetails() did not include main message")
	}
	for _, detail := range details {
		if !strings.Contains(result, detail) {
			t.Errorf("ErrorWithDetails() did not include detail: %s", detail)
		}
	}
	if !strings.Contains(result, "✗") {
		t.Error("ErrorWithDetails() did not include error icon")
	}
}

func TestErrorWithDetailsEmpty(t *testing.T) {
	result := ErrorWithDetails("Just the error", nil)

	if result == "" {
		t.Error("ErrorWithDetails() with nil details returned empty string")
	}
	if !strings.Contains(result, "Just the error") {
		t.Error("ErrorWithDetails() did not include message")
	}
}
