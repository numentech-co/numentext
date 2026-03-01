package terminal

import (
	"github.com/gdamore/tcell/v2"
)

// Cell represents a single character cell in the terminal grid
type Cell struct {
	Ch    rune
	Fg    tcell.Color
	Bg    tcell.Color
	Bold  bool
}

// VT is a minimal VT100/ANSI terminal state machine
type VT struct {
	cols, rows int
	cells      [][]Cell
	curRow     int
	curCol     int
	savedRow   int
	savedCol   int
	scrollback [][]Cell
	maxScroll  int

	// Current attributes
	fg   tcell.Color
	bg   tcell.Color
	bold bool

	// Parser state
	state    parseState
	paramBuf []byte
}

type parseState int

const (
	stateNormal parseState = iota
	stateEsc
	stateCSI
	stateOSC
)

func NewVT(cols, rows int) *VT {
	vt := &VT{
		cols:      cols,
		rows:      rows,
		maxScroll: 1000,
		fg:        tcell.ColorWhite,
		bg:        tcell.ColorDefault,
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
	// Copy existing content
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
}

func (vt *VT) Rows() int { return vt.rows }
func (vt *VT) Cols() int { return vt.cols }
func (vt *VT) CursorRow() int { return vt.curRow }
func (vt *VT) CursorCol() int { return vt.curCol }

func (vt *VT) Cell(row, col int) Cell {
	if row < 0 || row >= vt.rows || col < 0 || col >= vt.cols {
		return Cell{Ch: ' ', Fg: tcell.ColorWhite, Bg: tcell.ColorDefault}
	}
	return vt.cells[row][col]
}

func (vt *VT) Scrollback() [][]Cell {
	return vt.scrollback
}

// Write processes raw terminal output bytes
func (vt *VT) Write(data []byte) {
	for _, b := range data {
		vt.processByte(b)
	}
}

func (vt *VT) processByte(b byte) {
	switch vt.state {
	case stateNormal:
		switch b {
		case 0x1b: // ESC
			vt.state = stateEsc
			vt.paramBuf = nil
		case '\n':
			vt.lineFeed()
		case '\r':
			vt.curCol = 0
		case '\t':
			vt.curCol = (vt.curCol + 8) &^ 7
			if vt.curCol >= vt.cols {
				vt.curCol = vt.cols - 1
			}
		case '\b':
			if vt.curCol > 0 {
				vt.curCol--
			}
		case 0x07: // BEL - ignore
		default:
			if b >= 0x20 {
				vt.putChar(rune(b))
			}
		}
	case stateEsc:
		switch b {
		case '[':
			vt.state = stateCSI
			vt.paramBuf = nil
		case ']':
			vt.state = stateOSC
			vt.paramBuf = nil
		case '7': // Save cursor
			vt.savedRow = vt.curRow
			vt.savedCol = vt.curCol
			vt.state = stateNormal
		case '8': // Restore cursor
			vt.curRow = vt.savedRow
			vt.curCol = vt.savedCol
			vt.state = stateNormal
		case 'D': // Index (move down)
			vt.lineFeed()
			vt.state = stateNormal
		case 'M': // Reverse index (move up)
			if vt.curRow > 0 {
				vt.curRow--
			}
			vt.state = stateNormal
		case 'c': // Reset
			vt.reset()
			vt.state = stateNormal
		default:
			vt.state = stateNormal
		}
	case stateCSI:
		if b >= 0x30 && b <= 0x3f {
			// Parameter byte
			vt.paramBuf = append(vt.paramBuf, b)
		} else if b >= 0x20 && b <= 0x2f {
			// Intermediate byte
			vt.paramBuf = append(vt.paramBuf, b)
		} else {
			// Final byte
			vt.handleCSI(b)
			vt.state = stateNormal
		}
	case stateOSC:
		if b == 0x07 || b == 0x1b { // BEL or ESC terminates OSC
			vt.state = stateNormal
		}
		// Otherwise accumulate and ignore OSC sequences
	}
}

func (vt *VT) putChar(ch rune) {
	if vt.curCol >= vt.cols {
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
	}
	vt.curCol++
}

func (vt *VT) lineFeed() {
	if vt.curRow < vt.rows-1 {
		vt.curRow++
	} else {
		// Scroll up: save top line to scrollback
		if len(vt.scrollback) < vt.maxScroll {
			saved := make([]Cell, vt.cols)
			copy(saved, vt.cells[0])
			vt.scrollback = append(vt.scrollback, saved)
		} else if len(vt.scrollback) == vt.maxScroll {
			// Rotate scrollback
			copy(vt.scrollback, vt.scrollback[1:])
			saved := make([]Cell, vt.cols)
			copy(saved, vt.cells[0])
			vt.scrollback[vt.maxScroll-1] = saved
		}
		// Shift all lines up
		for r := 0; r < vt.rows-1; r++ {
			vt.cells[r] = vt.cells[r+1]
		}
		vt.cells[vt.rows-1] = make([]Cell, vt.cols)
		for c := range vt.cells[vt.rows-1] {
			vt.cells[vt.rows-1][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
		}
	}
}

func (vt *VT) reset() {
	vt.fg = tcell.ColorWhite
	vt.bg = tcell.ColorDefault
	vt.bold = false
	vt.curRow = 0
	vt.curCol = 0
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
	case 'D': // Cursor backward
		n := paramDefault(params, 0, 1)
		vt.curCol -= n
		if vt.curCol < 0 {
			vt.curCol = 0
		}
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
	case 'J': // Erase display
		n := paramDefault(params, 0, 0)
		switch n {
		case 0: // Erase from cursor to end
			vt.clearRange(vt.curRow, vt.curCol, vt.rows-1, vt.cols-1)
		case 1: // Erase from start to cursor
			vt.clearRange(0, 0, vt.curRow, vt.curCol)
		case 2, 3: // Erase entire display
			vt.clearRange(0, 0, vt.rows-1, vt.cols-1)
		}
	case 'K': // Erase line
		n := paramDefault(params, 0, 0)
		switch n {
		case 0: // Erase from cursor to end of line
			vt.clearRange(vt.curRow, vt.curCol, vt.curRow, vt.cols-1)
		case 1: // Erase from start of line to cursor
			vt.clearRange(vt.curRow, 0, vt.curRow, vt.curCol)
		case 2: // Erase entire line
			vt.clearRange(vt.curRow, 0, vt.curRow, vt.cols-1)
		}
	case 'm': // SGR (Select Graphic Rendition)
		if len(params) == 0 {
			params = []int{0}
		}
		for i := 0; i < len(params); i++ {
			switch params[i] {
			case 0: // Reset
				vt.fg = tcell.ColorWhite
				vt.bg = tcell.ColorDefault
				vt.bold = false
			case 1:
				vt.bold = true
			case 22:
				vt.bold = false
			case 30:
				vt.fg = tcell.ColorBlack
			case 31:
				vt.fg = tcell.ColorRed
			case 32:
				vt.fg = tcell.ColorGreen
			case 33:
				vt.fg = tcell.ColorYellow
			case 34:
				vt.fg = tcell.ColorBlue
			case 35:
				vt.fg = tcell.ColorDarkMagenta
			case 36:
				vt.fg = tcell.ColorDarkCyan
			case 37:
				vt.fg = tcell.ColorWhite
			case 39:
				vt.fg = tcell.ColorWhite
			case 40:
				vt.bg = tcell.ColorBlack
			case 41:
				vt.bg = tcell.ColorRed
			case 42:
				vt.bg = tcell.ColorGreen
			case 43:
				vt.bg = tcell.ColorYellow
			case 44:
				vt.bg = tcell.ColorBlue
			case 45:
				vt.bg = tcell.ColorDarkMagenta
			case 46:
				vt.bg = tcell.ColorDarkCyan
			case 47:
				vt.bg = tcell.ColorWhite
			case 49:
				vt.bg = tcell.ColorDefault
			case 90:
				vt.fg = tcell.ColorDarkGray
			case 91:
				vt.fg = tcell.ColorRed
			case 92:
				vt.fg = tcell.ColorGreen
			case 93:
				vt.fg = tcell.ColorYellow
			case 94:
				vt.fg = tcell.ColorBlue
			case 95:
				vt.fg = tcell.ColorDarkMagenta
			case 96:
				vt.fg = tcell.ColorDarkCyan
			case 97:
				vt.fg = tcell.ColorWhite
			}
		}
	case 'L': // Insert lines
		n := paramDefault(params, 0, 1)
		for i := 0; i < n && vt.curRow < vt.rows; i++ {
			// Shift lines down from cursor
			for r := vt.rows - 1; r > vt.curRow; r-- {
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
			for r := vt.curRow; r < vt.rows-1; r++ {
				vt.cells[r] = vt.cells[r+1]
			}
			vt.cells[vt.rows-1] = make([]Cell, vt.cols)
			for c := range vt.cells[vt.rows-1] {
				vt.cells[vt.rows-1][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
			}
		}
	case 'r': // Set scrolling region — simplified, just reset
		// Ignore scroll region for now
	case 'h', 'l': // Set/reset mode — ignore most
		// Ignore
	case 'n': // Device status report — ignore
	case 's': // Save cursor
		vt.savedRow = vt.curRow
		vt.savedCol = vt.curCol
	case 'u': // Restore cursor
		vt.curRow = vt.savedRow
		vt.curCol = vt.savedCol
	}
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
			vt.cells[r][c] = Cell{Ch: ' ', Fg: vt.fg, Bg: vt.bg}
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
