package config

// Config represents the clotilde configuration.
type Config struct {
	// DefaultModel is the default Claude model to use (e.g., "sonnet", "opus", "haiku")
	DefaultModel string `json:"model,omitempty"`
	// DefaultPermissions are the project-wide default permissions for all sessions
	DefaultPermissions *Permissions `json:"permissions,omitempty"`
}

// Permissions represents the permissions configuration for sessions.
// Kept in config package to avoid circular imports with session package.
type Permissions struct {
	Allow                        []string `json:"allow,omitempty"`
	Ask                          []string `json:"ask,omitempty"`
	Deny                         []string `json:"deny,omitempty"`
	AdditionalDirectories        []string `json:"additionalDirectories,omitempty"`
	DefaultMode                  string   `json:"defaultMode,omitempty"`
	DisableBypassPermissionsMode string   `json:"disableBypassPermissionsMode,omitempty"`
}

// NewConfig creates a new Config with sensible defaults.
func NewConfig() *Config {
	return &Config{
		// Leave DefaultModel empty - will use Claude Code's default
		DefaultModel: "",
	}
}
