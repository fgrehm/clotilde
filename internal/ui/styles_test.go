package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColorPaletteInitialization(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.Color
	}{
		{"SuccessColor", SuccessColor},
		{"ErrorColor", ErrorColor},
		{"WarningColor", WarningColor},
		{"InfoColor", InfoColor},
		{"MutedColor", MutedColor},
		{"RegularColor", RegularColor},
		{"ForkColor", ForkColor},
		{"IncognitoColor", IncognitoColor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color == "" {
				t.Errorf("%s is not initialized", tt.name)
			}
		})
	}
}

func TestTextStylesInitialization(t *testing.T) {
	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"BoldStyle", BoldStyle},
		{"DimStyle", DimStyle},
		{"ItalicStyle", ItalicStyle},
		{"SuccessStyle", SuccessStyle},
		{"ErrorStyle", ErrorStyle},
		{"WarningStyle", WarningStyle},
		{"InfoStyle", InfoStyle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the style can render text without panicking
			result := tt.style.Render("test")
			if result == "" {
				t.Errorf("%s failed to render text", tt.name)
			}
		})
	}
}

func TestBoxStylesInitialization(t *testing.T) {
	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"BoxStyle", BoxStyle},
		{"RegularBoxStyle", RegularBoxStyle},
		{"ForkBoxStyle", ForkBoxStyle},
		{"IncognitoBoxStyle", IncognitoBoxStyle},
		{"ErrorBoxStyle", ErrorBoxStyle},
		{"WarningBoxStyle", WarningBoxStyle},
		{"InfoBoxStyle", InfoBoxStyle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the box style can render text without panicking
			result := tt.style.Render("test content")
			if result == "" {
				t.Errorf("%s failed to render text", tt.name)
			}
			// Verify it contains box drawing characters (border)
			// Box styles should produce multi-line output due to borders
			if len(result) < 20 {
				t.Errorf("%s produced unexpectedly short output", tt.name)
			}
		})
	}
}
