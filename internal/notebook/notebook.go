package notebook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Mode describes whether the user is navigating between cells or editing within one.
type Mode int

const (
	ModeNavigate Mode = iota
	ModeEdit
)

// NotebookView is a tview.Box-based widget for viewing and editing Jupyter notebooks.
type NotebookView struct {
	*tview.Box
	notebook    *Notebook
	filePath    string
	executor    *Executor
	modified    bool
	hasFocus    bool

	// Navigation state
	mode        Mode
	currentCell int // index of the selected cell
	scrollY     int // vertical scroll offset in screen lines

	// "dd" delete: track if 'd' was pressed once
	pendingD    bool

	// Image counter for temp file naming
	imageCount  int

	// Execution counter
	execCounter int

	// Callbacks
	onChange     func()
	onExecute   func(cellIdx int)
}

// New creates a new NotebookView from a file path and its raw data.
func New(filePath string, data []byte) (*NotebookView, error) {
	nb, err := ParseNotebook(data)
	if err != nil {
		return nil, err
	}

	workDir := filepath.Dir(filePath)
	executor := NewExecutor(workDir, nb.Language)

	nv := &NotebookView{
		Box:      tview.NewBox(),
		notebook: nb,
		filePath: filePath,
		executor: executor,
	}
	nv.SetBackgroundColor(ui.ColorBg)

	// Process any existing image outputs
	for _, cell := range nb.Cells {
		for i := range cell.Outputs {
			if cell.Outputs[i].ImageData != "" && cell.Outputs[i].ImagePath == "" {
				nv.imageCount++
				path, err := SaveImageOutput(cell.Outputs[i].ImageData, nv.imageCount)
				if err == nil {
					cell.Outputs[i].ImagePath = path
				}
			}
		}
		if cell.ExecCount > nv.execCounter {
			nv.execCounter = cell.ExecCount
		}
	}

	// Select first cell if any
	if len(nb.Cells) > 0 {
		nb.Cells[0].Selected = true
	}

	return nv, nil
}

// FilePath returns the file path of the notebook.
func (nv *NotebookView) FilePath() string {
	return nv.filePath
}

// Modified returns whether the notebook has been modified.
func (nv *NotebookView) Modified() bool {
	return nv.modified
}

// Language returns the notebook kernel language.
func (nv *NotebookView) Language() string {
	if nv.notebook != nil {
		return nv.notebook.Language
	}
	return ""
}

// CellCount returns the number of cells.
func (nv *NotebookView) CellCount() int {
	if nv.notebook == nil {
		return 0
	}
	return len(nv.notebook.Cells)
}

// CurrentCellIndex returns the index of the selected cell.
func (nv *NotebookView) CurrentCellIndex() int {
	return nv.currentCell
}

// CurrentCellType returns the type of the selected cell.
func (nv *NotebookView) CurrentCellType() string {
	if nv.notebook == nil || nv.currentCell >= len(nv.notebook.Cells) {
		return ""
	}
	return nv.notebook.Cells[nv.currentCell].CellType
}

// IsEditMode returns whether the view is in cell edit mode.
func (nv *NotebookView) IsEditMode() bool {
	return nv.mode == ModeEdit
}

// SetOnChange sets a callback for when the notebook changes.
func (nv *NotebookView) SetOnChange(fn func()) {
	nv.onChange = fn
}

// SetOnExecute sets a callback for when a cell is executed.
func (nv *NotebookView) SetOnExecute(fn func(cellIdx int)) {
	nv.onExecute = fn
}

// Save writes the notebook back to its file path in .ipynb format.
func (nv *NotebookView) Save() error {
	if nv.filePath == "" {
		return fmt.Errorf("no file path")
	}

	data, err := SerializeNotebook(nv.notebook)
	if err != nil {
		return fmt.Errorf("serialize: %w", err)
	}

	if err := os.WriteFile(nv.filePath, data, 0644); err != nil {
		return err
	}

	nv.modified = false
	if nv.onChange != nil {
		nv.onChange()
	}
	return nil
}

// Shutdown cleans up resources (e.g. Jupyter kernel).
func (nv *NotebookView) Shutdown() {
	if nv.executor != nil {
		nv.executor.ShutdownKernel()
	}
}

