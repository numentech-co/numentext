package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// Tab represents an open file tab
type Tab struct {
	Name        string
	FilePath    string
	Buffer      *Buffer
	Highlighter *Highlighter
	CursorRow   int
	CursorCol   int
	ScrollRow   int
	ScrollCol   int
	SelectStart [2]int // row, col (-1 = no selection)
	SelectEnd   [2]int
	HasSelect   bool
}

// Editor is the core editor component
type Editor struct {
	*tview.Box
	tabs        []*Tab
	activeTab   int
	pageHeight  int
	onChange    func()
	onTabChange func()
	hasFocus    bool
	cachedHL    []HighlightedLine
	cachedHLVer int
	hlVersion   int
}

func NewEditor() *Editor {
	e := &Editor{
		Box:  tview.NewBox(),
		tabs: []*Tab{},
	}
	e.SetBorder(false)
	return e
}

func (e *Editor) SetOnChange(fn func()) {
	e.onChange = fn
}

func (e *Editor) SetOnTabChange(fn func()) {
	e.onTabChange = fn
}

func (e *Editor) notifyChange() {
	e.hlVersion++
	if e.onChange != nil {
		e.onChange()
	}
}

func (e *Editor) Focus(delegate func(p tview.Primitive)) {
	e.hasFocus = true
	e.Box.Focus(delegate)
}

func (e *Editor) Blur() {
	e.hasFocus = false
	e.Box.Blur()
}

func (e *Editor) HasFocus() bool {
	return e.hasFocus
}

func (e *Editor) notifyTabChange() {
	e.hlVersion++
	e.cachedHL = nil
	if e.onTabChange != nil {
		e.onTabChange()
	}
}

// Tab management
func (e *Editor) NewTab(name, filePath string, content string) {
	buf := NewBufferFromText(content)
	hl := NewHighlighter(filePath)
	tab := &Tab{
		Name:        name,
		FilePath:    filePath,
		Buffer:      buf,
		Highlighter: hl,
		SelectStart: [2]int{-1, -1},
		SelectEnd:   [2]int{-1, -1},
	}
	e.tabs = append(e.tabs, tab)
	e.activeTab = len(e.tabs) - 1
	e.notifyTabChange()
	e.notifyChange()
}

