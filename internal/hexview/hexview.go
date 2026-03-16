package hexview

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

const bytesPerRow = 16

// HexEdit represents a single edit operation for undo/redo.
type HexEdit struct {
	Offset  int
	OldByte byte
	NewByte byte
	Type    string // "replace", "insert", "delete"
}

// HexView is a two-pane hex editor widget.
type HexView struct {
	*tview.Box
	data         []byte
	filePath     string
	cursor       int    // byte offset of cursor
	scrollOffset int    // first visible row (in rows, not bytes)
	focusPane    string // "hex" or "ascii"
	insertMode   bool
	modified     bool
	undoStack    []HexEdit
	redoStack    []HexEdit
	hasFocus     bool

	// Partial hex input: when user types first nibble, store it here.
	// -1 means no partial input pending.
	pendingNibble int

	// Callbacks
	onChange func()

	// "Go to Address" dialog callback
	onGoToAddress func()
}

// New creates a new HexView for the given file data.
func New(filePath string, data []byte) *HexView {
	hv := &HexView{
		Box:           tview.NewBox(),
		data:          make([]byte, len(data)),
		filePath:      filePath,
		focusPane:     "hex",
		pendingNibble: -1,
	}
	copy(hv.data, data)
	hv.SetBackgroundColor(ui.ColorBg)
	return hv
}

// FilePath returns the file path.
func (hv *HexView) FilePath() string {
	return hv.filePath
}

// Modified returns whether the data has been modified.
func (hv *HexView) Modified() bool {
	return hv.modified
}

// DataSize returns the size of the data in bytes.
func (hv *HexView) DataSize() int {
	return len(hv.data)
}

// Cursor returns the current cursor offset.
func (hv *HexView) Cursor() int {
	return hv.cursor
}

// InsertMode returns whether insert mode is active.
func (hv *HexView) InsertMode() bool {
	return hv.insertMode
}

// FocusPane returns the currently focused pane ("hex" or "ascii").
func (hv *HexView) FocusPane() string {
	return hv.focusPane
}

// SetOnChange sets the callback for when data changes.
func (hv *HexView) SetOnChange(fn func()) {
	hv.onChange = fn
}

// SetOnGoToAddress sets the callback for the Go to Address dialog.
func (hv *HexView) SetOnGoToAddress(fn func()) {
	hv.onGoToAddress = fn
}

// GoToOffset moves the cursor to the given byte offset.
func (hv *HexView) GoToOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	if len(hv.data) > 0 && offset >= len(hv.data) {
		offset = len(hv.data) - 1
	}
	hv.cursor = offset
	hv.pendingNibble = -1
	hv.ensureCursorVisible()
}

// Save writes the data back to the file.
func (hv *HexView) Save() error {
	if hv.filePath == "" {
		return fmt.Errorf("no file path")
	}
	err := os.WriteFile(hv.filePath, hv.data, 0644)
	if err != nil {
		return err
	}
	hv.modified = false
	if hv.onChange != nil {
		hv.onChange()
	}
	return nil
}

// Undo reverts the last edit.
func (hv *HexView) Undo() {
	if len(hv.undoStack) == 0 {
		return
	}
	edit := hv.undoStack[len(hv.undoStack)-1]
	hv.undoStack = hv.undoStack[:len(hv.undoStack)-1]

	switch edit.Type {
	case "replace":
		if edit.Offset < len(hv.data) {
			hv.data[edit.Offset] = edit.OldByte
		}
	case "insert":
		// Reverse of insert is delete
		if edit.Offset < len(hv.data) {
			hv.data = append(hv.data[:edit.Offset], hv.data[edit.Offset+1:]...)
		}
	case "delete":
		// Reverse of delete is insert
		hv.data = append(hv.data[:edit.Offset+1], hv.data[edit.Offset:]...)
		hv.data[edit.Offset] = edit.OldByte
	}

	hv.redoStack = append(hv.redoStack, edit)
	hv.cursor = edit.Offset
	hv.modified = len(hv.undoStack) > 0
	hv.pendingNibble = -1
	hv.ensureCursorVisible()
	if hv.onChange != nil {
		hv.onChange()
	}
}

