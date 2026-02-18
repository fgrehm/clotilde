package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fgrehm/clotilde/internal/session"
)

// PickerModel represents the session picker state
type PickerModel struct {
	Sessions    []*session.Session
	Cursor      int
	Selected    *session.Session
	Cancelled   bool
	Title       string
	FilterText  string
	Filtering   bool
	ShowPreview bool // Show preview pane with session metadata
}

// NewPicker creates a new session picker
func NewPicker(sessions []*session.Session, title string) PickerModel {
	return PickerModel{
		Sessions: sessions,
		Title:    title,
		Cursor:   0,
	}
}

// WithPreview enables the preview pane
func (m PickerModel) WithPreview() PickerModel {
	m.ShowPreview = true
	return m
}

// Init initializes the model (required by bubbletea)
func (m PickerModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input
func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			filtered := m.filteredSessions()
			if len(filtered) > 0 {
				m.Selected = filtered[m.Cursor]
			}
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil

		case "down", "j":
			filtered := m.filteredSessions()
			if m.Cursor < len(filtered)-1 {
				m.Cursor++
			}
			return m, nil

		case "home", "g":
			m.Cursor = 0
			return m, nil

		case "end", "G":
			filtered := m.filteredSessions()
			if len(filtered) > 0 {
				m.Cursor = len(filtered) - 1
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the session picker
func (m PickerModel) View() string {
	if m.ShowPreview {
		return m.viewWithPreview()
	}
	return m.viewSimple()
}

// viewSimple renders the picker without preview pane
func (m PickerModel) viewSimple() string {
	var b strings.Builder

	// Title
	titleStyle := BoldStyle
	b.WriteString(titleStyle.Render(m.Title))
	b.WriteString("\n\n")

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

	// Get filtered sessions
	filtered := m.filteredSessions()

	// No sessions
	if len(filtered) == 0 {
		emptyStyle := DimStyle.Italic(true)
		if m.FilterText != "" {
			b.WriteString(emptyStyle.Render(fmt.Sprintf("No sessions matching '%s'", m.FilterText)))
		} else {
			b.WriteString(emptyStyle.Render("No sessions available"))
		}
		b.WriteString("\n\n")
		b.WriteString(DimStyle.Render("Press / to filter, q or Esc to cancel"))
		return b.String()
	}

	// Session list
	for i, sess := range filtered {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}

		// Build session line
		sessionLine := m.formatSessionLine(sess)

		// Highlight matching text
		if m.FilterText != "" {
			sessionLine = m.highlightMatch(sessionLine, m.FilterText)
		}

		// Highlight selected
		if m.Cursor == i {
			sessionLine = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true).
				Render(sessionLine)
		}

		fmt.Fprintf(&b, "%s %s\n", cursor, sessionLine)
	}

	// Help text
	b.WriteString("\n")
	helpStyle := DimStyle.Italic(true)
	if m.FilterText != "" {
		b.WriteString(helpStyle.Render("(Esc to clear filter, / to edit, ↑/↓ to navigate, enter to select)"))
	} else {
		b.WriteString(helpStyle.Render("(/ to filter, ↑/↓ or j/k to navigate, enter to select, q to quit)"))
	}

	return b.String()
}

// viewWithPreview renders the picker with a preview pane (split view)
func (m PickerModel) viewWithPreview() string {
	filtered := m.filteredSessions()

	// Build list pane
	listPane := m.renderListPane(filtered)

	// Build preview pane
	var previewPane string
	if len(filtered) > 0 {
		previewPane = m.renderPreviewPane(filtered[m.Cursor])
	} else {
		previewPane = DimStyle.Italic(true).Render("No session selected")
	}

	// Join panes side by side
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listPane,
		"  ", // Spacer
		previewPane,
	)
}

// renderListPane renders the left pane with session list
func (m PickerModel) renderListPane(filtered []*session.Session) string {
	var b strings.Builder

	// Title
	titleStyle := BoldStyle
	b.WriteString(titleStyle.Render(m.Title))
	b.WriteString("\n\n")

	// Filter input (if active or has text)
	if m.Filtering || m.FilterText != "" {
		filterPrefix := "Filter: "
		if m.Filtering {
			filterPrefix = "Filter: "
		}
		filterStyle := InfoStyle
		b.WriteString(filterStyle.Render(filterPrefix))
		b.WriteString(m.FilterText)
		if m.Filtering {
			b.WriteString("█")
		}
		b.WriteString("\n\n")
	}

	// No sessions
	if len(filtered) == 0 {
		emptyStyle := DimStyle.Italic(true)
		if m.FilterText != "" {
			b.WriteString(emptyStyle.Render(fmt.Sprintf("No matches for '%s'", m.FilterText)))
		} else {
			b.WriteString(emptyStyle.Render("No sessions"))
		}
		b.WriteString("\n\n")
		b.WriteString(DimStyle.Render("/ filter, q quit"))
		return b.String()
	}

	// Session list (limit to visible area)
	const maxVisible = 10
	start := m.Cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		sess := filtered[i]
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}

		// Build session line with "last used" info
		sessionLine := m.formatSessionLineWithTime(sess)

		// Highlight selected
		if m.Cursor == i {
			sessionLine = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true).
				Render(sessionLine)
		}

		fmt.Fprintf(&b, "%s %s\n", cursor, sessionLine)
	}

	// Help text
	b.WriteString("\n")
	helpStyle := DimStyle.Italic(true)
	b.WriteString(helpStyle.Render("↑/↓ navigate · / filter · enter select · q quit"))

	return b.String()
}

