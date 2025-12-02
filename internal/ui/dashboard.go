package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fgrehm/clotilde/internal/session"
)

// DashboardModel represents the main dashboard state
type DashboardModel struct {
	Sessions    []*session.Session
	Cursor      int
	Selected    string // Selected action ID
	Cancelled   bool
	Width       int
	Height      int
	recentLimit int // How many recent sessions to show
	menuItems   []MenuItem
}

// MenuItem represents a menu action
type MenuItem struct {
	ID          string
	Label       string
	Description string
}

// NewDashboard creates a new dashboard model
func NewDashboard(sessions []*session.Session) DashboardModel {
	return DashboardModel{
		Sessions:    sessions,
		Cursor:      0,
		recentLimit: 5,
		menuItems: []MenuItem{
			{ID: "start", Label: "Start new session", Description: "Create a new conversation"},
			{ID: "resume", Label: "Resume session", Description: "Continue an existing session"},
			{ID: "fork", Label: "Fork session", Description: "Branch from an existing session"},
			{ID: "list", Label: "List all sessions", Description: "View all sessions in a table"},
			{ID: "delete", Label: "Delete session", Description: "Remove a session"},
			{ID: "quit", Label: "Quit", Description: "Exit dashboard"},
		},
	}
}

// Init initializes the model (required by bubbletea)
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.Cancelled = true
			return m, tea.Quit

		case "enter", " ":
			if m.Cursor < len(m.menuItems) {
				m.Selected = m.menuItems[m.Cursor].ID
			}
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil

		case "down", "j":
			if m.Cursor < len(m.menuItems)-1 {
				m.Cursor++
			}
			return m, nil

		case "home", "g":
			m.Cursor = 0
			return m, nil

		case "end", "G":
			m.Cursor = len(m.menuItems) - 1
			return m, nil
		}
	}

	return m, nil
}

// View renders the dashboard
func (m DashboardModel) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(SuccessColor).
		Padding(1, 0)
	b.WriteString(titleStyle.Render("Clotilde Dashboard"))
	b.WriteString("\n\n")

	// Stats summary placeholder
	b.WriteString(m.renderStats())
	b.WriteString("\n\n")

	// Quick actions menu
	b.WriteString(m.renderMenu())
	b.WriteString("\n\n")

	// Recent sessions placeholder
	b.WriteString(m.renderRecentSessions())
	b.WriteString("\n\n")

	// Help text
	helpStyle := DimStyle.Italic(true)
	b.WriteString(helpStyle.Render("(↑/↓ or j/k to navigate, enter to select, q to quit)"))

	return b.String()
}

// renderStats renders the stats summary section
func (m DashboardModel) renderStats() string {
	total := len(m.Sessions)
	forks := 0
	incognito := 0

	for _, sess := range m.Sessions {
		if sess.Metadata.IsForkedSession {
			forks++
		}
		if sess.Metadata.IsIncognito {
			incognito++
		}
	}

	statsStyle := lipgloss.NewStyle().
		Foreground(InfoColor).
		Bold(true)

	var stats []string
	stats = append(stats, statsStyle.Render(fmt.Sprintf("%d total", total)))
	if forks > 0 {
		forkStyle := lipgloss.NewStyle().Foreground(ForkColor)
		stats = append(stats, forkStyle.Render(fmt.Sprintf("%d forks", forks)))
	}
	if incognito > 0 {
		incognitoStyle := lipgloss.NewStyle().Foreground(IncognitoColor)
		stats = append(stats, incognitoStyle.Render(fmt.Sprintf("%d incognito", incognito)))
	}

	return "Sessions: " + strings.Join(stats, " · ")
}

// renderMenu renders the quick actions menu
func (m DashboardModel) renderMenu() string {
	var b strings.Builder

	headerStyle := BoldStyle
	b.WriteString(headerStyle.Render("Quick Actions"))
	b.WriteString("\n\n")

	for i, item := range m.menuItems {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}

		itemStyle := lipgloss.NewStyle()
		if m.Cursor == i {
			itemStyle = itemStyle.Foreground(SuccessColor).Bold(true)
		}

		line := fmt.Sprintf("%s  %s", item.Label, DimStyle.Render("- "+item.Description))
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, itemStyle.Render(line)))
	}

	return b.String()
}

// renderRecentSessions renders the recent sessions list
func (m DashboardModel) renderRecentSessions() string {
	if len(m.Sessions) == 0 {
		return DimStyle.Italic(true).Render("No sessions yet. Start one to get going!")
	}

	var b strings.Builder

	headerStyle := BoldStyle
	b.WriteString(headerStyle.Render("Recent Sessions"))
	b.WriteString("\n\n")

	// Show up to recentLimit sessions
	limit := m.recentLimit
	if len(m.Sessions) < limit {
		limit = len(m.Sessions)
	}

	for i := 0; i < limit; i++ {
		sess := m.Sessions[i]

		// Format session line
		name := sess.Name
		typeIndicator := ""
		if sess.Metadata.IsForkedSession {
			typeStyle := lipgloss.NewStyle().Foreground(ForkColor)
			typeIndicator = typeStyle.Render(" [fork]")
		} else if sess.Metadata.IsIncognito {
			typeStyle := lipgloss.NewStyle().Foreground(IncognitoColor)
			typeIndicator = typeStyle.Render(" [incognito]")
		}

		b.WriteString(fmt.Sprintf("  • %s%s\n", name, typeIndicator))
	}

	if len(m.Sessions) > limit {
		moreStyle := DimStyle.Italic(true)
		b.WriteString(moreStyle.Render(fmt.Sprintf("\n  ...and %d more", len(m.Sessions)-limit)))
	}

	return b.String()
}

// RunDashboard runs the dashboard and returns the selected action
func RunDashboard(model DashboardModel) (string, error) {
	p := tea.NewProgram(model, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run dashboard: %w", err)
	}

	finalModel := m.(DashboardModel)
	if finalModel.Cancelled {
		return "", nil
	}

	return finalModel.Selected, nil
}
