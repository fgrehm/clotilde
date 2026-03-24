package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("FindClotildeRoot", func() {
	var tempDir string
	var originalWd string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Restore original working directory
		err := os.Chdir(originalWd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should find .claude/clotilde in current directory", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.FindClotildeRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(clotildePath))
	})

	It("should find .claude/clotilde in parent directory", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		nestedDir := filepath.Join(tempDir, "nested", "deep", "path")
		err = util.EnsureDir(nestedDir)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(nestedDir)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.FindClotildeRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(clotildePath))
	})

	It("should return error if .claude/clotilde not found", func() {
		err := os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		_, err = config.FindClotildeRoot()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})
})

var _ = Describe("ClotildeRootFromPath", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should find .claude/clotilde from given path", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.ClotildeRootFromPath(tempDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(clotildePath))
	})

	It("should find .claude/clotilde from nested path", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		nestedPath := filepath.Join(tempDir, "nested", "dir")
		err = util.EnsureDir(nestedPath)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.ClotildeRootFromPath(nestedPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(clotildePath))
	})

})

var _ = Describe("Path helpers", func() {
	It("should construct sessions directory path", func() {
		root := "/project/.claude/clotilde"
		path := config.GetSessionsDir(root)
		Expect(path).To(Equal("/project/.claude/clotilde/sessions"))
	})

	It("should construct session directory path", func() {
		root := "/project/.claude/clotilde"
		path := config.GetSessionDir(root, "my-session")
		Expect(path).To(Equal("/project/.claude/clotilde/sessions/my-session"))
	})

	It("should construct config path", func() {
		root := "/project/.claude/clotilde"
		path := config.GetConfigPath(root)
		Expect(path).To(Equal("/project/.claude/clotilde/config.json"))
	})
})

var _ = Describe("EnsureClotildeStructure", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should create .claude/clotilde structure", func() {
		err := config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		Expect(util.DirExists(clotildePath)).To(BeTrue())

		sessionsPath := filepath.Join(clotildePath, config.SessionsDir)
		Expect(util.DirExists(sessionsPath)).To(BeTrue())
	})

	It("should not error if structure already exists", func() {
		err := config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Run again
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("EnsureSessionsDir", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should create .claude/clotilde/sessions in one call", func() {
		err := config.EnsureSessionsDir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		sessionsPath := filepath.Join(tempDir, config.ClotildeDir, config.SessionsDir)
		Expect(util.DirExists(sessionsPath)).To(BeTrue())
	})

	It("should not create config.json", func() {
		err := config.EnsureSessionsDir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		configPath := filepath.Join(tempDir, config.ClotildeDir, config.ConfigFile)
		Expect(util.FileExists(configPath)).To(BeFalse())
	})

	It("should be idempotent", func() {
		err := config.EnsureSessionsDir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		err = config.EnsureSessionsDir(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("ProjectRootFromPath", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should find project root with .claude directory", func() {
		claudeDir := filepath.Join(tempDir, ".claude")
		err := util.EnsureDir(claudeDir)
		Expect(err).NotTo(HaveOccurred())

		root := config.ProjectRootFromPath(tempDir)
		Expect(root).To(Equal(tempDir))
	})

	It("should find project root from nested path", func() {
		claudeDir := filepath.Join(tempDir, ".claude")
		err := util.EnsureDir(claudeDir)
		Expect(err).NotTo(HaveOccurred())

		nestedDir := filepath.Join(tempDir, "nested", "deep")
		err = util.EnsureDir(nestedDir)
		Expect(err).NotTo(HaveOccurred())

		root := config.ProjectRootFromPath(nestedDir)
		Expect(root).To(Equal(tempDir))
	})

	It("should return start path if no .claude directory found", func() {
		root := config.ProjectRootFromPath(tempDir)
		Expect(root).To(Equal(tempDir))
	})

	It("should not walk above $HOME to find .claude", func() {
		// Simulate a subdirectory of $HOME without its own .claude/
		// The walk-up should NOT find ~/.claude/ (Claude Code's global config)
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())

		// Use a temp dir under $HOME to test the boundary
		// Since tempDir is under /tmp (not $HOME), create a subdir under $HOME
		subDir := filepath.Join(homeDir, "clotilde-test-walkup-"+GinkgoT().Name())
		err = util.EnsureDir(subDir)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(subDir) }()

		// ~/.claude/ exists (Claude Code creates it), but ProjectRootFromPath
		// should NOT treat $HOME as the project root
		root := config.ProjectRootFromPath(subDir)
		Expect(root).To(Equal(subDir))
		Expect(root).NotTo(Equal(homeDir))
	})
})

var _ = Describe("FindOrCreateClotildeRoot", func() {
	var tempDir string
	var originalWd string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.Chdir(originalWd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return existing clotilde root if present", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.FindOrCreateClotildeRoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(clotildePath))
	})

	It("should create clotilde structure if not present", func() {
		// Create .claude/ marker so project root is detected
		claudeDir := filepath.Join(tempDir, ".claude")
		err := util.EnsureDir(claudeDir)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.FindOrCreateClotildeRoot()
		Expect(err).NotTo(HaveOccurred())

		expectedRoot := filepath.Join(tempDir, config.ClotildeDir)
		Expect(root).To(Equal(expectedRoot))

		// Verify sessions dir was created
		sessionsDir := filepath.Join(root, config.SessionsDir)
		Expect(util.DirExists(sessionsDir)).To(BeTrue())
	})

	It("should create clotilde structure even without .claude marker", func() {
		err := os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		root, err := config.FindOrCreateClotildeRoot()
		Expect(err).NotTo(HaveOccurred())

		expectedRoot := filepath.Join(tempDir, config.ClotildeDir)
		Expect(root).To(Equal(expectedRoot))
		Expect(util.DirExists(filepath.Join(root, config.SessionsDir))).To(BeTrue())
	})
})

var _ = Describe("IsInitialized", func() {
	var tempDir string
	var originalWd string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		var err error
		originalWd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.Chdir(originalWd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return true if initialized", func() {
		clotildePath := filepath.Join(tempDir, config.ClotildeDir)
		err := util.EnsureDir(clotildePath)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.IsInitialized()).To(BeTrue())
	})

	It("should return false if not initialized", func() {
		err := os.Chdir(tempDir)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.IsInitialized()).To(BeFalse())
	})
})
