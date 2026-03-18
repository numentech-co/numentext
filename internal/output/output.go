package output

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Panel is the output/terminal panel for build/run results
type Panel struct {
	*tview.TextView
	lines    []string
	onChange func(hasContent bool)

	// Mouse drag selection state
	hasFocus      bool
	dragging      bool
	selStart      [2]int // row, col (-1 = no selection)
	selEnd        [2]int
	hasSelection  bool
	anchorRow     int
	anchorCol     int

	// Status message callback
	onStatus func(msg string)
}

func New() *Panel {
	p := &Panel{
		TextView: tview.NewTextView(),
		lines:    []string{},
		selStart: [2]int{-1, -1},
		selEnd:   [2]int{-1, -1},
	}

	p.SetBackgroundColor(ui.ColorOutputBg)
	p.SetTextColor(ui.ColorTextPrimary)
	p.SetDynamicColors(true)
	p.SetScrollable(true)
	p.SetBorder(true)
	p.SetTitle(" Output ")
	p.SetTitleColor(ui.ColorPanelBlurred)
	p.SetBorderColor(ui.ColorBorder)

	return p
}

// SetOnChange sets a callback when content changes (true = has content, false = empty)
// RefreshColors re-applies theme colors.
func (p *Panel) RefreshColors() {
	p.SetBackgroundColor(ui.ColorOutputBg)
	p.SetTextColor(ui.ColorTextPrimary)
	p.SetTitleColor(ui.ColorPanelBlurred)
	p.SetBorderColor(ui.ColorBorder)
}

func (p *Panel) SetOnChange(fn func(hasContent bool)) {
	p.onChange = fn
}

// SetOnStatus sets a callback for status messages.
func (p *Panel) SetOnStatus(fn func(msg string)) {
	p.onStatus = fn
}

func (p *Panel) statusMsg(msg string) {
	if p.onStatus != nil {
		p.onStatus(msg)
	}
}

func (p *Panel) notifyChange() {
	if p.onChange != nil {
		p.onChange(len(p.lines) > 0)
	}
}

// AppendText adds text to the output panel
func (p *Panel) AppendText(text string) {
	newLines := strings.Split(text, "\n")
	p.lines = append(p.lines, newLines...)
	p.updateContent()
	p.ScrollToEnd()
	p.notifyChange()
}

// AppendCommand shows a command being run
func (p *Panel) AppendCommand(cmd string) {
	p.AppendText(fmt.Sprintf("[#00ffff]> %s[-]", cmd))
}

// AppendError adds error text
func (p *Panel) AppendError(text string) {
	p.AppendText(fmt.Sprintf("[red]%s[-]", text))
}

// AppendSuccess adds success text
func (p *Panel) AppendSuccess(text string) {
	p.AppendText(fmt.Sprintf("[green]%s[-]", text))
}

// Clear clears all output
func (p *Panel) Clear() {
	p.lines = []string{}
	p.updateContent()
	p.notifyChange()
}

// Lines returns the current output lines
func (p *Panel) Lines() []string {
	return p.lines
}

func (p *Panel) updateContent() {
	p.SetText(strings.Join(p.lines, "\n"))
}

// stripColorTags removes tview color tags from a string to get plain text.
var colorTagRe = regexp.MustCompile(`\[([a-zA-Z#0-9:-]*)\]`)

func stripColorTags(s string) string {
	return colorTagRe.ReplaceAllString(s, "")
}

// plainLines returns lines with color tags stripped.
func (p *Panel) plainLines() []string {
	result := make([]string, len(p.lines))
	for i, line := range p.lines {
		result[i] = stripColorTags(line)
	}
	return result
}

// HasSelection returns true if the output panel has selected text.
func (p *Panel) HasSelection() bool {
	return p.hasSelection
}

