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
	"github.com/fgrehm/clotilde/internal/notify"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/testutil"
)

// testFakeTabRenamer records RenameTab calls for testing.
type testFakeTabRenamer struct {
	calls []string
}

func (f *testFakeTabRenamer) RenameTab(name string) error {
	f.calls = append(f.calls, name)
	return nil
}

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
		tempDir        string
		clotildeRoot   string
		originalWd     string
		originalLogDir string
		notifyLogDir   string
		store          session.Store
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

		// Override notify log dir for all hook tests
		originalLogDir = notify.LogDir
		notifyLogDir = filepath.Join(tempDir, "notify-logs")
		notify.LogDir = notifyLogDir
	})

	AfterEach(func() {
		notify.LogDir = originalLogDir

		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	Describe("hook sessionstart", func() {
		Context("source: startup", func() {
			It("should handle startup for new sessions without error", func() {
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

			It("should log event to JSONL file", func() {
				hookInput := map[string]string{
					"session_id": "log-test-uuid",
					"source":     "startup",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("sessionstart", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				logFile := filepath.Join(notifyLogDir, "log-test-uuid.events.jsonl")
				Expect(logFile).To(BeAnExistingFile())

				content, err := os.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("log-test-uuid"))
				Expect(string(content)).To(ContainSubstring("startup"))
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

	Describe("hook notify", func() {
		It("should exit without error on valid JSON input", func() {
			hookInput := map[string]string{
				"session_id": "test-notify-uuid",
			}
			inputJSON, err := json.Marshal(hookInput)
			Expect(err).NotTo(HaveOccurred())

			err = executeHookWithInput("notify", inputJSON)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should log event to JSONL file", func() {
			hookInput := map[string]string{
				"session_id":      "abc",
				"hook_event_name": "Stop",
			}
			inputJSON, err := json.Marshal(hookInput)
			Expect(err).NotTo(HaveOccurred())

			err = executeHookWithInput("notify", inputJSON)
			Expect(err).NotTo(HaveOccurred())

			logFile := filepath.Join(notifyLogDir, "abc.events.jsonl")
			Expect(logFile).To(BeAnExistingFile())

			content, err := os.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("abc"))
			Expect(string(content)).To(ContainSubstring("Stop"))
		})

		It("should handle invalid JSON gracefully", func() {
			err := executeHookWithInput("notify", []byte("not json"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse"))
		})

		It("should handle missing session_id gracefully", func() {
			hookInput := map[string]string{
				"hook_event_name": "Stop",
			}
			inputJSON, err := json.Marshal(hookInput)
			Expect(err).NotTo(HaveOccurred())

			err = executeHookWithInput("notify", inputJSON)
			Expect(err).NotTo(HaveOccurred())

			// No log file should be created (empty session_id)
			_, err = os.Stat(notifyLogDir)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("Zellij tab renaming", func() {
			var (
				fakeRenamer     *testFakeTabRenamer
				originalRenamer notify.TabRenamer
			)

			BeforeEach(func() {
				fakeRenamer = &testFakeTabRenamer{}
				originalRenamer = cmd.NotifyTabRenamer
				cmd.NotifyTabRenamer = fakeRenamer
			})

			AfterEach(func() {
				cmd.NotifyTabRenamer = originalRenamer
				_ = os.Unsetenv("ZELLIJ")
				_ = os.Unsetenv("CLOTILDE_SESSION_NAME")
			})

			It("should not rename tab when ZELLIJ is not set", func() {
				_ = os.Unsetenv("ZELLIJ")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "my-session")

				hookInput := map[string]interface{}{
					"session_id":      "zellij-test-uuid",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(BeEmpty())
			})

			It("should rename tab on Stop event with session name from env var", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "my-feature")

				hookInput := map[string]interface{}{
					"session_id":      "stop-test-uuid",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(Equal([]string{"\u2705 my-feature"}))
			})

			It("should rename tab on Notification with permission_prompt", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "auth-work")

				hookInput := map[string]interface{}{
					"session_id":        "notif-test-uuid",
					"hook_event_name":   "Notification",
					"notification_type": "permission_prompt",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(Equal([]string{"\u26a0\ufe0f auth-work"}))
			})

			It("should rename tab on PreToolUse with tool-specific emoji", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "debug-session")

				hookInput := map[string]interface{}{
					"session_id":      "tool-test-uuid",
					"hook_event_name": "PreToolUse",
					"tool_name":       "Bash",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(Equal([]string{"\u26a1 debug-session"}))
			})

			It("should not rename tab when session name cannot be resolved", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Unsetenv("CLOTILDE_SESSION_NAME")

				hookInput := map[string]interface{}{
					"session_id":      "unknown-session-uuid",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(BeEmpty())
			})

			It("should not rename tab on SessionEnd (empty emoji)", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "ending-session")

				hookInput := map[string]interface{}{
					"session_id":      "end-test-uuid",
					"hook_event_name": "SessionEnd",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(BeEmpty())
			})

			It("should resolve session name from env file fallback", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Unsetenv("CLOTILDE_SESSION_NAME")

				// Write session name to a fake CLAUDE_ENV_FILE
				envFile := filepath.Join(tempDir, "claude-env")
				err := os.WriteFile(envFile, []byte("CLOTILDE_SESSION=env-file-session\n"), 0o644)
				Expect(err).NotTo(HaveOccurred())
				_ = os.Setenv("CLAUDE_ENV_FILE", envFile)
				defer func() { _ = os.Unsetenv("CLAUDE_ENV_FILE") }()

				hookInput := map[string]interface{}{
					"session_id":      "env-file-uuid",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(Equal([]string{"\u2705 env-file-session"}))
			})

			It("should resolve session name via reverse UUID lookup", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Unsetenv("CLOTILDE_SESSION_NAME")
				_ = os.Unsetenv("CLAUDE_ENV_FILE")

				// Create a session with known UUID
				sess := session.NewSession("uuid-lookup-session", "lookup-uuid-123")
				err := store.Create(sess)
				Expect(err).NotTo(HaveOccurred())

				hookInput := map[string]interface{}{
					"session_id":      "lookup-uuid-123",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeRenamer.calls).To(Equal([]string{"\u2705 uuid-lookup-session"}))
			})

			It("should still log events even when Zellij rename happens", func() {
				_ = os.Setenv("ZELLIJ", "0")
				_ = os.Setenv("CLOTILDE_SESSION_NAME", "logged-session")

				hookInput := map[string]interface{}{
					"session_id":      "log-and-rename-uuid",
					"hook_event_name": "Stop",
				}
				inputJSON, err := json.Marshal(hookInput)
				Expect(err).NotTo(HaveOccurred())

				err = executeHookWithInput("notify", inputJSON)
				Expect(err).NotTo(HaveOccurred())

				// Verify rename happened
				Expect(fakeRenamer.calls).To(HaveLen(1))

				// Verify logging happened
				logFile := filepath.Join(notifyLogDir, "log-and-rename-uuid.events.jsonl")
				Expect(logFile).To(BeAnExistingFile())
			})
		})
	})
})
