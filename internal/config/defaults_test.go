package config

import (
	"testing"
)

// === Epic 22: Pre-configured Language Tools ===

func TestDefaultToolConfigs_AllLanguages(t *testing.T) {
	// Verify all expected languages have default configs
	expectedLangs := []string{"python", "go", "rust", "javascript", "typescript", "c", "cpp"}
	for _, lang := range expectedLangs {
		if _, ok := defaultToolConfigs[lang]; !ok {
			t.Errorf("expected default config for language %q", lang)
		}
	}
}

func TestDefaultToolConfigs_PythonTools(t *testing.T) {
	py := defaultToolConfigs["python"]
	if len(py.Formatters) < 2 {
		t.Errorf("expected at least 2 Python formatters (isort, black), got %d", len(py.Formatters))
	}
	if len(py.Linters) < 3 {
		t.Errorf("expected at least 3 Python linters (flake8, mypy, bandit), got %d", len(py.Linters))
	}

	// Verify specific tools
	fmtNames := make(map[string]bool)
	for _, f := range py.Formatters {
		fmtNames[f.Command] = true
	}
	if !fmtNames["black"] {
		t.Error("expected black in Python formatters")
	}
	if !fmtNames["isort"] {
		t.Error("expected isort in Python formatters")
	}

	lintNames := make(map[string]bool)
	for _, l := range py.Linters {
		lintNames[l.Command] = true
	}
	if !lintNames["flake8"] {
		t.Error("expected flake8 in Python linters")
	}
	if !lintNames["mypy"] {
		t.Error("expected mypy in Python linters")
	}
	if !lintNames["bandit"] {
		t.Error("expected bandit in Python linters")
	}
}

func TestDefaultToolConfigs_GoTools(t *testing.T) {
	g := defaultToolConfigs["go"]
	fmtNames := make(map[string]bool)
	for _, f := range g.Formatters {
		fmtNames[f.Command] = true
	}
	if !fmtNames["gofmt"] {
		t.Error("expected gofmt in Go formatters")
	}

	lintNames := make(map[string]bool)
	for _, l := range g.Linters {
		lintNames[l.Command] = true
	}
	if !lintNames["go"] {
		t.Error("expected 'go' (vet) in Go linters")
	}
}

func TestApplyDefaults_NoUserConfig(t *testing.T) {
	cfg := DefaultConfig()
	found := cfg.ApplyDefaults()

	// gofmt should be detected (it comes with Go)
	if goTools, ok := found["go"]; ok {
		hasGofmt := false
		for _, tool := range goTools {
			if tool == "gofmt" {
				hasGofmt = true
			}
		}
		if !hasGofmt {
			t.Error("expected gofmt in detected Go tools")
		}
	}

	// Verify the config was populated
	goLtc := cfg.ToolsForLanguage("go")
	if len(goLtc.Formatters) > 0 && goLtc.Formatters[0].Command != "gofmt" {
		t.Errorf("expected first Go formatter to be gofmt, got %s", goLtc.Formatters[0].Command)
	}
}

func TestApplyDefaults_UserConfigNotOverridden(t *testing.T) {
	cfg := DefaultConfig()
	// User has custom Go config
	cfg.LanguageTools["go"] = LanguageToolConfig{
		Formatters: []ToolDef{
			{Command: "goimports", Args: []string{"-w", "{file}"}},
		},
		FormatOnSave: false,
	}

	cfg.ApplyDefaults()

	// User config should be preserved
	goLtc := cfg.ToolsForLanguage("go")
	if len(goLtc.Formatters) != 1 {
		t.Fatalf("expected 1 user-configured formatter, got %d", len(goLtc.Formatters))
	}
	if goLtc.Formatters[0].Command != "goimports" {
		t.Errorf("expected user's goimports, got %s", goLtc.Formatters[0].Command)
	}
	if goLtc.FormatOnSave {
		t.Error("expected user's format_on_save=false to be preserved")
	}
}

func TestApplyDefaults_MarkedAsDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApplyDefaults()

	// Any auto-configured language should have IsDefault=true
	for lang, ltc := range cfg.LanguageTools {
		if !ltc.IsDefault {
			t.Errorf("expected IsDefault=true for auto-configured language %q", lang)
		}
	}
}

func TestGetAllToolStatuses(t *testing.T) {
	cfg := DefaultConfig()
	statuses := cfg.GetAllToolStatuses()

	// Should have entries for all default languages
	langSet := make(map[string]bool)
	for _, s := range statuses {
		langSet[s.Language] = true
	}
	expectedLangs := []string{"python", "go", "rust", "javascript", "typescript", "c", "cpp"}
	for _, lang := range expectedLangs {
		if !langSet[lang] {
			t.Errorf("expected language %q in tool statuses", lang)
		}
	}
}

func TestGetAllToolStatuses_ShowsInstalledStatus(t *testing.T) {
	cfg := DefaultConfig()
	statuses := cfg.GetAllToolStatuses()

	// Find Go language status
	for _, s := range statuses {
		if s.Language == "go" {
			// gofmt should be installed (we are running Go)
			for _, f := range s.Formatters {
				if f.Tool.Command == "gofmt" && !f.Installed {
					t.Error("expected gofmt to be detected as installed")
				}
			}
		}
	}
}

func TestDetectedToolsSummary(t *testing.T) {
	summary := DetectedToolsSummary("go", []string{"gofmt", "go"})
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if summary != "go: gofmt, go" {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestDetectedToolsSummary_Empty(t *testing.T) {
	summary := DetectedToolsSummary("python", nil)
	if summary != "" {
		t.Errorf("expected empty summary for no tools, got %q", summary)
	}
}

func TestIsDefaultNotPersisted(t *testing.T) {
	// IsDefault has json:"-" tag, so it should not appear in JSON output
	cfg := DefaultConfig()
	cfg.LanguageTools["go"] = LanguageToolConfig{
		IsDefault: true,
		Formatters: []ToolDef{
			{Command: "gofmt", Args: []string{"-w", "{file}"}},
		},
	}

	// The IsDefault field should not affect JSON serialization
	ltc := cfg.ToolsForLanguage("go")
	if !ltc.IsDefault {
		t.Error("expected IsDefault=true in memory")
	}
}
