package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	RecentFiles []string `json:"recent_files"`
	TabSize     int      `json:"tab_size"`
	Theme       string   `json:"theme"`
	ShowLineNum bool     `json:"show_line_numbers"`
	WordWrap    bool     `json:"word_wrap"`
}

func DefaultConfig() *Config {
	return &Config{
		RecentFiles: []string{},
		TabSize:     4,
		Theme:       "borland",
		ShowLineNum: true,
		WordWrap:    false,
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
	json.Unmarshal(data, cfg)
	return cfg
}

func (c *Config) Save() error {
	dir := ConfigDir()
	os.MkdirAll(dir, 0755)
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
