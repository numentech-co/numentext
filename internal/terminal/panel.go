package terminal

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Panel is a tview-compatible widget that renders a VT terminal
type Panel struct {
	*tview.Box
	term      *Terminal
	hasFocus  bool
	scrollOff int // lines scrolled back into scrollback (0 = live)

	// Multi-session tab bar
	tabNames  []string
	activeTab int

	// Block boxing mode
	boxMode       bool // when true, render finished blocks as boxes
	selectedBlock int  // index of the selected block (-1 = none, i.e. live area)

	// Callback for status messages (e.g. "Copied to clipboard")
	onStatus func(msg string)

	// Clickable regions for block header buttons
	clickRegions []clickRegion
}

// NewPanel creates a new terminal panel widget
func NewPanel() *Panel {
	p := &Panel{
		Box:           tview.NewBox(),
		boxMode:       true, // enabled by default
		selectedBlock: -1,
	}
	p.SetBackgroundColor(ui.ColorOutputBg)
	p.SetBorder(true)
	p.SetBorderColor(ui.ColorBorder)
	p.SetTitle(" Terminal ")
	p.SetTitleColor(ui.ColorPanelBlurred)
	return p
}

// SetTerminal attaches a Terminal to this panel
func (p *Panel) SetTerminal(t *Terminal) {
	p.term = t
	p.selectedBlock = -1
}

// Terminal returns the attached terminal
func (p *Panel) Terminal() *Terminal {
	return p.term
}

// SetTabs updates the tab bar labels and which tab is active.
// Pass nil or empty slice to hide the tab bar.
func (p *Panel) SetTabs(names []string, active int) {
	p.tabNames = names
	p.activeTab = active
}

// SetBoxMode enables or disables command boxing.
func (p *Panel) SetBoxMode(on bool) {
	p.boxMode = on
}

