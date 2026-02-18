package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableModel represents a table with headers, rows, and cursor navigation
type TableModel struct {
	Headers        []string
	Rows           [][]string
	Cursor         int
	Selected       int      // -1 if cancelled
	SelectedRow    []string // actual selected row data
	Cancelled      bool
	SortColumn     int    // -1 for no sort, 0+ for column index
	SortAscending  bool   // true for ascending, false for descending
	FilterText     string // current filter text
	Filtering      bool   // whether in filter mode
	sortingEnabled bool   // whether sorting is enabled
}

// NewTable creates a new table model
func NewTable(headers []string, rows [][]string) TableModel {
	return TableModel{
		Headers:    headers,
		Rows:       rows,
		Cursor:     0,
		Selected:   -1,
		SortColumn: -1, // No sorting by default
	}
}

// WithSorting enables sorting on this table
func (m TableModel) WithSorting() TableModel {
	m.sortingEnabled = true
	return m
}

// Init initializes the model (required by bubbletea)
func (m TableModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input
func (m TableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter mode separately
		if m.Filtering {
			switch msg.String() {
			case "esc":
				// Exit filter mode, clear filter
				m.Filtering = false
				m.FilterText = ""
				m.Cursor = 0
				return m, nil

			case "enter":
				// Exit filter mode, keep filter
				m.Filtering = false
				return m, nil

			case "backspace":
				if len(m.FilterText) > 0 {
					m.FilterText = m.FilterText[:len(m.FilterText)-1]
					m.Cursor = 0 // Reset cursor when filter changes
				}
				return m, nil

			default:
				// Add character to filter
				if len(msg.Runes) == 1 {
					m.FilterText += string(msg.Runes[0])
					m.Cursor = 0 // Reset cursor when filter changes
				}
				return m, nil
			}
		}

		// Normal mode (not filtering)
		switch msg.String() {
		case "ctrl+c":
			m.Cancelled = true
			return m, tea.Quit

		case "q":
			if !m.Filtering {
				m.Cancelled = true
				return m, tea.Quit
			}

		case "esc":
			if m.FilterText != "" {
				// Clear existing filter
				m.FilterText = ""
				m.Cursor = 0
				return m, nil
			}
			m.Cancelled = true
			return m, tea.Quit

		case "/":
			// Enter filter mode
			m.Filtering = true
			return m, nil

		case "enter", " ":
			filtered := m.filteredRows()
			if len(filtered) > 0 && m.Cursor < len(filtered) {
				m.Selected = m.Cursor
				m.SelectedRow = filtered[m.Cursor] // Store the actual row data
			}
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil

		case "down", "j":
			filtered := m.filteredRows()
			if m.Cursor < len(filtered)-1 {
				m.Cursor++
			}
			return m, nil

		case "home", "g":
			m.Cursor = 0
			return m, nil

		case "end", "G":
			filtered := m.filteredRows()
			if len(filtered) > 0 {
				m.Cursor = len(filtered) - 1
			}
			return m, nil

		default:
			// Handle number keys for sorting (1, 2, 3... for columns)
			if m.sortingEnabled && len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r >= '1' && r <= '9' {
					colIndex := int(r - '1')
					if colIndex < len(m.Headers) {
						// Toggle sort direction if same column, otherwise set ascending
						if m.SortColumn == colIndex {
							m.SortAscending = !m.SortAscending
						} else {
							m.SortColumn = colIndex
							m.SortAscending = true
						}
						m.sortRows()
						m.Cursor = 0 // Reset cursor after sort
					}
					return m, nil
				}
			}
		}
	}
	return m, nil
}

// View renders the table
func (m TableModel) View() string {
	var b strings.Builder

	// Filter input (if active or has text)
	if m.Filtering || m.FilterText != "" {
		filterPrefix := "Filter: "
		if m.Filtering {
			filterPrefix = "Filter (type to search): "
		}
		filterStyle := InfoStyle
		b.WriteString(filterStyle.Render(filterPrefix))
		b.WriteString(m.FilterText)
		if m.Filtering {
			b.WriteString("█") // Cursor
		}
		b.WriteString("\n\n")
	}

	// Get filtered rows
	filtered := m.filteredRows()

	// No rows (after filtering)
	if len(filtered) == 0 {
		emptyStyle := DimStyle.Italic(true)
		if m.FilterText != "" {
			b.WriteString(emptyStyle.Render(fmt.Sprintf("No rows matching '%s'", m.FilterText)))
		} else {
			b.WriteString(emptyStyle.Render("No data available"))
		}
		b.WriteString("\n\n")
		b.WriteString(DimStyle.Render("Press / to filter, q to quit"))
		return b.String()
	}

	// Calculate column widths
	widths := m.calculateColumnWidths()

	// Render header
	headerRow := m.renderHeaderRow(widths)
	b.WriteString(headerRow)
	b.WriteString("\n")

	// Render separator
	separator := m.renderSeparator(widths)
	b.WriteString(separator)
	b.WriteString("\n")

	// Render rows
	for i, row := range filtered {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}

		rowStr := m.renderRow(row, widths)
		if m.Cursor == i {
			rowStr = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true).
				Render(rowStr)
		}

		fmt.Fprintf(&b, "%s %s\n", cursor, rowStr)
	}

	// Help text
	b.WriteString("\n")
	helpStyle := DimStyle.Italic(true)
	switch {
	case m.FilterText != "":
		b.WriteString(helpStyle.Render("(Esc to clear filter, / to edit, ↑/↓ to navigate, enter to select)"))
	case m.sortingEnabled:
		b.WriteString(helpStyle.Render("(↑/↓ or j/k to navigate, / to filter, 1-9 to sort, enter to select, q to quit)"))
	default:
		b.WriteString(helpStyle.Render("(↑/↓ or j/k to navigate, / to filter, enter to select, q to quit)"))
	}

	return b.String()
}

