package mergeview

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Pane focus constants
const (
	PaneLocal  = 0
	PaneBase   = 1
	PaneRemote = 2
	PaneResult = 3
)

// MergeView is a three-pane merge tool widget.
// Top row: LOCAL | BASE | REMOTE (read-only)
// Bottom: RESULT (editable)
type MergeView struct {
	*tview.Box

	localLines  []string
	baseLines   []string
	remoteLines []string
	resultLines []string // editable

	conflicts   []Conflict
	conflictIdx int // current conflict index, -1 if none

	focusPane int // PaneLocal..PaneResult
	scrollRow int // synchronized scroll position for top panes

	// RESULT pane state
	resultScrollRow int
	cursorRow       int
	cursorCol       int
	modified        bool

	outputPath string // file to save result to
	hasFocus   bool

	// Callbacks
	onStatusMessage func(string)
	onChange         func()
	onClose          func(saved bool)
}

// New creates a new MergeView from pre-parsed content.
func New(local, base, remote, result []string, conflicts []Conflict, outputPath string) *MergeView {
	mv := &MergeView{
		Box:         tview.NewBox(),
		localLines:  local,
		baseLines:   base,
		remoteLines: remote,
		resultLines: result,
		conflicts:   conflicts,
		conflictIdx: -1,
		focusPane:   PaneResult,
		outputPath:  outputPath,
	}
	mv.SetBackgroundColor(ui.ColorBg)

	// Set initial conflict index if there are conflicts
	if len(conflicts) > 0 {
		mv.conflictIdx = 0
	}

	return mv
}

// NewFromConflictFile creates a MergeView by parsing conflict markers in file content.
func NewFromConflictFile(content string, outputPath string) *MergeView {
	local, base, remote, result, conflicts := ParseConflicts(content)
	return New(local, base, remote, result, conflicts, outputPath)
}

// NewFromThreeFiles creates a MergeView from three separate file contents.
func NewFromThreeFiles(localPath, basePath, remotePath, outputPath string) (*MergeView, error) {
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("reading local: %w", err)
	}
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("reading base: %w", err)
	}
	remoteData, err := os.ReadFile(remotePath)
	if err != nil {
		return nil, fmt.Errorf("reading remote: %w", err)
	}

	local := splitLines(string(localData))
	base := splitLines(string(baseData))
	remote := splitLines(string(remoteData))

	// Result starts as copy of local
	result := make([]string, len(local))
	copy(result, local)

	// Detect conflicts by comparing the three
	_, _, _, _, conflicts := ParseConflicts(buildConflictedContent(local, base, remote))

	// If no conflict markers found (three clean files), detect structural conflicts
	if len(conflicts) == 0 {
		_, _, _, result, conflicts = ParseThreeWay(string(localData), string(baseData), string(remoteData))
	}

	return New(local, base, remote, result, conflicts, outputPath), nil
}

// buildConflictedContent is a helper that doesn't apply here for three clean files.
// For three-way merge with separate files, we rely on ParseThreeWay instead.
func buildConflictedContent(local, base, remote []string) string {
	// This is only used as a fallback; real three-way merging uses ParseThreeWay
	return strings.Join(local, "\n")
}

// Modified returns whether the result has been modified.
func (mv *MergeView) Modified() bool {
	return mv.modified
}

// OutputPath returns the output file path.
func (mv *MergeView) OutputPath() string {
	return mv.outputPath
}

// ConflictCount returns the total number of conflicts.
func (mv *MergeView) ConflictCount() int {
	return len(mv.conflicts)
}

// UnresolvedCount returns the number of unresolved conflicts.
func (mv *MergeView) UnresolvedCount() int {
	count := 0
	for _, c := range mv.conflicts {
		if !c.Resolved {
			count++
		}
	}
	return count
}

// CurrentConflict returns the current conflict index (1-based) and total.
func (mv *MergeView) CurrentConflict() (int, int) {
	if mv.conflictIdx < 0 || len(mv.conflicts) == 0 {
		return 0, len(mv.conflicts)
	}
	return mv.conflictIdx + 1, len(mv.conflicts)
}