// BoxMode returns whether command boxing is enabled.
func (p *Panel) BoxMode() bool {
	return p.boxMode
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

// ScrollBy adjusts the scroll offset by delta lines (positive = scroll up into history).
func (p *Panel) ScrollBy(delta int) {
	p.scrollOff += delta
	if p.scrollOff < 0 {
		p.scrollOff = 0
	}
	if p.term != nil {
		p.term.Lock()
		maxScroll := len(p.term.VT().Scrollback())
		p.term.Unlock()
		if p.scrollOff > maxScroll {
			p.scrollOff = maxScroll
		}
	}
}

// drawTabBar renders a tab bar on the top row of the inner rect.
// Returns the number of rows consumed (0 or 1).
func (p *Panel) drawTabBar(screen tcell.Screen, x, y, width int) int {
	if len(p.tabNames) <= 1 {
		return 0
	}
	// Fill background
	bgStyle := tcell.StyleDefault.Background(ui.ColorDialogBg).Foreground(ui.ColorStatusText)
	for col := 0; col < width; col++ {
		screen.SetContent(x+col, y, ' ', nil, bgStyle)
	}
	col := 0
	for i, name := range p.tabNames {
		label := " " + name + " "
		var tabStyle tcell.Style
		if i == p.activeTab {
			tabStyle = tcell.StyleDefault.Background(ui.ColorPanelFocused).Foreground(tcell.ColorBlack).Bold(true)
		} else {
			tabStyle = tcell.StyleDefault.Background(ui.ColorDialogBg).Foreground(ui.ColorStatusText)
		}
		for _, ch := range label {
			if col >= width {
				break
			}
			screen.SetContent(x+col, y, ch, nil, tabStyle)
			col++
		}
		// Separator between tabs
		if i < len(p.tabNames)-1 && col < width {
			screen.SetContent(x+col, y, '|', nil, bgStyle)
			col++
		}
	}
	return 1
}

// drawString draws a string at the given position, truncating at maxCol.
// Returns the number of columns written.
func drawString(screen tcell.Screen, x, y, maxWidth int, s string, style tcell.Style) int {
	col := 0
	for _, ch := range s {
		if col >= maxWidth {
			break
		}
		screen.SetContent(x+col, y, ch, nil, style)
		col++
	}
	return col
}

// fillRow fills a row with spaces.
func fillRow(screen tcell.Screen, x, y, width int, style tcell.Style) {
	for col := 0; col < width; col++ {
		screen.SetContent(x+col, y, ' ', nil, style)
	}
}

// Draw renders the VT grid to the screen
func (p *Panel) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)
	x, y, width, height := p.GetInnerRect()

	// Draw tab bar (consumes top row if multiple sessions)
	tabRows := p.drawTabBar(screen, x, y, width)
	y += tabRows
	height -= tabRows

	if p.term == nil {
		// Draw empty panel with message
		msg := "Press Ctrl+` to open terminal"
		style := tcell.StyleDefault.Foreground(ui.ColorTextMuted).Background(ui.ColorOutputBg)
		for i, ch := range msg {
			if x+i < x+width {
				screen.SetContent(x+i, y, ch, nil, style)
			}
		}
		return
	}

	// Calculate effective VT height. When an active block is expanded,
	// its header (and any finished blocks above it) consume panel rows,
	// so the VT must be smaller to match the actual available space.
	// This ensures the running program's cursor position maps 1:1 to the
	// visible area (critical for TUI programs like Claude Code).
	p.term.Lock()
	vt := p.term.VT()
	bt := vt.Blocks()
	vtHeight := height
	useBoxed := p.boxMode && bt.BlockCount() > 0 && !bt.AltScreen()
	if useBoxed {
		activeBlock := bt.ActiveBlock()
		if activeBlock != nil && !activeBlock.Collapsed {
			// When an active block is running, finished blocks are rendered
			// as collapsed (1-row header only) to maximize space for the
			// active program. Count overhead accordingly.
			overhead := 1 // active block header
			for _, blk := range bt.Blocks {
				if blk == activeBlock {
					break
				}
				overhead++ // collapsed header = 1 row each
			}
			vtHeight = height - overhead
			if vtHeight < 4 {
				vtHeight = 4
			}
		}
	}
	needsResize := vt.Cols() != width || vt.Rows() != vtHeight
	p.term.Unlock()

	if needsResize && width > 0 && vtHeight > 0 {
		p.term.Resize(width, vtHeight)
	}

	// Always reset click regions at the start of each draw
	p.clickRegions = p.clickRegions[:0]

	p.term.Lock()
	vt = p.term.VT()
	bt = vt.Blocks()
	useBoxed = p.boxMode && bt.BlockCount() > 0 && !bt.AltScreen()

	if useBoxed {
		p.drawBoxed(screen, x, y, width, height, vt, bt)
	} else {
		p.drawRaw(screen, x, y, width, height, vt)
	}

	p.term.Unlock()

	// Reset dirty flag so next PTY read can trigger a new redraw
	p.term.MarkClean()
}