// Redo re-applies the last undone edit.
func (hv *HexView) Redo() {
	if len(hv.redoStack) == 0 {
		return
	}
	edit := hv.redoStack[len(hv.redoStack)-1]
	hv.redoStack = hv.redoStack[:len(hv.redoStack)-1]

	switch edit.Type {
	case "replace":
		if edit.Offset < len(hv.data) {
			hv.data[edit.Offset] = edit.NewByte
		}
	case "insert":
		hv.data = append(hv.data[:edit.Offset+1], hv.data[edit.Offset:]...)
		hv.data[edit.Offset] = edit.NewByte
	case "delete":
		if edit.Offset < len(hv.data) {
			hv.data = append(hv.data[:edit.Offset], hv.data[edit.Offset+1:]...)
		}
	}

	hv.undoStack = append(hv.undoStack, edit)
	hv.cursor = edit.Offset
	hv.modified = true
	hv.pendingNibble = -1
	hv.ensureCursorVisible()
	if hv.onChange != nil {
		hv.onChange()
	}
}

// replaceByte replaces the byte at cursor with a new value.
func (hv *HexView) replaceByte(newByte byte) {
	if hv.cursor >= len(hv.data) {
		return
	}
	oldByte := hv.data[hv.cursor]
	if oldByte == newByte {
		return
	}
	edit := HexEdit{
		Offset:  hv.cursor,
		OldByte: oldByte,
		NewByte: newByte,
		Type:    "replace",
	}
	hv.data[hv.cursor] = newByte
	hv.undoStack = append(hv.undoStack, edit)
	hv.redoStack = nil
	hv.modified = true
	if hv.onChange != nil {
		hv.onChange()
	}
}

// insertByte inserts a byte at the cursor position.
func (hv *HexView) insertByte(b byte) {
	edit := HexEdit{
		Offset:  hv.cursor,
		NewByte: b,
		Type:    "insert",
	}
	// Insert at cursor position
	hv.data = append(hv.data[:hv.cursor+1], hv.data[hv.cursor:]...)
	hv.data[hv.cursor] = b
	hv.undoStack = append(hv.undoStack, edit)
	hv.redoStack = nil
	hv.modified = true
	if hv.onChange != nil {
		hv.onChange()
	}
}

// deleteByte deletes the byte at cursor position.
func (hv *HexView) deleteByte() {
	if hv.cursor >= len(hv.data) || len(hv.data) == 0 {
		return
	}
	edit := HexEdit{
		Offset:  hv.cursor,
		OldByte: hv.data[hv.cursor],
		Type:    "delete",
	}
	hv.data = append(hv.data[:hv.cursor], hv.data[hv.cursor+1:]...)
	hv.undoStack = append(hv.undoStack, edit)
	hv.redoStack = nil
	hv.modified = true
	// Adjust cursor if at end
	if hv.cursor >= len(hv.data) && hv.cursor > 0 {
		hv.cursor = len(hv.data) - 1
	}
	if hv.onChange != nil {
		hv.onChange()
	}
}

// visibleRows returns how many rows can be displayed.
func (hv *HexView) visibleRows() int {
	_, _, _, h := hv.GetInnerRect()
	if h < 2 {
		return 1
	}
	return h - 1 // reserve 1 line for header
}

// totalRows returns the total number of rows in the data.
func (hv *HexView) totalRows() int {
	if len(hv.data) == 0 {
		return 1
	}
	return (len(hv.data) + bytesPerRow - 1) / bytesPerRow
}

// ensureCursorVisible scrolls to keep cursor in view.
func (hv *HexView) ensureCursorVisible() {
	cursorRow := hv.cursor / bytesPerRow
	visible := hv.visibleRows()
	if cursorRow < hv.scrollOffset {
		hv.scrollOffset = cursorRow
	}
	if cursorRow >= hv.scrollOffset+visible {
		hv.scrollOffset = cursorRow - visible + 1
	}
	if hv.scrollOffset < 0 {
		hv.scrollOffset = 0
	}
}

