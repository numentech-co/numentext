package terminal

import (
	"github.com/gdamore/tcell/v2"
)

// Cell represents a single character cell in the terminal grid
type Cell struct {
	Ch   rune
	Fg   tcell.Color
	Bg   tcell.Color
	Bold bool
}

// VT is a VT100/xterm terminal state machine with UTF-8, 256-color, RGB,
// alternate screen, and scroll region support.
type VT struct {
	cols, rows int
	cells      [][]Cell
	curRow     int
	curCol     int
	savedRow   int
	savedCol   int
	scrollback [][]Cell
	maxScroll  int

	// Scroll region (1-indexed top/bottom, 0 means full screen)
	scrollTop    int
	scrollBottom int

	// Alternate screen buffer
	altCells    [][]Cell
	altCurRow   int
	altCurCol   int
	altSavedRow int
	altSavedCol int
	altActive   bool

	// Cursor visibility (CSI ?25h / ?25l)
	cursorVisible bool

	// Current attributes
	fg        tcell.Color
	bg        tcell.Color
	bold      bool
	italic    bool
	underline bool
	dim       bool
	reverse   bool

	// Parser state
	state    parseState
	paramBuf []byte

	// UTF-8 decoder
	utf8Buf  [4]byte
	utf8Len  int // expected total bytes
	utf8Have int // bytes collected so far

	// Block tracking for command boxing
	blocks *BlockTracker
}

type parseState int

const (
	stateNormal  parseState = iota
	stateEsc
	stateCSI
	stateOSC
	stateOSCEsc  // saw ESC inside OSC, expecting '\'
	stateUTF8    // collecting UTF-8 continuation bytes
	stateCharset // ESC ( or ESC ) — consume one more byte (charset designator)
	stateDCS     // DCS string (ESC P ... ST) — consume until ST
	stateDCSEsc  // saw ESC inside DCS, expecting '\'
)

func NewVT(cols, rows int) *VT {
	vt := &VT{
		cols:          cols,
		rows:          rows,
		maxScroll:     1000,
		cursorVisible: true,
		fg:            tcell.ColorWhite,
		bg:            tcell.ColorDefault,
		blocks:        NewBlockTracker(),
	}
	vt.cells = vt.makeGrid(cols, rows)
	return vt
}

func (vt *VT) makeGrid(cols, rows int) [][]Cell {
	grid := make([][]Cell, rows)
	for i := range grid {
		grid[i] = make([]Cell, cols)
		for j := range grid[i] {
			grid[i][j] = Cell{Ch: ' ', Fg: tcell.ColorWhite, Bg: tcell.ColorDefault}
		}
	}
	return grid
}

func (vt *VT) Resize(cols, rows int) {
	if cols == vt.cols && rows == vt.rows {
		return
	}
	newGrid := vt.makeGrid(cols, rows)
	for r := 0; r < rows && r < vt.rows; r++ {
		for c := 0; c < cols && c < vt.cols; c++ {
			newGrid[r][c] = vt.cells[r][c]
		}
	}
	vt.cells = newGrid
	vt.cols = cols
	vt.rows = rows
	if vt.curRow >= rows {
		vt.curRow = rows - 1
	}
	if vt.curCol >= cols {
		vt.curCol = cols - 1
	}
	// Reset scroll region on resize
	vt.scrollTop = 0
	vt.scrollBottom = 0
}

func (vt *VT) Rows() int          { return vt.rows }
func (vt *VT) Cols() int          { return vt.cols }
func (vt *VT) CursorRow() int     { return vt.curRow }
func (vt *VT) CursorCol() int     { return vt.curCol }
func (vt *VT) CursorVisible() bool { return vt.cursorVisible }

func (vt *VT) Cell(row, col int) Cell {
	if row < 0 || row >= vt.rows || col < 0 || col >= vt.cols {
		return Cell{Ch: ' ', Fg: tcell.ColorWhite, Bg: tcell.ColorDefault}
	}
	return vt.cells[row][col]
}

