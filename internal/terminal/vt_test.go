package terminal

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewVT(t *testing.T) {
	vt := NewVT(80, 24)
	if vt.Cols() != 80 {
		t.Errorf("expected 80 cols, got %d", vt.Cols())
	}
	if vt.Rows() != 24 {
		t.Errorf("expected 24 rows, got %d", vt.Rows())
	}
	if vt.CursorRow() != 0 || vt.CursorCol() != 0 {
		t.Errorf("cursor should start at 0,0")
	}
}

func TestWriteText(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("Hello"))
	for i, ch := range "Hello" {
		cell := vt.Cell(0, i)
		if cell.Ch != ch {
			t.Errorf("cell[0][%d] = %c, want %c", i, cell.Ch, ch)
		}
	}
	if vt.CursorCol() != 5 {
		t.Errorf("cursor col = %d, want 5", vt.CursorCol())
	}
}

func TestNewline(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("line1\r\nline2"))
	if vt.CursorRow() != 1 {
		t.Errorf("cursor row = %d, want 1", vt.CursorRow())
	}
	cell := vt.Cell(1, 0)
	if cell.Ch != 'l' {
		t.Errorf("cell[1][0] = %c, want 'l'", cell.Ch)
	}
}

func TestCarriageReturn(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("abcdef\rXY"))
	cell := vt.Cell(0, 0)
	if cell.Ch != 'X' {
		t.Errorf("cell[0][0] = %c, want 'X'", cell.Ch)
	}
	cell = vt.Cell(0, 1)
	if cell.Ch != 'Y' {
		t.Errorf("cell[0][1] = %c, want 'Y'", cell.Ch)
	}
	// Original chars after overwrite should remain
	cell = vt.Cell(0, 2)
	if cell.Ch != 'c' {
		t.Errorf("cell[0][2] = %c, want 'c'", cell.Ch)
	}
}

func TestBackspace(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("abc\b"))
	if vt.CursorCol() != 2 {
		t.Errorf("cursor col = %d, want 2", vt.CursorCol())
	}
}

func TestTab(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("a\t"))
	if vt.CursorCol() != 8 {
		t.Errorf("cursor col = %d, want 8", vt.CursorCol())
	}
}

func TestCursorMovement(t *testing.T) {
	vt := NewVT(80, 24)
	// Move cursor to row 5, col 10 (1-based: 6, 11)
	vt.Write([]byte("\x1b[6;11H"))
	if vt.CursorRow() != 5 || vt.CursorCol() != 10 {
		t.Errorf("cursor = (%d,%d), want (5,10)", vt.CursorRow(), vt.CursorCol())
	}
}

