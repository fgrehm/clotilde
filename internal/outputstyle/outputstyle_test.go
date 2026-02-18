package outputstyle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fgrehm/clotilde/internal/outputstyle"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOutputStyle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OutputStyle Suite")
}

var _ = Describe("OutputStyle", func() {
	Describe("IsBuiltIn", func() {
		It("returns true for default", func() {
			Expect(outputstyle.IsBuiltIn("default")).To(BeTrue())
		})

		It("returns true for Explanatory", func() {
			Expect(outputstyle.IsBuiltIn("Explanatory")).To(BeTrue())
		})

		It("returns true for Learning", func() {
			Expect(outputstyle.IsBuiltIn("Learning")).To(BeTrue())
		})

		It("returns false for custom style", func() {
			Expect(outputstyle.IsBuiltIn("clotilde/myfeature")).To(BeFalse())
		})

		It("returns false for arbitrary style name", func() {
			Expect(outputstyle.IsBuiltIn("my-project-style")).To(BeFalse())
		})

		It("is case-sensitive", func() {
			Expect(outputstyle.IsBuiltIn("Default")).To(BeFalse())
			Expect(outputstyle.IsBuiltIn("EXPLANATORY")).To(BeFalse())
			Expect(outputstyle.IsBuiltIn("learning")).To(BeFalse())
		})
	})

	Describe("ValidateBuiltIn", func() {
		It("returns nil for valid built-in styles", func() {
			Expect(outputstyle.ValidateBuiltIn("default")).To(Succeed())
			Expect(outputstyle.ValidateBuiltIn("Explanatory")).To(Succeed())
			Expect(outputstyle.ValidateBuiltIn("Learning")).To(Succeed())
		})

		It("returns error for invalid built-in style", func() {
			err := outputstyle.ValidateBuiltIn("invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid built-in style"))
		})

		It("is case-sensitive for validation", func() {
			err := outputstyle.ValidateBuiltIn("Default")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetCustomStylePath", func() {
		It("returns correct path for custom style", func() {
			clotildeRoot := "/home/user/.claude/clotilde"
			sessionName := "my-session"
			expectedPath := "/home/user/.claude/output-styles/clotilde/my-session.md"

			path := outputstyle.GetCustomStylePath(clotildeRoot, sessionName)
			Expect(path).To(Equal(expectedPath))
		})

		It("handles different clotilde roots", func() {
			clotildeRoot := "/tmp/project/.claude/clotilde"
			sessionName := "test"
			expectedPath := "/tmp/project/.claude/output-styles/clotilde/test.md"

			path := outputstyle.GetCustomStylePath(clotildeRoot, sessionName)
			Expect(path).To(Equal(expectedPath))
		})
	})

	Describe("GetCustomStyleReference", func() {
		It("returns correct reference string", func() {
			ref := outputstyle.GetCustomStyleReference("my-session")
			Expect(ref).To(Equal("clotilde/my-session"))
		})

		It("handles different session names", func() {
			ref := outputstyle.GetCustomStyleReference("auth-feature")
			Expect(ref).To(Equal("clotilde/auth-feature"))
		})
	})

	Describe("CreateCustomStyleFile", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("creates output style file with frontmatter", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			content := "Be concise and use bullet points"
			err = outputstyle.CreateCustomStyleFile(clotildeRoot, "test-session", content)
			Expect(err).NotTo(HaveOccurred())

			// Verify file exists
			stylePath := outputstyle.GetCustomStylePath(clotildeRoot, "test-session")
			Expect(stylePath).To(BeAnExistingFile())

			// Verify content
			data, err := os.ReadFile(stylePath)
			Expect(err).NotTo(HaveOccurred())
			fileContent := string(data)

			Expect(fileContent).To(ContainSubstring("name: clotilde/test-session"))
			Expect(fileContent).To(ContainSubstring("description: Output style for session test-session"))
			Expect(fileContent).To(ContainSubstring("keep-coding-instructions: true"))
			Expect(fileContent).To(ContainSubstring(content))
		})

		It("creates directory structure if needed", func() {
			clotildeRoot := filepath.Join(tmpDir, "nested", "dir", ".claude", "clotilde")

			content := "Test content"
			err := outputstyle.CreateCustomStyleFile(clotildeRoot, "session", content)
			Expect(err).NotTo(HaveOccurred())

			stylePath := outputstyle.GetCustomStylePath(clotildeRoot, "session")
			Expect(stylePath).To(BeAnExistingFile())
		})

		It("includes valid YAML frontmatter", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFile(clotildeRoot, "test", "content")
			Expect(err).NotTo(HaveOccurred())

			data, _ := os.ReadFile(outputstyle.GetCustomStylePath(clotildeRoot, "test"))
			content := string(data)

			// Verify frontmatter structure
			Expect(content).To(HavePrefix("---\n"))
			Expect(content).To(ContainSubstring("\n---\n"))
		})
	})

	Describe("CreateCustomStyleFileFromFile", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("creates custom style from file without frontmatter", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			// Create source file without frontmatter
			sourceFile := filepath.Join(tmpDir, "source.md")
			sourceContent := "Be concise and detailed"
			err = os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, "from-file", sourceFile)
			Expect(err).NotTo(HaveOccurred())

			// Verify file was created with frontmatter injected
			stylePath := outputstyle.GetCustomStylePath(clotildeRoot, "from-file")
			data, _ := os.ReadFile(stylePath)
			fileContent := string(data)

			Expect(fileContent).To(ContainSubstring("name: clotilde/from-file"))
			Expect(fileContent).To(ContainSubstring(sourceContent))
		})

		It("creates custom style from file with valid frontmatter", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			// Create source file with valid frontmatter
			sourceFile := filepath.Join(tmpDir, "source.md")
			sourceContent := `---
name: original-name
description: Original description
keep-coding-instructions: false
---

Be detailed and explanatory`

			err = os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, "updated", sourceFile)
			Expect(err).NotTo(HaveOccurred())

			// Verify name was updated to match session
			stylePath := outputstyle.GetCustomStylePath(clotildeRoot, "updated")
			data, _ := os.ReadFile(stylePath)
			fileContent := string(data)

			Expect(fileContent).To(ContainSubstring("name: clotilde/updated"))
			Expect(fileContent).To(ContainSubstring("description: Output style for session updated"))
			Expect(fileContent).To(ContainSubstring("keep-coding-instructions: true"))
			Expect(fileContent).To(ContainSubstring("Be detailed and explanatory"))
		})

		It("returns error for non-existent source file", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			_ = os.MkdirAll(clotildeRoot, 0o755)

			err := outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, "test", "/nonexistent/file.md")
			Expect(err).To(HaveOccurred())
		})

		It("returns error for malformed frontmatter (unclosed)", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			// Create source file with unclosed frontmatter delimiter
			sourceFile := filepath.Join(tmpDir, "bad.md")
			sourceContent := `---
name: test
description: test`

			err = os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, "test", sourceFile)
			Expect(err).To(HaveOccurred())
		})

		It("returns error when frontmatter missing required fields", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			// Create source file with incomplete frontmatter
			sourceFile := filepath.Join(tmpDir, "incomplete.md")
			sourceContent := `---
name: test
---

Content`

			err = os.WriteFile(sourceFile, []byte(sourceContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, "test", sourceFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required fields"))
		})
	})

	Describe("DeleteCustomStyleFile", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("deletes existing custom style file", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			// Create a file
			err = outputstyle.CreateCustomStyleFile(clotildeRoot, "to-delete", "content")
			Expect(err).NotTo(HaveOccurred())

			stylePath := outputstyle.GetCustomStylePath(clotildeRoot, "to-delete")
			Expect(stylePath).To(BeAnExistingFile())

			// Delete it
			err = outputstyle.DeleteCustomStyleFile(clotildeRoot, "to-delete")
			Expect(err).NotTo(HaveOccurred())

			Expect(stylePath).NotTo(BeAnExistingFile())
		})

		It("returns nil when file doesn't exist", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")

			err := outputstyle.DeleteCustomStyleFile(clotildeRoot, "nonexistent")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CustomStyleExists", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("returns true when custom style file exists", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")
			err := os.MkdirAll(clotildeRoot, 0o755)
			Expect(err).NotTo(HaveOccurred())

			err = outputstyle.CreateCustomStyleFile(clotildeRoot, "existing", "content")
			Expect(err).NotTo(HaveOccurred())

			exists := outputstyle.CustomStyleExists(clotildeRoot, "existing")
			Expect(exists).To(BeTrue())
		})

		It("returns false when custom style file doesn't exist", func() {
			clotildeRoot := filepath.Join(tmpDir, ".claude", "clotilde")

			exists := outputstyle.CustomStyleExists(clotildeRoot, "nonexistent")
			Expect(exists).To(BeFalse())
		})
	})
})
