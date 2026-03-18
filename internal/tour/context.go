package tour

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ContextOptions controls how repo context is gathered.
type ContextOptions struct {
	MaxFiles        int    // max files to include snippets from (default: 20)
	MaxLinesPerFile int    // max lines per file snippet (default: 80)
	Focus           string // optional: area to focus on
}

func (o *ContextOptions) defaults() {
	if o.MaxFiles <= 0 {
		o.MaxFiles = 20
	}
	if o.MaxLinesPerFile <= 0 {
		o.MaxLinesPerFile = 80
	}
}

var excludeDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
	"dist":         true,
	".tours":       true,
	"__pycache__":  true,
	".next":        true,
}

// gitignorePatterns holds parsed patterns from .gitignore files
type gitignorePatterns struct {
	patterns []string
}

// getGlobalGitignorePath returns the global gitignore path from git config
func getGlobalGitignorePath() string {
	cmd := exec.Command("git", "config", "--global", "core.excludesFile")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}

	// Fallback to default path if git config not available
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "git", "ignore")
}

// loadGitignore loads and parses .gitignore file
func loadGitignore(filePath string) *gitignorePatterns {
	gp := &gitignorePatterns{}

	f, err := os.Open(filePath)
	if err != nil {
		return gp
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Unescape special characters
		if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\\\") {
			line = line[1:]
		}

		gp.patterns = append(gp.patterns, line)
	}

	return gp
}

// isIgnored checks if a file matches any gitignore pattern
func (gp *gitignorePatterns) isIgnored(relPath string) bool {
	for _, pattern := range gp.patterns {
		if matchGitignore(pattern, relPath) {
			return true
		}
	}
	return false
}

// matchGitignore matches a path against a gitignore pattern
// Handles basic gitignore rules:
// - * matches anything except /
// - ** matches zero or more directories
// - ? matches any one character
// - ! negates the pattern (not supported yet)
func matchGitignore(pattern string, path string) bool {
	// Handle negation patterns (gitignore ! syntax)
	if strings.HasPrefix(pattern, "!") {
		return false
	}

	// Trailing slash means directory only
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Leading slash means match from root
	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
	}

	// Match against full path
	if strings.Contains(pattern, "/") {
		// Pattern with slash: match as relative path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	} else {
		// Pattern without slash: match against any path component
		parts := strings.Split(path, string(filepath.Separator))
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}

		// Also try matching full path for patterns like "*.ext"
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	// Handle ** patterns (match any number of directories)
	if strings.Contains(pattern, "**") {
		pattern = strings.ReplaceAll(pattern, "**", "*")
		// Try matching the modified pattern
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// GatherContext collects repo information for tour generation.
func GatherContext(repoDir string, opts ContextOptions) (string, error) {
	opts.defaults()

	// Load gitignore patterns
	localGitignore := loadGitignore(filepath.Join(repoDir, ".gitignore"))
	globalGitignore := loadGitignore(getGlobalGitignorePath())

	var b strings.Builder
	const maxTotal = 30000

	// Collect file tree
	var allFiles []string
	err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(repoDir, path)
		if err != nil {
			return nil
		}

		// Normalize path separators for matching
		matchPath := filepath.ToSlash(rel)

		if d.IsDir() {
			if excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Check gitignore patterns for directories
			if localGitignore.isIgnored(matchPath) || globalGitignore.isIgnored(matchPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore patterns for files
		if localGitignore.isIgnored(matchPath) || globalGitignore.isIgnored(matchPath) {
			return nil
		}

		allFiles = append(allFiles, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk repo: %w", err)
	}

	// File tree
	b.WriteString("## File tree\n\n```\n")
	for _, f := range allFiles {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	b.WriteString("```\n\n")

	// Include README.md and CLAUDE.md in full
	for _, name := range []string{"README.md", "CLAUDE.md"} {
		content, err := os.ReadFile(filepath.Join(repoDir, name))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", name, string(content))
	}

	// Identify key files
	keyFiles := identifyKeyFiles(allFiles, opts.Focus)

	// Limit to MaxFiles
	if len(keyFiles) > opts.MaxFiles {
		keyFiles = keyFiles[:opts.MaxFiles]
	}

	// Include snippets of key files
	for _, f := range keyFiles {
		if b.Len() > maxTotal {
			break
		}

		content, err := readFirstLines(filepath.Join(repoDir, f), opts.MaxLinesPerFile)
		if err != nil {
			continue
		}

		fmt.Fprintf(&b, "## %s\n\n```\n%s```\n\n", f, content)
	}

	result := b.String()
	if len(result) > maxTotal {
		result = result[:maxTotal]
	}

	return result, nil
}

// identifyKeyFiles picks the most important files for context.
// Entry points come first, then files matching focus keyword, then the rest.
func identifyKeyFiles(files []string, focus string) []string {
	var entryPoints, focusMatches, rest []string

	for _, f := range files {
		base := filepath.Base(f)
		baseLower := strings.ToLower(base)

		switch {
		case isEntryPoint(baseLower):
			entryPoints = append(entryPoints, f)
		case focus != "" && strings.Contains(strings.ToLower(f), strings.ToLower(focus)):
			focusMatches = append(focusMatches, f)
		default:
			rest = append(rest, f)
		}
	}

	var result []string
	result = append(result, entryPoints...)
	result = append(result, focusMatches...)
	result = append(result, rest...)
	return result
}

func isEntryPoint(baseLower string) bool {
	prefixes := []string{"main.", "lib.", "mod.", "index.", "app."}
	for _, p := range prefixes {
		if strings.HasPrefix(baseLower, p) {
			return true
		}
	}
	return false
}

func readFirstLines(path string, maxLines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() && line < maxLines {
		b.WriteString(scanner.Text())
		b.WriteByte('\n')
		line++
	}
	return b.String(), scanner.Err()
}