// cellHeight returns the number of screen lines a cell occupies, including borders.
func (nv *NotebookView) cellHeight(cell *Cell, width int) int {
	_ = width // might use for wrapping later
	// Top border: 1 line
	// Source lines
	// If code cell with outputs: separator line + output lines
	// Bottom border: 1 line
	h := 2 // top + bottom border
	srcLines := len(cell.Source)
	if srcLines == 0 {
		srcLines = 1 // empty cell shows at least one line
	}
	h += srcLines

	if cell.CellType == "code" && len(cell.Outputs) > 0 {
		h++ // separator line between source and output
		for _, out := range cell.Outputs {
			outLines := countLines(out.Text)
			if outLines == 0 {
				outLines = 1
			}
			if out.ImagePath != "" || out.ImageData != "" {
				outLines++ // "[Image: path]" line
			}
			if out.OutputType == "error" && len(out.Traceback) > 0 {
				outLines = len(out.Traceback)
			}
			h += outLines
		}
	}
	return h
}

func countLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

// totalHeight returns the total height of all cells in screen lines.
func (nv *NotebookView) totalHeight(width int) int {
	if nv.notebook == nil {
		return 0
	}
	h := 0
	for _, cell := range nv.notebook.Cells {
		h += nv.cellHeight(cell, width)
	}
	return h
}

// cellStartY returns the Y offset (in screen lines from top) where cell at idx starts.
func (nv *NotebookView) cellStartY(idx int, width int) int {
	y := 0
	for i := 0; i < idx && i < len(nv.notebook.Cells); i++ {
		y += nv.cellHeight(nv.notebook.Cells[i], width)
	}
	return y
}

// ensureCellVisible adjusts scrollY so the current cell is visible.
func (nv *NotebookView) ensureCellVisible() {
	_, _, w, h := nv.GetInnerRect()
	if h <= 0 || nv.notebook == nil || len(nv.notebook.Cells) == 0 {
		return
	}
	cellY := nv.cellStartY(nv.currentCell, w)
	cellH := nv.cellHeight(nv.notebook.Cells[nv.currentCell], w)

	if cellY < nv.scrollY {
		nv.scrollY = cellY
	}
	if cellY+cellH > nv.scrollY+h {
		nv.scrollY = cellY + cellH - h
	}
	if nv.scrollY < 0 {
		nv.scrollY = 0
	}
}

// Draw renders the notebook view to the screen.
func (nv *NotebookView) Draw(screen tcell.Screen) {
	nv.Box.DrawForSubclass(screen, nv)
	x, y, width, height := nv.GetInnerRect()

	if width < 10 || height < 3 || nv.notebook == nil {
		return
	}

	bgStyle := tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBg)

	// Clear the area
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
		}
	}

	// Render cells
	screenY := -nv.scrollY // relative to the widget's top
	for cellIdx, cell := range nv.notebook.Cells {
		ch := nv.cellHeight(cell, width)
		// Skip cells entirely above viewport
		if screenY+ch <= 0 {
			screenY += ch
			continue
		}
		// Stop if below viewport
		if screenY >= height {
			break
		}

		nv.drawCell(screen, x, y, width, height, screenY, cellIdx, cell)
		screenY += ch
	}
}

