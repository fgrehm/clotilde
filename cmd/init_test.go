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
	"github.com/fgrehm/clotilde/internal/testutil"
)

var _ = Describe("Init Command", func() {
	var (
		tempDir      string
		originalWd   string
		originalPATH string
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

		// Setup fake claude binary (for detection check)
		fakeClaudeDir := filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		// Add fake claude to PATH
		originalPATH = os.Getenv("PATH")
		_ = os.Setenv("PATH", fakeClaudeDir+":"+originalPATH)
	})

	AfterEach(func() {
		// Restore PATH
		_ = os.Setenv("PATH", originalPATH)

		// Restore working directory
		_ = os.Chdir(originalWd)
	})

	It("should create clotilde directory structure", func() {
		// Execute init command
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"init"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify .claude/clotilde directory exists
		clotildeDir := filepath.Join(tempDir, config.ClotildeDir)
		info, err := os.Stat(clotildeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		// Verify sessions directory exists
		sessionsDir := filepath.Join(clotildeDir, config.SessionsDir)
		info, err = os.Stat(sessionsDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		// Verify config.json exists
		configPath := filepath.Join(clotildeDir, config.ConfigFile)
		_, err = os.Stat(configPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should create .claude/settings.local.json with hooks by default", func() {
		// Execute init command (without --global)
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"init"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify .claude/settings.local.json exists (not settings.json)
		settingsPath := filepath.Join(tempDir, ".claude", "settings.local.json")
		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		// Read and verify it contains hooks
		content, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		Expect(settings).To(HaveKey("hooks"))
		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("SessionStart"))

		// Verify SessionStart structure (unified hook without matchers)
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1))

		// Verify unified hook structure (no matcher field)
		hook := sessionStart[0].(map[string]interface{})
		Expect(hook).NotTo(HaveKey("matcher")) // Unified hook has no matcher
	})

	It("should create .claude/settings.json with --global flag", func() {
		// Execute init command with --global
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"init", "--global"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify .claude/settings.json exists
		settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		// Read and verify it contains hooks
		content, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		Expect(settings).To(HaveKey("hooks"))
		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("SessionStart"))
	})

	It("should merge hooks into existing .claude/settings.json with --global", func() {
		// Create existing .claude directory with settings
		claudeDir := filepath.Join(tempDir, ".claude")
		err := os.Mkdir(claudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		existingSettings := map[string]interface{}{
			"model": "sonnet",
			"hooks": map[string]string{
				"ExistingHook": "echo 'existing'",
			},
		}
		settingsPath := filepath.Join(claudeDir, "settings.json")
		content, err := json.Marshal(existingSettings)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(settingsPath, content, 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Execute init command with --global
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"init", "--global"})

		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Read and verify merged settings
		content, err = os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		// Original settings should be preserved
		Expect(settings["model"]).To(Equal("sonnet"))

		// Hooks should be merged
		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("ExistingHook"))
		Expect(hooks).To(HaveKey("SessionStart"))

		// Verify SessionStart has both matchers
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1)) // Now using unified hook
	})

	It("should allow re-initialization to update hooks only", func() {
		// Initialize first time
		rootCmd1 := cmd.NewRootCmd()
		rootCmd1.SetArgs([]string{"init"})
		err := rootCmd1.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify initial structure
		clotildeDir := filepath.Join(tempDir, config.ClotildeDir)
		configPath := filepath.Join(clotildeDir, config.ConfigFile)
		initialConfigStat, err := os.Stat(configPath)
		Expect(err).NotTo(HaveOccurred())
		initialModTime := initialConfigStat.ModTime()

		// Try to initialize again - should succeed and update hooks
		rootCmd2 := cmd.NewRootCmd()
		rootCmd2.SetArgs([]string{"init"})
		err = rootCmd2.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify config.json was not modified (same mod time)
		newConfigStat, err := os.Stat(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(newConfigStat.ModTime()).To(Equal(initialModTime))

		// Verify hooks still exist in settings.local.json
		settingsPath := filepath.Join(tempDir, ".claude", "settings.local.json")
		content, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("SessionStart"))

		// Verify SessionStart has unified hook
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1))
	})
})
