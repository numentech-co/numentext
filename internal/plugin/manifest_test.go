package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "A test plugin",
		"file_types": [".txt"],
		"commands": [{"id": "test.hello", "title": "Say Hello"}],
		"panels": [{"name": "test-panel", "position": "bottom"}]
	}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if m.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %q", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", m.Version)
	}
	if len(m.FileTypes) != 1 || m.FileTypes[0] != ".txt" {
		t.Errorf("unexpected file_types: %v", m.FileTypes)
	}
	if len(m.Commands) != 1 || m.Commands[0].ID != "test.hello" {
		t.Errorf("unexpected commands: %v", m.Commands)
	}
	if len(m.Panels) != 1 || m.Panels[0].Name != "test-panel" {
		t.Errorf("unexpected panels: %v", m.Panels)
	}
}

func TestLoadManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	data := `{"version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error about name, got: %v", err)
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing plugin.json")
	}
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{
			name:    "valid",
			m:       Manifest{Name: "foo", Version: "1.0"},
			wantErr: false,
		},
		{
			name:    "missing name",
			m:       Manifest{Version: "1.0"},
			wantErr: true,
		},
		{
			name:    "missing version",
			m:       Manifest{Name: "foo"},
			wantErr: true,
		},
		{
			name: "invalid panel position",
			m: Manifest{
				Name: "foo", Version: "1.0",
				Panels: []ManifestPanel{{Name: "p", Position: "left"}},
			},
			wantErr: true,
		},
		{
			name: "valid panel position",
			m: Manifest{
				Name: "foo", Version: "1.0",
				Panels: []ManifestPanel{{Name: "p", Position: "bottom"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
