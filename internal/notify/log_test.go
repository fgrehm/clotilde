package notify_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/notify"
)

var _ = Describe("LogEvent", func() {
	var (
		originalLogDir string
		logDir         string
	)

	BeforeEach(func() {
		originalLogDir = notify.LogDir
		logDir = filepath.Join(GinkgoT().TempDir(), "logs")
		notify.LogDir = logDir
	})

	AfterEach(func() {
		notify.LogDir = originalLogDir
	})

	It("should create JSONL file with session_id in filename", func() {
		input := []byte(`{"session_id":"test-uuid-123","hook_event_name":"Stop"}`)
		err := notify.LogEvent(input, "test-uuid-123")
		Expect(err).NotTo(HaveOccurred())

		logFile := filepath.Join(logDir, "test-uuid-123.events.jsonl")
		Expect(logFile).To(BeAnExistingFile())
	})

	It("should append multiple events to same file", func() {
		event1 := []byte(`{"session_id":"multi","hook_event_name":"Stop"}`)
		event2 := []byte(`{"session_id":"multi","hook_event_name":"PreToolUse"}`)

		err := notify.LogEvent(event1, "multi")
		Expect(err).NotTo(HaveOccurred())
		err = notify.LogEvent(event2, "multi")
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(filepath.Join(logDir, "multi.events.jsonl"))
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		Expect(lines).To(HaveLen(2))
	})

	It("should write each line as valid JSON", func() {
		event1 := []byte(`{"session_id":"json-check","event":"one"}`)
		event2 := []byte(`{"session_id":"json-check","event":"two"}`)

		err := notify.LogEvent(event1, "json-check")
		Expect(err).NotTo(HaveOccurred())
		err = notify.LogEvent(event2, "json-check")
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(filepath.Join(logDir, "json-check.events.jsonl"))
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		for _, line := range lines {
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(line), &parsed)
			Expect(err).NotTo(HaveOccurred(), "line should be valid JSON: %s", line)
		}
	})

	It("should be a no-op when session_id is empty", func() {
		input := []byte(`{"hook_event_name":"Stop"}`)
		err := notify.LogEvent(input, "")
		Expect(err).NotTo(HaveOccurred())

		// Log directory should not have been created
		_, err = os.Stat(logDir)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("should create log directory if it doesn't exist", func() {
		// logDir doesn't exist yet (BeforeEach only sets the path)
		_, err := os.Stat(logDir)
		Expect(os.IsNotExist(err)).To(BeTrue())

		input := []byte(`{"session_id":"create-dir"}`)
		err = notify.LogEvent(input, "create-dir")
		Expect(err).NotTo(HaveOccurred())

		Expect(logDir).To(BeADirectory())
	})
})
