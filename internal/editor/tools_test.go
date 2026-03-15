package editor

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"numentext/internal/config"
)

func TestSubstituteArgs(t *testing.T) {
	args := []string{"--quiet", "{file}", "--check"}
	result := substituteArgs(args, "/tmp/test.py")
	if result[0] != "--quiet" {
		t.Errorf("expected --quiet, got %s", result[0])
	}
	if result[1] != "/tmp/test.py" {
		t.Errorf("expected /tmp/test.py, got %s", result[1])
	}
	if result[2] != "--check" {
		t.Errorf("expected --check, got %s", result[2])
	}
}

func TestSubstituteArgs_NoPlaceholder(t *testing.T) {
	args := []string{"--fix"}
	result := substituteArgs(args, "/tmp/test.py")
	if result[0] != "--fix" {
		t.Errorf("expected --fix, got %s", result[0])
	}
}

func TestParseLinterOutput_DefaultPattern(t *testing.T) {
	output := `test.py:12:1: E302 expected 2 blank lines, got 1
test.py:25:5: W291 trailing whitespace
other.py:3:1: E303 too many blank lines`

	diags := parseLinterOutput(output, "/project/test.py", defaultErrorPattern)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics for test.py, got %d", len(diags))
	}
	if diags[0].Line != 12 {
		t.Errorf("expected line 12, got %d", diags[0].Line)
	}
	if diags[0].Col != 1 {
		t.Errorf("expected col 1, got %d", diags[0].Col)
	}
	if diags[1].Line != 25 {
		t.Errorf("expected line 25, got %d", diags[1].Line)
	}
}

func TestParseLinterOutput_NoColPattern(t *testing.T) {
	pattern := regexp.MustCompile(`^(.+?):(\d+):\s*(.+)$`)
	output := "test.go:10: undefined variable x\n"
	diags := parseLinterOutput(output, "test.go", pattern)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Line != 10 {
		t.Errorf("expected line 10, got %d", diags[0].Line)
	}
	if diags[0].Col != 0 {
		t.Errorf("expected col 0, got %d", diags[0].Col)
	}
}

func TestParseLinterOutput_EmptyOutput(t *testing.T) {
	diags := parseLinterOutput("", "test.py", defaultErrorPattern)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for empty output, got %d", len(diags))
	}
}

func TestRunFormatters_EmptyList(t *testing.T) {
	result := RunFormatters("/tmp/nonexistent", nil)
	if result.Changed {
		t.Error("expected no change with empty formatters")
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
}

func TestRunLinters_EmptyList(t *testing.T) {
	result := RunLinters("/tmp/nonexistent", nil)
	if len(result.Diagnostics) != 0 {
		t.Error("expected no diagnostics with empty linters")
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
}

func TestRunFormatters_CommandNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(testFile, []byte("x=1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := RunFormatters(testFile, []config.ToolDef{
		{Command: "nonexistent-formatter-xyz", Args: []string{"{file}"}},
	})
	if result.Error == nil {
		t.Error("expected error for nonexistent command")
	}

	// Original content should be preserved
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "x=1\n" {
		t.Error("original content should be preserved on formatter failure")
	}
}

func TestRunLinters_CommandNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(testFile, []byte("x=1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := RunLinters(testFile, []config.ToolDef{
		{Command: "nonexistent-linter-xyz", Args: []string{"{file}"}},
	})
	// Linters that fail to run should not produce diagnostics
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestRunFormatters_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello world\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 'true' command succeeds but does not modify the file
	result := RunFormatters(testFile, []config.ToolDef{
		{Command: "true", Args: []string{}},
	})
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
	if result.Changed {
		t.Error("expected no change when formatter does not modify file")
	}
}