// SetOnStatusMessage sets the status message callback.
func (mv *MergeView) SetOnStatusMessage(fn func(string)) {
	mv.onStatusMessage = fn
}

// SetOnChange sets the change callback.
func (mv *MergeView) SetOnChange(fn func()) {
	mv.onChange = fn
}

// SetOnClose sets the close callback.
func (mv *MergeView) SetOnClose(fn func(saved bool)) {
	mv.onClose = fn
}

// Save writes the RESULT content to the output file.
func (mv *MergeView) Save() error {
	if mv.outputPath == "" {
		return fmt.Errorf("no output path set")
	}
	content := strings.Join(mv.resultLines, "\n") + "\n"
	err := os.WriteFile(mv.outputPath, []byte(content), 0644)
	if err != nil {
		return err
	}
	mv.modified = false
	if mv.onChange != nil {
		mv.onChange()
	}
	return nil
}

// StatusText returns text suitable for the status bar.
func (mv *MergeView) StatusText() string {
	paneNames := []string{"LOCAL", "BASE", "REMOTE", "RESULT"}
	paneName := paneNames[mv.focusPane]

	conflictStr := ""
	if len(mv.conflicts) > 0 {
		current, total := mv.CurrentConflict()
		unresolved := mv.UnresolvedCount()
		conflictStr = fmt.Sprintf(" | Conflict %d of %d (%d unresolved)", current, total, unresolved)
	}

	cursorStr := ""
	if mv.focusPane == PaneResult {
		cursorStr = fmt.Sprintf(" | Ln %d, Col %d", mv.cursorRow+1, mv.cursorCol+1)
	}

	modStr := ""
	if mv.modified {
		modStr = " [modified]"
	}

	return fmt.Sprintf("MERGE: %s%s%s%s", paneName, conflictStr, cursorStr, modStr)
}

// NextConflict jumps to the next unresolved conflict.
func (mv *MergeView) NextConflict() {
	if len(mv.conflicts) == 0 {
		return
	}

	start := mv.conflictIdx + 1
	if start >= len(mv.conflicts) {
		start = 0
	}

	for i := 0; i < len(mv.conflicts); i++ {
		idx := (start + i) % len(mv.conflicts)
		if !mv.conflicts[idx].Resolved {
			mv.conflictIdx = idx
			mv.scrollToConflict(idx)
			mv.sendStatus(fmt.Sprintf("Conflict %d of %d", idx+1, len(mv.conflicts)))
			return
		}
	}

	// All resolved - just go to next
	mv.conflictIdx = start % len(mv.conflicts)
	mv.sendStatus("All conflicts resolved")
}

// PrevConflict jumps to the previous unresolved conflict.
func (mv *MergeView) PrevConflict() {
	if len(mv.conflicts) == 0 {
		return
	}

	start := mv.conflictIdx - 1
	if start < 0 {
		start = len(mv.conflicts) - 1
	}

	for i := 0; i < len(mv.conflicts); i++ {
		idx := (start - i + len(mv.conflicts)) % len(mv.conflicts)
		if !mv.conflicts[idx].Resolved {
			mv.conflictIdx = idx
			mv.scrollToConflict(idx)
			mv.sendStatus(fmt.Sprintf("Conflict %d of %d", idx+1, len(mv.conflicts)))
			return
		}
	}

	mv.conflictIdx = start
	mv.sendStatus("All conflicts resolved")
}

// AcceptLocal resolves the current conflict with LOCAL content.
func (mv *MergeView) AcceptLocal() {
	mv.resolveConflict("local")
}

// AcceptBase resolves the current conflict with BASE content.
func (mv *MergeView) AcceptBase() {
	mv.resolveConflict("base")
}

// AcceptRemote resolves the current conflict with REMOTE content.
func (mv *MergeView) AcceptRemote() {
	mv.resolveConflict("remote")
}

// AcceptBoth resolves the current conflict with LOCAL then REMOTE content.
func (mv *MergeView) AcceptBoth() {
	mv.resolveConflict("both")
}

