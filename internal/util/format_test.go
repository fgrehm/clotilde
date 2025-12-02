package util_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("FormatSize", func() {
	It("should format bytes", func() {
		Expect(util.FormatSize(512)).To(Equal("512 B"))
		Expect(util.FormatSize(100)).To(Equal("100 B"))
	})

	It("should format kilobytes", func() {
		Expect(util.FormatSize(1024)).To(Equal("1.0 KB"))
		Expect(util.FormatSize(1536)).To(Equal("1.5 KB"))
		Expect(util.FormatSize(10240)).To(Equal("10.0 KB"))
	})

	It("should format megabytes", func() {
		Expect(util.FormatSize(1048576)).To(Equal("1.0 MB"))
		Expect(util.FormatSize(1572864)).To(Equal("1.5 MB"))
		Expect(util.FormatSize(10485760)).To(Equal("10.0 MB"))
	})

	It("should format gigabytes", func() {
		Expect(util.FormatSize(1073741824)).To(Equal("1.0 GB"))
		Expect(util.FormatSize(1610612736)).To(Equal("1.5 GB"))
	})

	It("should format terabytes", func() {
		Expect(util.FormatSize(1099511627776)).To(Equal("1.0 TB"))
		Expect(util.FormatSize(1649267441664)).To(Equal("1.5 TB"))
	})
})

var _ = Describe("TruncateText", func() {
	It("should replace newlines with spaces", func() {
		text := "Hello\nWorld\nFoo"
		result := util.TruncateText(text, 100)
		Expect(result).To(Equal("Hello World Foo"))
	})

	It("should collapse multiple spaces", func() {
		text := "Hello    World   \n  Foo"
		result := util.TruncateText(text, 100)
		Expect(result).To(Equal("Hello World Foo"))
	})

	It("should truncate text longer than maxChars", func() {
		text := "This is a very long text that needs to be truncated"
		result := util.TruncateText(text, 20)
		Expect(result).To(Equal("This is a very lo..."))
	})

	It("should not truncate text shorter than maxChars", func() {
		text := "Short text"
		result := util.TruncateText(text, 20)
		Expect(result).To(Equal("Short text"))
	})

	It("should handle text exactly at maxChars", func() {
		text := "Exactly twenty chars"
		result := util.TruncateText(text, 20)
		Expect(result).To(Equal("Exactly twenty chars"))
	})

	It("should handle multiline text with truncation", func() {
		text := "Line one\nLine two\nLine three\nLine four\nLine five"
		result := util.TruncateText(text, 30)
		Expect(result).To(Equal("Line one Line two Line thre..."))
	})

	It("should handle carriage returns", func() {
		text := "Hello\r\nWorld\r\nFoo"
		result := util.TruncateText(text, 100)
		Expect(result).To(Equal("Hello World Foo"))
	})

	It("should handle empty text", func() {
		text := ""
		result := util.TruncateText(text, 10)
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("FormatRelativeTime", func() {
	It("should return 'just now' for times less than a minute ago", func() {
		now := time.Now()
		Expect(util.FormatRelativeTime(now)).To(Equal("just now"))
		Expect(util.FormatRelativeTime(now.Add(-30 * time.Second))).To(Equal("just now"))
		Expect(util.FormatRelativeTime(now.Add(-59 * time.Second))).To(Equal("just now"))
	})

	It("should return '1 minute ago' for times exactly 1 minute ago", func() {
		t := time.Now().Add(-1 * time.Minute)
		Expect(util.FormatRelativeTime(t)).To(Equal("1 minute ago"))
	})

	It("should return 'X minutes ago' for times 2-59 minutes ago", func() {
		Expect(util.FormatRelativeTime(time.Now().Add(-2 * time.Minute))).To(Equal("2 minutes ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-5 * time.Minute))).To(Equal("5 minutes ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-30 * time.Minute))).To(Equal("30 minutes ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-59 * time.Minute))).To(Equal("59 minutes ago"))
	})

	It("should return '1 hour ago' for times exactly 1 hour ago", func() {
		t := time.Now().Add(-1 * time.Hour)
		Expect(util.FormatRelativeTime(t)).To(Equal("1 hour ago"))
	})

	It("should return 'X hours ago' for times 2-23 hours ago", func() {
		Expect(util.FormatRelativeTime(time.Now().Add(-2 * time.Hour))).To(Equal("2 hours ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-12 * time.Hour))).To(Equal("12 hours ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-23 * time.Hour))).To(Equal("23 hours ago"))
	})

	It("should return '1 day ago' for times exactly 1 day ago", func() {
		t := time.Now().Add(-24 * time.Hour)
		Expect(util.FormatRelativeTime(t)).To(Equal("1 day ago"))
	})

	It("should return 'X days ago' for times 2-6 days ago", func() {
		Expect(util.FormatRelativeTime(time.Now().Add(-2 * 24 * time.Hour))).To(Equal("2 days ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-3 * 24 * time.Hour))).To(Equal("3 days ago"))
		Expect(util.FormatRelativeTime(time.Now().Add(-6 * 24 * time.Hour))).To(Equal("6 days ago"))
	})

	It("should return date format for times 7+ days ago", func() {
		// 7 days ago
		t := time.Now().Add(-7 * 24 * time.Hour)
		result := util.FormatRelativeTime(t)
		Expect(result).To(MatchRegexp(`^\d{4}-\d{2}-\d{2}$`))

		// Verify the date is correct
		expectedDate := t.Format("2006-01-02")
		Expect(result).To(Equal(expectedDate))
	})

	It("should return date format for times months ago", func() {
		t := time.Now().Add(-90 * 24 * time.Hour)
		result := util.FormatRelativeTime(t)
		Expect(result).To(MatchRegexp(`^\d{4}-\d{2}-\d{2}$`))
	})
})
