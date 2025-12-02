package util

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// GenerateUUID generates a new UUID v4 string.
// Returns a UUID in the format: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		// This should never happen with crypto/rand, but handle it gracefully
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}

	// Set version (4) and variant bits according to RFC 4122
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16])
}

// IsValidUUID checks if a string is a valid UUID format.
// Returns true if the string matches UUID format, false otherwise.
func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
