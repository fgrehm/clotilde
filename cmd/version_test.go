package cmd_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
)

var _ = Describe("Version Command", func() {
	It("should display version information", func() {
		// Capture stdout
		var buf bytes.Buffer
		rootCmd := cmd.NewRootCmd()
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"version"})

		err := rootCmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		// Should show multi-line version info
		Expect(output).To(ContainSubstring("clotilde version"))
		Expect(output).To(ContainSubstring("commit:"))
		Expect(output).To(ContainSubstring("built:"))
		Expect(output).To(ContainSubstring("go:"))
	})
})