func (e *Editor) OpenFile(filePath string) error {
	// Check if already open
	for i, tab := range e.tabs {
		if tab.FilePath == filePath {
			e.activeTab = i
			e.notifyTabChange()
			return nil
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	parts := strings.Split(filePath, "/")
	name := parts[len(parts)-1]
	e.NewTab(name, filePath, content)
	return nil
}

func (e *Editor) SaveCurrentFile() error {
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return fmt.Errorf("no file path")
	}
	content := tab.Buffer.Text()
	err := os.WriteFile(tab.FilePath, []byte(content), 0644)
	if err != nil {
		return err
	}
	tab.Buffer.SetModified(false)
	e.notifyChange()
	return nil
}

func (e *Editor) SaveAs(filePath string) error {
	tab := e.ActiveTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	tab.FilePath = filePath
	parts := strings.Split(filePath, "/")
	tab.Name = parts[len(parts)-1]
	tab.Highlighter.DetectLanguage(filePath)
	return e.SaveCurrentFile()
}

func (e *Editor) CloseTab(idx int) {
	if idx < 0 || idx >= len(e.tabs) {
		return
	}
	e.tabs = append(e.tabs[:idx], e.tabs[idx+1:]...)
	if e.activeTab >= len(e.tabs) {
		e.activeTab = len(e.tabs) - 1
	}
	if e.activeTab < 0 {
		e.activeTab = 0
	}
	e.notifyTabChange()
	e.notifyChange()
}

func (e *Editor) CloseCurrentTab() {
	e.CloseTab(e.activeTab)
}

func (e *Editor) ActiveTab() *Tab {
	if len(e.tabs) == 0 {
		return nil
	}
	if e.activeTab < 0 || e.activeTab >= len(e.tabs) {
		return nil
	}
	return e.tabs[e.activeTab]
}

func (e *Editor) Tabs() []*Tab {
	return e.tabs
}

func (e *Editor) ActiveTabIndex() int {
	return e.activeTab
}

func (e *Editor) SetActiveTab(idx int) {
	if idx >= 0 && idx < len(e.tabs) {
		e.activeTab = idx
		e.notifyTabChange()
		e.notifyChange()
	}
}

func (e *Editor) TabCount() int {
	return len(e.tabs)
}

// Cursor and editing
func (e *Editor) ensureCursorVisible(tab *Tab) {
	_, _, _, height := e.GetInnerRect()
	height -= 1 // tab bar

	if tab.CursorRow < tab.ScrollRow {
		tab.ScrollRow = tab.CursorRow
	}
	if tab.CursorRow >= tab.ScrollRow+height {
		tab.ScrollRow = tab.CursorRow - height + 1
	}
}

func (e *Editor) clampCursor(tab *Tab) {
	if tab.CursorRow < 0 {
		tab.CursorRow = 0
	}
	if tab.CursorRow >= tab.Buffer.LineCount() {
		tab.CursorRow = tab.Buffer.LineCount() - 1
	}
	lineLen := len(tab.Buffer.Line(tab.CursorRow))
	if tab.CursorCol > lineLen {
		tab.CursorCol = lineLen
	}
	if tab.CursorCol < 0 {
		tab.CursorCol = 0
	}
}

func (e *Editor) clearSelection(tab *Tab) {
	tab.HasSelect = false
	tab.SelectStart = [2]int{-1, -1}
	tab.SelectEnd = [2]int{-1, -1}
}

func (e *Editor) startSelection(tab *Tab) {
	if !tab.HasSelect {
		tab.HasSelect = true
		tab.SelectStart = [2]int{tab.CursorRow, tab.CursorCol}
		tab.SelectEnd = [2]int{tab.CursorRow, tab.CursorCol}
	}
}

func (e *Editor) updateSelectionEnd(tab *Tab) {
	tab.SelectEnd = [2]int{tab.CursorRow, tab.CursorCol}
}

// selectionRange returns ordered start/end of selection
func (e *Editor) selectionRange(tab *Tab) (int, int, int, int) {
	sr, sc := tab.SelectStart[0], tab.SelectStart[1]
	er, ec := tab.SelectEnd[0], tab.SelectEnd[1]
	if sr > er || (sr == er && sc > ec) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	return sr, sc, er, ec
}

func (e *Editor) selectedText(tab *Tab) string {
	if !tab.HasSelect {
		return ""
	}
	sr, sc, er, ec := e.selectionRange(tab)
	var sb strings.Builder
	for r := sr; r <= er; r++ {
		line := tab.Buffer.Line(r)
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
		sb.WriteString(line[startCol:endCol])
		if r < er {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (e *Editor) deleteSelection(tab *Tab) {
	if !tab.HasSelect {
		return
	}
	sr, sc, er, ec := e.selectionRange(tab)
	tab.Buffer.Delete(sr, sc, er, ec, [2]int{tab.CursorRow, tab.CursorCol})
	tab.CursorRow = sr
	tab.CursorCol = sc
	e.clearSelection(tab)
}

// Clipboard operations
func (e *Editor) clipboardCopy(text string) {
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
	cmd.Run()
}

func (e *Editor) clipboardPaste() string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	default:
		return ""
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	result := string(out)
	result = strings.ReplaceAll(result, "\r\n", "\n")
	result = strings.ReplaceAll(result, "\r", "\n")
	return result
}

// wordBoundaryLeft finds the column of the start of the word to the left
func wordBoundaryLeft(line string, col int) int {
	if col <= 0 {
		return 0
	}
	runes := []rune(line)
	if col > len(runes) {
		col = len(runes)
	}
	i := col - 1
	// Skip whitespace
	for i > 0 && unicode.IsSpace(runes[i]) {
		i--
	}
	// Skip word chars
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	return i
}

// wordBoundaryRight finds the column of the end of the word to the right
func wordBoundaryRight(line string, col int) int {
	runes := []rune(line)
	if col >= len(runes) {
		return len(runes)
	}
	i := col
	// Skip current word
	for i < len(runes) && !unicode.IsSpace(runes[i]) {
		i++
	}
	// Skip whitespace
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	return i
}

// HandleAction processes an editor action
func (e *Editor) HandleAction(action Action, ch rune) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}

	switch action {
	case ActionCursorLeft:
		e.clearSelection(tab)
		if tab.CursorCol > 0 {
			tab.CursorCol--
		} else if tab.CursorRow > 0 {
			tab.CursorRow--
			tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
		}
	case ActionCursorRight:
		e.clearSelection(tab)
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			tab.CursorCol++
		} else if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
			tab.CursorCol = 0
		}
	case ActionCursorUp:
		e.clearSelection(tab)
		if tab.CursorRow > 0 {
			tab.CursorRow--
		}
	case ActionCursorDown:
		e.clearSelection(tab)
		if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
		}
	case ActionCursorHome:
		e.clearSelection(tab)
		tab.CursorCol = 0
	case ActionCursorEnd:
		e.clearSelection(tab)
		tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
	case ActionCursorPageUp:
		e.clearSelection(tab)
		_, _, _, h := e.GetInnerRect()
		tab.CursorRow -= h - 1
		if tab.CursorRow < 0 {
			tab.CursorRow = 0
		}
	case ActionCursorPageDown:
		e.clearSelection(tab)
		_, _, _, h := e.GetInnerRect()
		tab.CursorRow += h - 1
		if tab.CursorRow >= tab.Buffer.LineCount() {
			tab.CursorRow = tab.Buffer.LineCount() - 1
		}
	case ActionCursorWordLeft:
		e.clearSelection(tab)
		if tab.CursorCol > 0 {
			tab.CursorCol = wordBoundaryLeft(tab.Buffer.Line(tab.CursorRow), tab.CursorCol)
		} else if tab.CursorRow > 0 {
			tab.CursorRow--
			tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
		}
	case ActionCursorWordRight:
		e.clearSelection(tab)
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			tab.CursorCol = wordBoundaryRight(tab.Buffer.Line(tab.CursorRow), tab.CursorCol)
		} else if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
			tab.CursorCol = 0
		}
	case ActionCursorDocStart:
		e.clearSelection(tab)
		tab.CursorRow = 0
		tab.CursorCol = 0
	case ActionCursorDocEnd:
		e.clearSelection(tab)
		tab.CursorRow = tab.Buffer.LineCount() - 1
		tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))

	// Selection
	case ActionSelectLeft:
		e.startSelection(tab)
		if tab.CursorCol > 0 {
			tab.CursorCol--
		} else if tab.CursorRow > 0 {
			tab.CursorRow--
			tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
		}
		e.updateSelectionEnd(tab)
	case ActionSelectRight:
		e.startSelection(tab)
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			tab.CursorCol++
		} else if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
			tab.CursorCol = 0
		}
		e.updateSelectionEnd(tab)
	case ActionSelectUp:
		e.startSelection(tab)
		if tab.CursorRow > 0 {
			tab.CursorRow--
		}
		e.updateSelectionEnd(tab)
	case ActionSelectDown:
		e.startSelection(tab)
		if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
		}
		e.updateSelectionEnd(tab)
	case ActionSelectHome:
		e.startSelection(tab)
		tab.CursorCol = 0
		e.updateSelectionEnd(tab)
	case ActionSelectEnd:
		e.startSelection(tab)
		tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
		e.updateSelectionEnd(tab)
	case ActionSelectPageUp:
		e.startSelection(tab)
		_, _, _, h := e.GetInnerRect()
		tab.CursorRow -= h - 1
		if tab.CursorRow < 0 {
			tab.CursorRow = 0
		}
		e.updateSelectionEnd(tab)
	case ActionSelectPageDown:
		e.startSelection(tab)
		_, _, _, h := e.GetInnerRect()
		tab.CursorRow += h - 1
		if tab.CursorRow >= tab.Buffer.LineCount() {
			tab.CursorRow = tab.Buffer.LineCount() - 1
		}
		e.updateSelectionEnd(tab)
	case ActionSelectWordLeft:
		e.startSelection(tab)
		if tab.CursorCol > 0 {
			tab.CursorCol = wordBoundaryLeft(tab.Buffer.Line(tab.CursorRow), tab.CursorCol)
		} else if tab.CursorRow > 0 {
			tab.CursorRow--
			tab.CursorCol = len(tab.Buffer.Line(tab.CursorRow))
		}
		e.updateSelectionEnd(tab)
	case ActionSelectWordRight:
		e.startSelection(tab)
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			tab.CursorCol = wordBoundaryRight(tab.Buffer.Line(tab.CursorRow), tab.CursorCol)
		} else if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
			tab.CursorCol = 0
		}
		e.updateSelectionEnd(tab)
	case ActionSelectAll:
		tab.HasSelect = true
		tab.SelectStart = [2]int{0, 0}
		lastLine := tab.Buffer.LineCount() - 1
		tab.SelectEnd = [2]int{lastLine, len(tab.Buffer.Line(lastLine))}
		tab.CursorRow = lastLine
		tab.CursorCol = len(tab.Buffer.Line(lastLine))

	// Editing
	case ActionInsertChar:
		if tab.HasSelect {
			e.deleteSelection(tab)
		}
		cursor := [2]int{tab.CursorRow, tab.CursorCol}
		newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, string(ch), cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]
	case ActionInsertNewline:
		if tab.HasSelect {
			e.deleteSelection(tab)
		}
		cursor := [2]int{tab.CursorRow, tab.CursorCol}
		newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, "\n", cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]
	case ActionInsertTab:
		if tab.HasSelect {
			e.deleteSelection(tab)
		}
		cursor := [2]int{tab.CursorRow, tab.CursorCol}
		newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, "    ", cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]
	case ActionDeleteChar:
		if tab.HasSelect {
			e.deleteSelection(tab)
		} else {
			lineLen := len(tab.Buffer.Line(tab.CursorRow))
			if tab.CursorCol < lineLen {
				tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, tab.CursorCol+1, [2]int{tab.CursorRow, tab.CursorCol})
			} else if tab.CursorRow < tab.Buffer.LineCount()-1 {
				tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow+1, 0, [2]int{tab.CursorRow, tab.CursorCol})
			}
		}
	case ActionBackspace:
		if tab.HasSelect {
			e.deleteSelection(tab)
		} else if tab.CursorCol > 0 {
			tab.Buffer.Delete(tab.CursorRow, tab.CursorCol-1, tab.CursorRow, tab.CursorCol, [2]int{tab.CursorRow, tab.CursorCol})
			tab.CursorCol--
		} else if tab.CursorRow > 0 {
			prevLineLen := len(tab.Buffer.Line(tab.CursorRow - 1))
			tab.Buffer.Delete(tab.CursorRow-1, prevLineLen, tab.CursorRow, 0, [2]int{tab.CursorRow, tab.CursorCol})
			tab.CursorRow--
			tab.CursorCol = prevLineLen
		}
	case ActionDeleteWord:
		if tab.HasSelect {
			e.deleteSelection(tab)
		} else {
			endCol := wordBoundaryRight(tab.Buffer.Line(tab.CursorRow), tab.CursorCol)
			if endCol > tab.CursorCol {
				tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, endCol, [2]int{tab.CursorRow, tab.CursorCol})
			}
		}
	case ActionDeleteLine:
		if tab.Buffer.LineCount() > 1 {
			endRow := tab.CursorRow
			if endRow < tab.Buffer.LineCount()-1 {
				tab.Buffer.Delete(tab.CursorRow, 0, tab.CursorRow+1, 0, [2]int{tab.CursorRow, tab.CursorCol})
			} else {
				// Last line - delete to end of prev line
				if tab.CursorRow > 0 {
					prevLen := len(tab.Buffer.Line(tab.CursorRow - 1))
					tab.Buffer.Delete(tab.CursorRow-1, prevLen, tab.CursorRow, len(tab.Buffer.Line(tab.CursorRow)), [2]int{tab.CursorRow, tab.CursorCol})
					tab.CursorRow--
				} else {
					tab.Buffer.Delete(0, 0, 0, len(tab.Buffer.Line(0)), [2]int{tab.CursorRow, tab.CursorCol})
				}
			}
			tab.CursorCol = 0
		}

	// Clipboard
	case ActionCopy:
		if tab.HasSelect {
			e.clipboardCopy(e.selectedText(tab))
		}
	case ActionCut:
		if tab.HasSelect {
			e.clipboardCopy(e.selectedText(tab))
			e.deleteSelection(tab)
		}
	case ActionPaste:
		text := e.clipboardPaste()
		if text != "" {
			if tab.HasSelect {
				e.deleteSelection(tab)
			}
			cursor := [2]int{tab.CursorRow, tab.CursorCol}
			newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, text, cursor)
			tab.CursorRow = newPos[0]
			tab.CursorCol = newPos[1]
		}

	// Undo/Redo
	case ActionUndo:
		if pos, ok := tab.Buffer.Undo(); ok {
			tab.CursorRow = pos[0]
			tab.CursorCol = pos[1]
			e.clearSelection(tab)
		}
	case ActionRedo:
		if pos, ok := tab.Buffer.Redo(); ok {
			tab.CursorRow = pos[0]
			tab.CursorCol = pos[1]
			e.clearSelection(tab)
		}
	}

	e.clampCursor(tab)
	e.ensureCursorVisible(tab)
	e.notifyChange()
}