func (vt *VT) Scrollback() [][]Cell {
	return vt.scrollback
}

// Blocks returns the block tracker for command boxing.
func (vt *VT) Blocks() *BlockTracker {
	return vt.blocks
}

// scrollRegionTop returns the effective top of the scroll region (0-indexed).
func (vt *VT) scrollRegionTop() int {
	if vt.scrollTop > 0 {
		return vt.scrollTop - 1
	}
	return 0
}

// scrollRegionBottom returns the effective bottom of the scroll region (0-indexed).
func (vt *VT) scrollRegionBottom() int {
	if vt.scrollBottom > 0 && vt.scrollBottom <= vt.rows {
		return vt.scrollBottom - 1
	}
	return vt.rows - 1
}

// Write processes raw terminal output bytes
func (vt *VT) Write(data []byte) {
	for _, b := range data {
		vt.processByte(b)
	}
}

func (vt *VT) processByte(b byte) {
	// If we're collecting UTF-8 continuation bytes
	if vt.state == stateUTF8 {
		if b&0xC0 == 0x80 {
			vt.utf8Buf[vt.utf8Have] = b
			vt.utf8Have++
			if vt.utf8Have == vt.utf8Len {
				// Decode the rune
				r := decodeUTF8(vt.utf8Buf[:vt.utf8Len])
				vt.state = stateNormal
				vt.putChar(r)
			}
			return
		}
		// Invalid continuation — discard partial and reprocess this byte
		vt.state = stateNormal
		// Fall through to process b normally
	}

	switch vt.state {
	case stateNormal:
		switch {
		case b == 0x1b: // ESC
			vt.state = stateEsc
			vt.paramBuf = nil
		case b == '\n':
			vt.blocks.FeedNewline()
			vt.lineFeed()
		case b == '\r':
			vt.blocks.FeedCR()
			vt.curCol = 0
		case b == '\t':
			newCol := (vt.curCol + 8) &^ 7
			if newCol >= vt.cols {
				newCol = vt.cols - 1
			}
			for i := vt.curCol; i < newCol; i++ {
				vt.blocks.FeedChar(' ')
			}
			vt.curCol = newCol
		case b == '\b':
			if vt.curCol > 0 {
				vt.curCol--
			}
		case b == 0x07: // BEL - ignore
		case b >= 0xC0 && b <= 0xDF: // 2-byte UTF-8
			vt.utf8Buf[0] = b
			vt.utf8Len = 2
			vt.utf8Have = 1
			vt.state = stateUTF8
		case b >= 0xE0 && b <= 0xEF: // 3-byte UTF-8
			vt.utf8Buf[0] = b
			vt.utf8Len = 3
			vt.utf8Have = 1
			vt.state = stateUTF8
		case b >= 0xF0 && b <= 0xF7: // 4-byte UTF-8
			vt.utf8Buf[0] = b
			vt.utf8Len = 4
			vt.utf8Have = 1
			vt.state = stateUTF8
		case b >= 0x20 && b < 0x80:
			vt.putChar(rune(b))
		// Ignore other C0 control characters
		}
	case stateEsc:
		switch b {
		case '[':
			vt.state = stateCSI
			vt.paramBuf = nil
		case ']':
			vt.state = stateOSC
			vt.paramBuf = nil
		case '7': // Save cursor (DECSC)
			vt.savedRow = vt.curRow
			vt.savedCol = vt.curCol
			vt.state = stateNormal
		case '8': // Restore cursor (DECRC)
			vt.curRow = vt.savedRow
			vt.curCol = vt.savedCol
			vt.state = stateNormal
		case 'D': // Index (move down, scroll if at bottom)
			vt.index()
			vt.state = stateNormal
		case 'M': // Reverse index (move up, scroll if at top)
			vt.reverseIndex()
			vt.state = stateNormal
		case 'E': // Next line
			vt.curCol = 0
			vt.index()
			vt.state = stateNormal
		case 'c': // Full reset (RIS)
			vt.reset()
			vt.state = stateNormal
		case '(', ')', '*', '+': // Character set designation — consume next byte
			vt.state = stateCharset
		case 'P': // DCS (Device Control String) — consume until ST
			vt.state = stateDCS
			vt.paramBuf = nil
		case '_', '^': // APC, PM — consume until ST
			vt.state = stateDCS // reuse DCS handler (just consume)
			vt.paramBuf = nil
		case '=': // DECKPAM (application keypad mode) — ignore
			vt.state = stateNormal
		case '>': // DECKPNM (normal keypad mode) — ignore
			vt.state = stateNormal
		case '#': // DEC test — consume next byte
			vt.state = stateCharset // reuse: just eat one byte
		default:
			vt.state = stateNormal
		}
	case stateCSI:
		if b >= 0x30 && b <= 0x3f {
			vt.paramBuf = append(vt.paramBuf, b)
		} else if b >= 0x20 && b <= 0x2f {
			vt.paramBuf = append(vt.paramBuf, b)
		} else {
			vt.handleCSI(b)
			vt.state = stateNormal
		}
	case stateOSC:
		if b == 0x07 { // BEL terminates OSC
			vt.handleOSC()
			vt.state = stateNormal
		} else if b == 0x1b { // ESC — might be start of ST (ESC \)
			vt.state = stateOSCEsc
		} else {
			vt.paramBuf = append(vt.paramBuf, b)
		}
	case stateOSCEsc:
		// After ESC inside OSC: if '\' then it's ST, otherwise abort OSC
		vt.handleOSC()
		vt.state = stateNormal
		if b != '\\' {
			// Not ST — reprocess this byte
			vt.processByte(b)
		}
	case stateCharset:
		// Consume one byte (the charset designator like 'B', '0', etc.) and return to normal
		vt.state = stateNormal
	case stateDCS:
		// Consume bytes until ESC (start of ST) or BEL
		if b == 0x1b {
			vt.state = stateDCSEsc
		} else if b == 0x07 {
			vt.state = stateNormal
		}
		// Otherwise keep consuming
	case stateDCSEsc:
		// After ESC inside DCS: '\' completes ST
		vt.state = stateNormal
		if b != '\\' {
			vt.processByte(b)
		}
	}
}

