package testutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateFakeClaude creates a fake claude binary in the given directory
// that captures all invocation arguments to a file.
// Returns the path to the binary and the path to the args capture file.
func CreateFakeClaude(dir string) (binaryPath, argsFile string, err error) {
	binaryPath = filepath.Join(dir, "claude")
	argsFile = filepath.Join(dir, "claude-args.txt")

	// Create a shell script that captures arguments
	script := fmt.Sprintf(`#!/bin/bash
echo "$@" > %s
exit 0
`, argsFile)

	if err := os.WriteFile(binaryPath, []byte(script), 0755); err != nil {
		return "", "", err
	}

	return binaryPath, argsFile, nil
}

// ReadClaudeArgs reads the captured arguments from the fake claude invocation.
func ReadClaudeArgs(argsFile string) (string, error) {
	content, err := os.ReadFile(argsFile)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
