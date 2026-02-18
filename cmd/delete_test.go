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

var _ = Describe("Delete Command", func() {
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

		Expect(err).NotTo(HaveOccurred())

		// Initialize clotilde
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
	})

	AfterEach(func() {
		// Restore PATH

		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	It("should delete session with --force flag", func() {
		// Create a session first
		sess := session.NewSession("to-delete", "uuid-delete-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Verify session exists
		sessionDir := config.GetSessionDir(clotildeRoot, "to-delete")
		_, err = os.Stat(sessionDir)
		Expect(err).NotTo(HaveOccurred())

		// Execute delete command with --force
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"delete", "to-delete", "--force"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify session was deleted
		_, err = os.Stat(sessionDir)
		Expect(err).To(HaveOccurred())
		Expect(os.IsNotExist(err)).To(BeTrue())

		// Verify session is no longer in store
		_, err = store.Get("to-delete")
		Expect(err).To(HaveOccurred())
	})

	It("should return error for non-existent session", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"delete", "does-not-exist", "--force"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("should delete session data including settings and prompts", func() {
		// Create session with settings and system prompt
		sess := session.NewSession("full-session", "uuid-full-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		settings := &session.Settings{Model: "sonnet"}
		err = store.SaveSettings("full-session", settings)
		Expect(err).NotTo(HaveOccurred())

		err = store.SaveSystemPrompt("full-session", "Test prompt")
		Expect(err).NotTo(HaveOccurred())

		// Verify files exist
		sessionDir := config.GetSessionDir(clotildeRoot, "full-session")
		settingsPath := filepath.Join(sessionDir, "settings.json")
		promptPath := filepath.Join(sessionDir, "system-prompt.md")

		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Stat(promptPath)
		Expect(err).NotTo(HaveOccurred())

		// Delete session
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"delete", "full-session", "-f"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify all files deleted
		_, err = os.Stat(sessionDir)
		Expect(err).To(HaveOccurred())
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
})