// drawBoxed renders all command blocks as boxes, with a prompt line at the bottom.
// Each command (finished or active) gets its own box. The view auto-scrolls to
// keep the latest block and prompt visible. Blocks alternate background colors
// (both header and output use the same alternating color per block).
//
// For the active (unfinished) block, output is rendered directly from VT cells
// rather than tracked Output lines. This correctly handles TUI programs (like
// Claude Code) that use cursor positioning to render their display.
func (p *Panel) drawBoxed(screen tcell.Screen, x, y, width, height int, vt *VT, bt *BlockTracker) {
	bgStyle := tcell.StyleDefault.Background(ui.ColorOutputBg).Foreground(ui.ColorTextPrimary)
	selectedHeaderStyle := tcell.StyleDefault.Background(ui.ColorPanelFocused).Foreground(tcell.ColorBlack).Bold(true)
	collapsedHint := tcell.StyleDefault.Background(ui.ColorOutputBg).Foreground(ui.ColorTextMuted)

	// Alternating styles: even blocks and odd blocks get different backgrounds
	// Both header and output use the same bg per block
	headerStyleEven := tcell.StyleDefault.Background(ui.ColorBgAlt).Foreground(tcell.ColorWhite).Bold(true)
	headerStyleOdd := tcell.StyleDefault.Background(ui.ColorOutputBgStripe).Foreground(tcell.ColorWhite).Bold(true)
	outputStyleEven := tcell.StyleDefault.Background(ui.ColorOutputBg).Foreground(ui.ColorTextPrimary)
	outputStyleOdd := tcell.StyleDefault.Background(ui.ColorOutputBgStripe).Foreground(ui.ColorTextPrimary)

	// Check for active (unfinished, expanded) block — it renders VT cells
	activeBlock := bt.ActiveBlock()
	activeExpanded := activeBlock != nil && !activeBlock.Collapsed

	// Reserve 1 row for the prompt line (only when no active expanded block,
	// since the active block's VT cells include the prompt/cursor area).
	// Safety: maxBlockArea is always >= 1, preventing overflow when drawing
	// block headers below. The drawBlock call at the bottom of this function
	// passes maxBlockArea-row as the available height, which is always > 0
	// because the loop guard checks row < maxBlockArea.
	maxBlockArea := height
	if !activeExpanded {
		maxBlockArea = height - 1
		if maxBlockArea < 1 {
			maxBlockArea = 1
		}
	}

	// Calculate total height of all blocks.
	// When an active block is expanded, finished blocks above it are
	// auto-collapsed (shown as 1-row headers) to maximize TUI program space.
	totalRows := 0
	for i, blk := range bt.Blocks {
		if blk == activeBlock && !blk.Collapsed {
			totalRows++ // just the header row; VT cells fill remaining space
		} else if activeExpanded && blk != activeBlock {
			totalRows++ // auto-collapsed: 1 row header only
		} else {
			totalRows += p.blockHeight(blk, width, i == p.selectedBlock)
		}
	}

	// Clamp scrollOff: can't scroll past all content
	if p.scrollOff > totalRows {
		p.scrollOff = totalRows
	}

	// Calculate how many rows to skip from the top.
	skipRows := 0
	if totalRows > maxBlockArea {
		skipRows = totalRows - maxBlockArea
	}
	skipRows -= p.scrollOff
	if skipRows < 0 {
		skipRows = 0
	}

	// Render blocks top-down with skipRows offset
	row := 0
	skipped := 0
	for i, blk := range bt.Blocks {
		if row >= maxBlockArea {
			break
		}
		isSelected := i == p.selectedBlock
		isActiveExpanded := blk == activeBlock && !blk.Collapsed

		if isActiveExpanded {
			// Active expanded block: draw header, then VT cells for remaining space
			bh := 1 // header only for skip calculation
			if skipped+bh <= skipRows {
				skipped += bh
				continue
			}

			lineSkip := 0
			if skipped < skipRows {
				lineSkip = skipRows - skipped
			}

			// Draw header (if not skipped)
			if lineSkip == 0 && row < maxBlockArea {
				hStyle := headerStyleEven
				if i%2 == 1 {
					hStyle = headerStyleOdd
				}
				if isSelected {
					hStyle = selectedHeaderStyle
				}
				fillRow(screen, x, y+row, width, hStyle)

				toggle := "[-]"
				col := drawString(screen, x, y+row, width, toggle, hStyle)
				cmdText := " $ " + blk.Command
				col += drawString(screen, x+col, y+row, width-col, cmdText, hStyle)

				copyLabel := "[copy]"
				copyX := -1
				if col+1+len(copyLabel) < width {
					copyX = x + width - len(copyLabel)
					drawString(screen, copyX, y+row, len(copyLabel), copyLabel, hStyle)
				}

				if copyX >= 0 {
					p.clickRegions = append(p.clickRegions, clickRegion{x: copyX, y: y + row, w: len(copyLabel), blockIdx: i, action: "copy"})
					p.clickRegions = append(p.clickRegions, clickRegion{x: x, y: y + row, w: copyX - x, blockIdx: i, action: "toggle"})
				} else {
					p.clickRegions = append(p.clickRegions, clickRegion{x: x, y: y + row, w: width, blockIdx: i, action: "toggle"})
				}
				row++
			}

			// Render VT cells in all remaining space.
			// The VT has been resized to match this area, so all rows fit.
			remainingRows := height - row
			vtStartRow := 0
			if vt.Rows() > remainingRows {
				vtStartRow = vt.Rows() - remainingRows
			}
			curRow := vt.CursorRow()
			curCol := vt.CursorCol()
			// Always show cursor for the active block — the user needs to see
			// where they're typing. Programs like Claude Code hide the cursor
			// during redraws, but we should always show it in the embedded terminal.
			showCursor := p.hasFocus && p.term.RunningNoLock()

			for vtR := vtStartRow; vtR < vt.Rows() && row < height; vtR++ {
				for col := 0; col < width; col++ {
					if col < vt.Cols() {
						cell := vt.Cell(vtR, col)
						style := tcell.StyleDefault.Foreground(cell.Fg).Background(cell.Bg)
						if cell.Bold {
							style = style.Bold(true)
						}
						ch := cell.Ch
						if ch == 0 {
							ch = ' '
						}
						screen.SetContent(x+col, y+row, ch, nil, style)
					} else {
						screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
					}
				}

				// Draw cursor on the cursor row (only if VT cursor is visible)
				if showCursor && vtR == curRow {
					if curCol >= 0 && curCol < width {
						cell := vt.Cell(curRow, curCol)
						style := tcell.StyleDefault.
							Foreground(cell.Bg).
							Background(cell.Fg).
							Reverse(true)
						ch := cell.Ch
						if ch == 0 || ch == ' ' {
							ch = ' '
						}
						screen.SetContent(x+curCol, y+row, ch, nil, style)
					}
				}
				row++
			}
			// Active block consumed remaining space
			break

		} else {
			// Finished block or collapsed active — render from tracked Output.
			// When an active block is expanded, force-collapse finished blocks
			// to maximize space for the running TUI program.
			renderBlk := blk
			forceCollapsed := activeExpanded && blk != activeBlock && !blk.Collapsed
			if forceCollapsed {
				// Temporarily treat as collapsed for height/rendering
				renderBlk = &CommandBlock{
					Command:   blk.Command,
					Output:    blk.Output,
					Collapsed: true,
					Finished:  blk.Finished,
				}
			}
			bh := p.blockHeight(renderBlk, width, isSelected)

			if skipped+bh <= skipRows {
				skipped += bh
				continue
			}

			lineSkip := 0
			if skipped < skipRows {
				lineSkip = skipRows - skipped
				skipped = skipRows
			}

			hStyle := headerStyleEven
			outStyle := outputStyleEven
			cHint := collapsedHint
			if i%2 == 1 {
				hStyle = headerStyleOdd
				outStyle = outputStyleOdd
				cHint = tcell.StyleDefault.Background(ui.ColorOutputBgStripe).Foreground(ui.ColorTextMuted)
			}

			row += p.drawBlock(screen, x, y+row, width, maxBlockArea-row, renderBlk, i, isSelected, lineSkip,
				hStyle, selectedHeaderStyle, outStyle, cHint, bgStyle)
		}
	}

	// Draw prompt line (only when no active expanded block)
	if !activeExpanded {
		curRow := vt.CursorRow()
		if p.scrollOff == 0 && row < height && curRow >= 0 && curRow < vt.Rows() {
			for col := 0; col < width; col++ {
				if col < vt.Cols() {
					cell := vt.Cell(curRow, col)
					style := tcell.StyleDefault.Foreground(cell.Fg).Background(cell.Bg)
					if cell.Bold {
						style = style.Bold(true)
					}
					ch := cell.Ch
					if ch == 0 {
						ch = ' '
					}
					screen.SetContent(x+col, y+row, ch, nil, style)
				} else {
					screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
				}
			}

			// Draw cursor on the prompt line
			if p.hasFocus && p.term.RunningNoLock() && vt.CursorVisible() {
				curCol := vt.CursorCol()
				if curCol >= 0 && curCol < width {
					cell := vt.Cell(curRow, curCol)
					style := tcell.StyleDefault.
						Foreground(cell.Bg).
						Background(cell.Fg).
						Reverse(true)
					ch := cell.Ch
					if ch == 0 || ch == ' ' {
						ch = ' '
					}
					screen.SetContent(x+curCol, y+row, ch, nil, style)
				}
			}
			row++
		}
	}

	// Fill any remaining rows
	for row < height {
		fillRow(screen, x, y+row, width, bgStyle)
		row++
	}
}

