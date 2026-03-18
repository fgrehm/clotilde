package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"nhooyr.io/websocket"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/tour"
	"github.com/fgrehm/clotilde/internal/util"
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
	sessionID string
	started   bool // true after first successful invocation
	mu        sync.Mutex
	busy      bool
}

// InvokeStreamingFunc is the function used to invoke Claude Code in streaming mode.
// Can be overridden in tests.
var InvokeStreamingFunc = claude.InvokeStreaming

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	cs := &chatSession{}
	ctx := r.Context()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg chatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			writeWS(conn, chatResponse{Type: "error", Message: "invalid message format"})
			continue
		}

		if msg.Type != "chat" || msg.Message == "" {
			continue
		}

		cs.mu.Lock()
		if cs.busy {
			cs.mu.Unlock()
			writeWS(conn, chatResponse{Type: "error", Message: "still processing previous message"})
			continue
		}
		cs.busy = true
		cs.mu.Unlock()

		go s.handleChat(conn, cs, msg)
	}
}

func (s *Server) handleChat(conn *websocket.Conn, cs *chatSession, msg chatMessage) {
	defer func() {
		cs.mu.Lock()
		cs.busy = false
		cs.mu.Unlock()
	}()

	// Build prompt with tour context
	prompt := buildPrompt(s.tours, msg)

	// Create session ID on first message
	if cs.sessionID == "" {
		cs.sessionID = util.GenerateUUID()
	}

	opts := claude.InvokeOptions{
		SessionID:      cs.sessionID,
		Resume:         cs.started,
		AdditionalArgs: []string{"--model", "haiku"},
	}

	err := InvokeStreamingFunc(opts, prompt, func(line string) {
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
						writeWS(conn, chatResponse{Type: "token", Content: text})
					}
				}
			}
		}

	})

	if err != nil {
		writeWS(conn, chatResponse{Type: "error", Message: fmt.Sprintf("Claude error: %v", err)})
		return
	}

	cs.started = true
	writeWS(conn, chatResponse{Type: "done"})
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

func writeWS(conn *websocket.Conn, resp chatResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	conn.Write(context.Background(), websocket.MessageText, data)
}
