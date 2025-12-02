package config

// Config represents the clotilde configuration.
type Config struct {
	// DefaultModel is the default Claude model to use (e.g., "sonnet", "opus", "haiku")
	DefaultModel string `json:"defaultModel,omitempty"`
}

// NewConfig creates a new Config with sensible defaults.
func NewConfig() *Config {
	return &Config{
		// Leave DefaultModel empty - will use Claude Code's default
		DefaultModel: "",
	}
}
