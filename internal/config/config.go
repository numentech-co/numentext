package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	RecentFiles  []string `json:"recent_files"`
	TabSize      int      `json:"tab_size"`
	Theme        string   `json:"theme"`
	ShowLineNum  bool     `json:"show_line_numbers"`
	WordWrap     bool     `json:"word_wrap"`
	KeyboardMode string   `json:"keyboard_mode"`
	FileTreeWidth int     `json:"file_tree_width"`
	OutputHeight  int     `json:"output_height"`
	UIStyle       string  `json:"ui_style"`
	IconSet       string  `json:"icon_set"`
}

func DefaultConfig() *Config {
	return &Config{
		RecentFiles:   []string{},
		TabSize:       4,
		Theme:         "borland",
		ShowLineNum:   true,
		WordWrap:      false,
		KeyboardMode:  "default",
		FileTreeWidth: 20,
		OutputHeight:  8,
		UIStyle:       "modern",
		IconSet:       "unicode",
	}
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".numentext")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func Load() *Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	if cfg.FileTreeWidth == 0 {
		cfg.FileTreeWidth = 20
	}
	if cfg.OutputHeight == 0 {
		cfg.OutputHeight = 8
	}
	if cfg.UIStyle == "" {
		cfg.UIStyle = "modern"
	}
	if cfg.IconSet == "" {
		cfg.IconSet = "unicode"
	}
	return cfg
}

func (c *Config) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

func (c *Config) AddRecentFile(path string) {
	// Remove if already present
	filtered := make([]string, 0, len(c.RecentFiles))
	for _, f := range c.RecentFiles {
		if f != path {
			filtered = append(filtered, f)
		}
	}
	// Prepend
	c.RecentFiles = append([]string{path}, filtered...)
	if len(c.RecentFiles) > 20 {
		c.RecentFiles = c.RecentFiles[:20]
	}
}
