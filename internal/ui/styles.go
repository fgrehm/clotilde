package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Status colors
	SuccessColor = lipgloss.Color("#00D787") // Bright green
	ErrorColor   = lipgloss.Color("#FF5F87") // Bright red
	WarningColor = lipgloss.Color("#FFD700") // Gold/yellow
	InfoColor    = lipgloss.Color("#5FAFD7") // Blue
	MutedColor   = lipgloss.Color("#6C6C6C") // Gray

	// Session type colors
	RegularColor   = lipgloss.Color("#FFFFFF") // White/default
	ForkColor      = lipgloss.Color("#00D7D7") // Cyan
	IncognitoColor = lipgloss.Color("#6C6C6C") // Dim gray
)

// Base text styles
var (
	BoldStyle   = lipgloss.NewStyle().Bold(true)
	DimStyle    = lipgloss.NewStyle().Foreground(MutedColor)
	ItalicStyle = lipgloss.NewStyle().Italic(true)

	// Status text styles
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	WarningStyle = lipgloss.NewStyle().Foreground(WarningColor).Bold(true)
	InfoStyle    = lipgloss.NewStyle().Foreground(InfoColor).Bold(true)
)

// Box and border styles
var (
	// Base box style with padding
	BoxStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder())

	// Session type box styles
	RegularBoxStyle = BoxStyle.
			BorderForeground(RegularColor)

	ForkBoxStyle = BoxStyle.
			BorderForeground(ForkColor)

	IncognitoBoxStyle = BoxStyle.
				BorderForeground(IncognitoColor)

	// Status box styles
	ErrorBoxStyle = BoxStyle.
			BorderForeground(ErrorColor).
			Padding(1, 2)

	WarningBoxStyle = BoxStyle.
			BorderForeground(WarningColor).
			Padding(1, 2)

	InfoBoxStyle = BoxStyle.
			BorderForeground(InfoColor).
			Padding(1, 2)
)
