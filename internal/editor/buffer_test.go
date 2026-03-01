package editor

import (
	"testing"
)

// === Story 1.1: Text Buffer Management ===

func TestNewBuffer_InsertAtZero(t *testing.T) {
	b := NewBuffer()
	b.Insert(0, 0, "hello", [2]int{0, 0})
	if b.Line(0) != "hello" {
		t.Errorf("expected 'hello', got %q", b.Line(0))
	}
	if b.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", b.LineCount())
	}
}

func TestBuffer_InsertMiddle(t *testing.T) {
	b := NewBufferFromText("helloworld")
	b.Insert(0, 5, " ", [2]int{0, 5})
	if b.Line(0) != "hello world" {
		t.Errorf("expected 'hello world', got %q", b.Line(0))
	}
}

func TestBuffer_MultiLineInsert(t *testing.T) {
	b := NewBufferFromText("helloworld")
	b.Insert(0, 5, "\n", [2]int{0, 5})
	if b.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", b.LineCount())
	}
	if b.Line(0) != "hello" || b.Line(1) != "world" {
		t.Errorf("got lines %q and %q", b.Line(0), b.Line(1))
	}
}

func TestBuffer_DeleteMergesLines(t *testing.T) {
	b := NewBufferFromText("hello\nworld")
	// Delete from end of line 0 to start of line 1
	b.Delete(0, 5, 1, 0, [2]int{0, 5})
	if b.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", b.LineCount())
	}
	if b.Line(0) != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", b.Line(0))
	}
}

func TestBuffer_CRLFNormalization(t *testing.T) {
	b := NewBufferFromText("line1\r\nline2\r\nline3")
	if b.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", b.LineCount())
	}
	if b.Line(0) != "line1" || b.Line(1) != "line2" || b.Line(2) != "line3" {
		t.Errorf("CRLF normalization failed: %q %q %q", b.Line(0), b.Line(1), b.Line(2))
	}
}

func TestBuffer_MixedLineEndings(t *testing.T) {
	b := NewBufferFromText("line1\rline2\r\nline3\nline4")
	if b.LineCount() != 4 {
		t.Errorf("expected 4 lines, got %d", b.LineCount())
	}
	for i, expected := range []string{"line1", "line2", "line3", "line4"} {
		if b.Line(i) != expected {
			t.Errorf("line %d: expected %q, got %q", i, expected, b.Line(i))
		}
	}
}

func TestBuffer_ModifiedFlag(t *testing.T) {
	b := NewBuffer()
	if b.Modified() {
		t.Error("new buffer should not be modified")
	}
	b.Insert(0, 0, "x", [2]int{0, 0})
	if !b.Modified() {
		t.Error("buffer should be modified after insert")
	}
	b.SetModified(false)
	if b.Modified() {
		t.Error("buffer should not be modified after SetModified(false)")
	}
}

// === Story 1.2: Undo and Redo ===

func TestBuffer_UndoInsert(t *testing.T) {
	b := NewBuffer()
	b.Insert(0, 0, "hello", [2]int{0, 0})
	pos, ok := b.Undo()
	if !ok {
		t.Fatal("undo should succeed")
	}
	if pos != [2]int{0, 0} {
		t.Errorf("cursor should return to (0,0), got %v", pos)
	}
	if b.Text() != "" {
		t.Errorf("buffer should be empty after undo, got %q", b.Text())
	}
}

func TestBuffer_UndoDelete(t *testing.T) {
	b := NewBufferFromText("hello world")
	b.Delete(0, 5, 0, 11, [2]int{0, 5})
	if b.Line(0) != "hello" {
		t.Errorf("expected 'hello' after delete, got %q", b.Line(0))
	}
	pos, ok := b.Undo()
	if !ok {
		t.Fatal("undo should succeed")
	}
	_ = pos
	if b.Line(0) != "hello world" {
		t.Errorf("expected 'hello world' after undo, got %q", b.Line(0))
	}
}

func TestBuffer_RedoAfterUndo(t *testing.T) {
	b := NewBuffer()
	b.Insert(0, 0, "a", [2]int{0, 0})
	b.Insert(0, 1, "b", [2]int{0, 1})
	b.Insert(0, 2, "c", [2]int{0, 2})
	b.Undo() // remove 'c'
	b.Undo() // remove 'b'
	b.Undo() // remove 'a'
	pos, ok := b.Redo()
	if !ok {
		t.Fatal("redo should succeed")
	}
	if b.Text() != "a" {
		t.Errorf("expected 'a' after one redo, got %q", b.Text())
	}
	if pos != [2]int{0, 1} {
		t.Errorf("cursor should be at (0,1), got %v", pos)
	}
}

