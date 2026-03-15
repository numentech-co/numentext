package editor

import (
	"testing"

	"numentext/internal/ui"
)

func TestIsBracket(t *testing.T) {
	brackets := []byte{'(', ')', '[', ']', '{', '}'}
	for _, b := range brackets {
		if !isBracket(b) {
			t.Errorf("expected %c to be a bracket", b)
		}
	}
	nonBrackets := []byte{'a', '1', ' ', '+', '"'}
	for _, b := range nonBrackets {
		if isBracket(b) {
			t.Errorf("expected %c to NOT be a bracket", b)
		}
	}
}

func TestIsOpenBracket(t *testing.T) {
	if !isOpenBracket('(') {
		t.Error("( should be open bracket")
	}
	if !isOpenBracket('[') {
		t.Error("[ should be open bracket")
	}
	if !isOpenBracket('{') {
		t.Error("{ should be open bracket")
	}
	if isOpenBracket(')') {
		t.Error(") should NOT be open bracket")
	}
}

// makeHighlighted creates dummy highlight data with all code-style colors
// (not string/comment) for the given lines.
func makeHighlighted(lines []string) []HighlightedLine {
	result := make([]HighlightedLine, len(lines))
	for i, line := range lines {
		styles := make([]CharStyle, len(line))
		for j := range styles {
			styles[j] = CharStyle{Fg: ui.ColorText}
		}
		result[i] = HighlightedLine{Styles: styles}
	}
	return result
}

// makeHighlightedWithStringAt creates highlight data where certain byte ranges
// are marked as string color.
func makeHighlightedWithStringAt(lines []string, stringRanges map[int][2]int) []HighlightedLine {
	result := makeHighlighted(lines)
	for lineIdx, rng := range stringRanges {
		for col := rng[0]; col < rng[1] && col < len(result[lineIdx].Styles); col++ {
			result[lineIdx].Styles[col] = CharStyle{Fg: ui.ColorString}
		}
	}
	return result
}

func newTestEditor(text string) *Editor {
	e := NewEditor()
	e.NewTab("test.go", "", text)
	return e
}

func TestFindMatchingBracket_SimpleParens(t *testing.T) {
	e := newTestEditor("foo(bar)")
	tab := e.ActiveTab()
	lines := []string{"foo(bar)"}
	hl := makeHighlighted(lines)

	// Cursor on '(' at col 3
	tab.CursorRow = 0
	tab.CursorCol = 3
	m := e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find bracket at cursor")
	}
	if !m.HasMatch {
		t.Fatal("expected to find matching bracket")
	}
	if m.MatchLine != 0 || m.MatchCol != 7 {
		t.Errorf("expected match at (0,7), got (%d,%d)", m.MatchLine, m.MatchCol)
	}

	// Cursor on ')' at col 7
	tab.CursorCol = 7
	m = e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find bracket at cursor")
	}
	if !m.HasMatch {
		t.Fatal("expected to find matching bracket")
	}
	if m.MatchLine != 0 || m.MatchCol != 3 {
		t.Errorf("expected match at (0,3), got (%d,%d)", m.MatchLine, m.MatchCol)
	}
}

func TestFindMatchingBracket_MultiLine(t *testing.T) {
	text := "func() {\n  return\n}"
	e := newTestEditor(text)
	tab := e.ActiveTab()
	lines := []string{"func() {", "  return", "}"}
	hl := makeHighlighted(lines)

	// Cursor on '{' at line 0, col 7
	tab.CursorRow = 0
	tab.CursorCol = 7
	m := e.FindMatchingBracket(tab, hl)
	if !m.HasMatch {
		t.Fatal("expected match for {")
	}
	if m.MatchLine != 2 || m.MatchCol != 0 {
		t.Errorf("expected match at (2,0), got (%d,%d)", m.MatchLine, m.MatchCol)
	}

	// Cursor on '}' at line 2, col 0
	tab.CursorRow = 2
	tab.CursorCol = 0
	m = e.FindMatchingBracket(tab, hl)
	if !m.HasMatch {
		t.Fatal("expected match for }")
	}
	if m.MatchLine != 0 || m.MatchCol != 7 {
		t.Errorf("expected match at (0,7), got (%d,%d)", m.MatchLine, m.MatchCol)
	}
}

func TestFindMatchingBracket_Nested(t *testing.T) {
	e := newTestEditor("((()))")
	tab := e.ActiveTab()
	hl := makeHighlighted([]string{"((()))"})

	// Cursor on outer '(' at col 0 -> should match outer ')' at col 5
	tab.CursorRow = 0
	tab.CursorCol = 0
	m := e.FindMatchingBracket(tab, hl)
	if !m.HasMatch {
		t.Fatal("expected match")
	}
	if m.MatchCol != 5 {
		t.Errorf("expected match at col 5, got %d", m.MatchCol)
	}

	// Cursor on middle '(' at col 1 -> should match middle ')' at col 4
	tab.CursorCol = 1
	m = e.FindMatchingBracket(tab, hl)
	if !m.HasMatch {
		t.Fatal("expected match")
	}
	if m.MatchCol != 4 {
		t.Errorf("expected match at col 4, got %d", m.MatchCol)
	}

	// Cursor on inner '(' at col 2 -> should match inner ')' at col 3
	tab.CursorCol = 2
	m = e.FindMatchingBracket(tab, hl)
	if !m.HasMatch {
		t.Fatal("expected match")
	}
	if m.MatchCol != 3 {
		t.Errorf("expected match at col 3, got %d", m.MatchCol)
	}
}