// renderPreviewPane renders the right pane with session metadata
func (m PickerModel) renderPreviewPane(sess *session.Session) string {
	var lines []string

	// Session name header
	nameStyle := BoldStyle
	if sess.Metadata.IsForkedSession {
		nameStyle = lipgloss.NewStyle().Foreground(ForkColor).Bold(true)
	} else if sess.Metadata.IsIncognito {
		nameStyle = lipgloss.NewStyle().Foreground(IncognitoColor).Bold(true)
	}
	lines = append(lines, nameStyle.Render(sess.Name))
	lines = append(lines, "")

	// Session type
	typeLabel := "Session type:"
	if sess.Metadata.IsForkedSession {
		typeValue := lipgloss.NewStyle().Foreground(ForkColor).Render("Fork")
		lines = append(lines, DimStyle.Render(typeLabel))
		lines = append(lines, "  "+typeValue)
		lines = append(lines, DimStyle.Render("  Parent: "+sess.Metadata.ParentSession))
	} else if sess.Metadata.IsIncognito {
		typeValue := lipgloss.NewStyle().Foreground(IncognitoColor).Render("Incognito")
		lines = append(lines, DimStyle.Render(typeLabel))
		lines = append(lines, "  "+typeValue)
	}
	lines = append(lines, "")

	// Timestamps
	lines = append(lines, DimStyle.Render("Created:"))
	lines = append(lines, "  "+sess.Metadata.Created.Format("2006-01-02 15:04"))
	lines = append(lines, "")

	lines = append(lines, DimStyle.Render("Last accessed:"))
	lines = append(lines, "  "+formatTimeAgo(sess.Metadata.LastAccessed))

	// System prompt mode
	if sess.Metadata.SystemPromptMode != "" {
		lines = append(lines, "")
		lines = append(lines, DimStyle.Render("System prompt:"))
		lines = append(lines, "  "+sess.Metadata.SystemPromptMode)
	}

	return InfoBoxStyle.Render(strings.Join(lines, "\n"))
}

// formatSessionLine formats a single session for display
func (m PickerModel) formatSessionLine(sess *session.Session) string {
	name := sess.Name

	// Add type indicator
	typeIndicator := ""
	if sess.Metadata.IsForkedSession {
		typeStyle := lipgloss.NewStyle().Foreground(ForkColor)
		typeIndicator = typeStyle.Render(" [fork]")
	} else if sess.Metadata.IsIncognito {
		typeStyle := lipgloss.NewStyle().Foreground(IncognitoColor)
		typeIndicator = typeStyle.Render(" [incognito]")
	}

	return name + typeIndicator
}

// filteredSessions returns sessions that match the current filter
func (m PickerModel) filteredSessions() []*session.Session {
	if m.FilterText == "" {
		return m.Sessions
	}

	var filtered []*session.Session
	lowerFilter := strings.ToLower(m.FilterText)

	for _, sess := range m.Sessions {
		if strings.Contains(strings.ToLower(sess.Name), lowerFilter) {
			filtered = append(filtered, sess)
		}
	}

	return filtered
}

// highlightMatch highlights the matching part of the text (simple version)
func (m PickerModel) highlightMatch(text, filter string) string {
	// For now, just return the text as-is
	// A full implementation would highlight the matching substring
	return text
}

// formatSessionLineWithTime formats a session line with "last used" time
func (m PickerModel) formatSessionLineWithTime(sess *session.Session) string {
	name := sess.Name

	// Add type indicator
	if sess.Metadata.IsForkedSession {
		typeStyle := lipgloss.NewStyle().Foreground(ForkColor)
		name += typeStyle.Render(" [fork]")
	} else if sess.Metadata.IsIncognito {
		typeStyle := lipgloss.NewStyle().Foreground(IncognitoColor)
		name += typeStyle.Render(" [inc]")
	}

	// Add time ago
	timeAgo := DimStyle.Render(" · " + formatTimeAgo(sess.Metadata.LastAccessed))

	return name + timeAgo
}

// formatTimeAgo formats a time as "X ago" (e.g., "2 hours ago", "just now")
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration.Seconds() < 60:
		return "just now"
	case duration.Minutes() < 2:
		return "1 minute ago"
	case duration.Minutes() < 60:
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	case duration.Hours() < 2:
		return "1 hour ago"
	case duration.Hours() < 24:
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	case duration.Hours() < 48:
		return "1 day ago"
	default:
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
}

// RunPicker runs the session picker and returns the selected session
func RunPicker(model PickerModel) (*session.Session, error) {
	p := tea.NewProgram(model, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run picker: %w", err)
	}

	finalModel := m.(PickerModel)
	if finalModel.Cancelled {
		return nil, nil
	}

	return finalModel.Selected, nil
}
