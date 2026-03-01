package config

import (
	"testing"
)

// === Story 7.1: Configuration File ===

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TabSize != 4 {
		t.Errorf("expected tab size 4, got %d", cfg.TabSize)
	}
	if cfg.Theme != "borland" {
		t.Errorf("expected theme 'borland', got %s", cfg.Theme)
	}
	if !cfg.ShowLineNum {
		t.Error("expected show_line_numbers to be true by default")
	}
	if cfg.WordWrap {
		t.Error("expected word_wrap to be false by default")
	}
	if len(cfg.RecentFiles) != 0 {
		t.Error("expected empty recent files")
	}
}

// === Story 3.6: Recent Files ===

func TestAddRecentFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddRecentFile("/tmp/a.go")
	cfg.AddRecentFile("/tmp/b.py")
	if len(cfg.RecentFiles) != 2 {
		t.Fatalf("expected 2 recent files, got %d", len(cfg.RecentFiles))
	}
	if cfg.RecentFiles[0] != "/tmp/b.py" {
		t.Errorf("most recent should be b.py, got %s", cfg.RecentFiles[0])
	}
	if cfg.RecentFiles[1] != "/tmp/a.go" {
		t.Errorf("second should be a.go, got %s", cfg.RecentFiles[1])
	}
}

func TestAddRecentFile_NoDuplicates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddRecentFile("/tmp/a.go")
	cfg.AddRecentFile("/tmp/b.py")
	cfg.AddRecentFile("/tmp/a.go") // Re-add a.go
	if len(cfg.RecentFiles) != 2 {
		t.Fatalf("expected 2 recent files (no duplicates), got %d", len(cfg.RecentFiles))
	}
	if cfg.RecentFiles[0] != "/tmp/a.go" {
		t.Errorf("most recent should be a.go, got %s", cfg.RecentFiles[0])
	}
}

func TestAddRecentFile_MaxTwenty(t *testing.T) {
	cfg := DefaultConfig()
	for i := 0; i < 25; i++ {
		cfg.AddRecentFile("/tmp/file" + string(rune('a'+i)) + ".go")
	}
	if len(cfg.RecentFiles) != 20 {
		t.Errorf("expected max 20 recent files, got %d", len(cfg.RecentFiles))
	}
}
