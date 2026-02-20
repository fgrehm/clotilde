package cmd_test

import (
	"encoding/json"
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

// executeHookWithInput executes a hook command with JSON input via stdin
func executeHookWithInput(hookName string, input []byte) error { //nolint:unparam // test helper, hookName kept for clarity
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdin = r

	go func() {
		defer func() { _ = w.Close() }()
		_, _ = w.Write(input)
	}()

	rootCmd := cmd.NewRootCmd()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"hook", hookName})
	err = rootCmd.Execute()

	os.Stdin = oldStdin
	return err
}

var _ = Describe("Hook Commands", func() {
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

	Describe("hook sessionstart", func() {
		Context("source: startup", func() {
			It("should output contexts for new sessions", func() {
				// Create global context
				globalContext := filepath.Join(clotildeRoot, "context.md")
				err := os.WriteFile(globalContext, []byte("Global context content"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				// Create hook input
				hookInput := map[string]string{
					"session_id": "some-uuid",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart with input via stdin
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())
				// Note: We can't easily capture stdout in tests, but we verify it doesn't error
			})

			It("should be idempotent - not overwrite existing UUID", func() {
				// Create fork with existing sessionId
				fork := session.NewSession("existing-fork", "existing-uuid")
				fork.Metadata.IsForkedSession = true
				fork.Metadata.ParentSession = "parent"
				err := store.Create(fork)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable for fork registration
				_ = os.Setenv("CLOTILDE_FORK_NAME", "existing-fork")
				defer func() { _ = os.Unsetenv("CLOTILDE_FORK_NAME") }()

				// Create hook input with different UUID
				hookInput := map[string]string{
					"session_id": "new-different-uuid",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify UUID was NOT changed
				updatedFork, err := store.Get("existing-fork")
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedFork.Metadata.SessionID).To(Equal("existing-uuid"))
			})

			It("should output global context if exists", func() {
				// Create global context file
				globalCtx := filepath.Join(clotildeRoot, config.GlobalContextFile)
				err := os.WriteFile(globalCtx, []byte("Global context content"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				// Create hook input
				hookInput := map[string]string{
					"session_id": "test-uuid",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Note: Output goes to stdout, would need to capture to verify content
			})

			It("should handle non-clotilde project gracefully", func() {
				// Change to a directory without clotilde
				nonClotildeDir := GinkgoT().TempDir()
				err := os.Chdir(nonClotildeDir)
				Expect(err).NotTo(HaveOccurred())

				// Create hook input
				hookInput := map[string]string{
					"session_id": "test-uuid",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart - should not error
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error when session has context set", func() {
				// Create session with context
				sess := session.NewSession("session-with-context", "test-uuid-ctx")
				sess.Metadata.Context = "working on GH-123"
				err := store.Create(sess)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable with session name
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "session-with-context")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				// Create hook input
				hookInput := map[string]string{
					"session_id": "test-uuid-ctx",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())
				// Note: Output includes "Session name: session-with-context" and "Context: working on GH-123"
				// but we can't easily capture stdout in tests
			})

			It("should save transcript path from hook input", func() {
				// Create session
				sess := session.NewSession("session-with-transcript", "test-uuid-123")
				err := store.Create(sess)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable with session name
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "session-with-transcript")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				// Create hook input with transcript_path
				hookInput := map[string]string{
					"session_id":      "test-uuid-123",
					"transcript_path": "/home/user/.claude/projects/test-project/test-uuid-123.jsonl",
					"source":          "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify transcript path was saved
				updatedSess, err := store.Get("session-with-transcript")
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedSess.Metadata.TranscriptPath).To(Equal("/home/user/.claude/projects/test-project/test-uuid-123.jsonl"))
			})
		})

		Context("source: resume", func() {
			It("should register fork session ID", func() {
				// Create fork with empty sessionId
				fork := session.NewSession("test-fork", "")
				fork.Metadata.IsForkedSession = true
				fork.Metadata.ParentSession = "parent"
				err := store.Create(fork)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable for fork registration
				_ = os.Setenv("CLOTILDE_FORK_NAME", "test-fork")
				defer func() { _ = os.Unsetenv("CLOTILDE_FORK_NAME") }()

				// Create hook input with session UUID
				hookInput := map[string]string{
					"session_id": "new-fork-uuid-123",
					"source":     "resume",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify UUID was registered
				updatedFork, err := store.Get("test-fork")
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedFork.Metadata.SessionID).To(Equal("new-fork-uuid-123"))
			})

			It("should be idempotent - not overwrite existing fork UUID", func() {
				// Create fork with existing sessionId
				fork := session.NewSession("existing-fork-resume", "existing-uuid-resume")
				fork.Metadata.IsForkedSession = true
				fork.Metadata.ParentSession = "parent"
				err := store.Create(fork)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable for fork registration
				_ = os.Setenv("CLOTILDE_FORK_NAME", "existing-fork-resume")
				defer func() { _ = os.Unsetenv("CLOTILDE_FORK_NAME") }()

				// Create hook input with different UUID
				hookInput := map[string]string{
					"session_id": "new-different-uuid-resume",
					"source":     "resume",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify UUID was NOT changed
				updatedFork, err := store.Get("existing-fork-resume")
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedFork.Metadata.SessionID).To(Equal("existing-uuid-resume"))
			})

			It("should output global context if exists", func() {
				// Create global context file
				globalCtx := filepath.Join(clotildeRoot, config.GlobalContextFile)
				err := os.WriteFile(globalCtx, []byte("Global context for resume"), 0o644)
				Expect(err).NotTo(HaveOccurred())

				// Create hook input
				hookInput := map[string]string{
					"session_id": "resume-uuid",
					"source":     "resume",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle non-clotilde project gracefully", func() {
				// Change to a directory without clotilde
				nonClotildeDir := GinkgoT().TempDir()
				err := os.Chdir(nonClotildeDir)
				Expect(err).NotTo(HaveOccurred())

				// Create hook input
				hookInput := map[string]string{
					"session_id": "resume-uuid",
					"source":     "resume",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart - should not error
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should work with invalid JSON input gracefully", func() {
				// Execute hook sessionstart with invalid input
				err := executeHookWithInput("sessionstart", []byte("not json"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("parse"))
			})

			It("should save transcript path from hook input on resume", func() {
				// Create session
				sess := session.NewSession("session-resume-transcript", "test-uuid-456")
				err := store.Create(sess)
				Expect(err).NotTo(HaveOccurred())

				// Set environment variable with session name
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "session-resume-transcript")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				// Create hook input with transcript_path
				hookInput := map[string]string{
					"session_id":      "test-uuid-456",
					"transcript_path": "/home/user/.claude/projects/test-project/test-uuid-456.jsonl",
					"source":          "resume",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				// Execute hook sessionstart
				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify transcript path was saved
				updatedSess, err := store.Get("session-resume-transcript")
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedSess.Metadata.TranscriptPath).To(Equal("/home/user/.claude/projects/test-project/test-uuid-456.jsonl"))
			})
		})
	})
})
