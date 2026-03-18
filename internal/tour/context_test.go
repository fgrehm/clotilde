package tour_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/tour"
)

var _ = Describe("GatherContext", func() {
	var repoDir string

	BeforeEach(func() {
		repoDir = GinkgoT().TempDir()

		// Create a repo structure
		Expect(os.MkdirAll(filepath.Join(repoDir, "src"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(repoDir, ".git", "objects"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(repoDir, "node_modules", "foo"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(repoDir, "target"), 0o755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "src", "lib.rs"), []byte("pub fn hello() {}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "src", "config.go"), []byte("package src\n\ntype Config struct{}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "node_modules", "foo", "index.js"), []byte("module.exports = {}"), 0o644)).To(Succeed())
	})

	It("collects file tree excluding .git, node_modules, target", func() {
		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ctx).To(ContainSubstring("main.go"))
		Expect(ctx).To(ContainSubstring("src/lib.rs"))
		Expect(ctx).NotTo(ContainSubstring("node_modules"))
		Expect(ctx).NotTo(ContainSubstring(".git"))
		Expect(ctx).NotTo(ContainSubstring("target"))
	})

	It("includes README.md when present", func() {
		Expect(os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# My Project\nThis is great."), 0o644)).To(Succeed())

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ctx).To(ContainSubstring("# My Project"))
		Expect(ctx).To(ContainSubstring("This is great."))
	})

	It("includes CLAUDE.md when present", func() {
		Expect(os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("# Claude Context\nBuild with make."), 0o644)).To(Succeed())

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ctx).To(ContainSubstring("# Claude Context"))
	})

	It("identifies entry point files (main.*, lib.*)", func() {
		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())
		// main.go and lib.rs should have their contents included
		Expect(ctx).To(ContainSubstring("package main"))
		Expect(ctx).To(ContainSubstring("pub fn hello"))
	})

	It("respects MaxFiles limit", func() {
		// Create many files
		for i := 0; i < 30; i++ {
			name := filepath.Join(repoDir, "src", "file"+string(rune('a'+i))+".go")
			Expect(os.WriteFile(name, []byte("package src\n"), 0o644)).To(Succeed())
		}

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{MaxFiles: 5})
		Expect(err).NotTo(HaveOccurred())
		// Should still have the file tree but limited file contents
		Expect(ctx).To(ContainSubstring("File tree"))
	})

	It("respects MaxLinesPerFile limit", func() {
		// Create a file with many lines
		var content string
		for i := 0; i < 200; i++ {
			content += "// line\n"
		}
		Expect(os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(content), 0o644)).To(Succeed())

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{MaxLinesPerFile: 10})
		Expect(err).NotTo(HaveOccurred())
		// The file content should be truncated
		Expect(ctx).NotTo(BeEmpty())
	})

	It("caps total output size", func() {
		// Create a large file
		big := make([]byte, 50000)
		for i := range big {
			big[i] = 'x'
		}
		Expect(os.WriteFile(filepath.Join(repoDir, "main.go"), big, 0o644)).To(Succeed())

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(ctx)).To(BeNumerically("<", 35000))
	})

	It("respects .gitignore patterns", func() {
		// Create a .gitignore file
		gitignore := `*.log
build/
*.tmp
`
		Expect(os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(gitignore), 0o644)).To(Succeed())

		// Create files that should be ignored
		Expect(os.WriteFile(filepath.Join(repoDir, "debug.log"), []byte("log content"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "cache.tmp"), []byte("temp"), 0o644)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(repoDir, "build"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repoDir, "build", "artifact"), []byte("built"), 0o644)).To(Succeed())

		ctx, err := tour.GatherContext(repoDir, tour.ContextOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Ignored files should not appear in file tree
		lines := strings.Split(ctx, "\n")
		var fileTreeSection []string
		inFileTree := false
		for _, line := range lines {
			if strings.Contains(line, "## File tree") {
				inFileTree = true
				continue
			}
			if inFileTree && strings.HasPrefix(line, "##") {
				break
			}
			if inFileTree {
				fileTreeSection = append(fileTreeSection, line)
			}
		}

		fileTreeContent := strings.Join(fileTreeSection, "\n")
		Expect(fileTreeContent).NotTo(ContainSubstring("debug.log"))
		Expect(fileTreeContent).NotTo(ContainSubstring("cache.tmp"))
		Expect(fileTreeContent).NotTo(ContainSubstring("build/artifact"))

		// But included files should be in file tree
		Expect(fileTreeContent).To(ContainSubstring("main.go"))
		Expect(fileTreeContent).To(ContainSubstring("src/lib.rs"))
	})
})
