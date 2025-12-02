package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewConfirm(t *testing.T) {
	model := NewConfirm("Test Title", "Test message")

	if model.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", model.Title)
	}
	if model.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", model.Message)
	}
	if model.Focused != 0 {
		t.Errorf("Expected default focus on Cancel (0), got %d", model.Focused)
	}
	if model.Confirmed {
		t.Error("Expected Confirmed to be false initially")
	}
	if model.Cancelled {
		t.Error("Expected Cancelled to be false initially")
	}
}

func TestConfirmWithDetails(t *testing.T) {
	details := []string{"Detail 1", "Detail 2"}
	model := NewConfirm("Title", "Message").WithDetails(details)

	if len(model.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(model.Details))
	}
	if model.Details[0] != "Detail 1" {
		t.Errorf("Expected first detail 'Detail 1', got '%s'", model.Details[0])
	}
}

func TestConfirmWithDestructive(t *testing.T) {
	model := NewConfirm("Title", "Message").WithDestructive()

	if !model.Destructive {
		t.Error("Expected Destructive to be true")
	}
}

func TestConfirmUpdate_YKey(t *testing.T) {
	model := NewConfirm("Title", "Message")
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	m := updatedModel.(ConfirmModel)
	if !m.Confirmed {
		t.Error("Expected Confirmed to be true after 'y' key")
	}
	if cmd == nil {
		t.Error("Expected quit command after confirmation")
	}
}

func TestConfirmUpdate_NKey(t *testing.T) {
	model := NewConfirm("Title", "Message")
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	m := updatedModel.(ConfirmModel)
	if !m.Cancelled {
		t.Error("Expected Cancelled to be true after 'n' key")
	}
	if cmd == nil {
		t.Error("Expected quit command after cancellation")
	}
}

func TestConfirmUpdate_Navigation(t *testing.T) {
	model := NewConfirm("Title", "Message")

	// Start with focus on Cancel (0)
	if model.Focused != 0 {
		t.Errorf("Expected initial focus on Cancel (0), got %d", model.Focused)
	}

	// Press right/tab to move to Confirm
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	m := updatedModel.(ConfirmModel)
	if m.Focused != 1 {
		t.Errorf("Expected focus on Confirm (1) after tab, got %d", m.Focused)
	}

	// Press left to move back to Cancel
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updatedModel.(ConfirmModel)
	if m.Focused != 0 {
		t.Errorf("Expected focus on Cancel (0) after left, got %d", m.Focused)
	}
}

func TestConfirmUpdate_EnterOnCancel(t *testing.T) {
	model := NewConfirm("Title", "Message")
	model.Focused = 0 // Focus on Cancel

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(ConfirmModel)

	if !m.Cancelled {
		t.Error("Expected Cancelled to be true when Enter pressed on Cancel")
	}
	if m.Confirmed {
		t.Error("Expected Confirmed to be false when Enter pressed on Cancel")
	}
	if cmd == nil {
		t.Error("Expected quit command")
	}
}

func TestConfirmUpdate_EnterOnConfirm(t *testing.T) {
	model := NewConfirm("Title", "Message")
	model.Focused = 1 // Focus on Confirm

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(ConfirmModel)

	if !m.Confirmed {
		t.Error("Expected Confirmed to be true when Enter pressed on Confirm")
	}
	if m.Cancelled {
		t.Error("Expected Cancelled to be false when Enter pressed on Confirm")
	}
	if cmd == nil {
		t.Error("Expected quit command")
	}
}

func TestConfirmUpdate_Escape(t *testing.T) {
	model := NewConfirm("Title", "Message")
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	m := updatedModel.(ConfirmModel)
	if !m.Cancelled {
		t.Error("Expected Cancelled to be true after Escape")
	}
	if cmd == nil {
		t.Error("Expected quit command after Escape")
	}
}

func TestConfirmView(t *testing.T) {
	model := NewConfirm("Delete Session", "Are you sure?")
	view := model.View()

	if view == "" {
		t.Error("View should not be empty")
	}
	if !strings.Contains(view, "Delete Session") {
		t.Error("View should contain title")
	}
	if !strings.Contains(view, "Are you sure?") {
		t.Error("View should contain message")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("View should contain Cancel button")
	}
	if !strings.Contains(view, "Confirm") {
		t.Error("View should contain Confirm button")
	}
}

func TestConfirmView_WithDetails(t *testing.T) {
	details := []string{"Session folder", "Transcript files"}
	model := NewConfirm("Delete", "Confirm?").WithDetails(details)
	view := model.View()

	if !strings.Contains(view, "Session folder") {
		t.Error("View should contain first detail")
	}
	if !strings.Contains(view, "Transcript files") {
		t.Error("View should contain second detail")
	}
}
