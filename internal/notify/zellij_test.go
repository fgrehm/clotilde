package notify_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/notify"
)

// fakeTabRenamer records RenameTab calls for testing.
type fakeTabRenamer struct {
	calls []string
}

func (f *fakeTabRenamer) RenameTab(name string) error {
	f.calls = append(f.calls, name)
	return nil
}

var _ notify.TabRenamer = (*fakeTabRenamer)(nil)

var _ = Describe("ZellijTabRenamer", func() {
	It("should implement TabRenamer interface", func() {
		var _ notify.TabRenamer = &notify.ZellijTabRenamer{}
	})
})

var _ = Describe("TabRenamer (fake)", func() {
	It("should record calls", func() {
		fake := &fakeTabRenamer{}
		err := fake.RenameTab("\u2705 my-session")
		Expect(err).NotTo(HaveOccurred())
		Expect(fake.calls).To(Equal([]string{"\u2705 my-session"}))
	})
})