// calculateColumnWidths determines the width of each column
func (m TableModel) calculateColumnWidths() []int {
	if len(m.Headers) == 0 {
		return []int{}
	}

	widths := make([]int, len(m.Headers))

	// Start with header widths (including indicators)
	for i, header := range m.Headers {
		headerText := header

		// Add sort indicator if this column is being sorted
		if m.SortColumn == i {
			headerText += " ↑" // Both ↑ and ↓ are same width
		}

		// Add column number hint if sorting is enabled
		if m.sortingEnabled {
			headerText = fmt.Sprintf("%s [%d]", headerText, i+1)
		}

		widths[i] = len(headerText)
	}

	// Check row widths
	for _, row := range m.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	return widths
}

// renderHeaderRow renders the header row
func (m TableModel) renderHeaderRow(widths []int) string {
	var cells []string
	for i, header := range m.Headers {
		width := widths[i]
		headerText := header

		// Add sort indicator if this column is being sorted
		if m.SortColumn == i {
			if m.SortAscending {
				headerText = header + " ↑"
			} else {
				headerText = header + " ↓"
			}
		}

		// Add column number hint if sorting is enabled
		if m.sortingEnabled {
			headerText = fmt.Sprintf("%s [%d]", headerText, i+1)
		}

		cell := BoldStyle.Render(padRight(headerText, width))
		cells = append(cells, cell)
	}
	return strings.Join(cells, "  ")
}

// renderSeparator renders the separator line between header and rows
func (m TableModel) renderSeparator(widths []int) string {
	var parts []string
	for _, width := range widths {
		parts = append(parts, strings.Repeat("─", width))
	}
	return DimStyle.Render(strings.Join(parts, "  "))
}

// renderRow renders a single data row
func (m TableModel) renderRow(row []string, widths []int) string {
	var cells []string
	for i, cell := range row {
		if i < len(widths) {
			width := widths[i]
			cells = append(cells, padRight(cell, width))
		}
	}
	return strings.Join(cells, "  ")
}

// filteredRows returns rows that match the current filter
func (m TableModel) filteredRows() [][]string {
	if m.FilterText == "" {
		return m.Rows
	}

	var filtered [][]string
	lowerFilter := strings.ToLower(m.FilterText)

	for _, row := range m.Rows {
		// Check if any cell in the row matches the filter
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), lowerFilter) {
				filtered = append(filtered, row)
				break // Only add row once even if multiple cells match
			}
		}
	}

	return filtered
}

// sortRows sorts the rows based on SortColumn and SortAscending
func (m *TableModel) sortRows() {
	if m.SortColumn < 0 || m.SortColumn >= len(m.Headers) {
		return
	}

	// Simple bubble sort - good enough for typical row counts
	for i := 0; i < len(m.Rows)-1; i++ {
		for j := 0; j < len(m.Rows)-i-1; j++ {
			// Get values to compare
			val1 := ""
			val2 := ""
			if m.SortColumn < len(m.Rows[j]) {
				val1 = m.Rows[j][m.SortColumn]
			}
			if m.SortColumn < len(m.Rows[j+1]) {
				val2 = m.Rows[j+1][m.SortColumn]
			}

			// Compare and swap if needed
			var shouldSwap bool
			if m.SortAscending {
				shouldSwap = val1 > val2
			} else {
				shouldSwap = val1 < val2
			}

			if shouldSwap {
				m.Rows[j], m.Rows[j+1] = m.Rows[j+1], m.Rows[j]
			}
		}
	}
}

// padRight pads a string with spaces to reach the desired width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// RunTable runs the table and returns the selected row data (or nil if cancelled)
func RunTable(model TableModel) ([]string, error) {
	p := tea.NewProgram(model, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run table: %w", err)
	}

	finalModel := m.(TableModel)
	if finalModel.Cancelled {
		return nil, nil
	}

	return finalModel.SelectedRow, nil
}
