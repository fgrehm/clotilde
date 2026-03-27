package claude_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

var _ = Describe("Hooks", func() {
	Describe("GenerateHookConfig", func() {
		It("should generate SessionStart hook with sessionstart command", func() {
			binaryPath := "/usr/local/bin/clotilde"
			config := claude.GenerateHookConfig(binaryPath, claude.HookConfigOptions{})

			Expect(config.SessionStart).To(HaveLen(1))
			Expect(config.SessionStart[0].Matcher).To(BeEmpty())
			Expect(config.SessionStart[0].Hooks).To(HaveLen(1))
			Expect(config.SessionStart[0].Hooks[0].Type).To(Equal("command"))
			Expect(config.SessionStart[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook sessionstart"))
		})

		It("should work with relative paths", func() {
			binaryPath := "./clotilde"
			config := claude.GenerateHookConfig(binaryPath, claude.HookConfigOptions{})

			Expect(config.SessionStart[0].Hooks[0].Command).To(Equal("./clotilde hook sessionstart"))
		})

		It("should not register SessionEnd when stats disabled", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde", claude.HookConfigOptions{})

			Expect(config.Stop).To(BeEmpty())
			Expect(config.Notification).To(BeEmpty())
			Expect(config.PreToolUse).To(BeEmpty())
			Expect(config.PostToolUse).To(BeEmpty())
			Expect(config.SessionEnd).To(BeEmpty())
		})

		It("should include SessionEnd when stats enabled", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde", claude.HookConfigOptions{StatsEnabled: true})

			Expect(config.SessionEnd).To(HaveLen(1))
			Expect(config.SessionEnd[0].Matcher).To(BeEmpty())
			Expect(config.SessionEnd[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook sessionend"))
		})
	})

})