// FormatRow formats a single row of hex data for display.
// Returns the address string, hex bytes string, and ascii string.
func FormatRow(data []byte, rowOffset int) (addr string, hex string, ascii string) {
	addr = fmt.Sprintf("%08X", rowOffset)

	var hexParts []string
	var asciiChars []byte

	count := bytesPerRow
	if rowOffset+count > len(data) {
		count = len(data) - rowOffset
	}

	for i := 0; i < bytesPerRow; i++ {
		if i == 8 {
			// Extra space between byte 8 and 9
			hexParts = append(hexParts, "")
		}
		if i < count {
			hexParts = append(hexParts, fmt.Sprintf("%02X", data[rowOffset+i]))
			b := data[rowOffset+i]
			if b >= 0x20 && b <= 0x7E {
				asciiChars = append(asciiChars, b)
			} else {
				asciiChars = append(asciiChars, '.')
			}
		} else {
			hexParts = append(hexParts, "  ")
			asciiChars = append(asciiChars, ' ')
		}
	}

	hex = strings.Join(hexParts, " ")
	ascii = string(asciiChars)
	return
}

// Draw renders the hex view to the screen.
func (hv *HexView) Draw(screen tcell.Screen) {
	hv.Box.DrawForSubclass(screen, hv)
	x, y, width, height := hv.GetInnerRect()

	if width < 20 || height < 2 {
		return
	}

	bgStyle := tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBg)
	headerStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorGutterBg)
	addrStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)
	hexStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
	asciiStyle := tcell.StyleDefault.Foreground(ui.ColorString).Background(ui.ColorBg)
	cursorHexStyle := tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
	cursorAsciiStyle := tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
	modifiedStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorBg)

	// Clear the area
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
		}
	}

	// Header line
	// Format: "Offset   00 01 02 ... 0F  ASCII"
	hdr := "Offset    00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  ASCII"
	for i, ch := range hdr {
		if x+i < x+width {
			screen.SetContent(x+i, y, ch, nil, headerStyle)
		}
	}
	// Fill rest of header
	for i := len(hdr); i < width; i++ {
		screen.SetContent(x+i, y, ' ', nil, headerStyle)
	}

	// Data rows
	visRows := hv.visibleRows()
	for row := 0; row < visRows; row++ {
		rowIdx := hv.scrollOffset + row
		rowOffset := rowIdx * bytesPerRow
		screenY := y + 1 + row

		if screenY >= y+height {
			break
		}

		if rowOffset >= len(hv.data) {
			// Empty row past end of data
			for col := 0; col < width; col++ {
				screen.SetContent(x+col, screenY, ' ', nil, bgStyle)
			}
			continue
		}

		// Address column (8 hex digits + colon + space = 10 chars)
		addr := fmt.Sprintf("%08X: ", rowOffset)
		for i, ch := range addr {
			if x+i < x+width {
				screen.SetContent(x+i, screenY, ch, nil, addrStyle)
			}
		}

		// Hex bytes
		hexStartX := x + 10
		count := bytesPerRow
		if rowOffset+count > len(hv.data) {
			count = len(hv.data) - rowOffset
		}

		colX := hexStartX
		for i := 0; i < bytesPerRow; i++ {
			if i == 8 {
				// Extra space between byte 8 and 9
				if colX < x+width {
					screen.SetContent(colX, screenY, ' ', nil, bgStyle)
					colX++
				}
			}

			byteOffset := rowOffset + i
			if i < count {
				b := hv.data[byteOffset]
				hi := "0123456789ABCDEF"[b>>4]
				lo := "0123456789ABCDEF"[b&0x0F]

				style := hexStyle
				// Check if this byte has been modified (exists in undo stack)
				if hv.isByteModified(byteOffset) {
					style = modifiedStyle
				}
				if byteOffset == hv.cursor {
					if hv.focusPane == "hex" && hv.hasFocus {
						style = cursorHexStyle
					} else if hv.hasFocus {
						// Highlight in hex pane when ASCII pane is focused
						style = tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBgDarker)
					}
				}

				if colX < x+width {
					screen.SetContent(colX, screenY, rune(hi), nil, style)
					colX++
				}
				if colX < x+width {
					screen.SetContent(colX, screenY, rune(lo), nil, style)
					colX++
				}
			} else {
				// Empty space for missing bytes
				if colX < x+width {
					screen.SetContent(colX, screenY, ' ', nil, bgStyle)
					colX++
				}
				if colX < x+width {
					screen.SetContent(colX, screenY, ' ', nil, bgStyle)
					colX++
				}
			}

			// Space between bytes
			if colX < x+width {
				screen.SetContent(colX, screenY, ' ', nil, bgStyle)
				colX++
			}
		}

		// ASCII separator
		asciiStartX := colX + 1
		if asciiStartX < x+width {
			screen.SetContent(asciiStartX-1, screenY, '|', nil, addrStyle)
		}

		// ASCII representation
		for i := 0; i < bytesPerRow; i++ {
			byteOffset := rowOffset + i
			charX := asciiStartX + i
			if charX >= x+width {
				break
			}

			if i < count {
				b := hv.data[byteOffset]
				ch := rune('.')
				if b >= 0x20 && b <= 0x7E {
					ch = rune(b)
				}

				style := asciiStyle
				if hv.isByteModified(byteOffset) {
					style = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorBg)
				}
				if byteOffset == hv.cursor {
					if hv.focusPane == "ascii" && hv.hasFocus {
						style = cursorAsciiStyle
					} else if hv.hasFocus {
						style = tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBgDarker)
					}
				}
				screen.SetContent(charX, screenY, ch, nil, style)
			} else {
				screen.SetContent(charX, screenY, ' ', nil, bgStyle)
			}
		}

		// Closing pipe
		endX := asciiStartX + bytesPerRow
		if endX < x+width {
			screen.SetContent(endX, screenY, '|', nil, addrStyle)
		}
	}
}

