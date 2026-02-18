package util

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultFileMode = 0o644
	DefaultDirMode  = 0o755
)

func ensureDirForFile(filePath string) error {
	return EnsureDir(filepath.Dir(filePath))
}

// EnsureDir creates a directory and all parent directories if they don't exist.
// Returns an error if creation fails.
func EnsureDir(path string) error {
	return os.MkdirAll(path, DefaultDirMode)
}

// FileExists checks if a file exists at the given path.
// Returns true if the file exists, false otherwise.
func FileExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular()
}

// DirExists checks if a directory exists at the given path.
// Returns true if the directory exists, false otherwise.
func DirExists(path string) bool {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return pathInfo.IsDir()
}

// CopyFile copies a file from src to dst.
// Creates parent directories if they don't exist.
// Returns an error if copy fails.
func CopyFile(src, dst string) error {
	if err := ensureDirForFile(dst); err != nil {
		return err
	}

	srcFileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcFileInfo.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}

	bytesRead, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, bytesRead, srcFileInfo.Mode())
}

// ReadJSON reads a JSON file and unmarshals it into the provided interface.
// Returns an error if reading or unmarshaling fails.
func ReadJSON(path string, v interface{}) error {
	data, err := ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteJSON marshals the provided interface to JSON and writes it to a file.
// Creates parent directories if they don't exist.
// Uses indented formatting for readability.
// Returns an error if marshaling or writing fails.
func WriteJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, data)
}

// CountLines counts the number of lines in a file.
// Returns the line count and any error encountered.
func CountLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// HomeDir returns the user's home directory path.
// Returns an error if the home directory cannot be determined.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// ExpandHome expands a path with a leading ~ to the user's home directory.
// Returns the expanded path or an error if home directory cannot be determined.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, path[2:]), nil
}

// WriteFile writes content to a file, creating parent directories if needed.
// Returns an error if writing fails.
func WriteFile(path string, content []byte) error {
	if err := ensureDirForFile(path); err != nil {
		return err
	}
	return os.WriteFile(path, content, DefaultFileMode)
}

// ReadFile reads the entire contents of a file.
// Returns the file contents and any error encountered.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// CopyDir recursively copies a directory from src to dst.
// Returns an error if copy fails.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return errors.New("source is not a directory")
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemoveAll removes a path and all its contents.
// Returns an error if removal fails.
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}
