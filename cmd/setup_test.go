package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
	"github.com/fgrehm/clotilde/internal/testutil"
)

var _ = Describe("Setup Command", func() {
	var (
		tempDir      string
		originalWd   string
		originalHome string
		originalPATH string
		fakeHome     string
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()

		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Setup fake claude binary
		fakeClaudeDir := filepath.Join(tempDir, "bin")
		err = os.Mkdir(fakeClaudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = testutil.CreateFakeClaude(fakeClaudeDir)
		Expect(err).NotTo(HaveOccurred())

		originalPATH = os.Getenv("PATH")
		_ = os.Setenv("PATH", fakeClaudeDir+":"+originalPATH)

		// Use a fake home directory
		fakeHome = filepath.Join(tempDir, "home")
		err = os.MkdirAll(fakeHome, 0o755)
		Expect(err).NotTo(HaveOccurred())

		originalHome = os.Getenv("HOME")
		_ = os.Setenv("HOME", fakeHome)
	})

	AfterEach(func() {
		_ = os.Setenv("PATH", originalPATH)
		_ = os.Setenv("HOME", originalHome)
		_ = os.Chdir(originalWd)
	})

	It("should create ~/.claude/settings.json with all hook events", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetArgs([]string{"setup"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify ~/.claude/settings.json exists
		settingsPath := filepath.Join(fakeHome, ".claude", "settings.json")
		content, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		Expect(settings).To(HaveKey("hooks"))
		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("SessionStart"))

		// Only SessionStart should be present (GenerateHookConfig doesn't set the others)
		Expect(hooks).NotTo(HaveKey("Stop"))
		Expect(hooks).NotTo(HaveKey("Notification"))
		Expect(hooks).NotTo(HaveKey("PreToolUse"))
		Expect(hooks).NotTo(HaveKey("PostToolUse"))
		Expect(hooks).NotTo(HaveKey("SessionEnd"))

		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1))
	})

	It("should create ~/.claude/settings.local.json with --local flag", func() {
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetArgs([]string{"setup", "--local"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify ~/.claude/settings.local.json exists
		settingsPath := filepath.Join(fakeHome, ".claude", "settings.local.json")
		_, err = os.Stat(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		// settings.json should NOT exist
		globalPath := filepath.Join(fakeHome, ".claude", "settings.json")
		_, err = os.Stat(globalPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("should be idempotent", func() {
		rootCmd1 := cmd.NewRootCmd()
		rootCmd1.SetArgs([]string{"setup"})
		err := rootCmd1.Execute()
		Expect(err).NotTo(HaveOccurred())

		rootCmd2 := cmd.NewRootCmd()
		rootCmd2.SetArgs([]string{"setup"})
		err = rootCmd2.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Verify still has exactly one SessionStart hook entry
		settingsPath := filepath.Join(fakeHome, ".claude", "settings.json")
		content, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		hooks := settings["hooks"].(map[string]interface{})
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1))
	})

	It("should merge with existing settings", func() {
		// Create existing ~/.claude/settings.json with a user-defined hook
		claudeDir := filepath.Join(fakeHome, ".claude")
		err := os.MkdirAll(claudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		existingSettings := map[string]interface{}{
			"model": "opus",
			"hooks": map[string]interface{}{
				"UserPromptSubmit": []interface{}{"echo existing"},
			},
		}
		settingsPath := filepath.Join(claudeDir, "settings.json")
		content, err := json.Marshal(existingSettings)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(settingsPath, content, 0o644)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetArgs([]string{"setup"})
		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Read back and verify merge
		content, err = os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		// Original settings preserved
		Expect(settings["model"]).To(Equal("opus"))

		// User hook preserved alongside clotilde hooks
		hooks := settings["hooks"].(map[string]interface{})
		Expect(hooks).To(HaveKey("UserPromptSubmit"))
		Expect(hooks).To(HaveKey("SessionStart"))
	})

	It("should not overwrite existing hooks for types not in HookConfig", func() {
		// Pre-populate settings with user-defined Stop and SessionEnd hooks
		claudeDir := filepath.Join(fakeHome, ".claude")
		err := os.MkdirAll(claudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		existingSettings := map[string]interface{}{
			"hooks": map[string]interface{}{
				"Stop":       []interface{}{"echo my-stop-hook"},
				"SessionEnd": []interface{}{"echo my-sessionend-hook"},
			},
		}
		settingsPath := filepath.Join(claudeDir, "settings.json")
		content, err := json.Marshal(existingSettings)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(settingsPath, content, 0o644)
		Expect(err).NotTo(HaveOccurred())

		rootCmd := cmd.NewRootCmd()
		rootCmd.SetArgs([]string{"setup"})
		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Read back and verify
		content, err = os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		hooks := settings["hooks"].(map[string]interface{})

		// Clotilde's SessionStart hook should be present
		Expect(hooks).To(HaveKey("SessionStart"))

		// User-defined hooks should be preserved (not overwritten with null)
		Expect(hooks["Stop"]).To(Equal([]interface{}{"echo my-stop-hook"}))
		Expect(hooks["SessionEnd"]).To(Equal([]interface{}{"echo my-sessionend-hook"}))

		// Hook types not set by GenerateHookConfig should not appear as null keys
		Expect(hooks).NotTo(HaveKey("Notification"))
		Expect(hooks).NotTo(HaveKey("PreToolUse"))
		Expect(hooks).NotTo(HaveKey("PostToolUse"))
	})
})
