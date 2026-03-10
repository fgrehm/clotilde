package notify_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/notify"
)

var _ = Describe("EmojiForEvent", func() {
	It("should return check mark for Stop", func() {
		Expect(notify.EmojiForEvent("Stop", nil)).To(Equal("\u2705"))
	})

	It("should return warning for permission_prompt notification", func() {
		data := map[string]interface{}{"notification_type": "permission_prompt"}
		Expect(notify.EmojiForEvent("Notification", data)).To(Equal("\u26a0\ufe0f"))
	})

	It("should return zzz for idle_prompt notification", func() {
		data := map[string]interface{}{"notification_type": "idle_prompt"}
		Expect(notify.EmojiForEvent("Notification", data)).To(Equal("\U0001f4a4"))
	})

	It("should return question mark for unknown notification type", func() {
		data := map[string]interface{}{"notification_type": "something_new"}
		Expect(notify.EmojiForEvent("Notification", data)).To(Equal("\u2753"))
	})

	It("should return question mark when notification_type is missing", func() {
		Expect(notify.EmojiForEvent("Notification", nil)).To(Equal("\u2753"))
	})

	It("should return thinking face for PostToolUse", func() {
		Expect(notify.EmojiForEvent("PostToolUse", nil)).To(Equal("\U0001f914"))
	})

	It("should return empty string for SessionEnd", func() {
		Expect(notify.EmojiForEvent("SessionEnd", nil)).To(Equal(""))
	})

	It("should return blue circle for unknown events", func() {
		Expect(notify.EmojiForEvent("SomeFutureEvent", nil)).To(Equal("\U0001f535"))
	})

	DescribeTable("PreToolUse tool emojis",
		func(toolName, expectedEmoji string) {
			data := map[string]interface{}{"tool_name": toolName}
			Expect(notify.EmojiForEvent("PreToolUse", data)).To(Equal(expectedEmoji))
		},
		Entry("Bash", "Bash", "\u26a1"),
		Entry("Read", "Read", "\U0001f4d6"),
		Entry("Grep", "Grep", "\U0001f4d6"),
		Entry("Glob", "Glob", "\U0001f4d6"),
		Entry("ToolSearch", "ToolSearch", "\U0001f4d6"),
		Entry("Write", "Write", "\u270f\ufe0f"),
		Entry("Edit", "Edit", "\u270f\ufe0f"),
		Entry("MultiEdit", "MultiEdit", "\u270f\ufe0f"),
		Entry("Task", "Task", "\U0001f500"),
		Entry("WebSearch", "WebSearch", "\U0001f310"),
		Entry("WebFetch", "WebFetch", "\U0001f310"),
		Entry("unknown tool", "SomePlugin", "\u2699\ufe0f"),
	)

	It("should return gear for PreToolUse with missing tool_name", func() {
		Expect(notify.EmojiForEvent("PreToolUse", nil)).To(Equal("\u2699\ufe0f"))
	})
})
