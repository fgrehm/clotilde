package claude_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

var _ = Describe("Detect", func() {
	Describe("IsInstalled", func() {
		It("should check if claude is in PATH", func() {
			// Note: This test depends on whether claude is actually installed
			// We can't easily mock exec.LookPath, so we just verify the function exists
			// and returns an error or nil based on actual installation
			err := claude.IsInstalled()

			// Either it's installed (err == nil) or not installed (err != nil with helpful message)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("claude CLI not found"))
				Expect(err.Error()).To(ContainSubstring("https://code.claude.com"))
			}
		})
	})

	Describe("GetVersion", func() {
		It("should get claude version if installed", func() {
			// Skip this test if claude is not installed
			if claude.IsInstalled() != nil {
				Skip("claude CLI not installed")
			}

			version, err := claude.GetVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).NotTo(BeEmpty())
		})
	})
})
