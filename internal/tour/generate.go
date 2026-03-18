package tour

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerationPrompt is the system prompt for tour generation.
const GenerationPrompt = `IMPORTANT: Your ENTIRE response must be a single JSON object. No preamble, no explanation, no markdown fences, no asking for permissions. Just the raw JSON.

You are analyzing a codebase to produce a CodeTour JSON object (printed in your response).

Requirements:
- Start at the entry point
- Walk through the architecture module by module
- Explain key design decisions and patterns
- Be useful for someone unfamiliar with the codebase
- 8-15 steps (not too many, not too few)
- Each step: file (relative path), line (specific line number), description (2-4 sentences, markdown)
- Start each description with a ## heading
%s

Output format (raw JSON, nothing else):
{
  "$schema": "https://aka.ms/codetour-schema",
  "title": "<descriptive title>",
  "steps": [
    { "file": "<relative/path>", "line": <number>, "description": "<markdown>" }
  ]
}

Repository context:

%s`

// BuildGenerationPrompt constructs the prompt for tour generation.
func BuildGenerationPrompt(context, focus string) string {
	var focusLine string
	if focus != "" {
		focusLine = fmt.Sprintf("Focus specifically on: %s", focus)
	}
	return fmt.Sprintf(GenerationPrompt, focusLine, context)
}

// ValidateTourJSON parses and validates generated tour JSON against the repo.
func ValidateTourJSON(data []byte, repoDir string) (*Tour, error) {
	var t Tour
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse tour JSON: %w", err)
	}

	if err := validate(&t); err != nil {
		return nil, err
	}

	// Validate files exist and lines are in range
	for i, step := range t.Steps {
		absPath := filepath.Join(repoDir, step.File)
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("step %d: file %q does not exist", i+1, step.File)
		}

		lineCount, err := countLines(absPath)
		if err != nil {
			continue
		}
		if step.Line > lineCount {
			return nil, fmt.Errorf("step %d: line %d exceeds file length (%d lines) in %s", i+1, step.Line, lineCount, step.File)
		}
	}

	return &t, nil
}

// ExtractJSON tries to extract JSON from Claude's output, handling markdown fences
// and preamble text before the fence.
func ExtractJSON(output string) string {
	output = strings.TrimSpace(output)

	// Try to extract from markdown code fence (handles preamble text before the fence)
	if idx := strings.Index(output, "```"); idx >= 0 {
		lines := strings.Split(output[idx:], "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				if inBlock {
					break
				}
				inBlock = true
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		if len(jsonLines) > 0 {
			return strings.Join(jsonLines, "\n")
		}
	}

	return output
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