// drawCell renders a single cell.
func (nv *NotebookView) drawCell(screen tcell.Screen, x, y, width, height, cellScreenY, cellIdx int, cell *Cell) {
	isSelected := cellIdx == nv.currentCell

	// Style definitions
	borderStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBg)
	borderSelectedStyle := tcell.StyleDefault.Foreground(ui.ColorBorder).Background(ui.ColorBg)
	codeStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
	mdStyle := tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBg)
	rawStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBg)
	outputStyle := tcell.StyleDefault.Foreground(ui.ColorString).Background(ui.ColorBgDarker)
	errorStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorBgDarker)
	gutterStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)
	labelStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBg)
	editCursorStyle := tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)

	bStyle := borderStyle
	if isSelected {
		bStyle = borderSelectedStyle
	}

	lineY := 0 // line within cell (0 = top border)

	// Top border with cell type label
	if drawY := cellScreenY + lineY; drawY >= 0 && drawY < height {
		label := nv.cellLabel(cell)
		nv.drawHLine(screen, x, y+drawY, width, bStyle, label, labelStyle)
	}
	lineY++

	// Source lines
	srcLines := cell.Source
	if len(srcLines) == 0 {
		srcLines = []string{""}
	}

	for i, line := range srcLines {
		drawY := cellScreenY + lineY
		if drawY >= 0 && drawY < height {
			// Gutter (line number for code cells)
			gutterWidth := 0
			if cell.CellType == "code" {
				gutterWidth = 4
				numStr := fmt.Sprintf("%3d ", i+1)
				for gi, ch := range numStr {
					if x+gi < x+width {
						screen.SetContent(x+gi, y+drawY, ch, nil, gutterStyle)
					}
				}
			}

			// Content
			style := codeStyle
			switch cell.CellType {
			case "markdown":
				style = nv.markdownLineStyle(line, mdStyle)
			case "raw":
				style = rawStyle
			}

			contentX := x + gutterWidth
			for ci, ch := range line {
				if contentX+ci >= x+width {
					break
				}
				drawStyle := style
				// Edit mode cursor
				if isSelected && nv.mode == ModeEdit && i == cell.CursorRow && ci == cell.CursorCol {
					drawStyle = editCursorStyle
				}
				screen.SetContent(contentX+ci, y+drawY, ch, nil, drawStyle)
			}
			// Draw cursor at end of line in edit mode
			if isSelected && nv.mode == ModeEdit && i == cell.CursorRow && cell.CursorCol >= len(line) {
				cursorX := contentX + len(line)
				if cursorX < x+width {
					screen.SetContent(cursorX, y+drawY, ' ', nil, editCursorStyle)
				}
			}
		}
		lineY++
	}

	// Output section for code cells
	if cell.CellType == "code" && len(cell.Outputs) > 0 {
		// Separator
		if drawY := cellScreenY + lineY; drawY >= 0 && drawY < height {
			nv.drawHLine(screen, x, y+drawY, width, bStyle, "Output", labelStyle)
		}
		lineY++

		for _, out := range cell.Outputs {
			outStyle := outputStyle
			if out.OutputType == "error" {
				outStyle = errorStyle
			}

			var outLines []string
			if out.OutputType == "error" && len(out.Traceback) > 0 {
				outLines = out.Traceback
			} else if out.Text != "" {
				outLines = strings.Split(out.Text, "\n")
				if len(outLines) > 0 && outLines[len(outLines)-1] == "" {
					outLines = outLines[:len(outLines)-1]
				}
			}

			if len(outLines) == 0 && out.ImagePath == "" && out.ImageData == "" {
				outLines = []string{""}
			}

			for _, ol := range outLines {
				if drawY := cellScreenY + lineY; drawY >= 0 && drawY < height {
					for ci, ch := range ol {
						if x+ci >= x+width {
							break
						}
						screen.SetContent(x+ci, y+drawY, ch, nil, outStyle)
					}
					// Fill rest with output background
					for ci := len(ol); ci < width; ci++ {
						screen.SetContent(x+ci, y+drawY, ' ', nil, outStyle)
					}
				}
				lineY++
			}

			if out.ImagePath != "" || out.ImageData != "" {
				if drawY := cellScreenY + lineY; drawY >= 0 && drawY < height {
					imgLabel := "[Image: " + out.ImagePath + "]"
					if out.ImagePath == "" {
						imgLabel = "[Image: base64 PNG data]"
					}
					imgStyle := tcell.StyleDefault.Foreground(ui.ColorMarkdownLink).Background(ui.ColorBgDarker)
					for ci, ch := range imgLabel {
						if x+ci >= x+width {
							break
						}
						screen.SetContent(x+ci, y+drawY, ch, nil, imgStyle)
					}
				}
				lineY++
			}
		}
	}

	// Bottom border
	if drawY := cellScreenY + lineY; drawY >= 0 && drawY < height {
		nv.drawHLine(screen, x, y+drawY, width, bStyle, "", labelStyle)
	}
}

