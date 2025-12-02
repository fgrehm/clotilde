package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("GenerateUUID", func() {
	It("should generate a valid UUID v4 string", func() {
		uuid := util.GenerateUUID()
		Expect(uuid).NotTo(BeEmpty())
		Expect(util.IsValidUUID(uuid)).To(BeTrue())
	})

	It("should generate unique UUIDs", func() {
		uuid1 := util.GenerateUUID()
		uuid2 := util.GenerateUUID()
		Expect(uuid1).NotTo(Equal(uuid2))
	})

	It("should generate UUIDs in the correct format", func() {
		uuid := util.GenerateUUID()
		// Format: 8-4-4-4-12 (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
		Expect(uuid).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
	})
})

var _ = Describe("IsValidUUID", func() {
	It("should return true for valid UUIDs", func() {
		validUUIDs := []string{
			"550e8400-e29b-41d4-a716-446655440000",
			"123e4567-e89b-12d3-a456-426614174000",
			"00000000-0000-0000-0000-000000000000",
		}
		for _, uuid := range validUUIDs {
			Expect(util.IsValidUUID(uuid)).To(BeTrue(), "UUID %s should be valid", uuid)
		}
	})

	It("should return false for invalid UUIDs", func() {
		invalidUUIDs := []string{
			"",
			"not-a-uuid",
			"550e8400-e29b-41d4-a716", // too short
			"550e8400-e29b-41d4-a716-446655440000-extra", // too long
			"550e8400e29b41d4a716446655440000",           // no dashes
			"550e8400-e29b-41d4-a716-44665544000g",       // invalid character
			"XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",       // invalid hex
		}
		for _, uuid := range invalidUUIDs {
			Expect(util.IsValidUUID(uuid)).To(BeFalse(), "UUID %s should be invalid", uuid)
		}
	})
})
