package config

// Config represents the clotilde configuration.
type Config struct {
	// Profiles is a map of named session profiles
	Profiles map[string]Profile `json:"profiles,omitempty"`
}

// Profile represents a named preset of session settings.
type Profile struct {
	Model          string       `json:"model,omitempty"`
	PermissionMode string       `json:"permissionMode,omitempty"`
	Permissions    *Permissions `json:"permissions,omitempty"`
	OutputStyle    string       `json:"outputStyle,omitempty"`
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
		Profiles: make(map[string]Profile),
	}
}
