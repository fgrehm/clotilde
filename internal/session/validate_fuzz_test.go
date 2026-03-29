package session_test

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/fgrehm/clotilde/internal/session"
)

func FuzzValidateName(f *testing.F) {
	// Seed corpus with valid and invalid names
	seeds := []string{
		"ab", "test", "my-session", "bug-123",
		"a", "", "INVALID", "-test", "test-",
		"test--session", "test_session", "test session",
		"a123456789012345678901234567890123456789012345678901234567890123",
		"a1234567890123456789012345678901234567890123456789012345678901234",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, name string) {
		err := session.ValidateName(name)

		if err == nil {
			// Valid names must satisfy all invariants
			if len(name) < session.MinNameLength {
				t.Errorf("accepted name shorter than MinNameLength: %q", name)
			}
			if len(name) > session.MaxNameLength {
				t.Errorf("accepted name longer than MaxNameLength: %q", name)
			}
			if !utf8.ValidString(name) {
				t.Errorf("accepted non-UTF8 name: %q", name)
			}
			for _, r := range name {
				isLower := r >= 'a' && r <= 'z'
				isDigit := r >= '0' && r <= '9'
				if !isLower && !isDigit && r != '-' {
					t.Errorf("accepted name with invalid character %q: %q", r, name)
				}
			}
			if name[0] == '-' || name[len(name)-1] == '-' {
				t.Errorf("accepted name starting or ending with hyphen: %q", name)
			}
			for i := range len(name) - 1 {
				if name[i] == '-' && name[i+1] == '-' {
					t.Errorf("accepted name with consecutive hyphens: %q", name)
				}
			}
		} else if !errors.Is(err, session.ErrInvalidName) {
			t.Errorf("error does not wrap ErrInvalidName: %v", err)
		}
	})
}
