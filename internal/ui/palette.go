package ui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PaletteCommand represents a single command in the palette.
type PaletteCommand struct {
	Label    string
	Shortcut string
	Action   func()
}

// CommandPalette is an overlay widget that renders a fuzzy-searchable command
// list directly to the tcell.Screen, similar to how the editor renders itself.
type CommandPalette struct {
	*tview.Box
	input     string
	commands  []PaletteCommand
	filtered  []PaletteCommand
	selected  int
	onExecute func()
	onClose   func()
}

// NewCommandPalette creates a CommandPalette with the given commands.
// onExecute is called after the chosen command's Action runs.
// onClose is called when the palette is dismissed without executing.
func NewCommandPalette(commands []PaletteCommand, onExecute func(), onClose func()) *CommandPalette {
	p := &CommandPalette{
		Box:       tview.NewBox(),
		commands:  commands,
		onExecute: onExecute,
		onClose:   onClose,
	}
	p.refilter()
	return p
}

// refilter rebuilds the filtered slice based on the current input.
func (p *CommandPalette) refilter() {
	if p.input == "" {
		p.filtered = make([]PaletteCommand, len(p.commands))
		copy(p.filtered, p.commands)
	} else {
		needle := strings.ToLower(p.input)
		p.filtered = p.filtered[:0]
		for _, cmd := range p.commands {
			if fuzzyMatch(needle, strings.ToLower(cmd.Label)) {
				p.filtered = append(p.filtered, cmd)
			}
		}
	}
	if p.selected >= len(p.filtered) {
		p.selected = 0
	}
}

// fuzzyMatch returns true when all characters of needle appear in haystack
// in order (case-insensitive substring-subsequence match).
// Simple but effective: matches "saveas" against "Save As...".
func fuzzyMatch(needle, haystack string) bool {
	ni := 0
	needleRunes := []rune(needle)
	for _, ch := range haystack {
		if ni < len(needleRunes) && unicode.ToLower(ch) == needleRunes[ni] {
			ni++
		}
	}
	return ni == len(needleRunes)
}