// blockHeight returns the number of rows a block occupies.
func (p *Panel) blockHeight(blk *CommandBlock, width int, isSelected bool) int {
	if blk.Collapsed {
		return 1 // just the header
	}
	// Header + output lines + bottom border
	lines := 1 // header
	lines += len(blk.Output)
	if len(blk.Output) == 0 {
		lines++ // at least one empty output line
	}
	return lines
}

// clickRegion stores the screen position and block index for a clickable element.
type clickRegion struct {
	x, y, w int
	blockIdx int
	action   string // "toggle" or "copy"
}

// drawBlock renders a single command block. Returns the number of rows consumed.
func (p *Panel) drawBlock(screen tcell.Screen, x, y, width, maxRows int,
	blk *CommandBlock, blockIdx int, isSelected bool, lineSkip int,
	headerStyle, selectedHeaderStyle, outputStyle, collapsedHint, bgStyle tcell.Style) int {

	if maxRows <= 0 {
		return 0
	}

	row := 0

	// Header line always renders — never skip it. This prevents the top
	// visible block from losing its command header when content overflows.
	if row >= maxRows {
		return row
	}
	hStyle := headerStyle
	if isSelected {
		hStyle = selectedHeaderStyle
	}
	fillRow(screen, x, y+row, width, hStyle)

	toggle := "[+]"
	if !blk.Collapsed {
		toggle = "[-]"
	}

	// Draw toggle button
	col := drawString(screen, x, y+row, width, toggle, hStyle)

	// Draw command
	cmdText := " $ " + blk.Command
	col += drawString(screen, x+col, y+row, width-col, cmdText, hStyle)

	// Draw [copy] button at right edge
	copyLabel := "[copy]"
	copyX := -1
	if col+1+len(copyLabel) < width {
		copyX = x + width - len(copyLabel)
		drawString(screen, copyX, y+row, len(copyLabel), copyLabel, hStyle)
	}

	// Register click regions: [copy] at right, toggle for the rest of the header row
	if copyX >= 0 {
		p.clickRegions = append(p.clickRegions, clickRegion{x: copyX, y: y + row, w: len(copyLabel), blockIdx: blockIdx, action: "copy"})
		p.clickRegions = append(p.clickRegions, clickRegion{x: x, y: y + row, w: copyX - x, blockIdx: blockIdx, action: "toggle"})
	} else {
		p.clickRegions = append(p.clickRegions, clickRegion{x: x, y: y + row, w: width, blockIdx: blockIdx, action: "toggle"})
	}

	row++

	if blk.Collapsed {
		return row
	}

	// Output lines — skip output rows (not the header) when lineSkip > 0.
	// lineSkip counts from the block start (header=0), so subtract 1 for the
	// header we already rendered to get the number of output lines to skip.
	outputSkip := lineSkip
	if outputSkip > 0 {
		outputSkip-- // header was counted as line 0 but already rendered
	}
	drawnLines := 0

	if len(blk.Output) == 0 {
		if drawnLines >= outputSkip {
			if row >= maxRows {
				return row
			}
			fillRow(screen, x, y+row, width, outputStyle)
			drawString(screen, x+1, y+row, width-1, "(no output)", collapsedHint)
			row++
		}
	} else {
		for _, line := range blk.Output {
			if drawnLines >= outputSkip {
				if row >= maxRows {
					return row
				}
				fillRow(screen, x, y+row, width, outputStyle)
				drawString(screen, x+1, y+row, width-1, line, outputStyle)
				row++
			}
			drawnLines++
		}
	}

	return row
}

