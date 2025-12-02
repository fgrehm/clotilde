package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewTable(t *testing.T) {
	headers := []string{"Name", "Status"}
	rows := [][]string{
		{"session1", "active"},
		{"session2", "inactive"},
	}
	model := NewTable(headers, rows)

	if len(model.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(model.Headers))
	}
	if len(model.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(model.Rows))
	}
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}
	if model.Selected != -1 {
		t.Errorf("Expected Selected to be -1, got %d", model.Selected)
	}
}

func TestTableUpdate_Navigation(t *testing.T) {
	rows := [][]string{
		{"row1", "data1"},
		{"row2", "data2"},
		{"row3", "data3"},
	}
	model := NewTable([]string{"Col1", "Col2"}, rows)

	// Start at 0
	if model.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.Cursor)
	}

	// Move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := updatedModel.(TableModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after down, got %d", m.Cursor)
	}

	// Move down again
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(TableModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor at 2 after down, got %d", m.Cursor)
	}

	// Try to move down beyond end (should stay at 2)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(TableModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", m.Cursor)
	}

	// Move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updatedModel.(TableModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after up, got %d", m.Cursor)
	}
}

func TestTableUpdate_VimNavigation(t *testing.T) {
	rows := [][]string{
		{"row1", "data1"},
		{"row2", "data2"},
	}
	model := NewTable([]string{"Col1", "Col2"}, rows)

	// j to move down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := updatedModel.(TableModel)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor at 1 after 'j', got %d", m.Cursor)
	}

	// k to move up
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedModel.(TableModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at 0 after 'k', got %d", m.Cursor)
	}
}

func TestTableUpdate_HomeEnd(t *testing.T) {
	rows := [][]string{
		{"row1", "data1"},
		{"row2", "data2"},
		{"row3", "data3"},
	}
	model := NewTable([]string{"Col1", "Col2"}, rows)
	model.Cursor = 1 // Start in middle

	// G (shift+g) to go to end
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m := updatedModel.(TableModel)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor at end (2) after 'G', got %d", m.Cursor)
	}

	// g to go to home
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updatedModel.(TableModel)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor at home (0) after 'g', got %d", m.Cursor)
	}
}

func TestTableUpdate_EnterSelects(t *testing.T) {
	rows := [][]string{
		{"row1", "data1"},
		{"row2", "data2"},
	}
	model := NewTable([]string{"Col1", "Col2"}, rows)
	model.Cursor = 1

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updatedModel.(TableModel)

	if m.Selected != 1 {
		t.Errorf("Expected Selected to be 1, got %d", m.Selected)
	}
	if cmd == nil {
		t.Error("Expected quit command after selection")
	}
}

func TestTableUpdate_QuitCancels(t *testing.T) {
	rows := [][]string{
		{"row1", "data1"},
	}
	model := NewTable([]string{"Col1", "Col2"}, rows)

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m := updatedModel.(TableModel)

	if !m.Cancelled {
		t.Error("Expected Cancelled to be true after 'q'")
	}
	if m.Selected != -1 {
		t.Error("Expected no selection when cancelled")
	}
	if cmd == nil {
		t.Error("Expected quit command after cancel")
	}
}

func TestTableView(t *testing.T) {
	rows := [][]string{
		{"session1", "active"},
		{"session2", "inactive"},
	}
	model := NewTable([]string{"Name", "Status"}, rows)
	view := model.View()

	if view == "" {
		t.Error("View should not be empty")
	}
	if !strings.Contains(view, "Name") {
		t.Error("View should contain 'Name' header")
	}
	if !strings.Contains(view, "Status") {
		t.Error("View should contain 'Status' header")
	}
	if !strings.Contains(view, "session1") {
		t.Error("View should contain first row data")
	}
	if !strings.Contains(view, "session2") {
		t.Error("View should contain second row data")
	}
	if !strings.Contains(view, ">") {
		t.Error("View should contain cursor indicator")
	}
}

func TestTableView_EmptyRows(t *testing.T) {
	model := NewTable([]string{"Name", "Status"}, [][]string{})
	view := model.View()

	if !strings.Contains(view, "No data available") {
		t.Error("View should show 'No data available' for empty table")
	}
}

