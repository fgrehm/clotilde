package util

import (
	"strings"
	"testing"
)

func TestGenerateRandomName(t *testing.T) {
	name := GenerateRandomName()

	// Should be in format "adjective-noun"
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Errorf("Expected name in format 'adjective-noun', got '%s'", name)
	}

	// Should contain valid adjective
	adjective := parts[0]
	found := false
	for _, adj := range adjectives {
		if adj == adjective {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Generated name '%s' has invalid adjective '%s'", name, adjective)
	}

	// Should contain valid noun
	noun := parts[1]
	found = false
	for _, n := range nouns {
		if n == noun {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Generated name '%s' has invalid noun '%s'", name, noun)
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
	existing := []string{"happy-fox", "brave-wolf", "clever-bear"}

	name := GenerateUniqueRandomName(existing)

	// Should not match any existing name
	for _, existingName := range existing {
		if name == existingName {
			t.Errorf("Generated name '%s' conflicts with existing name '%s'", name, existingName)
		}
	}

	// Should be in valid format
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		t.Errorf("Expected name in format 'adjective-noun', got '%s'", name)
	}
}

func TestGenerateUniqueRandomName_FallbackWithNumber(t *testing.T) {
	// Create a scenario where all possible combinations are taken
	// This is actually hard to test since we have 25*25 = 625 combinations
	// So we'll just test that the function doesn't hang
	existing := []string{}
	for _, adj := range adjectives {
		for _, noun := range nouns {
			existing = append(existing, adj+"-"+noun)
		}
	}

	name := GenerateUniqueRandomName(existing)

	// Should have added a number suffix
	parts := strings.Split(name, "-")
	if len(parts) != 3 {
		t.Errorf("Expected name with number suffix in format 'adjective-noun-number', got '%s'", name)
	}
}

func TestGenerateUniqueRandomName_Empty(t *testing.T) {
	name := GenerateUniqueRandomName([]string{})

	// Should generate a valid name
	if name == "" {
		t.Error("Expected non-empty name")
	}

	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Errorf("Expected name in format 'adjective-noun', got '%s'", name)
	}
}
