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
	lines []string
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
	p.SetBorder(false)
	p.SetTitle(" Output ")
	p.SetTitleColor(ui.ColorTextWhite)
	p.SetBorderColor(ui.ColorBorder)

	return p
}

// AppendText adds text to the output panel
func (p *Panel) AppendText(text string) {
	newLines := strings.Split(text, "\n")
	p.lines = append(p.lines, newLines...)
	p.updateContent()
	p.ScrollToEnd()
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
}

func (p *Panel) updateContent() {
	p.SetText(strings.Join(p.lines, "\n"))
}

// Draw override for styling
func (p *Panel) Draw(screen tcell.Screen) {
	p.TextView.Draw(screen)
}
