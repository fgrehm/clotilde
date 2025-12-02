package claude_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

var _ = Describe("Hooks", func() {
	Describe("GenerateHookConfig", func() {
		It("should generate hook configuration with binary path", func() {
			binaryPath := "/usr/local/bin/clotilde"
			config := claude.GenerateHookConfig(binaryPath)

			// Verify SessionStart structure has unified hook (no matcher)
			Expect(config.SessionStart).To(HaveLen(1))

			// Verify unified hook has no matcher field
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
	})

	Describe("HookConfigString", func() {
		It("should format hook config as JSON string", func() {
			config := claude.GenerateHookConfig("/usr/local/bin/clotilde")

			str := claude.HookConfigString(config)
			Expect(str).To(ContainSubstring(`"hooks":`))
			Expect(str).To(ContainSubstring(`"SessionStart"`))
			Expect(str).To(ContainSubstring(`"type": "command"`))
			Expect(str).To(ContainSubstring("/usr/local/bin/clotilde hook sessionstart"))
			// Unified hook should NOT have matcher fields
			Expect(str).NotTo(ContainSubstring(`"matcher"`))
		})
	})
})
