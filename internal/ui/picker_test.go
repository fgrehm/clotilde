package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fgrehm/clotilde/internal/session"
)

func TestNewPicker(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
	}
	model := NewPicker(sessions, "Select Session")

	if model.Title != "Select Session" {
		t.Errorf("Expected title 'Select Session', got '%s'", model.Title)
	}
	if len(model.Sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(model.Sessions))
	}
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}
}

func TestPickerUpdate_Navigation(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
		session.NewSession("test3", "uuid-3"),
	}
	model := NewPicker(sessions, "Select")

	// Start at 0
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}

	// Move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := updatedModel.(PickerModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after down, got %d", m.Cursor)
	}

	// Move down again
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(PickerModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor at 2 after down, got %d", m.Cursor)
	}

	// Try to move down beyond end (should stay at 2)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(PickerModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", m.Cursor)
	}

	// Move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updatedModel.(PickerModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after up, got %d", m.Cursor)
	}
}

func TestPickerUpdate_VimNavigation(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
	}
	model := NewPicker(sessions, "Select")

	// j to move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := updatedModel.(PickerModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after 'j', got %d", m.Cursor)
	}

	// k to move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedModel.(PickerModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at 0 after 'k', got %d", m.Cursor)
	}
}

func TestPickerUpdate_HomeEnd(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
		session.NewSession("test3", "uuid-3"),
	}
	model := NewPicker(sessions, "Select")
	model.Cursor = 1 // Start in middle

	// G (shift+g) to go to end
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m := updatedModel.(PickerModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor at end (2) after 'G', got %d", m.Cursor)
	}

	// g to go to home
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updatedModel.(PickerModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at home (0) after 'g', got %d", m.Cursor)
	}
}

func TestPickerUpdate_EnterSelects(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
	}
	model := NewPicker(sessions, "Select")
	model.Cursor = 1

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(PickerModel)

	if m.Selected == nil {
		t.Error("Expected a session to be selected")
	}
	if m.Selected.Name != "test2" {
		t.Errorf("Expected selected session 'test2', got '%s'", m.Selected.Name)
	}
	if cmd == nil {
		t.Error("Expected quit command after selection")
	}
}

func TestPickerUpdate_QuitCancels(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
	}
	model := NewPicker(sessions, "Select")

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m := updatedModel.(PickerModel)

	if !m.Cancelled {
		t.Error("Expected Cancelled to be true after 'q'")
	}
	if m.Selected != nil {
		t.Error("Expected no selection when cancelled")
	}
	if cmd == nil {
		t.Error("Expected quit command after cancel")
	}
}

func TestPickerView(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("session1", "uuid-1"),
		session.NewSession("session2", "uuid-2"),
	}
	model := NewPicker(sessions, "Choose Session")
	view := model.View()

	if view == "" {
		t.Error("View should not be empty")
	}
	if !strings.Contains(view, "Choose Session") {
		t.Error("View should contain title")
	}
	if !strings.Contains(view, "session1") {
		t.Error("View should contain first session")
	}
	if !strings.Contains(view, "session2") {
		t.Error("View should contain second session")
	}
	if !strings.Contains(view, ">") {
		t.Error("View should contain cursor indicator")
	}
}

func TestPickerView_EmptySessions(t *testing.T) {
	model := NewPicker([]*session.Session{}, "Select Session")
	view := model.View()

	if !strings.Contains(view, "No sessions available") {
		t.Error("View should show 'No sessions available' for empty list")
	}
}

func TestPickerView_ForkIndicator(t *testing.T) {
	fork := session.NewSession("fork1", "uuid-1")
	fork.Metadata.IsForkedSession = true
	fork.Metadata.ParentSession = "parent"

	sessions := []*session.Session{fork}
	model := NewPicker(sessions, "Select")
	view := model.View()

	if !strings.Contains(view, "fork") {
		t.Error("View should indicate forked session")
	}
}

func TestPickerView_IncognitoIndicator(t *testing.T) {
	incognito := session.NewIncognitoSession("temp", "uuid-1")

	sessions := []*session.Session{incognito}
	model := NewPicker(sessions, "Select")
	view := model.View()

	if !strings.Contains(view, "incognito") {
		t.Error("View should indicate incognito session")
	}
}
