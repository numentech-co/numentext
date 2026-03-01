package terminal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Panel is a tview-compatible widget that renders a VT terminal
type Panel struct {
	*tview.Box
	term       *Terminal
	hasFocus   bool
	scrollOff  int // lines scrolled back into scrollback (0 = live)
}

// NewPanel creates a new terminal panel widget
func NewPanel() *Panel {
	p := &Panel{
		Box: tview.NewBox(),
	}
	p.SetBackgroundColor(ui.ColorOutputBg)
	return p
}

// SetTerminal attaches a Terminal to this panel
func (p *Panel) SetTerminal(t *Terminal) {
	p.term = t
}

// Terminal returns the attached terminal
func (p *Panel) Terminal() *Terminal {
	return p.term
}

// Draw renders the VT grid to the screen
func (p *Panel) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)
	x, y, width, height := p.GetInnerRect()

	if p.term == nil {
		// Draw empty panel with message
		msg := "Press Ctrl+` to open terminal"
		style := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorOutputBg)
		for i, ch := range msg {
			if x+i < x+width {
				screen.SetContent(x+i, y, ch, nil, style)
			}
		}
		return
	}

	p.term.Lock()
	vt := p.term.VT()

	// Resize VT to match panel dimensions if needed
	if vt.Cols() != width || vt.Rows() != height {
		vt.Resize(width, height)
	}

	scrollback := vt.Scrollback()
	scrollOff := p.scrollOff
	if scrollOff > len(scrollback) {
		scrollOff = len(scrollback)
	}

	if scrollOff > 0 {
		// Render scrollback + partial screen
		// We show: scrollback lines from (len-scrollOff) .. end, then screen lines
		sbStart := len(scrollback) - scrollOff
		screenRow := 0

		// Scrollback lines
		for i := sbStart; i < len(scrollback) && screenRow < height; i++ {
			line := scrollback[i]
			for col := 0; col < width; col++ {
				if col < len(line) {
					cell := line[col]
					style := tcell.StyleDefault.Foreground(cell.Fg).Background(cell.Bg)
					if cell.Bold {
						style = style.Bold(true)
					}
					ch := cell.Ch
					if ch == 0 {
						ch = ' '
					}
					screen.SetContent(x+col, y+screenRow, ch, nil, style)
				} else {
					screen.SetContent(x+col, y+screenRow, ' ', nil,
						tcell.StyleDefault.Background(ui.ColorOutputBg))
				}
			}
			screenRow++
		}

		// Remaining rows from live screen
		vtRow := 0
		for screenRow < height && vtRow < vt.Rows() {
			for col := 0; col < width && col < vt.Cols(); col++ {
				cell := vt.Cell(vtRow, col)
				style := tcell.StyleDefault.Foreground(cell.Fg).Background(cell.Bg)
				if cell.Bold {
					style = style.Bold(true)
				}
				ch := cell.Ch
				if ch == 0 {
					ch = ' '
				}
				screen.SetContent(x+col, y+screenRow, ch, nil, style)
			}
			screenRow++
			vtRow++
		}
	} else {
		// Live view: render screen cells directly
		for row := 0; row < height && row < vt.Rows(); row++ {
			for col := 0; col < width && col < vt.Cols(); col++ {
				cell := vt.Cell(row, col)
				style := tcell.StyleDefault.
					Foreground(cell.Fg).
					Background(cell.Bg)
				if cell.Bold {
					style = style.Bold(true)
				}
				ch := cell.Ch
				if ch == 0 {
					ch = ' '
				}
				screen.SetContent(x+col, y+row, ch, nil, style)
			}
		}
	}

	// Draw cursor if focused, live view, and terminal running
	if p.hasFocus && scrollOff == 0 && p.term.Running() {
		curRow := vt.CursorRow()
		curCol := vt.CursorCol()
		if curRow >= 0 && curRow < height && curCol >= 0 && curCol < width {
			cell := vt.Cell(curRow, curCol)
			style := tcell.StyleDefault.
				Foreground(cell.Bg).
				Background(cell.Fg).
				Reverse(true)
			ch := cell.Ch
			if ch == 0 || ch == ' ' {
				ch = ' '
			}
			screen.SetContent(x+curCol, y+curRow, ch, nil, style)
		}
	}

	p.term.Unlock()
}

// InputHandler processes keyboard input, forwarding to PTY
func (p *Panel) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		if p.term == nil || !p.term.Running() {
			return
		}

		// Shift+PgUp/PgDn for scrollback
		shift := event.Modifiers()&tcell.ModShift != 0
		if shift {
			_, _, _, height := p.GetInnerRect()
			switch event.Key() {
			case tcell.KeyPgUp:
				p.scrollOff += height / 2
				p.term.Lock()
				maxScroll := len(p.term.VT().Scrollback())
				p.term.Unlock()
				if p.scrollOff > maxScroll {
					p.scrollOff = maxScroll
				}
				return
			case tcell.KeyPgDn:
				p.scrollOff -= height / 2
				if p.scrollOff < 0 {
					p.scrollOff = 0
				}
				return
			}
		}

		// Any other key resets scroll to live
		if p.scrollOff > 0 {
			p.scrollOff = 0
		}

		data := keyToBytes(event)
		if data != nil {
			p.term.WriteInput(data)
		}
	})
}