// decodeUTF8 decodes a UTF-8 byte sequence into a rune.
func decodeUTF8(buf []byte) rune {
	switch len(buf) {
	case 2:
		return rune(buf[0]&0x1F)<<6 | rune(buf[1]&0x3F)
	case 3:
		return rune(buf[0]&0x0F)<<12 | rune(buf[1]&0x3F)<<6 | rune(buf[2]&0x3F)
	case 4:
		return rune(buf[0]&0x07)<<18 | rune(buf[1]&0x3F)<<12 | rune(buf[2]&0x3F)<<6 | rune(buf[3]&0x3F)
	}
	return 0xFFFD // replacement character
}

func (vt *VT) handleOSC() {
	// Check for OSC 133 shell integration: "133;X" where X is A/B/C/D
	if len(vt.paramBuf) >= 4 &&
		vt.paramBuf[0] == '1' && vt.paramBuf[1] == '3' && vt.paramBuf[2] == '3' && vt.paramBuf[3] == ';' {
		if len(vt.paramBuf) >= 5 {
			param := vt.paramBuf[4]
			// Snapshot VT cells before finishing a block, so TUI programs
			// get clean output instead of garbled tracked lines.
			if param == 'D' {
				vt.snapshotActiveBlock()
			}
			vt.blocks.HandleOSC133(param)
		}
	}
	// Other OSC sequences (window title, etc.) are silently ignored
}

// ClearGrid resets all VT cells to blank. Called when a new heuristic block
// starts so previous command output doesn't bleed into the new block's display.
func (vt *VT) ClearGrid() {
	for r := 0; r < vt.rows; r++ {
		for c := 0; c < vt.cols; c++ {
			vt.cells[r][c] = Cell{Ch: ' ', Fg: tcell.ColorWhite, Bg: tcell.ColorDefault}
		}
	}
	vt.curRow = 0
	vt.curCol = 0
}

