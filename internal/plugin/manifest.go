package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest represents a plugin.json descriptor.
type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	MinVersion  string            `json:"min_version,omitempty"`
	FileTypes   []string          `json:"file_types,omitempty"`
	Commands    []ManifestCommand `json:"commands,omitempty"`
	Menus       []ManifestMenu    `json:"menus,omitempty"`
	Panels      []ManifestPanel   `json:"panels,omitempty"`
}

// ManifestCommand describes a command declared in plugin.json.
type ManifestCommand struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ManifestMenu describes a menu item declared in plugin.json.
type ManifestMenu struct {
	Menu    string `json:"menu"`
	Label   string `json:"label"`
	Command string `json:"command"`
}

// ManifestPanel describes a panel declared in plugin.json.
type ManifestPanel struct {
	Name     string `json:"name"`
	Position string `json:"position"` // "bottom" or "right"
}

// LoadManifest reads and parses a plugin.json file.
func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plugin manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing plugin manifest: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("plugin manifest missing required field: name")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("plugin manifest missing required field: version")
	}
	return &m, nil
}

// Validate checks that the manifest has required fields and valid values.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	for _, p := range m.Panels {
		if p.Position != "bottom" && p.Position != "right" {
			return fmt.Errorf("panel %q has invalid position %q (must be bottom or right)", p.Name, p.Position)
		}
	}
	return nil
}
