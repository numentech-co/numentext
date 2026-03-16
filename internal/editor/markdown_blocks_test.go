package editor

import (
	"testing"
)

func TestDetectFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  bool
		end   int
	}{
		{
			name:  "valid frontmatter",
			lines: []string{"---", "title: Hello", "date: 2024-01-01", "---", "# Content"},
			want:  true,
			end:   3,
		},
		{
			name:  "no frontmatter",
			lines: []string{"# Hello", "Some text"},
			want:  false,
		},
		{
			name:  "not at line 0",
			lines: []string{"", "---", "title: Hello", "---"},
			want:  false,
		},
		{
			name:  "unclosed frontmatter",
			lines: []string{"---", "title: Hello", "no closing"},
			want:  false,
		},
		{
			name:  "too short",
			lines: []string{"---"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := DetectBlocks(tt.lines)
			found := false
			for _, b := range blocks {
				if b.Type == BlockFrontmatter {
					found = true
					if b.EndLine != tt.end {
						t.Errorf("EndLine = %d, want %d", b.EndLine, tt.end)
					}
				}
			}
			if found != tt.want {
				t.Errorf("frontmatter found = %v, want %v", found, tt.want)
			}
		})
	}
}

func TestDetectFencedCodeBlocks(t *testing.T) {
	lines := []string{
		"# Title",
		"Some text",
		"```go",
		"func main() {}",
		"```",
		"More text",
		"```",
		"plain code",
		"```",
	}

	blocks := DetectBlocks(lines)
	var codeBlocks []BlockInfo
	for _, b := range blocks {
		if b.Type == BlockFencedCode {
			codeBlocks = append(codeBlocks, b)
		}
	}

	if len(codeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(codeBlocks))
	}

	if codeBlocks[0].StartLine != 2 || codeBlocks[0].EndLine != 4 {
		t.Errorf("block 0: start=%d end=%d, want 2-4", codeBlocks[0].StartLine, codeBlocks[0].EndLine)
	}
	if codeBlocks[0].Language != "go" {
		t.Errorf("block 0 language = %q, want %q", codeBlocks[0].Language, "go")
	}

	if codeBlocks[1].StartLine != 6 || codeBlocks[1].EndLine != 8 {
		t.Errorf("block 1: start=%d end=%d, want 6-8", codeBlocks[1].StartLine, codeBlocks[1].EndLine)
	}
	if codeBlocks[1].Language != "" {
		t.Errorf("block 1 language = %q, want empty", codeBlocks[1].Language)
	}
}

func TestDetectTables(t *testing.T) {
	lines := []string{
		"# Title",
		"| Name | Age |",
		"| --- | --- |",
		"| Alice | 30 |",
		"| Bob | 25 |",
		"",
		"More text",
	}

	blocks := DetectBlocks(lines)
	var tables []BlockInfo
	for _, b := range blocks {
		if b.Type == BlockTable {
			tables = append(tables, b)
		}
	}

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	tb := tables[0]
	if tb.StartLine != 1 || tb.EndLine != 4 {
		t.Errorf("table: start=%d end=%d, want 1-4", tb.StartLine, tb.EndLine)
	}

	if len(tb.ColWidths) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(tb.ColWidths))
	}

	// "Alice" = 5 chars, "Name" = 4, so width should be 5
	if tb.ColWidths[0] != 5 {
		t.Errorf("col 0 width = %d, want 5", tb.ColWidths[0])
	}
}

func TestDetectTableAlignment(t *testing.T) {
	lines := []string{
		"| Left | Center | Right |",
		"| :--- | :---: | ---: |",
		"| a | b | c |",
	}

	blocks := DetectBlocks(lines)
	var tables []BlockInfo
	for _, b := range blocks {
		if b.Type == BlockTable {
			tables = append(tables, b)
		}
	}

	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	tb := tables[0]
	if len(tb.ColAligns) < 3 {
		t.Fatalf("expected 3 column alignments, got %d", len(tb.ColAligns))
	}

	if tb.ColAligns[0] != -1 {
		t.Errorf("col 0 align = %d, want -1 (left)", tb.ColAligns[0])
	}
	if tb.ColAligns[1] != 0 {
		t.Errorf("col 1 align = %d, want 0 (center)", tb.ColAligns[1])
	}
	if tb.ColAligns[2] != 1 {
		t.Errorf("col 2 align = %d, want 1 (right)", tb.ColAligns[2])
	}
}