// snapshotActiveBlock captures the current VT cell contents as the active
// block's output. Called before a block finishes to preserve the display state.
func (vt *VT) snapshotActiveBlock() {
	vt.blocks.SnapshotVTOutput(vt.rows, vt.cols, func(r, c int) rune {
		if r >= 0 && r < vt.rows && c >= 0 && c < vt.cols {
			return vt.cells[r][c].Ch
		}
		return ' '
	})
}

func (vt *VT) putChar(ch rune) {
	w := runeWidth(ch)
	if w == 0 {
		return // zero-width characters (combining marks, etc.) — skip
	}

	// Wrap if we can't fit the character
	if vt.curCol+w > vt.cols {
		vt.curCol = 0
		vt.lineFeed()
	}
	if vt.curRow >= 0 && vt.curRow < vt.rows && vt.curCol >= 0 && vt.curCol < vt.cols {
		vt.cells[vt.curRow][vt.curCol] = Cell{
			Ch:   ch,
			Fg:   vt.fg,
			Bg:   vt.bg,
			Bold: vt.bold,
		}
		// For wide characters, fill the next cell with a zero-width placeholder
		if w == 2 && vt.curCol+1 < vt.cols {
			vt.cells[vt.curRow][vt.curCol+1] = Cell{
				Ch: 0, // placeholder (will be skipped during rendering)
				Fg: vt.fg,
				Bg: vt.bg,
			}
		}
	}
	vt.curCol += w
	vt.blocks.FeedChar(ch)

	// Proactive prompt detection: when a space follows a likely shell prompt
	// during an active block in heuristic mode, the command has finished.
	// This detects the shell prompt as soon as it's drawn (e.g. after Claude exits).
	if ch == ' ' {
		bt := vt.blocks
		active := bt.ActiveBlock()
		if active != nil && !active.Finished && !bt.HasOSC133() && !bt.AltScreen() {
			if bt.IsLikelyShellPrompt() {
				vt.snapshotActiveBlock()
				bt.FinishActiveBlock()
			}
		}
	}
}

