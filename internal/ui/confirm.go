package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmModel represents the confirmation dialog state
type ConfirmModel struct {
	Title       string
	Message     string
	Details     []string
	Destructive bool
	Confirmed   bool
	Cancelled   bool
	Focused     int // 0 = Cancel (default), 1 = Confirm
}

// NewConfirm creates a new confirmation dialog
func NewConfirm(title, message string) ConfirmModel {
	return ConfirmModel{
		Title:   title,
		Message: message,
		Focused: 0, // Default focus on Cancel (safe default)
	}
}

// WithDetails adds detail lines to the confirmation dialog
func (m ConfirmModel) WithDetails(details []string) ConfirmModel {
	m.Details = details
	return m
}

// WithDestructive marks the action as destructive (red styling)
func (m ConfirmModel) WithDestructive() ConfirmModel {
	m.Destructive = true
	return m
}

// Init initializes the model (required by bubbletea)
func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input
func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.Cancelled = true
			return m, tea.Quit

		case "enter", " ":
			if m.Focused == 1 {
				m.Confirmed = true
			} else {
				m.Cancelled = true
			}
			return m, tea.Quit

		case "y", "Y":
			m.Confirmed = true
			return m, tea.Quit

		case "n", "N":
			m.Cancelled = true
			return m, tea.Quit

		case "left", "h", "shift+tab":
			m.Focused = 0
			return m, nil

		case "right", "l", "tab":
			m.Focused = 1
			return m, nil
		}
	}

	return m, nil
}

// View renders the confirmation dialog
func (m ConfirmModel) View() string {
	var b strings.Builder

	// Title
	titleStyle := BoldStyle
	if m.Destructive {
		titleStyle = ErrorStyle
	}
	b.WriteString(titleStyle.Render(m.Title))
	b.WriteString("\n\n")

	// Message
	b.WriteString(m.Message)
	b.WriteString("\n")

	// Details (if any)
	if len(m.Details) > 0 {
		b.WriteString("\n")
		detailStyle := DimStyle
		for _, detail := range m.Details {
			b.WriteString(detailStyle.Render("  • " + detail))
			b.WriteString("\n")
		}
	}

	// Buttons
	b.WriteString("\n")
	b.WriteString(m.renderButtons())

	// Help text
	b.WriteString("\n\n")
	helpStyle := DimStyle.Italic(true)
	b.WriteString(helpStyle.Render("(y/n, arrows to navigate, enter to confirm)"))

	return b.String()
}

// renderButtons renders the Cancel/Confirm buttons
func (m ConfirmModel) renderButtons() string {
	// Unfocused buttons: dim text, no border
	unfocusedStyle := DimStyle

	// Focused button: bold, background color, border
	cancelFocusedStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(MutedColor).
		Background(MutedColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	confirmFocusedStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		Bold(true)

	if m.Destructive {
		confirmFocusedStyle = confirmFocusedStyle.
			BorderForeground(ErrorColor).
			Background(ErrorColor).
			Foreground(lipgloss.Color("#FFFFFF"))
	} else {
		confirmFocusedStyle = confirmFocusedStyle.
			BorderForeground(SuccessColor).
			Background(SuccessColor).
			Foreground(lipgloss.Color("#FFFFFF"))
	}

	cancelBtn := "Cancel"
	confirmBtn := "Confirm"

	var cancelRendered, confirmRendered string
	if m.Focused == 0 {
		// Cancel is focused
		cancelRendered = cancelFocusedStyle.Render(cancelBtn)
		confirmRendered = unfocusedStyle.Render(confirmBtn)
	} else {
		// Confirm is focused
		cancelRendered = unfocusedStyle.Render(cancelBtn)
		confirmRendered = confirmFocusedStyle.Render(confirmBtn)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, cancelRendered, "    ", confirmRendered)
}

// RunConfirm runs the confirmation dialog and returns true if confirmed
func RunConfirm(model ConfirmModel) (bool, error) {
	p := tea.NewProgram(model, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run confirmation dialog: %w", err)
	}

	finalModel := m.(ConfirmModel)
	return finalModel.Confirmed, nil
}