func (mv *MergeView) resolveConflict(resolution string) {
	if mv.conflictIdx < 0 || mv.conflictIdx >= len(mv.conflicts) {
		mv.sendStatus("No conflict selected")
		return
	}

	c := &mv.conflicts[mv.conflictIdx]

	var newContent []string
	switch resolution {
	case "local":
		newContent = c.LocalContent
	case "base":
		newContent = c.BaseContent
	case "remote":
		newContent = c.RemoteContent
	case "both":
		newContent = append(append([]string{}, c.LocalContent...), c.RemoteContent...)
	}

	// Replace the conflict region in result
	mv.replaceResultRegion(c.ResultStart, c.ResultEnd, newContent)

	c.Resolved = true
	c.Resolution = resolution
	mv.modified = true

	if mv.onChange != nil {
		mv.onChange()
	}

	mv.sendStatus(fmt.Sprintf("Conflict %d resolved (%s)", mv.conflictIdx+1, resolution))
}

// replaceResultRegion replaces lines from start to end (inclusive) with new content.
func (mv *MergeView) replaceResultRegion(start, end int, newContent []string) {
	if start < 0 || start > len(mv.resultLines) {
		return
	}
	if end >= len(mv.resultLines) {
		end = len(mv.resultLines) - 1
	}

	oldLen := end - start + 1
	newLen := len(newContent)

	// Build new result lines
	var newResult []string
	newResult = append(newResult, mv.resultLines[:start]...)
	newResult = append(newResult, newContent...)
	if end+1 < len(mv.resultLines) {
		newResult = append(newResult, mv.resultLines[end+1:]...)
	}
	mv.resultLines = newResult

	// Adjust subsequent conflict ranges
	delta := newLen - oldLen
	for i := mv.conflictIdx + 1; i < len(mv.conflicts); i++ {
		mv.conflicts[i].ResultStart += delta
		mv.conflicts[i].ResultEnd += delta
	}

	// Update current conflict's result range
	if mv.conflictIdx >= 0 && mv.conflictIdx < len(mv.conflicts) {
		mv.conflicts[mv.conflictIdx].ResultEnd = start + newLen - 1
		if newLen == 0 {
			mv.conflicts[mv.conflictIdx].ResultEnd = start
		}
	}
}

func (mv *MergeView) scrollToConflict(idx int) {
	if idx < 0 || idx >= len(mv.conflicts) {
		return
	}
	c := mv.conflicts[idx]

	// Scroll top panes to show the conflict in LOCAL
	mv.scrollRow = c.LocalStart
	if mv.scrollRow > 0 {
		mv.scrollRow -= 2 // show a bit of context
		if mv.scrollRow < 0 {
			mv.scrollRow = 0
		}
	}

	// Scroll result pane to show the conflict
	mv.resultScrollRow = c.ResultStart
	if mv.resultScrollRow > 0 {
		mv.resultScrollRow -= 2
		if mv.resultScrollRow < 0 {
			mv.resultScrollRow = 0
		}
	}

	// Move cursor to conflict start in result
	mv.cursorRow = c.ResultStart
	mv.cursorCol = 0
}

func (mv *MergeView) sendStatus(msg string) {
	if mv.onStatusMessage != nil {
		mv.onStatusMessage(msg)
	}
}

// isInConflict returns the conflict index if the given line (in the respective pane)
// falls within a conflict region. Returns -1 if not in a conflict.
func (mv *MergeView) isInConflictLocal(lineIdx int) int {
	for i, c := range mv.conflicts {
		if lineIdx >= c.LocalStart && lineIdx <= c.LocalEnd {
			return i
		}
	}
	return -1
}

func (mv *MergeView) isInConflictRemote(lineIdx int) int {
	for i, c := range mv.conflicts {
		if lineIdx >= c.RemoteStart && lineIdx <= c.RemoteEnd {
			return i
		}
	}
	return -1
}

func (mv *MergeView) isInConflictBase(lineIdx int) int {
	for i, c := range mv.conflicts {
		if lineIdx >= c.BaseStart && lineIdx <= c.BaseEnd {
			return i
		}
	}
	return -1
}

func (mv *MergeView) isInConflictResult(lineIdx int) int {
	for i, c := range mv.conflicts {
		if lineIdx >= c.ResultStart && lineIdx <= c.ResultEnd {
			return i
		}
	}
	return -1
}

