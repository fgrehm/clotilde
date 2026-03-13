package cmd_test

import (
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/testutil"
)

var _ = Describe("Stats Command", func() {
	var (
		tempDir      string
		clotildeRoot string
		originalWd   string
		store        session.Store
	)

	BeforeEach(func() {
		// Create temp directory
		tempDir = GinkgoT().TempDir()

		// Save original working directory
		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		// Change to temp directory
		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Setup fake claude binary
		fakeClaudeDir := filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		// Initialize clotilde
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
	})

	AfterEach(func() {
		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	It("should return error for non-existent session", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"stats", "does-not-exist"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("should show no transcript found for session without transcript", func() {
		// Create a session
		sess := session.NewSession("empty-session", "uuid-empty-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Execute stats command
		output := captureOutput(func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"stats", "empty-session"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(output).To(ContainSubstring("No transcript found"))
	})

	It("should show turns count for session with transcript", func() {
		// Create a session
		sess := session.NewSession("with-transcript", "uuid-transcript-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Create a fake transcript with one turn
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())

		projectDir := filepath.Join(".claude", "projects", "-temp-bin")
		claudeProjectDir := filepath.Join(homeDir, projectDir)
		err = os.MkdirAll(claudeProjectDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		transcriptPath := filepath.Join(claudeProjectDir, "uuid-transcript-123.jsonl")
		transcriptData := `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:15Z","message":{"content":"hi"}}`

		err = os.WriteFile(transcriptPath, []byte(transcriptData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Update session with transcript path
		sess.Metadata.TranscriptPath = transcriptPath
		err = store.Update(sess)
		Expect(err).NotTo(HaveOccurred())

		// Execute stats command
		output := captureOutput(func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"stats", "with-transcript"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(output).To(ContainSubstring("Turns"))
		Expect(output).To(ContainSubstring("Started"))
		Expect(output).To(ContainSubstring("Last active"))
		Expect(output).To(ContainSubstring("(approx)"))
	})

	It("sums turns across previous and current transcripts", func() {
		// Point HOME at the temp dir so transcript paths stay hermetic.
		GinkgoT().Setenv("HOME", tempDir)
		homeDir := tempDir

		projectDir := claude.ProjectDir(clotildeRoot)
		claudeProjectDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
		err := os.MkdirAll(claudeProjectDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		// Previous transcript: 1 turn
		prevID := "uuid-prev-stats-123"
		prevPath := filepath.Join(claudeProjectDir, prevID+".jsonl")
		prevData := `{"type":"progress","timestamp":"2025-01-01T10:00:00Z"}
{"type":"user","timestamp":"2025-01-01T10:00:10Z","message":{"content":"old question"}}
{"type":"assistant","timestamp":"2025-01-01T10:00:20Z","message":{"content":"old answer"}}`
		err = os.WriteFile(prevPath, []byte(prevData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Current transcript: 2 turns
		currentID := "uuid-current-stats-456"
		currentPath := filepath.Join(claudeProjectDir, currentID+".jsonl")
		currentData := `{"type":"progress","timestamp":"2025-02-01T10:00:00Z"}
{"type":"user","timestamp":"2025-02-01T10:00:10Z","message":{"content":"turn 1"}}
{"type":"assistant","timestamp":"2025-02-01T10:00:20Z","message":{"content":"answer 1"}}
{"type":"user","timestamp":"2025-02-01T10:01:00Z","message":{"content":"turn 2"}}
{"type":"assistant","timestamp":"2025-02-01T10:01:15Z","message":{"content":"answer 2"}}`
		err = os.WriteFile(currentPath, []byte(currentData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		sess := session.NewSession("multi-transcript-stats", currentID)
		sess.Metadata.TranscriptPath = currentPath
		sess.Metadata.PreviousSessionIDs = []string{prevID}
		err = store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		output := captureOutput(func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"stats", "multi-transcript-stats"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		// 1 turn from previous + 2 turns from current = 3 total
		Expect(output).To(ContainSubstring("Turns         3"))
		// Started should reflect the earlier transcript
		Expect(output).To(ContainSubstring("Jan 1, 2025"))
		// Last active should reflect the newer transcript
		Expect(output).To(ContainSubstring("Feb 1, 2025"))
	})

	It("should show zero turns for empty transcript", func() {
		// Create a session
		sess := session.NewSession("empty-transcript", "uuid-empty-trans")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Create an empty transcript
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())

		projectDir := filepath.Join(".claude", "projects", "-temp-bin")
		claudeProjectDir := filepath.Join(homeDir, projectDir)
		err = os.MkdirAll(claudeProjectDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		transcriptPath := filepath.Join(claudeProjectDir, "uuid-empty-trans.jsonl")
		err = os.WriteFile(transcriptPath, []byte(""), 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Update session with transcript path
		sess.Metadata.TranscriptPath = transcriptPath
		err = store.Update(sess)
		Expect(err).NotTo(HaveOccurred())

		// Execute stats command
		output := captureOutput(func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"stats", "empty-transcript"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(output).To(ContainSubstring("No transcript found"))
	})
})

// Helper function to capture stdout
func captureOutput(fn func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	bytes := make([]byte, 4096)
	n, _ := r.Read(bytes)
	return string(bytes[:n])
}
