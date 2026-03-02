package output

import (
	"fmt"
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
}

func New() *Panel {
	p := &Panel{
		TextView: tview.NewTextView(),
		lines:    []string{},
	}

	p.SetBackgroundColor(ui.ColorOutputBg)
	p.SetTextColor(ui.ColorTextWhite)
	p.SetDynamicColors(true)
	p.SetScrollable(true)
	p.SetBorder(true)
	p.SetTitle(" Output ")
	p.SetTitleColor(ui.ColorPanelBlurred)
	p.SetBorderColor(ui.ColorBorder)

	return p
}

// SetOnChange sets a callback when content changes (true = has content, false = empty)
func (p *Panel) SetOnChange(fn func(hasContent bool)) {
	p.onChange = fn
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

// Draw override for styling
func (p *Panel) Draw(screen tcell.Screen) {
	p.TextView.Draw(screen)
}
