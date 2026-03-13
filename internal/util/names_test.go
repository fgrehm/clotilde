package util

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// setGitBranch overrides GitBranchFunc for the duration of the test and restores it on cleanup.
// Pass "" to disable branch detection and force the random-name fallback.
func setGitBranch(t *testing.T, branch string) {
	t.Helper()
	orig := GitBranchFunc
	GitBranchFunc = func() string { return branch }
	t.Cleanup(func() { GitBranchFunc = orig })
}

func TestGenerateRandomName(t *testing.T) {
	name := GenerateRandomName()

	// Should be in format "YYYY-MM-DD-adjective-noun"
	datePrefix := time.Now().Format("2006-01-02")
	if !strings.HasPrefix(name, datePrefix+"-") {
		t.Errorf("Expected name to start with '%s-', got '%s'", datePrefix, name)
	}

	// Extract adjective-noun suffix
	suffix := strings.TrimPrefix(name, datePrefix+"-")
	parts := strings.Split(suffix, "-")
	if len(parts) != 2 {
		t.Errorf("Expected adjective-noun suffix, got '%s'", suffix)
	}

	// Should contain valid adjective
	found := false
	for _, adj := range adjectives {
		if adj == parts[0] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Generated name '%s' has invalid adjective '%s'", name, parts[0])
	}

	// Should contain valid noun
	found = false
	for _, n := range nouns {
		if n == parts[1] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Generated name '%s' has invalid noun '%s'", name, parts[1])
	}

	// Full format check: YYYY-MM-DD-adjective-noun
	pattern := `^\d{4}-\d{2}-\d{2}-[a-z]+-[a-z]+$`
	if !regexp.MustCompile(pattern).MatchString(name) {
		t.Errorf("Expected name matching '%s', got '%s'", pattern, name)
	}
}

func TestGenerateRandomName_Variety(t *testing.T) {
	// Generate 50 names and ensure we get some variety
	names := make(map[string]bool)
	for i := 0; i < 50; i++ {
		name := GenerateRandomName()
		names[name] = true
	}

	// Should have at least 10 unique names (very conservative check)
	if len(names) < 10 {
		t.Errorf("Expected variety in generated names, got only %d unique names out of 50", len(names))
	}
}

func TestGenerateUniqueRandomName(t *testing.T) {
	setGitBranch(t, "")
	datePrefix := time.Now().Format("2006-01-02")
	existing := []string{
		datePrefix + "-happy-fox",
		datePrefix + "-brave-wolf",
		datePrefix + "-clever-bear",
	}

	name := GenerateUniqueRandomName(existing)

	// Should not match any existing name
	for _, existingName := range existing {
		if name == existingName {
			t.Errorf("Generated name '%s' conflicts with existing name '%s'", name, existingName)
		}
	}

	// Should start with date prefix
	if !strings.HasPrefix(name, datePrefix+"-") {
		t.Errorf("Expected name to start with '%s-', got '%s'", datePrefix, name)
	}
}

func TestGenerateUniqueRandomName_FallbackWithNumber(t *testing.T) {
	setGitBranch(t, "")
	// Create a scenario where all possible combinations are taken
	// We have 25*25 = 625 combinations
	datePrefix := time.Now().Format("2006-01-02")
	existing := []string{}
	for _, adj := range adjectives {
		for _, noun := range nouns {
			existing = append(existing, datePrefix+"-"+adj+"-"+noun)
		}
	}

	name := GenerateUniqueRandomName(existing)

	// Should have added a number suffix: YYYY-MM-DD-adjective-noun-number
	pattern := `^\d{4}-\d{2}-\d{2}-[a-z]+-[a-z]+-\d+$`
	if !regexp.MustCompile(pattern).MatchString(name) {
		t.Errorf("Expected name with number suffix matching '%s', got '%s'", pattern, name)
	}
}

func TestGenerateUniqueRandomName_Empty(t *testing.T) {
	setGitBranch(t, "")
	name := GenerateUniqueRandomName([]string{})

	// Should generate a valid name
	if name == "" {
		t.Error("Expected non-empty name")
	}

	// Should match YYYY-MM-DD-adjective-noun format
	pattern := `^\d{4}-\d{2}-\d{2}-[a-z]+-[a-z]+$`
	if !regexp.MustCompile(pattern).MatchString(name) {
		t.Errorf("Expected name matching '%s', got '%s'", pattern, name)
	}
}

func TestGenerateUniqueRandomName_UsesGitBranch(t *testing.T) {
	setGitBranch(t, "feature/my-ticket")

	name := GenerateUniqueRandomName([]string{})

	if name != "feature-my-ticket" {
		t.Errorf("Expected 'feature-my-ticket', got '%s'", name)
	}
}

func TestGenerateUniqueRandomName_SkipsMainBranch(t *testing.T) {
	for _, branch := range []string{"main", "master", ""} {
		t.Run(branch, func(t *testing.T) {
			setGitBranch(t, branch)

			datePrefix := time.Now().Format("2006-01-02")
			name := GenerateUniqueRandomName([]string{})

			if !strings.HasPrefix(name, datePrefix+"-") {
				t.Errorf("Expected random name starting with '%s-', got '%s'", datePrefix, name)
			}
		})
	}
}

func TestGenerateUniqueRandomName_BranchConflictFallback(t *testing.T) {
	setGitBranch(t, "my-feature")

	// All branch-derived candidates are taken
	existing := []string{
		"my-feature",
		"my-feature-2",
		"my-feature-3",
		"my-feature-4",
		"my-feature-5",
		"my-feature-6",
		"my-feature-7",
		"my-feature-8",
		"my-feature-9",
	}

	datePrefix := time.Now().Format("2006-01-02")
	name := GenerateUniqueRandomName(existing)

	if !strings.HasPrefix(name, datePrefix+"-") {
		t.Errorf("Expected random fallback starting with '%s-', got '%s'", datePrefix, name)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feature/my-ticket", "feature-my-ticket"},
		{"ISSUE-123", "issue-123"},
		{"my_feature_branch", "my-feature-branch"},
		{"release-2.0", "release-2-0"},
		{"feature---double-hyphen", "feature-double-hyphen"},
		{"-leading-hyphen", "leading-hyphen"},
		{"trailing-hyphen-", "trailing-hyphen"},
		{"", ""},
		{"!!!", ""},
		{"abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeBranchName_Truncation(t *testing.T) {
	// 70-char branch name should be truncated to 62
	long := strings.Repeat("a", 70)
	result := SanitizeBranchName(long)
	if len(result) > 62 {
		t.Errorf("Expected truncation to 62 chars, got %d: %q", len(result), result)
	}
}
