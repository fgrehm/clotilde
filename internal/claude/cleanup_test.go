package claude_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

var _ = Describe("Cleanup", func() {
	var (
		tempDir      string
		clotildeRoot string
		projectDir   string
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		clotildeRoot = filepath.Join(tempDir, ".claude", "clotilde")
		projectDir = filepath.Join(tempDir, ".claude", "projects", "test-project")

		// Create project directory
		err := os.MkdirAll(projectDir, 0o755)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("DeleteSessionData", func() {
		It("should delete transcript using stored transcript path", func() {
			// Create transcript file
			transcriptPath := filepath.Join(projectDir, "session-uuid-123.jsonl")
			err := os.WriteFile(transcriptPath, []byte("transcript content"), 0o644)
			Expect(err).NotTo(HaveOccurred())
			Expect(transcriptPath).To(BeAnExistingFile())

			// Delete session data with transcript path
			deleted, err := claude.DeleteSessionData(clotildeRoot, "session-uuid-123", transcriptPath)
			Expect(err).NotTo(HaveOccurred())

			// Verify transcript was deleted
			Expect(transcriptPath).NotTo(BeAnExistingFile())
			Expect(deleted.Transcript).To(HaveLen(1))
			Expect(deleted.Transcript[0]).To(Equal(transcriptPath))
		})

		It("should delete agent logs referencing the session", func() {
			// Create transcript file
			transcriptPath := filepath.Join(projectDir, "session-uuid-456.jsonl")
			err := os.WriteFile(transcriptPath, []byte("transcript"), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Create agent log that references the session
			agentLogPath := filepath.Join(projectDir, "agent-test-123.jsonl")
			agentLogContent := `{"type":"agent","sessionId":"session-uuid-456"}` + "\n"
			err = os.WriteFile(agentLogPath, []byte(agentLogContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Create another agent log that doesn't reference this session
			otherAgentLog := filepath.Join(projectDir, "agent-other-456.jsonl")
			err = os.WriteFile(otherAgentLog, []byte(`{"type":"agent","sessionId":"other-session"}`), 0o644)
			Expect(err).NotTo(HaveOccurred())

			// Delete session data
			deleted, err := claude.DeleteSessionData(clotildeRoot, "session-uuid-456", transcriptPath)
			Expect(err).NotTo(HaveOccurred())

			// Verify transcript was deleted
			Expect(transcriptPath).NotTo(BeAnExistingFile())
			Expect(deleted.Transcript).To(HaveLen(1))
			Expect(deleted.Transcript[0]).To(Equal(transcriptPath))

			// Verify agent log referencing session was deleted
			Expect(agentLogPath).NotTo(BeAnExistingFile())
			Expect(deleted.AgentLogs).To(HaveLen(1))
			Expect(deleted.AgentLogs[0]).To(Equal(agentLogPath))

			// Verify other agent log was NOT deleted
			Expect(otherAgentLog).To(BeAnExistingFile())
		})

		It("should handle missing transcript file gracefully", func() {
			// Delete session data for non-existent transcript
			transcriptPath := filepath.Join(projectDir, "non-existent.jsonl")
			deleted, err := claude.DeleteSessionData(clotildeRoot, "session-uuid-789", transcriptPath)

			// Should not error even though file doesn't exist
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted.Transcript).To(BeEmpty())
			Expect(deleted.AgentLogs).To(BeEmpty())
		})

		It("should fall back to computed path if transcript path is empty", func() {
			// Create project directory matching clotildeRoot encoding
			// For this test, we won't create actual files, just verify it doesn't error
			deleted, err := claude.DeleteSessionData(tempDir, "test-session-id", "")

			// Should not error (files don't exist, which is fine)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).NotTo(BeNil())
		})
	})
})
