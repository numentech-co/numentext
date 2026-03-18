package plugin

import (
	"strings"
	"testing"
)

func TestRenderMarkdownToTview_Headers(t *testing.T) {
	md := "# Hello World"
	result := RenderMarkdownToTview(md)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Should contain the text "Hello World" (headers hide the # prefix)
	if !strings.Contains(result, "Hello World") {
		t.Errorf("expected 'Hello World' in result, got %q", result)
	}
}

func TestRenderMarkdownToTview_Bold(t *testing.T) {
	md := "this is **bold** text"
	result := RenderMarkdownToTview(md)
	if !strings.Contains(result, "bold") {
		t.Errorf("expected 'bold' in result, got %q", result)
	}
	// Should contain bold tview tag
	if !strings.Contains(result, "b") {
		t.Errorf("expected bold formatting in result, got %q", result)
	}
}

func TestRenderMarkdownToTview_CodeSpan(t *testing.T) {
	md := "use `fmt.Println` here"
	result := RenderMarkdownToTview(md)
	if !strings.Contains(result, "fmt.Println") {
		t.Errorf("expected code span text in result, got %q", result)
	}
}

func TestRenderMarkdownToTview_MultiLine(t *testing.T) {
	md := "line one\n# Header\nline three"
	result := RenderMarkdownToTview(md)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRenderCodeToTview_Go(t *testing.T) {
	code := `package main

func main() {
	fmt.Println("hello")
}
`
	result := RenderCodeToTview(code, "Go")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Keywords should have color tags
	if !strings.Contains(result, "package") {
		t.Errorf("expected 'package' in result, got %q", result)
	}
	if !strings.Contains(result, "func") {
		t.Errorf("expected 'func' in result, got %q", result)
	}
}

func TestRenderCodeToTview_UnknownLang(t *testing.T) {
	code := "just some text"
	result := RenderCodeToTview(code, "nonexistent-language")
	if !strings.Contains(result, "just some text") {
		t.Errorf("expected plain text fallback, got %q", result)
	}
}

func TestRenderCodeToTview_Python(t *testing.T) {
	code := `def hello():
    print("world")
`
	result := RenderCodeToTview(code, "Python")
	if !strings.Contains(result, "def") {
		t.Errorf("expected 'def' in result, got %q", result)
	}
}

func TestRenderCellsToTview_Empty(t *testing.T) {
	result := RenderCellsToTview(nil)
	if result != "" {
		t.Errorf("expected empty result for nil cells, got %q", result)
	}
}

func TestRenderCellsToTview_SingleCodeCell(t *testing.T) {
	cells := []PanelCell{
		{Type: "code", Language: "Go", Content: "package main"},
	}
	result := RenderCellsToTview(cells)
	if !strings.Contains(result, "code (Go)") {
		t.Errorf("expected cell label in result, got %q", result)
	}
	if !strings.Contains(result, "package") {
		t.Errorf("expected code content in result, got %q", result)
	}
}

func TestRenderCellsToTview_MultipleCells(t *testing.T) {
	cells := []PanelCell{
		{Type: "markdown", Content: "# Title"},
		{Type: "code", Language: "Python", Content: "print('hi')"},
		{Type: "output", Content: "hi"},
		{Type: "raw", Content: "raw text"},
	}
	result := RenderCellsToTview(cells)
	// Should have separators between cells
	if !strings.Contains(result, "---") {
		t.Errorf("expected separator line in result, got %q", result)
	}
	if !strings.Contains(result, "Title") {
		t.Errorf("expected markdown content in result, got %q", result)
	}
	if !strings.Contains(result, "print") {
		t.Errorf("expected code content in result, got %q", result)
	}
	if !strings.Contains(result, "hi") {
		t.Errorf("expected output content in result, got %q", result)
	}
	if !strings.Contains(result, "raw text") {
		t.Errorf("expected raw content in result, got %q", result)
	}
}

func TestFormatPanelSelectHighlight(t *testing.T) {
	content := "line 0\nline 1\nline 2"
	result := FormatPanelSelectHighlight(content, 1)

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Selected line should have highlight prefix
	if !strings.Contains(lines[1], "> ") {
		t.Errorf("expected highlight prefix on selected line, got %q", lines[1])
	}

	// Non-selected lines should have plain prefix
	if !strings.HasPrefix(lines[0], "  ") {
		t.Errorf("expected plain prefix on non-selected line, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "  ") {
		t.Errorf("expected plain prefix on non-selected line, got %q", lines[2])
	}
}

func TestEscapeTview(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"[bold]", "[[]bold]"},
		{"a[b]c", "a[[]b]c"},
		{"no brackets", "no brackets"},
	}
	for _, tt := range tests {
		result := escapeTview(tt.input)
		if result != tt.expected {
			t.Errorf("escapeTview(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
