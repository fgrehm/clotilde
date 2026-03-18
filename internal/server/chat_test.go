package server_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"nhooyr.io/websocket"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/server"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("WebSocket Chat", func() {
	var (
		srv    *server.Server
		ts     *httptest.Server
		repoDir string
	)

	BeforeEach(func() {
		repoDir = GinkgoT().TempDir()

		// Create tour
		toursDir := filepath.Join(repoDir, ".tours")
		Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())
		tourJSON := `{"title": "Test Tour", "steps": [{"file": "main.go", "line": 1, "description": "Entry"}]}`
		Expect(os.WriteFile(filepath.Join(toursDir, "test.tour"), []byte(tourJSON), 0o644)).To(Succeed())

		sess := session.NewSession("test-tour", util.GenerateUUID())
		srv = server.New(0, repoDir, "haiku", sess)
		ts = httptest.NewServer(srv.Handler())
	})

	AfterEach(func() {
		ts.Close()
	})

	It("establishes WebSocket connection", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL := "ws" + ts.URL[4:] + "/ws/chat"
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		Expect(err).NotTo(HaveOccurred())
		conn.Close(websocket.StatusNormalClosure, "")
	})

	It("calls InvokeStreaming and streams response", func() {
		// Override InvokeStreaming to simulate Claude output
		origFunc := server.InvokeStreamingFunc
		defer func() { server.InvokeStreamingFunc = origFunc }()

		server.InvokeStreamingFunc = func(opts claude.InvokeOptions, prompt string, onLine func(string)) error {
			// Simulate stream-json output
			onLine(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello "}]}}`)
			onLine(`{"type":"assistant","message":{"content":[{"type":"text","text":"world"}]}}`)
			onLine(`{"type":"result","subtype":"success","result":"Hello world"}`)
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL := "ws" + ts.URL[4:] + "/ws/chat"
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close(websocket.StatusNormalClosure, "")

		// Send chat message
		msg := map[string]any{
			"type":    "chat",
			"message": "What is this?",
			"context": map[string]any{
				"tour": "test",
				"step": 0,
				"file": "main.go",
				"line": 1,
			},
		}
		data, _ := json.Marshal(msg)
		err = conn.Write(ctx, websocket.MessageText, data)
		Expect(err).NotTo(HaveOccurred())

		// Read responses
		var responses []map[string]any
		for {
			_, respData, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())

			var resp map[string]any
			Expect(json.Unmarshal(respData, &resp)).To(Succeed())
			responses = append(responses, resp)

			if resp["type"] == "done" || resp["type"] == "error" {
				break
			}
		}

		// Should have token messages then done
		Expect(responses).To(HaveLen(3))
		Expect(responses[0]["type"]).To(Equal("token"))
		Expect(responses[0]["content"]).To(Equal("Hello "))
		Expect(responses[1]["type"]).To(Equal("token"))
		Expect(responses[1]["content"]).To(Equal("world"))
		Expect(responses[2]["type"]).To(Equal("done"))
	})

	It("includes tour context in prompt", func() {
		var capturedPrompt string

		origFunc := server.InvokeStreamingFunc
		defer func() { server.InvokeStreamingFunc = origFunc }()

		server.InvokeStreamingFunc = func(opts claude.InvokeOptions, prompt string, onLine func(string)) error {
			capturedPrompt = prompt
			onLine(`{"type":"result","subtype":"success","result":"ok"}`)
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL := "ws" + ts.URL[4:] + "/ws/chat"
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close(websocket.StatusNormalClosure, "")

		msg := map[string]any{
			"type":    "chat",
			"message": "Explain this code",
			"context": map[string]any{
				"tour": "test",
				"step": 0,
				"file": "main.go",
				"line": 1,
			},
		}
		data, _ := json.Marshal(msg)
		conn.Write(ctx, websocket.MessageText, data)

		// Read until done
		for {
			_, respData, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())
			var resp map[string]any
			json.Unmarshal(respData, &resp)
			if resp["type"] == "done" {
				break
			}
		}

		Expect(capturedPrompt).To(ContainSubstring("Test Tour"))
		Expect(capturedPrompt).To(ContainSubstring("Step 1/1"))
		Expect(capturedPrompt).To(ContainSubstring("main.go:1"))
		Expect(capturedPrompt).To(ContainSubstring("Explain this code"))
	})

	It("sends error when invoke fails", func() {
		origFunc := server.InvokeStreamingFunc
		defer func() { server.InvokeStreamingFunc = origFunc }()

		server.InvokeStreamingFunc = func(_ claude.InvokeOptions, _ string, _ func(string)) error {
			return context.DeadlineExceeded
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL := "ws" + ts.URL[4:] + "/ws/chat"
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close(websocket.StatusNormalClosure, "")

		msg := map[string]any{
			"type":    "chat",
			"message": "hello",
			"context": map[string]any{},
		}
		data, _ := json.Marshal(msg)
		conn.Write(ctx, websocket.MessageText, data)

		_, respData, err := conn.Read(ctx)
		Expect(err).NotTo(HaveOccurred())

		var resp map[string]any
		Expect(json.Unmarshal(respData, &resp)).To(Succeed())
		Expect(resp["type"]).To(Equal("error"))
		Expect(resp["message"]).To(ContainSubstring("Claude error"))
	})
})
