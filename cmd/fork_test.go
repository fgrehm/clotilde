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
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("Fork Command", func() {
	var (
		tempDir        string
		clotildeRoot   string
		originalWd     string
		claudeArgsFile string
		fakeClaudeDir  string
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
		fakeClaudeDir = filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		_, claudeArgsFile, err = testutil.CreateFakeClaude(fakeClaudeDir)
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

	It("should create fork with inherited settings", func() {
		// Create parent session with settings
		parent := session.NewSession("parent", "uuid-parent-123")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		settings := &session.Settings{Model: "opus"}
		err = store.SaveSettings("parent", settings)
		Expect(err).NotTo(HaveOccurred())

		// Execute fork command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "parent", "child"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify fork was created
		fork, err := store.Get("child")
		Expect(err).NotTo(HaveOccurred())
		Expect(fork.Metadata.IsForkedSession).To(BeTrue())
		Expect(fork.Metadata.ParentSession).To(Equal("parent"))

		// Verify settings were copied
		forkSettings, err := store.LoadSettings("child")
		Expect(err).NotTo(HaveOccurred())
		Expect(forkSettings.Model).To(Equal("opus"))

		// Verify claude was invoked with --fork-session
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--fork-session"))
		Expect(args).To(ContainSubstring("--resume"))
		Expect(args).To(ContainSubstring("uuid-parent-123"))
	})

	It("should inherit system prompt from parent", func() {
		// Create parent with system prompt
		parent := session.NewSession("parent-prompt", "uuid-parent-prompt")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		err = store.SaveSystemPrompt("parent-prompt", "You are a helpful assistant")
		Expect(err).NotTo(HaveOccurred())

		// Execute fork command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "parent-prompt", "child-prompt"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify system prompt was copied
		childPrompt, err := store.LoadSystemPrompt("child-prompt")
		Expect(err).NotTo(HaveOccurred())
		Expect(childPrompt).To(Equal("You are a helpful assistant"))

		// Verify claude was invoked with system prompt file
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--append-system-prompt-file"))
	})

	It("should inherit system prompt mode from parent", func() {
		// Create parent with replace mode
		parent := session.NewSession("parent-replace", "uuid-parent-replace")
		parent.Metadata.SystemPromptMode = "replace"
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		err = store.SaveSystemPrompt("parent-replace", "You are a code reviewer")
		Expect(err).NotTo(HaveOccurred())

		// Execute fork command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "parent-replace", "child-replace"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify fork inherited replace mode
		fork, err := store.Get("child-replace")
		Expect(err).NotTo(HaveOccurred())
		Expect(fork.Metadata.SystemPromptMode).To(Equal("replace"))

		// Verify claude was invoked with --system-prompt-file (not --append-system-prompt-file)
		args, err := testutil.ReadClaudeArgs(claudeArgsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(ContainSubstring("--system-prompt-file"))
		Expect(args).NotTo(ContainSubstring("--append-system-prompt-file"))
	})

	It("should reject fork of non-existent parent", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "does-not-exist", "child"})

		err := rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("should reject duplicate fork name", func() {
		// Create parent
		parent := session.NewSession("parent", "uuid-parent")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		// Create another session that would conflict
		existing := session.NewSession("existing", "uuid-existing")
		err = store.Create(existing)
		Expect(err).NotTo(HaveOccurred())

		// Try to fork with existing name
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "parent", "existing"})

		err = rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("already exists"))
	})

	It("should reject invalid fork name", func() {
		// Create parent
		parent := session.NewSession("parent", "uuid-parent")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		// Try to fork with invalid name
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "parent", "INVALID-NAME"})

		err = rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid session name"))
	})

	It("should create fork even when parent has no settings", func() {
		// Create minimal parent
		parent := session.NewSession("minimal-parent", "uuid-minimal")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		// Don't add settings/prompts to parent

		// Execute fork command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "minimal-parent", "minimal-child"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify fork was created
		fork, err := store.Get("minimal-child")
		Expect(err).NotTo(HaveOccurred())
		Expect(fork.Metadata.IsForkedSession).To(BeTrue())

		// Verify no settings file in fork (since parent had none)
		forkDir := config.GetSessionDir(clotildeRoot, "minimal-child")
		settingsPath := filepath.Join(forkDir, "settings.json")
		Expect(util.FileExists(settingsPath)).To(BeFalse())
	})

	It("should reject forking FROM incognito session", func() {
		// Create incognito parent
		incognitoParent := session.NewIncognitoSession("incognito-parent", "uuid-incognito")
		err := store.Create(incognitoParent)
		Expect(err).NotTo(HaveOccurred())

		// Try to fork from incognito session
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "incognito-parent", "child"})

		err = rootCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot fork from incognito session"))
		Expect(err.Error()).To(ContainSubstring("auto-delete"))
	})

	It("should allow forking TO incognito session with --incognito flag", func() {
		// Create regular parent
		parent := session.NewSession("regular-parent", "uuid-regular")
		err := store.Create(parent)
		Expect(err).NotTo(HaveOccurred())

		settings := &session.Settings{Model: "sonnet"}
		err = store.SaveSettings("regular-parent", settings)
		Expect(err).NotTo(HaveOccurred())

		// Execute fork command with --incognito flag
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"--claude-bin", filepath.Join(fakeClaudeDir, "claude"), "fork", "regular-parent", "incognito-fork", "--incognito"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify incognito fork was auto-deleted after Claude exited
		Expect(store.Exists("incognito-fork")).To(BeFalse())
	})
})
