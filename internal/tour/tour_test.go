package tour_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/tour"
)

var _ = Describe("LoadFile", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("parses valid tour JSON with all fields", func() {
		tourJSON := `{
			"$schema": "https://aka.ms/codetour-schema",
			"title": "Architecture Overview",
			"steps": [
				{"file": "src/main.rs", "line": 28, "description": "## Entry Point\nThe main function."},
				{"file": "src/config.rs", "line": 5, "description": "## Config\nConfiguration module."}
			]
		}`
		path := filepath.Join(tempDir, "overview.tour")
		Expect(os.WriteFile(path, []byte(tourJSON), 0o644)).To(Succeed())

		t, err := tour.LoadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Title).To(Equal("Architecture Overview"))
		Expect(t.Steps).To(HaveLen(2))
		Expect(t.Steps[0].File).To(Equal("src/main.rs"))
		Expect(t.Steps[0].Line).To(Equal(28))
		Expect(t.Steps[0].Description).To(Equal("## Entry Point\nThe main function."))
		Expect(t.Steps[1].File).To(Equal("src/config.rs"))
		Expect(t.Steps[1].Line).To(Equal(5))
	})

	It("defaults line to 1 when omitted", func() {
		tourJSON := `{
			"title": "Minimal",
			"steps": [{"file": "README.md", "description": "The readme."}]
		}`
		path := filepath.Join(tempDir, "minimal.tour")
		Expect(os.WriteFile(path, []byte(tourJSON), 0o644)).To(Succeed())

		t, err := tour.LoadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Steps[0].Line).To(Equal(1))
	})

	It("returns error for invalid JSON", func() {
		path := filepath.Join(tempDir, "bad.tour")
		Expect(os.WriteFile(path, []byte("not json"), 0o644)).To(Succeed())

		_, err := tour.LoadFile(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parse"))
	})

	It("returns error for empty steps array", func() {
		tourJSON := `{"title": "Empty", "steps": []}`
		path := filepath.Join(tempDir, "empty.tour")
		Expect(os.WriteFile(path, []byte(tourJSON), 0o644)).To(Succeed())

		_, err := tour.LoadFile(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("at least one step"))
	})

	It("returns error when step has no file field", func() {
		tourJSON := `{"title": "Bad", "steps": [{"description": "no file"}]}`
		path := filepath.Join(tempDir, "nofile.tour")
		Expect(os.WriteFile(path, []byte(tourJSON), 0o644)).To(Succeed())

		_, err := tour.LoadFile(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("file"))
	})

	It("returns error for nonexistent file", func() {
		_, err := tour.LoadFile(filepath.Join(tempDir, "nope.tour"))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("LoadFromDir", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
	})

	It("finds all .tour files in a directory", func() {
		toursDir := filepath.Join(tempDir, ".tours")
		Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())

		tour1 := `{"title": "Tour One", "steps": [{"file": "a.go", "line": 1, "description": "first"}]}`
		tour2 := `{"title": "Tour Two", "steps": [{"file": "b.go", "line": 2, "description": "second"}]}`
		Expect(os.WriteFile(filepath.Join(toursDir, "one.tour"), []byte(tour1), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(toursDir, "two.tour"), []byte(tour2), 0o644)).To(Succeed())

		tours, err := tour.LoadFromDir(toursDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tours).To(HaveLen(2))
		Expect(tours).To(HaveKey("one"))
		Expect(tours).To(HaveKey("two"))
		Expect(tours["one"].Title).To(Equal("Tour One"))
		Expect(tours["two"].Title).To(Equal("Tour Two"))
	})

	It("returns empty map when directory does not exist", func() {
		tours, err := tour.LoadFromDir(filepath.Join(tempDir, "nonexistent"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tours).To(BeEmpty())
	})

	It("skips files that fail to parse", func() {
		toursDir := filepath.Join(tempDir, ".tours")
		Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())

		good := `{"title": "Good", "steps": [{"file": "a.go", "line": 1, "description": "ok"}]}`
		bad := `not json`
		Expect(os.WriteFile(filepath.Join(toursDir, "good.tour"), []byte(good), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(toursDir, "bad.tour"), []byte(bad), 0o644)).To(Succeed())

		tours, err := tour.LoadFromDir(toursDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tours).To(HaveLen(1))
		Expect(tours).To(HaveKey("good"))
	})

	It("ignores non-.tour files", func() {
		toursDir := filepath.Join(tempDir, ".tours")
		Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())

		tourFile := `{"title": "Real", "steps": [{"file": "a.go", "line": 1, "description": "ok"}]}`
		Expect(os.WriteFile(filepath.Join(toursDir, "real.tour"), []byte(tourFile), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(toursDir, "readme.md"), []byte("# hi"), 0o644)).To(Succeed())

		tours, err := tour.LoadFromDir(toursDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tours).To(HaveLen(1))
		Expect(tours).To(HaveKey("real"))
	})
})