func TestDetectBlockquotes(t *testing.T) {
	lines := []string{
		"# Title",
		"> First line",
		"> Second line",
		"",
		"Normal",
		"> Another quote",
	}

	blocks := DetectBlocks(lines)
	var quotes []BlockInfo
	for _, b := range blocks {
		if b.Type == BlockBlockquote {
			quotes = append(quotes, b)
		}
	}

	if len(quotes) != 2 {
		t.Fatalf("expected 2 blockquotes, got %d", len(quotes))
	}

	if quotes[0].StartLine != 1 || quotes[0].EndLine != 2 {
		t.Errorf("quote 0: start=%d end=%d, want 1-2", quotes[0].StartLine, quotes[0].EndLine)
	}
	if quotes[1].StartLine != 5 || quotes[1].EndLine != 5 {
		t.Errorf("quote 1: start=%d end=%d, want 5-5", quotes[1].StartLine, quotes[1].EndLine)
	}
}

func TestDetectLists(t *testing.T) {
	lines := []string{
		"# Title",
		"- item one",
		"- item two",
		"  - nested",
		"",
		"1. first",
		"2. second",
	}

	blocks := DetectBlocks(lines)
	var lists []BlockInfo
	for _, b := range blocks {
		if b.Type == BlockList {
			lists = append(lists, b)
		}
	}

	if len(lists) != 2 {
		t.Fatalf("expected 2 lists, got %d", len(lists))
	}

	if lists[0].StartLine != 1 || lists[0].EndLine != 3 {
		t.Errorf("list 0: start=%d end=%d, want 1-3", lists[0].StartLine, lists[0].EndLine)
	}
	if lists[1].StartLine != 5 || lists[1].EndLine != 6 {
		t.Errorf("list 1: start=%d end=%d, want 5-6", lists[1].StartLine, lists[1].EndLine)
	}
}

func TestIsInBlock(t *testing.T) {
	blocks := []BlockInfo{
		{Type: BlockFencedCode, StartLine: 2, EndLine: 5},
		{Type: BlockTable, StartLine: 8, EndLine: 11},
	}

	if b := IsInBlock(blocks, 0); b != nil {
		t.Error("line 0 should not be in a block")
	}
	if b := IsInBlock(blocks, 3); b == nil || b.Type != BlockFencedCode {
		t.Error("line 3 should be in fenced code block")
	}
	if b := IsInBlock(blocks, 9); b == nil || b.Type != BlockTable {
		t.Error("line 9 should be in table block")
	}
}

func TestBlockquoteDepth(t *testing.T) {
	tests := []struct {
		line  string
		depth int
	}{
		{"> hello", 1},
		{">> nested", 2},
		{"> > also nested", 2},
		{"no quote", 0},
	}

	for _, tt := range tests {
		d := BlockquoteDepth(tt.line)
		if d != tt.depth {
			t.Errorf("BlockquoteDepth(%q) = %d, want %d", tt.line, d, tt.depth)
		}
	}
}

func TestBlockquoteContent(t *testing.T) {
	tests := []struct {
		line    string
		content string
	}{
		{"> hello", "hello"},
		{">> nested", "nested"},
		{"> > also nested", "also nested"},
	}

	for _, tt := range tests {
		c := BlockquoteContent(tt.line)
		if c != tt.content {
			t.Errorf("BlockquoteContent(%q) = %q, want %q", tt.line, c, tt.content)
		}
	}
}

