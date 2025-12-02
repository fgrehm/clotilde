package session_test

import (
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("FileStore", func() {
	var (
		tempDir      string
		clotildeRoot string
		store        *session.FileStore
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		clotildeRoot = filepath.Join(tempDir, config.ClotildeDir)
		err := config.EnsureClotildeStructure(tempDir)
		Expect(err).NotTo(HaveOccurred())

		store = session.NewFileStore(clotildeRoot)
	})

	Describe("Create and Get", func() {
		It("should create and retrieve a session", func() {
			s := session.NewSession("test-session", "uuid-123")

			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := store.Get("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("test-session"))
			Expect(retrieved.Metadata.SessionID).To(Equal("uuid-123"))
		})

		It("should reject invalid session names", func() {
			s := session.NewSession("INVALID", "uuid")
			err := store.Create(s)
			Expect(err).To(HaveOccurred())
		})

		It("should error if session already exists", func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			err = store.Create(s)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})
	})

	Describe("Update", func() {
		It("should update session metadata", func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(10 * time.Millisecond)
			s.UpdateLastAccessed()
			err = store.Update(s)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := store.Get("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Metadata.LastAccessed).To(BeTemporally("~", s.Metadata.LastAccessed, time.Millisecond))
		})

		It("should error if session doesn't exist", func() {
			s := session.NewSession("nonexistent", "uuid")
			err := store.Update(s)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Describe("Delete", func() {
		It("should delete a session", func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			err = store.Delete("test-session")
			Expect(err).NotTo(HaveOccurred())

			_, err = store.Get("test-session")
			Expect(err).To(HaveOccurred())
		})

		It("should error if session doesn't exist", func() {
			err := store.Delete("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Describe("Exists", func() {
		It("should return true if session exists", func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			Expect(store.Exists("test-session")).To(BeTrue())
		})

		It("should return false if session doesn't exist", func() {
			Expect(store.Exists("nonexistent")).To(BeFalse())
		})
	})

	Describe("List", func() {
		It("should list all sessions sorted by lastAccessed", func() {
			s1 := session.NewSession("session-1", "uuid-1")
			s2 := session.NewSession("session-2", "uuid-2")
			s3 := session.NewSession("session-3", "uuid-3")

			err := store.Create(s1)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(10 * time.Millisecond)
			err = store.Create(s2)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(10 * time.Millisecond)
			err = store.Create(s3)
			Expect(err).NotTo(HaveOccurred())

			sessions, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(sessions).To(HaveLen(3))

			// Should be sorted by lastAccessed (most recent first)
			Expect(sessions[0].Name).To(Equal("session-3"))
			Expect(sessions[1].Name).To(Equal("session-2"))
			Expect(sessions[2].Name).To(Equal("session-1"))
		})

		It("should return empty list if no sessions", func() {
			sessions, err := store.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(sessions).To(BeEmpty())
		})
	})

	Describe("Settings operations", func() {
		BeforeEach(func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should save and load settings", func() {
			settings := &session.Settings{
				Model: "sonnet",
				Permissions: session.Permissions{
					Allow: []string{"Bash(git:*)"},
					Deny:  []string{"Read(./.env)"},
				},
			}

			err := store.SaveSettings("test-session", settings)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := store.LoadSettings("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded.Model).To(Equal("sonnet"))
			Expect(loaded.Permissions.Allow).To(ContainElement("Bash(git:*)"))
			Expect(loaded.Permissions.Deny).To(ContainElement("Read(./.env)"))
		})

		It("should return nil if settings don't exist", func() {
			loaded, err := store.LoadSettings("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeNil())
		})
	})

	Describe("System prompt operations", func() {
		BeforeEach(func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should save and load system prompt", func() {
			content := "You are a helpful assistant"

			err := store.SaveSystemPrompt("test-session", content)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := store.LoadSystemPrompt("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(Equal(content))
		})

		It("should return empty string if system prompt doesn't exist", func() {
			loaded, err := store.LoadSystemPrompt("test-session")
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeEmpty())
		})
	})

	Describe("File existence checks", func() {
		It("should check if settings file exists", func() {
			s := session.NewSession("test-session", "uuid-123")
			err := store.Create(s)
			Expect(err).NotTo(HaveOccurred())

			sessionDir := config.GetSessionDir(clotildeRoot, "test-session")
			settingsPath := filepath.Join(sessionDir, "settings.json")

			Expect(util.FileExists(settingsPath)).To(BeFalse())

			settings := &session.Settings{Model: "sonnet"}
			err = store.SaveSettings("test-session", settings)
			Expect(err).NotTo(HaveOccurred())

			Expect(util.FileExists(settingsPath)).To(BeTrue())
		})
	})
})
