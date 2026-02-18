package claude_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/session"
)

var _ = Describe("defaultSessionUsed", func() {
	var (
		tempDir      string
		clotildeRoot string
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		// Create a clotilde root that maps to a predictable project dir
		clotildeRoot = filepath.Join(tempDir, "project", ".claude", "clotilde")
		err := os.MkdirAll(clotildeRoot, 0o755)
		Expect(err).NotTo(HaveOccurred())

		// Reset to default implementation
		claude.SessionUsedFunc = claude.DefaultSessionUsed
	})

	AfterEach(func() {
		claude.SessionUsedFunc = claude.DefaultSessionUsed
	})

	It("should return false when session has no ID", func() {
		sess := &session.Session{
			Metadata: session.Metadata{SessionID: ""},
		}
		Expect(claude.SessionUsedFunc(clotildeRoot, sess)).To(BeFalse())
	})

	It("should use TranscriptPath from metadata when available", func() {
		// Create a transcript file at a custom path (simulating symlink scenario)
		transcriptDir := filepath.Join(tempDir, "custom-path")
		err := os.MkdirAll(transcriptDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		transcriptPath := filepath.Join(transcriptDir, "test-uuid.jsonl")
		err = os.WriteFile(transcriptPath, []byte("transcript content"), 0o644)
		Expect(err).NotTo(HaveOccurred())

		sess := &session.Session{
			Metadata: session.Metadata{
				SessionID:      "test-uuid",
				TranscriptPath: transcriptPath,
			},
		}
		Expect(claude.SessionUsedFunc(clotildeRoot, sess)).To(BeTrue())
	})

	It("should return false when metadata TranscriptPath does not exist", func() {
		sess := &session.Session{
			Metadata: session.Metadata{
				SessionID:      "test-uuid",
				TranscriptPath: filepath.Join(tempDir, "nonexistent", "test-uuid.jsonl"),
			},
		}
		Expect(claude.SessionUsedFunc(clotildeRoot, sess)).To(BeFalse())
	})

	It("should prefer metadata TranscriptPath over computed path", func() {
		// Create a transcript at the computed path (this should NOT be found)
		homeDir, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())

		projectDir := claude.ProjectDir(clotildeRoot)
		computedDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
		err = os.MkdirAll(computedDir, 0o755)
		Expect(err).NotTo(HaveOccurred())

		computedTranscript := filepath.Join(computedDir, "test-uuid.jsonl")
		err = os.WriteFile(computedTranscript, []byte("transcript"), 0o644)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.Remove(computedTranscript) }()

		// Set metadata TranscriptPath to a non-existent file
		sess := &session.Session{
			Metadata: session.Metadata{
				SessionID:      "test-uuid",
				TranscriptPath: filepath.Join(tempDir, "wrong-path", "test-uuid.jsonl"),
			},
		}
		// Should return false because it uses metadata path (which doesn't exist),
		// NOT the computed path (which does exist)
		Expect(claude.SessionUsedFunc(clotildeRoot, sess)).To(BeFalse())
	})
})
