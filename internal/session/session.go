package session

import (
	"time"
)

// Session represents a named Claude Code session.
type Session struct {
	Name     string
	Metadata Metadata
}

// Metadata represents the session metadata stored in metadata.json.
type Metadata struct {
	Name                 string    `json:"name"`
	SessionID            string    `json:"sessionId"`
	TranscriptPath       string    `json:"transcriptPath,omitempty"`
	Created              time.Time `json:"created"`
	LastAccessed         time.Time `json:"lastAccessed"`
	ParentSession        string    `json:"parentSession,omitempty"`
	IsForkedSession      bool      `json:"isForkedSession"`
	IsIncognito          bool      `json:"isIncognito"`
	PreviousSessionIDs   []string  `json:"previousSessionIds,omitempty"`
	Context              string    `json:"context,omitempty"`
	SystemPromptMode     string    `json:"systemPromptMode,omitempty"` // "append" (default) or "replace"
	HasCustomOutputStyle bool      `json:"hasCustomOutputStyle,omitempty"`
}

// Settings represents Claude Code session-specific settings stored in settings.json.
type Settings struct {
	Model       string      `json:"model,omitempty"`
	OutputStyle string      `json:"outputStyle,omitempty"`
	Permissions Permissions `json:"permissions,omitempty"`
}

// Permissions represents the permissions configuration for a session.
type Permissions struct {
	Allow                        []string `json:"allow,omitempty"`
	Ask                          []string `json:"ask,omitempty"`
	Deny                         []string `json:"deny,omitempty"`
	AdditionalDirectories        []string `json:"additionalDirectories,omitempty"`
	DefaultMode                  string   `json:"defaultMode,omitempty"`
	DisableBypassPermissionsMode string   `json:"disableBypassPermissionsMode,omitempty"`
}

// NewSession creates a new session with the given name and UUID.
func NewSession(name, sessionID string) *Session {
	now := time.Now()
	return &Session{
		Name: name,
		Metadata: Metadata{
			Name:            name,
			SessionID:       sessionID,
			Created:         now,
			LastAccessed:    now,
			IsForkedSession: false,
		},
	}
}

// NewForkedSession creates a new forked session with empty sessionId.
// The sessionId will be filled in later by the session-start hook.
func NewForkedSession(name, parentName string) *Session {
	now := time.Now()
	return &Session{
		Name: name,
		Metadata: Metadata{
			Name:            name,
			SessionID:       "", // Will be filled by hook
			Created:         now,
			LastAccessed:    now,
			ParentSession:   parentName,
			IsForkedSession: true,
		},
	}
}

// NewIncognitoSession creates a new incognito session that will auto-delete on exit.
func NewIncognitoSession(name, sessionID string) *Session {
	sess := NewSession(name, sessionID)
	sess.Metadata.IsIncognito = true
	return sess
}

// UpdateLastAccessed updates the lastAccessed timestamp to now.
func (s *Session) UpdateLastAccessed() {
	s.Metadata.LastAccessed = time.Now()
}

// AddPreviousSessionID appends the current session ID to the history and updates to the new ID.
// This is idempotent - won't add duplicates.
func (s *Session) AddPreviousSessionID(newSessionID string) {
	// Only add current ID to history if it's not empty and different from new ID
	if s.Metadata.SessionID != "" && s.Metadata.SessionID != newSessionID {
		// Check if already in history (idempotent)
		found := false
		for _, id := range s.Metadata.PreviousSessionIDs {
			if id == s.Metadata.SessionID {
				found = true
				break
			}
		}
		if !found {
			s.Metadata.PreviousSessionIDs = append(s.Metadata.PreviousSessionIDs, s.Metadata.SessionID)
		}
	}

	// Always update to new session ID, even if current is empty
	// (handles forks that haven't had registerFork called yet)
	s.Metadata.SessionID = newSessionID
}

// GetSystemPromptMode returns the system prompt mode ("append" or "replace").
// Returns empty string if not set.
func (m *Metadata) GetSystemPromptMode() string {
	return m.SystemPromptMode
}