// drawRaw renders the traditional VT grid (scrollback + live).
func (p *Panel) drawRaw(screen tcell.Screen, x, y, width, height int, vt *VT) {
	bgStyle := tcell.StyleDefault.Background(ui.ColorOutputBg)
	scrollback := vt.Scrollback()
	scrollOff := p.scrollOff
	if scrollOff > len(scrollback) {
		scrollOff = len(scrollback)
	}

	screenRow := 0

	if scrollOff > 0 {
		sbStart := len(scrollback) - scrollOff

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
					screen.SetContent(x+col, y+screenRow, ' ', nil, bgStyle)
				}
			}
			screenRow++
		}
	}

	// Draw VT rows
	vtRow := 0
	for screenRow < height && vtRow < vt.Rows() {
		for col := 0; col < width; col++ {
			if col < vt.Cols() {
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
			} else {
				screen.SetContent(x+col, y+screenRow, ' ', nil, bgStyle)
			}
		}
		screenRow++
		vtRow++
	}

	// Fill any remaining rows with background
	for screenRow < height {
		fillRow(screen, x, y+screenRow, width, bgStyle)
		screenRow++
	}

	// Draw cursor if focused, live view, terminal running, and cursor visible
	if p.hasFocus && scrollOff == 0 && p.term.RunningNoLock() && vt.CursorVisible() {
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
}

