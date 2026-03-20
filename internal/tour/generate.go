package tour

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fgrehm/clotilde/internal/util"
)

// StreamEvent represents a parsed stream-json event from Claude.
type StreamEvent struct {
	Type    string         `json:"type"`
	Message *StreamMessage `json:"message,omitempty"`
	Result  string         `json:"result,omitempty"`
}

// StreamMessage is the message payload in an "assistant" stream event.
type StreamMessage struct {
	Content []StreamContent `json:"content"`
}

// StreamContent is one content block in an assistant message.
type StreamContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

// ParseStreamEvent parses one stream-json line into a StreamEvent.
func ParseStreamEvent(line string) (StreamEvent, error) {
	var ev StreamEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return ev, err
	}
	return ev, nil
}

// ToolCallSummary returns a short human-readable description of what tool Claude
// is calling, for progress display. Returns empty string if not a tool call.
func ToolCallSummary(ev StreamEvent) string {
	if ev.Type != "assistant" || ev.Message == nil {
		return ""
	}
	for _, c := range ev.Message.Content {
		if c.Type != "tool_use" {
			continue
		}
		switch c.Name {
		case "Read":
			if path, ok := c.Input["file_path"].(string); ok {
				return fmt.Sprintf("Reading %s", path)
			}
		case "Glob":
			if pattern, ok := c.Input["pattern"].(string); ok {
				return fmt.Sprintf("Globbing %s", pattern)
			}
		case "Grep":
			if pattern, ok := c.Input["pattern"].(string); ok {
				return fmt.Sprintf("Grepping for %q", pattern)
			}
		default:
			return fmt.Sprintf("Using %s", c.Name)
		}
	}
	return ""
}

// GenerationOpts controls the generation prompt parameters.
type GenerationOpts struct {
	RepoDir  string
	Focus    string
	MaxFiles int // max files to read (default 20)
	MinSteps int // min steps to produce (default 8)
	MaxSteps int // max steps to produce (default 15)
}

// BuildGenerationPrompt constructs the prompt for tour generation.
func BuildGenerationPrompt(opts GenerationOpts) string {
	maxFiles := opts.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 20
	}
	minSteps := opts.MinSteps
	if minSteps <= 0 {
		minSteps = 8
	}
	maxSteps := opts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 15
	}
	if maxSteps < minSteps {
		maxSteps = minSteps
	}

	var focusLine string
	if opts.Focus != "" {
		focusLine = fmt.Sprintf("\n- Focus specifically on: %s", opts.Focus)
	}

	return fmt.Sprintf(`Explore the repository at %s using your file tools (Glob, Read, Grep).

Your goal: produce a CodeTour that walks an unfamiliar developer through the codebase architecture.

Rules:
- Read at most %d files total. Start with entry points and README, then follow the most important paths.
- Produce %d-%d steps. Do not produce more or fewer.
- Each step: file path relative to repo root, a specific line number, 2-4 sentence description.
- Start each description with a ## heading.
- Steps must follow logical reading order (entry point -> core modules -> periphery).%s

When you are done exploring, output ONLY a raw JSON object. No preamble, no explanation, no markdown fences.

Output format:
{
  "$schema": "https://aka.ms/codetour-schema",
  "title": "<descriptive title>",
  "steps": [
    { "file": "<relative/path>", "line": <number>, "description": "<markdown>" }
  ]
}`, opts.RepoDir, maxFiles, minSteps, maxSteps, focusLine)
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
		if filepath.IsAbs(step.File) {
			return nil, fmt.Errorf("step %d: file path must be relative, got %q", i+1, step.File)
		}

		absPath := filepath.Join(repoDir, step.File)
		rel, relErr := filepath.Rel(repoDir, absPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("step %d: file path %q escapes repo directory", i+1, step.File)
		}

		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("step %d: file %q does not exist", i+1, step.File)
		}

		lineCount, err := util.CountLines(absPath)
		if err != nil {
			return nil, fmt.Errorf("step %d: failed to count lines in %s: %w", i+1, step.File, err)
		}
		if step.Line > lineCount {
			return nil, fmt.Errorf("step %d: line %d exceeds file length (%d lines) in %s", i+1, step.Line, lineCount, step.File)
		}
	}

	return &t, nil
}

// ExtractJSON tries to extract JSON from Claude's output, handling markdown fences
// and preamble text before the JSON object.
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

	// Strip preamble text before the first '{' (Claude sometimes adds commentary)
	if idx := strings.Index(output, "{"); idx > 0 {
		// Find the matching closing brace by scanning from the end
		if last := strings.LastIndex(output, "}"); last > idx {
			return output[idx : last+1]
		}
	}

	return output
}
