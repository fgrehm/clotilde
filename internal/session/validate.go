package session

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// MinNameLength is the minimum allowed session name length
	MinNameLength = 2

	// MaxNameLength is the maximum allowed session name length
	MaxNameLength = 64
)

var (
	// sessionNameRegex validates session name format:
	// - Must start and end with alphanumeric (lowercase)
	// - Can contain hyphens in the middle
	// - No consecutive hyphens
	sessionNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

	// ErrInvalidName is returned when session name validation fails
	ErrInvalidName = errors.New("invalid session name")
)

// ValidateName checks if a session name is valid.
// Returns an error if the name is invalid, with details about why.
func ValidateName(name string) error {
	if len(name) < MinNameLength {
		return fmt.Errorf("%w: name must be at least %d characters", ErrInvalidName, MinNameLength)
	}

	if len(name) > MaxNameLength {
		return fmt.Errorf("%w: name must be at most %d characters", ErrInvalidName, MaxNameLength)
	}

	if !sessionNameRegex.MatchString(name) {
		return fmt.Errorf("%w: name must be lowercase alphanumeric with hyphens, starting and ending with alphanumeric", ErrInvalidName)
	}

	// Check for consecutive hyphens
	if strings.Contains(name, "--") {
		return fmt.Errorf("%w: name cannot contain consecutive hyphens", ErrInvalidName)
	}

	return nil
}
