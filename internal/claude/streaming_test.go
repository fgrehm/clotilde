package claude_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/claude"
)

// createStreamingFakeClaude creates a fake claude binary that writes the given
// lines to stdout (one per line) and captures args to a file.
func createStreamingFakeClaude(dir string, stdoutLines []string, exitCode int) (binaryPath, argsFile string, err error) {
	binaryPath = filepath.Join(dir, "claude")
	argsFile = filepath.Join(dir, "claude-args.txt")

	// Build echo statements for each line
	var echos string
	for _, line := range stdoutLines {
		echos += fmt.Sprintf("echo '%s'\n", line)
	}

	script := fmt.Sprintf(`#!/bin/bash
echo "$@" > %s
%sexit %d
`, argsFile, echos, exitCode)

	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		return "", "", err
	}

	return binaryPath, argsFile, nil
}

var _ = Describe("InvokeStreaming", func() {
	var (
		tempDir  string
		origFunc func() string
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		origFunc = claude.ClaudeBinaryPathFunc
	})

	AfterEach(func() {
		claude.ClaudeBinaryPathFunc = origFunc
	})

	It("passes correct args including -p, --output-format stream-json, --verbose", func() {
		binPath, argsFile, err := createStreamingFakeClaude(tempDir, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		opts := claude.InvokeOptions{
			SessionID: "test-uuid",
		}
		err = claude.InvokeStreaming(context.Background(), opts, "Hello", func(_ string) {})
		Expect(err).NotTo(HaveOccurred())

		args, err := os.ReadFile(argsFile)
		Expect(err).NotTo(HaveOccurred())
		argsStr := string(args)

		Expect(argsStr).To(ContainSubstring("--session-id test-uuid"))
		Expect(argsStr).To(ContainSubstring("-p Hello"))
		Expect(argsStr).To(ContainSubstring("--output-format stream-json"))
		Expect(argsStr).To(ContainSubstring("--verbose"))
	})

	It("uses --resume when Resume is true", func() {
		binPath, argsFile, err := createStreamingFakeClaude(tempDir, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		opts := claude.InvokeOptions{
			SessionID: "test-uuid",
			Resume:    true,
		}
		err = claude.InvokeStreaming(context.Background(), opts, "Hello", func(_ string) {})
		Expect(err).NotTo(HaveOccurred())

		args, err := os.ReadFile(argsFile)
		Expect(err).NotTo(HaveOccurred())
		argsStr := string(args)

		Expect(argsStr).To(ContainSubstring("--resume test-uuid"))
		Expect(argsStr).NotTo(ContainSubstring("--session-id"))
	})

	It("streams stdout lines to callback one at a time", func() {
		lines := []string{
			`{"type":"system","subtype":"init"}`,
			`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`,
			`{"type":"result","subtype":"success"}`,
		}
		binPath, _, err := createStreamingFakeClaude(tempDir, lines, 0)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		var received []string
		opts := claude.InvokeOptions{SessionID: "test-uuid"}
		err = claude.InvokeStreaming(context.Background(), opts, "test", func(line string) {
			received = append(received, line)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(received).To(HaveLen(3))
		Expect(received[0]).To(ContainSubstring("system"))
		Expect(received[1]).To(ContainSubstring("assistant"))
		Expect(received[2]).To(ContainSubstring("result"))
	})

	It("returns error on non-zero exit code", func() {
		binPath, _, err := createStreamingFakeClaude(tempDir, nil, 1)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		opts := claude.InvokeOptions{SessionID: "test-uuid"}
		err = claude.InvokeStreaming(context.Background(), opts, "test", func(_ string) {})
		Expect(err).To(HaveOccurred())
	})

	It("returns error when binary not found", func() {
		claude.ClaudeBinaryPathFunc = func() string { return "/nonexistent/claude" }

		opts := claude.InvokeOptions{SessionID: "test-uuid"}
		err := claude.InvokeStreaming(context.Background(), opts, "test", func(_ string) {})
		Expect(err).To(HaveOccurred())
	})

	It("includes settings and system prompt file when provided", func() {
		binPath, argsFile, err := createStreamingFakeClaude(tempDir, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		settingsPath := filepath.Join(tempDir, "settings.json")
		Expect(os.WriteFile(settingsPath, []byte("{}"), 0o644)).To(Succeed())

		promptPath := filepath.Join(tempDir, "system-prompt.md")
		Expect(os.WriteFile(promptPath, []byte("prompt"), 0o644)).To(Succeed())

		opts := claude.InvokeOptions{
			SessionID:        "test-uuid",
			SettingsFile:     settingsPath,
			SystemPromptFile: promptPath,
		}
		err = claude.InvokeStreaming(context.Background(), opts, "test", func(_ string) {})
		Expect(err).NotTo(HaveOccurred())

		args, err := os.ReadFile(argsFile)
		Expect(err).NotTo(HaveOccurred())
		argsStr := string(args)

		Expect(argsStr).To(ContainSubstring("--settings " + settingsPath))
		Expect(argsStr).To(ContainSubstring("--append-system-prompt-file " + promptPath))
	})

	It("includes additional args", func() {
		binPath, argsFile, err := createStreamingFakeClaude(tempDir, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		claude.ClaudeBinaryPathFunc = func() string { return binPath }

		opts := claude.InvokeOptions{
			SessionID:      "test-uuid",
			AdditionalArgs: []string{"--model", "haiku"},
		}
		err = claude.InvokeStreaming(context.Background(), opts, "test", func(_ string) {})
		Expect(err).NotTo(HaveOccurred())

		args, err := os.ReadFile(argsFile)
		Expect(err).NotTo(HaveOccurred())
		argsStr := strings.TrimSpace(string(args))

		Expect(argsStr).To(ContainSubstring("--model haiku"))
	})
})
