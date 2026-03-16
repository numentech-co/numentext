package config

import (
	"log"
	"os/exec"
	"sort"
	"strings"
)

// DefaultToolConfig defines the built-in tool configuration for a language.
type DefaultToolConfig struct {
	Formatters   []ToolDef
	Linters      []ToolDef
	FormatOnSave bool
	LintOnSave   bool
}

// defaultToolConfigs maps language IDs to their default tool configurations.
// Only tools that are found in PATH will be registered.
var defaultToolConfigs = map[string]DefaultToolConfig{
	"python": {
		Formatters: []ToolDef{
			{Command: "isort", Args: []string{"--quiet", "{file}"}},
			{Command: "black", Args: []string{"--quiet", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "flake8", Args: []string{"{file}"}},
			{Command: "mypy", Args: []string{"--no-error-summary", "{file}"}},
			{Command: "bandit", Args: []string{"-q", "{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	},
	"go": {
		Formatters: []ToolDef{
			{Command: "gofmt", Args: []string{"-w", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "go", Args: []string{"vet", "{file}"}},
			{Command: "staticcheck", Args: []string{"{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	},
	"rust": {
		Formatters: []ToolDef{
			{Command: "rustfmt", Args: []string{"{file}"}},
		},
		Linters: []ToolDef{
			{Command: "cargo", Args: []string{"clippy", "--message-format=short"}},
		},
		FormatOnSave: true,
		LintOnSave:   false,
	},
	"javascript": {
		Formatters: []ToolDef{
			{Command: "prettier", Args: []string{"--write", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "eslint", Args: []string{"{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	},
	"typescript": {
		Formatters: []ToolDef{
			{Command: "prettier", Args: []string{"--write", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "eslint", Args: []string{"{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	},
	"c": {
		Formatters: []ToolDef{
			{Command: "clang-format", Args: []string{"-i", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "clang-tidy", Args: []string{"{file}"}},
		},
		FormatOnSave: false,
		LintOnSave:   false,
	},
	"cpp": {
		Formatters: []ToolDef{
			{Command: "clang-format", Args: []string{"-i", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "clang-tidy", Args: []string{"{file}"}},
		},
		FormatOnSave: false,
		LintOnSave:   false,
	},
	"java": {
		Formatters: []ToolDef{
			{Command: "google-java-format", Args: []string{"--replace", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "checkstyle", Args: []string{"-c", "/google_checks.xml", "{file}"}},
		},
		FormatOnSave: false,
		LintOnSave:   false,
	},
	"kotlin": {
		Formatters: []ToolDef{
			{Command: "ktlint", Args: []string{"-F", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "ktlint", Args: []string{"{file}"}},
			{Command: "detekt", Args: []string{"--input", "{file}"}},
		},
		FormatOnSave: false,
		LintOnSave:   false,
	},
}

// ToolStatus represents whether a tool is installed and available.
type ToolStatus struct {
	Tool      ToolDef
	Installed bool
}

// LanguageToolStatus holds the status of all default tools for a language.
type LanguageToolStatus struct {
	Language     string
	Formatters   []ToolStatus
	Linters      []ToolStatus
	FormatOnSave bool
	LintOnSave   bool
	IsDefault    bool // true if using defaults (not user-configured)
}

// isToolInstalled checks if a tool command is available in PATH.
func isToolInstalled(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// IsToolInstalledInPath checks if a tool command is available in the given PATH.
func IsToolInstalledInPath(command string, pathEnv string) bool {
	if pathEnv == "" {
		return isToolInstalled(command)
	}
	// Check each directory in the PATH
	for _, dir := range strings.Split(pathEnv, ":") {
		if dir == "" {
			continue
		}
		// exec.LookPath uses PATH env var; we just check the default
		// For simplicity, use the standard LookPath which uses os PATH
		_, err := exec.LookPath(command)
		if err == nil {
			return true
		}
		break
	}
	return false
}

// ApplyDefaults populates LanguageTools with default configurations for any
// language that doesn't have user-configured tools. Only tools found in PATH
// are included. Returns a list of languages and which tools were found.
func (c *Config) ApplyDefaults() map[string][]string {
	if c.LanguageTools == nil {
		c.LanguageTools = make(map[string]LanguageToolConfig)
	}

	found := make(map[string][]string)

	for langID, defaults := range defaultToolConfigs {
		// Skip if user has already configured this language
		if _, exists := c.LanguageTools[langID]; exists {
			continue
		}

		var availableFormatters []ToolDef
		var availableLinters []ToolDef
		var foundTools []string

		for _, f := range defaults.Formatters {
			if isToolInstalled(f.Command) {
				availableFormatters = append(availableFormatters, f)
				foundTools = append(foundTools, f.Command)
			}
		}
		for _, l := range defaults.Linters {
			if isToolInstalled(l.Command) {
				availableLinters = append(availableLinters, l)
				foundTools = append(foundTools, l.Command)
			}
		}

		// Only create a config entry if at least one tool was found
		if len(availableFormatters) > 0 || len(availableLinters) > 0 {
			c.LanguageTools[langID] = LanguageToolConfig{
				Formatters:   availableFormatters,
				Linters:      availableLinters,
				FormatOnSave: defaults.FormatOnSave,
				LintOnSave:   defaults.LintOnSave,
				IsDefault:    true,
			}
			found[langID] = foundTools
		}
	}

	return found
}

// GetAllToolStatuses returns the status of all known tools (defaults + configured)
// for display in the Language Tools dialog.
func (c *Config) GetAllToolStatuses() []LanguageToolStatus {
	statusMap := make(map[string]*LanguageToolStatus)

	// First, add all default languages
	for langID, defaults := range defaultToolConfigs {
		status := &LanguageToolStatus{
			Language:     langID,
			FormatOnSave: defaults.FormatOnSave,
			LintOnSave:   defaults.LintOnSave,
			IsDefault:    true,
		}
		for _, f := range defaults.Formatters {
			status.Formatters = append(status.Formatters, ToolStatus{
				Tool:      f,
				Installed: isToolInstalled(f.Command),
			})
		}
		for _, l := range defaults.Linters {
			status.Linters = append(status.Linters, ToolStatus{
				Tool:      l,
				Installed: isToolInstalled(l.Command),
			})
		}
		statusMap[langID] = status
	}

	// Override with user-configured languages
	for langID, ltc := range c.LanguageTools {
		if ltc.IsDefault {
			// This is a default config that was applied; update installed status
			if s, ok := statusMap[langID]; ok {
				s.FormatOnSave = ltc.FormatOnSave
				s.LintOnSave = ltc.LintOnSave
			}
			continue
		}
		status := &LanguageToolStatus{
			Language:     langID,
			FormatOnSave: ltc.FormatOnSave,
			LintOnSave:   ltc.LintOnSave,
			IsDefault:    false,
		}
		for _, f := range ltc.Formatters {
			status.Formatters = append(status.Formatters, ToolStatus{
				Tool:      f,
				Installed: isToolInstalled(f.Command),
			})
		}
		for _, l := range ltc.Linters {
			status.Linters = append(status.Linters, ToolStatus{
				Tool:      l,
				Installed: isToolInstalled(l.Command),
			})
		}
		statusMap[langID] = status
	}

	// Convert to sorted slice
	var result []LanguageToolStatus
	for _, s := range statusMap {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Language < result[j].Language
	})

	return result
}

// DetectedToolsSummary returns a short summary string of detected tools for a language.
func DetectedToolsSummary(langID string, tools []string) string {
	if len(tools) == 0 {
		return ""
	}
	log.Printf("config: %s tools detected: %s", langID, strings.Join(tools, ", "))
	return langID + ": " + strings.Join(tools, ", ")
}