// Draw renders the merge view to the screen.
func (mv *MergeView) Draw(screen tcell.Screen) {
	mv.Box.DrawForSubclass(screen, mv)
	x, y, width, height := mv.GetInnerRect()

	if width < 30 || height < 6 {
		return
	}

	// Layout: 60% top panes, 40% result pane
	// Reserve 1 line each for top header and bottom header
	topHeight := (height * 60) / 100
	if topHeight < 4 {
		topHeight = 4
	}
	bottomHeight := height - topHeight
	if bottomHeight < 3 {
		bottomHeight = 3
	}
	topHeight = height - bottomHeight

	// Top panes each get width/3
	paneWidth := width / 3
	lastPaneWidth := width - 2*paneWidth // last pane gets remainder

	// Draw top pane headers (1 line each)
	mv.drawPaneHeader(screen, x, y, paneWidth, "LOCAL", PaneLocal)
	mv.drawPaneHeader(screen, x+paneWidth, y, paneWidth, "BASE", PaneBase)
	mv.drawPaneHeader(screen, x+2*paneWidth, y, lastPaneWidth, "REMOTE", PaneRemote)

	// Draw top pane content
	topContentHeight := topHeight - 1 // minus header
	mv.drawPane(screen, x, y+1, paneWidth, topContentHeight, mv.localLines, mv.scrollRow, PaneLocal)
	mv.drawPane(screen, x+paneWidth, y+1, paneWidth, topContentHeight, mv.baseLines, mv.scrollRow, PaneBase)
	mv.drawPane(screen, x+2*paneWidth, y+1, lastPaneWidth, topContentHeight, mv.remoteLines, mv.scrollRow, PaneRemote)

	// Draw result pane header
	resultY := y + topHeight
	mv.drawPaneHeader(screen, x, resultY, width, "RESULT", PaneResult)

	// Draw result pane content
	mv.drawResultPane(screen, x, resultY+1, width, bottomHeight-1)
}

func (mv *MergeView) drawPaneHeader(screen tcell.Screen, x, y, width int, title string, pane int) {
	style := tcell.StyleDefault.Foreground(ui.ColorStatusText).Background(ui.ColorStatusBg)
	if mv.focusPane == pane && mv.hasFocus {
		style = tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected).Bold(true)
	}

	// Fill header line
	for i := 0; i < width; i++ {
		screen.SetContent(x+i, y, ' ', nil, style)
	}

	// Center title
	startX := x + (width-len(title))/2
	for i, ch := range title {
		if startX+i < x+width {
			screen.SetContent(startX+i, y, ch, nil, style)
		}
	}
}

func (mv *MergeView) drawPane(screen tcell.Screen, x, y, width, height int, lines []string, scrollRow int, pane int) {
	bgStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)

	// Conflict highlight styles
	localStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(0, 80, 0))      // dark green
	baseStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(60, 60, 80))      // neutral gray-blue
	remoteStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(0, 0, 120))     // dark blue
	resolvedStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(0, 60, 0))    // darker green

	for row := 0; row < height; row++ {
		lineIdx := scrollRow + row
		screenY := y + row

		// Clear line
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, screenY, ' ', nil, bgStyle)
		}

		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}

		line := lines[lineIdx]

		// Determine if this line is in a conflict
		conflictIdx := -1
		lineStyle := bgStyle
		switch pane {
		case PaneLocal:
			conflictIdx = mv.isInConflictLocal(lineIdx)
			if conflictIdx >= 0 {
				if mv.conflicts[conflictIdx].Resolved {
					lineStyle = resolvedStyle
				} else {
					lineStyle = localStyle
				}
			}
		case PaneBase:
			conflictIdx = mv.isInConflictBase(lineIdx)
			if conflictIdx >= 0 {
				if mv.conflicts[conflictIdx].Resolved {
					lineStyle = resolvedStyle
				} else {
					lineStyle = baseStyle
				}
			}
		case PaneRemote:
			conflictIdx = mv.isInConflictRemote(lineIdx)
			if conflictIdx >= 0 {
				if mv.conflicts[conflictIdx].Resolved {
					lineStyle = resolvedStyle
				} else {
					lineStyle = remoteStyle
				}
			}
		}

		// Draw word-level diffs if in an unresolved conflict
		if conflictIdx >= 0 && !mv.conflicts[conflictIdx].Resolved && (pane == PaneLocal || pane == PaneRemote) {
			mv.drawWordDiffLine(screen, x, screenY, width, line, lineIdx, pane, conflictIdx, lineStyle)
		} else {
			// Regular line drawing
			for col := 0; col < width && col < len(line); col++ {
				screen.SetContent(x+col, screenY, rune(line[col]), nil, lineStyle)
			}
		}
	}
}

