package tour_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTour(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tour Suite")
}
