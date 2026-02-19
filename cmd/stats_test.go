package cmd_test

import (
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
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
