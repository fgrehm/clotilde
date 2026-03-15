package config

import (
	"errors"
	"fmt"
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

// ProjectRoot returns the project root directory from the clotilde root path.
// The clotilde root is .claude/clotilde, so the project root is two levels up.
func ProjectRoot(clotildeRoot string) string {
	return filepath.Dir(filepath.Dir(clotildeRoot))
}

// GetConfigPath returns the path to the config.json file.
func GetConfigPath(clotildeRoot string) string {
	return filepath.Join(clotildeRoot, ConfigFile)
}

// GlobalConfigPath returns the path to the global config file.
// Respects $XDG_CONFIG_HOME if set, otherwise uses ~/.config/clotilde/config.json.
func GlobalConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "clotilde", ConfigFile)
}

// EnsureClotildeStructure creates the .claude/clotilde directory structure
// at the given path if it doesn't exist.
func EnsureClotildeStructure(projectRoot string) error {
	return EnsureSessionsDir(projectRoot)
}

// EnsureSessionsDir creates .claude/clotilde/sessions/ at the given project root.
// Creates all parent directories as needed.
func EnsureSessionsDir(projectRoot string) error {
	sessionsPath := filepath.Join(projectRoot, ClotildeDir, SessionsDir)
	return os.MkdirAll(sessionsPath, 0o755)
}

// FindProjectRoot determines the project root directory.
// Walks up from cwd looking for a .claude/ directory (Claude Code's project marker).
// If found, returns its parent. If not found, returns cwd.
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return ProjectRootFromPath(cwd), nil
}

// ProjectRootFromPath determines the project root starting from the given path.
// Walks up looking for a .claude/ directory. If found, returns its parent.
// If not found, returns the starting path.
func ProjectRootFromPath(startPath string) string {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return startPath
	}

	currentPath := absPath
	for {
		claudePath := filepath.Join(currentPath, ".claude")
		info, err := os.Stat(claudePath)
		if err == nil && info.IsDir() {
			return currentPath
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached filesystem root, use original path
			return absPath
		}
		currentPath = parentPath
	}
}

// FindOrCreateClotildeRoot finds an existing .claude/clotilde directory or creates one.
// First tries FindClotildeRoot(). If not found, determines the project root,
// creates the sessions directory, and returns the new clotilde root path.
func FindOrCreateClotildeRoot() (string, error) {
	if root, err := FindClotildeRoot(); err == nil {
		return root, nil
	}

	projectRoot, err := FindProjectRoot()
	if err != nil {
		return "", err
	}

	if err := EnsureSessionsDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create clotilde structure: %w", err)
	}

	return filepath.Join(projectRoot, ClotildeDir), nil
}

// IsInitialized checks if clotilde is initialized in the current directory tree.
func IsInitialized() bool {
	_, err := FindClotildeRoot()
	return err == nil
}