func TestCursorUp(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[5;1H")) // row 4
	vt.Write([]byte("\x1b[2A"))    // up 2
	if vt.CursorRow() != 2 {
		t.Errorf("cursor row = %d, want 2", vt.CursorRow())
	}
}

func TestCursorDown(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[3B")) // down 3
	if vt.CursorRow() != 3 {
		t.Errorf("cursor row = %d, want 3", vt.CursorRow())
	}
}

func TestCursorForwardBackward(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[10C")) // forward 10
	if vt.CursorCol() != 10 {
		t.Errorf("cursor col = %d, want 10", vt.CursorCol())
	}
	vt.Write([]byte("\x1b[3D")) // backward 3
	if vt.CursorCol() != 7 {
		t.Errorf("cursor col = %d, want 7", vt.CursorCol())
	}
}

func TestEraseDisplay(t *testing.T) {
	vt := NewVT(10, 5)
	vt.Write([]byte("AAAAAAAAAA")) // fill first row
	vt.Write([]byte("\x1b[2J"))     // erase all
	for c := 0; c < 10; c++ {
		cell := vt.Cell(0, c)
		if cell.Ch != ' ' {
			t.Errorf("cell[0][%d] = %c, want ' '", c, cell.Ch)
		}
	}
}

func TestEraseLine(t *testing.T) {
	vt := NewVT(10, 5)
	vt.Write([]byte("ABCDEFGHIJ"))
	vt.Write([]byte("\x1b[1;5H")) // cursor to col 4
	vt.Write([]byte("\x1b[K"))     // erase from cursor to end of line
	// Cols 0-3 should be intact
	for c := 0; c < 4; c++ {
		cell := vt.Cell(0, c)
		expected := rune("ABCDEFGHIJ"[c])
		if cell.Ch != expected {
			t.Errorf("cell[0][%d] = %c, want %c", c, cell.Ch, expected)
		}
	}
	// Cols 4-9 should be cleared
	for c := 4; c < 10; c++ {
		cell := vt.Cell(0, c)
		if cell.Ch != ' ' {
			t.Errorf("cell[0][%d] = %c, want ' '", c, cell.Ch)
		}
	}
}

func TestSGRColors(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[31mR\x1b[32mG\x1b[0mN"))
	cell := vt.Cell(0, 0)
	if cell.Ch != 'R' || cell.Fg != tcell.ColorMaroon {
		t.Errorf("cell[0][0] = %c fg=%v, want 'R' Maroon", cell.Ch, cell.Fg)
	}
	cell = vt.Cell(0, 1)
	if cell.Ch != 'G' || cell.Fg != tcell.ColorGreen {
		t.Errorf("cell[0][1] = %c fg=%v, want 'G' Green", cell.Ch, cell.Fg)
	}
	cell = vt.Cell(0, 2)
	if cell.Ch != 'N' || cell.Fg != tcell.ColorWhite {
		t.Errorf("cell[0][2] = %c fg=%v, want 'N' White (reset)", cell.Ch, cell.Fg)
	}
}

func TestSGRBold(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[1mB\x1b[22mN"))
	cell := vt.Cell(0, 0)
	if !cell.Bold {
		t.Error("cell[0][0] should be bold")
	}
	cell = vt.Cell(0, 1)
	if cell.Bold {
		t.Error("cell[0][1] should not be bold")
	}
}

func TestScrollUp(t *testing.T) {
	vt := NewVT(10, 3)
	vt.Write([]byte("line1\r\nline2\r\nline3\r\nline4"))
	// After 4 lines in 3 rows, line1 should be in scrollback
	sb := vt.Scrollback()
	if len(sb) != 1 {
		t.Fatalf("scrollback len = %d, want 1", len(sb))
	}
	if sb[0][0].Ch != 'l' || sb[0][4].Ch != '1' {
		t.Error("scrollback should contain line1")
	}
	// Current display should show line2, line3, line4
	cell := vt.Cell(0, 4)
	if cell.Ch != '2' {
		t.Errorf("row 0 should be line2, got char '%c' at col 4", cell.Ch)
	}
}

func TestResize(t *testing.T) {
	vt := NewVT(10, 5)
	vt.Write([]byte("Hello"))
	vt.Resize(20, 10)
	if vt.Cols() != 20 || vt.Rows() != 10 {
		t.Errorf("size = %dx%d, want 20x10", vt.Cols(), vt.Rows())
	}
	// Content should be preserved
	cell := vt.Cell(0, 0)
	if cell.Ch != 'H' {
		t.Errorf("cell[0][0] = %c, want 'H'", cell.Ch)
	}
}

func TestSaveCursorRestore(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[5;10H")) // move to (4,9)
	vt.Write([]byte("\x1b7"))       // save
	vt.Write([]byte("\x1b[1;1H"))   // move to (0,0)
	vt.Write([]byte("\x1b8"))       // restore
	if vt.CursorRow() != 4 || vt.CursorCol() != 9 {
		t.Errorf("cursor = (%d,%d), want (4,9)", vt.CursorRow(), vt.CursorCol())
	}
}

func TestCSISaveCursorRestore(t *testing.T) {
	vt := NewVT(80, 24)
	vt.Write([]byte("\x1b[5;10H")) // move to (4,9)
	vt.Write([]byte("\x1b[s"))      // save
	vt.Write([]byte("\x1b[1;1H"))   // move to (0,0)
	vt.Write([]byte("\x1b[u"))      // restore
	if vt.CursorRow() != 4 || vt.CursorCol() != 9 {
		t.Errorf("cursor = (%d,%d), want (4,9)", vt.CursorRow(), vt.CursorCol())
	}
}

func TestLineWrap(t *testing.T) {
	vt := NewVT(5, 3)
	vt.Write([]byte("ABCDEFGH"))
	// "ABCDE" on row 0, "FGH" on row 1
	cell := vt.Cell(0, 4)
	if cell.Ch != 'E' {
		t.Errorf("cell[0][4] = %c, want 'E'", cell.Ch)
	}
	cell = vt.Cell(1, 0)
	if cell.Ch != 'F' {
		t.Errorf("cell[1][0] = %c, want 'F'", cell.Ch)
	}
}

func TestOSCIgnored(t *testing.T) {
	vt := NewVT(80, 24)
	// OSC sequence (set window title) should be ignored
	vt.Write([]byte("\x1b]0;My Title\x07Hello"))
	cell := vt.Cell(0, 0)
	if cell.Ch != 'H' {
		t.Errorf("cell[0][0] = %c, want 'H'", cell.Ch)
	}
}

func TestParseParams(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"", nil},
		{"1", []int{1}},
		{"1;2", []int{1, 2}},
		{"10;20;30", []int{10, 20, 30}},
		{"0", []int{0}},
	}
	for _, tt := range tests {
		got := parseParams([]byte(tt.input))
		if len(got) != len(tt.expected) {
			t.Errorf("parseParams(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseParams(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestInsertLines(t *testing.T) {
	vt := NewVT(5, 3)
	vt.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	vt.Write([]byte("\x1b[2;1H")) // cursor to row 1
	vt.Write([]byte("\x1b[1L"))    // insert 1 line
	// Row 1 should now be blank, BBBBB pushed to row 2
	cell := vt.Cell(1, 0)
	if cell.Ch != ' ' {
		t.Errorf("cell[1][0] = %c, want ' '", cell.Ch)
	}
	cell = vt.Cell(2, 0)
	if cell.Ch != 'B' {
		t.Errorf("cell[2][0] = %c, want 'B'", cell.Ch)
	}
}

func TestDeleteLines(t *testing.T) {
	vt := NewVT(5, 3)
	vt.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	vt.Write([]byte("\x1b[1;1H")) // cursor to row 0
	vt.Write([]byte("\x1b[1M"))    // delete 1 line
	// Row 0 should now be BBBBB
	cell := vt.Cell(0, 0)
	if cell.Ch != 'B' {
		t.Errorf("cell[0][0] = %c, want 'B'", cell.Ch)
	}
	// Row 2 should be blank
	cell = vt.Cell(2, 0)
	if cell.Ch != ' ' {
		t.Errorf("cell[2][0] = %c, want ' '", cell.Ch)
	}
}

func TestReset(t *testing.T) {
	vt := NewVT(10, 5)
	vt.Write([]byte("Hello"))
	vt.Write([]byte("\x1bc")) // ESC c = reset
	cell := vt.Cell(0, 0)
	if cell.Ch != ' ' {
		t.Errorf("cell[0][0] = %c, want ' ' after reset", cell.Ch)
	}
	if vt.CursorRow() != 0 || vt.CursorCol() != 0 {
		t.Error("cursor should be at 0,0 after reset")
	}
}
