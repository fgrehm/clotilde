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

	It("should construct global context path", func() {
		root := "/project/.claude/clotilde"
		path := config.GetGlobalContextPath(root)
		Expect(path).To(Equal("/project/.claude/clotilde/context.md"))
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

		configPath := filepath.Join(clotildePath, config.ConfigFile)
		Expect(util.FileExists(configPath)).To(BeTrue())
	})

	It("should not error if structure already exists", func() {
		err := config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		// Run again
		err = config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())
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