// cellLabel builds the label for a cell's top border.
func (nv *NotebookView) cellLabel(cell *Cell) string {
	label := strings.Title(cell.CellType)
	if cell.CellType == "code" && cell.ExecCount > 0 {
		label = fmt.Sprintf("Code [%d]", cell.ExecCount)
	} else if cell.CellType == "code" {
		label = "Code [ ]"
	}
	if cell.EditMode {
		label += " *EDIT*"
	}
	return label
}

// drawHLine draws a horizontal line with an optional label.
func (nv *NotebookView) drawHLine(screen tcell.Screen, x, y, width int, lineStyle tcell.Style, label string, labelStyle tcell.Style) {
	// Draw: +-- Label ---...---+
	if width < 2 {
		return
	}
	screen.SetContent(x, y, '+', nil, lineStyle)
	screen.SetContent(x+width-1, y, '+', nil, lineStyle)

	col := 1
	if label != "" {
		screen.SetContent(x+col, y, '-', nil, lineStyle)
		col++
		screen.SetContent(x+col, y, ' ', nil, lineStyle)
		col++
		for _, ch := range label {
			if x+col >= x+width-1 {
				break
			}
			screen.SetContent(x+col, y, ch, nil, labelStyle)
			col++
		}
		screen.SetContent(x+col, y, ' ', nil, lineStyle)
		col++
	}
	for col < width-1 {
		screen.SetContent(x+col, y, '-', nil, lineStyle)
		col++
	}
}

// markdownLineStyle returns a style for a markdown line based on its prefix.
func (nv *NotebookView) markdownLineStyle(line string, defaultStyle tcell.Style) tcell.Style {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "# "):
		return tcell.StyleDefault.Foreground(ui.ColorMarkdownH1).Background(ui.ColorBg).Bold(true)
	case strings.HasPrefix(trimmed, "## "):
		return tcell.StyleDefault.Foreground(ui.ColorMarkdownH2).Background(ui.ColorBg).Bold(true)
	case strings.HasPrefix(trimmed, "### "):
		return tcell.StyleDefault.Foreground(ui.ColorMarkdownH3).Background(ui.ColorBg).Bold(true)
	case strings.HasPrefix(trimmed, "#### "):
		return tcell.StyleDefault.Foreground(ui.ColorMarkdownH4).Background(ui.ColorBg)
	case strings.HasPrefix(trimmed, "```"):
		return tcell.StyleDefault.Foreground(ui.ColorMarkdownCode).Background(ui.ColorMarkdownCodeBg)
	case strings.HasPrefix(trimmed, "> "):
		return tcell.StyleDefault.Foreground(ui.ColorComment).Background(ui.ColorBg).Italic(true)
	case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
		return defaultStyle
	default:
		return defaultStyle
	}
}

// Focus handles focus events.
func (nv *NotebookView) Focus(delegate func(p tview.Primitive)) {
	nv.hasFocus = true
	nv.Box.Focus(delegate)
}

// Blur handles blur events.
func (nv *NotebookView) Blur() {
	nv.hasFocus = false
	nv.Box.Blur()
}

// InputHandler handles keyboard input.
func (nv *NotebookView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return nv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if nv.notebook == nil || len(nv.notebook.Cells) == 0 {
			return
		}

		if nv.mode == ModeEdit {
			nv.handleEditInput(event)
		} else {
			nv.handleNavigateInput(event)
		}
	})
}

