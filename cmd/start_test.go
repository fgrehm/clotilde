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

var _ = Describe("Start Command", func() {
	var (
		tempDir        string
		clotildeRoot   string
		originalWd     string
		claudeArgsFile string
		fakeClaudeDir  string
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
		fakeClaudeDir = filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		_, claudeArgsFile, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		Expect(err).NotTo(HaveOccurred())

		// Initialize clotilde
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)

		// Fake claude doesn't create transcripts, so pretend sessions are used
		// to avoid empty session cleanup in most tests
		claude.SessionUsedFunc = func(_ string, _ *session.Session) bool { return true }
	})

	AfterEach(func() {
		// Restore SessionUsedFunc
		claude.SessionUsedFunc = func(_ string, _ *session.Session) bool { return true }

		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	It("should create a session and invoke claude with correct arguments", func() {
		// Execute start command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "test-session", "--model", "sonnet"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify session was created
		sessionDir := config.GetSessionDir(clotildeRoot, "test-session")
		info, err := os.Stat(sessionDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		// Verify metadata.json exists
		metadataPath := filepath.Join(sessionDir, "metadata.json")
		_, err = os.Stat(metadataPath)
		Expect(err).NotTo(HaveOccurred())

		// Verify settings.json was created (because we passed --model)
		settingsPath := filepath.Join(sessionDir, "settings.json")
		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		// Verify claude was invoked with correct arguments
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--session-id"))
		Expect(args).To(ContainSubstring("--settings"))
		Expect(args).To(ContainSubstring(settingsPath))
	})

	It("should create session with empty settings file if model not specified", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "simple-session"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify session was created
		sessionDir := config.GetSessionDir(clotildeRoot, "simple-session")
		info, err := os.Stat(sessionDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		// Verify settings.json exists but is empty (no model)
		settingsPath := filepath.Join(sessionDir, "settings.json")
		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		// Verify claude was invoked WITH --settings (always pass settings file)
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--settings"))
		Expect(args).To(ContainSubstring(settingsPath))
	})

	It("should save system prompt when provided", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "prompt-session", "--append-system-prompt", "You are helpful"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify system-prompt.md was created
		sessionDir := config.GetSessionDir(clotildeRoot, "prompt-session")
		promptPath := filepath.Join(sessionDir, "system-prompt.md")
		_, err = os.Stat(promptPath)
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(promptPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("You are helpful"))

		// Verify claude was invoked with --append-system-prompt-file
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--append-system-prompt-file"))

		// Verify metadata has append mode
		store := session.NewFileStore(clotildeRoot)
		sess, err := store.Get("prompt-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(sess.Metadata.SystemPromptMode).To(Equal("append"))
	})

	It("should save system prompt with replace mode when --replace-system-prompt is used", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "replace-session", "--replace-system-prompt", "You are a code reviewer"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify system-prompt.md was created
		sessionDir := config.GetSessionDir(clotildeRoot, "replace-session")
		promptPath := filepath.Join(sessionDir, "system-prompt.md")
		_, err = os.Stat(promptPath)
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(promptPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("You are a code reviewer"))

		// Verify claude was invoked with --system-prompt-file (not --append-system-prompt-file)
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--system-prompt-file"))
		Expect(args).NotTo(ContainSubstring("--append-system-prompt-file"))

		// Verify metadata has replace mode
		store := session.NewFileStore(clotildeRoot)
		sess, err := store.Get("replace-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(sess.Metadata.SystemPromptMode).To(Equal("replace"))
	})

	It("should reject both append and replace system prompt flags", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "conflict-session", "--append-system-prompt", "append", "--replace-system-prompt", "replace"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot use both append and replace"))
	})

	It("should reject invalid session names", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "INVALID-NAME"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid session name"))
	})

	It("should suggest resuming when session already exists (non-TTY)", func() {
		// Create first session
		rootCmd1 := cmd.NewRootCmd()
		rootCmd1.SetOut(io.Discard)
		rootCmd1.SetErr(io.Discard)
		rootCmd1.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "duplicate"})
		err := rootCmd1.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Try to create again - should suggest resume
		rootCmd2 := cmd.NewRootCmd()
		rootCmd2.SetOut(io.Discard)
		rootCmd2.SetErr(io.Discard)
		rootCmd2.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "duplicate"})
		err = rootCmd2.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("already exists"))
		Expect(err.Error()).To(ContainSubstring("clotilde resume duplicate"))
	})

	It("should cleanup session when no messages were sent", func() {
		// Simulate Claude Code not creating a transcript (user exited without typing)
		claude.SessionUsedFunc = func(_ string, _ *session.Session) bool { return false }

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "empty-session"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify session was cleaned up
		store := session.NewFileStore(clotildeRoot)
		Expect(store.Exists("empty-session")).To(BeFalse())
	})

	It("should keep session when messages were sent", func() {
		// Simulate Claude Code creating a transcript (user sent messages)
		claude.SessionUsedFunc = func(_ string, _ *session.Session) bool { return true }

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "used-session"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify session still exists
		store := session.NewFileStore(clotildeRoot)
		Expect(store.Exists("used-session")).To(BeTrue())
	})

	It("should use profile model when specified", func() {
		// Set up global config with profiles
		cfg := &config.Config{
			Profiles: map[string]config.Profile{
				"quick": {
					Model: "haiku",
				},
			},
		}
		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "profile-session", "--profile", "quick"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify settings.json contains the model from the profile
		store := session.NewFileStore(clotildeRoot)
		settings, err := store.LoadSettings("profile-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Model).To(Equal("haiku"))
	})

	It("should override profile model with command-line flag", func() {
		// Set up global config with profiles
		cfg := &config.Config{
			Profiles: map[string]config.Profile{
				"quick": {
					Model: "haiku",
				},
			},
		}
		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "override-session", "--profile", "quick", "--model", "opus"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify settings.json contains the overridden model, not the profile model
		store := session.NewFileStore(clotildeRoot)
		settings, err := store.LoadSettings("override-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Model).To(Equal("opus"))
	})

	It("should use profile permissions when specified", func() {
		// Set up global config with profiles
		cfg := &config.Config{
			Profiles: map[string]config.Profile{
				"strict": {
					Permissions: &config.Permissions{
						Deny:        []string{"Bash", "Write"},
						DefaultMode: "ask",
					},
				},
			},
		}
		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "strict-session", "--profile", "strict"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify settings.json contains the permissions from the profile
		store := session.NewFileStore(clotildeRoot)
		settings, err := store.LoadSettings("strict-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Permissions.Deny).To(Equal([]string{"Bash", "Write"}))
		Expect(settings.Permissions.DefaultMode).To(Equal("ask"))
	})

	It("should error when profile not found", func() {
		cfg := &config.Config{
			Profiles: map[string]config.Profile{},
		}
		err := config.Save(clotildeRoot, cfg)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "bad-session", "--profile", "nonexistent"})

		err = rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("profile 'nonexistent' not found"))
	})
})