// InputHandler processes keyboard input, forwarding to PTY
func (p *Panel) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		if p.term == nil || !p.term.Running() {
			return
		}

		key := event.Key()
		mod := event.Modifiers()
		ctrl := mod&tcell.ModCtrl != 0
		shift := mod&tcell.ModShift != 0

		// Block navigation: Ctrl+Up/Down to select blocks
		if ctrl && !shift {
			switch key {
			case tcell.KeyUp:
				p.selectPrevBlock()
				return
			case tcell.KeyDown:
				p.selectNextBlock()
				return
			}
		}

		// Block actions when a block is selected
		if p.selectedBlock >= 0 {
			switch key {
			case tcell.KeyEnter:
				// Toggle expand/collapse
				p.toggleSelectedBlock()
				return
			case tcell.KeyEscape:
				// Deselect block, return to live
				p.selectedBlock = -1
				return
			case tcell.KeyRune:
				if !ctrl {
					switch event.Rune() {
					case 'y':
						// Copy entire block (command + output)
						p.copyBlock(false, false)
						return
					case 'c':
						// Copy command only
						p.copyBlock(true, false)
						return
					case 'o':
						// Copy output only
						p.copyBlock(false, true)
						return
					case 'e':
						// Expand all blocks
						p.setAllBlocksCollapsed(false)
						return
					case 'a':
						// Collapse (fold) all blocks
						p.setAllBlocksCollapsed(true)
						return
					}
				}
			}
			// Any non-handled key while block selected: deselect and pass through
			p.selectedBlock = -1
		}

		// Shift+PgUp/PgDn for scrollback
		if shift {
			_, _, _, h := p.GetInnerRect()
			switch key {
			case tcell.KeyPgUp:
				p.scrollOff += h / 2
				p.term.Lock()
				maxScroll := len(p.term.VT().Scrollback())
				p.term.Unlock()
				if p.scrollOff > maxScroll {
					p.scrollOff = maxScroll
				}
				return
			case tcell.KeyPgDn:
				p.scrollOff -= h / 2
				if p.scrollOff < 0 {
					p.scrollOff = 0
				}
				return
			}
		}

		// Any other key resets scroll to live and deselects
		if p.scrollOff > 0 {
			p.scrollOff = 0
		}

		data := keyToBytes(event)
		if data != nil {
			p.term.WriteInput(data)
		}
	})
}

// selectPrevBlock moves selection to the previous block.
func (p *Panel) selectPrevBlock() {
	if p.term == nil {
		return
	}
	p.term.Lock()
	count := p.term.VT().Blocks().BlockCount()
	p.term.Unlock()

	if count == 0 {
		return
	}
	if p.selectedBlock < 0 {
		// Select the last block
		p.selectedBlock = count - 1
	} else if p.selectedBlock > 0 {
		p.selectedBlock--
	}
}

