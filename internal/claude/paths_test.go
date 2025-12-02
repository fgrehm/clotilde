package claude_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

var _ = Describe("Paths", func() {
	Describe("ProjectDir", func() {
		It("should encode project path correctly", func() {
			clotildeRoot := "/home/user/project/.claude/clotilde"
			encoded := claude.ProjectDir(clotildeRoot)
			Expect(encoded).To(Equal("-home-user-project"))
		})

		It("should handle paths with dots", func() {
			clotildeRoot := "/home/user/my.project/.claude/clotilde"
			encoded := claude.ProjectDir(clotildeRoot)
			Expect(encoded).To(Equal("-home-user-my-project"))
		})

		It("should handle nested paths", func() {
			clotildeRoot := "/home/user/projects/foo/bar/.claude/clotilde"
			encoded := claude.ProjectDir(clotildeRoot)
			Expect(encoded).To(Equal("-home-user-projects-foo-bar"))
		})
	})

	Describe("TranscriptPath", func() {
		It("should generate correct transcript path", func() {
			homeDir := "/home/user"
			clotildeRoot := "/home/user/project/.claude/clotilde"
			sessionID := "550e8400-e29b-41d4-a716-446655440000"

			path := claude.TranscriptPath(homeDir, clotildeRoot, sessionID)
			expected := filepath.Join(homeDir, ".claude", "projects", "-home-user-project", sessionID+".jsonl")
			Expect(path).To(Equal(expected))
		})
	})

	Describe("AgentLogPattern", func() {
		It("should generate correct agent log glob pattern", func() {
			homeDir := "/home/user"
			clotildeRoot := "/home/user/project/.claude/clotilde"

			pattern := claude.AgentLogPattern(homeDir, clotildeRoot)
			expected := filepath.Join(homeDir, ".claude", "projects", "-home-user-project", "agent-*.jsonl")
			Expect(pattern).To(Equal(expected))
		})
	})
})
