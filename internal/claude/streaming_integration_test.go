package claude_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

// runStreamingClaude spawns claude in non-interactive streaming mode and returns
// parsed JSON lines from stdout.
func runStreamingClaude(args ...string) ([]map[string]any, error) {
	cmd := exec.Command("claude", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	var lines []map[string]any
	scanner := bufio.NewScanner(stdout)
	// Claude can emit large JSON lines (init message with tool lists, etc.)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for scanner.Scan() {
		var parsed map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse JSON line: %w\nline: %s", err, scanner.Text())
		}
		lines = append(lines, parsed)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("claude exited with error: %w", err)
	}

	return lines, nil
}

// extractTextContent extracts all text content from assistant messages.
func extractTextContent(lines []map[string]any) string {
	var texts []string
	for _, line := range lines {
		if line["type"] != "assistant" {
			continue
		}
		msg, ok := line["message"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, block := range content {
			blockMap, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if blockMap["type"] == "text" {
				if text, ok := blockMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
	}
	return strings.Join(texts, "")
}

// findResultMessage returns the result message from parsed lines, or nil.
func findResultMessage(lines []map[string]any) map[string]any {
	for _, line := range lines {
		if line["type"] == "result" {
			return line
		}
	}
	return nil
}

var _ = Describe("Streaming Claude Integration", Label("integration"), func() {
	var (
		clotildeRoot string
		store        *session.FileStore
		sess         *session.Session
		sessionName  string
	)

	BeforeEach(func() {
		// Skip if claude binary is not available
		if _, err := exec.LookPath("claude"); err != nil {
			Skip("claude binary not found on PATH, skipping integration test")
		}

		// Use the real clotilde root (we need hooks to work)
		var err error
		clotildeRoot, err = config.FindOrCreateClotildeRoot()
		Expect(err).NotTo(HaveOccurred())

		// Create a session with tour- prefix
		sessionName = "tour-" + util.GenerateRandomName()
		sessionID := util.GenerateUUID()
		sess = session.NewSession(sessionName, sessionID)
		store = session.NewFileStore(clotildeRoot)
		Expect(store.Create(sess)).To(Succeed())
	})

	AfterEach(func() {
		if store == nil || sessionName == "" {
			return
		}
		// Clean up Claude transcript data
		_, _ = claude.DeleteSessionData(clotildeRoot, sess.Metadata.SessionID, sess.Metadata.TranscriptPath)
		// Clean up Clotilde session
		_ = store.Delete(sessionName)
	})

	It("captures streaming JSON output and resumes session", func() {
		// Turn 1: start a new session
		lines, err := runStreamingClaude(
			"--session-id", sess.Metadata.SessionID,
			"-p", "Say just the word hello",
			"--output-format", "stream-json",
			"--verbose",
			"--model", "haiku",
		)
		Expect(err).NotTo(HaveOccurred())

		// Should have parsed at least some JSON lines
		Expect(lines).NotTo(BeEmpty())

		// Should have received text content
		text := extractTextContent(lines)
		Expect(strings.ToLower(text)).To(ContainSubstring("hello"))

		// Should have a successful result message
		result := findResultMessage(lines)
		Expect(result).NotTo(BeNil())
		Expect(result["subtype"]).To(Equal("success"))
		Expect(result["is_error"]).To(Equal(false))

		// Turn 2: resume the same session
		resumeLines, err := runStreamingClaude(
			"--resume", sess.Metadata.SessionID,
			"-p", "What did I just ask you to do?",
			"--output-format", "stream-json",
			"--verbose",
			"--model", "haiku",
		)
		Expect(err).NotTo(HaveOccurred())

		// Should reference "hello" from the previous turn
		resumeText := extractTextContent(resumeLines)
		Expect(strings.ToLower(resumeText)).To(ContainSubstring("hello"))

		// Should also succeed
		resumeResult := findResultMessage(resumeLines)
		Expect(resumeResult).NotTo(BeNil())
		Expect(resumeResult["subtype"]).To(Equal("success"))
	})
})
