package config

import (
	"encoding/json"
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

// === Epic 16: Language Tools Config ===

func TestDefaultConfig_LanguageTools(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LanguageTools == nil {
		t.Fatal("LanguageTools map should not be nil")
	}
	if len(cfg.LanguageTools) != 0 {
		t.Errorf("expected empty LanguageTools, got %d entries", len(cfg.LanguageTools))
	}
}

func TestToolsForLanguage_NotConfigured(t *testing.T) {
	cfg := DefaultConfig()
	ltc := cfg.ToolsForLanguage("python")
	if len(ltc.Formatters) != 0 {
		t.Error("expected no formatters for unconfigured language")
	}
	if len(ltc.Linters) != 0 {
		t.Error("expected no linters for unconfigured language")
	}
	if ltc.FormatOnSave {
		t.Error("expected format_on_save false for unconfigured language")
	}
	if ltc.LintOnSave {
		t.Error("expected lint_on_save false for unconfigured language")
	}
}

func TestToolsForLanguage_Configured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LanguageTools["python"] = LanguageToolConfig{
		Formatters: []ToolDef{
			{Command: "isort", Args: []string{"{file}"}},
			{Command: "black", Args: []string{"--quiet", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "flake8", Args: []string{"{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	}

	ltc := cfg.ToolsForLanguage("python")
	if len(ltc.Formatters) != 2 {
		t.Fatalf("expected 2 formatters, got %d", len(ltc.Formatters))
	}
	if ltc.Formatters[0].Command != "isort" {
		t.Errorf("expected first formatter isort, got %s", ltc.Formatters[0].Command)
	}
	if ltc.Formatters[1].Command != "black" {
		t.Errorf("expected second formatter black, got %s", ltc.Formatters[1].Command)
	}
	if !ltc.FormatOnSave {
		t.Error("expected format_on_save true")
	}
	if !ltc.LintOnSave {
		t.Error("expected lint_on_save true")
	}
	if len(ltc.Linters) != 1 {
		t.Fatalf("expected 1 linter, got %d", len(ltc.Linters))
	}
	if ltc.Linters[0].Command != "flake8" {
		t.Errorf("expected linter flake8, got %s", ltc.Linters[0].Command)
	}
}

func TestLanguageTools_JSONRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LanguageTools["python"] = LanguageToolConfig{
		Formatters: []ToolDef{
			{Command: "black", Args: []string{"--quiet", "{file}"}},
		},
		FormatOnSave: true,
	}
	cfg.LanguageTools["go"] = LanguageToolConfig{
		Formatters: []ToolDef{
			{Command: "gofmt", Args: []string{"-w", "{file}"}},
		},
		Linters: []ToolDef{
			{Command: "golangci-lint", Args: []string{"run", "{file}"}},
		},
		FormatOnSave: true,
		LintOnSave:   true,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	cfg2 := DefaultConfig()
	if err := json.Unmarshal(data, cfg2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	pyTools := cfg2.ToolsForLanguage("python")
	if len(pyTools.Formatters) != 1 || pyTools.Formatters[0].Command != "black" {
		t.Error("python formatter not preserved through JSON round-trip")
	}
	if !pyTools.FormatOnSave {
		t.Error("python format_on_save not preserved")
	}

	goTools := cfg2.ToolsForLanguage("go")
	if len(goTools.Formatters) != 1 || goTools.Formatters[0].Command != "gofmt" {
		t.Error("go formatter not preserved through JSON round-trip")
	}
	if len(goTools.Linters) != 1 || goTools.Linters[0].Command != "golangci-lint" {
		t.Error("go linter not preserved through JSON round-trip")
	}
}

// === Epic 24: LSP Auto-Install Prompting ===

func TestTrackLanguage_NewLanguage(t *testing.T) {
	cfg := DefaultConfig()
	isNew := cfg.TrackLanguage("python")
	if !isNew {
		t.Error("expected new language to return true")
	}
	if len(cfg.TrackedLanguages) != 1 || cfg.TrackedLanguages[0] != "python" {
		t.Errorf("expected [python], got %v", cfg.TrackedLanguages)
	}
}

func TestTrackLanguage_DuplicateLanguage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrackLanguage("go")
	isNew := cfg.TrackLanguage("go")
	if isNew {
		t.Error("expected duplicate language to return false")
	}
	if len(cfg.TrackedLanguages) != 1 {
		t.Errorf("expected 1 tracked language, got %d", len(cfg.TrackedLanguages))
	}
}

func TestTrackLanguage_MultipleLanguages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrackLanguage("python")
	cfg.TrackLanguage("go")
	cfg.TrackLanguage("rust")
	if len(cfg.TrackedLanguages) != 3 {
		t.Errorf("expected 3 tracked languages, got %d", len(cfg.TrackedLanguages))
	}
}

func TestDeclineLSP(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IsLSPDeclined("python") {
		t.Error("expected python not declined initially")
	}
	cfg.DeclineLSP("python")
	if !cfg.IsLSPDeclined("python") {
		t.Error("expected python declined after DeclineLSP")
	}
	// Decline again should not duplicate
	cfg.DeclineLSP("python")
	if len(cfg.DeclinedLSP) != 1 {
		t.Errorf("expected 1 declined entry, got %d", len(cfg.DeclinedLSP))
	}
}

func TestTrackedLanguages_JSONRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrackLanguage("go")
	cfg.TrackLanguage("python")
	cfg.DeclineLSP("python")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	cfg2 := DefaultConfig()
	if err := json.Unmarshal(data, cfg2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(cfg2.TrackedLanguages) != 2 {
		t.Errorf("expected 2 tracked languages after round-trip, got %d", len(cfg2.TrackedLanguages))
	}
	if len(cfg2.DeclinedLSP) != 1 || cfg2.DeclinedLSP[0] != "python" {
		t.Errorf("expected declined [python] after round-trip, got %v", cfg2.DeclinedLSP)
	}
}

func TestTestCommandForLanguage(t *testing.T) {
	cfg := DefaultConfig()
	// No test commands configured
	if cmd := cfg.TestCommandForLanguage("go"); cmd != "" {
		t.Errorf("expected empty, got %s", cmd)
	}

	// Configure custom test command
	cfg.TestCommands = map[string]string{
		"go": "make test",
	}
	if cmd := cfg.TestCommandForLanguage("go"); cmd != "make test" {
		t.Errorf("expected 'make test', got %s", cmd)
	}
}

func TestLanguageTools_InvalidToolSkipped(t *testing.T) {
	jsonData := `{
		"tab_size": 4,
		"language_tools": {
			"python": {
				"formatters": [
					{"command": "black", "args": ["--quiet", "{file}"]},
					{"command": "", "args": ["bad"]}
				],
				"linters": [
					{"command": "", "args": []},
					{"command": "flake8", "args": ["{file}"]}
				],
				"format_on_save": true
			}
		}
	}`

	cfg := DefaultConfig()
	if err := json.Unmarshal([]byte(jsonData), cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// Simulate the validation that Load() does
	for lang, ltc := range cfg.LanguageTools {
		validFmt := make([]ToolDef, 0, len(ltc.Formatters))
		for _, f := range ltc.Formatters {
			if f.Command != "" {
				validFmt = append(validFmt, f)
			}
		}
		validLint := make([]ToolDef, 0, len(ltc.Linters))
		for _, l := range ltc.Linters {
			if l.Command != "" {
				validLint = append(validLint, l)
			}
		}
		ltc.Formatters = validFmt
		ltc.Linters = validLint
		cfg.LanguageTools[lang] = ltc
	}

	pyTools := cfg.ToolsForLanguage("python")
	if len(pyTools.Formatters) != 1 {
		t.Fatalf("expected 1 valid formatter after skipping empty command, got %d", len(pyTools.Formatters))
	}
	if pyTools.Formatters[0].Command != "black" {
		t.Errorf("expected black, got %s", pyTools.Formatters[0].Command)
	}
	if len(pyTools.Linters) != 1 {
		t.Fatalf("expected 1 valid linter after skipping empty command, got %d", len(pyTools.Linters))
	}
	if pyTools.Linters[0].Command != "flake8" {
		t.Errorf("expected flake8, got %s", pyTools.Linters[0].Command)
	}
}
