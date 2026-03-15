package cmd_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/notify"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/testutil"
)

// readAllStatsRecords reads all stats records from all daily files in a stats directory.
func readAllStatsRecords(dataHome string) ([]claude.SessionStatsRecord, error) {
	statsDir := filepath.Join(dataHome, "clotilde", "stats")
	entries, err := os.ReadDir(statsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []claude.SessionStatsRecord
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		records, err := claude.ReadStatsFile(filepath.Join(statsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
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

		Context("crash recovery on resume", func() {
			var statsDir string

			BeforeEach(func() {
				statsDir = filepath.Join(tempDir, "stats-recovery")
				_ = os.Setenv("XDG_DATA_HOME", statsDir)
			})

			AfterEach(func() {
				_ = os.Unsetenv("XDG_DATA_HOME")
			})

			writeTranscriptForRecovery := func(path string) {
				dir := filepath.Dir(path)
				Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
				transcript := `{"type":"progress","timestamp":"2026-03-14T10:00:00Z"}
{"type":"user","timestamp":"2026-03-14T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2026-03-14T10:00:15Z","message":{"model":"claude-sonnet-4-5-20250929","content":"hi","usage":{"input_tokens":100,"output_tokens":50}}}
`
				Expect(os.WriteFile(path, []byte(transcript), 0o644)).To(Succeed())
			}

			It("should write recovery record when prior SessionEnd is missing", func() {
				// Create session with stale lastAccessed (> 30s ago)
				sess := session.NewSession("crash-test", "crash-uuid")
				sess.Metadata.LastAccessed = time.Now().Add(-5 * time.Minute)
				transcriptPath := filepath.Join(tempDir, "crash-transcript.jsonl")
				sess.Metadata.TranscriptPath = transcriptPath
				Expect(store.Create(sess)).To(Succeed())
				writeTranscriptForRecovery(transcriptPath)

				_ = os.Setenv("CLOTILDE_SESSION_NAME", "crash-test")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				hookInput := map[string]string{
					"session_id": "crash-uuid",
					"source":     "resume",
				}
				inputJSON, _ := json.Marshal(hookInput)
				Expect(executeHookWithInput("sessionstart", inputJSON)).To(Succeed())

				// Verify recovery record was written
				records, err := readAllStatsRecords(statsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(records).To(HaveLen(1))
				Expect(records[0].SessionName).To(Equal("crash-test"))
				Expect(records[0].Turns).To(Equal(1))
			})

			It("should skip recovery when prior SessionEnd record exists", func() {
				sess := session.NewSession("no-crash-test", "no-crash-uuid")
				sess.Metadata.LastAccessed = time.Now().Add(-5 * time.Minute)
				transcriptPath := filepath.Join(tempDir, "no-crash-transcript.jsonl")
				sess.Metadata.TranscriptPath = transcriptPath
				Expect(store.Create(sess)).To(Succeed())
				writeTranscriptForRecovery(transcriptPath)

				// Write a prior stats record so recovery is skipped
				priorRecord := claude.SessionStatsRecord{
					SessionID: "no-crash-uuid",
					Turns:     1,
					EndedAt:   time.Now().Add(-5 * time.Minute).UTC(),
				}
				Expect(claude.AppendStatsRecord(priorRecord)).To(Succeed())

				_ = os.Setenv("CLOTILDE_SESSION_NAME", "no-crash-test")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				hookInput := map[string]string{
					"session_id": "no-crash-uuid",
					"source":     "resume",
				}
				inputJSON, _ := json.Marshal(hookInput)
				Expect(executeHookWithInput("sessionstart", inputJSON)).To(Succeed())

				// Only the prior record should exist (no recovery record added)
				records, err := readAllStatsRecords(statsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(records).To(HaveLen(1))
			})

			It("should skip recovery for brand-new sessions", func() {
				// Session with very recent lastAccessed (< 30s)
				sess := session.NewSession("new-test", "new-uuid")
				// lastAccessed is Now() from NewSession, so < 30s
				transcriptPath := filepath.Join(tempDir, "new-transcript.jsonl")
				sess.Metadata.TranscriptPath = transcriptPath
				Expect(store.Create(sess)).To(Succeed())
				writeTranscriptForRecovery(transcriptPath)

				_ = os.Setenv("CLOTILDE_SESSION_NAME", "new-test")
				defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

				hookInput := map[string]string{
					"session_id": "new-uuid",
					"source":     "resume",
				}
				inputJSON, _ := json.Marshal(hookInput)
				Expect(executeHookWithInput("sessionstart", inputJSON)).To(Succeed())

				// No stats records should be written (fast-path skip)
				records, err := readAllStatsRecords(statsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(records).To(BeEmpty())
			})
		})
	})

	Describe("hook sessionend", func() {
		var statsDir string

		BeforeEach(func() {
			statsDir = filepath.Join(tempDir, "stats-data")
			_ = os.Setenv("XDG_DATA_HOME", statsDir)
		})

		AfterEach(func() {
			_ = os.Unsetenv("XDG_DATA_HOME")
		})

		writeTranscript := func(path string) {
			dir := filepath.Dir(path)
			Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
			transcript := `{"type":"progress","timestamp":"2026-03-15T10:00:00Z"}
{"type":"user","timestamp":"2026-03-15T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2026-03-15T10:00:15Z","message":{"model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":5,"cache_read_input_tokens":80}}}
`
			Expect(os.WriteFile(path, []byte(transcript), 0o644)).To(Succeed())
		}

		It("should write stats record from transcript", func() {
			// Create session and transcript
			sess := session.NewSession("stats-test", "stats-uuid-1")
			transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
			sess.Metadata.TranscriptPath = transcriptPath
			Expect(store.Create(sess)).To(Succeed())
			writeTranscript(transcriptPath)

			_ = os.Setenv("CLOTILDE_SESSION_NAME", "stats-test")
			defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

			hookInput := map[string]string{
				"session_id":      "stats-uuid-1",
				"transcript_path": transcriptPath,
			}
			inputJSON, _ := json.Marshal(hookInput)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())

			// Read the stats file
			records, err := readAllStatsRecords(statsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(1))
			Expect(records[0].SessionName).To(Equal("stats-test"))
			Expect(records[0].SessionID).To(Equal("stats-uuid-1"))
			Expect(records[0].Turns).To(Equal(1))
			Expect(records[0].InputTokens).To(Equal(100))
			Expect(records[0].OutputTokens).To(Equal(50))
			Expect(records[0].Models).To(ContainElement("claude-sonnet-4-5-20250929"))
		})

		It("should fall back to payload transcript_path when clotilde root not found", func() {
			nonClotildeDir := GinkgoT().TempDir()
			Expect(os.Chdir(nonClotildeDir)).To(Succeed())

			transcriptPath := filepath.Join(nonClotildeDir, "transcript.jsonl")
			writeTranscript(transcriptPath)

			hookInput := map[string]string{
				"session_id":      "fallback-uuid",
				"transcript_path": transcriptPath,
			}
			inputJSON, _ := json.Marshal(hookInput)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())

			records, err := readAllStatsRecords(statsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(1))
			Expect(records[0].SessionName).To(BeEmpty())
			Expect(records[0].ProjectPath).To(BeEmpty())
			Expect(records[0].Turns).To(Equal(1))
		})

		It("should exit 0 when transcript is unreadable", func() {
			_ = os.Setenv("CLOTILDE_SESSION_NAME", "nonexistent")
			defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

			hookInput := map[string]string{
				"session_id":      "bad-transcript-uuid",
				"transcript_path": "/nonexistent/transcript.jsonl",
			}
			inputJSON, _ := json.Marshal(hookInput)

			// Should not error (exit 0)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())
		})

		It("should populate prev_* fields from prior record", func() {
			sess := session.NewSession("prev-test", "prev-uuid")
			transcriptPath := filepath.Join(tempDir, "prev-transcript.jsonl")
			sess.Metadata.TranscriptPath = transcriptPath
			Expect(store.Create(sess)).To(Succeed())
			writeTranscript(transcriptPath)

			_ = os.Setenv("CLOTILDE_SESSION_NAME", "prev-test")
			defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

			// First invocation
			hookInput := map[string]string{
				"session_id":      "prev-uuid",
				"transcript_path": transcriptPath,
			}
			inputJSON, _ := json.Marshal(hookInput)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())

			// Reset double-execution guard for second invocation
			_ = os.Unsetenv("CLOTILDE_HOOK_EXECUTED")
			envFile := os.Getenv("CLAUDE_ENV_FILE")
			if envFile != "" {
				_ = os.Remove(envFile)
			}

			// Second invocation (same session)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())

			records, err := readAllStatsRecords(statsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(records)).To(BeNumerically(">=", 2))

			last := records[len(records)-1]
			Expect(last.PrevTurns).To(Equal(1))
			Expect(last.PrevInputTokens).To(Equal(100))
		})

		It("should prevent duplicate records via double-execution guard", func() {
			sess := session.NewSession("guard-test", "guard-uuid")
			transcriptPath := filepath.Join(tempDir, "guard-transcript.jsonl")
			sess.Metadata.TranscriptPath = transcriptPath
			Expect(store.Create(sess)).To(Succeed())
			writeTranscript(transcriptPath)

			_ = os.Setenv("CLOTILDE_SESSION_NAME", "guard-test")
			defer func() { _ = os.Unsetenv("CLOTILDE_SESSION_NAME") }()

			// Guard requires CLAUDE_ENV_FILE to persist the marker
			envFile := filepath.Join(tempDir, "env-guard")
			_ = os.Setenv("CLAUDE_ENV_FILE", envFile)
			defer func() { _ = os.Unsetenv("CLAUDE_ENV_FILE") }()

			hookInput := map[string]string{
				"session_id":      "guard-uuid",
				"transcript_path": transcriptPath,
			}
			inputJSON, _ := json.Marshal(hookInput)

			// First invocation
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())
			// Second invocation (guard blocks it)
			Expect(executeHookWithInput("sessionend", inputJSON)).To(Succeed())

			records, err := readAllStatsRecords(statsDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(1))
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
	})
})