func (mv *MergeView) drawWordDiffLine(screen tcell.Screen, x, y, width int, line string, lineIdx int, pane int, conflictIdx int, baseLineStyle tcell.Style) {
	c := mv.conflicts[conflictIdx]

	// Find the corresponding line in the other pane for diff
	var otherLine string
	localLineOffset := lineIdx - c.LocalStart
	remoteLineOffset := lineIdx - c.RemoteStart

	if pane == PaneLocal {
		if remoteLineOffset >= 0 && remoteLineOffset < len(c.RemoteContent) {
			otherLine = c.RemoteContent[remoteLineOffset]
		} else if localLineOffset >= 0 && localLineOffset < len(c.RemoteContent) {
			otherLine = c.RemoteContent[localLineOffset]
		}
	} else { // PaneRemote
		if localLineOffset >= 0 && localLineOffset < len(c.LocalContent) {
			otherLine = c.LocalContent[localLineOffset]
		} else if remoteLineOffset >= 0 && remoteLineOffset < len(c.LocalContent) {
			otherLine = c.LocalContent[remoteLineOffset]
		}
	}

	if otherLine == "" && line == "" {
		return
	}

	// Compute word diffs
	var diffs []WordDiff
	if pane == PaneLocal {
		diffs, _ = ComputeWordDiffs(line, otherLine)
	} else {
		_, diffs = ComputeWordDiffs(otherLine, line)
	}

	// Highlighted style for changed words
	changedStyle := baseLineStyle.Bold(true).Underline(true)

	col := 0
	for _, wd := range diffs {
		style := baseLineStyle
		if !wd.IsCommon {
			style = changedStyle
		}
		for _, ch := range wd.Text {
			if col < width {
				screen.SetContent(x+col, y, ch, nil, style)
				col++
			}
		}
	}
}

func (mv *MergeView) drawResultPane(screen tcell.Screen, x, y, width, height int) {
	bgStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
	conflictStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(120, 0, 0)) // red for unresolved
	resolvedStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.NewRGBColor(0, 60, 0))  // green for resolved
	cursorStyle := tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)

	for row := 0; row < height; row++ {
		lineIdx := mv.resultScrollRow + row
		screenY := y + row

		// Clear line
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, screenY, ' ', nil, bgStyle)
		}

		if lineIdx < 0 || lineIdx >= len(mv.resultLines) {
			// Draw cursor on empty line if needed
			if lineIdx == mv.cursorRow && mv.focusPane == PaneResult && mv.hasFocus {
				screen.SetContent(x, screenY, ' ', nil, cursorStyle)
			}
			continue
		}

		line := mv.resultLines[lineIdx]

		// Check if line is in a conflict region
		lineStyle := bgStyle
		cIdx := mv.isInConflictResult(lineIdx)
		if cIdx >= 0 {
			if mv.conflicts[cIdx].Resolved {
				lineStyle = resolvedStyle
			} else {
				lineStyle = conflictStyle
			}
		}

		// Draw the line
		for col := 0; col < width; col++ {
			ch := ' '
			if col < len(line) {
				ch = rune(line[col])
			}

			style := lineStyle
			// Cursor highlight
			if lineIdx == mv.cursorRow && col == mv.cursorCol && mv.focusPane == PaneResult && mv.hasFocus {
				style = cursorStyle
			}

			screen.SetContent(x+col, screenY, ch, nil, style)
		}
	}
}

