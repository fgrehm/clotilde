package tour

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Tour represents a CodeTour file.
type Tour struct {
	Schema string `json:"$schema,omitempty"`
	Title  string `json:"title"`
	Steps  []Step `json:"steps"`
}

// Step represents a single step in a tour.
type Step struct {
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
}

// LoadFile parses a single .tour file.
func LoadFile(path string) (*Tour, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tour file: %w", err)
	}

	var t Tour
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse tour file: %w", err)
	}

	if err := validate(&t); err != nil {
		return nil, err
	}

	return &t, nil
}

func validate(t *Tour) error {
	if len(t.Steps) == 0 {
		return fmt.Errorf("tour must have at least one step")
	}
	for i := range t.Steps {
		if t.Steps[i].File == "" {
			return fmt.Errorf("step %d: file field is required", i+1)
		}
		if t.Steps[i].Line < 1 {
			t.Steps[i].Line = 1
		}
	}
	return nil
}

// LoadFromDir scans dir for .tour files, returns map[name]*Tour.
func LoadFromDir(dir string) (map[string]*Tour, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Tour{}, nil
		}
		return nil, fmt.Errorf("failed to read tour directory: %w", err)
	}

	tours := make(map[string]*Tour)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tour") {
			continue
		}

		t, err := LoadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			// Skip files that fail to parse
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".tour")
		tours[name] = t
	}

	return tours, nil
}