// selectNextBlock moves selection to the next block.
func (p *Panel) selectNextBlock() {
	if p.term == nil {
		return
	}
	p.term.Lock()
	count := p.term.VT().Blocks().BlockCount()
	p.term.Unlock()

	if count == 0 {
		return
	}
	if p.selectedBlock < 0 {
		p.selectedBlock = 0
	} else if p.selectedBlock < count-1 {
		p.selectedBlock++
	} else {
		// Past last block = deselect (back to live)
		p.selectedBlock = -1
	}
}

// toggleSelectedBlock toggles expand/collapse on the selected block.
func (p *Panel) toggleSelectedBlock() {
	if p.term == nil || p.selectedBlock < 0 {
		return
	}
	p.term.Lock()
	blk := p.term.VT().Blocks().SelectedBlock(p.selectedBlock)
	p.term.Unlock()
	if blk != nil {
		blk.Collapsed = !blk.Collapsed
	}
}

// setAllBlocksCollapsed sets all blocks to collapsed or expanded.
func (p *Panel) setAllBlocksCollapsed(collapsed bool) {
	if p.term == nil {
		return
	}
	p.term.Lock()
	bt := p.term.VT().Blocks()
	for _, blk := range bt.Blocks {
		blk.Collapsed = collapsed
	}
	p.term.Unlock()
	if collapsed {
		p.statusMsg("All blocks collapsed")
	} else {
		p.statusMsg("All blocks expanded")
	}
}

// copyBlock copies the selected block's text to the clipboard.
// If cmdOnly, copies just the command. If outputOnly, copies just the output.
// Otherwise copies both.
func (p *Panel) copyBlock(cmdOnly, outputOnly bool) {
	if p.term == nil || p.selectedBlock < 0 {
		return
	}
	p.term.Lock()
	blk := p.term.VT().Blocks().SelectedBlock(p.selectedBlock)
	p.term.Unlock()
	if blk == nil {
		return
	}

	var text string
	switch {
	case cmdOnly:
		text = blk.Command
	case outputOnly:
		text = blk.OutputText()
	default:
		text = blk.PlainText()
	}

	if text == "" {
		p.statusMsg("Nothing to copy")
		return
	}

	clipboardCopy(text)

	switch {
	case cmdOnly:
		p.statusMsg("Command copied")
	case outputOnly:
		p.statusMsg("Output copied")
	default:
		p.statusMsg("Block copied")
	}
}

// clipboardCopy writes text to the system clipboard.
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

// MouseHandler handles mouse events (scrolling, block clicks)
func (p *Panel) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
	return p.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if !p.InRect(event.Position()) {
			return false, nil
		}

		if p.term == nil {
			return false, nil
		}

		switch action {
		case tview.MouseScrollUp:
			p.scrollOff += 3
			p.term.Lock()
			maxScroll := len(p.term.VT().Scrollback())
			p.term.Unlock()
			if p.scrollOff > maxScroll {
				p.scrollOff = maxScroll
			}
			return true, nil
		case tview.MouseScrollDown:
			p.scrollOff -= 3
			if p.scrollOff < 0 {
				p.scrollOff = 0
			}
			return true, nil
		case tview.MouseLeftClick:
			setFocus(p)
			mx, my := event.Position()
			// Check click regions for toggle/copy buttons
			for _, cr := range p.clickRegions {
				if my == cr.y && mx >= cr.x && mx < cr.x+cr.w {
					p.term.Lock()
					blk := p.term.VT().Blocks().SelectedBlock(cr.blockIdx)
					if blk != nil {
						switch cr.action {
						case "toggle":
							blk.Collapsed = !blk.Collapsed
						case "copy":
							text := blk.PlainText()
							p.term.Unlock()
							clipboardCopy(text)
							p.statusMsg("Block copied")
							return true, nil
						}
					}
					p.term.Unlock()
					return true, nil
				}
			}
			return true, nil
		}

		return false, nil
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