func TestParseListLine(t *testing.T) {
	tests := []struct {
		line       string
		indent     int
		isOrdered  bool
		isCheckbox bool
		isChecked  bool
		content    string
		rawMarker  string
	}{
		{"- item", 0, false, false, false, "item", "-"},
		{"  * nested", 2, false, false, false, "nested", "*"},
		{"1. first", 0, true, false, false, "first", "1."},
		{"- [ ] todo", 0, false, true, false, "todo", "- [ ]"},
		{"- [x] done", 0, false, true, true, "done", "- [x]"},
		{"    + deep", 4, false, false, false, "deep", "+"},
	}

	for _, tt := range tests {
		info := ParseListLine(tt.line)
		if info.Indent != tt.indent {
			t.Errorf("ParseListLine(%q).Indent = %d, want %d", tt.line, info.Indent, tt.indent)
		}
		if info.IsOrdered != tt.isOrdered {
			t.Errorf("ParseListLine(%q).IsOrdered = %v, want %v", tt.line, info.IsOrdered, tt.isOrdered)
		}
		if info.IsCheckbox != tt.isCheckbox {
			t.Errorf("ParseListLine(%q).IsCheckbox = %v, want %v", tt.line, info.IsCheckbox, tt.isCheckbox)
		}
		if info.IsChecked != tt.isChecked {
			t.Errorf("ParseListLine(%q).IsChecked = %v, want %v", tt.line, info.IsChecked, tt.isChecked)
		}
		if info.Content != tt.content {
			t.Errorf("ParseListLine(%q).Content = %q, want %q", tt.line, info.Content, tt.content)
		}
		if info.RawMarker != tt.rawMarker {
			t.Errorf("ParseListLine(%q).RawMarker = %q, want %q", tt.line, info.RawMarker, tt.rawMarker)
		}
	}
}

func TestRenderTableRow(t *testing.T) {
	colWidths := []int{5, 3}
	colAligns := []int{-1, 1} // left, right

	row := "| Alice | 30 |"
	result := RenderTableRow(row, colWidths, colAligns)

	// Should contain Alice padded to 5 and 30 right-aligned to 3
	if len(result) == 0 {
		t.Fatal("RenderTableRow returned empty string")
	}

	// Verify it contains the content
	if !containsStr(result, "Alice") {
		t.Errorf("result %q does not contain 'Alice'", result)
	}
	if !containsStr(result, "30") {
		t.Errorf("result %q does not contain '30'", result)
	}
}

func TestPadCell(t *testing.T) {
	tests := []struct {
		content string
		width   int
		align   int
		want    string
	}{
		{"hi", 5, -1, "hi   "},
		{"hi", 5, 1, "   hi"},
		{"hi", 5, 0, " hi  "},
		{"hello", 5, -1, "hello"},
		{"toolong", 5, -1, "toolo"},
	}

	for _, tt := range tests {
		got := padCell(tt.content, tt.width, tt.align)
		if got != tt.want {
			t.Errorf("padCell(%q, %d, %d) = %q, want %q", tt.content, tt.width, tt.align, got, tt.want)
		}
	}
}

func TestIsMarkdownFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"README.md", true},
		{"notes.markdown", true},
		{"file.go", false},
		{"file.MD", true},
		{"doc.mdown", true},
	}

	for _, tt := range tests {
		got := IsMarkdownFile(tt.name)
		if got != tt.want {
			t.Errorf("IsMarkdownFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCodeBlockNotInsideFrontmatter(t *testing.T) {
	// Fenced code after frontmatter should not conflict
	lines := []string{
		"---",
		"title: Test",
		"---",
		"",
		"```go",
		"package main",
		"```",
	}

	blocks := DetectBlocks(lines)
	var fm, code int
	for _, b := range blocks {
		switch b.Type {
		case BlockFrontmatter:
			fm++
		case BlockFencedCode:
			code++
		}
	}

	if fm != 1 {
		t.Errorf("expected 1 frontmatter block, got %d", fm)
	}
	if code != 1 {
		t.Errorf("expected 1 code block, got %d", code)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