// handleNavigateInput handles input in navigation mode.
func (nv *NotebookView) handleNavigateInput(event *tcell.EventKey) {
	key := event.Key()
	mod := event.Modifiers()
	shift := mod&tcell.ModShift != 0

	switch {
	case key == tcell.KeyUp, key == tcell.KeyRune && event.Rune() == 'k':
		nv.pendingD = false
		nv.moveToPrevCell()

	case key == tcell.KeyDown, key == tcell.KeyRune && event.Rune() == 'j':
		nv.pendingD = false
		nv.moveToNextCell()

	case key == tcell.KeyHome:
		nv.pendingD = false
		nv.selectCell(0)

	case key == tcell.KeyEnd:
		nv.pendingD = false
		if len(nv.notebook.Cells) > 0 {
			nv.selectCell(len(nv.notebook.Cells) - 1)
		}

	case key == tcell.KeyPgUp:
		nv.pendingD = false
		_, _, _, h := nv.GetInnerRect()
		nv.scrollY -= h
		if nv.scrollY < 0 {
			nv.scrollY = 0
		}

	case key == tcell.KeyPgDn:
		nv.pendingD = false
		_, _, w, h := nv.GetInnerRect()
		maxScroll := nv.totalHeight(w) - h
		if maxScroll < 0 {
			maxScroll = 0
		}
		nv.scrollY += h
		if nv.scrollY > maxScroll {
			nv.scrollY = maxScroll
		}

	case key == tcell.KeyEnter:
		nv.pendingD = false
		nv.enterEditMode()

	case key == tcell.KeyRune && event.Rune() == 'a':
		// Insert cell above
		nv.pendingD = false
		nv.insertCellAbove()

	case key == tcell.KeyRune && event.Rune() == 'b':
		// Insert cell below
		nv.pendingD = false
		nv.insertCellBelow()

	case key == tcell.KeyRune && event.Rune() == 'd':
		// "dd" to delete
		if nv.pendingD {
			nv.deleteCurrentCell()
			nv.pendingD = false
		} else {
			nv.pendingD = true
		}

	case key == tcell.KeyRune && event.Rune() == 'm':
		// Toggle code/markdown
		nv.pendingD = false
		nv.toggleCellType()

	case key == tcell.KeyRune && event.Rune() == 'y':
		// Change to code
		nv.pendingD = false
		nv.setCellType("code")

	case key == tcell.KeyRune && event.Rune() == 'r' && !shift:
		// Change to raw
		nv.pendingD = false
		nv.setCellType("raw")

	default:
		nv.pendingD = false
	}
}

// handleEditInput handles input in edit mode.
func (nv *NotebookView) handleEditInput(event *tcell.EventKey) {
	key := event.Key()
	cell := nv.notebook.Cells[nv.currentCell]

	switch key {
	case tcell.KeyEscape:
		nv.exitEditMode()

	case tcell.KeyUp:
		if cell.CursorRow > 0 {
			cell.CursorRow--
			if cell.CursorCol > len(cell.Source[cell.CursorRow]) {
				cell.CursorCol = len(cell.Source[cell.CursorRow])
			}
		}

	case tcell.KeyDown:
		if cell.CursorRow < len(cell.Source)-1 {
			cell.CursorRow++
			if cell.CursorCol > len(cell.Source[cell.CursorRow]) {
				cell.CursorCol = len(cell.Source[cell.CursorRow])
			}
		}

	case tcell.KeyLeft:
		if cell.CursorCol > 0 {
			cell.CursorCol--
		} else if cell.CursorRow > 0 {
			cell.CursorRow--
			cell.CursorCol = len(cell.Source[cell.CursorRow])
		}

	case tcell.KeyRight:
		if cell.CursorRow < len(cell.Source) && cell.CursorCol < len(cell.Source[cell.CursorRow]) {
			cell.CursorCol++
		} else if cell.CursorRow < len(cell.Source)-1 {
			cell.CursorRow++
			cell.CursorCol = 0
		}

	case tcell.KeyHome:
		cell.CursorCol = 0

	case tcell.KeyEnd:
		if cell.CursorRow < len(cell.Source) {
			cell.CursorCol = len(cell.Source[cell.CursorRow])
		}

	case tcell.KeyEnter:
		// Split line at cursor
		if cell.CursorRow < len(cell.Source) {
			line := cell.Source[cell.CursorRow]
			before := line[:cell.CursorCol]
			after := line[cell.CursorCol:]
			cell.Source[cell.CursorRow] = before
			// Insert new line after
			newSource := make([]string, len(cell.Source)+1)
			copy(newSource, cell.Source[:cell.CursorRow+1])
			newSource[cell.CursorRow+1] = after
			copy(newSource[cell.CursorRow+2:], cell.Source[cell.CursorRow+1:])
			cell.Source = newSource
			cell.CursorRow++
			cell.CursorCol = 0
			nv.markModified()
		}

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if cell.CursorCol > 0 {
			line := cell.Source[cell.CursorRow]
			cell.Source[cell.CursorRow] = line[:cell.CursorCol-1] + line[cell.CursorCol:]
			cell.CursorCol--
			nv.markModified()
		} else if cell.CursorRow > 0 {
			// Join with previous line
			prevLen := len(cell.Source[cell.CursorRow-1])
			cell.Source[cell.CursorRow-1] += cell.Source[cell.CursorRow]
			cell.Source = append(cell.Source[:cell.CursorRow], cell.Source[cell.CursorRow+1:]...)
			cell.CursorRow--
			cell.CursorCol = prevLen
			nv.markModified()
		}

	case tcell.KeyDelete:
		if cell.CursorRow < len(cell.Source) {
			line := cell.Source[cell.CursorRow]
			if cell.CursorCol < len(line) {
				cell.Source[cell.CursorRow] = line[:cell.CursorCol] + line[cell.CursorCol+1:]
				nv.markModified()
			} else if cell.CursorRow < len(cell.Source)-1 {
				// Join with next line
				cell.Source[cell.CursorRow] += cell.Source[cell.CursorRow+1]
				cell.Source = append(cell.Source[:cell.CursorRow+1], cell.Source[cell.CursorRow+2:]...)
				nv.markModified()
			}
		}

	case tcell.KeyTab:
		// Insert 4 spaces
		if cell.CursorRow < len(cell.Source) {
			line := cell.Source[cell.CursorRow]
			cell.Source[cell.CursorRow] = line[:cell.CursorCol] + "    " + line[cell.CursorCol:]
			cell.CursorCol += 4
			nv.markModified()
		}

	case tcell.KeyRune:
		// Insert character
		if cell.CursorRow < len(cell.Source) {
			line := cell.Source[cell.CursorRow]
			r := event.Rune()
			cell.Source[cell.CursorRow] = line[:cell.CursorCol] + string(r) + line[cell.CursorCol:]
			cell.CursorCol++
			nv.markModified()
		}
	}

	nv.ensureCellVisible()
}

