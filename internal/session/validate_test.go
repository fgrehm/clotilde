package session_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/session"
)

var _ = Describe("ValidateName", func() {
	Context("valid names", func() {
		It("should accept simple lowercase names", func() {
			validNames := []string{
				"ab",
				"test",
				"feature",
				"my-session",
				"bug-123",
				"feature-auth-v2",
			}

			for _, name := range validNames {
				err := session.ValidateName(name)
				Expect(err).NotTo(HaveOccurred(), "name %s should be valid", name)
			}
		})

		It("should accept names with numbers", func() {
			validNames := []string{
				"test123",
				"123test",
				"v1-alpha",
				"feature-v2-123",
			}

			for _, name := range validNames {
				err := session.ValidateName(name)
				Expect(err).NotTo(HaveOccurred(), "name %s should be valid", name)
			}
		})

		It("should accept maximum length names", func() {
			name := "a123456789012345678901234567890123456789012345678901234567890123"
			Expect(len(name)).To(Equal(64))
			err := session.ValidateName(name)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("invalid names", func() {
		It("should reject names that are too short", func() {
			err := session.ValidateName("a")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at least 2 characters"))
		})

		It("should reject names that are too long", func() {
			name := "a1234567890123456789012345678901234567890123456789012345678901234"
			Expect(len(name)).To(Equal(65))
			err := session.ValidateName(name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at most 64 characters"))
		})

		It("should reject names with uppercase letters", func() {
			invalidNames := []string{
				"MySession",
				"FEATURE",
				"Test-Session",
			}

			for _, name := range invalidNames {
				err := session.ValidateName(name)
				Expect(err).To(HaveOccurred(), "name %s should be invalid", name)
			}
		})

		It("should reject names starting or ending with hyphen", func() {
			invalidNames := []string{
				"-test",
				"test-",
				"-",
				"--",
			}

			for _, name := range invalidNames {
				err := session.ValidateName(name)
				Expect(err).To(HaveOccurred(), "name %s should be invalid", name)
			}
		})

		It("should reject names with consecutive hyphens", func() {
			invalidNames := []string{
				"test--session",
				"my--feature",
				"a--b",
			}

			for _, name := range invalidNames {
				err := session.ValidateName(name)
				Expect(err).To(HaveOccurred(), "name %s should be invalid", name)
				Expect(err.Error()).To(ContainSubstring("consecutive hyphens"))
			}
		})

		It("should reject names with special characters", func() {
			invalidNames := []string{
				"test_session",
				"my.session",
				"session@123",
				"test session",
				"test/session",
			}

			for _, name := range invalidNames {
				err := session.ValidateName(name)
				Expect(err).To(HaveOccurred(), "name %s should be invalid", name)
			}
		})
	})
})