// isByteModified checks if a byte offset has been modified.
func (hv *HexView) isByteModified(offset int) bool {
	for _, edit := range hv.undoStack {
		if edit.Offset == offset {
			return true
		}
	}
	return false
}

// Focus handles focus events.
func (hv *HexView) Focus(delegate func(p tview.Primitive)) {
	hv.hasFocus = true
	hv.Box.Focus(delegate)
}

// Blur handles blur events.
func (hv *HexView) Blur() {
	hv.hasFocus = false
	hv.Box.Blur()
}

// InputHandler handles keyboard input.
func (hv *HexView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return hv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		mod := event.Modifiers()
		ctrl := mod&tcell.ModCtrl != 0

		switch {
		case key == tcell.KeyTab:
			// Switch between hex and ascii panes
			if hv.focusPane == "hex" {
				hv.focusPane = "ascii"
			} else {
				hv.focusPane = "hex"
			}
			hv.pendingNibble = -1

		case key == tcell.KeyUp:
			hv.cursor -= bytesPerRow
			if hv.cursor < 0 {
				hv.cursor = 0
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyDown:
			hv.cursor += bytesPerRow
			if len(hv.data) > 0 && hv.cursor >= len(hv.data) {
				hv.cursor = len(hv.data) - 1
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyLeft:
			if hv.cursor > 0 {
				hv.cursor--
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyRight:
			if len(hv.data) > 0 && hv.cursor < len(hv.data)-1 {
				hv.cursor++
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyHome:
			if ctrl {
				hv.cursor = 0
			} else {
				// Go to start of current row
				hv.cursor = (hv.cursor / bytesPerRow) * bytesPerRow
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyEnd:
			if ctrl {
				if len(hv.data) > 0 {
					hv.cursor = len(hv.data) - 1
				}
			} else {
				// Go to end of current row
				rowStart := (hv.cursor / bytesPerRow) * bytesPerRow
				rowEnd := rowStart + bytesPerRow - 1
				if rowEnd >= len(hv.data) {
					rowEnd = len(hv.data) - 1
				}
				hv.cursor = rowEnd
			}
			hv.pendingNibble = -1
			hv.ensureCursorVisible()

		case key == tcell.KeyPgUp:
			rows := hv.visibleRows()
			hv.cursor -= rows * bytesPerRow
			if hv.cursor < 0 {
				hv.cursor = 0
			}
			hv.scrollOffset -= rows
			if hv.scrollOffset < 0 {
				hv.scrollOffset = 0
			}
			hv.pendingNibble = -1

		case key == tcell.KeyPgDn:
			rows := hv.visibleRows()
			hv.cursor += rows * bytesPerRow
			if len(hv.data) > 0 && hv.cursor >= len(hv.data) {
				hv.cursor = len(hv.data) - 1
			}
			hv.scrollOffset += rows
			maxScroll := hv.totalRows() - rows
			if maxScroll < 0 {
				maxScroll = 0
			}
			if hv.scrollOffset > maxScroll {
				hv.scrollOffset = maxScroll
			}
			hv.pendingNibble = -1

		case key == tcell.KeyInsert:
			hv.insertMode = !hv.insertMode
			hv.pendingNibble = -1

		case key == tcell.KeyDelete:
			hv.deleteByte()
			hv.pendingNibble = -1

		case ctrl && (key == tcell.KeyCtrlZ || event.Rune() == 'z'):
			hv.Undo()

		case ctrl && (key == tcell.KeyCtrlY || event.Rune() == 'y'):
			hv.Redo()

		case ctrl && (key == tcell.KeyCtrlS || event.Rune() == 's'):
			// Save handled by app level, but we support it here too
			_ = hv.Save()

		case ctrl && (key == tcell.KeyCtrlG || event.Rune() == 'g'):
			if hv.onGoToAddress != nil {
				hv.onGoToAddress()
			}

		case key == tcell.KeyRune && !ctrl:
			r := event.Rune()
			if hv.focusPane == "hex" {
				hv.handleHexInput(r)
			} else {
				hv.handleAsciiInput(r)
			}
		}
	})
}

// handleHexInput processes hex digit input in the hex pane.
func (hv *HexView) handleHexInput(r rune) {
	nibble := hexDigit(r)
	if nibble < 0 {
		return
	}

	if len(hv.data) == 0 {
		return
	}

	if hv.pendingNibble < 0 {
		// First nibble
		hv.pendingNibble = nibble
	} else {
		// Second nibble - complete the byte
		newByte := byte(hv.pendingNibble<<4 | nibble)
		hv.pendingNibble = -1
		if hv.insertMode {
			hv.insertByte(newByte)
			hv.cursor++
			if hv.cursor >= len(hv.data) {
				hv.cursor = len(hv.data) - 1
			}
		} else {
			hv.replaceByte(newByte)
			// Advance cursor
			if hv.cursor < len(hv.data)-1 {
				hv.cursor++
			}
		}
		hv.ensureCursorVisible()
	}
}

// handleAsciiInput processes ASCII character input in the ascii pane.
func (hv *HexView) handleAsciiInput(r rune) {
	if r < 0x20 || r > 0x7E {
		return
	}
	if len(hv.data) == 0 {
		return
	}

	newByte := byte(r)
	if hv.insertMode {
		hv.insertByte(newByte)
		hv.cursor++
		if hv.cursor >= len(hv.data) {
			hv.cursor = len(hv.data) - 1
		}
	} else {
		hv.replaceByte(newByte)
		if hv.cursor < len(hv.data)-1 {
			hv.cursor++
		}
	}
	hv.ensureCursorVisible()
}

// hexDigit returns the numeric value of a hex digit rune, or -1 if not a hex digit.
func hexDigit(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	default:
		return -1
	}
}

// IsBinaryData checks if data appears to be binary by scanning for non-text bytes.
// Returns true if more than 5% of bytes in the first 8KB are non-printable.
func IsBinaryData(data []byte) bool {
	size := len(data)
	if size == 0 {
		return false
	}

	scanSize := size
	if scanSize > 8192 {
		scanSize = 8192
	}

	nonPrintable := 0
	for i := 0; i < scanSize; i++ {
		b := data[i]
		// Allow common text bytes: printable ASCII, tab, newline, carriage return
		if b == 0 || (b < 0x20 && b != '\t' && b != '\n' && b != '\r') || b == 0x7F {
			nonPrintable++
		}
	}

	// Binary if more than 5% non-printable
	threshold := scanSize * 5 / 100
	if threshold < 1 {
		threshold = 1
	}
	return nonPrintable > threshold
}

// IsBinaryFile reads a file and checks if it is binary.
func IsBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}
	return IsBinaryData(buf[:n]), nil
}