// runeWidth returns the display width of a rune (1 or 2 cells).
// Returns 0 for zero-width characters.
func runeWidth(r rune) int {
	// Zero-width characters
	if r == 0 || r == 0xFEFF { // null, BOM
		return 0
	}
	// Combining marks (U+0300-U+036F, U+1AB0-U+1AFF, U+1DC0-U+1DFF, U+20D0-U+20FF, U+FE20-U+FE2F)
	if (r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x1AB0 && r <= 0x1AFF) ||
		(r >= 0x1DC0 && r <= 0x1DFF) ||
		(r >= 0x20D0 && r <= 0x20FF) ||
		(r >= 0xFE20 && r <= 0xFE2F) {
		return 0
	}
	// Wide characters: CJK, emoji, fullwidth forms
	if (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		r == 0x2329 || r == 0x232A || // angle brackets
		(r >= 0x2E80 && r <= 0x303E) || // CJK radicals, Kangxi, ideographic
		(r >= 0x3040 && r <= 0x33BF) || // Hiragana, Katakana, Bopomofo, Hangul compat, Kanbun, CJK
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x4E00 && r <= 0xA4CF) || // CJK Unified Ideographs, Yi
		(r >= 0xA960 && r <= 0xA97C) || // Hangul Jamo Extended-A
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE10 && r <= 0xFE19) || // Vertical forms
		(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
		(r >= 0xFF01 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Signs
		(r >= 0x1F000 && r <= 0x1FBFF) || // Emoji & symbols (Mahjong, Dominos, Playing Cards, Emoji, etc.)
		(r >= 0x20000 && r <= 0x2FFFF) || // CJK Unified Ideographs Extensions B-F
		(r >= 0x30000 && r <= 0x3FFFF) { // CJK Extension G+
		return 2
	}
	return 1
}

// index moves cursor down one line; scrolls the scroll region if at the bottom.
func (vt *VT) index() {
	bottom := vt.scrollRegionBottom()
	if vt.curRow == bottom {
		vt.scrollUp(1)
	} else if vt.curRow < vt.rows-1 {
		vt.curRow++
	}
}

// reverseIndex moves cursor up one line; scrolls the scroll region if at the top.
func (vt *VT) reverseIndex() {
	top := vt.scrollRegionTop()
	if vt.curRow == top {
		vt.scrollDown(1)
	} else if vt.curRow > 0 {
		vt.curRow--
	}
}

// lineFeed moves down; if at scroll region bottom, scrolls up.
func (vt *VT) lineFeed() {
	bottom := vt.scrollRegionBottom()
	if vt.curRow == bottom {
		vt.scrollUp(1)
	} else if vt.curRow < vt.rows-1 {
		vt.curRow++
	}
}

// scrollUp scrolls lines up within the scroll region by n lines.
func (vt *VT) scrollUp(n int) {
	top := vt.scrollRegionTop()
	bottom := vt.scrollRegionBottom()

	for i := 0; i < n; i++ {
		// Save top line to scrollback only if scroll region is the full screen
		if top == 0 && bottom == vt.rows-1 {
			if len(vt.scrollback) < vt.maxScroll {
				saved := make([]Cell, vt.cols)
				copy(saved, vt.cells[0])
				vt.scrollback = append(vt.scrollback, saved)
			} else if len(vt.scrollback) == vt.maxScroll {
				copy(vt.scrollback, vt.scrollback[1:])
				saved := make([]Cell, vt.cols)
				copy(saved, vt.cells[0])
				vt.scrollback[vt.maxScroll-1] = saved
			}
		}
		// Shift lines up within the region
		for r := top; r < bottom; r++ {
			vt.cells[r] = vt.cells[r+1]
		}
		vt.cells[bottom] = make([]Cell, vt.cols)
		for c := range vt.cells[bottom] {
			vt.cells[bottom][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
		}
	}
}

// scrollDown scrolls lines down within the scroll region by n lines.
func (vt *VT) scrollDown(n int) {
	top := vt.scrollRegionTop()
	bottom := vt.scrollRegionBottom()

	for i := 0; i < n; i++ {
		for r := bottom; r > top; r-- {
			vt.cells[r] = vt.cells[r-1]
		}
		vt.cells[top] = make([]Cell, vt.cols)
		for c := range vt.cells[top] {
			vt.cells[top][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
		}
	}
}

func (vt *VT) reset() {
	vt.fg = tcell.ColorWhite
	vt.bg = tcell.ColorDefault
	vt.bold = false
	vt.italic = false
	vt.underline = false
	vt.dim = false
	vt.reverse = false
	vt.cursorVisible = true
	vt.curRow = 0
	vt.curCol = 0
	vt.scrollTop = 0
	vt.scrollBottom = 0
	vt.cells = vt.makeGrid(vt.cols, vt.rows)
}

func (vt *VT) handleCSI(final byte) {
	params := parseParams(vt.paramBuf)

	switch final {
	case 'A': // Cursor up
		n := paramDefault(params, 0, 1)
		vt.curRow -= n
		if vt.curRow < 0 {
			vt.curRow = 0
		}
	case 'B': // Cursor down
		n := paramDefault(params, 0, 1)
		vt.curRow += n
		if vt.curRow >= vt.rows {
			vt.curRow = vt.rows - 1
		}
	case 'C': // Cursor forward
		n := paramDefault(params, 0, 1)
		vt.curCol += n
		if vt.curCol >= vt.cols {
			vt.curCol = vt.cols - 1
		}
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'D': // Cursor backward
		n := paramDefault(params, 0, 1)
		vt.curCol -= n
		if vt.curCol < 0 {
			vt.curCol = 0
		}
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'E': // Cursor next line
		n := paramDefault(params, 0, 1)
		vt.curCol = 0
		vt.curRow += n
		if vt.curRow >= vt.rows {
			vt.curRow = vt.rows - 1
		}
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'F': // Cursor previous line
		n := paramDefault(params, 0, 1)
		vt.curCol = 0
		vt.curRow -= n
		if vt.curRow < 0 {
			vt.curRow = 0
		}
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'G', '`': // Cursor horizontal absolute
		col := paramDefault(params, 0, 1) - 1
		if col < 0 {
			col = 0
		}
		if col >= vt.cols {
			col = vt.cols - 1
		}
		vt.curCol = col
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'd': // Cursor vertical absolute (VPA)
		row := paramDefault(params, 0, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= vt.rows {
			row = vt.rows - 1
		}
		vt.curRow = row
	case 'H', 'f': // Cursor position
		row := paramDefault(params, 0, 1) - 1
		col := paramDefault(params, 1, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= vt.rows {
			row = vt.rows - 1
		}
		if col < 0 {
			col = 0
		}
		if col >= vt.cols {
			col = vt.cols - 1
		}
		vt.curRow = row
		vt.curCol = col
		vt.blocks.FeedCursorMove(vt.curCol)
	case 'J': // Erase display
		n := paramDefault(params, 0, 0)
		switch n {
		case 0:
			vt.clearRange(vt.curRow, vt.curCol, vt.rows-1, vt.cols-1)
		case 1:
			vt.clearRange(0, 0, vt.curRow, vt.curCol)
		case 2, 3:
			vt.clearRange(0, 0, vt.rows-1, vt.cols-1)
		}
	case 'K': // Erase line
		n := paramDefault(params, 0, 0)
		switch n {
		case 0:
			vt.clearRange(vt.curRow, vt.curCol, vt.curRow, vt.cols-1)
		case 1:
			vt.clearRange(vt.curRow, 0, vt.curRow, vt.curCol)
		case 2:
			vt.clearRange(vt.curRow, 0, vt.curRow, vt.cols-1)
		}
	case 'X': // Erase characters
		n := paramDefault(params, 0, 1)
		endCol := vt.curCol + n - 1
		if endCol >= vt.cols {
			endCol = vt.cols - 1
		}
		vt.clearRange(vt.curRow, vt.curCol, vt.curRow, endCol)
	case '@': // Insert characters (ICH)
		n := paramDefault(params, 0, 1)
		if vt.curRow >= 0 && vt.curRow < vt.rows {
			row := vt.cells[vt.curRow]
			// Shift characters right from cursor
			for c := vt.cols - 1; c >= vt.curCol+n; c-- {
				row[c] = row[c-n]
			}
			// Clear inserted area
			for c := vt.curCol; c < vt.curCol+n && c < vt.cols; c++ {
				row[c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
			}
		}
	case 'P': // Delete characters (DCH)
		n := paramDefault(params, 0, 1)
		if vt.curRow >= 0 && vt.curRow < vt.rows {
			row := vt.cells[vt.curRow]
			// Shift characters left from cursor+n
			for c := vt.curCol; c < vt.cols-n; c++ {
				row[c] = row[c+n]
			}
			// Clear vacated area at end
			for c := vt.cols - n; c < vt.cols; c++ {
				if c >= 0 {
					row[c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
				}
			}
		}
	case 'S': // Scroll up
		n := paramDefault(params, 0, 1)
		vt.scrollUp(n)
	case 'T': // Scroll down
		n := paramDefault(params, 0, 1)
		vt.scrollDown(n)
	case 'm': // SGR (Select Graphic Rendition)
		vt.handleSGR(params)
	case 'L': // Insert lines
		n := paramDefault(params, 0, 1)
		for i := 0; i < n && vt.curRow < vt.rows; i++ {
			bottom := vt.scrollRegionBottom()
			for r := bottom; r > vt.curRow; r-- {
				vt.cells[r] = vt.cells[r-1]
			}
			vt.cells[vt.curRow] = make([]Cell, vt.cols)
			for c := range vt.cells[vt.curRow] {
				vt.cells[vt.curRow][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
			}
		}
	case 'M': // Delete lines
		n := paramDefault(params, 0, 1)
		for i := 0; i < n && vt.curRow < vt.rows; i++ {
			bottom := vt.scrollRegionBottom()
			for r := vt.curRow; r < bottom; r++ {
				vt.cells[r] = vt.cells[r+1]
			}
			vt.cells[bottom] = make([]Cell, vt.cols)
			for c := range vt.cells[bottom] {
				vt.cells[bottom][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
			}
		}
	case 'r': // Set scrolling region (DECSTBM)
		top := paramDefault(params, 0, 1)
		bottom := paramDefault(params, 1, vt.rows)
		if top < 1 {
			top = 1
		}
		if bottom > vt.rows {
			bottom = vt.rows
		}
		if top < bottom {
			vt.scrollTop = top
			vt.scrollBottom = bottom
		} else {
			vt.scrollTop = 0
			vt.scrollBottom = 0
		}
		// DECSTBM also homes the cursor
		vt.curRow = 0
		vt.curCol = 0
	case 'h': // Set mode
		if len(vt.paramBuf) > 0 && vt.paramBuf[0] == '?' {
			for _, p := range params {
				switch p {
				case 25: // Show cursor (DECTCEM)
					vt.cursorVisible = true
				case 1049: // Alternate screen buffer (save + switch + clear)
					vt.enterAltScreen()
				case 47, 1047: // Alternate screen (switch only)
					vt.enterAltScreen()
				}
			}
		}
	case 'l': // Reset mode
		if len(vt.paramBuf) > 0 && vt.paramBuf[0] == '?' {
			for _, p := range params {
				switch p {
				case 25: // Hide cursor (DECTCEM)
					vt.cursorVisible = false
				case 1049:
					vt.leaveAltScreen()
				case 47, 1047:
					vt.leaveAltScreen()
				}
			}
		}
	case 'n': // Device status report — ignore
	case 's': // Save cursor
		vt.savedRow = vt.curRow
		vt.savedCol = vt.curCol
	case 'u': // Restore cursor
		vt.curRow = vt.savedRow
		vt.curCol = vt.savedCol
	case 't': // Window manipulation — ignore
	case 'c': // Device attributes — ignore
	}
}

// handleSGR processes Select Graphic Rendition parameters.
func (vt *VT) handleSGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0: // Reset
			vt.fg = tcell.ColorWhite
			vt.bg = tcell.ColorDefault
			vt.bold = false
			vt.italic = false
			vt.underline = false
			vt.dim = false
			vt.reverse = false
		case p == 1:
			vt.bold = true
		case p == 2:
			vt.dim = true
		case p == 3:
			vt.italic = true
		case p == 4:
			vt.underline = true
		case p == 7:
			vt.reverse = true
		case p == 22:
			vt.bold = false
			vt.dim = false
		case p == 23:
			vt.italic = false
		case p == 24:
			vt.underline = false
		case p == 27:
			vt.reverse = false
		case p >= 30 && p <= 37: // Standard foreground
			vt.fg = ansiColor(p - 30)
		case p == 38: // Extended foreground
			if i+1 < len(params) {
				switch params[i+1] {
				case 5: // 256-color: 38;5;N
					if i+2 < len(params) {
						vt.fg = tcell.PaletteColor(params[i+2])
						i += 2
					}
				case 2: // RGB: 38;2;R;G;B
					if i+4 < len(params) {
						vt.fg = tcell.NewRGBColor(int32(params[i+2]), int32(params[i+3]), int32(params[i+4]))
						i += 4
					}
				}
			}
		case p == 39: // Default foreground
			vt.fg = tcell.ColorWhite
		case p >= 40 && p <= 47: // Standard background
			vt.bg = ansiColor(p - 40)
		case p == 48: // Extended background
			if i+1 < len(params) {
				switch params[i+1] {
				case 5: // 256-color: 48;5;N
					if i+2 < len(params) {
						vt.bg = tcell.PaletteColor(params[i+2])
						i += 2
					}
				case 2: // RGB: 48;2;R;G;B
					if i+4 < len(params) {
						vt.bg = tcell.NewRGBColor(int32(params[i+2]), int32(params[i+3]), int32(params[i+4]))
						i += 4
					}
				}
			}
		case p == 49: // Default background
			vt.bg = tcell.ColorDefault
		case p >= 90 && p <= 97: // Bright foreground
			vt.fg = ansiBrightColor(p - 90)
		case p >= 100 && p <= 107: // Bright background
			vt.bg = ansiBrightColor(p - 100)
		}
	}
}

// ansiColor maps 0-7 to tcell standard colors.
func ansiColor(idx int) tcell.Color {
	switch idx {
	case 0:
		return tcell.ColorBlack
	case 1:
		return tcell.ColorMaroon
	case 2:
		return tcell.ColorGreen
	case 3:
		return tcell.ColorOlive
	case 4:
		return tcell.ColorNavy
	case 5:
		return tcell.ColorPurple
	case 6:
		return tcell.ColorTeal
	case 7:
		return tcell.ColorSilver
	}
	return tcell.ColorWhite
}

// ansiBrightColor maps 0-7 to tcell bright colors.
func ansiBrightColor(idx int) tcell.Color {
	switch idx {
	case 0:
		return tcell.ColorGray
	case 1:
		return tcell.ColorRed
	case 2:
		return tcell.ColorLime
	case 3:
		return tcell.ColorYellow
	case 4:
		return tcell.ColorBlue
	case 5:
		return tcell.ColorFuchsia
	case 6:
		return tcell.ColorAqua
	case 7:
		return tcell.ColorWhite
	}
	return tcell.ColorWhite
}

// enterAltScreen saves the main screen and switches to a blank alternate screen.
func (vt *VT) enterAltScreen() {
	if vt.altActive {
		return
	}
	vt.altActive = true
	vt.blocks.SetAltScreen(true)

	// Save main screen state
	vt.altCells = vt.cells
	vt.altCurRow = vt.curRow
	vt.altCurCol = vt.curCol
	vt.altSavedRow = vt.savedRow
	vt.altSavedCol = vt.savedCol

	// Create blank alternate screen
	vt.cells = vt.makeGrid(vt.cols, vt.rows)
	vt.curRow = 0
	vt.curCol = 0
	vt.scrollTop = 0
	vt.scrollBottom = 0
}

// leaveAltScreen restores the main screen from the alternate screen.
func (vt *VT) leaveAltScreen() {
	if !vt.altActive {
		return
	}
	vt.altActive = false
	vt.blocks.SetAltScreen(false)

	// Restore main screen state
	if vt.altCells != nil {
		vt.cells = vt.altCells
		vt.altCells = nil
	}
	vt.curRow = vt.altCurRow
	vt.curCol = vt.altCurCol
	vt.savedRow = vt.altSavedRow
	vt.savedCol = vt.altSavedCol
	vt.scrollTop = 0
	vt.scrollBottom = 0
}

func (vt *VT) clearRange(r1, c1, r2, c2 int) {
	for r := r1; r <= r2 && r < vt.rows; r++ {
		startC := 0
		endC := vt.cols - 1
		if r == r1 {
			startC = c1
		}
		if r == r2 {
			endC = c2
		}
		for c := startC; c <= endC && c < vt.cols; c++ {
			if c >= 0 {
				vt.cells[r][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
			}
		}
	}
}

func parseParams(buf []byte) []int {
	if len(buf) == 0 {
		return nil
	}
	var params []int
	current := 0
	hasDigit := false
	for _, b := range buf {
		if b >= '0' && b <= '9' {
			current = current*10 + int(b-'0')
			hasDigit = true
		} else if b == ';' {
			params = append(params, current)
			current = 0
			hasDigit = false
		} else if b == '?' || b == '>' || b == '!' {
			// Private mode indicators — skip, params parsed from digits
			continue
		}
	}
	if hasDigit || len(buf) > 0 {
		params = append(params, current)
	}
	return params
}

func paramDefault(params []int, idx, def int) int {
	if idx < len(params) && params[idx] > 0 {
		return params[idx]
	}
	return def
}
