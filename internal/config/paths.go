package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fgrehm/clotilde/internal/util"
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
// Note: unlike ProjectRootFromPath, this does NOT stop at $HOME. If .claude/clotilde
// was already created at ~ (e.g., from before the walk-up fix), we still need to find
// it so users can list/delete those sessions.
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
// Stops at $HOME to avoid treating ~/.claude/ (Claude Code's global config) as a project marker.
func ProjectRootFromPath(startPath string) string {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return startPath
	}

	homeDir, err := util.HomeDir()
	if err != nil {
		homeDir = ""
	}

	currentPath := absPath
	for homeDir == "" || currentPath != homeDir {
		claudePath := filepath.Join(currentPath, ".claude")
		info, err := os.Stat(claudePath)
		if err == nil && info.IsDir() {
			return currentPath
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached filesystem root
			break
		}
		currentPath = parentPath
	}

	return absPath
}

// FindOrCreateClotildeRoot finds or creates the .claude/clotilde directory for the
// current project. It resolves the project root first (which stops at $HOME), then
// checks if .claude/clotilde exists there. This avoids the bug where an existing
// ~/.claude/clotilde (from legacy usage) would shadow the correct project-local root.
func FindOrCreateClotildeRoot() (string, error) {
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return "", err
	}

	clotildeRoot := filepath.Join(projectRoot, ClotildeDir)
	if info, statErr := os.Stat(clotildeRoot); statErr == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("%s exists and is not a directory", clotildeRoot)
		}
		return clotildeRoot, nil
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("failed to stat clotilde root %s: %w", clotildeRoot, statErr)
	}

	if err := EnsureSessionsDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create clotilde structure: %w", err)
	}

	return clotildeRoot, nil
}

// IsInitialized checks if clotilde is initialized in the current directory tree.
func IsInitialized() bool {
	_, err := FindClotildeRoot()
	return err == nil
}
