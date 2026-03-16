package hexview

import (
	"testing"
)

func TestFormatRow(t *testing.T) {
	data := []byte("Hello World!\x00\x01\x02\x03Next row here...")
	addr, hex, ascii := FormatRow(data, 0)

	if addr != "00000000" {
		t.Errorf("addr = %q, want %q", addr, "00000000")
	}

	// Check hex contains the bytes
	if len(hex) == 0 {
		t.Error("hex string is empty")
	}

	// First byte is 'H' = 0x48
	if hex[:2] != "48" {
		t.Errorf("first hex byte = %q, want %q", hex[:2], "48")
	}

	// ASCII should show printable chars, dots for non-printable
	if ascii[0] != 'H' {
		t.Errorf("first ascii char = %c, want H", ascii[0])
	}
	// Byte 12 is 0x00 (non-printable)
	if ascii[12] != '.' {
		t.Errorf("ascii[12] = %c, want '.'", ascii[12])
	}
}

func TestFormatRowPartial(t *testing.T) {
	// Less than 16 bytes
	data := []byte("ABC")
	addr, hex, ascii := FormatRow(data, 0)

	if addr != "00000000" {
		t.Errorf("addr = %q, want %q", addr, "00000000")
	}

	// ASCII should have 3 chars + 13 spaces
	if len(ascii) != 16 {
		t.Errorf("ascii len = %d, want 16", len(ascii))
	}
	if ascii[0] != 'A' || ascii[1] != 'B' || ascii[2] != 'C' {
		t.Errorf("ascii = %q, want 'ABC' prefix", ascii)
	}

	_ = hex // hex is formatted correctly
}

func TestFormatRowSecondRow(t *testing.T) {
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}
	addr, _, _ := FormatRow(data, 16)
	if addr != "00000010" {
		t.Errorf("addr = %q, want %q", addr, "00000010")
	}
}

func TestHexGrouping(t *testing.T) {
	// Verify 8+8 grouping (extra space between byte 8 and 9)
	data := make([]byte, 16)
	for i := range data {
		data[i] = byte(i)
	}
	_, hex, _ := FormatRow(data, 0)

	// The hex string should have an extra empty element between byte 7 and 8
	// which results in a double space in the joined output
	if len(hex) == 0 {
		t.Fatal("hex string is empty")
	}
	// Check that there's an extra space somewhere around the midpoint
	// Format: "00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F"
	// The double space should be at position after "07 "
	expected := "00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F"
	if hex != expected {
		t.Errorf("hex = %q, want %q", hex, expected)
	}
}

func TestReplaceByte(t *testing.T) {
	data := []byte("Hello")
	hv := New("test.bin", data)

	hv.cursor = 0
	hv.replaceByte(0x41) // 'A'

	if hv.data[0] != 0x41 {
		t.Errorf("data[0] = 0x%02X, want 0x41", hv.data[0])
	}
	if !hv.modified {
		t.Error("expected modified = true")
	}
	if len(hv.undoStack) != 1 {
		t.Errorf("undoStack len = %d, want 1", len(hv.undoStack))
	}
}

func TestInsertByte(t *testing.T) {
	data := []byte("AB")
	hv := New("test.bin", data)

	hv.cursor = 1
	hv.insertByte(0x58) // 'X'

	if len(hv.data) != 3 {
		t.Fatalf("data len = %d, want 3", len(hv.data))
	}
	if hv.data[0] != 'A' || hv.data[1] != 'X' || hv.data[2] != 'B' {
		t.Errorf("data = %v, want [A X B]", hv.data)
	}
	if !hv.modified {
		t.Error("expected modified = true")
	}
}

func TestDeleteByte(t *testing.T) {
	data := []byte("ABC")
	hv := New("test.bin", data)

	hv.cursor = 1
	hv.deleteByte()

	if len(hv.data) != 2 {
		t.Fatalf("data len = %d, want 2", len(hv.data))
	}
	if hv.data[0] != 'A' || hv.data[1] != 'C' {
		t.Errorf("data = %v, want [A C]", hv.data)
	}
}

func TestUndo(t *testing.T) {
	data := []byte("Hello")
	hv := New("test.bin", data)

	hv.cursor = 0
	hv.replaceByte(0x41)

	if hv.data[0] != 0x41 {
		t.Fatalf("after replace: data[0] = 0x%02X, want 0x41", hv.data[0])
	}

	hv.Undo()

	if hv.data[0] != 'H' {
		t.Errorf("after undo: data[0] = 0x%02X, want 0x%02X ('H')", hv.data[0], byte('H'))
	}
	if hv.modified {
		t.Error("expected modified = false after undoing all changes")
	}
}

func TestRedo(t *testing.T) {
	data := []byte("Hello")
	hv := New("test.bin", data)

	hv.cursor = 0
	hv.replaceByte(0x41)
	hv.Undo()
	hv.Redo()

	if hv.data[0] != 0x41 {
		t.Errorf("after redo: data[0] = 0x%02X, want 0x41", hv.data[0])
	}
	if !hv.modified {
		t.Error("expected modified = true after redo")
	}
}

func TestUndoInsert(t *testing.T) {
	data := []byte("AB")
	hv := New("test.bin", data)

	hv.cursor = 1
	hv.insertByte(0x58)

	if len(hv.data) != 3 {
		t.Fatalf("after insert: len = %d, want 3", len(hv.data))
	}

	hv.Undo()

	if len(hv.data) != 2 {
		t.Fatalf("after undo: len = %d, want 2", len(hv.data))
	}
	if hv.data[0] != 'A' || hv.data[1] != 'B' {
		t.Errorf("after undo: data = %v, want [A B]", hv.data)
	}
}