// Find searches for text and positions cursor
func (e *Editor) Find(query string, forward bool) bool {
	tab := e.ActiveTab()
	if tab == nil || query == "" {
		return false
	}

	startRow := tab.CursorRow
	startCol := tab.CursorCol
	if forward {
		startCol++
	}

	lowerQuery := strings.ToLower(query)

	if forward {
		for r := startRow; r < tab.Buffer.LineCount(); r++ {
			line := strings.ToLower(tab.Buffer.Line(r))
			sc := 0
			if r == startRow {
				sc = startCol
			}
			if sc > len(line) {
				continue
			}
			idx := strings.Index(line[sc:], lowerQuery)
			if idx >= 0 {
				tab.CursorRow = r
				tab.CursorCol = sc + idx
				tab.HasSelect = true
				tab.SelectStart = [2]int{r, sc + idx}
				tab.SelectEnd = [2]int{r, sc + idx + len(query)}
				e.ensureCursorVisible(tab)
				e.notifyChange()
				return true
			}
		}
		// Wrap around
		for r := 0; r <= startRow; r++ {
			line := strings.ToLower(tab.Buffer.Line(r))
			idx := strings.Index(line, lowerQuery)
			if idx >= 0 {
				tab.CursorRow = r
				tab.CursorCol = idx
				tab.HasSelect = true
				tab.SelectStart = [2]int{r, idx}
				tab.SelectEnd = [2]int{r, idx + len(query)}
				e.ensureCursorVisible(tab)
				e.notifyChange()
				return true
			}
		}
	}

	return false
}

