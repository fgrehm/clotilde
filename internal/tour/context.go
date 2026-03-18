package tour

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
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

// GatherContext collects repo information for tour generation.
func GatherContext(repoDir string, opts ContextOptions) (string, error) {
	opts.defaults()

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
		if d.IsDir() {
			if excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
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