// Focus handles focus events.
func (mv *MergeView) Focus(delegate func(p tview.Primitive)) {
	mv.hasFocus = true
	mv.Box.Focus(delegate)
}

// Blur handles blur events.
func (mv *MergeView) Blur() {
	mv.hasFocus = false
	mv.Box.Blur()
}

// InputHandler handles keyboard input.
func (mv *MergeView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return mv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		mod := event.Modifiers()
		ctrl := mod&tcell.ModCtrl != 0

		switch {
		case key == tcell.KeyTab:
			// Cycle focus between panes
			mv.focusPane = (mv.focusPane + 1) % 4

		case key == tcell.KeyBacktab:
			// Reverse cycle
			mv.focusPane = (mv.focusPane + 3) % 4

		case ctrl && (event.Rune() == 'e' || key == tcell.KeyCtrlE):
			mv.NextConflict()

		case ctrl && (event.Rune() == 's' || key == tcell.KeyCtrlS):
			if err := mv.Save(); err != nil {
				mv.sendStatus("Save error: " + err.Error())
			} else {
				mv.sendStatus("Saved to " + mv.outputPath)
			}

		case key == tcell.KeyUp:
			mv.handleUp()

		case key == tcell.KeyDown:
			mv.handleDown()

		case key == tcell.KeyLeft:
			mv.handleLeft()

		case key == tcell.KeyRight:
			mv.handleRight()

		case key == tcell.KeyPgUp:
			mv.handlePageUp()

		case key == tcell.KeyPgDn:
			mv.handlePageDown()

		case key == tcell.KeyHome:
			if mv.focusPane == PaneResult {
				mv.cursorCol = 0
			}

		case key == tcell.KeyEnd:
			if mv.focusPane == PaneResult {
				if mv.cursorRow < len(mv.resultLines) {
					mv.cursorCol = len(mv.resultLines[mv.cursorRow])
				}
			}

		case key == tcell.KeyRune && !ctrl:
			r := event.Rune()
			// Check for conflict resolution keys (only when not in RESULT editing)
			if mv.focusPane != PaneResult {
				switch r {
				case '1', 'l':
					mv.AcceptLocal()
					return
				case '2', 'b':
					mv.AcceptBase()
					return
				case '3', 'r':
					mv.AcceptRemote()
					return
				case 'a':
					mv.AcceptBoth()
					return
				}
			}
			// Text input in RESULT pane
			if mv.focusPane == PaneResult {
				mv.insertChar(r)
			}

		case key == tcell.KeyEnter:
			if mv.focusPane == PaneResult {
				mv.insertNewline()
			}

		case key == tcell.KeyBackspace || key == tcell.KeyBackspace2:
			if mv.focusPane == PaneResult {
				mv.deleteCharBack()
			}

		case key == tcell.KeyDelete:
			if mv.focusPane == PaneResult {
				mv.deleteCharForward()
			}
		}
	})
}

// Navigation handlers
func (mv *MergeView) handleUp() {
	if mv.focusPane == PaneResult {
		if mv.cursorRow > 0 {
			mv.cursorRow--
			mv.clampCursorCol()
			mv.ensureResultCursorVisible()
		}
	} else {
		if mv.scrollRow > 0 {
			mv.scrollRow--
		}
	}
}

func (mv *MergeView) handleDown() {
	if mv.focusPane == PaneResult {
		if mv.cursorRow < len(mv.resultLines)-1 {
			mv.cursorRow++
			mv.clampCursorCol()
			mv.ensureResultCursorVisible()
		}
	} else {
		maxLine := maxLen(mv.localLines, mv.baseLines, mv.remoteLines) - 1
		if mv.scrollRow < maxLine {
			mv.scrollRow++
		}
	}
}

func (mv *MergeView) handleLeft() {
	if mv.focusPane == PaneResult {
		if mv.cursorCol > 0 {
			mv.cursorCol--
		} else if mv.cursorRow > 0 {
			mv.cursorRow--
			if mv.cursorRow < len(mv.resultLines) {
				mv.cursorCol = len(mv.resultLines[mv.cursorRow])
			}
			mv.ensureResultCursorVisible()
		}
	}
}

