package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/coder/websocket"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/tour"
)

// chatMessage is sent from the browser to the server.
type chatMessage struct {
	Type    string      `json:"type"`
	Message string      `json:"message"`
	Context chatContext `json:"context"`
}

// chatContext carries the current tour position.
type chatContext struct {
	Tour string `json:"tour"`
	Step int    `json:"step"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// chatResponse is sent from the server to the browser.
type chatResponse struct {
	Type    string `json:"type"`              // "token", "done", "error"
	Content string `json:"content,omitempty"` // for "token" type
	Message string `json:"message,omitempty"` // for "error" type
}

// chatSession tracks Claude Code session state for a WebSocket connection.
type chatSession struct {
	started bool // whether a successful call has been made (transcript exists)
	mu      sync.Mutex
	busy    bool
}

// InvokeStreamingFunc is the function used to invoke Claude Code in streaming mode.
// Can be overridden in tests.
var InvokeStreamingFunc = claude.InvokeStreaming

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		return
	}
	defer conn.CloseNow() //nolint:errcheck

	cs := &chatSession{
		started: claude.DefaultSessionUsed(s.clotildeRoot, s.session),
	}
	ctx := r.Context()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg chatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = writeWS(ctx, conn, chatResponse{Type: "error", Message: "invalid message format"})
			continue
		}

		if msg.Type != "chat" || msg.Message == "" {
			continue
		}

		cs.mu.Lock()
		if cs.busy {
			cs.mu.Unlock()
			_ = writeWS(ctx, conn, chatResponse{Type: "error", Message: "still processing previous message"})
			continue
		}
		cs.busy = true
		cs.mu.Unlock()

		go s.handleChat(ctx, conn, cs, msg)
	}
}

func (s *Server) handleChat(ctx context.Context, conn *websocket.Conn, cs *chatSession, msg chatMessage) {
	defer func() {
		cs.mu.Lock()
		cs.busy = false
		cs.mu.Unlock()
	}()

	// Build prompt with tour context (system prompt will come from session)
	prompt := buildPrompt(s.tours, msg)

	// Read started under lock to avoid data race with the busy guard.
	cs.mu.Lock()
	resume := cs.started
	cs.mu.Unlock()

	// Build path to system prompt file
	sessionDir := config.GetSessionDir(s.clotildeRoot, s.session.Name)
	systemPromptPath := filepath.Join(sessionDir, "system-prompt.md")

	opts := claude.InvokeOptions{
		SessionID:        s.session.Metadata.SessionID,
		Resume:           resume, // first message uses --session-id to create transcript
		SystemPromptFile: systemPromptPath,
		SystemPromptMode: s.session.Metadata.GetSystemPromptMode(),
		AdditionalArgs:   []string{"--model", s.model},
	}

	var aborted bool
	err := InvokeStreamingFunc(opts, prompt, func(line string) {
		if aborted {
			return
		}
		// Parse streaming JSON to extract text content
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			return
		}

		if parsed["type"] == "assistant" {
			msgObj, ok := parsed["message"].(map[string]any)
			if !ok {
				return
			}
			content, ok := msgObj["content"].([]any)
			if !ok {
				return
			}
			for _, block := range content {
				blockMap, ok := block.(map[string]any)
				if !ok {
					continue
				}
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						if err := writeWS(ctx, conn, chatResponse{Type: "token", Content: text}); err != nil {
							aborted = true
							return
						}
					}
				}
			}
		}
	})
	if err != nil {
		writeWS(ctx, conn, chatResponse{Type: "error", Message: fmt.Sprintf("Claude error: %v", err)}) //nolint:errcheck
		return
	}

	cs.mu.Lock()
	cs.started = true
	cs.mu.Unlock()

	writeWS(ctx, conn, chatResponse{Type: "done"}) //nolint:errcheck
}

func buildPrompt(tours map[string]*tour.Tour, msg chatMessage) string {
	var context string
	if t, ok := tours[msg.Context.Tour]; ok {
		stepNum := msg.Context.Step + 1
		total := len(t.Steps)
		var stepDesc string
		if msg.Context.Step >= 0 && msg.Context.Step < len(t.Steps) {
			stepDesc = t.Steps[msg.Context.Step].Description
		}
		context = fmt.Sprintf("[Tour: %q - Step %d/%d]\n[File: %s:%d]\n[Step description: %s]\n\n",
			t.Title, stepNum, total, msg.Context.File, msg.Context.Line, stepDesc)
	}
	return context + msg.Message
}

func writeWS(ctx context.Context, conn *websocket.Conn, resp chatResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
