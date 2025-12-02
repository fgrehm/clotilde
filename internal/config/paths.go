package config

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	// ClotildeDir is the directory name for clotilde within .claude/
	ClotildeDir = ".claude/clotilde"

	// SessionsDir is the subdirectory for sessions
	SessionsDir = "sessions"

	// ConfigFile is the config file name
	ConfigFile = "config.json"

	// GlobalContextFile is the global context file name
	GlobalContextFile = "context.md"
)

// FindClotildeRoot searches for the .claude/clotilde directory by walking up
// from the current working directory. Returns the absolute path to the
// .claude/clotilde directory, or an error if not found.
func FindClotildeRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return ClotildeRootFromPath(cwd)
}

// ClotildeRootFromPath searches for .claude/clotilde starting from the given path.
// Returns the absolute path to the .claude/clotilde directory, or an error if not found.
func ClotildeRootFromPath(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	currentPath := absPath
	for {
		clotildePath := filepath.Join(currentPath, ClotildeDir)
		info, err := os.Stat(clotildePath)
		if err == nil && info.IsDir() {
			return clotildePath, nil
		}

		// Move up to parent directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached filesystem root
			return "", errors.New(".claude/clotilde not found in directory tree")
		}
		currentPath = parentPath
	}
}

// GetSessionsDir returns the path to the sessions directory within the clotilde root.
func GetSessionsDir(clotildeRoot string) string {
	return filepath.Join(clotildeRoot, SessionsDir)
}

// GetSessionDir returns the path to a specific session directory.
func GetSessionDir(clotildeRoot, sessionName string) string {
	return filepath.Join(GetSessionsDir(clotildeRoot), sessionName)
}

// GetConfigPath returns the path to the config.json file.
func GetConfigPath(clotildeRoot string) string {
	return filepath.Join(clotildeRoot, ConfigFile)
}

// GetGlobalContextPath returns the path to the global context.md file.
func GetGlobalContextPath(clotildeRoot string) string {
	return filepath.Join(clotildeRoot, GlobalContextFile)
}

// EnsureClotildeStructure creates the .claude/clotilde directory structure
// at the given path if it doesn't exist.
func EnsureClotildeStructure(projectRoot string) error {
	clotildePath := filepath.Join(projectRoot, ClotildeDir)

	// Create .claude/clotilde/ directory
	if err := os.MkdirAll(clotildePath, 0755); err != nil {
		return err
	}

	// Create sessions/ subdirectory
	sessionsPath := filepath.Join(clotildePath, SessionsDir)
	if err := os.MkdirAll(sessionsPath, 0755); err != nil {
		return err
	}

	// Create config.json if it doesn't exist
	configPath := filepath.Join(clotildePath, ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := NewConfig()
		if err := Save(clotildePath, cfg); err != nil {
			return err
		}
	}

	return nil
}

// IsInitialized checks if clotilde is initialized in the current directory tree.
func IsInitialized() bool {
	_, err := FindClotildeRoot()
	return err == nil
}