// Replace replaces selected text and finds next
func (e *Editor) Replace(find, replace string) bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}
	if tab.HasSelect {
		sel := e.selectedText(tab)
		if strings.EqualFold(sel, find) {
			e.deleteSelection(tab)
			cursor := [2]int{tab.CursorRow, tab.CursorCol}
			newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, replace, cursor)
			tab.CursorRow = newPos[0]
			tab.CursorCol = newPos[1]
		}
	}
	return e.Find(find, true)
}

// ReplaceAll replaces all occurrences
func (e *Editor) ReplaceAll(find, replace string) int {
	tab := e.ActiveTab()
	if tab == nil {
		return 0
	}
	text := tab.Buffer.Text()
	count := strings.Count(strings.ToLower(text), strings.ToLower(find))
	if count == 0 {
		return 0
	}
	// Simple case-insensitive replace
	newText := caseInsensitiveReplace(text, find, replace)
	tab.Buffer = NewBufferFromText(newText)
	tab.Buffer.SetModified(true)
	tab.CursorRow = 0
	tab.CursorCol = 0
	e.clearSelection(tab)
	e.notifyChange()
	return count
}

func caseInsensitiveReplace(s, old, new string) string {
	lower := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	var result strings.Builder
	idx := 0
	for {
		pos := strings.Index(lower[idx:], lowerOld)
		if pos == -1 {
			result.WriteString(s[idx:])
			break
		}
		result.WriteString(s[idx : idx+pos])
		result.WriteString(new)
		idx += pos + len(old)
	}
	return result.String()
}