// SelectedText returns the selected text, or empty string if none.
func (p *Panel) SelectedText() string {
	if !p.hasSelection {
		return ""
	}
	plain := p.plainLines()
	sr, sc, er, ec := p.orderedSelection()
	var sb strings.Builder
	for r := sr; r <= er && r < len(plain); r++ {
		line := plain[r]
		startCol := 0
		endCol := len(line)
		if r == sr {
			startCol = sc
		}
		if r == er {
			endCol = ec
		}
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}
		if startCol <= endCol {
			sb.WriteString(line[startCol:endCol])
		}
		if r < er {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// ClearSelection clears any selection in the output panel.
func (p *Panel) ClearSelection() {
	p.hasSelection = false
	p.selStart = [2]int{-1, -1}
	p.selEnd = [2]int{-1, -1}
}

// CopySelection copies the selected text to clipboard.
func (p *Panel) CopySelection() {
	text := p.SelectedText()
	if text == "" {
		return
	}
	clipboardCopy(text)
	p.statusMsg("Copied to clipboard")
}

func clipboardCopy(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return
	}
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

// orderedSelection returns selection bounds ordered so start <= end.
func (p *Panel) orderedSelection() (int, int, int, int) {
	sr, sc := p.selStart[0], p.selStart[1]
	er, ec := p.selEnd[0], p.selEnd[1]
	if sr > er || (sr == er && sc > ec) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	return sr, sc, er, ec
}

// Draw override for styling -- adds selection highlighting
func (p *Panel) Draw(screen tcell.Screen) {
	p.TextView.Draw(screen)

	// Overlay selection highlighting
	if !p.hasSelection {
		return
	}

	x, y, width, height := p.GetInnerRect()
	sr, sc, er, ec := p.orderedSelection()
	plain := p.plainLines()

	// Get the current scroll offset from the TextView.
	// tview.TextView GetScrollOffset returns (row, col).
	scrollRow, scrollCol := p.GetScrollOffset()

	selStyle := tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)

	for screenRow := 0; screenRow < height; screenRow++ {
		lineIdx := scrollRow + screenRow
		if lineIdx < 0 || lineIdx >= len(plain) {
			continue
		}
		if lineIdx < sr || lineIdx > er {
			continue
		}
		line := plain[lineIdx]
		lineStart := 0
		lineEnd := len(line)
		if lineIdx == sr {
			lineStart = sc
		}
		if lineIdx == er {
			lineEnd = ec
		}
		for col := lineStart; col < lineEnd; col++ {
			screenCol := col - scrollCol
			if screenCol < 0 || screenCol >= width {
				continue
			}
			ch := ' '
			if col < len(line) {
				ch = rune(line[col])
			}
			screen.SetContent(x+screenCol, y+screenRow, ch, nil, selStyle)
		}
	}
}

// Focus marks this panel as focused
func (p *Panel) Focus(delegate func(tview.Primitive)) {
	p.hasFocus = true
	p.TextView.Focus(delegate)
}

// Blur marks this panel as unfocused
func (p *Panel) Blur() {
	p.hasFocus = false
	p.TextView.Blur()
}

// InputHandler wraps the default handler to add Ctrl+C copy support.
func (p *Panel) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		// Ctrl+C with selection: copy
		if event.Key() == tcell.KeyCtrlC && p.hasSelection {
			p.CopySelection()
			return
		}
		// Any other key clears selection
		if p.hasSelection {
			p.ClearSelection()
		}
		// Delegate to the default TextView handler
		defHandler := p.TextView.InputHandler()
		if defHandler != nil {
			defHandler(event, setFocus)
		}
	})
}

// MouseHandler handles mouse events for drag selection
func (p *Panel) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
	return p.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if !p.InRect(event.Position()) {
			return false, nil
		}

		mx, my := event.Position()
		bx, by, _, _ := p.GetInnerRect()
		scrollRow, scrollCol := p.GetScrollOffset()

		// Convert screen coords to text coords
		textRow := scrollRow + (my - by)
		textCol := scrollCol + (mx - bx)

		switch action {
		case tview.MouseLeftDown:
			setFocus(p)
			plain := p.plainLines()
			if textRow >= 0 && textRow < len(plain) {
				if textCol < 0 {
					textCol = 0
				}
				lineLen := len(plain[textRow])
				if textCol > lineLen {
					textCol = lineLen
				}
				p.dragging = true
				p.anchorRow = textRow
				p.anchorCol = textCol
				p.ClearSelection()
			}
			return true, p

		case tview.MouseMove:
			if p.dragging {
				plain := p.plainLines()
				if textRow < 0 {
					textRow = 0
				}
				if textRow >= len(plain) {
					textRow = len(plain) - 1
				}
				if textCol < 0 {
					textCol = 0
				}
				if textRow >= 0 && textRow < len(plain) {
					lineLen := len(plain[textRow])
					if textCol > lineLen {
						textCol = lineLen
					}
				}
				p.hasSelection = true
				p.selStart = [2]int{p.anchorRow, p.anchorCol}
				p.selEnd = [2]int{textRow, textCol}
				return true, p
			}
			return false, nil

		case tview.MouseLeftUp:
			if p.dragging {
				p.dragging = false
				// If start equals end, clear selection (just a click)
				if p.hasSelection && p.selStart[0] == p.selEnd[0] && p.selStart[1] == p.selEnd[1] {
					p.ClearSelection()
				}
				return true, nil
			}
			return false, nil

		case tview.MouseLeftClick:
			setFocus(p)
			p.ClearSelection()
			return true, nil

		case tview.MouseScrollUp:
			defHandler := p.TextView.MouseHandler()
			if defHandler != nil {
				return defHandler(action, event, setFocus)
			}
			return true, nil

		case tview.MouseScrollDown:
			defHandler := p.TextView.MouseHandler()
			if defHandler != nil {
				return defHandler(action, event, setFocus)
			}
			return true, nil
		}

		return false, nil
	})
}
