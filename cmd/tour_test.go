package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/cmd"
)

var _ = Describe("Tour Command", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	Describe("tour list", func() {
		It("shows tours from .tours/ directory", func() {
			toursDir := filepath.Join(tempDir, ".tours")
			Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())

			tourJSON := `{"title": "Architecture Overview", "steps": [{"file": "main.go", "line": 1, "description": "Entry point"}]}`
			Expect(os.WriteFile(filepath.Join(toursDir, "overview.tour"), []byte(tourJSON), 0o644)).To(Succeed())

			var buf bytes.Buffer
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			rootCmd.SetArgs([]string{"tour", "list", "--dir", tempDir})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			Expect(output).To(ContainSubstring("Tours (1)"))
			Expect(output).To(ContainSubstring("overview"))
			Expect(output).To(ContainSubstring("Architecture Overview"))
			Expect(output).To(ContainSubstring("1 steps"))
		})

		It("shows 'no tours found' when directory is empty", func() {
			var buf bytes.Buffer
			rootCmd := cmd.NewRootCmd()
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			rootCmd.SetArgs([]string{"tour", "list", "--dir", tempDir})

			err := rootCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			Expect(buf.String()).To(ContainSubstring("No tours found"))
		})
	})
})
