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
			config := claude.GenerateHookConfig(binaryPath)

			Expect(config.SessionStart).To(HaveLen(1))
			Expect(config.SessionStart[0].Matcher).To(BeEmpty())
			Expect(config.SessionStart[0].Hooks).To(HaveLen(1))
			Expect(config.SessionStart[0].Hooks[0].Type).To(Equal("command"))
			Expect(config.SessionStart[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook sessionstart"))
		})

		It("should work with relative paths", func() {
			binaryPath := "./clotilde"
			config := claude.GenerateHookConfig(binaryPath)

			Expect(config.SessionStart[0].Hooks[0].Command).To(Equal("./clotilde hook sessionstart"))
		})

		It("should include Stop hook with notify command", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			Expect(config.Stop).To(HaveLen(1))
			Expect(config.Stop[0].Matcher).To(BeEmpty())
			Expect(config.Stop[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook notify"))
		})

		It("should include Notification hook with notify command", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			Expect(config.Notification).To(HaveLen(1))
			Expect(config.Notification[0].Matcher).To(BeEmpty())
			Expect(config.Notification[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook notify"))
		})

		It("should include PreToolUse hook with matcher '.*'", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			Expect(config.PreToolUse).To(HaveLen(1))
			Expect(config.PreToolUse[0].Matcher).To(Equal(".*"))
			Expect(config.PreToolUse[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook notify"))
		})

		It("should include PostToolUse hook with matcher '.*'", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			Expect(config.PostToolUse).To(HaveLen(1))
			Expect(config.PostToolUse[0].Matcher).To(Equal(".*"))
			Expect(config.PostToolUse[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook notify"))
		})

		It("should include SessionEnd hook with notify command", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			Expect(config.SessionEnd).To(HaveLen(1))
			Expect(config.SessionEnd[0].Matcher).To(BeEmpty())
			Expect(config.SessionEnd[0].Hooks[0].Command).To(Equal("/usr/local/bin/clotilde hook notify"))
		})
	})

	Describe("HookConfigString", func() {
		It("should format hook config as JSON string", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			str := claude.HookConfigString(config)
			Expect(str).To(ContainSubstring(`"hooks":`))
			Expect(str).To(ContainSubstring(`"SessionStart"`))
			Expect(str).To(ContainSubstring(`"Stop"`))
			Expect(str).To(ContainSubstring(`"Notification"`))
			Expect(str).To(ContainSubstring(`"PreToolUse"`))
			Expect(str).To(ContainSubstring(`"PostToolUse"`))
			Expect(str).To(ContainSubstring(`"SessionEnd"`))
			Expect(str).To(ContainSubstring("hook sessionstart"))
			Expect(str).To(ContainSubstring("hook notify"))
		})
	})
})
