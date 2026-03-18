package diffview

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// DiffView is a side-by-side diff viewer widget that displays the committed
// version (left, read-only) and working copy (right, read-only) of a file.
// Both panes scroll in sync with filler lines for alignment.
type DiffView struct {
	*tview.Box
	leftLines  []DiffLine // committed version with diff info
	rightLines []DiffLine // working copy with diff info
	scrollRow  int        // synchronized scroll offset (row in the aligned view)
	focusPane  int        // 0=left, 1=right
	filePath   string     // path to the file being diffed
	hasFocus   bool

	// Callbacks
	onClose func() // called when Escape is pressed
}

// New creates a new DiffView for the given file path.
// It runs git show HEAD:<file> to get the committed version and reads the
// working copy from disk, then computes the aligned diff.
func New(filePath string) (*DiffView, error) {
	// Get committed version from git
	committedText, err := gitShowHead(filePath)
	if err != nil {
		return nil, fmt.Errorf("git show HEAD: %w", err)
	}

	// Read working copy from disk
	workingData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read working copy: %w", err)
	}
	workingText := string(workingData)

	return NewFromStrings(filePath, committedText, workingText), nil
}

// NewFromStrings creates a DiffView from two raw strings (for testing or
// when the content is already available).
func NewFromStrings(filePath, committedText, workingText string) *DiffView {
	leftLines := splitLines(committedText)
	rightLines := splitLines(workingText)

	result := ComputeDiff(leftLines, rightLines)

	dv := &DiffView{
		Box:        tview.NewBox(),
		leftLines:  result.Left,
		rightLines: result.Right,
		filePath:   filePath,
	}
	dv.SetBackgroundColor(ui.ColorBg)
	return dv
}

// splitLines splits text into lines, handling various line endings.
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// gitShowHead returns the committed version of a file from HEAD.
func gitShowHead(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(absPath)

	// Get the git repository root
	rootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	rootCmd.Dir = dir
	rootOut, err := rootCmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	repoRoot := strings.TrimSpace(string(rootOut))

	// Get the relative path from the repo root
	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", err
	}

	// git show HEAD:<relative-path>
	cmd := exec.Command("git", "show", "HEAD:"+relPath)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("file not in HEAD (new file?)")
	}

	return string(out), nil
}

// FilePath returns the path of the file being diffed.
func (dv *DiffView) FilePath() string {
	return dv.filePath
}

// SetOnClose sets the callback for when Escape is pressed.
func (dv *DiffView) SetOnClose(fn func()) {
	dv.onClose = fn
}

// TotalRows returns the total number of aligned rows.
func (dv *DiffView) TotalRows() int {
	return len(dv.leftLines)
}

// visibleRows returns how many content rows fit in the view area.
func (dv *DiffView) visibleRows() int {
	_, _, _, h := dv.GetInnerRect()
	if h < 2 {
		return 1
	}
	return h - 1 // reserve 1 line for header
}

// ensureScrollVisible adjusts scroll to keep a target row visible.
func (dv *DiffView) ensureScrollVisible(targetRow int) {
	visible := dv.visibleRows()
	if targetRow < dv.scrollRow {
		dv.scrollRow = targetRow
	}
	if targetRow >= dv.scrollRow+visible {
		dv.scrollRow = targetRow - visible + 1
	}
	dv.clampScroll()
}

// clampScroll ensures scrollRow stays in valid range.
func (dv *DiffView) clampScroll() {
	maxScroll := dv.TotalRows() - dv.visibleRows()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if dv.scrollRow > maxScroll {
		dv.scrollRow = maxScroll
	}
	if dv.scrollRow < 0 {
		dv.scrollRow = 0
	}
}

// Focus handles focus events.
func (dv *DiffView) Focus(delegate func(p tview.Primitive)) {
	dv.hasFocus = true
	dv.Box.Focus(delegate)
}

// Blur handles blur events.
func (dv *DiffView) Blur() {
	dv.hasFocus = false
	dv.Box.Blur()
}

// Draw renders the side-by-side diff view to the screen.
func (dv *DiffView) Draw(screen tcell.Screen) {
	dv.Box.DrawForSubclass(screen, dv)
	x, y, width, height := dv.GetInnerRect()

	if width < 20 || height < 2 {
		return
	}

	// Styles
	bgStyle := tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(ui.ColorBg)
	headerStyle := tcell.StyleDefault.Foreground(ui.ColorTextMuted).Background(ui.ColorGutterBg)
	gutterStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)
	normalStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
	addedStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.NewRGBColor(0, 170, 0))
	deletedStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.NewRGBColor(170, 0, 0))
	modifiedBgLeft := tcell.NewRGBColor(100, 0, 0)
	modifiedBgRight := tcell.NewRGBColor(0, 100, 0)
	modifiedStyleLeft := tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(modifiedBgLeft)
	modifiedStyleRight := tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(modifiedBgRight)
	wordDeletedStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(200, 0, 0)).Bold(true)
	wordAddedStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(0, 200, 0)).Bold(true)
	fillerStyle := tcell.StyleDefault.Foreground(ui.ColorTextMuted).Background(ui.ColorBgAlt)
	dividerStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)

	// Calculate pane widths: 50/50 split with 1-char divider in the middle
	halfWidth := (width - 1) / 2
	rightStartX := x + halfWidth + 1
	rightWidth := width - halfWidth - 1

	// Clear the entire area
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
		}
	}

	// Draw header line
	fileName := filepath.Base(dv.filePath)
	leftHeader := " HEAD: " + fileName
	rightHeader := " Working: " + fileName
	dv.drawHeaderText(screen, x, y, halfWidth, leftHeader, headerStyle)
	screen.SetContent(x+halfWidth, y, '|', nil, dividerStyle)
	dv.drawHeaderText(screen, rightStartX, y, rightWidth, rightHeader, headerStyle)

	// Gutter width: up to 5 digits + space
	gutterW := 5

	// Draw content rows
	visRows := dv.visibleRows()
	for row := 0; row < visRows; row++ {
		rowIdx := dv.scrollRow + row
		screenY := y + 1 + row
		if screenY >= y+height {
			break
		}

		// Draw divider
		screen.SetContent(x+halfWidth, screenY, '|', nil, dividerStyle)

		// Draw left pane
		if rowIdx < len(dv.leftLines) {
			dv.drawLine(screen, x, screenY, halfWidth, gutterW,
				dv.leftLines[rowIdx], true,
				gutterStyle, normalStyle, addedStyle, deletedStyle,
				modifiedStyleLeft, wordDeletedStyle, fillerStyle)
		}

		// Draw right pane
		if rowIdx < len(dv.rightLines) {
			dv.drawLine(screen, rightStartX, screenY, rightWidth, gutterW,
				dv.rightLines[rowIdx], false,
				gutterStyle, normalStyle, addedStyle, deletedStyle,
				modifiedStyleRight, wordAddedStyle, fillerStyle)
		}
	}
}

