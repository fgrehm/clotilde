package outputstyle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OutputStyleType represents the type of output style
type OutputStyleType int

const (
	BuiltIn OutputStyleType = iota // "default", "Explanatory", "Learning"
	Custom                         // "clotilde/<name>"
	None                           // No output style
)

// OutputStyle represents an output style configuration
type OutputStyle struct {
	Type    OutputStyleType
	Value   string // "default" | "Explanatory" | "Learning" | "clotilde/<name>"
	Content string // Only for custom styles (file content)
}

// IsBuiltIn checks if a style value is a built-in style
func IsBuiltIn(style string) bool {
	return style == "default" || style == "Explanatory" || style == "Learning"
}

// ValidateBuiltIn validates a built-in style value
func ValidateBuiltIn(style string) error {
	if !IsBuiltIn(style) {
		return fmt.Errorf("invalid built-in style: %s (must be 'default', 'Explanatory', or 'Learning')", style)
	}
	return nil
}

// StyleExists checks if a style file exists in standard locations
func StyleExists(clotildeRoot, styleName string) bool {
	// Check project level: .claude/output-styles/<name>.md
	projectPath := filepath.Join(clotildeRoot, "..", "output-styles", styleName+".md")
	if _, err := os.Stat(projectPath); err == nil {
		return true
	}

	// Check user level: ~/.claude/output-styles/<name>.md
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(homeDir, ".claude", "output-styles", styleName+".md")
		if _, err := os.Stat(userPath); err == nil {
			return true
		}
	}

	return false
}

// GetCustomStylePath returns the path to a custom output style file
func GetCustomStylePath(clotildeRoot, sessionName string) string {
	return filepath.Join(clotildeRoot, "..", "output-styles", "clotilde", sessionName+".md")
}

// GetCustomStyleReference returns the reference string for settings.json
func GetCustomStyleReference(sessionName string) string {
	return fmt.Sprintf("clotilde/%s", sessionName)
}

// CreateCustomStyleFile creates a custom output style file from content
func CreateCustomStyleFile(clotildeRoot, sessionName, content string) error {
	stylePath := GetCustomStylePath(clotildeRoot, sessionName)

	// Create directory if needed
	dir := filepath.Dir(stylePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output-styles directory: %w", err)
	}

	// Build frontmatter + content
	fileContent := fmt.Sprintf(`---
name: %s
description: Output style for session %s
keep-coding-instructions: true
---

%s
`, GetCustomStyleReference(sessionName), sessionName, content)

	// Write file
	if err := os.WriteFile(stylePath, []byte(fileContent), 0o644); err != nil {
		return fmt.Errorf("failed to write output style file: %w", err)
	}

	return nil
}

// CreateCustomStyleFileFromFile creates a custom output style from a file, validating/injecting frontmatter
func CreateCustomStyleFileFromFile(clotildeRoot, sessionName, sourceFilePath string) error {
	// Read source file
	content, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	contentStr := string(content)

	// Check if file has frontmatter
	if !strings.HasPrefix(contentStr, "---") {
		// No frontmatter - treat entire content as style content
		return CreateCustomStyleFile(clotildeRoot, sessionName, contentStr)
	}

	// Has frontmatter - validate it
	parts := strings.SplitN(contentStr, "---", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid frontmatter format (missing closing ---)")
	}

	frontmatter := strings.TrimSpace(parts[1])
	styleContent := strings.TrimSpace(parts[2])

	// Parse frontmatter as YAML (simple validation)
	if !strings.Contains(frontmatter, "name:") || !strings.Contains(frontmatter, "description:") {
		return fmt.Errorf("frontmatter missing required fields (name, description)")
	}

	// File has valid frontmatter - use it as-is but override name to match session
	// (We want name to be clotilde/<session-name> regardless of what user provided)
	updatedFrontmatter := fmt.Sprintf(`---
name: %s
description: Output style for session %s
keep-coding-instructions: true
---

%s
`, GetCustomStyleReference(sessionName), sessionName, styleContent)

	// Write to output styles directory
	stylePath := GetCustomStylePath(clotildeRoot, sessionName)
	dir := filepath.Dir(stylePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output-styles directory: %w", err)
	}

	if err := os.WriteFile(stylePath, []byte(updatedFrontmatter), 0o644); err != nil {
		return fmt.Errorf("failed to write output style file: %w", err)
	}

	return nil
}

// DeleteCustomStyleFile deletes a custom output style file
func DeleteCustomStyleFile(clotildeRoot, sessionName string) error {
	stylePath := GetCustomStylePath(clotildeRoot, sessionName)

	// Check if file exists
	if _, err := os.Stat(stylePath); os.IsNotExist(err) {
		return nil // Already deleted, no error
	}

	// Delete file
	if err := os.Remove(stylePath); err != nil {
		return fmt.Errorf("failed to delete output style file: %w", err)
	}

	return nil
}

// CustomStyleExists checks if a custom style file exists
func CustomStyleExists(clotildeRoot, sessionName string) bool {
	stylePath := GetCustomStylePath(clotildeRoot, sessionName)
	_, err := os.Stat(stylePath)
	return err == nil
}