// Focus marks this panel as focused
func (p *Panel) Focus(delegate func(tview.Primitive)) {
	p.hasFocus = true
	p.Box.Focus(delegate)
}

// Blur marks this panel as unfocused
func (p *Panel) Blur() {
	p.hasFocus = false
	p.Box.Blur()
}

// HasFocus returns true if the panel has focus
func (p *Panel) HasFocus() bool {
	return p.hasFocus
}

// keyToBytes converts a tcell key event to terminal escape bytes
func keyToBytes(ev *tcell.EventKey) []byte {
	// Check for special keys first
	switch ev.Key() {
	case tcell.KeyEnter:
		return []byte{'\r'}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return []byte{0x7f}
	case tcell.KeyTab:
		return []byte{'\t'}
	case tcell.KeyEscape:
		return []byte{0x1b}
	case tcell.KeyUp:
		return []byte("\x1b[A")
	case tcell.KeyDown:
		return []byte("\x1b[B")
	case tcell.KeyRight:
		return []byte("\x1b[C")
	case tcell.KeyLeft:
		return []byte("\x1b[D")
	case tcell.KeyHome:
		return []byte("\x1b[H")
	case tcell.KeyEnd:
		return []byte("\x1b[F")
	case tcell.KeyInsert:
		return []byte("\x1b[2~")
	case tcell.KeyDelete:
		return []byte("\x1b[3~")
	case tcell.KeyPgUp:
		return []byte("\x1b[5~")
	case tcell.KeyPgDn:
		return []byte("\x1b[6~")
	case tcell.KeyF1:
		return []byte("\x1bOP")
	case tcell.KeyF2:
		return []byte("\x1bOQ")
	case tcell.KeyF3:
		return []byte("\x1bOR")
	case tcell.KeyF4:
		return []byte("\x1bOS")
	case tcell.KeyF5:
		return []byte("\x1b[15~")
	case tcell.KeyF6:
		return []byte("\x1b[17~")
	case tcell.KeyF7:
		return []byte("\x1b[18~")
	case tcell.KeyF8:
		return []byte("\x1b[19~")
	case tcell.KeyF9:
		return []byte("\x1b[20~")
	case tcell.KeyF10:
		return []byte("\x1b[21~")
	case tcell.KeyF11:
		return []byte("\x1b[23~")
	case tcell.KeyF12:
		return []byte("\x1b[24~")
	case tcell.KeyCtrlA:
		return []byte{0x01}
	case tcell.KeyCtrlB:
		return []byte{0x02}
	case tcell.KeyCtrlC:
		return []byte{0x03}
	case tcell.KeyCtrlD:
		return []byte{0x04}
	case tcell.KeyCtrlE:
		return []byte{0x05}
	case tcell.KeyCtrlF:
		return []byte{0x06}
	case tcell.KeyCtrlG:
		return []byte{0x07}
	case tcell.KeyCtrlK:
		return []byte{0x0b}
	case tcell.KeyCtrlL:
		return []byte{0x0c}
	case tcell.KeyCtrlN:
		return []byte{0x0e}
	case tcell.KeyCtrlO:
		return []byte{0x0f}
	case tcell.KeyCtrlP:
		return []byte{0x10}
	case tcell.KeyCtrlR:
		return []byte{0x12}
	case tcell.KeyCtrlT:
		return []byte{0x14}
	case tcell.KeyCtrlU:
		return []byte{0x15}
	case tcell.KeyCtrlW:
		return []byte{0x17}
	case tcell.KeyCtrlZ:
		return []byte{0x1a}
	case tcell.KeyRune:
		r := ev.Rune()
		if r != 0 {
			buf := make([]byte, 4)
			n := encodeRune(buf, r)
			return buf[:n]
		}
	}
	return nil
}

// encodeRune encodes a rune as UTF-8 bytes
func encodeRune(buf []byte, r rune) int {
	if r < 0x80 {
		buf[0] = byte(r)
		return 1
	} else if r < 0x800 {
		buf[0] = byte(0xC0 | (r >> 6))
		buf[1] = byte(0x80 | (r & 0x3F))
		return 2
	} else if r < 0x10000 {
		buf[0] = byte(0xE0 | (r >> 12))
		buf[1] = byte(0x80 | ((r >> 6) & 0x3F))
		buf[2] = byte(0x80 | (r & 0x3F))
		return 3
	}
	buf[0] = byte(0xF0 | (r >> 18))
	buf[1] = byte(0x80 | ((r >> 12) & 0x3F))
	buf[2] = byte(0x80 | ((r >> 6) & 0x3F))
	buf[3] = byte(0x80 | (r & 0x3F))
	return 4
}
