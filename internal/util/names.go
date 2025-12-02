package util

import (
	"fmt"
	"math/rand"
	"time"
)

var adjectives = []string{
	"quiet", "swift", "brave", "bright", "clever",
	"gentle", "happy", "jolly", "kind", "lively",
	"merry", "nice", "proud", "silly", "witty",
	"calm", "eager", "fancy", "great", "mighty",
	"quick", "smart", "super", "wise", "zany",
}

var nouns = []string{
	"fox", "bear", "wolf", "hawk", "deer",
	"lion", "tiger", "eagle", "otter", "panda",
	"raven", "shark", "snake", "spider", "whale",
	"cat", "dog", "bird", "fish", "frog",
	"mouse", "owl", "seal", "swan", "bat",
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GenerateRandomName generates a random name in the format "adjective-noun"
func GenerateRandomName() string {
	adjective := adjectives[rng.Intn(len(adjectives))]
	noun := nouns[rng.Intn(len(nouns))]
	return fmt.Sprintf("%s-%s", adjective, noun)
}

// GenerateUniqueRandomName generates a random name that doesn't conflict with existing names
func GenerateUniqueRandomName(existingNames []string) string {
	// Build a map for quick lookups
	nameMap := make(map[string]bool)
	for _, name := range existingNames {
		nameMap[name] = true
	}

	// Try generating unique names
	const maxAttempts = 100
	for i := 0; i < maxAttempts; i++ {
		name := GenerateRandomName()
		if !nameMap[name] {
			return name
		}
	}

	// If we still can't find a unique name, append a random number
	return fmt.Sprintf("%s-%d", GenerateRandomName(), rng.Intn(1000))
}