func TestBuffer_NewEditClearsRedoStack(t *testing.T) {
	b := NewBuffer()
	b.Insert(0, 0, "a", [2]int{0, 0})
	b.Insert(0, 1, "b", [2]int{0, 1})
	b.Undo() // remove 'b'
	b.Insert(0, 1, "x", [2]int{0, 1}) // new edit
	_, ok := b.Redo()
	if ok {
		t.Error("redo should fail after new edit clears redo stack")
	}
}

func TestBuffer_UndoEmptyStack(t *testing.T) {
	b := NewBuffer()
	_, ok := b.Undo()
	if ok {
		t.Error("undo on empty stack should return false")
	}
}

func TestBuffer_RedoEmptyStack(t *testing.T) {
	b := NewBuffer()
	_, ok := b.Redo()
	if ok {
		t.Error("redo on empty stack should return false")
	}
}

func TestBuffer_UndoMultiLineInsert(t *testing.T) {
	b := NewBuffer()
	b.Insert(0, 0, "line1\nline2\nline3", [2]int{0, 0})
	if b.LineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", b.LineCount())
	}
	b.Undo()
	if b.LineCount() != 1 || b.Text() != "" {
		t.Errorf("expected empty buffer after undo, got %d lines: %q", b.LineCount(), b.Text())
	}
}

// === Story 1.3: Cursor Movement (tested via integration with Editor) ===
// Cursor movement is tested in editor_test.go since it requires the Editor component

// === Story 1.7: Basic Editing (buffer-level tests) ===

func TestBuffer_InsertCharAtPosition(t *testing.T) {
	b := NewBufferFromText("hllo")
	pos := b.Insert(0, 1, "e", [2]int{0, 1})
	if b.Line(0) != "hello" {
		t.Errorf("expected 'hello', got %q", b.Line(0))
	}
	if pos != [2]int{0, 2} {
		t.Errorf("expected cursor at (0,2), got %v", pos)
	}
}

func TestBuffer_InsertNewlineSplitsLine(t *testing.T) {
	b := NewBufferFromText("hello world")
	pos := b.Insert(0, 5, "\n", [2]int{0, 5})
	if b.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", b.LineCount())
	}
	if b.Line(0) != "hello" || b.Line(1) != " world" {
		t.Errorf("got %q and %q", b.Line(0), b.Line(1))
	}
	if pos != [2]int{1, 0} {
		t.Errorf("expected cursor at (1,0), got %v", pos)
	}
}

func TestBuffer_BackspaceDeletesPrevChar(t *testing.T) {
	b := NewBufferFromText("hello")
	b.Delete(0, 2, 0, 3, [2]int{0, 3})
	if b.Line(0) != "helo" {
		t.Errorf("expected 'helo', got %q", b.Line(0))
	}
}

func TestBuffer_DeleteAtEndMergesNextLine(t *testing.T) {
	b := NewBufferFromText("hello\nworld")
	b.Delete(0, 5, 1, 0, [2]int{0, 5})
	if b.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", b.LineCount())
	}
	if b.Line(0) != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", b.Line(0))
	}
}

func TestBuffer_InsertTabFourSpaces(t *testing.T) {
	b := NewBuffer()
	pos := b.Insert(0, 0, "    ", [2]int{0, 0})
	if b.Line(0) != "    " {
		t.Errorf("expected 4 spaces, got %q", b.Line(0))
	}
	if pos != [2]int{0, 4} {
		t.Errorf("expected cursor at (0,4), got %v", pos)
	}
}

func TestBuffer_DeleteEntireLine(t *testing.T) {
	b := NewBufferFromText("line1\nline2\nline3")
	// Delete line2 by removing from start of line2 to start of line3
	b.Delete(1, 0, 2, 0, [2]int{1, 0})
	if b.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", b.LineCount())
	}
	if b.Line(0) != "line1" || b.Line(1) != "line3" {
		t.Errorf("got %q and %q", b.Line(0), b.Line(1))
	}
}

func TestBuffer_Text(t *testing.T) {
	b := NewBufferFromText("hello\nworld")
	if b.Text() != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", b.Text())
	}
}

func TestBuffer_InsertAtEnd(t *testing.T) {
	b := NewBufferFromText("hello")
	pos := b.Insert(0, 5, " world", [2]int{0, 5})
	if b.Line(0) != "hello world" {
		t.Errorf("expected 'hello world', got %q", b.Line(0))
	}
	if pos != [2]int{0, 11} {
		t.Errorf("expected cursor at (0,11), got %v", pos)
	}
}
