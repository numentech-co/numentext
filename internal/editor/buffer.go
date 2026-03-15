package editor

import (
	"strings"
)

// EditOp represents an edit operation for undo/redo
type EditOp struct {
	Type     EditOpType
	Pos      int // byte offset in flat text
	Text     string
	OldText  string // for replace ops
	CursorBefore [2]int // row, col before
	CursorAfter  [2]int // row, col after
}

type EditOpType int

const (
	OpInsert EditOpType = iota
	OpDelete
)

// Buffer manages text content with undo/redo
type Buffer struct {
	lines    []string
	undoStack []EditOp
	redoStack []EditOp
	modified  bool
}

func NewBuffer() *Buffer {
	return &Buffer{
		lines: []string{""},
	}
}

func NewBufferFromText(text string) *Buffer {
	// Normalize line endings: CRLF -> LF, CR -> LF
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	return &Buffer{
		lines: lines,
	}
}

func (b *Buffer) Lines() []string {
	return b.lines
}

func (b *Buffer) LineCount() int {
	return len(b.lines)
}

func (b *Buffer) Line(row int) string {
	if row < 0 || row >= len(b.lines) {
		return ""
	}
	return b.lines[row]
}

func (b *Buffer) ReplaceLine(row int, text string) {
	if row < 0 || row >= len(b.lines) {
		return
	}
	oldText := b.lines[row]
	op := EditOp{
		Type:         OpDelete,
		Text:         oldText,
		OldText:      text,
		CursorBefore: [2]int{row, len(oldText)},
		CursorAfter:  [2]int{row, len(text)},
	}
	b.undoStack = append(b.undoStack, op)
	b.redoStack = nil
	b.lines[row] = text
	b.modified = true
}

func (b *Buffer) Text() string {
	return strings.Join(b.lines, "\n")
}

// SetText replaces the entire buffer content with the given text.
// Undo history is cleared.
func (b *Buffer) SetText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	b.lines = lines
	b.undoStack = nil
	b.redoStack = nil
}

func (b *Buffer) Modified() bool {
	return b.modified
}

func (b *Buffer) SetModified(m bool) {
	b.modified = m
}

// Insert inserts text at the given row and column
func (b *Buffer) Insert(row, col int, text string, cursorBefore [2]int) [2]int {
	if row < 0 || row >= len(b.lines) {
		return cursorBefore
	}
	line := b.lines[row]
	if col > len(line) {
		col = len(line)
	}

	op := EditOp{
		Type:         OpInsert,
		Text:         text,
		CursorBefore: cursorBefore,
	}

	// Split the text into lines
	insertLines := strings.Split(text, "\n")
	before := line[:col]
	after := line[col:]

	if len(insertLines) == 1 {
		b.lines[row] = before + insertLines[0] + after
		op.CursorAfter = [2]int{row, col + len(insertLines[0])}
	} else {
		// Multi-line insert
		newLines := make([]string, 0, len(b.lines)+len(insertLines)-1)
		newLines = append(newLines, b.lines[:row]...)
		newLines = append(newLines, before+insertLines[0])
		for i := 1; i < len(insertLines)-1; i++ {
			newLines = append(newLines, insertLines[i])
		}
		lastInsert := insertLines[len(insertLines)-1]
		newLines = append(newLines, lastInsert+after)
		newLines = append(newLines, b.lines[row+1:]...)
		b.lines = newLines
		op.CursorAfter = [2]int{row + len(insertLines) - 1, len(lastInsert)}
	}

	b.undoStack = append(b.undoStack, op)
	b.redoStack = nil
	b.modified = true
	return op.CursorAfter
}

// Delete deletes text from (startRow, startCol) to (endRow, endCol)
func (b *Buffer) Delete(startRow, startCol, endRow, endCol int, cursorBefore [2]int) {
	if startRow < 0 || startRow >= len(b.lines) || endRow < 0 || endRow >= len(b.lines) {
		return
	}

	// Collect deleted text
	var deleted strings.Builder
	if startRow == endRow {
		line := b.lines[startRow]
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}
		deleted.WriteString(line[startCol:endCol])
		b.lines[startRow] = line[:startCol] + line[endCol:]
	} else {
		firstLine := b.lines[startRow]
		lastLine := b.lines[endRow]
		if startCol > len(firstLine) {
			startCol = len(firstLine)
		}
		if endCol > len(lastLine) {
			endCol = len(lastLine)
		}
		deleted.WriteString(firstLine[startCol:])
		for i := startRow + 1; i < endRow; i++ {
			deleted.WriteString("\n")
			deleted.WriteString(b.lines[i])
		}
		deleted.WriteString("\n")
		deleted.WriteString(lastLine[:endCol])

		b.lines[startRow] = firstLine[:startCol] + lastLine[endCol:]
		newLines := make([]string, 0, len(b.lines)-(endRow-startRow))
		newLines = append(newLines, b.lines[:startRow+1]...)
		newLines = append(newLines, b.lines[endRow+1:]...)
		b.lines = newLines
	}

	op := EditOp{
		Type:         OpDelete,
		Text:         deleted.String(),
		CursorBefore: cursorBefore,
		CursorAfter:  [2]int{startRow, startCol},
	}
	b.undoStack = append(b.undoStack, op)
	b.redoStack = nil
	b.modified = true
}