func TestTableColumnWidths(t *testing.T) {
	rows := [][]string{
		{"short", "a very long value"},
		{"another short", "tiny"},
	}
	model := NewTable([]string{"Column1", "Column2"}, rows)
	widths := model.calculateColumnWidths()

	if len(widths) != 2 {
		t.Errorf("Expected 2 column widths, got %d", len(widths))
	}

	// First column should be max("Column1", "short", "another short") = "another short" = 13
	if widths[0] != 13 {
		t.Errorf("Expected first column width 13, got %d", widths[0])
	}

	// Second column should be max("Column2", "a very long value", "tiny") = "a very long value" = 17
	if widths[1] != 17 {
		t.Errorf("Expected second column width 17, got %d", widths[1])
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"hello", 5, "hello"},
		{"hello", 3, "hello"}, // Already longer than width
	}

	for _, tt := range tests {
		result := padRight(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("padRight(%q, %d) = %q, expected %q", tt.input, tt.width, result, tt.expected)
		}
	}
}

func TestTableSorting_Ascending(t *testing.T) {
	rows := [][]string{
		{"zebra", "10"},
		{"alpha", "30"},
		{"beta", "20"},
	}
	model := NewTable([]string{"Name", "Value"}, rows).WithSorting()

	// Sort by first column (Name)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updatedModel.(TableModel)

	if m.SortColumn != 0 {
		t.Errorf("Expected SortColumn to be 0, got %d", m.SortColumn)
	}
	if !m.SortAscending {
		t.Error("Expected SortAscending to be true")
	}

	// Check rows are sorted alphabetically
	if m.Rows[0][0] != "alpha" {
		t.Errorf("Expected first row to be 'alpha', got '%s'", m.Rows[0][0])
	}
	if m.Rows[1][0] != "beta" {
		t.Errorf("Expected second row to be 'beta', got '%s'", m.Rows[1][0])
	}
	if m.Rows[2][0] != "zebra" {
		t.Errorf("Expected third row to be 'zebra', got '%s'", m.Rows[2][0])
	}
}

func TestTableSorting_Descending(t *testing.T) {
	rows := [][]string{
		{"alpha", "30"},
		{"beta", "20"},
		{"zebra", "10"},
	}
	model := NewTable([]string{"Name", "Value"}, rows).WithSorting()

	// Sort by first column (ascending)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updatedModel.(TableModel)

	// Sort by first column again (descending)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updatedModel.(TableModel)

	if m.SortAscending {
		t.Error("Expected SortAscending to be false after toggle")
	}

	// Check rows are sorted in reverse
	if m.Rows[0][0] != "zebra" {
		t.Errorf("Expected first row to be 'zebra', got '%s'", m.Rows[0][0])
	}
	if m.Rows[1][0] != "beta" {
		t.Errorf("Expected second row to be 'beta', got '%s'", m.Rows[1][0])
	}
	if m.Rows[2][0] != "alpha" {
		t.Errorf("Expected third row to be 'alpha', got '%s'", m.Rows[2][0])
	}
}

func TestTableSorting_DifferentColumn(t *testing.T) {
	rows := [][]string{
		{"alpha", "30"},
		{"beta", "20"},
		{"gamma", "10"},
	}
	model := NewTable([]string{"Name", "Value"}, rows).WithSorting()

	// Sort by second column (Value)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m := updatedModel.(TableModel)

	if m.SortColumn != 1 {
		t.Errorf("Expected SortColumn to be 1, got %d", m.SortColumn)
	}

	// Check rows are sorted by value
	if m.Rows[0][1] != "10" {
		t.Errorf("Expected first row value to be '10', got '%s'", m.Rows[0][1])
	}
	if m.Rows[1][1] != "20" {
		t.Errorf("Expected second row value to be '20', got '%s'", m.Rows[1][1])
	}
	if m.Rows[2][1] != "30" {
		t.Errorf("Expected third row value to be '30', got '%s'", m.Rows[2][1])
	}
}

func TestTableSorting_WithoutSortingEnabled(t *testing.T) {
	rows := [][]string{
		{"zebra", "10"},
		{"alpha", "30"},
	}
	model := NewTable([]string{"Name", "Value"}, rows) // No .WithSorting()

	// Try to sort (should be ignored)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updatedModel.(TableModel)

	// Rows should remain in original order
	if m.Rows[0][0] != "zebra" {
		t.Errorf("Expected first row to still be 'zebra', got '%s'", m.Rows[0][0])
	}
	if m.SortColumn != -1 {
		t.Errorf("Expected SortColumn to remain -1, got %d", m.SortColumn)
	}
}

