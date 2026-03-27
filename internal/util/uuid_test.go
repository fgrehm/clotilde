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
		Expect(uuid).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
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