// selectCell selects a cell by index.
func (nv *NotebookView) selectCell(idx int) {
	if idx < 0 || idx >= len(nv.notebook.Cells) {
		return
	}
	if nv.currentCell < len(nv.notebook.Cells) {
		nv.notebook.Cells[nv.currentCell].Selected = false
	}
	nv.currentCell = idx
	nv.notebook.Cells[idx].Selected = true
	nv.ensureCellVisible()
}

func (nv *NotebookView) moveToNextCell() {
	if nv.currentCell < len(nv.notebook.Cells)-1 {
		nv.selectCell(nv.currentCell + 1)
	}
}

func (nv *NotebookView) moveToPrevCell() {
	if nv.currentCell > 0 {
		nv.selectCell(nv.currentCell - 1)
	}
}

func (nv *NotebookView) enterEditMode() {
	cell := nv.notebook.Cells[nv.currentCell]
	nv.mode = ModeEdit
	cell.EditMode = true
	if len(cell.Source) == 0 {
		cell.Source = []string{""}
	}
	cell.CursorRow = 0
	cell.CursorCol = 0
}

func (nv *NotebookView) exitEditMode() {
	cell := nv.notebook.Cells[nv.currentCell]
	nv.mode = ModeNavigate
	cell.EditMode = false
}

func (nv *NotebookView) insertCellAbove() {
	newCell := &Cell{
		CellType: "code",
		Source:   []string{""},
	}
	cells := make([]*Cell, 0, len(nv.notebook.Cells)+1)
	cells = append(cells, nv.notebook.Cells[:nv.currentCell]...)
	cells = append(cells, newCell)
	cells = append(cells, nv.notebook.Cells[nv.currentCell:]...)
	nv.notebook.Cells = cells
	nv.selectCell(nv.currentCell) // stays on same index, which is now the new cell
	nv.markModified()
}

func (nv *NotebookView) insertCellBelow() {
	newCell := &Cell{
		CellType: "code",
		Source:   []string{""},
	}
	idx := nv.currentCell + 1
	cells := make([]*Cell, 0, len(nv.notebook.Cells)+1)
	cells = append(cells, nv.notebook.Cells[:idx]...)
	cells = append(cells, newCell)
	cells = append(cells, nv.notebook.Cells[idx:]...)
	nv.notebook.Cells = cells
	nv.selectCell(idx)
	nv.markModified()
}