// GoToLine moves cursor to a specific line number (1-based)
func (e *Editor) GoToLine(lineNum int) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	lineNum-- // Convert to 0-based
	if lineNum < 0 {
		lineNum = 0
	}
	if lineNum >= tab.Buffer.LineCount() {
		lineNum = tab.Buffer.LineCount() - 1
	}
	tab.CursorRow = lineNum
	tab.CursorCol = 0
	e.clearSelection(tab)
	e.ensureCursorVisible(tab)
	e.notifyChange()
}

// Draw renders the editor
func (e *Editor) Draw(screen tcell.Screen) {
	e.Box.DrawForSubclass(screen, e)
	x, y, width, height := e.GetInnerRect()

	if len(e.tabs) == 0 {
		e.drawWelcome(screen, x, y, width, height)
		return
	}

	// Draw tab bar
	e.drawTabBar(screen, x, y, width)
	y++
	height--

	tab := e.ActiveTab()
	if tab == nil {
		return
	}

	// Calculate gutter width
	gutterW := GutterWidth(tab.Buffer.LineCount())

	// Highlight all lines (cached)
	if e.cachedHLVer != e.hlVersion || e.cachedHL == nil {
		e.cachedHL = tab.Highlighter.Highlight(tab.Buffer.Text())
		e.cachedHLVer = e.hlVersion
	}
	highlighted := e.cachedHL

	// Draw gutter + editor content
	for row := 0; row < height; row++ {
		lineIdx := tab.ScrollRow + row

		// Clear the line
		for cx := x; cx < x+width; cx++ {
			screen.SetContent(cx, y+row, ' ', nil, tcell.StyleDefault.Background(ui.ColorBg))
		}

		if lineIdx >= tab.Buffer.LineCount() {
			// Draw tilde for empty lines
			screen.SetContent(x+gutterW, y+row, '~', nil, tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBg))
			continue
		}

		// Draw gutter
		gutterStr := FormatGutterLine(lineIdx+1, tab.Buffer.LineCount())
		for i, ch := range gutterStr {
			if x+i < x+gutterW {
				screen.SetContent(x+i, y+row, ch, nil, tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg))
			}
		}

		// Draw line content with syntax highlighting
		line := tab.Buffer.Line(lineIdx)
		editorX := x + gutterW

		if lineIdx < len(highlighted) {
			// Use tview's tagged text drawing - but we'll manually draw for more control
			e.drawHighlightedLine(screen, editorX, y+row, width-gutterW, line, highlighted[lineIdx], lineIdx, tab)
		} else {
			e.drawPlainLine(screen, editorX, y+row, width-gutterW, line, lineIdx, tab)
		}
	}

	// Draw cursor only when focused
	if e.hasFocus {
		cursorScreenX := x + gutterW + tab.CursorCol - tab.ScrollCol
		cursorScreenY := y + tab.CursorRow - tab.ScrollRow
		if cursorScreenY >= y && cursorScreenY < y+height && cursorScreenX >= x+gutterW && cursorScreenX < x+width {
			screen.ShowCursor(cursorScreenX, cursorScreenY)
		}
	}
}