// Undo undoes the last operation, returns new cursor position
func (b *Buffer) Undo() ([2]int, bool) {
	if len(b.undoStack) == 0 {
		return [2]int{}, false
	}
	op := b.undoStack[len(b.undoStack)-1]
	b.undoStack = b.undoStack[:len(b.undoStack)-1]

	switch op.Type {
	case OpInsert:
		// Reverse an insert by deleting
		b.removeTextDirect(op.CursorBefore, op.Text)
	case OpDelete:
		// Reverse a delete by inserting
		b.insertTextDirect(op.CursorAfter[0], op.CursorAfter[1], op.Text)
	}

	b.redoStack = append(b.redoStack, op)
	b.modified = len(b.undoStack) > 0
	return op.CursorBefore, true
}

// Redo redoes the last undone operation
func (b *Buffer) Redo() ([2]int, bool) {
	if len(b.redoStack) == 0 {
		return [2]int{}, false
	}
	op := b.redoStack[len(b.redoStack)-1]
	b.redoStack = b.redoStack[:len(b.redoStack)-1]

	switch op.Type {
	case OpInsert:
		b.insertTextDirect(op.CursorBefore[0], op.CursorBefore[1], op.Text)
	case OpDelete:
		b.removeTextDirect(op.CursorBefore, op.Text)
	}

	b.undoStack = append(b.undoStack, op)
	b.modified = true
	return op.CursorAfter, true
}

// insertTextDirect inserts text without recording an undo op
func (b *Buffer) insertTextDirect(row, col int, text string) {
	if row < 0 || row >= len(b.lines) {
		return
	}
	line := b.lines[row]
	if col > len(line) {
		col = len(line)
	}
	insertLines := strings.Split(text, "\n")
	before := line[:col]
	after := line[col:]

	if len(insertLines) == 1 {
		b.lines[row] = before + insertLines[0] + after
	} else {
		newLines := make([]string, 0, len(b.lines)+len(insertLines)-1)
		newLines = append(newLines, b.lines[:row]...)
		newLines = append(newLines, before+insertLines[0])
		for i := 1; i < len(insertLines)-1; i++ {
			newLines = append(newLines, insertLines[i])
		}
		newLines = append(newLines, insertLines[len(insertLines)-1]+after)
		newLines = append(newLines, b.lines[row+1:]...)
		b.lines = newLines
	}
}

// removeTextDirect removes text starting at cursor position without recording undo
func (b *Buffer) removeTextDirect(cursor [2]int, text string) {
	row, col := cursor[0], cursor[1]
	lines := strings.Split(text, "\n")
	endRow := row + len(lines) - 1
	var endCol int
	if len(lines) == 1 {
		endCol = col + len(lines[0])
	} else {
		endCol = len(lines[len(lines)-1])
	}

	if row < 0 || row >= len(b.lines) || endRow >= len(b.lines) {
		return
	}

	firstLine := b.lines[row]
	lastLine := b.lines[endRow]
	if col > len(firstLine) {
		col = len(firstLine)
	}
	if endCol > len(lastLine) {
		endCol = len(lastLine)
	}

	b.lines[row] = firstLine[:col] + lastLine[endCol:]
	if endRow > row {
		newLines := make([]string, 0, len(b.lines)-(endRow-row))
		newLines = append(newLines, b.lines[:row+1]...)
		newLines = append(newLines, b.lines[endRow+1:]...)
		b.lines = newLines
	}
}

// RuneAt returns the rune at the given position
func (b *Buffer) RuneAt(row, col int) (rune, bool) {
	if row < 0 || row >= len(b.lines) {
		return 0, false
	}
	runes := []rune(b.lines[row])
	if col < 0 || col >= len(runes) {
		return 0, false
	}
	return runes[col], true
}