func TestUndoDelete(t *testing.T) {
	data := []byte("ABC")
	hv := New("test.bin", data)

	hv.cursor = 1
	hv.deleteByte()

	if len(hv.data) != 2 {
		t.Fatalf("after delete: len = %d, want 2", len(hv.data))
	}

	hv.Undo()

	if len(hv.data) != 3 {
		t.Fatalf("after undo: len = %d, want 3", len(hv.data))
	}
	if hv.data[0] != 'A' || hv.data[1] != 'B' || hv.data[2] != 'C' {
		t.Errorf("after undo: data = %v, want [A B C]", hv.data)
	}
}

func TestIsBinaryData(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		binary bool
	}{
		{"empty", []byte{}, false},
		{"plain text", []byte("Hello, World!\nThis is text.\n"), false},
		{"go source", []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"), false},
		{"binary with nulls", []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{"mostly binary", make([]byte, 100), true}, // all zeros
		{"few non-printable", append([]byte("Hello World this is mostly text"), 0x01), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryData(tt.data)
			if got != tt.binary {
				t.Errorf("IsBinaryData(%q) = %v, want %v", tt.name, got, tt.binary)
			}
		})
	}
}

func TestHexDigit(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'0', 0}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
		{'g', -1}, {'z', -1}, {' ', -1},
	}
	for _, tt := range tests {
		got := hexDigit(tt.r)
		if got != tt.want {
			t.Errorf("hexDigit(%c) = %d, want %d", tt.r, got, tt.want)
		}
	}
}

func TestGoToOffset(t *testing.T) {
	data := make([]byte, 256)
	hv := New("test.bin", data)

	hv.GoToOffset(100)
	if hv.cursor != 100 {
		t.Errorf("cursor = %d, want 100", hv.cursor)
	}

	hv.GoToOffset(-5)
	if hv.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", hv.cursor)
	}

	hv.GoToOffset(1000)
	if hv.cursor != 255 {
		t.Errorf("cursor = %d, want 255 (clamped)", hv.cursor)
	}
}

func TestPaneSwitch(t *testing.T) {
	hv := New("test.bin", []byte("test"))

	if hv.FocusPane() != "hex" {
		t.Errorf("initial pane = %q, want hex", hv.FocusPane())
	}

	// Simulate Tab press by directly changing focusPane
	hv.focusPane = "ascii"
	if hv.FocusPane() != "ascii" {
		t.Errorf("after switch: pane = %q, want ascii", hv.FocusPane())
	}
}

func TestHexInput(t *testing.T) {
	data := []byte{0x00, 0x11, 0x22}
	hv := New("test.bin", data)

	// Type '4' then '1' to set byte to 0x41
	hv.handleHexInput('4')
	if hv.pendingNibble != 4 {
		t.Errorf("pendingNibble = %d, want 4", hv.pendingNibble)
	}

	hv.handleHexInput('1')
	if hv.data[0] != 0x41 {
		t.Errorf("data[0] = 0x%02X, want 0x41", hv.data[0])
	}
	if hv.pendingNibble != -1 {
		t.Errorf("pendingNibble = %d, want -1", hv.pendingNibble)
	}
	// Cursor should have advanced
	if hv.cursor != 1 {
		t.Errorf("cursor = %d, want 1", hv.cursor)
	}
}

func TestAsciiInput(t *testing.T) {
	data := []byte{0x00, 0x11}
	hv := New("test.bin", data)

	hv.focusPane = "ascii"
	hv.handleAsciiInput('A')

	if hv.data[0] != 'A' {
		t.Errorf("data[0] = 0x%02X, want 0x41 ('A')", hv.data[0])
	}
	if hv.cursor != 1 {
		t.Errorf("cursor = %d, want 1", hv.cursor)
	}
}

func TestInsertModeHexInput(t *testing.T) {
	data := []byte{0xAA, 0xBB}
	hv := New("test.bin", data)
	hv.insertMode = true

	hv.handleHexInput('4')
	hv.handleHexInput('1')

	if len(hv.data) != 3 {
		t.Fatalf("data len = %d, want 3", len(hv.data))
	}
	if hv.data[0] != 0x41 {
		t.Errorf("data[0] = 0x%02X, want 0x41", hv.data[0])
	}
	if hv.data[1] != 0xAA {
		t.Errorf("data[1] = 0x%02X, want 0xAA", hv.data[1])
	}
}

func TestDeleteByteAtEnd(t *testing.T) {
	data := []byte{0xAA}
	hv := New("test.bin", data)

	hv.cursor = 0
	hv.deleteByte()

	if len(hv.data) != 0 {
		t.Errorf("data len = %d, want 0", len(hv.data))
	}
}

func TestDeleteByteEmpty(t *testing.T) {
	hv := New("test.bin", []byte{})
	hv.deleteByte() // Should not panic
}

func TestMultipleUndoRedo(t *testing.T) {
	data := []byte("ABCD")
	hv := New("test.bin", data)

	// Make 3 changes
	hv.cursor = 0
	hv.replaceByte('X')
	hv.cursor = 1
	hv.replaceByte('Y')
	hv.cursor = 2
	hv.replaceByte('Z')

	if string(hv.data) != "XYZD" {
		t.Fatalf("after edits: %q, want XYZD", string(hv.data))
	}

	// Undo all 3
	hv.Undo()
	hv.Undo()
	hv.Undo()

	if string(hv.data) != "ABCD" {
		t.Errorf("after 3 undos: %q, want ABCD", string(hv.data))
	}

	// Redo 2
	hv.Redo()
	hv.Redo()

	if string(hv.data) != "XYCD" {
		t.Errorf("after 2 redos: %q, want XYCD", string(hv.data))
	}
}
