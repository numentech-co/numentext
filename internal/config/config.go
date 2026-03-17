package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// ToolDef defines an external tool (formatter or linter) command.
type ToolDef struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	// ErrorPattern is an optional regex for parsing linter output.
	// It should have named groups: file, line, col (optional), message.
	// Default pattern: file:line:col: message
	ErrorPattern string `json:"error_pattern,omitempty"`
}

// LanguageToolConfig holds formatter/linter settings for a single language.
type LanguageToolConfig struct {
	Formatters   []ToolDef `json:"formatters,omitempty"`
	Linters      []ToolDef `json:"linters,omitempty"`
	FormatOnSave bool      `json:"format_on_save"`
	LintOnSave   bool      `json:"lint_on_save"`
	IsDefault    bool      `json:"-"` // true if auto-detected from defaults (not persisted)
}

type Config struct {
	RecentFiles      []string                      `json:"recent_files"`
	TabSize          int                           `json:"tab_size"`
	Theme            string                        `json:"theme"`
	ShowLineNum      bool                          `json:"show_line_numbers"`
	WordWrap         bool                          `json:"word_wrap"`
	KeyboardMode     string                        `json:"keyboard_mode"`
	FileTreeWidth    int                           `json:"file_tree_width"`
	OutputHeight     int                           `json:"output_height"`
	UIStyle          string                        `json:"ui_style"`
	IconSet          string                        `json:"icon_set"`
	LanguageTools    map[string]LanguageToolConfig  `json:"language_tools"`
	TrackedLanguages []string                       `json:"tracked_languages,omitempty"`
	DeclinedLSP      []string                       `json:"declined_lsp,omitempty"`
	TestCommands     map[string]string              `json:"test_commands,omitempty"`
	DisabledPlugins  []string                       `json:"disabled_plugins,omitempty"`
	PluginRegistryURL string                        `json:"plugin_registry_url,omitempty"`
	ActiveVenv       *VenvInfo                      `json:"-"` // runtime-only: detected Python venv
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
		LanguageTools: make(map[string]LanguageToolConfig),
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
	if cfg.LanguageTools == nil {
		cfg.LanguageTools = make(map[string]LanguageToolConfig)
	}
	// Validate tool entries: skip any with empty Command
	for lang, ltc := range cfg.LanguageTools {
		validFmt := make([]ToolDef, 0, len(ltc.Formatters))
		for _, f := range ltc.Formatters {
			if f.Command == "" {
				log.Printf("config: skipping formatter with empty command for language %q", lang)
				continue
			}
			validFmt = append(validFmt, f)
		}
		validLint := make([]ToolDef, 0, len(ltc.Linters))
		for _, l := range ltc.Linters {
			if l.Command == "" {
				log.Printf("config: skipping linter with empty command for language %q", lang)
				continue
			}
			validLint = append(validLint, l)
		}
		ltc.Formatters = validFmt
		ltc.Linters = validLint
		cfg.LanguageTools[lang] = ltc
	}
	return cfg
}

// ToolsForLanguage returns the tool config for a language ID (e.g. "go", "python").
// Returns an empty config if none is configured.
func (c *Config) ToolsForLanguage(langID string) LanguageToolConfig {
	if c.LanguageTools == nil {
		return LanguageToolConfig{}
	}
	return c.LanguageTools[langID]
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

// TrackLanguage adds a language to the tracked set if not already present.
func (c *Config) TrackLanguage(lang string) bool {
	for _, l := range c.TrackedLanguages {
		if l == lang {
			return false // already tracked
		}
	}
	c.TrackedLanguages = append(c.TrackedLanguages, lang)
	return true // newly tracked
}

// IsLSPDeclined returns true if the user has declined the LSP install prompt for a language.
func (c *Config) IsLSPDeclined(lang string) bool {
	for _, l := range c.DeclinedLSP {
		if l == lang {
			return true
		}
	}
	return false
}

// DeclineLSP records that the user dismissed the LSP install prompt for a language.
func (c *Config) DeclineLSP(lang string) {
	if !c.IsLSPDeclined(lang) {
		c.DeclinedLSP = append(c.DeclinedLSP, lang)
	}
}

// TestCommandForLanguage returns the user-configured test command for a language,
// or empty string if none is configured.
func (c *Config) TestCommandForLanguage(lang string) string {
	if c.TestCommands == nil {
		return ""
	}
	return c.TestCommands[lang]
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
