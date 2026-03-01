package editor

import (
	"testing"

	"numentext/internal/ui"
)

// === Story 2.1: Language Detection ===

func TestHighlighter_DetectGo(t *testing.T) {
	h := NewHighlighter("main.go")
	if h.Language() != "Go" {
		t.Errorf("expected Go, got %s", h.Language())
	}
}

func TestHighlighter_DetectPython(t *testing.T) {
	h := NewHighlighter("app.py")
	if h.Language() != "Python" {
		t.Errorf("expected Python, got %s", h.Language())
	}
}

func TestHighlighter_DetectC(t *testing.T) {
	h := NewHighlighter("main.c")
	if h.Language() != "C" {
		t.Errorf("expected C, got %s", h.Language())
	}
}

func TestHighlighter_DetectCpp(t *testing.T) {
	h := NewHighlighter("main.cpp")
	if h.Language() != "C++" {
		t.Errorf("expected C++, got %s", h.Language())
	}
}

func TestHighlighter_DetectRust(t *testing.T) {
	h := NewHighlighter("main.rs")
	if h.Language() != "Rust" {
		t.Errorf("expected Rust, got %s", h.Language())
	}
}

func TestHighlighter_DetectJavaScript(t *testing.T) {
	h := NewHighlighter("app.js")
	if h.Language() != "JavaScript" {
		t.Errorf("expected JavaScript, got %s", h.Language())
	}
}

func TestHighlighter_DetectTypeScript(t *testing.T) {
	h := NewHighlighter("app.ts")
	if h.Language() != "TypeScript" {
		t.Errorf("expected TypeScript, got %s", h.Language())
	}
}

func TestHighlighter_DetectJava(t *testing.T) {
	h := NewHighlighter("Main.java")
	if h.Language() != "Java" {
		t.Errorf("expected Java, got %s", h.Language())
	}
}

func TestHighlighter_DetectCSS(t *testing.T) {
	h := NewHighlighter("styles.css")
	if h.Language() != "CSS" {
		t.Errorf("expected CSS, got %s", h.Language())
	}
}

func TestHighlighter_FallbackToText(t *testing.T) {
	h := NewHighlighter("README.txt")
	if h.Language() != "Text" {
		t.Errorf("expected Text, got %s", h.Language())
	}
}

func TestHighlighter_RedetectOnRename(t *testing.T) {
	h := NewHighlighter("main.c")
	if h.Language() != "C" {
		t.Errorf("expected C, got %s", h.Language())
	}
	h.DetectLanguage("main.py")
	if h.Language() != "Python" {
		t.Errorf("expected Python after rename, got %s", h.Language())
	}
}

// === Story 2.2: Token Colorization ===

func TestHighlighter_GoKeywordIsWhiteBold(t *testing.T) {
	h := NewHighlighter("main.go")
	lines := h.Highlight("func main() {}")
	// "func" starts at col 0
	if len(lines) == 0 || len(lines[0].Styles) < 4 {
		t.Fatal("expected highlighted output")
	}
	// "func" should be keyword = white + bold
	s := lines[0].Styles[0]
	if s.Fg != ui.ColorKeyword {
		t.Errorf("expected keyword color for 'f' in func, got %v", s.Fg)
	}
	if !s.Bold {
		t.Error("expected bold for keyword 'func'")
	}
}

func TestHighlighter_GoStringIsCyan(t *testing.T) {
	h := NewHighlighter("main.go")
	lines := h.Highlight(`x := "hello"`)
	// Find the 'h' in "hello" - it's at col 6
	found := false
	for i, s := range lines[0].Styles {
		if i >= 6 && i <= 10 {
			if s.Fg == ui.ColorString {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected string color (cyan) for string literal")
	}
}

func TestHighlighter_GoCommentIsGray(t *testing.T) {
	h := NewHighlighter("main.go")
	lines := h.Highlight("// this is a comment")
	if len(lines) == 0 || len(lines[0].Styles) < 3 {
		t.Fatal("expected highlighted output")
	}
	s := lines[0].Styles[0]
	if s.Fg != ui.ColorComment {
		t.Errorf("expected comment color for '//', got %v", s.Fg)
	}
}

func TestHighlighter_NumberIsMagenta(t *testing.T) {
	h := NewHighlighter("main.go")
	lines := h.Highlight("x := 42")
	// '4' at col 5
	if len(lines) == 0 || len(lines[0].Styles) < 6 {
		t.Fatal("expected highlighted output")
	}
	s := lines[0].Styles[5]
	if s.Fg != ui.ColorNumber {
		t.Errorf("expected number color for '4', got %v", s.Fg)
	}
}

// === Story 2.3: Highlighting Performance ===

func TestHighlighter_MultiLineComment(t *testing.T) {
	h := NewHighlighter("main.go")
	code := "/*\nline inside comment\n*/\nx := 1"
	lines := h.Highlight(code)
	if len(lines) < 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	// Line 1 (inside comment) should be gray
	if len(lines[1].Styles) > 0 && lines[1].Styles[0].Fg != ui.ColorComment {
		t.Errorf("line inside block comment should be gray, got %v", lines[1].Styles[0].Fg)
	}
}

func TestHighlighter_PlainTextNoStyles(t *testing.T) {
	h := NewHighlighter("readme.txt")
	lines := h.Highlight("just plain text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// All chars should have default text color
	for i, s := range lines[0].Styles {
		if s.Fg != ui.ColorText {
			t.Errorf("char %d should have default text color, got %v", i, s.Fg)
		}
	}
}
