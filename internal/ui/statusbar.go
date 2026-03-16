package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// StatusBar displays file info, cursor position, and shortcut hints
type StatusBar struct {
	*tview.Box
	filename     string
	line         int
	col          int
	language     string
	encoding     string
	modified     bool
	message      string
	modeLabel    string
	modePending  string
	commandText  string
	focusedPanel string // name of currently focused panel
	wordWrap     bool
	hasErrors    bool
	lineEnding   string // "LF", "CRLF", or "CR"
	hasBOM       bool
}

func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		Box:      tview.NewBox(),
		encoding: "UTF-8",
		line:     1,
		col:      1,
	}
	sb.SetBackgroundColor(ColorStatusBg)
	return sb
}

func (sb *StatusBar) Update(filename string, line, col int, language string, modified bool) {
	sb.filename = filename
	sb.line = line + 1 // Display as 1-based
	sb.col = col + 1
	sb.language = language
	sb.modified = modified
}

func (sb *StatusBar) SetMessage(msg string) {
	sb.message = msg
}

func (sb *StatusBar) SetModeInfo(label, pending string) {
	sb.modeLabel = label
	sb.modePending = pending
}

func (sb *StatusBar) SetCommandText(text string) {
	sb.commandText = text
}

// SetFocusedPanel updates the panel name shown in the status bar (e.g. "Editor", "File Tree").
func (sb *StatusBar) SetFocusedPanel(name string) {
	sb.focusedPanel = name
}

func (sb *StatusBar) SetWordWrap(on bool) {
	sb.wordWrap = on
}

func (sb *StatusBar) SetHasErrors(has bool) {
	sb.hasErrors = has
}

func (sb *StatusBar) SetLineEnding(le string) {
	sb.lineEnding = le
}

func (sb *StatusBar) SetHasBOM(has bool) {
	sb.hasBOM = has
}

func (sb *StatusBar) Draw(screen tcell.Screen) {
	sb.Box.DrawForSubclass(screen, sb)
	x, y, width, _ := sb.GetInnerRect()

	style := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorStatusBg)

	// Clear
	for cx := x; cx < x+width; cx++ {
		screen.SetContent(cx, y, ' ', nil, style)
	}

	// If showing command text (Vi : mode), render just that
	if sb.commandText != "" {
		for i, ch := range sb.commandText {
			if x+i < x+width {
				screen.SetContent(x+i, y, ch, nil, style)
			}
		}
		return
	}

	// Left side: mode indicator + filename and position
	left := ""
	if sb.modeLabel != "" {
		left = " " + sb.modeLabel
		if sb.modePending != "" {
			left += " " + sb.modePending
		}
		left += " |"
	}
	if sb.filename != "" {
		left += " " + sb.filename
		if sb.modified {
			left += " " + Style.ModifiedIndicator()
		}
		left += fmt.Sprintf(" | Ln %d, Col %d", sb.line, sb.col)
		if sb.language != "" {
			left += " | " + sb.language
		}
		left += " | " + sb.encoding
		if sb.hasBOM {
			left += " BOM"
		}
		if sb.lineEnding != "" {
			left += " | " + sb.lineEnding
		}
		if sb.wordWrap {
			left += " | " + Style.WrapIndicator()
		}
	} else if sb.message != "" {
		left += " " + sb.message
	} else if left == "" {
		left = " NumenText"
	}

	for i, ch := range left {
		if x+i < x+width {
			screen.SetContent(x+i, y, ch, nil, style)
		}
	}
	leftEnd := x + len(left)

	// Right side: panel indicator + shortcut hints
	right := "F5:Run  F9:Build  F10:Menu "
	if sb.hasErrors {
		right = "^E:Err  " + right
	}
	if sb.focusedPanel != "" {
		right = "[" + sb.focusedPanel + "]  " + right
	}
	rightStart := x + width - len(right)
	for i, ch := range right {
		if rightStart+i >= x && rightStart+i < x+width {
			screen.SetContent(rightStart+i, y, ch, nil, style)
		}
	}

	// Middle: message (between file info and shortcuts)
	if sb.message != "" && sb.filename != "" {
		msgStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(ColorStatusBg).Bold(true)
		msg := " | " + sb.message
		for i, ch := range msg {
			pos := leftEnd + i
			if pos >= rightStart-1 {
				break // don't overlap right side
			}
			if pos < x+width {
				screen.SetContent(pos, y, ch, nil, msgStyle)
			}
		}
	}
}