// drawHeaderText draws a header string clipped to the given width.
func (dv *DiffView) drawHeaderText(screen tcell.Screen, x, y, width int, text string, style tcell.Style) {
	for i := 0; i < width; i++ {
		ch := ' '
		if i < len(text) {
			ch = rune(text[i])
		}
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

// drawLine draws a single diff line (gutter + content) in one pane.
func (dv *DiffView) drawLine(screen tcell.Screen, x, y, width, gutterW int,
	line DiffLine, isLeft bool,
	gutterStyle, normalStyle, addedStyle, deletedStyle, modifiedStyle, wordStyle, fillerStyle tcell.Style) {

	// Determine base style for this line type
	var baseStyle tcell.Style
	switch line.Type {
	case DiffNormal:
		baseStyle = normalStyle
	case DiffAdded:
		baseStyle = addedStyle
	case DiffDeleted:
		baseStyle = deletedStyle
	case DiffModified:
		baseStyle = modifiedStyle
	case DiffFiller:
		baseStyle = fillerStyle
	}

	// Draw gutter (line number)
	gutterText := ""
	if line.Type != DiffFiller && line.LineNum > 0 {
		gutterText = fmt.Sprintf("%*d ", gutterW-1, line.LineNum)
	} else {
		gutterText = strings.Repeat(" ", gutterW)
	}
	for i := 0; i < gutterW && i < width; i++ {
		ch := ' '
		if i < len(gutterText) {
			ch = rune(gutterText[i])
		}
		screen.SetContent(x+i, y, ch, nil, gutterStyle)
	}

	// Draw content
	contentX := x + gutterW
	contentW := width - gutterW
	if contentW <= 0 {
		return
	}

	text := line.Text
	if line.Type == DiffFiller {
		// Fill with a subtle pattern
		for i := 0; i < contentW; i++ {
			screen.SetContent(contentX+i, y, ' ', nil, fillerStyle)
		}
		return
	}

	// Build a style map for word-level diffs
	wordDiffMap := make(map[int]bool)
	for _, wd := range line.WordDiffs {
		for j := wd.Start; j < wd.End; j++ {
			wordDiffMap[j] = true
		}
	}

	// Draw each character
	col := 0
	for byteIdx := 0; byteIdx < len(text) && col < contentW; byteIdx++ {
		ch := rune(text[byteIdx])
		if ch == '\t' {
			// Expand tab to spaces
			tabStop := 4 - (col % 4)
			for t := 0; t < tabStop && col < contentW; t++ {
				style := baseStyle
				if wordDiffMap[byteIdx] {
					style = wordStyle
				}
				screen.SetContent(contentX+col, y, ' ', nil, style)
				col++
			}
			continue
		}

		style := baseStyle
		if wordDiffMap[byteIdx] {
			style = wordStyle
		}
		screen.SetContent(contentX+col, y, ch, nil, style)
		col++
	}

	// Fill remaining space
	for col < contentW {
		screen.SetContent(contentX+col, y, ' ', nil, baseStyle)
		col++
	}
}

// InputHandler handles keyboard input for the diff view.
func (dv *DiffView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return dv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyEscape:
			if dv.onClose != nil {
				dv.onClose()
			}

		case tcell.KeyTab:
			// Switch focus between panes (visual indicator)
			if dv.focusPane == 0 {
				dv.focusPane = 1
			} else {
				dv.focusPane = 0
			}

		case tcell.KeyUp:
			if dv.scrollRow > 0 {
				dv.scrollRow--
			}

		case tcell.KeyDown:
			maxScroll := dv.TotalRows() - dv.visibleRows()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if dv.scrollRow < maxScroll {
				dv.scrollRow++
			}

		case tcell.KeyPgUp:
			dv.scrollRow -= dv.visibleRows()
			if dv.scrollRow < 0 {
				dv.scrollRow = 0
			}

		case tcell.KeyPgDn:
			dv.scrollRow += dv.visibleRows()
			dv.clampScroll()

		case tcell.KeyHome:
			dv.scrollRow = 0

		case tcell.KeyEnd:
			dv.scrollRow = dv.TotalRows() - dv.visibleRows()
			if dv.scrollRow < 0 {
				dv.scrollRow = 0
			}
		}
	})
}
