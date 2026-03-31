package util_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("EnsureDir", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should create a directory if it doesn't exist", func() {
		testPath := filepath.Join(tempDir, "newdir")
		err := util.EnsureDir(testPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.DirExists(testPath)).To(BeTrue())
	})

	It("should create nested directories", func() {
		testPath := filepath.Join(tempDir, "parent", "child", "grandchild")
		err := util.EnsureDir(testPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.DirExists(testPath)).To(BeTrue())
	})

	It("should not error if directory already exists", func() {
		testPath := filepath.Join(tempDir, "existing")
		err := os.Mkdir(testPath, 0o755)
		Expect(err).NotTo(HaveOccurred())

		err = util.EnsureDir(testPath)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("FileExists", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should return true if file exists", func() {
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		Expect(util.FileExists(testFile)).To(BeTrue())
	})

	It("should return false if file doesn't exist", func() {
		testFile := filepath.Join(tempDir, "nonexistent.txt")
		Expect(util.FileExists(testFile)).To(BeFalse())
	})

	It("should return false for directories", func() {
		Expect(util.FileExists(tempDir)).To(BeFalse())
	})
})

var _ = Describe("DirExists", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should return true if directory exists", func() {
		Expect(util.DirExists(tempDir)).To(BeTrue())
	})

	It("should return false if directory doesn't exist", func() {
		testDir := filepath.Join(tempDir, "nonexistent")
		Expect(util.DirExists(testDir)).To(BeFalse())
	})

	It("should return false for files", func() {
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		Expect(util.DirExists(testFile)).To(BeFalse())
	})
})

var _ = Describe("CopyFile", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should copy a file", func() {
		srcFile := filepath.Join(tempDir, "source.txt")
		dstFile := filepath.Join(tempDir, "dest.txt")

		content := []byte("test content")
		err := os.WriteFile(srcFile, content, 0o644)
		Expect(err).NotTo(HaveOccurred())

		err = util.CopyFile(srcFile, dstFile)
		Expect(err).NotTo(HaveOccurred())

		readContent, err := os.ReadFile(dstFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(readContent).To(Equal(content))
	})

	It("should create parent directories if needed", func() {
		srcFile := filepath.Join(tempDir, "source.txt")
		dstFile := filepath.Join(tempDir, "nested", "dir", "dest.txt")

		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		err = util.CopyFile(srcFile, dstFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.FileExists(dstFile)).To(BeTrue())
	})

	It("should preserve file permissions", func() {
		srcFile := filepath.Join(tempDir, "source.txt")
		dstFile := filepath.Join(tempDir, "dest.txt")

		err := os.WriteFile(srcFile, []byte("test"), 0o600)
		Expect(err).NotTo(HaveOccurred())

		err = util.CopyFile(srcFile, dstFile)
		Expect(err).NotTo(HaveOccurred())

		srcInfo, _ := os.Stat(srcFile)
		dstInfo, _ := os.Stat(dstFile)
		Expect(dstInfo.Mode()).To(Equal(srcInfo.Mode()))
	})
})

var _ = Describe("ReadJSON and WriteJSON", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should write and read JSON", func() {
		testFile := filepath.Join(tempDir, "test.json")

		data := map[string]any{
			"name":  "test",
			"value": 123,
			"nested": map[string]string{
				"key": "value",
			},
		}

		err := util.WriteJSON(testFile, data)
		Expect(err).NotTo(HaveOccurred())

		var readData map[string]any
		err = util.ReadJSON(testFile, &readData)
		Expect(err).NotTo(HaveOccurred())
		Expect(readData["name"]).To(Equal("test"))
		Expect(readData["value"]).To(BeNumerically("==", 123))
	})

	It("should create parent directories when writing JSON", func() {
		testFile := filepath.Join(tempDir, "nested", "dir", "test.json")

		data := map[string]string{"key": "value"}
		err := util.WriteJSON(testFile, data)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.FileExists(testFile)).To(BeTrue())
	})

	It("should format JSON with indentation", func() {
		testFile := filepath.Join(tempDir, "test.json")

		data := map[string]string{"key": "value"}
		err := util.WriteJSON(testFile, data)
		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(testFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("  ")) // Check for indentation
	})
})

var _ = Describe("HomeDir", func() {
	It("should return the user's home directory", func() {
		home, err := util.HomeDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(home).NotTo(BeEmpty())
	})
})

var _ = Describe("WriteFile and ReadFile", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should write and read file content", func() {
		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("test content")

		err := util.WriteFile(testFile, content)
		Expect(err).NotTo(HaveOccurred())

		readContent, err := util.ReadFile(testFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(readContent).To(Equal(content))
	})

	It("should create parent directories when writing", func() {
		testFile := filepath.Join(tempDir, "nested", "dir", "test.txt")

		err := util.WriteFile(testFile, []byte("test"))
		Expect(err).NotTo(HaveOccurred())
		Expect(util.FileExists(testFile)).To(BeTrue())
	})
})

var _ = Describe("RemoveAll", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("should remove a directory and all contents", func() {
		testDir := filepath.Join(tempDir, "toremove")
		err := os.MkdirAll(filepath.Join(testDir, "subdir"), 0o755)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		err = util.RemoveAll(testDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.DirExists(testDir)).To(BeFalse())
	})

	It("should remove a single file", func() {
		testFile := filepath.Join(tempDir, "file.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		err = util.RemoveAll(testFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(util.FileExists(testFile)).To(BeFalse())
	})
})