func (nv *NotebookView) deleteCurrentCell() {
	if len(nv.notebook.Cells) <= 1 {
		// Don't delete the last cell; clear it instead
		nv.notebook.Cells[0].Source = []string{""}
		nv.notebook.Cells[0].Outputs = nil
		nv.markModified()
		return
	}
	nv.notebook.Cells = append(nv.notebook.Cells[:nv.currentCell], nv.notebook.Cells[nv.currentCell+1:]...)
	if nv.currentCell >= len(nv.notebook.Cells) {
		nv.currentCell = len(nv.notebook.Cells) - 1
	}
	nv.notebook.Cells[nv.currentCell].Selected = true
	nv.markModified()
	nv.ensureCellVisible()
}

func (nv *NotebookView) toggleCellType() {
	cell := nv.notebook.Cells[nv.currentCell]
	if cell.CellType == "code" {
		cell.CellType = "markdown"
		cell.Outputs = nil // markdown cells don't have outputs
	} else {
		cell.CellType = "code"
	}
	nv.markModified()
}

func (nv *NotebookView) setCellType(ct string) {
	cell := nv.notebook.Cells[nv.currentCell]
	if cell.CellType == ct {
		return
	}
	cell.CellType = ct
	if ct != "code" {
		cell.Outputs = nil
	}
	nv.markModified()
}

func (nv *NotebookView) markModified() {
	nv.modified = true
	if nv.onChange != nil {
		nv.onChange()
	}
}

// ExecuteCurrentCell executes the current cell and captures output.
func (nv *NotebookView) ExecuteCurrentCell() error {
	if nv.notebook == nil || nv.currentCell >= len(nv.notebook.Cells) {
		return nil
	}
	cell := nv.notebook.Cells[nv.currentCell]
	if cell.CellType != "code" {
		return nil
	}

	code := strings.Join(cell.Source, "\n")
	if strings.TrimSpace(code) == "" {
		return nil
	}

	out, err := nv.executor.ExecuteCell(code)
	if err != nil {
		return err
	}

	nv.execCounter++
	cell.ExecCount = nv.execCounter
	cell.Outputs = []Output{*out}

	// Handle image output
	if out.ImageData != "" {
		nv.imageCount++
		path, err := SaveImageOutput(out.ImageData, nv.imageCount)
		if err == nil {
			cell.Outputs[0].ImagePath = path
		}
	}

	nv.markModified()

	if nv.onExecute != nil {
		nv.onExecute(nv.currentCell)
	}

	return nil
}

// ExecuteCurrentAndAdvance executes the current cell and moves to the next.
func (nv *NotebookView) ExecuteCurrentAndAdvance() error {
	if err := nv.ExecuteCurrentCell(); err != nil {
		return err
	}
	nv.moveToNextCell()
	return nil
}

// ExecuteAllCells executes all code cells in order.
func (nv *NotebookView) ExecuteAllCells() error {
	if nv.notebook == nil {
		return nil
	}
	for i := range nv.notebook.Cells {
		nv.selectCell(i)
		if nv.notebook.Cells[i].CellType == "code" {
			if err := nv.ExecuteCurrentCell(); err != nil {
				return err
			}
		}
	}
	return nil
}

// RestartKernel shuts down and restarts the executor.
func (nv *NotebookView) RestartKernel() {
	nv.executor.ShutdownKernel()
	nv.execCounter = 0
	// Clear all execution counts and outputs
	for _, cell := range nv.notebook.Cells {
		cell.ExecCount = 0
		cell.Outputs = nil
	}
	nv.markModified()
}

// StatusText returns a status string for the status bar.
func (nv *NotebookView) StatusText() string {
	if nv.notebook == nil {
		return ""
	}
	modeStr := "NAV"
	if nv.mode == ModeEdit {
		modeStr = "EDIT"
	}
	cellInfo := ""
	if nv.currentCell < len(nv.notebook.Cells) {
		cell := nv.notebook.Cells[nv.currentCell]
		cellInfo = fmt.Sprintf("Cell %d/%d (%s)", nv.currentCell+1, len(nv.notebook.Cells), cell.CellType)
	}
	lang := nv.notebook.Language
	if lang == "" {
		lang = "unknown"
	}
	return fmt.Sprintf("%s | %s | %s", modeStr, cellInfo, lang)
}
