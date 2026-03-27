package export

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
)

//go:embed template.html template.css template.js vendored/marked.min.js vendored/highlight.min.js
var templateFS embed.FS

// transcriptEntry is a minimal struct for filtering JSONL lines.
type transcriptEntry struct {
	Type string `json:"type"`
}

// ExportData is the top-level structure serialized to JSON and base64-encoded.
type ExportData struct {
	SessionName string            `json:"sessionName"`
	Entries     []json.RawMessage `json:"entries"`
}

// FilterTranscript reads JSONL from r and returns only user and assistant entries as raw JSON.
// Uses bufio.Reader instead of bufio.Scanner to handle arbitrarily long lines
// (Claude transcripts can contain tool output blocks larger than 1MB).
func FilterTranscript(r io.Reader) ([]json.RawMessage, error) {
	reader := bufio.NewReader(r)

	var entries []json.RawMessage
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Trim trailing newline/carriage return
			line = bytes.TrimRight(line, "\r\n")

			if len(line) > 0 {
				var entry transcriptEntry
				if jsonErr := json.Unmarshal(line, &entry); jsonErr == nil {
					if entry.Type == "user" || entry.Type == "assistant" {
						raw := make(json.RawMessage, len(line))
						copy(raw, line)
						entries = append(entries, raw)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading transcript: %w", err)
		}
	}

	if entries == nil {
		entries = []json.RawMessage{}
	}

	return entries, nil
}

// BuildHTML assembles a self-contained HTML file from the session name and filtered entries.
func BuildHTML(sessionName string, entries []json.RawMessage) (string, error) {
	if entries == nil {
		entries = []json.RawMessage{}
	}

	data := ExportData{
		SessionName: sessionName,
		Entries:     entries,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshaling export data: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(jsonBytes)

	// Read template files
	htmlTemplate, err := templateFS.ReadFile("template.html")
	if err != nil {
		return "", fmt.Errorf("reading template.html: %w", err)
	}
	css, err := templateFS.ReadFile("template.css")
	if err != nil {
		return "", fmt.Errorf("reading template.css: %w", err)
	}
	js, err := templateFS.ReadFile("template.js")
	if err != nil {
		return "", fmt.Errorf("reading template.js: %w", err)
	}
	markedJS, err := templateFS.ReadFile("vendored/marked.min.js")
	if err != nil {
		return "", fmt.Errorf("reading marked.min.js: %w", err)
	}
	highlightJS, err := templateFS.ReadFile("vendored/highlight.min.js")
	if err != nil {
		return "", fmt.Errorf("reading highlight.min.js: %w", err)
	}

	// Replace placeholders (single pass)
	r := strings.NewReplacer(
		"{{TITLE}}", html.EscapeString(sessionName),
		"{{CSS}}", string(css),
		"{{SESSION_DATA}}", b64,
		"{{MARKED_JS}}", string(markedJS),
		"{{HIGHLIGHT_JS}}", string(highlightJS),
		"{{JS}}", string(js),
	)

	return r.Replace(string(htmlTemplate)), nil
}