func TestTableView_WithSortIndicator(t *testing.T) {
	rows := [][]string{
		{"alpha", "1"},
		{"beta", "2"},
	}
	model := NewTable([]string{"Name", "Value"}, rows).WithSorting()

	// Sort by first column
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updatedModel.(TableModel)
	view := m.View()

	// Should contain sort indicator
	if !strings.Contains(view, "↑") && !strings.Contains(view, "↓") {
		t.Error("View should contain sort indicator (↑ or ↓)")
	}

	// Should contain column number hints
	if !strings.Contains(view, "[1]") || !strings.Contains(view, "[2]") {
		t.Error("View should contain column number hints when sorting is enabled")
	}
}

func TestTableFiltering_EnterFilterMode(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
		{"beta", "test2"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)

	// Press / to enter filter mode
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m := updatedModel.(TableModel)

	if !m.Filtering {
		t.Error("Expected Filtering to be true after pressing '/'")
	}
}

func TestTableFiltering_TypeAndFilter(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
		{"beta", "test2"},
		{"gamma", "test3"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)

	// Enter filter mode
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m := updatedModel.(TableModel)

	// Type 'bet'
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updatedModel.(TableModel)

	if m.FilterText != "bet" {
		t.Errorf("Expected FilterText to be 'bet', got '%s'", m.FilterText)
	}

	// Check filtered rows
	filtered := m.filteredRows()
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered row, got %d", len(filtered))
	}
	if filtered[0][0] != "beta" {
		t.Errorf("Expected filtered row to be 'beta', got '%s'", filtered[0][0])
	}
}

func TestTableFiltering_Backspace(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)

	// Enter filter mode and type
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m := updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updatedModel.(TableModel)

	if m.FilterText != "ab" {
		t.Errorf("Expected FilterText to be 'ab', got '%s'", m.FilterText)
	}

	// Press backspace
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updatedModel.(TableModel)

	if m.FilterText != "a" {
		t.Errorf("Expected FilterText to be 'a' after backspace, got '%s'", m.FilterText)
	}
}

func TestTableFiltering_ExitWithEsc(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)

	// Enter filter mode and type
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m := updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updatedModel.(TableModel)

	// Press esc (should clear filter and exit filter mode)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(TableModel)

	if m.Filtering {
		t.Error("Expected Filtering to be false after Esc")
	}
	if m.FilterText != "" {
		t.Errorf("Expected FilterText to be empty after Esc, got '%s'", m.FilterText)
	}
}

func TestTableFiltering_ExitWithEnter(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)

	// Enter filter mode and type
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m := updatedModel.(TableModel)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updatedModel.(TableModel)

	// Press enter (should keep filter and exit filter mode)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedModel.(TableModel)

	if m.Filtering {
		t.Error("Expected Filtering to be false after Enter")
	}
	if m.FilterText != "a" {
		t.Errorf("Expected FilterText to be 'a' after Enter, got '%s'", m.FilterText)
	}
}

func TestTableFiltering_NoMatches(t *testing.T) {
	rows := [][]string{
		{"alpha", "test1"},
		{"beta", "test2"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)
	model.FilterText = "xyz"

	filtered := model.filteredRows()
	if len(filtered) != 0 {
		t.Errorf("Expected 0 filtered rows, got %d", len(filtered))
	}
}

func TestTableFiltering_CaseInsensitive(t *testing.T) {
	rows := [][]string{
		{"Alpha", "TEST1"},
		{"beta", "test2"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)
	model.FilterText = "alpha"

	filtered := model.filteredRows()
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered row (case insensitive), got %d", len(filtered))
	}
	if filtered[0][0] != "Alpha" {
		t.Errorf("Expected filtered row to be 'Alpha', got '%s'", filtered[0][0])
	}
}

func TestTableView_WithFilter(t *testing.T) {
	rows := [][]string{
		{"alpha", "1"},
		{"beta", "2"},
	}
	model := NewTable([]string{"Name", "Value"}, rows)
	model.FilterText = "alpha"

	view := model.View()

	if !strings.Contains(view, "Filter:") {
		t.Error("View should contain 'Filter:' when filter is active")
	}
	if !strings.Contains(view, "alpha") {
		t.Error("View should contain filter text 'alpha'")
	}
}