// Draw renders the palette overlay centered at the top of the screen.
func (p *CommandPalette) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)

	sw, sh := screen.Size()

	// Palette dimensions: 60 wide, up to 16 rows (1 header + 1 input + 14 list)
	maxVisible := 14
	if maxVisible > len(p.filtered) {
		maxVisible = len(p.filtered)
	}
	paletteW := 64
	if paletteW > sw-4 {
		paletteW = sw - 4
	}
	paletteH := 2 + maxVisible // input row + list rows
	if paletteH < 3 {
		paletteH = 3
	}

	// Position: horizontally centered, near the top
	px := (sw - paletteW) / 2
	py := 2
	if py+paletteH > sh {
		py = 0
	}

	bgStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)
	hlStyle := tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)
	scStyle := tcell.StyleDefault.Foreground(ColorMenuShortcut).Background(ColorDialogBg)
	scHlStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorMenuHighlight)
	borderStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)

	// Draw border frame (ASCII only, per project convention)
	// Top border
	p.drawHLine(screen, px, py, paletteW, borderStyle, true)
	// Title
	title := " Command Palette "
	titleX := px + (paletteW-len(title))/2
	if titleX < px+1 {
		titleX = px + 1
	}
	for i, ch := range title {
		if titleX+i < px+paletteW-1 {
			screen.SetContent(titleX+i, py, ch, nil, borderStyle)
		}
	}

	// Input row border
	inputY := py + 1
	screen.SetContent(px, inputY, '|', nil, borderStyle)
	screen.SetContent(px+paletteW-1, inputY, '|', nil, borderStyle)

	// Draw input field background
	prompt := "> "
	for cx := px + 1; cx < px+paletteW-1; cx++ {
		screen.SetContent(cx, inputY, ' ', nil, bgStyle)
	}
	// Draw prompt
	for i, ch := range prompt {
		if px+1+i < px+paletteW-2 {
			screen.SetContent(px+1+i, inputY, ch, nil, bgStyle)
		}
	}
	// Draw input text
	inputStart := px + 1 + len(prompt)
	displayInput := p.input
	maxInputLen := paletteW - 2 - len(prompt) - 1
	if len(displayInput) > maxInputLen {
		displayInput = displayInput[len(displayInput)-maxInputLen:]
	}
	for i, ch := range displayInput {
		if inputStart+i < px+paletteW-1 {
			screen.SetContent(inputStart+i, inputY, ch, nil,
				tcell.StyleDefault.Foreground(ColorTextPrimary).Background(ColorDialogBg))
		}
	}
	// Draw cursor (block)
	cursorX := inputStart + len([]rune(displayInput))
	if cursorX < px+paletteW-1 {
		screen.SetContent(cursorX, inputY, '_', nil,
			tcell.StyleDefault.Foreground(ColorTextPrimary).Background(ColorDialogBg))
	}

	// Separator between input and list
	sepY := inputY + 1
	p.drawHLine(screen, px, sepY, paletteW, borderStyle, false)

	// Draw list items
	scrollOffset := 0
	if p.selected >= maxVisible {
		scrollOffset = p.selected - maxVisible + 1
	}

	for i := 0; i < maxVisible; i++ {
		idx := scrollOffset + i
		if idx >= len(p.filtered) {
			break
		}
		cmd := p.filtered[idx]
		iy := sepY + 1 + i

		isSelected := idx == p.selected
		rowStyle := bgStyle
		shortcutRowStyle := scStyle
		if isSelected {
			rowStyle = hlStyle
			shortcutRowStyle = scHlStyle
		}

		// Clear line
		screen.SetContent(px, iy, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, iy, '|', nil, borderStyle)
		for cx := px + 1; cx < px+paletteW-1; cx++ {
			screen.SetContent(cx, iy, ' ', nil, rowStyle)
		}

		// Draw label (with 2-char left padding)
		labelRunes := []rune(cmd.Label)
		for j, ch := range labelRunes {
			cx := px + 2 + j
			if cx >= px+paletteW-2 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, rowStyle)
		}

		// Draw shortcut right-aligned if it fits
		if cmd.Shortcut != "" {
			scRunes := []rune(cmd.Shortcut)
			scX := px + paletteW - 1 - len(scRunes) - 1
			if scX > px+2+len(labelRunes)+1 {
				for j, ch := range scRunes {
					screen.SetContent(scX+j, iy, ch, nil, shortcutRowStyle)
				}
			}
		}
	}

	// Bottom border
	bottomY := sepY + 1 + maxVisible
	if maxVisible == 0 {
		// No results: draw a "no results" row
		noY := sepY + 1
		screen.SetContent(px, noY, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, noY, '|', nil, borderStyle)
		msg := "  No matching commands"
		for i, ch := range msg {
			cx := px + 1 + i
			if cx < px+paletteW-1 {
				screen.SetContent(cx, noY, ch, nil, bgStyle)
			}
		}
		bottomY = noY + 1
	}
	p.drawHLine(screen, px, bottomY, paletteW, borderStyle, false)
}

// drawHLine draws a horizontal ASCII border line.
// If isTop is true it uses the top-corners style with '+' corners.
func (p *CommandPalette) drawHLine(screen tcell.Screen, x, y, w int, style tcell.Style, isTop bool) {
	screen.SetContent(x, y, '+', nil, style)
	for cx := x + 1; cx < x+w-1; cx++ {
		screen.SetContent(cx, y, '-', nil, style)
	}
	screen.SetContent(x+w-1, y, '+', nil, style)
}

// InputHandler handles all keyboard input for the palette.
func (p *CommandPalette) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEscape:
			if p.onClose != nil {
				p.onClose()
			}

		case tcell.KeyEnter:
			if len(p.filtered) > 0 {
				cmd := p.filtered[p.selected]
				if p.onExecute != nil {
					p.onExecute()
				}
				if cmd.Action != nil {
					cmd.Action()
				}
			} else {
				if p.onClose != nil {
					p.onClose()
				}
			}

		case tcell.KeyUp:
			if len(p.filtered) > 0 {
				p.selected--
				if p.selected < 0 {
					p.selected = len(p.filtered) - 1
				}
			}

		case tcell.KeyDown:
			if len(p.filtered) > 0 {
				p.selected++
				if p.selected >= len(p.filtered) {
					p.selected = 0
				}
			}

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(p.input) > 0 {
				runes := []rune(p.input)
				p.input = string(runes[:len(runes)-1])
				p.selected = 0
				p.refilter()
			}

		case tcell.KeyRune:
			p.input += string(event.Rune())
			p.selected = 0
			p.refilter()
		}
	})
}
