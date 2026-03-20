package tour_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/tour"
)

var _ = Describe("ExtractJSON", func() {
	It("returns raw JSON if no code fence", func() {
		input := `{"title": "Test", "steps": []}`
		Expect(tour.ExtractJSON(input)).To(Equal(input))
	})

	It("extracts JSON from bare code fence", func() {
		input := "```\n{\"title\": \"Test\"}\n```"
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test"}`))
	})

	It("extracts JSON from json-tagged code fence", func() {
		input := "```json\n{\"title\": \"Test\"}\n```"
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test"}`))
	})

	It("extracts JSON when there is preamble text before the fence", func() {
		input := "Now I have everything I need. Let me write it:\n\n```json\n{\"title\": \"Test\"}\n```"
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test"}`))
	})

	It("extracts JSON when there is preamble text without a fence", func() {
		input := "I've read all the files. Here's the CodeTour:\n\n{\"title\": \"Test\", \"steps\": []}"
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test", "steps": []}`))
	})

	It("extracts JSON when there is preamble and trailing text", func() {
		input := "Here it is:\n{\"title\": \"Test\"}\n\nLet me know if you need changes."
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test"}`))
	})

	It("trims surrounding whitespace", func() {
		input := "  \n  {\"title\": \"Test\"}  \n  "
		Expect(tour.ExtractJSON(input)).To(Equal(`{"title": "Test"}`))
	})
})

var _ = Describe("ValidateTourJSON", func() {
	var repoDir string

	BeforeEach(func() {
		repoDir = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0o644)).To(Succeed())
	})

	It("accepts valid tour JSON", func() {
		data := []byte(`{"title": "Test", "steps": [{"file": "main.go", "line": 1, "description": "entry"}]}`)
		t, err := tour.ValidateTourJSON(data, repoDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Title).To(Equal("Test"))
	})

	It("rejects invalid JSON", func() {
		_, err := tour.ValidateTourJSON([]byte("not json"), repoDir)
		Expect(err).To(HaveOccurred())
	})

	It("rejects missing steps", func() {
		_, err := tour.ValidateTourJSON([]byte(`{"title": "Empty"}`), repoDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("at least one step"))
	})

	It("rejects steps pointing to nonexistent files", func() {
		data := []byte(`{"title": "Bad", "steps": [{"file": "nope.go", "line": 1, "description": "x"}]}`)
		_, err := tour.ValidateTourJSON(data, repoDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nope.go"))
	})

	It("warns on lines beyond file length", func() {
		data := []byte(`{"title": "T", "steps": [{"file": "main.go", "line": 9999, "description": "x"}]}`)
		_, err := tour.ValidateTourJSON(data, repoDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("line 9999"))
	})
})
