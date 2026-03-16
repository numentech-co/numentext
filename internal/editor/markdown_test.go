package editor

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"numentext/internal/ui"
)

func baseStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
}

func TestParseMarkdownLine_PlainText(t *testing.T) {
	segs := ParseMarkdownLine("hello world", true, baseStyle())
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", segs[0].Text)
	}
}

func TestParseMarkdownLine_Bold_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("before **bold** after", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[0].Text != "before " {
		t.Errorf("seg 0: expected 'before ', got %q", segs[0].Text)
	}
	if segs[1].Text != "bold" {
		t.Errorf("seg 1: expected 'bold', got %q", segs[1].Text)
	}
	if segs[2].Text != " after" {
		t.Errorf("seg 2: expected ' after', got %q", segs[2].Text)
	}
}

func TestParseMarkdownLine_Bold_ShowMarkers(t *testing.T) {
	segs := ParseMarkdownLine("before **bold** after", false, baseStyle())
	// With markers shown: "before ", "**", "bold", "**", " after"
	if len(segs) != 5 {
		t.Fatalf("expected 5 segments, got %d", len(segs))
	}
	if segs[1].Text != "**" {
		t.Errorf("seg 1: expected '**', got %q", segs[1].Text)
	}
	if segs[2].Text != "bold" {
		t.Errorf("seg 2: expected 'bold', got %q", segs[2].Text)
	}
	if segs[3].Text != "**" {
		t.Errorf("seg 3: expected '**', got %q", segs[3].Text)
	}
}

func TestParseMarkdownLine_Italic_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("some *italic* text", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[1].Text != "italic" {
		t.Errorf("seg 1: expected 'italic', got %q", segs[1].Text)
	}
}

func TestParseMarkdownLine_BoldItalic_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("***bold italic***", true, baseStyle())
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Text != "bold italic" {
		t.Errorf("expected 'bold italic', got %q", segs[0].Text)
	}
}

func TestParseMarkdownLine_Code_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("use `fmt.Println` here", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[1].Text != "fmt.Println" {
		t.Errorf("seg 1: expected 'fmt.Println', got %q", segs[1].Text)
	}
}

func TestParseMarkdownLine_Link_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("click [here](http://example.com) now", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[1].Text != "here" {
		t.Errorf("seg 1: expected 'here', got %q", segs[1].Text)
	}
}

func TestParseMarkdownLine_ImageLink_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("see ![alt text](img.png) here", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[1].Text != "[img:alt text]" {
		t.Errorf("seg 1: expected '[img:alt text]', got %q", segs[1].Text)
	}
}

func TestParseMarkdownLine_Strikethrough_HideMarkers(t *testing.T) {
	segs := ParseMarkdownLine("not ~~deleted~~ text", true, baseStyle())
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[1].Text != "deleted" {
		t.Errorf("seg 1: expected 'deleted', got %q", segs[1].Text)
	}
}

func TestParseMarkdownLine_Header(t *testing.T) {
	tests := []struct {
		input       string
		hideMarkers bool
		wantText    string
	}{
		{"# Title", true, "Title"},
		{"## Subtitle", true, "Subtitle"},
		{"###### Tiny", true, "Tiny"},
		{"# Title", false, "Title"},  // second segment
		{"#NotAHeader", true, "#NotAHeader"}, // no space after #
	}
	for _, tt := range tests {
		segs := ParseMarkdownLine(tt.input, tt.hideMarkers, baseStyle())
		found := false
		for _, seg := range segs {
			if seg.Text == tt.wantText {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ParseMarkdownLine(%q, hide=%v): wanted segment with text %q, got segments: %v",
				tt.input, tt.hideMarkers, tt.wantText, segTexts(segs))
		}
	}
}

func TestParseMarkdownLine_HorizontalRule(t *testing.T) {
	tests := []string{"---", "***", "___", "- - -", "* * *"}
	for _, tt := range tests {
		segs := ParseMarkdownLine(tt, true, baseStyle())
		if len(segs) != 1 {
			t.Errorf("HR %q: expected 1 segment, got %d", tt, len(segs))
			continue
		}
		if segs[0].Text != "" {
			t.Errorf("HR %q: expected empty text (rendered as line), got %q", tt, segs[0].Text)
		}
	}
}

func TestParseMarkdownLine_NotHorizontalRule(t *testing.T) {
	// Two dashes is not a horizontal rule
	segs := ParseMarkdownLine("--", true, baseStyle())
	if len(segs) == 1 && segs[0].Text == "" {
		t.Error("'--' should not be treated as horizontal rule")
	}
}

func TestParseMarkdownLine_UnmatchedMarkers(t *testing.T) {
	// Unmatched bold marker should be literal
	segs := ParseMarkdownLine("before **unmatched", true, baseStyle())
	total := ""
	for _, s := range segs {
		total += s.Text
	}
	if total != "before **unmatched" {
		t.Errorf("unmatched bold: expected literal text, got %q", total)
	}

	// Unmatched backtick
	segs = ParseMarkdownLine("code ` here", true, baseStyle())
	total = ""
	for _, s := range segs {
		total += s.Text
	}
	if total != "code ` here" {
		t.Errorf("unmatched backtick: expected literal text, got %q", total)
	}
}

