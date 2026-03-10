package export_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/export"
)

var _ = Describe("FilterTranscript", func() {
	It("keeps user and assistant entries from mixed input", func() {
		input := strings.NewReader(`{"type":"progress","timestamp":"2025-01-01T00:00:00Z"}
{"type":"user","timestamp":"2025-01-01T00:01:00Z","message":{"content":"hello"}}
{"type":"system","timestamp":"2025-01-01T00:01:01Z"}
{"type":"assistant","timestamp":"2025-01-01T00:01:05Z","message":{"content":[{"type":"text","text":"hi"}]}}
{"type":"file-history-snapshot","timestamp":"2025-01-01T00:00:00Z"}
`)
		entries, err := export.FilterTranscript(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(2))
	})

	It("skips all non-user/assistant types", func() {
		input := strings.NewReader(`{"type":"progress"}
{"type":"system"}
{"type":"file-history-snapshot"}
{"type":"last-prompt"}
{"type":"queue-operation"}
{"type":"summary"}
`)
		entries, err := export.FilterTranscript(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("skips malformed JSON lines without error", func() {
		input := strings.NewReader(`not json at all
{"type":"user","message":{"content":"hello"}}
{broken json
{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}
`)
		entries, err := export.FilterTranscript(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(2))
	})

	It("returns empty slice for empty input", func() {
		input := strings.NewReader("")
		entries, err := export.FilterTranscript(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("preserves raw JSON exactly (round-trip check)", func() {
		original := `{"type":"user","timestamp":"2025-01-01T00:01:00Z","message":{"content":"hello world"}}`
		input := strings.NewReader(original + "\n")
		entries, err := export.FilterTranscript(input)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
		Expect(string(entries[0])).To(Equal(original))
	})
})

var _ = Describe("BuildHTML", func() {
	It("contains base64-encoded JSON decodable to ExportData", func() {
		entries := []json.RawMessage{
			json.RawMessage(`{"type":"user","message":{"content":"hello"}}`),
			json.RawMessage(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`),
		}

		html, err := export.BuildHTML("test-session", entries)
		Expect(err).NotTo(HaveOccurred())

		// Extract base64 data from HTML
		startMarker := `<script id="session-data" type="application/json">`
		endMarker := `</script>`
		startIdx := strings.Index(html, startMarker)
		Expect(startIdx).To(BeNumerically(">=", 0), "should contain session-data script tag")
		startIdx += len(startMarker)
		endIdx := strings.Index(html[startIdx:], endMarker)
		Expect(endIdx).To(BeNumerically(">=", 0))
		b64Data := html[startIdx : startIdx+endIdx]

		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(b64Data)
		Expect(err).NotTo(HaveOccurred())

		// Parse JSON
		var data export.ExportData
		err = json.Unmarshal(decoded, &data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data.SessionName).To(Equal("test-session"))
		Expect(data.Entries).To(HaveLen(2))
	})

	It("contains session name in title tag", func() {
		html, err := export.BuildHTML("my-feature", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(html).To(ContainSubstring("<title>Session: my-feature</title>"))
	})

	It("escapes HTML metacharacters in session name", func() {
		html, err := export.BuildHTML(`<script>alert("xss")</script>`, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(html).NotTo(ContainSubstring(`<script>alert("xss")</script>`))
		Expect(html).To(ContainSubstring("&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;"))
	})

	It("has no external resource references", func() {
		html, err := export.BuildHTML("test", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(html).NotTo(ContainSubstring(`href="http`))
		Expect(html).NotTo(ContainSubstring(`src="http`))
	})
})

var _ = Describe("Export", func() {
	It("writes HTML file to disk from fixture JSONL", func() {
		tmpDir := GinkgoT().TempDir()

		// Create fixture JSONL
		transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
		data := `{"type":"user","timestamp":"2025-01-01T00:01:00Z","message":{"content":"hello"}}
{"type":"assistant","timestamp":"2025-01-01T00:01:05Z","message":{"content":[{"type":"text","text":"hi"}]}}
`
		err := os.WriteFile(transcriptPath, []byte(data), 0o644)
		Expect(err).NotTo(HaveOccurred())

		outputPath := filepath.Join(tmpDir, "output.html")
		err = export.Export(transcriptPath, "test-session", outputPath)
		Expect(err).NotTo(HaveOccurred())

		// Verify file exists and contains HTML
		content, err := os.ReadFile(outputPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("<!DOCTYPE html>"))
		Expect(string(content)).To(ContainSubstring("test-session"))
	})

	It("returns error for non-existent transcript path", func() {
		tmpDir := GinkgoT().TempDir()
		outputPath := filepath.Join(tmpDir, "output.html")
		err := export.Export("/nonexistent/path.jsonl", "test", outputPath)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("ExportToWriter", func() {
	It("writes HTML to a bytes.Buffer", func() {
		tmpDir := GinkgoT().TempDir()

		transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
		data := `{"type":"user","message":{"content":"hello"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}
`
		err := os.WriteFile(transcriptPath, []byte(data), 0o644)
		Expect(err).NotTo(HaveOccurred())

		var buf bytes.Buffer
		err = export.ExportToWriter(transcriptPath, "test-session", &buf)
		Expect(err).NotTo(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("<!DOCTYPE html>"))
		Expect(buf.String()).To(ContainSubstring("test-session"))
	})
})
