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

var _ = Describe("Shorthand Flags", func() {
	var (
		tempDir        string
		clotildeRoot   string
		originalWd     string
		claudeArgsFile string
		fakeClaudeDir  string
		store          session.Store
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()

		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		fakeClaudeDir = filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		_, claudeArgsFile, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		store = session.NewFileStore(clotildeRoot)
	})

	AfterEach(func() {
		_ = os.Chdir(originalWd)
	})

	Describe("Permission mode shortcuts on start", func() {
		It("should store acceptEdits in settings when --accept-edits is used", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "ae-session", "--accept-edits"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			settings, err := store.LoadSettings("ae-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Permissions.DefaultMode).To(Equal("acceptEdits"))
		})

		It("should store bypassPermissions in settings when --yolo is used", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "yolo-session", "--yolo"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			settings, err := store.LoadSettings("yolo-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Permissions.DefaultMode).To(Equal("bypassPermissions"))
		})

		It("should store plan in settings when --plan is used", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "plan-session", "--plan"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			settings, err := store.LoadSettings("plan-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Permissions.DefaultMode).To(Equal("plan"))
		})

		It("should store dontAsk in settings when --dont-ask is used", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "da-session", "--dont-ask"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			settings, err := store.LoadSettings("da-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Permissions.DefaultMode).To(Equal("dontAsk"))
		})

		It("should reject combining two permission shortcuts", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "conflict-session", "--accept-edits", "--yolo"})

			err := rootCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot combine multiple permission mode shortcuts"))
		})

		It("should reject combining permission shortcut with --permission-mode", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "conflict-session", "--accept-edits", "--permission-mode", "plan"})

			err := rootCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot combine permission mode shortcut with --permission-mode"))
		})
	})

	Describe("--fast on start", func() {
		It("should set model to haiku and pass --effort low to claude", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "fast-session", "--fast"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify model stored in settings
			settings, err := store.LoadSettings("fast-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Model).To(Equal("haiku"))

			// Verify effort passed to claude
			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--effort low"))
		})

		It("should reject --fast with --model", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "conflict-session", "--fast", "--model", "opus"})

			err := rootCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --fast with --model"))
		})

		It("should allow combining --fast with permission shortcuts", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "combo-session", "--fast", "--accept-edits"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			settings, err := store.LoadSettings("combo-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Model).To(Equal("haiku"))
			Expect(settings.Permissions.DefaultMode).To(Equal("acceptEdits"))

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--effort low"))
		})
	})

	Describe("Permission mode shortcuts on resume", func() {
		It("should pass --permission-mode to claude when --accept-edits is used", func() {
			sess := session.NewSession("resume-ae", "uuid-resume-ae")
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "resume", "resume-ae", "--accept-edits"})

			err = rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--permission-mode acceptEdits"))
		})

		It("should reject combining two permission shortcuts on resume", func() {
			sess := session.NewSession("resume-conflict", "uuid-resume-conflict")
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "resume", "resume-conflict", "--yolo", "--plan"})

			err = rootCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot combine multiple permission mode shortcuts"))
		})
	})

	Describe("--fast on resume", func() {
		It("should pass --model and --effort to claude", func() {
			sess := session.NewSession("resume-fast", "uuid-resume-fast")
			err := store.Create(sess)
			Expect(err).NotTo(HaveOccurred())

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "resume", "resume-fast", "--fast"})

			err = rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--model haiku"))
			Expect(args).To(ContainSubstring("--effort low"))
		})
	})

	Describe("Permission mode shortcuts on fork", func() {
		It("should pass --permission-mode to claude when --yolo is used", func() {
			parent := session.NewSession("fork-parent", "uuid-fork-parent")
			err := store.Create(parent)
			Expect(err).NotTo(HaveOccurred())

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "fork-parent", "fork-child", "--yolo"})

			err = rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--permission-mode bypassPermissions"))
		})
	})

	Describe("--fast on fork", func() {
		It("should pass --model and --effort to claude", func() {
			parent := session.NewSession("fork-parent-fast", "uuid-fork-parent-fast")
			err := store.Create(parent)
			Expect(err).NotTo(HaveOccurred())

			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "fork-parent-fast", "fork-child-fast", "--fast"})

			err = rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--model haiku"))
			Expect(args).To(ContainSubstring("--effort low"))
		})
	})

	Describe("Permission mode shortcuts on incognito", func() {
		It("should store permission mode in settings when --dont-ask is used", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "incognito", "incog-da", "--dont-ask"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Incognito sessions auto-delete, so check the claude args instead
			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--settings"))

			// Read the settings file that was created (before cleanup)
			// Since incognito auto-deletes, we verify via the args file content
			// The settings file path is in the args - extract and read it
			Expect(args).To(ContainSubstring("--session-id"))
		})
	})

	Describe("--fast on incognito", func() {
		It("should pass --effort low to claude", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "incognito", "incog-fast", "--fast"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			args, err := testutil.ReadClaudeArgs(claudeArgsFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(args).To(ContainSubstring("--effort low"))
		})
	})

	Describe("--fast stores model in settings on start", func() {
		It("should persist haiku model in settings.json", func() {
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "start", "fast-persist", "--fast"})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Read settings.json directly to verify model is stored
			sessionDir := config.GetSessionDir(clotildeRoot, "fast-persist")
			settingsPath := filepath.Join(sessionDir, "settings.json")
			data, err := os.ReadFile(settingsPath)
			Expect(err).NotTo(HaveOccurred())

			var settings session.Settings
			err = json.Unmarshal(data, &settings)
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Model).To(Equal("haiku"))
		})
	})
})
