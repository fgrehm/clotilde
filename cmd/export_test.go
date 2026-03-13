package cmd_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/testutil"
)

var _ = Describe("Export Command", func() {
	var (
		tempDir      string
		clotildeRoot string
		originalWd   string
		store        session.Store
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()

		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		fakeClaudeDir := filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
	})

	AfterEach(func() {
		_ = os.Chdir(originalWd)
	})

	createSessionWithTranscript := func(name, uuid string) {
		sess := session.NewSession(name, uuid)
		transcriptPath := filepath.Join(tempDir, uuid+".jsonl")
		transcriptData := `{"type":"user","timestamp":"2025-01-01T00:01:00Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-01-01T00:01:05Z","message":{"content":[{"type":"text","text":"hi"}]}}
`
		err := os.WriteFile(transcriptPath, []byte(transcriptData), 0o644)
		Expect(err).NotTo(HaveOccurred())
		sess.Metadata.TranscriptPath = transcriptPath
		err = store.Create(sess)
		Expect(err).NotTo(HaveOccurred())
	}

	It("writes HTML file to working directory with default name", func() {
		createSessionWithTranscript("my-session", "uuid-123")

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "my-session"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		outputPath := filepath.Join(tempDir, "my-session.html")
		content, err := os.ReadFile(outputPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("<!DOCTYPE html>"))
		Expect(string(content)).To(ContainSubstring("my-session"))
	})

	It("writes to custom output path with -o flag", func() {
		createSessionWithTranscript("my-session", "uuid-456")

		outputPath := filepath.Join(tempDir, "custom.html")
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "my-session", "-o", outputPath})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(outputPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("<!DOCTYPE html>"))
	})

	It("writes HTML to stdout with --stdout flag", func() {
		createSessionWithTranscript("my-session", "uuid-789")

		var buf bytes.Buffer
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "my-session", "--stdout"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("<!DOCTYPE html>"))

		// No file should be created
		_, err = os.Stat(filepath.Join(tempDir, "my-session.html"))
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("returns error for non-existent session", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "does-not-exist"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("includes entries from previous transcripts when previousSessionIds is set", func() {
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())

		projectDir := claude.ProjectDir(clotildeRoot)
		claudeProjectDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
		err = os.MkdirAll(claudeProjectDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		// Previous transcript
		prevID := "uuid-prev-export-123"
		prevPath := filepath.Join(claudeProjectDir, prevID+".jsonl")
		prevData := `{"type":"user","message":{"content":"question from old session"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"answer from old session"}]}}
`
		err = os.WriteFile(prevPath, []byte(prevData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Current transcript
		currentID := "uuid-current-export-456"
		currentPath := filepath.Join(tempDir, currentID+".jsonl")
		currentData := `{"type":"user","message":{"content":"question from new session"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"answer from new session"}]}}
`
		err = os.WriteFile(currentPath, []byte(currentData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		sess := session.NewSession("multi-transcript-export", currentID)
		sess.Metadata.TranscriptPath = currentPath
		sess.Metadata.PreviousSessionIDs = []string{prevID}
		err = store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		var buf bytes.Buffer
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(&buf)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "multi-transcript-export", "--stdout"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		html := buf.String()
		// Session data is base64-encoded; decode it to check message content.
		const marker = `<script id="session-data" type="application/json">`
		start := strings.Index(html, marker)
		Expect(start).To(BeNumerically(">", 0), "session-data script tag should be present")
		start += len(marker)
		end := strings.Index(html[start:], "</script>")
		Expect(end).To(BeNumerically(">", 0))
		decoded, err := base64.StdEncoding.DecodeString(html[start : start+end])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(decoded)).To(ContainSubstring("old session"))
		Expect(string(decoded)).To(ContainSubstring("new session"))
	})

	It("returns error when transcript is missing", func() {
		sess := session.NewSession("no-transcript", "uuid-missing")
		sess.Metadata.TranscriptPath = "/nonexistent/path.jsonl"
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"export", "no-transcript"})

		err = rootCmd.Execute()
		Expect(err).To(HaveOccurred())
	})
})