func TestFindMatchingBracket_Unmatched(t *testing.T) {
	e := newTestEditor("foo(bar")
	tab := e.ActiveTab()
	hl := makeHighlighted([]string{"foo(bar"})

	// Cursor on '(' at col 3 with no closing bracket
	tab.CursorRow = 0
	tab.CursorCol = 3
	m := e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find bracket at cursor")
	}
	if m.HasMatch {
		t.Fatal("expected unmatched bracket")
	}
}

func TestFindMatchingBracket_NoBracketAtCursor(t *testing.T) {
	e := newTestEditor("hello world")
	tab := e.ActiveTab()
	hl := makeHighlighted([]string{"hello world"})

	tab.CursorRow = 0
	tab.CursorCol = 3
	m := e.FindMatchingBracket(tab, hl)
	if m.FoundBracket {
		t.Fatal("expected no bracket at cursor")
	}
}

func TestFindMatchingBracket_SkipsStrings(t *testing.T) {
	// The ')' inside the string should be skipped
	text := `foo(")", bar)`
	e := newTestEditor(text)
	tab := e.ActiveTab()
	lines := []string{text}

	// Mark the string portion as string-colored: chars at indices 4..7 are ")"
	hl := makeHighlightedWithStringAt(lines, map[int][2]int{
		0: {4, 8}, // ")" is indices 4,5,6,7
	})

	// Cursor on '(' at col 3
	tab.CursorRow = 0
	tab.CursorCol = 3
	m := e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find bracket")
	}
	if !m.HasMatch {
		t.Fatal("expected match (skipping string)")
	}
	// Should match the ')' at col 12, not the one inside the string
	if m.MatchCol != 12 {
		t.Errorf("expected match at col 12, got %d", m.MatchCol)
	}
}

func TestFindMatchingBracket_SkipsComments(t *testing.T) {
	text := "foo(bar // )\n)"
	e := newTestEditor(text)
	tab := e.ActiveTab()
	lines := []string{"foo(bar // )", ")"}

	// Mark the comment as comment-colored
	hl := makeHighlighted(lines)
	// Indices 8..11 in line 0 are "// )" — mark as comment
	for col := 8; col < 12 && col < len(hl[0].Styles); col++ {
		hl[0].Styles[col] = CharStyle{Fg: ui.ColorComment}
	}

	// Cursor on '(' at col 3
	tab.CursorRow = 0
	tab.CursorCol = 3
	m := e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find bracket")
	}
	if !m.HasMatch {
		t.Fatal("expected match (skipping comment)")
	}
	// Should match ')' on line 1, col 0
	if m.MatchLine != 1 || m.MatchCol != 0 {
		t.Errorf("expected match at (1,0), got (%d,%d)", m.MatchLine, m.MatchCol)
	}
}

func TestFindMatchingBracket_AdjacentBracket(t *testing.T) {
	// When cursor is just after a bracket (adjacent), it should still match
	e := newTestEditor("(foo)")
	tab := e.ActiveTab()
	hl := makeHighlighted([]string{"(foo)"})

	// Cursor at col 1 — just after '(' at col 0
	tab.CursorRow = 0
	tab.CursorCol = 1
	m := e.FindMatchingBracket(tab, hl)
	if !m.FoundBracket {
		t.Fatal("expected to find adjacent bracket")
	}
	if !m.HasMatch {
		t.Fatal("expected match")
	}
	if m.StartCol != 0 {
		t.Errorf("expected start bracket at col 0, got %d", m.StartCol)
	}
	if m.MatchCol != 4 {
		t.Errorf("expected match at col 4, got %d", m.MatchCol)
	}
}

func TestFindMatchingBracket_AllBracketTypes(t *testing.T) {
	tests := []struct {
		text     string
		curCol   int
		matchCol int
	}{
		{"()", 0, 1},
		{"[]", 0, 1},
		{"{}", 0, 1},
		{"()", 1, 0},
		{"[]", 1, 0},
		{"{}", 1, 0},
	}
	for _, tt := range tests {
		e := newTestEditor(tt.text)
		tab := e.ActiveTab()
		hl := makeHighlighted([]string{tt.text})

		tab.CursorRow = 0
		tab.CursorCol = tt.curCol
		m := e.FindMatchingBracket(tab, hl)
		if !m.HasMatch {
			t.Errorf("text=%q curCol=%d: expected match", tt.text, tt.curCol)
			continue
		}
		if m.MatchCol != tt.matchCol {
			t.Errorf("text=%q curCol=%d: expected matchCol=%d, got %d", tt.text, tt.curCol, tt.matchCol, m.MatchCol)
		}
	}
}

func TestFindMatchingBracket_BracketInString(t *testing.T) {
	// Cursor on a bracket that is itself inside a string — should not match
	e := newTestEditor(`"("`)
	tab := e.ActiveTab()
	hl := makeHighlightedWithStringAt([]string{`"("`}, map[int][2]int{
		0: {0, 3}, // entire thing is a string
	})

	tab.CursorRow = 0
	tab.CursorCol = 1 // the '(' inside the string
	m := e.FindMatchingBracket(tab, hl)
	// The bracket at cursor is in a string, so FoundBracket should be false
	if m.FoundBracket {
		t.Error("bracket inside string should not trigger matching")
	}
}