func (mv *MergeView) handleRight() {
	if mv.focusPane == PaneResult {
		lineLen := 0
		if mv.cursorRow < len(mv.resultLines) {
			lineLen = len(mv.resultLines[mv.cursorRow])
		}
		if mv.cursorCol < lineLen {
			mv.cursorCol++
		} else if mv.cursorRow < len(mv.resultLines)-1 {
			mv.cursorRow++
			mv.cursorCol = 0
			mv.ensureResultCursorVisible()
		}
	}
}

func (mv *MergeView) handlePageUp() {
	_, _, _, height := mv.GetInnerRect()
	pageSize := height / 3

	if mv.focusPane == PaneResult {
		mv.cursorRow -= pageSize
		if mv.cursorRow < 0 {
			mv.cursorRow = 0
		}
		mv.clampCursorCol()
		mv.ensureResultCursorVisible()
	} else {
		mv.scrollRow -= pageSize
		if mv.scrollRow < 0 {
			mv.scrollRow = 0
		}
	}
}

func (mv *MergeView) handlePageDown() {
	_, _, _, height := mv.GetInnerRect()
	pageSize := height / 3

	if mv.focusPane == PaneResult {
		mv.cursorRow += pageSize
		if mv.cursorRow >= len(mv.resultLines) {
			mv.cursorRow = len(mv.resultLines) - 1
			if mv.cursorRow < 0 {
				mv.cursorRow = 0
			}
		}
		mv.clampCursorCol()
		mv.ensureResultCursorVisible()
	} else {
		mv.scrollRow += pageSize
		maxLine := maxLen(mv.localLines, mv.baseLines, mv.remoteLines) - 1
		if mv.scrollRow > maxLine {
			mv.scrollRow = maxLine
			if mv.scrollRow < 0 {
				mv.scrollRow = 0
			}
		}
	}
}

func (mv *MergeView) clampCursorCol() {
	if mv.cursorRow >= 0 && mv.cursorRow < len(mv.resultLines) {
		lineLen := len(mv.resultLines[mv.cursorRow])
		if mv.cursorCol > lineLen {
			mv.cursorCol = lineLen
		}
	} else {
		mv.cursorCol = 0
	}
}

func (mv *MergeView) ensureResultCursorVisible() {
	_, _, _, height := mv.GetInnerRect()
	bottomHeight := height - (height*60)/100
	if bottomHeight < 3 {
		bottomHeight = 3
	}
	visibleRows := bottomHeight - 1 // minus header

	if mv.cursorRow < mv.resultScrollRow {
		mv.resultScrollRow = mv.cursorRow
	}
	if mv.cursorRow >= mv.resultScrollRow+visibleRows {
		mv.resultScrollRow = mv.cursorRow - visibleRows + 1
	}
	if mv.resultScrollRow < 0 {
		mv.resultScrollRow = 0
	}
}

// Text editing in RESULT pane
func (mv *MergeView) insertChar(ch rune) {
	mv.ensureResultLines()

	if mv.cursorRow >= len(mv.resultLines) {
		mv.resultLines = append(mv.resultLines, "")
	}

	line := mv.resultLines[mv.cursorRow]
	col := mv.cursorCol
	if col > len(line) {
		col = len(line)
	}

	newLine := line[:col] + string(ch) + line[col:]
	mv.resultLines[mv.cursorRow] = newLine
	mv.cursorCol = col + 1
	mv.modified = true
	mv.markManualResolution()

	if mv.onChange != nil {
		mv.onChange()
	}
}