func TestVisualToBufferCol_NoMarkers(t *testing.T) {
	segs := ParseMarkdownLine("hello world", true, baseStyle())
	if got := VisualToBufferCol(segs, 5); got != 5 {
		t.Errorf("VisualToBufferCol(5) = %d, want 5", got)
	}
}

func TestVisualToBufferCol_HiddenBold(t *testing.T) {
	// "before **bold** after" with hidden markers:
	// Buffer: b(0)e(1)f(2)o(3)r(4)e(5) (6)*(7)*(8)b(9)o(10)l(11)d(12)*(13)*(14) (15)a(16)...
	// Visual: "before bold after"
	// Visual: b(0)e(1)f(2)o(3)r(4)e(5) (6)b(7)o(8)l(9)d(10) (11)a(12)...
	segs := ParseMarkdownLine("before **bold** after", true, baseStyle())
	// Visual col 7 = 'b' in "bold" -> buffer col 9 (after "before **")
	got := VisualToBufferCol(segs, 7)
	if got != 9 {
		t.Errorf("VisualToBufferCol(7) = %d, want 9", got)
	}
	// Visual col 11 = ' ' after "bold" -> buffer col 15 (the space after **)
	got = VisualToBufferCol(segs, 11)
	if got != 15 {
		t.Errorf("VisualToBufferCol(11) = %d, want 15", got)
	}
}

func TestBufferToVisualCol_HiddenBold(t *testing.T) {
	segs := ParseMarkdownLine("before **bold** after", true, baseStyle())
	// Buffer col 9 ('b' in bold) -> visual col 7
	got := BufferToVisualCol(segs, 9)
	if got != 7 {
		t.Errorf("BufferToVisualCol(9) = %d, want 7", got)
	}
	// Buffer col 15 (' ' after **bold**) -> visual col 11
	got = BufferToVisualCol(segs, 15)
	if got != 11 {
		t.Errorf("BufferToVisualCol(15) = %d, want 11", got)
	}
}

func TestBufferToVisualCol_Header(t *testing.T) {
	segs := ParseMarkdownLine("# Title", true, baseStyle())
	// Buffer col 2 ('T') -> visual col 0 (since "# " is hidden)
	got := BufferToVisualCol(segs, 2)
	if got != 0 {
		t.Errorf("BufferToVisualCol(2) = %d, want 0", got)
	}
}

func TestVisualToBufferCol_Header(t *testing.T) {
	segs := ParseMarkdownLine("# Title", true, baseStyle())
	// Visual col 0 ('T') -> buffer col 2 (after "# " prefix)
	got := VisualToBufferCol(segs, 0)
	if got != 2 {
		t.Errorf("VisualToBufferCol(0) = %d, want 2", got)
	}
}

func TestParseMarkdownLine_MultipleCodeSpans(t *testing.T) {
	segs := ParseMarkdownLine("use `a` and `b` here", true, baseStyle())
	total := ""
	for _, s := range segs {
		total += s.Text
	}
	if total != "use a and b here" {
		t.Errorf("multiple code spans: expected 'use a and b here', got %q", total)
	}
}

func TestIsMarkdownFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"README.md", true},
		{"doc.markdown", true},
		{"README.MD", true},
		{"main.go", false},
		{"file.txt", false},
	}
	for _, tt := range tests {
		if got := isMarkdownFile(tt.path); got != tt.want {
			t.Errorf("isMarkdownFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsHorizontalRule(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"---", true},
		{"***", true},
		{"___", true},
		{"- - -", true},
		{"--", false},
		{"abc", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHorizontalRule(tt.input); got != tt.want {
			t.Errorf("isHorizontalRule(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseHeader(t *testing.T) {
	tests := []struct {
		input     string
		wantLevel int
		wantText  string
	}{
		{"# Title", 1, "Title"},
		{"## Sub", 2, "Sub"},
		{"###### H6", 6, "H6"},
		{"#NoSpace", 0, ""},
		{"####### TooMany", 0, ""},
		{"plain text", 0, ""},
	}
	for _, tt := range tests {
		level, text, _ := parseHeader(tt.input)
		if level != tt.wantLevel {
			t.Errorf("parseHeader(%q): level = %d, want %d", tt.input, level, tt.wantLevel)
		}
		if level > 0 && text != tt.wantText {
			t.Errorf("parseHeader(%q): text = %q, want %q", tt.input, text, tt.wantText)
		}
	}
}

func TestParseMarkdownLine_YAMLFrontmatter(t *testing.T) {
	// --- at line 0 should NOT be treated as horizontal rule (YAML frontmatter)
	segs := ParseMarkdownLine("---", true, baseStyle(), 0)
	if len(segs) == 1 && segs[0].Text == "" {
		t.Error("--- at line 0 should not be rendered as horizontal rule")
	}
	// --- at line 5 SHOULD be a horizontal rule
	segs = ParseMarkdownLine("---", true, baseStyle(), 5)
	if len(segs) != 1 || segs[0].Text != "" {
		t.Error("--- at line 5 should be rendered as horizontal rule")
	}
}

// segTexts returns the Text field from each segment for debugging.
func segTexts(segs []MarkdownSegment) []string {
	var out []string
	for _, s := range segs {
		out = append(out, s.Text)
	}
	return out
}