func (e *Editor) drawTabBar(screen tcell.Screen, x, y, width int) {
	// Clear tab bar
	for cx := x; cx < x+width; cx++ {
		screen.SetContent(cx, y, ' ', nil, tcell.StyleDefault.Background(ui.ColorTabBarBg))
	}

	cx := x + 1
	for i, tab := range e.tabs {
		name := tab.Name
		if tab.Buffer.Modified() {
			name = "*" + name
		}
		label := " " + name + " "

		fg := ui.ColorTabInactive
		bg := ui.ColorTabBarBg
		if i == e.activeTab {
			fg = ui.ColorTabActive
			bg = ui.ColorTabActiveBg
		}

		for _, ch := range label {
			if cx < x+width {
				screen.SetContent(cx, y, ch, nil, tcell.StyleDefault.Foreground(fg).Background(bg))
				cx++
			}
		}
		// Separator
		if cx < x+width {
			screen.SetContent(cx, y, '│', nil, tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorTabBarBg))
			cx++
		}
	}
}

func (e *Editor) drawHighlightedLine(screen tcell.Screen, x, y, maxWidth int, rawLine string, hl HighlightedLine, lineIdx int, tab *Tab) {
	for i, ch := range rawLine {
		sx := x + i - tab.ScrollCol
		if sx < x {
			continue
		}
		if sx >= x+maxWidth {
			break
		}

		fg := ui.ColorText
		bold := false
		if i < len(hl.Styles) {
			fg = hl.Styles[i].Fg
			bold = hl.Styles[i].Bold
		}

		style := tcell.StyleDefault.Foreground(fg).Background(ui.ColorBg)
		if bold {
			style = style.Bold(true)
		}

		// Selection overrides colors
		if tab.HasSelect && e.isInSelection(tab, lineIdx, i) {
			style = tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
		}

		screen.SetContent(sx, y, ch, nil, style)
	}
}

