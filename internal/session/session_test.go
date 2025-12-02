package session_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/session"
)

var _ = Describe("Session", func() {
	Describe("NewSession", func() {
		It("should create a new session with given name and UUID", func() {
			name := "test-session"
			sessionID := "550e8400-e29b-41d4-a716-446655440000"

			s := session.NewSession(name, sessionID)

			Expect(s.Name).To(Equal(name))
			Expect(s.Metadata.Name).To(Equal(name))
			Expect(s.Metadata.SessionID).To(Equal(sessionID))
			Expect(s.Metadata.Created).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.LastAccessed).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.IsForkedSession).To(BeFalse())
			Expect(s.Metadata.ParentSession).To(BeEmpty())
		})
	})

	Describe("NewForkedSession", func() {
		It("should create a forked session with empty sessionId", func() {
			name := "forked-session"
			parentName := "parent-session"

			s := session.NewForkedSession(name, parentName)

			Expect(s.Name).To(Equal(name))
			Expect(s.Metadata.Name).To(Equal(name))
			Expect(s.Metadata.SessionID).To(BeEmpty())
			Expect(s.Metadata.Created).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.LastAccessed).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.IsForkedSession).To(BeTrue())
			Expect(s.Metadata.ParentSession).To(Equal(parentName))
		})
	})

	Describe("NewIncognitoSession", func() {
		It("should create an incognito session with IsIncognito set to true", func() {
			name := "incognito-session"
			sessionID := "550e8400-e29b-41d4-a716-446655440000"

			s := session.NewIncognitoSession(name, sessionID)

			Expect(s.Name).To(Equal(name))
			Expect(s.Metadata.Name).To(Equal(name))
			Expect(s.Metadata.SessionID).To(Equal(sessionID))
			Expect(s.Metadata.Created).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.LastAccessed).To(BeTemporally("~", time.Now(), time.Second))
			Expect(s.Metadata.IsIncognito).To(BeTrue())
			Expect(s.Metadata.IsForkedSession).To(BeFalse())
			Expect(s.Metadata.ParentSession).To(BeEmpty())
		})
	})

	Describe("UpdateLastAccessed", func() {
		It("should update the lastAccessed timestamp", func() {
			s := session.NewSession("test", "uuid")
			originalTime := s.Metadata.LastAccessed

			time.Sleep(10 * time.Millisecond)
			s.UpdateLastAccessed()

			Expect(s.Metadata.LastAccessed).To(BeTemporally(">", originalTime))
			Expect(s.Metadata.LastAccessed).To(BeTemporally("~", time.Now(), time.Second))
		})
	})

	Describe("GetSystemPromptMode", func() {
		It("should return empty string when SystemPromptMode is not set", func() {
			s := session.NewSession("test", "uuid")
			Expect(s.Metadata.SystemPromptMode).To(BeEmpty())
			Expect(s.Metadata.GetSystemPromptMode()).To(BeEmpty())
		})

		It("should return 'append' when SystemPromptMode is set to 'append'", func() {
			s := session.NewSession("test", "uuid")
			s.Metadata.SystemPromptMode = "append"
			Expect(s.Metadata.GetSystemPromptMode()).To(Equal("append"))
		})

		It("should return 'replace' when SystemPromptMode is set to 'replace'", func() {
			s := session.NewSession("test", "uuid")
			s.Metadata.SystemPromptMode = "replace"
			Expect(s.Metadata.GetSystemPromptMode()).To(Equal("replace"))
		})
	})
})
