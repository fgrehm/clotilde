package util

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// GitBranchFunc returns the current git branch name.
// Returns empty string if not in a git repo, on a detached HEAD, or if git is unavailable.
// Can be overridden in tests.
var GitBranchFunc = defaultGitBranch

func defaultGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

var (
	sanitizeReplacer        = strings.NewReplacer("/", "-", "_", "-", ".", "-")
	sanitizeInvalidChars    = regexp.MustCompile(`[^a-z0-9-]`)
	sanitizeMultipleHyphens = regexp.MustCompile(`-{2,}`)
)

// SanitizeBranchName converts a git branch name into a valid session name.
// Returns empty string if the result is too short to be a valid session name.
func SanitizeBranchName(branch string) string {
	s := strings.ToLower(branch)
	s = sanitizeReplacer.Replace(s)
	s = sanitizeInvalidChars.ReplaceAllString(s, "")
	s = sanitizeMultipleHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	// Truncate to 62 chars, leaving room for a "-N" conflict suffix (max total: 64)
	const maxBase = 62
	if len(s) > maxBase {
		s = strings.TrimRight(s[:maxBase], "-")
	}

	return s
}

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

// GenerateRandomName generates a random name in the format "YYYY-MM-DD-noun"
func GenerateRandomName() string {
	date := time.Now().Format("2006-01-02")
	adjective := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return fmt.Sprintf("%s-%s-%s", date, adjective, noun)
}

// GenerateUniqueRandomName generates a unique session name.
// If the current directory is a git repo on a non-main branch, the branch name is used.
// Otherwise a random adjective-noun name with a date prefix is generated.
func GenerateUniqueRandomName(existingNames []string) string {
	nameMap := make(map[string]struct{})
	for _, name := range existingNames {
		nameMap[name] = struct{}{}
	}

	// Try branch-based name first; skip trunk/detached-HEAD branches.
	branch := GitBranchFunc()
	switch branch {
	case "", "main", "master", "HEAD":
		// not on a meaningful branch — fall through to random name
	default:
		sanitized := SanitizeBranchName(branch)
		if len(sanitized) >= 2 {
			if _, taken := nameMap[sanitized]; !taken {
				return sanitized
			}
			// Branch name taken; try appending a number suffix
			for i := 2; i <= 9; i++ {
				candidate := fmt.Sprintf("%s-%d", sanitized, i)
				if _, taken := nameMap[candidate]; !taken {
					return candidate
				}
			}
		}
	}

	// Fall back to a random adjective-noun name
	const maxAttempts = 100
	for i := 0; i < maxAttempts; i++ {
		name := GenerateRandomName()
		if _, taken := nameMap[name]; !taken {
			return name
		}
	}

	return fmt.Sprintf("%s-%d", GenerateRandomName(), rand.IntN(1000))
}