func (e *Editor) drawPlainLine(screen tcell.Screen, x, y, maxWidth int, line string, lineIdx int, tab *Tab) {
	for i, ch := range line {
		sx := x + i - tab.ScrollCol
		if sx < x || sx >= x+maxWidth {
			continue
		}
		style := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
		if tab.HasSelect && e.isInSelection(tab, lineIdx, i) {
			style = style.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
		}
		screen.SetContent(sx, y, ch, nil, style)
	}
}

func (e *Editor) isInSelection(tab *Tab, row, col int) bool {
	if !tab.HasSelect {
		return false
	}
	sr, sc, er, ec := e.selectionRange(tab)
	if row < sr || row > er {
		return false
	}
	if row == sr && row == er {
		return col >= sc && col < ec
	}
	if row == sr {
		return col >= sc
	}
	if row == er {
		return col < ec
	}
	return true
}

func (e *Editor) drawWelcome(screen tcell.Screen, x, y, width, height int) {
	lines := []string{
		"",
		"  _   _ _   _ __  __ _____ _   _",
		" | \\ | | | | |  \\/  | ____| \\ | |",
		" |  \\| | | | | |\\/| |  _| |  \\| |",
		" | |\\  | |_| | |  | | |___| |\\  |",
		" |_| \\_|\\___/|_|  |_|_____|_| \\_|",
		"            T E X T",
		"",
		"      A Modern Terminal IDE",
		"",
		"  Ctrl+N  New File    Ctrl+O  Open File",
		"  Ctrl+S  Save        Ctrl+Q  Quit",
		"  F5      Run         F9      Build",
		"  Ctrl+F  Find        Ctrl+G  Go to Line",
		"  F10     Menu Bar",
		"",
		"       Inspired by Borland C++",
	}

	startY := y + (height-len(lines))/2
	for i, line := range lines {
		row := startY + i
		if row < y || row >= y+height {
			continue
		}
		startX := x + (width-len(line))/2
		for j, ch := range line {
			cx := startX + j
			if cx >= x && cx < x+width {
				screen.SetContent(cx, row, ch, nil, tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBg))
			}
		}
	}
}

// InputHandler handles key events
func (e *Editor) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return e.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		action := MapKey(event)
		if action != ActionNone {
			e.HandleAction(action, event.Rune())
		}
	})
}

// MouseHandler handles mouse events
func (e *Editor) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return e.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if !e.InRect(event.Position()) {
			return false, nil
		}

		mx, my := event.Position()
		bx, by, _, _ := e.GetInnerRect()

		tab := e.ActiveTab()
		if tab == nil {
			return false, nil
		}

		// Check tab bar click
		if my == by {
			// Tab bar area
			if action == tview.MouseLeftClick {
				e.handleTabBarClick(mx-bx, tab)
				return true, nil
			}
			return false, nil
		}

		gutterW := GutterWidth(tab.Buffer.LineCount())
		editorX := mx - bx - gutterW
		editorY := my - by - 1 // -1 for tab bar

		if editorX < 0 {
			return false, nil
		}

		switch action {
		case tview.MouseLeftClick:
			setFocus(e)
			row := tab.ScrollRow + editorY
			col := editorX + tab.ScrollCol
			if row >= 0 && row < tab.Buffer.LineCount() {
				tab.CursorRow = row
				tab.CursorCol = col
				e.clampCursor(tab)
				e.clearSelection(tab)
				e.notifyChange()
			}
			return true, nil
		case tview.MouseScrollUp:
			if tab.ScrollRow > 0 {
				tab.ScrollRow -= 3
				if tab.ScrollRow < 0 {
					tab.ScrollRow = 0
				}
				e.notifyChange()
			}
			return true, nil
		case tview.MouseScrollDown:
			if tab.ScrollRow < tab.Buffer.LineCount()-1 {
				tab.ScrollRow += 3
				e.notifyChange()
			}
			return true, nil
		}

		return false, nil
	})
}

func (e *Editor) handleTabBarClick(relX int, tab *Tab) {
	cx := 1
	for i, t := range e.tabs {
		name := t.Name
		if t.Buffer.Modified() {
			name = "*" + name
		}
		labelLen := len(name) + 2 // spaces
		if relX >= cx && relX < cx+labelLen {
			e.activeTab = i
			e.notifyTabChange()
			e.notifyChange()
			return
		}
		cx += labelLen + 1 // +1 for separator
	}
}
