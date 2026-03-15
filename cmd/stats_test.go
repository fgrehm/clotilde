package cmd_test

import (
	"io"
	"os"
	"path/filepath"
	"time"

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

	It("should return error when no name and no --all", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"stats"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("session name required"))
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

		// Create a fake transcript in the temp dir (path is set explicitly, so
		// it doesn't need to follow Claude's ~/.claude/projects/... convention).
		transcriptPath := filepath.Join(tempDir, "uuid-transcript-123.jsonl")
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

		// Create an empty transcript in the temp dir (path is set explicitly).
		transcriptPath := filepath.Join(tempDir, "uuid-empty-trans.jsonl")
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

	It("should show tokens, models, and tool usage", func() {
		sess := session.NewSession("rich-stats", "uuid-rich-123")
		err := store.Create(sess)
		Expect(err).NotTo(HaveOccurred())

		transcriptPath := filepath.Join(tempDir, "uuid-rich-123.jsonl")
		transcriptData := `{"type":"progress","timestamp":"2025-02-17T20:35:00Z"}
{"type":"user","timestamp":"2025-02-17T20:35:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T20:35:15Z","message":{"model":"claude-sonnet-4-5-20250929","content":[{"type":"text","text":"Let me check."},{"type":"tool_use","name":"Read","id":"t1"}],"usage":{"input_tokens":1500,"output_tokens":200,"cache_read_input_tokens":3000}}}`

		err = os.WriteFile(transcriptPath, []byte(transcriptData), 0o644)
		Expect(err).NotTo(HaveOccurred())

		sess.Metadata.TranscriptPath = transcriptPath
		err = store.Update(sess)
		Expect(err).NotTo(HaveOccurred())

		output := captureOutput(func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"stats", "rich-stats"})
			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		Expect(output).To(ContainSubstring("Input tokens"))
		Expect(output).To(ContainSubstring("Output tokens"))
		Expect(output).To(ContainSubstring("Cache read"))
		Expect(output).To(ContainSubstring("Models"))
		Expect(output).To(ContainSubstring("sonnet"))
		Expect(output).To(ContainSubstring("Tool usage:"))
		Expect(output).To(ContainSubstring("Read"))
	})

	Context("--all flag", func() {
		var projectPath string

		BeforeEach(func() {
			// Isolate stats files from real system
			GinkgoT().Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "xdg-data"))
			projectPath = config.ProjectRoot(clotildeRoot)
		})

		It("should aggregate from JSONL stats files with per-session breakdown", func() {
			// Use minute offsets to avoid crossing day boundaries near midnight
			now := time.Now()
			err := claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionName:  "session-1",
				SessionID:    "uuid-all-1",
				ProjectPath:  projectPath,
				Turns:        5,
				ActiveTimeS:  300,
				TotalTimeS:   600,
				InputTokens:  1000,
				OutputTokens: 500,
				Models:       []string{"claude-sonnet-4-5-20250929"},
				ToolUses:     map[string]int{"Read": 3, "Bash": 2},
				EndedAt:      now.Add(-10 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			err = claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionName:  "session-2",
				SessionID:    "uuid-all-2",
				ProjectPath:  projectPath,
				Turns:        10,
				ActiveTimeS:  600,
				TotalTimeS:   1200,
				InputTokens:  2000,
				OutputTokens: 1000,
				Models:       []string{"claude-opus-4-6-20260301"},
				ToolUses:     map[string]int{"Edit": 5},
				EndedAt:      now.Add(-20 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("Aggregate stats (2 sessions, last 7 days)"))
			Expect(output).To(ContainSubstring("Turns         15"))
			Expect(output).To(ContainSubstring("Input tokens"))
			Expect(output).To(ContainSubstring("Models"))
			Expect(output).To(ContainSubstring("Tool usage:"))

			// Per-session breakdown (sorted by active time desc: session-2 first)
			Expect(output).To(ContainSubstring("Session"))
			Expect(output).To(ContainSubstring("session-2"))
			Expect(output).To(ContainSubstring("session-1"))
		})

		It("should not show breakdown for single session", func() {
			now := time.Now()
			err := claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionName: "only-one",
				SessionID:   "uuid-single",
				ProjectPath: projectPath,
				Turns:       5,
				ActiveTimeS: 300,
				EndedAt:     now.Add(-10 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("Aggregate stats (1 sessions"))
			Expect(output).NotTo(ContainSubstring("Session"))
		})

		It("should deduplicate by session_id keeping latest", func() {
			// Use minute offsets to avoid crossing day boundaries near midnight
			now := time.Now()
			err := claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionID:   "uuid-dup",
				ProjectPath: projectPath,
				Turns:       5,
				InputTokens: 1000,
				EndedAt:     now.Add(-20 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			err = claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionID:       "uuid-dup",
				ProjectPath:     projectPath,
				Turns:           12,
				PrevTurns:       5,
				InputTokens:     3000,
				PrevInputTokens: 1000,
				EndedAt:         now.Add(-10 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			// Should use delta: 12 - 5 = 7 turns from the latest record
			Expect(output).To(ContainSubstring("Aggregate stats (1 sessions"))
			Expect(output).To(ContainSubstring("Turns         7"))
		})

		It("should fall back to transcripts when no stats files exist", func() {
			// No JSONL files written; create sessions with transcripts
			sess1 := session.NewSession("session-1", "uuid-all-1")
			sess1.Metadata.LastAccessed = time.Now().Add(-1 * time.Hour)
			err := store.Create(sess1)
			Expect(err).NotTo(HaveOccurred())

			t1Path := filepath.Join(tempDir, "uuid-all-1.jsonl")
			err = os.WriteFile(t1Path, []byte(`{"type":"progress","timestamp":"2025-02-17T10:00:00Z"}
{"type":"user","timestamp":"2025-02-17T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T10:00:20Z","message":{"content":"hi","usage":{"input_tokens":100,"output_tokens":50}}}`), 0o644)
			Expect(err).NotTo(HaveOccurred())
			sess1.Metadata.TranscriptPath = t1Path
			err = store.Update(sess1)
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("Aggregate stats (1 sessions, last 7 days)"))
			Expect(output).To(ContainSubstring("from transcripts"))
			Expect(output).To(ContainSubstring("Turns         1"))
		})

		It("should show no activity when stats files exist but have zero turns", func() {
			now := time.Now()
			err := claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionID:   "uuid-zero",
				ProjectPath: projectPath,
				Turns:       0,
				EndedAt:     now.Add(-5 * time.Minute),
			})
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("No activity recorded"))
		})

		It("should exclude old sessions in transcript fallback", func() {
			// No JSONL files; session with old lastAccessed
			sess := session.NewSession("old-session", "uuid-old-1")
			sess.Metadata.LastAccessed = time.Now().Add(-10 * 24 * time.Hour)
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			t1Path := filepath.Join(tempDir, "uuid-old-1.jsonl")
			err = os.WriteFile(t1Path, []byte(`{"type":"progress","timestamp":"2025-01-01T10:00:00Z"}
{"type":"user","timestamp":"2025-01-01T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-01-01T10:00:20Z","message":{"content":"hi"}}`), 0o644)
			Expect(err).NotTo(HaveOccurred())
			sess.Metadata.TranscriptPath = t1Path
			err = store.Update(sess)
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "--all"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("No sessions active in the last 7 days"))
		})
	})
	Context("backfill subcommand", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "xdg-data"))
		})

		It("should write records for sessions with transcripts", func() {
			sess := session.NewSession("backfill-test", "uuid-bf-1")
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			transcriptPath := filepath.Join(tempDir, "uuid-bf-1.jsonl")
			err = os.WriteFile(transcriptPath, []byte(`{"type":"progress","timestamp":"2025-02-17T10:00:00Z"}
{"type":"user","timestamp":"2025-02-17T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T10:00:20Z","message":{"content":"hi","usage":{"input_tokens":500,"output_tokens":200}}}`), 0o644)
			Expect(err).NotTo(HaveOccurred())

			sess.Metadata.TranscriptPath = transcriptPath
			err = store.Update(sess)
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "backfill"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("backfill-test"))
			Expect(output).To(ContainSubstring("1 written"))
		})

		It("should skip sessions that already have records", func() {
			sess := session.NewSession("already-recorded", "uuid-bf-2")
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			transcriptPath := filepath.Join(tempDir, "uuid-bf-2.jsonl")
			err = os.WriteFile(transcriptPath, []byte(`{"type":"progress","timestamp":"2025-02-17T10:00:00Z"}
{"type":"user","timestamp":"2025-02-17T10:00:10Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-02-17T10:00:20Z","message":{"content":"hi"}}`), 0o644)
			Expect(err).NotTo(HaveOccurred())

			sess.Metadata.TranscriptPath = transcriptPath
			err = store.Update(sess)
			Expect(err).NotTo(HaveOccurred())

			// Pre-write a record
			err = claude.AppendStatsRecord(claude.SessionStatsRecord{
				SessionID: "uuid-bf-2",
				Turns:     1,
				EndedAt:   time.Now(),
			})
			Expect(err).NotTo(HaveOccurred())

			output := captureOutput(func() {
				rootCmd := cmd.NewRootCmd()
				rootCmd.SetOut(os.Stdout)
				rootCmd.SetErr(io.Discard)
				rootCmd.SetArgs([]string{"stats", "backfill"})
				err := rootCmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("0 written"))
			Expect(output).To(ContainSubstring("1 skipped"))
		})
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

	out, _ := io.ReadAll(r)
	return string(out)
}