func (mv *MergeView) insertNewline() {
	mv.ensureResultLines()

	if mv.cursorRow >= len(mv.resultLines) {
		mv.resultLines = append(mv.resultLines, "")
		mv.cursorRow = len(mv.resultLines) - 1
	}

	line := mv.resultLines[mv.cursorRow]
	col := mv.cursorCol
	if col > len(line) {
		col = len(line)
	}

	before := line[:col]
	after := line[col:]

	mv.resultLines[mv.cursorRow] = before

	// Insert new line after current
	newLines := make([]string, len(mv.resultLines)+1)
	copy(newLines, mv.resultLines[:mv.cursorRow+1])
	newLines[mv.cursorRow+1] = after
	copy(newLines[mv.cursorRow+2:], mv.resultLines[mv.cursorRow+1:])
	mv.resultLines = newLines

	mv.cursorRow++
	mv.cursorCol = 0
	mv.modified = true
	mv.markManualResolution()
	mv.ensureResultCursorVisible()

	// Adjust conflict result ranges after insertion
	for i := range mv.conflicts {
		if mv.conflicts[i].ResultStart > mv.cursorRow-1 {
			mv.conflicts[i].ResultStart++
			mv.conflicts[i].ResultEnd++
		} else if mv.conflicts[i].ResultEnd >= mv.cursorRow-1 {
			mv.conflicts[i].ResultEnd++
		}
	}

	if mv.onChange != nil {
		mv.onChange()
	}
}

func (mv *MergeView) deleteCharBack() {
	if mv.cursorCol > 0 {
		line := mv.resultLines[mv.cursorRow]
		col := mv.cursorCol
		if col > len(line) {
			col = len(line)
		}
		mv.resultLines[mv.cursorRow] = line[:col-1] + line[col:]
		mv.cursorCol = col - 1
		mv.modified = true
		mv.markManualResolution()
	} else if mv.cursorRow > 0 {
		// Join with previous line
		prevLine := mv.resultLines[mv.cursorRow-1]
		curLine := mv.resultLines[mv.cursorRow]
		mv.cursorCol = len(prevLine)
		mv.resultLines[mv.cursorRow-1] = prevLine + curLine

		// Remove current line
		mv.resultLines = append(mv.resultLines[:mv.cursorRow], mv.resultLines[mv.cursorRow+1:]...)

		mv.cursorRow--
		mv.modified = true
		mv.markManualResolution()
		mv.ensureResultCursorVisible()

		// Adjust conflict result ranges after deletion
		for i := range mv.conflicts {
			if mv.conflicts[i].ResultStart > mv.cursorRow {
				mv.conflicts[i].ResultStart--
				mv.conflicts[i].ResultEnd--
			} else if mv.conflicts[i].ResultEnd > mv.cursorRow {
				mv.conflicts[i].ResultEnd--
			}
		}
	}

	if mv.onChange != nil {
		mv.onChange()
	}
}

func (mv *MergeView) deleteCharForward() {
	if mv.cursorRow >= len(mv.resultLines) {
		return
	}

	line := mv.resultLines[mv.cursorRow]
	col := mv.cursorCol

	if col < len(line) {
		mv.resultLines[mv.cursorRow] = line[:col] + line[col+1:]
		mv.modified = true
		mv.markManualResolution()
	} else if mv.cursorRow < len(mv.resultLines)-1 {
		// Join with next line
		nextLine := mv.resultLines[mv.cursorRow+1]
		mv.resultLines[mv.cursorRow] = line + nextLine
		mv.resultLines = append(mv.resultLines[:mv.cursorRow+1], mv.resultLines[mv.cursorRow+2:]...)
		mv.modified = true
		mv.markManualResolution()

		for i := range mv.conflicts {
			if mv.conflicts[i].ResultStart > mv.cursorRow {
				mv.conflicts[i].ResultStart--
				mv.conflicts[i].ResultEnd--
			} else if mv.conflicts[i].ResultEnd > mv.cursorRow {
				mv.conflicts[i].ResultEnd--
			}
		}
	}

	if mv.onChange != nil {
		mv.onChange()
	}
}

func (mv *MergeView) ensureResultLines() {
	if len(mv.resultLines) == 0 {
		mv.resultLines = []string{""}
	}
}

// markManualResolution marks the current conflict (if cursor is in one) as manually resolved.
func (mv *MergeView) markManualResolution() {
	cIdx := mv.isInConflictResult(mv.cursorRow)
	if cIdx >= 0 && !mv.conflicts[cIdx].Resolved {
		mv.conflicts[cIdx].Resolved = true
		mv.conflicts[cIdx].Resolution = "manual"
	}
}

func maxLen(a, b, c []string) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	if len(c) > n {
		n = len(c)
	}
	if n == 0 {
		return 1
	}
	return n
}
