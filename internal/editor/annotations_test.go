package editor

import (
	"testing"
)

func TestScanBuffer_GoSingleLine(t *testing.T) {
	lines := []string{
		"package main",
		"",
		"// TODO: fix this",
		"func main() {}",
	}
	anns := ScanBuffer("main.go", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "TODO" {
		t.Errorf("expected tag TODO, got %s", anns[0].Tag)
	}
	if anns[0].Text != "fix this" {
		t.Errorf("expected text 'fix this', got '%s'", anns[0].Text)
	}
	if anns[0].Line != 3 {
		t.Errorf("expected line 3, got %d", anns[0].Line)
	}
}

func TestScanBuffer_PythonHash(t *testing.T) {
	lines := []string{
		"# FIXME: broken",
		"def foo():",
		"    pass",
	}
	anns := ScanBuffer("test.py", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "FIXME" {
		t.Errorf("expected tag FIXME, got %s", anns[0].Tag)
	}
	if anns[0].Text != "broken" {
		t.Errorf("expected text 'broken', got '%s'", anns[0].Text)
	}
}

func TestScanBuffer_CBlockComment(t *testing.T) {
	lines := []string{
		"/* BUG: race condition */",
		"int x = 0;",
	}
	anns := ScanBuffer("test.c", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "BUG" {
		t.Errorf("expected tag BUG, got %s", anns[0].Tag)
	}
	if anns[0].Text != "race condition" {
		t.Errorf("expected text 'race condition', got '%s'", anns[0].Text)
	}
}

func TestScanBuffer_SpaceSeparator(t *testing.T) {
	lines := []string{
		"// TODO fix spacing",
	}
	anns := ScanBuffer("test.go", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "TODO" {
		t.Errorf("expected tag TODO, got %s", anns[0].Tag)
	}
	if anns[0].Text != "fix spacing" {
		t.Errorf("expected text 'fix spacing', got '%s'", anns[0].Text)
	}
}

func TestScanBuffer_CaseInsensitive(t *testing.T) {
	lines := []string{
		"// todo: lowercase tag",
		"# Fixme: mixed case",
	}
	anns := ScanBuffer("test.go", lines)
	if len(anns) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(anns))
	}
	if anns[0].Tag != "TODO" {
		t.Errorf("expected tag TODO, got %s", anns[0].Tag)
	}
	if anns[1].Tag != "FIXME" {
		t.Errorf("expected tag FIXME, got %s", anns[1].Tag)
	}
}

func TestScanBuffer_IgnoresNonComment(t *testing.T) {
	lines := []string{
		`fmt.Println("TODO: not a real todo")`,
		"x := 42",
	}
	anns := ScanBuffer("test.go", lines)
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(anns))
	}
}

func TestScanBuffer_MultipleFiles(t *testing.T) {
	tab1 := &Tab{
		Name:     "a.go",
		FilePath: "a.go",
		Buffer:   NewBufferFromText("// TODO: first\ncode"),
	}
	tab2 := &Tab{
		Name:     "b.py",
		FilePath: "b.py",
		Buffer:   NewBufferFromText("# HACK: workaround\ndef x(): pass"),
	}
	tab3 := &Tab{
		Name:     "c.c",
		FilePath: "c.c",
		Buffer:   NewBufferFromText("/* XXX: check this */\nint x;"),
	}
	anns := ScanAllTabs([]*Tab{tab1, tab2, tab3})
	if len(anns) != 3 {
		t.Fatalf("expected 3 annotations from 3 files, got %d", len(anns))
	}
}

func TestScanBuffer_MultiLineBlock(t *testing.T) {
	lines := []string{
		"/*",
		" * WONTFIX: legacy behavior",
		" */",
		"int y = 1;",
	}
	anns := ScanBuffer("test.c", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "WONTFIX" {
		t.Errorf("expected tag WONTFIX, got %s", anns[0].Tag)
	}
	if anns[0].Line != 2 {
		t.Errorf("expected line 2, got %d", anns[0].Line)
	}
}

func TestScanBuffer_AllTags(t *testing.T) {
	lines := []string{
		"// TODO: a",
		"// FIXME: b",
		"// BUG: c",
		"// HACK: d",
		"// WONTFIX: e",
		"// XXX: f",
		"// NOTE: g",
	}
	anns := ScanBuffer("test.go", lines)
	if len(anns) != 7 {
		t.Fatalf("expected 7 annotations, got %d", len(anns))
	}
	expected := []string{"TODO", "FIXME", "BUG", "HACK", "WONTFIX", "XXX", "NOTE"}
	for i, tag := range expected {
		if anns[i].Tag != tag {
			t.Errorf("annotation %d: expected tag %s, got %s", i, tag, anns[i].Tag)
		}
	}
}

func TestScanBuffer_SQLComment(t *testing.T) {
	lines := []string{
		"-- TODO: add index",
		"SELECT * FROM users;",
	}
	anns := ScanBuffer("test.sql", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "TODO" {
		t.Errorf("expected tag TODO, got %s", anns[0].Tag)
	}
}

func TestScanBuffer_SemicolonComment(t *testing.T) {
	lines := []string{
		"; NOTE: lisp style comment",
	}
	anns := ScanBuffer("test.el", lines)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Tag != "NOTE" {
		t.Errorf("expected tag NOTE, got %s", anns[0].Tag)
	}
}
