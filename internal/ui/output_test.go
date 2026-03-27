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
