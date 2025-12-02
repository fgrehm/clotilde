package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/util"
)

const (
	metadataFile     = "metadata.json"
	settingsFile     = "settings.json"
	systemPromptFile = "system-prompt.md"
	contextFile      = "context.md"
)

// Store defines the interface for session storage operations.
type Store interface {
	// List returns all sessions, sorted by lastAccessed (most recent first)
	List() ([]*Session, error)

	// Get retrieves a session by name
	Get(name string) (*Session, error)

	// Create creates a new session folder structure with metadata
	Create(session *Session) error

	// Update updates session metadata
	Update(session *Session) error

	// Delete removes a session folder and all its contents
	Delete(name string) error

	// Exists checks if a session exists
	Exists(name string) bool

	// LoadSettings loads settings.json for a session (returns nil if not exists)
	LoadSettings(name string) (*Settings, error)

	// SaveSettings saves settings.json for a session
	SaveSettings(name string, settings *Settings) error

	// LoadSystemPrompt loads system-prompt.md for a session (returns empty if not exists)
	LoadSystemPrompt(name string) (string, error)

	// SaveSystemPrompt saves system-prompt.md for a session
	SaveSystemPrompt(name string, content string) error

	// LoadContext loads context.md for a session (returns empty if not exists)
	LoadContext(name string) (string, error)

	// SaveContext saves context.md for a session
	SaveContext(name string, content string) error
}

// FileStore implements Store using the filesystem.
type FileStore struct {
	clotildeRoot string
}

// NewFileStore creates a new FileStore.
func NewFileStore(clotildeRoot string) *FileStore {
	return &FileStore{
		clotildeRoot: clotildeRoot,
	}
}

// List returns all sessions, sorted by lastAccessed (most recent first).
func (fs *FileStore) List() ([]*Session, error) {
	sessionsDir := config.GetSessionsDir(fs.clotildeRoot)

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, err
	}

	var sessions []*Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		session, err := fs.Get(entry.Name())
		if err != nil {
			// Skip sessions that can't be loaded
			continue
		}
		sessions = append(sessions, session)
	}

	// Sort by lastAccessed (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Metadata.LastAccessed.After(sessions[j].Metadata.LastAccessed)
	})

	return sessions, nil
}

// Get retrieves a session by name.
func (fs *FileStore) Get(name string) (*Session, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	if !util.DirExists(sessionDir) {
		return nil, fmt.Errorf("session '%s' not found", name)
	}

	metadataPath := filepath.Join(sessionDir, metadataFile)
	var metadata Metadata
	if err := util.ReadJSON(metadataPath, &metadata); err != nil {
		return nil, fmt.Errorf("failed to read session metadata: %w", err)
	}

	return &Session{
		Name:     name,
		Metadata: metadata,
	}, nil
}

// Create creates a new session folder structure with metadata.
func (fs *FileStore) Create(session *Session) error {
	if err := ValidateName(session.Name); err != nil {
		return err
	}

	if fs.Exists(session.Name) {
		return fmt.Errorf("session '%s' already exists", session.Name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, session.Name)
	if err := util.EnsureDir(sessionDir); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	metadataPath := filepath.Join(sessionDir, metadataFile)
	if err := util.WriteJSON(metadataPath, session.Metadata); err != nil {
		return fmt.Errorf("failed to write session metadata: %w", err)
	}

	return nil
}

// Update updates session metadata.
func (fs *FileStore) Update(session *Session) error {
	if err := ValidateName(session.Name); err != nil {
		return err
	}

	if !fs.Exists(session.Name) {
		return fmt.Errorf("session '%s' not found", session.Name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, session.Name)
	metadataPath := filepath.Join(sessionDir, metadataFile)
	if err := util.WriteJSON(metadataPath, session.Metadata); err != nil {
		return fmt.Errorf("failed to update session metadata: %w", err)
	}

	return nil
}

// Delete removes a session folder and all its contents.
func (fs *FileStore) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if !fs.Exists(name) {
		return fmt.Errorf("session '%s' not found", name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	return util.RemoveAll(sessionDir)
}

// Exists checks if a session exists.
func (fs *FileStore) Exists(name string) bool {
	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	return util.DirExists(sessionDir)
}

// LoadSettings loads settings.json for a session (returns nil if not exists).
func (fs *FileStore) LoadSettings(name string) (*Settings, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	settingsPath := filepath.Join(sessionDir, settingsFile)

	if !util.FileExists(settingsPath) {
		return nil, nil
	}

	var settings Settings
	if err := util.ReadJSON(settingsPath, &settings); err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	return &settings, nil
}

// SaveSettings saves settings.json for a session.
func (fs *FileStore) SaveSettings(name string, settings *Settings) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if !fs.Exists(name) {
		return fmt.Errorf("session '%s' not found", name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	settingsPath := filepath.Join(sessionDir, settingsFile)

	return util.WriteJSON(settingsPath, settings)
}

// LoadSystemPrompt loads system-prompt.md for a session (returns empty if not exists).
func (fs *FileStore) LoadSystemPrompt(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	promptPath := filepath.Join(sessionDir, systemPromptFile)

	if !util.FileExists(promptPath) {
		return "", nil
	}

	content, err := util.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt: %w", err)
	}

	return string(content), nil
}

// SaveSystemPrompt saves system-prompt.md for a session.
func (fs *FileStore) SaveSystemPrompt(name string, content string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if !fs.Exists(name) {
		return fmt.Errorf("session '%s' not found", name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	promptPath := filepath.Join(sessionDir, systemPromptFile)

	return util.WriteFile(promptPath, []byte(content))
}

// LoadContext loads context.md for a session (returns empty if not exists).
func (fs *FileStore) LoadContext(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	contextPath := filepath.Join(sessionDir, contextFile)

	if !util.FileExists(contextPath) {
		return "", nil
	}

	content, err := util.ReadFile(contextPath)
	if err != nil {
		return "", fmt.Errorf("failed to read context: %w", err)
	}

	return string(content), nil
}

// SaveContext saves context.md for a session.
func (fs *FileStore) SaveContext(name string, content string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if !fs.Exists(name) {
		return fmt.Errorf("session '%s' not found", name)
	}

	sessionDir := config.GetSessionDir(fs.clotildeRoot, name)
	contextPath := filepath.Join(sessionDir, contextFile)

	return util.WriteFile(contextPath, []byte(content))
}

var _ = errors.New // Import marker to ensure errors is imported
