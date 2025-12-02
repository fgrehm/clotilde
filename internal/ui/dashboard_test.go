package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fgrehm/clotilde/internal/session"
)

func TestNewDashboard(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("test1", "uuid-1"),
		session.NewSession("test2", "uuid-2"),
	}
	model := NewDashboard(sessions)

	if len(model.Sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(model.Sessions))
	}
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}
	if len(model.menuItems) == 0 {
		t.Error("Expected menu items to be populated")
	}
}

func TestDashboardUpdate_Navigation(t *testing.T) {
	model := NewDashboard([]*session.Session{})

	// Start at 0
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}

	// Move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := updatedModel.(DashboardModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after down, got %d", m.Cursor)
	}

	// Move down again
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(DashboardModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor at 2 after down, got %d", m.Cursor)
	}

	// Move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updatedModel.(DashboardModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after up, got %d", m.Cursor)
	}
}

func TestDashboardUpdate_VimNavigation(t *testing.T) {
	model := NewDashboard([]*session.Session{})

	// j to move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := updatedModel.(DashboardModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after 'j', got %d", m.Cursor)
	}

	// k to move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedModel.(DashboardModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at 0 after 'k', got %d", m.Cursor)
	}
}

func TestDashboardUpdate_HomeEnd(t *testing.T) {
	model := NewDashboard([]*session.Session{})
	model.Cursor = 2 // Start in middle

	// G (shift+g) to go to end
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m := updatedModel.(DashboardModel)
	if m.Cursor != len(m.menuItems)-1 {
		t.Errorf("Expected cursor at end after 'G', got %d", m.Cursor)
	}

	// g to go to home
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updatedModel.(DashboardModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at home (0) after 'g', got %d", m.Cursor)
	}
}

func TestDashboardUpdate_EnterSelects(t *testing.T) {
	model := NewDashboard([]*session.Session{})
	model.Cursor = 1 // Select second menu item

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(DashboardModel)

	if m.Selected == "" {
		t.Error("Expected a menu item to be selected")
	}
	if m.Selected != m.menuItems[1].ID {
		t.Errorf("Expected selected action '%s', got '%s'", m.menuItems[1].ID, m.Selected)
	}
	if cmd == nil {
		t.Error("Expected quit command after selection")
	}
}

func TestDashboardUpdate_QuitCancels(t *testing.T) {
	model := NewDashboard([]*session.Session{})

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m := updatedModel.(DashboardModel)

	if !m.Cancelled {
		t.Error("Expected Cancelled to be true after 'q'")
	}
	if m.Selected != "" {
		t.Error("Expected no selection when cancelled")
	}
	if cmd == nil {
		t.Error("Expected quit command after cancel")
	}
}

func TestDashboardView(t *testing.T) {
	sessions := []*session.Session{
		session.NewSession("session1", "uuid-1"),
		session.NewSession("session2", "uuid-2"),
	}
	model := NewDashboard(sessions)
	view := model.View()

	if view == "" {
		t.Error("View should not be empty")
	}
	if !strings.Contains(view, "Clotilde Dashboard") {
		t.Error("View should contain dashboard title")
	}
	if !strings.Contains(view, "Quick Actions") {
		t.Error("View should contain quick actions header")
	}
	if !strings.Contains(view, "Recent Sessions") {
		t.Error("View should contain recent sessions header")
	}
	if !strings.Contains(view, "session1") {
		t.Error("View should contain first session")
	}
}

func TestDashboardView_EmptySessions(t *testing.T) {
	model := NewDashboard([]*session.Session{})
	view := model.View()

	if !strings.Contains(view, "No sessions yet") {
		t.Error("View should show empty state message when no sessions")
	}
}

func TestDashboardView_Stats(t *testing.T) {
	fork := session.NewSession("fork1", "uuid-1")
	fork.Metadata.IsForkedSession = true

	incognito := session.NewIncognitoSession("incog1", "uuid-2")

	sessions := []*session.Session{
		session.NewSession("regular", "uuid-3"),
		fork,
		incognito,
	}
	model := NewDashboard(sessions)
	view := model.View()

	if !strings.Contains(view, "3 total") {
		t.Error("View should show total session count")
	}
	if !strings.Contains(view, "1 forks") {
		t.Error("View should show fork count")
	}
	if !strings.Contains(view, "1 incognito") {
		t.Error("View should show incognito count")
	}
}

func TestDashboardView_RecentSessionsLimit(t *testing.T) {
	// Create more sessions than the recent limit (5)
	sessions := []*session.Session{
		session.NewSession("sess1", "uuid-1"),
		session.NewSession("sess2", "uuid-2"),
		session.NewSession("sess3", "uuid-3"),
		session.NewSession("sess4", "uuid-4"),
		session.NewSession("sess5", "uuid-5"),
		session.NewSession("sess6", "uuid-6"),
		session.NewSession("sess7", "uuid-7"),
	}
	model := NewDashboard(sessions)
	view := model.View()

	// Should show first 5 sessions
	if !strings.Contains(view, "sess1") {
		t.Error("View should contain first session")
	}
	if !strings.Contains(view, "sess5") {
		t.Error("View should contain fifth session")
	}

	// Should show "and X more" message
	if !strings.Contains(view, "and 2 more") {
		t.Error("View should show 'and X more' message for remaining sessions")
	}
}

func TestDashboardMenuItems(t *testing.T) {
	model := NewDashboard([]*session.Session{})

	// Verify menu items are populated
	if len(model.menuItems) == 0 {
		t.Error("Menu items should be populated")
	}

	// Check for expected menu items
	expectedIDs := []string{"start", "resume", "fork", "list", "delete", "quit"}
	actualIDs := make([]string, len(model.menuItems))
	for i, item := range model.menuItems {
		actualIDs[i] = item.ID
	}

	for _, expectedID := range expectedIDs {
		found := false
		for _, actualID := range actualIDs {
			if actualID == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected menu item with ID '%s' not found", expectedID)
		}
	}
}
