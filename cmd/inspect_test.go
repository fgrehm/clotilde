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

var _ = Describe("Inspect Command", func() {
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

	It("should show basic session information", func() {
		// Create a session
		sess := session.NewSession("inspect-me", "uuid-inspect-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Execute inspect command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "inspect-me"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Note: We're just verifying it doesn't error
		// Actual output would be to stdout
	})

	It("should show session with settings and prompt", func() {
		// Create session with settings and system prompt
		sess := session.NewSession("full-inspect", "uuid-full-inspect")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		settings := &session.Settings{
			Model: "sonnet",
			Permissions: session.Permissions{
				Allow: []string{"Bash", "Read"},
				Deny:  []string{"Write"},
			},
		}
		err = store.SaveSettings("full-inspect", settings)
		Expect(err).NotTo(HaveOccurred())

		err = store.SaveSystemPrompt("full-inspect", "Test system prompt")
		Expect(err).NotTo(HaveOccurred())

		// Execute inspect command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "full-inspect"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should show fork information", func() {
		// Create parent
		parent := session.NewSession("parent", "uuid-parent")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		// Create fork
		fork := session.NewSession("fork", "uuid-fork")
		fork.Metadata.IsForkedSession = true
		fork.Metadata.ParentSession = "parent"
		err = store.Create(fork)
		Expect(err).NotTo(HaveOccurred())

		// Execute inspect command on fork
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "fork"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should show context information", func() {
		// Create session with context
		sess := session.NewSession("ctx-session", "uuid-ctx")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Add global context
		globalCtx := filepath.Join(clotildeRoot, config.GlobalContextFile)
		err = os.WriteFile(globalCtx, []byte("Global context line 1\nGlobal context line 2\n"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Execute inspect command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "ctx-session"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return error for non-existent session", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "does-not-exist"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("should handle minimal session with no files", func() {
		// Create minimal session (just metadata.json)
		sess := session.NewSession("minimal", "uuid-minimal")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		// Execute inspect command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"inspect", "minimal"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())
	})
})
