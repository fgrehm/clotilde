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

		// Only SessionStart should be present (GenerateHookConfig doesn't set the others)
		Expect(hooks).NotTo(HaveKey("Stop"))
		Expect(hooks).NotTo(HaveKey("Notification"))
		Expect(hooks).NotTo(HaveKey("PreToolUse"))
		Expect(hooks).NotTo(HaveKey("PostToolUse"))
		Expect(hooks).NotTo(HaveKey("SessionEnd"))

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

		// Verify SessionStart has unified hook
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(1)) // Now using unified hook
	})

	It("should preserve third-party hooks when merging", func() {
		// Create .claude directory with settings that have third-party hooks
		// alongside clotilde hooks (simulates manual merge or other tools)
		claudeDir := filepath.Join(tempDir, ".claude")
		err := os.Mkdir(claudeDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		existingSettings := map[string]interface{}{
			"hooks": map[string]interface{}{
				"SessionStart": []interface{}{
					map[string]interface{}{
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "/old/path/clotilde hook sessionstart",
							},
							map[string]interface{}{
								"type":    "command",
								"command": "/some/other/tool.sh",
								"timeout": 5,
							},
						},
					},
				},
				"SessionEnd": []interface{}{
					map[string]interface{}{
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "/some/other/tool.sh",
							},
						},
					},
				},
				"Stop": []interface{}{
					map[string]interface{}{
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "/some/other/tool.sh",
							},
						},
					},
				},
			},
		}
		settingsPath := filepath.Join(claudeDir, "settings.local.json")
		content, err := json.Marshal(existingSettings)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(settingsPath, content, 0o644)
		Expect(err).NotTo(HaveOccurred())

		// Also need clotilde structure for init
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Run init (which calls mergeHooksIntoSettings)
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{"init"})
		err = rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Read resulting settings
		content, err = os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]interface{}
		err = json.Unmarshal(content, &settings)
		Expect(err).NotTo(HaveOccurred())

		hooks := settings["hooks"].(map[string]interface{})

		// SessionStart should have 2 matchers: third-party (preserved) + clotilde (new)
		sessionStart := hooks["SessionStart"].([]interface{})
		Expect(sessionStart).To(HaveLen(2))

		// First matcher: the third-party hook only (old clotilde hook stripped)
		firstMatcher := sessionStart[0].(map[string]interface{})
		firstHooks := firstMatcher["hooks"].([]interface{})
		Expect(firstHooks).To(HaveLen(1))
		firstCmd := firstHooks[0].(map[string]interface{})["command"]
		Expect(firstCmd).To(Equal("/some/other/tool.sh"))

		// Second matcher: new clotilde hook
		secondMatcher := sessionStart[1].(map[string]interface{})
		secondHooks := secondMatcher["hooks"].([]interface{})
		Expect(secondHooks).To(HaveLen(1))
		secondCmd := secondHooks[0].(map[string]interface{})["command"].(string)
		Expect(secondCmd).To(ContainSubstring("hook sessionstart"))

		// SessionEnd: third-party preserved, no clotilde added (init doesn't enable stats)
		sessionEnd := hooks["SessionEnd"].([]interface{})
		Expect(sessionEnd).To(HaveLen(1))
		endHooks := sessionEnd[0].(map[string]interface{})["hooks"].([]interface{})
		endCmd := endHooks[0].(map[string]interface{})["command"]
		Expect(endCmd).To(Equal("/some/other/tool.sh"))

		// Stop: untouched (clotilde doesn't generate Stop hooks)
		stop := hooks["Stop"].([]interface{})
		Expect(stop).To(HaveLen(1))
	})

	It("should allow re-initialization to update hooks only", func() {
		// Initialize first time
		rootCmd1 := cmd.NewRootCmd()
		rootCmd1.SetArgs([]string{"init"})
		err := rootCmd1.Execute()
		Expect(err).NotTo(HaveOccurred())

		// Try to initialize again - should succeed and update hooks
		rootCmd2 := cmd.NewRootCmd()
		rootCmd2.SetArgs([]string{"init"})
		err = rootCmd2.Execute()
		Expect(err).NotTo(HaveOccurred())

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
