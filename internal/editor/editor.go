package editor

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/editor/keymode"
	"numentext/internal/ui"
)

// Line ending constants
const (
	LineEndingLF   = "\n"
	LineEndingCRLF = "\r\n"
	LineEndingCR   = "\r"
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
	LineEnding  string // "\n", "\r\n", or "\r"
	HasBOM      bool   // true if file had UTF-8 BOM
	Bookmarks   map[int]bool // set of bookmarked line indices (0-based)
}

// MaxBookmarks is the maximum number of bookmarks allowed per tab.
const MaxBookmarks = 50

// Editor is the core editor component
type Editor struct {
	*tview.Box
	tabs        []*Tab
	activeTab   int
	onChange    func()
	onTabChange func()
	hasFocus       bool
	showLineNumbers bool
	wordWrap        bool
	tabSize         int
	cachedHL       []HighlightedLine
	cachedHLVer    int
	hlVersion      int

	// LSP callbacks
	onFileOpen   func(filePath, text string)
	onFileChange func(filePath, text string)
	onFileClose  func(filePath string)

	// Completion
	completion         *CompletionPopup
	onRequestComplete  func(filePath string, row, col int, callback func([]CompletionItem))

	// Diagnostics: filePath -> line -> severity (1=error, 2=warning, 3=info, 4=hint)
	diagnostics map[string]map[int]DiagnosticInfo

	// Build error diagnostics (separate from LSP, take precedence in gutter)
	buildDiags map[string]map[int]DiagnosticInfo

	// Breakpoint check callback (returns true if breakpoint at 1-based line)
	hasBreakpoint func(filePath string, line int) bool

	// Bracket matching (computed per Draw call)
	bracketMatch BracketMatch

	// Keyboard mode
	keyMode keymode.KeyMapper

	// Status message callback (for showing messages in status bar)
	onStatusMessage func(msg string)

	// Action callbacks for actions that need app-level handling
	onSearchForward func()
	onSearchNext    func()
	onSearchPrev    func()

	// Last search query for n/N repeat
	lastSearch string

	// Breadcrumb: document symbols from LSP
	breadcrumbSymbols []BreadcrumbSymbol
	breadcrumbFile    string // file path for cached symbols
}

// BreadcrumbSymbol represents a symbol for breadcrumb display.
type BreadcrumbSymbol struct {
	Name      string
	Kind      int
	StartLine int
	EndLine   int
	Children  []BreadcrumbSymbol
}

// DiagnosticInfo holds diagnostic data for a single line
type DiagnosticInfo struct {
	Severity int    // 1=error, 2=warning, 3=info, 4=hint
	Message  string
}

func NewEditor() *Editor {
	e := &Editor{
		Box:             tview.NewBox(),
		tabs:            []*Tab{},
		showLineNumbers: true,
		tabSize:         4,
		completion:      NewCompletionPopup(),
		diagnostics:     make(map[string]map[int]DiagnosticInfo),
		buildDiags:      make(map[string]map[int]DiagnosticInfo),
		keyMode:         keymode.NewDefaultMode(),
	}
	e.SetBorder(false)
	return e
}

func (e *Editor) SetShowLineNumbers(show bool) {
	e.showLineNumbers = show
}

func (e *Editor) ShowLineNumbers() bool {
	return e.showLineNumbers
}

func (e *Editor) SetWordWrap(on bool) {
	e.wordWrap = on
	e.hlVersion++ // force redraw
}

func (e *Editor) WordWrap() bool {
	return e.wordWrap
}

// SetTabSize sets the tab display width
func (e *Editor) SetTabSize(size int) {
	if size < 1 {
		size = 1
	}
	e.tabSize = size
}

// TabSize returns the configured tab size.
func (e *Editor) TabSize() int {
	return e.tabSize
}

// LineEndingLabel returns a display label for the active tab's line ending (LF, CRLF, CR).
func (e *Editor) LineEndingLabel() string {
	tab := e.ActiveTab()
	if tab == nil {
		return ""
	}
	switch tab.LineEnding {
	case LineEndingCRLF:
		return "CRLF"
	case LineEndingCR:
		return "CR"
	default:
		return "LF"
	}
}

// SetLineEnding changes the line ending for the active tab.
func (e *Editor) SetLineEnding(le string) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	tab.LineEnding = le
	tab.Buffer.SetModified(true)
	e.notifyChange()
}

// HasBOM returns whether the active tab has a UTF-8 BOM.
func (e *Editor) HasBOM() bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}
	return tab.HasBOM
}

// SetBOM sets or clears the UTF-8 BOM for the active tab.
func (e *Editor) SetBOM(hasBOM bool) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	tab.HasBOM = hasBOM
	tab.Buffer.SetModified(true)
	e.notifyChange()
}

// ConvertTabsToSpaces converts tab characters to spaces in the buffer.
// If there is a selection, only converts within the selection; otherwise converts entire file.
func (e *Editor) ConvertTabsToSpaces() {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	spaces := strings.Repeat(" ", e.tabSize)
	if tab.HasSelect {
		startRow, endRow := tab.SelectStart[0], tab.SelectEnd[0]
		if startRow > endRow {
			startRow, endRow = endRow, startRow
		}
		for row := startRow; row <= endRow && row < tab.Buffer.LineCount(); row++ {
			line := tab.Buffer.Line(row)
			newLine := strings.ReplaceAll(line, "\t", spaces)
			if newLine != line {
				tab.Buffer.ReplaceLine(row, newLine)
			}
		}
	} else {
		for row := 0; row < tab.Buffer.LineCount(); row++ {
			line := tab.Buffer.Line(row)
			newLine := strings.ReplaceAll(line, "\t", spaces)
			if newLine != line {
				tab.Buffer.ReplaceLine(row, newLine)
			}
		}
	}
	e.hlVersion++
	e.notifyChange()
}

// ConvertSpacesToTabs converts runs of tabSize spaces to tab characters.
// If there is a selection, only converts within the selection; otherwise converts entire file.
func (e *Editor) ConvertSpacesToTabs() {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	spaces := strings.Repeat(" ", e.tabSize)
	if tab.HasSelect {
		startRow, endRow := tab.SelectStart[0], tab.SelectEnd[0]
		if startRow > endRow {
			startRow, endRow = endRow, startRow
		}
		for row := startRow; row <= endRow && row < tab.Buffer.LineCount(); row++ {
			line := tab.Buffer.Line(row)
			newLine := strings.ReplaceAll(line, spaces, "\t")
			if newLine != line {
				tab.Buffer.ReplaceLine(row, newLine)
			}
		}
	} else {
		for row := 0; row < tab.Buffer.LineCount(); row++ {
			line := tab.Buffer.Line(row)
			newLine := strings.ReplaceAll(line, spaces, "\t")
			if newLine != line {
				tab.Buffer.ReplaceLine(row, newLine)
			}
		}
	}
	e.hlVersion++
	e.notifyChange()
}

// byteOffsetToScreenCol converts a byte offset in a line to its screen column,
// accounting for multi-byte UTF-8 characters and tab expansion.
func byteOffsetToScreenCol(line string, byteOff, tabSize int) int {
	if tabSize < 1 {
		tabSize = 4
	}
	col := 0
	for i, ch := range line {
		if i >= byteOff {
			break
		}
		if ch == '\t' {
			col += tabSize - (col % tabSize)
		} else {
			col++
		}
	}
	return col
}

// screenColToByteOffset converts a screen column to the corresponding byte offset,
// accounting for multi-byte UTF-8 characters and tab expansion.
func screenColToByteOffset(line string, screenCol, tabSize int) int {
	if tabSize < 1 {
		tabSize = 4
	}
	col := 0
	for i, ch := range line {
		if col >= screenCol {
			return i
		}
		if ch == '\t' {
			col += tabSize - (col % tabSize)
		} else {
			col++
		}
	}
	return len(line)
}

// wrapSegment represents one visual line within a wrapped logical line.
type wrapSegment struct {
	startCol int
	endCol   int
}

// wrapLine splits a logical line into visual segments of at most maxWidth screen columns.
// It accounts for multi-byte UTF-8 characters and tab expansion.
// Returned startCol/endCol are byte offsets for compatibility with downstream code.
func wrapLine(line string, maxWidth, tabSize int) []wrapSegment {
	if maxWidth < 1 {
		maxWidth = 1
	}
	if tabSize < 1 {
		tabSize = 4
	}
	if len(line) == 0 {
		return []wrapSegment{{0, 0}}
	}

	// Build a lookup of rune index -> byte offset and visual width info
	type runeInfo struct {
		byteOff  int
		ch       rune
		runeSize int
	}
	var runes []runeInfo
	for i, ch := range line {
		runes = append(runes, runeInfo{i, ch, utf8.RuneLen(ch)})
	}

	var segs []wrapSegment
	startRune := 0
	for startRune < len(runes) {
		visualCol := 0
		lastBreak := -1 // rune index of last break point (break after this rune)
		endRune := startRune

		for endRune < len(runes) {
			ch := runes[endRune].ch
			var charWidth int
			if ch == '\t' {
				charWidth = tabSize - (visualCol % tabSize)
			} else {
				charWidth = 1
			}

			if visualCol+charWidth > maxWidth && endRune > startRune {
				break
			}

			visualCol += charWidth

			if ch == ' ' || ch == '\t' || ch == '-' || ch == ',' || ch == ';' ||
				ch == '.' || ch == ':' || ch == '/' || ch == '\\' || ch == ')' ||
				ch == ']' || ch == '}' {
				lastBreak = endRune
			}

			endRune++

			if visualCol >= maxWidth {
				break
			}
		}

		if endRune >= len(runes) {
			// Rest of line fits
			segs = append(segs, wrapSegment{runes[startRune].byteOff, len(line)})
			break
		}

		// Try to break at last break point
		breakRune := endRune
		if lastBreak >= startRune && lastBreak+1 < endRune {
			breakRune = lastBreak + 1
		}

		endByteOff := runes[breakRune].byteOff
		segs = append(segs, wrapSegment{runes[startRune].byteOff, endByteOff})
		startRune = breakRune
	}
	return segs
}

func (e *Editor) SetOnChange(fn func()) {
	e.onChange = fn
}

func (e *Editor) SetOnTabChange(fn func()) {
	e.onTabChange = fn
}

func (e *Editor) SetOnFileOpen(fn func(filePath, text string)) {
	e.onFileOpen = fn
}

func (e *Editor) SetOnFileChange(fn func(filePath, text string)) {
	e.onFileChange = fn
}

func (e *Editor) SetOnFileClose(fn func(filePath string)) {
	e.onFileClose = fn
}

func (e *Editor) SetOnRequestComplete(fn func(filePath string, row, col int, callback func([]CompletionItem))) {
	e.onRequestComplete = fn
}

// Completion returns the completion popup for external drawing
func (e *Editor) Completion() *CompletionPopup {
	return e.completion
}

// notifyChange signals that file content has changed (edits, not cursor moves).
func (e *Editor) notifyChange() {
	e.hlVersion++
	if e.onChange != nil {
		e.onChange()
	}
	if e.onFileChange != nil {
		tab := e.ActiveTab()
		if tab != nil && tab.FilePath != "" {
			e.onFileChange(tab.FilePath, tab.Buffer.Text())
		}
	}
}

// notifyCursorMove signals that only the cursor position changed (no content change).
func (e *Editor) notifyCursorMove() {
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

// SetHasBreakpoint sets the callback to check for breakpoints
func (e *Editor) SetHasBreakpoint(fn func(filePath string, line int) bool) {
	e.hasBreakpoint = fn
}

// KeyMode returns the current key mapper
func (e *Editor) KeyMode() keymode.KeyMapper {
	return e.keyMode
}

// SetKeyMode sets the keyboard mode
func (e *Editor) SetKeyMode(km keymode.KeyMapper) {
	e.keyMode = km
}

// KeyModeContext builds a KeyContext from the current editor state
func (e *Editor) KeyModeContext() keymode.KeyContext {
	tab := e.ActiveTab()
	if tab == nil {
		return keymode.KeyContext{}
	}
	_, _, _, height := e.GetInnerRect()
	return keymode.KeyContext{
		CursorRow:    tab.CursorRow,
		CursorCol:    tab.CursorCol,
		LineLen:      len(tab.Buffer.Line(tab.CursorRow)),
		LineCount:    tab.Buffer.LineCount(),
		CurrentLine:  tab.Buffer.Line(tab.CursorRow),
		HasSelection: tab.HasSelect,
		PageHeight:   height - 1, // minus tab bar
	}
}

// SetOnStatusMessage sets the callback for status bar messages
func (e *Editor) SetOnStatusMessage(fn func(string)) { e.onStatusMessage = fn }

// SetOnSearchForward sets the callback for / search action
func (e *Editor) SetOnSearchForward(fn func()) { e.onSearchForward = fn }

// SetOnSearchNext sets the callback for n search action
func (e *Editor) SetOnSearchNext(fn func()) { e.onSearchNext = fn }

// SetOnSearchPrev sets the callback for N search action
func (e *Editor) SetOnSearchPrev(fn func()) { e.onSearchPrev = fn }

// Diagnostics methods

// SetDiagnostics updates the diagnostics for a file
func (e *Editor) SetDiagnostics(filePath string, diags map[int]DiagnosticInfo) {
	if len(diags) == 0 {
		delete(e.diagnostics, filePath)
	} else {
		e.diagnostics[filePath] = diags
	}
}

// DiagnosticAtLine returns the diagnostic for the active file at the given line, if any
func (e *Editor) DiagnosticAtLine(line int) (DiagnosticInfo, bool) {
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return DiagnosticInfo{}, false
	}
	diags, ok := e.diagnostics[tab.FilePath]
	if !ok {
		return DiagnosticInfo{}, false
	}
	d, ok := diags[line]
	return d, ok
}

// SetBuildDiagnostics updates build error diagnostics for a file (separate from LSP).
func (e *Editor) SetBuildDiagnostics(filePath string, diags map[int]DiagnosticInfo) {
	if len(diags) == 0 {
		delete(e.buildDiags, filePath)
	} else {
		e.buildDiags[filePath] = diags
	}
}

// ClearAllBuildDiagnostics removes all build error markers.
func (e *Editor) ClearAllBuildDiagnostics() {
	e.buildDiags = make(map[string]map[int]DiagnosticInfo)
}

// Completion methods

func (e *Editor) maybeRequestCompletion(ch rune) {
	if e.onRequestComplete == nil {
		return
	}
	// Trigger on dot (member access) or after typing identifier chars
	if ch != '.' {
		return
	}
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	row := tab.CursorRow
	col := tab.CursorCol
	filePath := tab.FilePath
	e.onRequestComplete(filePath, row, col, func(items []CompletionItem) {
		if len(items) == 0 {
			return
		}
		e.completion.Show(items, "", row, col)
	})
}

func (e *Editor) updateCompletionPrefix() {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	// Get the text from startCol to cursor
	line := tab.Buffer.Line(tab.CursorRow)
	startCol := e.completion.StartCol()
	if startCol < 0 {
		startCol = 0
	}
	endCol := tab.CursorCol
	if endCol > len(line) {
		endCol = len(line)
	}
	if startCol > endCol {
		e.completion.Hide()
		return
	}
	prefix := line[startCol:endCol]
	e.completion.UpdatePrefix(prefix)
}

func (e *Editor) acceptCompletion() {
	item := e.completion.Selected()
	if item == nil {
		e.completion.Hide()
		return
	}
	tab := e.ActiveTab()
	if tab == nil {
		e.completion.Hide()
		return
	}

	// Replace from startCol to cursor with the completion text
	insertText := item.InsertText
	if insertText == "" {
		insertText = item.Label
	}
	startCol := e.completion.StartCol()
	line := tab.Buffer.Line(tab.CursorRow)

	// Build new line
	prefix := ""
	if startCol > 0 && startCol <= len(line) {
		prefix = line[:startCol]
	}
	suffix := ""
	if tab.CursorCol < len(line) {
		suffix = line[tab.CursorCol:]
	}
	newLine := prefix + insertText + suffix
	tab.Buffer.ReplaceLine(tab.CursorRow, newLine)
	tab.CursorCol = startCol + len(insertText)
	e.completion.Hide()
	e.notifyChange()
}

// Tab management
func (e *Editor) NewTab(name, filePath string, content string) {
	buf := NewBufferFromText(content)
	hl := NewHighlighter(filePath)
	// Default line ending: platform-dependent
	defaultLE := LineEndingLF
	if runtime.GOOS == "windows" {
		defaultLE = LineEndingCRLF
	}
	tab := &Tab{
		Name:        name,
		FilePath:    filePath,
		Buffer:      buf,
		Highlighter: hl,
		SelectStart: [2]int{-1, -1},
		SelectEnd:   [2]int{-1, -1},
		LineEnding:  defaultLE,
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

	// Detect UTF-8 BOM
	hasBOM := false
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		hasBOM = true
		data = data[3:] // strip BOM from content
	}

	content := string(data)

	// Detect line endings from raw content before normalizing
	lineEnding := detectLineEnding(content)

	// Normalize line endings for internal buffer
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	parts := strings.Split(filePath, "/")
	name := parts[len(parts)-1]
	e.NewTab(name, filePath, content)

	// Set detected line ending and BOM on the new tab
	tab := e.ActiveTab()
	if tab != nil {
		tab.LineEnding = lineEnding
		tab.HasBOM = hasBOM
	}

	if e.onFileOpen != nil && filePath != "" {
		e.onFileOpen(filePath, content)
	}
	return nil
}

// detectLineEnding examines content and returns the detected line ending.
// Checks the first few KB for \r\n, \r, or \n.
func detectLineEnding(content string) string {
	// Only scan first 8KB for performance
	scan := content
	if len(scan) > 8192 {
		scan = scan[:8192]
	}

	crlfCount := strings.Count(scan, "\r\n")
	// Count standalone \r (not part of \r\n)
	crCount := strings.Count(scan, "\r") - crlfCount
	lfCount := strings.Count(scan, "\n") - crlfCount

	if crlfCount > lfCount && crlfCount > crCount {
		return LineEndingCRLF
	}
	if crCount > lfCount && crCount > crlfCount {
		return LineEndingCR
	}
	// Default to LF (platform default set in NewTab for new files)
	return LineEndingLF
}

func (e *Editor) SaveCurrentFile() error {
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return fmt.Errorf("no file path")
	}
	// Buffer stores lines joined by \n internally
	content := tab.Buffer.Text()

	// Convert to tab's line ending format
	if tab.LineEnding == LineEndingCRLF {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	} else if tab.LineEnding == LineEndingCR {
		content = strings.ReplaceAll(content, "\n", "\r")
	}

	var data []byte
	// Prepend BOM if needed
	if tab.HasBOM {
		data = append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	} else {
		data = []byte(content)
	}

	err := os.WriteFile(tab.FilePath, data, 0644)
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

// ReloadCurrentFile re-reads the active tab's file from disk into the buffer.
func (e *Editor) ReloadCurrentFile() error {
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return fmt.Errorf("no file path")
	}
	data, err := os.ReadFile(tab.FilePath)
	if err != nil {
		return err
	}

	// Detect BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		tab.HasBOM = true
		data = data[3:]
	} else {
		tab.HasBOM = false
	}

	content := string(data)
	tab.LineEnding = detectLineEnding(content)
	tab.Buffer.SetText(content)
	tab.Buffer.SetModified(false)
	e.hlVersion++
	e.notifyChange()
	return nil
}

// SetCursorPos sets the cursor position for the active tab, clamped to valid range.
func (e *Editor) SetCursorPos(row, col int) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	lineCount := tab.Buffer.LineCount()
	if row >= lineCount {
		row = lineCount - 1
	}
	if row < 0 {
		row = 0
	}
	tab.CursorRow = row
	lineLen := len(tab.Buffer.Line(row))
	if col > lineLen {
		col = lineLen
	}
	if col < 0 {
		col = 0
	}
	tab.CursorCol = col
}

func (e *Editor) CloseTab(idx int) {
	if idx < 0 || idx >= len(e.tabs) {
		return
	}
	closedTab := e.tabs[idx]
	e.tabs = append(e.tabs[:idx], e.tabs[idx+1:]...)
	if e.activeTab >= len(e.tabs) {
		e.activeTab = len(e.tabs) - 1
	}
	if e.activeTab < 0 {
		e.activeTab = 0
	}
	if e.onFileClose != nil && closedTab.FilePath != "" {
		e.onFileClose(closedTab.FilePath)
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

// ToggleBookmark toggles a bookmark at the current cursor line.
// Returns true if bookmark was added, false if removed.
// Returns -1 if at limit and cannot add.
func (e *Editor) ToggleBookmark() int {
	tab := e.ActiveTab()
	if tab == nil {
		return -1
	}
	if tab.Bookmarks == nil {
		tab.Bookmarks = make(map[int]bool)
	}
	line := tab.CursorRow
	if tab.Bookmarks[line] {
		delete(tab.Bookmarks, line)
		return 0 // removed
	}
	if len(tab.Bookmarks) >= MaxBookmarks {
		return -1 // at limit
	}
	tab.Bookmarks[line] = true
	return 1 // added
}

// ToggleBookmarkAtLine toggles a bookmark at the given 0-based line index.
func (e *Editor) ToggleBookmarkAtLine(lineIdx int) int {
	tab := e.ActiveTab()
	if tab == nil {
		return -1
	}
	if tab.Bookmarks == nil {
		tab.Bookmarks = make(map[int]bool)
	}
	if lineIdx < 0 || lineIdx >= tab.Buffer.LineCount() {
		return -1
	}
	if tab.Bookmarks[lineIdx] {
		delete(tab.Bookmarks, lineIdx)
		return 0
	}
	if len(tab.Bookmarks) >= MaxBookmarks {
		return -1
	}
	tab.Bookmarks[lineIdx] = true
	return 1
}

// HasBookmark returns true if the given line has a bookmark.
func (e *Editor) HasBookmark(lineIdx int) bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}
	return tab.Bookmarks[lineIdx]
}

// BookmarkCount returns the number of bookmarks in the active tab.
func (e *Editor) BookmarkCount() int {
	tab := e.ActiveTab()
	if tab == nil {
		return 0
	}
	return len(tab.Bookmarks)
}

// SortedBookmarks returns sorted list of bookmarked line indices for the active tab.
func (e *Editor) SortedBookmarks() []int {
	tab := e.ActiveTab()
	if tab == nil {
		return nil
	}
	return sortedBookmarkLines(tab)
}

// sortedBookmarkLines returns sorted bookmarked lines for a tab.
func sortedBookmarkLines(tab *Tab) []int {
	if len(tab.Bookmarks) == 0 {
		return nil
	}
	lines := make([]int, 0, len(tab.Bookmarks))
	for line := range tab.Bookmarks {
		lines = append(lines, line)
	}
	// Simple insertion sort (bookmarks are small sets)
	for i := 1; i < len(lines); i++ {
		key := lines[i]
		j := i - 1
		for j >= 0 && lines[j] > key {
			lines[j+1] = lines[j]
			j--
		}
		lines[j+1] = key
	}
	return lines
}

// NextBookmark jumps to the next bookmark after cursor. Returns (lineNum 1-based, index 1-based, total) or (0,0,0) if none.
func (e *Editor) NextBookmark() (int, int, int) {
	tab := e.ActiveTab()
	if tab == nil {
		return 0, 0, 0
	}
	lines := sortedBookmarkLines(tab)
	if len(lines) == 0 {
		return 0, 0, 0
	}
	curLine := tab.CursorRow
	for i, l := range lines {
		if l > curLine {
			tab.CursorRow = l
			tab.CursorCol = 0
			e.ensureCursorVisible(tab)
			e.notifyCursorMove()
			return l + 1, i + 1, len(lines)
		}
	}
	// Wrap to first
	tab.CursorRow = lines[0]
	tab.CursorCol = 0
	e.ensureCursorVisible(tab)
	e.notifyCursorMove()
	return lines[0] + 1, 1, len(lines)
}

// PrevBookmark jumps to the previous bookmark before cursor. Returns (lineNum 1-based, index 1-based, total) or (0,0,0) if none.
func (e *Editor) PrevBookmark() (int, int, int) {
	tab := e.ActiveTab()
	if tab == nil {
		return 0, 0, 0
	}
	lines := sortedBookmarkLines(tab)
	if len(lines) == 0 {
		return 0, 0, 0
	}
	curLine := tab.CursorRow
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] < curLine {
			tab.CursorRow = lines[i]
			tab.CursorCol = 0
			e.ensureCursorVisible(tab)
			e.notifyCursorMove()
			return lines[i] + 1, i + 1, len(lines)
		}
	}
	// Wrap to last
	last := lines[len(lines)-1]
	tab.CursorRow = last
	tab.CursorCol = 0
	e.ensureCursorVisible(tab)
	e.notifyCursorMove()
	return last + 1, len(lines), len(lines)
}

// AllBookmarks returns all bookmarks across all open tabs.
// Each entry is (tab index, tab name, file path, line index 0-based, line text).
type BookmarkEntry struct {
	TabIndex int
	TabName  string
	FilePath string
	Line     int    // 0-based
	Text     string // content of the line
}

func (e *Editor) AllBookmarks() []BookmarkEntry {
	var entries []BookmarkEntry
	for ti, tab := range e.tabs {
		lines := sortedBookmarkLines(tab)
		for _, l := range lines {
			text := ""
			if l < tab.Buffer.LineCount() {
				text = tab.Buffer.Line(l)
			}
			entries = append(entries, BookmarkEntry{
				TabIndex: ti,
				TabName:  tab.Name,
				FilePath: tab.FilePath,
				Line:     l,
				Text:     text,
			})
		}
	}
	return entries
}

// Cursor and editing
func (e *Editor) ensureCursorVisible(tab *Tab) {
	_, _, width, height := e.GetInnerRect()
	height -= 1 // tab bar

	if e.wordWrap {
		tab.ScrollCol = 0
		e.ensureCursorVisibleWrapped(tab, width, height)
		return
	}

	if tab.CursorRow < tab.ScrollRow {
		tab.ScrollRow = tab.CursorRow
	}
	if tab.CursorRow >= tab.ScrollRow+height {
		tab.ScrollRow = tab.CursorRow - height + 1
	}

	// Horizontal scrolling
	gutterW := 0
	if e.showLineNumbers {
		gutterW = GutterWidth(tab.Buffer.LineCount())
	}
	visibleWidth := width - gutterW
	if visibleWidth < 1 {
		visibleWidth = 1
	}
	const margin = 4
	if tab.CursorCol < tab.ScrollCol {
		tab.ScrollCol = tab.CursorCol - margin
		if tab.ScrollCol < 0 {
			tab.ScrollCol = 0
		}
	}
	if tab.CursorCol >= tab.ScrollCol+visibleWidth {
		tab.ScrollCol = tab.CursorCol - visibleWidth + 1 + margin
	}
}

// ensureCursorVisibleWrapped adjusts ScrollRow so the cursor is visible in word wrap mode.
func (e *Editor) ensureCursorVisibleWrapped(tab *Tab, width, height int) {
	gutterW := 0
	if e.showLineNumbers {
		gutterW = GutterWidth(tab.Buffer.LineCount())
	}
	editorWidth := width - gutterW
	if editorWidth < 1 {
		editorWidth = 1
	}

	// If cursor is above scroll region, scroll up to it
	if tab.CursorRow < tab.ScrollRow {
		tab.ScrollRow = tab.CursorRow
	}

	// Count visual rows from ScrollRow to cursor to see if cursor is on screen
	for {
		visualRow := 0
		for lineIdx := tab.ScrollRow; lineIdx <= tab.CursorRow && lineIdx < tab.Buffer.LineCount(); lineIdx++ {
			line := tab.Buffer.Line(lineIdx)
			segs := wrapLine(line, editorWidth, e.tabSize)
			if lineIdx == tab.CursorRow {
				// Find which segment the cursor is in
				for _, seg := range segs {
					if tab.CursorCol >= seg.startCol && tab.CursorCol <= seg.endCol {
						if tab.CursorCol == seg.endCol && seg.endCol < len(line) {
							continue // cursor at boundary, belongs to next segment
						}
						// Cursor is on this visual row
						if visualRow < height {
							return // cursor is visible
						}
						// Not visible — need to scroll down
						tab.ScrollRow++
						break
					}
					visualRow++
				}
				break
			}
			visualRow += len(segs)
		}
		if visualRow < height {
			return
		}
		tab.ScrollRow++
		if tab.ScrollRow > tab.CursorRow {
			tab.ScrollRow = tab.CursorRow
			return
		}
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
	_ = cmd.Run()
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

	// Extended actions for keyboard modes
	case ActionOverwriteChar:
		if tab.HasSelect {
			e.deleteSelection(tab)
		}
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			// Delete char at cursor, then insert
			tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, tab.CursorCol+1, [2]int{tab.CursorRow, tab.CursorCol})
		}
		cursor := [2]int{tab.CursorRow, tab.CursorCol}
		newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, string(ch), cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]

	case ActionJoinLine:
		if tab.CursorRow < tab.Buffer.LineCount()-1 {
			// Remove newline between current and next line, add a space
			nextLine := tab.Buffer.Line(tab.CursorRow + 1)
			curLineLen := len(tab.Buffer.Line(tab.CursorRow))
			tab.Buffer.Delete(tab.CursorRow, curLineLen, tab.CursorRow+1, 0, [2]int{tab.CursorRow, tab.CursorCol})
			// Add space if next line is non-empty and doesn't start with space
			if len(nextLine) > 0 && nextLine[0] != ' ' {
				cursor := [2]int{tab.CursorRow, curLineLen}
				tab.Buffer.Insert(tab.CursorRow, curLineLen, " ", cursor)
			}
			tab.CursorCol = curLineLen
		}

	case ActionOpenLineBelow:
		// Insert new line below cursor and move to it
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		cursor := [2]int{tab.CursorRow, lineLen}
		tab.Buffer.Insert(tab.CursorRow, lineLen, "\n", cursor)
		tab.CursorRow++
		tab.CursorCol = 0

	case ActionOpenLineAbove:
		// Insert new line above cursor and move to it
		cursor := [2]int{tab.CursorRow, 0}
		tab.Buffer.Insert(tab.CursorRow, 0, "\n", cursor)
		tab.CursorCol = 0

	case ActionDeleteCharForward:
		if tab.HasSelect {
			e.deleteSelection(tab)
		} else {
			lineLen := len(tab.Buffer.Line(tab.CursorRow))
			if tab.CursorCol < lineLen {
				e.clipboardCopy(string(tab.Buffer.Line(tab.CursorRow)[tab.CursorCol]))
				tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, tab.CursorCol+1, [2]int{tab.CursorRow, tab.CursorCol})
			}
		}

	case ActionPasteAfter:
		text := e.clipboardPaste()
		if text != "" {
			// Move cursor right one, then paste
			lineLen := len(tab.Buffer.Line(tab.CursorRow))
			col := tab.CursorCol
			if col < lineLen {
				col++
			}
			cursor := [2]int{tab.CursorRow, col}
			newPos := tab.Buffer.Insert(tab.CursorRow, col, text, cursor)
			tab.CursorRow = newPos[0]
			tab.CursorCol = newPos[1]
		}

	case ActionPasteBefore:
		text := e.clipboardPaste()
		if text != "" {
			cursor := [2]int{tab.CursorRow, tab.CursorCol}
			tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, text, cursor)
		}

	case ActionYankLine:
		line := tab.Buffer.Line(tab.CursorRow)
		e.clipboardCopy(line + "\n")

	case ActionDeleteToLineEnd:
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			text := tab.Buffer.Line(tab.CursorRow)[tab.CursorCol:]
			e.clipboardCopy(text)
			tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, lineLen, [2]int{tab.CursorRow, tab.CursorCol})
		}

	case ActionChangeToLineEnd:
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		if tab.CursorCol < lineLen {
			text := tab.Buffer.Line(tab.CursorRow)[tab.CursorCol:]
			e.clipboardCopy(text)
			tab.Buffer.Delete(tab.CursorRow, tab.CursorCol, tab.CursorRow, lineLen, [2]int{tab.CursorRow, tab.CursorCol})
		}

	case ActionCursorFirstNonBlank:
		e.clearSelection(tab)
		line := tab.Buffer.Line(tab.CursorRow)
		for i, ch := range line {
			if !unicode.IsSpace(ch) {
				tab.CursorCol = i
				break
			}
		}

	case ActionSelectLine:
		// Select the current line (Helix x)
		tab.HasSelect = true
		tab.SelectStart = [2]int{tab.CursorRow, 0}
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		tab.SelectEnd = [2]int{tab.CursorRow, lineLen}
		tab.CursorCol = lineLen

	case ActionExtendLineSelect:
		// Extend selection to include next line (Helix X)
		if !tab.HasSelect {
			tab.HasSelect = true
			tab.SelectStart = [2]int{tab.CursorRow, 0}
		}
		if tab.CursorRow < tab.Buffer.LineCount()-1 {
			tab.CursorRow++
		}
		lineLen := len(tab.Buffer.Line(tab.CursorRow))
		tab.SelectEnd = [2]int{tab.CursorRow, lineLen}
		tab.CursorCol = lineLen

	case ActionSearchForward:
		if e.onSearchForward != nil {
			e.onSearchForward()
		}
	case ActionSearchNext:
		if e.onSearchNext != nil {
			e.onSearchNext()
		}
	case ActionSearchPrev:
		if e.onSearchPrev != nil {
			e.onSearchPrev()
		}
	case ActionMatchBracket:
		// Jump to matching bracket
		// Use cached highlight data for string/comment detection
		highlighted := e.cachedHL
		if highlighted == nil {
			highlighted = tab.Highlighter.Highlight(tab.Buffer.Text())
		}
		match := e.FindMatchingBracket(tab, highlighted)
		if match.FoundBracket {
			if match.HasMatch {
				e.clearSelection(tab)
				tab.CursorRow = match.MatchLine
				tab.CursorCol = match.MatchCol
			} else if e.onStatusMessage != nil {
				e.onStatusMessage("No matching bracket found")
			}
		}

	case ActionEnterCommandMode:
		// Handled by the key mode itself
	}

	e.clampCursor(tab)
	e.ensureCursorVisible(tab)

	switch action {
	case ActionInsertChar, ActionInsertNewline, ActionInsertTab,
		ActionDeleteChar, ActionBackspace, ActionDeleteWord, ActionDeleteLine,
		ActionCut, ActionPaste, ActionUndo, ActionRedo,
		ActionOverwriteChar, ActionJoinLine,
		ActionOpenLineBelow, ActionOpenLineAbove,
		ActionDeleteCharForward, ActionPasteAfter, ActionPasteBefore,
		ActionDeleteToLineEnd, ActionChangeToLineEnd:
		e.notifyChange()
	default:
		e.notifyCursorMove()
	}
}

// Find searches for text and positions cursor (literal case-insensitive search)
func (e *Editor) Find(query string, forward bool) bool {
	return e.FindWithOptions(query, forward, false)
}

// FindWithOptions searches for text with optional regex mode
func (e *Editor) FindWithOptions(query string, forward bool, useRegex bool) bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}
	if query == "" {
		query = e.lastSearch
	}
	if query == "" {
		return false
	}
	e.lastSearch = query

	startRow := tab.CursorRow
	startCol := tab.CursorCol
	if forward {
		startCol++
	}

	if useRegex {
		return e.findRegex(tab, query, forward, startRow, startCol)
	}
	return e.findLiteral(tab, query, forward, startRow, startCol)
}

// findLiteral performs case-insensitive literal search
func (e *Editor) findLiteral(tab *Tab, query string, forward bool, startRow, startCol int) bool {
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

// findRegex performs regex search
func (e *Editor) findRegex(tab *Tab, pattern string, forward bool, startRow, startCol int) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		if e.onStatusMessage != nil {
			e.onStatusMessage("Regex error: " + err.Error())
		}
		return false
	}

	if forward {
		for r := startRow; r < tab.Buffer.LineCount(); r++ {
			line := tab.Buffer.Line(r)
			sc := 0
			if r == startRow {
				sc = startCol
			}
			if sc > len(line) {
				continue
			}
			loc := re.FindStringIndex(line[sc:])
			if loc != nil {
				matchStart := sc + loc[0]
				matchEnd := sc + loc[1]
				tab.CursorRow = r
				tab.CursorCol = matchStart
				tab.HasSelect = true
				tab.SelectStart = [2]int{r, matchStart}
				tab.SelectEnd = [2]int{r, matchEnd}
				e.ensureCursorVisible(tab)
				e.notifyChange()
				return true
			}
		}
		// Wrap around
		for r := 0; r <= startRow; r++ {
			line := tab.Buffer.Line(r)
			loc := re.FindStringIndex(line)
			if loc != nil {
				tab.CursorRow = r
				tab.CursorCol = loc[0]
				tab.HasSelect = true
				tab.SelectStart = [2]int{r, loc[0]}
				tab.SelectEnd = [2]int{r, loc[1]}
				e.ensureCursorVisible(tab)
				e.notifyChange()
				return true
			}
		}
	}

	return false
}

// FindRegexError checks if a regex pattern is valid and returns the error if not
func (e *Editor) FindRegexError(pattern string) error {
	_, err := regexp.Compile(pattern)
	return err
}

// Replace replaces selected text and finds next (literal mode)
func (e *Editor) Replace(find, replace string) bool {
	return e.ReplaceWithOptions(find, replace, false)
}

// ReplaceWithOptions replaces selected text and finds next with optional regex mode
func (e *Editor) ReplaceWithOptions(find, replace string, useRegex bool) bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}
	if tab.HasSelect {
		sel := e.selectedText(tab)
		doReplace := false
		actualReplace := replace

		if useRegex {
			re, err := regexp.Compile(find)
			if err == nil && re.MatchString(sel) {
				doReplace = true
				// Apply capture group substitution
				actualReplace = re.ReplaceAllString(sel, replace)
			}
		} else {
			if strings.EqualFold(sel, find) {
				doReplace = true
			}
		}

		if doReplace {
			e.deleteSelection(tab)
			cursor := [2]int{tab.CursorRow, tab.CursorCol}
			newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, actualReplace, cursor)
			tab.CursorRow = newPos[0]
			tab.CursorCol = newPos[1]
		}
	}
	return e.FindWithOptions(find, true, useRegex)
}

// ReplaceAll replaces all occurrences (literal mode)
func (e *Editor) ReplaceAll(find, replace string) int {
	return e.ReplaceAllWithOptions(find, replace, false)
}

// ReplaceAllWithOptions replaces all occurrences with optional regex mode
func (e *Editor) ReplaceAllWithOptions(find, replace string, useRegex bool) int {
	tab := e.ActiveTab()
	if tab == nil {
		return 0
	}
	return e.replaceAllInTab(tab, find, replace, useRegex)
}

// replaceAllInTab replaces all occurrences in a specific tab
func (e *Editor) replaceAllInTab(tab *Tab, find, replace string, useRegex bool) int {
	text := tab.Buffer.Text()

	var newText string
	var count int

	if useRegex {
		re, err := regexp.Compile(find)
		if err != nil {
			if e.onStatusMessage != nil {
				e.onStatusMessage("Regex error: " + err.Error())
			}
			return 0
		}
		matches := re.FindAllStringIndex(text, -1)
		count = len(matches)
		if count == 0 {
			return 0
		}
		newText = re.ReplaceAllString(text, replace)
	} else {
		count = strings.Count(strings.ToLower(text), strings.ToLower(find))
		if count == 0 {
			return 0
		}
		newText = caseInsensitiveReplace(text, find, replace)
	}

	tab.Buffer = NewBufferFromText(newText)
	tab.Buffer.SetModified(true)
	tab.CursorRow = 0
	tab.CursorCol = 0
	e.clearSelection(tab)
	e.notifyChange()
	return count
}

// ReplaceAllInAllFiles replaces all occurrences across all open tabs
// Returns a map of filename -> replacement count
func (e *Editor) ReplaceAllInAllFiles(find, replace string, useRegex bool) map[string]int {
	results := make(map[string]int)
	for _, tab := range e.tabs {
		count := e.replaceAllInTab(tab, find, replace, useRegex)
		if count > 0 {
			name := tab.Name
			if tab.FilePath != "" {
				name = tab.FilePath
			}
			results[name] = count
		}
	}
	e.notifyChange()
	return results
}

// SearchResult represents a single search match across files
type SearchResult struct {
	FilePath string
	TabIndex int
	Line     int // 0-based
	Col      int // 0-based
	MatchLen int
	LineText string
}

// FindAllFiles searches across all open tabs and returns grouped results
func (e *Editor) FindAllFiles(query string, useRegex bool) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	var re *regexp.Regexp
	var err error
	if useRegex {
		re, err = regexp.Compile(query)
		if err != nil {
			return nil, err
		}
	}

	var results []SearchResult
	lowerQuery := strings.ToLower(query)

	for tabIdx, tab := range e.tabs {
		name := tab.Name
		if tab.FilePath != "" {
			name = tab.FilePath
		}
		for r := 0; r < tab.Buffer.LineCount(); r++ {
			line := tab.Buffer.Line(r)
			if useRegex {
				locs := re.FindAllStringIndex(line, -1)
				for _, loc := range locs {
					results = append(results, SearchResult{
						FilePath: name,
						TabIndex: tabIdx,
						Line:     r,
						Col:      loc[0],
						MatchLen: loc[1] - loc[0],
						LineText: line,
					})
				}
			} else {
				lowerLine := strings.ToLower(line)
				offset := 0
				for {
					idx := strings.Index(lowerLine[offset:], lowerQuery)
					if idx < 0 {
						break
					}
					results = append(results, SearchResult{
						FilePath: name,
						TabIndex: tabIdx,
						Line:     r,
						Col:      offset + idx,
						MatchLen: len(query),
						LineText: line,
					})
					offset += idx + len(query)
				}
			}
		}
	}
	return results, nil
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

	// Draw breadcrumb bar in modern mode
	if ui.Style.Modern && height > 2 {
		e.drawBreadcrumb(screen, x, y, width)
		y++
		height--
	}

	tab := e.ActiveTab()
	if tab == nil {
		return
	}

	// Calculate gutter width
	gutterW := 0
	if e.showLineNumbers {
		gutterW = GutterWidth(tab.Buffer.LineCount())
	}

	// Reserve space for scrollbar in modern mode
	scrollbarW := 0
	if ui.Style.Modern && tab.Buffer.LineCount() > height {
		scrollbarW = 1
		width -= scrollbarW
	}

	// Highlight all lines (cached)
	if e.cachedHLVer != e.hlVersion || e.cachedHL == nil {
		e.cachedHL = tab.Highlighter.Highlight(tab.Buffer.Text())
		e.cachedHLVer = e.hlVersion
	}
	highlighted := e.cachedHL

	// Compute bracket matching for cursor position
	e.bracketMatch = e.FindMatchingBracket(tab, highlighted)

	// Word wrap mode: different rendering path
	if e.wordWrap {
		tab.ScrollCol = 0 // no horizontal scrolling in wrap mode
		e.drawWrapped(screen, x, y, width, height, gutterW, tab, highlighted)
		return
	}

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
		if e.showLineNumbers {
			gutterStr := FormatGutterLine(lineIdx+1, tab.Buffer.LineCount())
			gutterStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)

			// Check for breakpoint marker on this line
			if e.hasBreakpoint != nil && tab.FilePath != "" && e.hasBreakpoint(tab.FilePath, lineIdx+1) {
				screen.SetContent(x, y+row, '*', nil,
					tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorGutterBg).Bold(true))
				for i, ch := range gutterStr {
					if i > 0 && x+i < x+gutterW {
						screen.SetContent(x+i, y+row, ch, nil, gutterStyle)
					}
				}
				goto gutterDone
			}

			// Check for diagnostic marker on this line (build errors take precedence over LSP)
			if tab.FilePath != "" {
				diag, hasDiag := DiagnosticInfo{}, false
				if bd, ok := e.buildDiags[tab.FilePath]; ok {
					diag, hasDiag = bd[lineIdx]
				}
				if !hasDiag {
					if ld, ok := e.diagnostics[tab.FilePath]; ok {
						diag, hasDiag = ld[lineIdx]
					}
				}
				if hasDiag {
					markerCh, markerFg := diagnosticMarker(diag.Severity)
					// Draw marker in first gutter column
					screen.SetContent(x, y+row, markerCh, nil,
						tcell.StyleDefault.Foreground(markerFg).Background(ui.ColorGutterBg).Bold(true))
					// Draw rest of gutter (line number) starting at position 1
					for i, ch := range gutterStr {
						if i > 0 && x+i < x+gutterW {
							screen.SetContent(x+i, y+row, ch, nil, gutterStyle)
						}
					}
					goto gutterDone
				}
			}

			// Check for bookmark marker on this line
			if tab.Bookmarks[lineIdx] {
				screen.SetContent(x, y+row, '#', nil,
					tcell.StyleDefault.Foreground(tcell.ColorAqua).Background(ui.ColorGutterBg).Bold(true))
				for i, ch := range gutterStr {
					if i > 0 && x+i < x+gutterW {
						screen.SetContent(x+i, y+row, ch, nil, gutterStyle)
					}
				}
				goto gutterDone
			}

			for i, ch := range gutterStr {
				if x+i < x+gutterW {
					screen.SetContent(x+i, y+row, ch, nil, gutterStyle)
				}
			}
		gutterDone:
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
		line := tab.Buffer.Line(tab.CursorRow)
		cursorCol := byteOffsetToScreenCol(line, tab.CursorCol, e.tabSize)
		scrollCol := byteOffsetToScreenCol(line, tab.ScrollCol, e.tabSize)
		cursorScreenX := x + gutterW + cursorCol - scrollCol
		cursorScreenY := y + tab.CursorRow - tab.ScrollRow
		if cursorScreenY >= y && cursorScreenY < y+height && cursorScreenX >= x+gutterW && cursorScreenX < x+width {
			if e.keyMode.CursorStyle() == keymode.CursorBlock {
				// Block cursor: draw char at cursor position with reversed colors
				ch := ' '
				if tab.CursorCol < len(line) {
					ch, _ = utf8.DecodeRuneInString(line[tab.CursorCol:])
				}
				style := tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorText)
				screen.SetContent(cursorScreenX, cursorScreenY, ch, nil, style)
				screen.HideCursor()
			} else {
				screen.ShowCursor(cursorScreenX, cursorScreenY)
			}
		}
	}

	// Draw scrollbar in modern mode
	if scrollbarW > 0 {
		e.drawScrollbar(screen, x+width, y, height, tab)
	}

	// Draw completion popup on top
	if e.completion.Visible() {
		e.completion.Draw(screen, x, y, gutterW, tab.ScrollRow, tab.ScrollCol)
	}
}

// drawScrollbar draws a scrollbar track with thumb and markers on the right edge.
func (e *Editor) drawScrollbar(screen tcell.Screen, x, y, height int, tab *Tab) {
	totalLines := tab.Buffer.LineCount()
	if totalLines == 0 || height == 0 {
		return
	}

	trackStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBgDarker)
	thumbStyle := tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorTextGray)

	// Calculate thumb position and size
	thumbSize := height * height / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}
	thumbPos := 0
	if totalLines > height {
		thumbPos = tab.ScrollRow * (height - thumbSize) / (totalLines - height)
	}

	// Draw track
	for row := 0; row < height; row++ {
		ch := ui.Style.ScrollTrack()
		style := trackStyle
		if row >= thumbPos && row < thumbPos+thumbSize {
			ch = ui.Style.ScrollThumb()
			style = thumbStyle
		}
		screen.SetContent(x, y+row, ch, nil, style)
	}

	// Draw markers at proportional positions
	if tab.FilePath != "" {
		// Diagnostic markers (LSP)
		if diags, ok := e.diagnostics[tab.FilePath]; ok {
			for lineIdx, diag := range diags {
				markerRow := lineIdx * height / totalLines
				if markerRow >= height {
					markerRow = height - 1
				}
				markerFg := tcell.ColorBlue
				if diag.Severity == 1 {
					markerFg = tcell.ColorRed
				} else if diag.Severity == 2 {
					markerFg = tcell.ColorYellow
				}
				screen.SetContent(x, y+markerRow, '\u2588', nil,
					tcell.StyleDefault.Foreground(markerFg).Background(ui.ColorBgDarker))
			}
		}
		// Build error markers (overwrite LSP markers on same line)
		if bdiags, ok := e.buildDiags[tab.FilePath]; ok {
			for lineIdx, diag := range bdiags {
				markerRow := lineIdx * height / totalLines
				if markerRow >= height {
					markerRow = height - 1
				}
				markerFg := tcell.ColorBlue
				if diag.Severity == 1 {
					markerFg = tcell.ColorRed
				} else if diag.Severity == 2 {
					markerFg = tcell.ColorYellow
				}
				screen.SetContent(x, y+markerRow, '\u2588', nil,
					tcell.StyleDefault.Foreground(markerFg).Background(ui.ColorBgDarker))
			}
		}

		// Breakpoint markers
		if e.hasBreakpoint != nil {
			for lineIdx := 0; lineIdx < totalLines; lineIdx++ {
				if e.hasBreakpoint(tab.FilePath, lineIdx+1) {
					markerRow := lineIdx * height / totalLines
					if markerRow >= height {
						markerRow = height - 1
					}
					screen.SetContent(x, y+markerRow, '\u25cf', nil,
						tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorBgDarker))
				}
			}
		}
	}

	// Bookmark markers (cyan)
	for lineIdx := range tab.Bookmarks {
		if lineIdx < totalLines {
			markerRow := lineIdx * height / totalLines
			if markerRow >= height {
				markerRow = height - 1
			}
			screen.SetContent(x, y+markerRow, '#', nil,
				tcell.StyleDefault.Foreground(tcell.ColorAqua).Background(ui.ColorBgDarker))
		}
	}
}

// drawWrapped renders the editor content with word wrapping enabled.
func (e *Editor) drawWrapped(screen tcell.Screen, x, y, width, height, gutterW int, tab *Tab, highlighted []HighlightedLine) {
	editorWidth := width - gutterW
	if editorWidth < 1 {
		editorWidth = 1
	}

	cursorScreenX := -1
	cursorScreenY := -1
	visualRow := 0
	lineIdx := tab.ScrollRow

	for visualRow < height && lineIdx < tab.Buffer.LineCount() {
		line := tab.Buffer.Line(lineIdx)
		segs := wrapLine(line, editorWidth, e.tabSize)

		for segIdx, seg := range segs {
			if visualRow >= height {
				break
			}

			// Clear the visual row
			for cx := x; cx < x+width; cx++ {
				screen.SetContent(cx, y+visualRow, ' ', nil, tcell.StyleDefault.Background(ui.ColorBg))
			}

			// Draw gutter: line number on first segment, blank on continuations
			if e.showLineNumbers {
				gutterStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)
				if segIdx == 0 {
					e.drawGutterForLine(screen, x, y+visualRow, gutterW, lineIdx, tab)
				} else {
					// Blank gutter for continuation lines
					for i := 0; i < gutterW; i++ {
						screen.SetContent(x+i, y+visualRow, ' ', nil, gutterStyle)
					}
				}
			}

			// Draw the segment content
			editorX := x + gutterW
			if lineIdx < len(highlighted) {
				e.drawHighlightedLineSegment(screen, editorX, y+visualRow, editorWidth, line, highlighted[lineIdx], lineIdx, tab, seg.startCol, seg.endCol)
			} else {
				e.drawPlainLineSegment(screen, editorX, y+visualRow, editorWidth, line, lineIdx, tab, seg.startCol, seg.endCol)
			}

			// Track cursor position (convert byte offsets to screen columns)
			if lineIdx == tab.CursorRow && tab.CursorCol >= seg.startCol && tab.CursorCol <= seg.endCol {
				if tab.CursorCol == seg.endCol && segIdx < len(segs)-1 {
					// Cursor at exact boundary — it belongs to the next segment
				} else {
					cursorScreenCol := byteOffsetToScreenCol(line, tab.CursorCol, e.tabSize)
					segStartScreenCol := byteOffsetToScreenCol(line, seg.startCol, e.tabSize)
					cursorScreenX = editorX + cursorScreenCol - segStartScreenCol
					cursorScreenY = y + visualRow
				}
			}

			visualRow++
		}
		lineIdx++
	}

	// Fill remaining rows with tildes
	for visualRow < height {
		for cx := x; cx < x+width; cx++ {
			screen.SetContent(cx, y+visualRow, ' ', nil, tcell.StyleDefault.Background(ui.ColorBg))
		}
		screen.SetContent(x+gutterW, y+visualRow, '~', nil, tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBg))
		visualRow++
	}

	// Draw cursor
	if e.hasFocus && cursorScreenX >= 0 && cursorScreenY >= 0 {
		if cursorScreenX < x+width {
			if e.keyMode.CursorStyle() == keymode.CursorBlock {
				ch := ' '
				curLine := tab.Buffer.Line(tab.CursorRow)
				if tab.CursorCol < len(curLine) {
					ch, _ = utf8.DecodeRuneInString(curLine[tab.CursorCol:])
				}
				style := tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorText)
				screen.SetContent(cursorScreenX, cursorScreenY, ch, nil, style)
				screen.HideCursor()
			} else {
				screen.ShowCursor(cursorScreenX, cursorScreenY)
			}
		}
	}

	// Draw completion popup on top
	if e.completion.Visible() {
		e.completion.Draw(screen, x, y, gutterW, tab.ScrollRow, tab.ScrollCol)
	}
}

// SetBreadcrumbSymbols sets the document symbols for breadcrumb display.
func (e *Editor) SetBreadcrumbSymbols(filePath string, symbols []BreadcrumbSymbol) {
	e.breadcrumbFile = filePath
	e.breadcrumbSymbols = symbols
}

// BreadcrumbPath returns the symbol path at the current cursor position.
func (e *Editor) BreadcrumbPath() []string {
	tab := e.ActiveTab()
	if tab == nil || tab.FilePath != e.breadcrumbFile || len(e.breadcrumbSymbols) == 0 {
		return nil
	}
	var path []string
	findSymbol(e.breadcrumbSymbols, tab.CursorRow, &path)
	return path
}

func findSymbol(symbols []BreadcrumbSymbol, line int, path *[]string) {
	for i := range symbols {
		s := &symbols[i]
		if line >= s.StartLine && line <= s.EndLine {
			*path = append(*path, s.Name)
			if len(s.Children) > 0 {
				findSymbol(s.Children, line, path)
			}
			return
		}
	}
}

// drawBreadcrumb draws the breadcrumb bar.
func (e *Editor) drawBreadcrumb(screen tcell.Screen, x, y, width int) {
	style := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBgDarker)
	// Clear breadcrumb bar
	for cx := x; cx < x+width; cx++ {
		screen.SetContent(cx, y, ' ', nil, style)
	}

	tab := e.ActiveTab()
	if tab == nil {
		return
	}

	// Build breadcrumb text
	parts := e.BreadcrumbPath()
	sep := " " + ui.Style.BreadcrumbSep() + " "

	text := ""
	if tab.FilePath != "" {
		// Show filename as first segment
		name := tab.Name
		text = name
	}

	for _, part := range parts {
		if text != "" {
			text += sep
		}
		text += part
	}

	if text == "" {
		return
	}

	// Render
	nameStyle := tcell.StyleDefault.Foreground(ui.ColorTextWhite).Background(ui.ColorBgDarker)
	sepStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorBgDarker)

	cx := x + 1
	inSep := false
	for _, ch := range " " + text {
		if cx >= x+width {
			break
		}
		// Detect separator characters
		isSepChar := false
		for _, sc := range sep {
			if ch == sc {
				isSepChar = true
				break
			}
		}
		if isSepChar {
			inSep = true
		} else if inSep {
			inSep = false
		}

		if inSep {
			screen.SetContent(cx, y, ch, nil, sepStyle)
		} else {
			screen.SetContent(cx, y, ch, nil, nameStyle)
		}
		cx++
	}
}

// diagnosticMarker returns the appropriate marker character and color for a diagnostic severity.
func diagnosticMarker(severity int) (rune, tcell.Color) {
	switch severity {
	case 1: // error
		return ui.Style.ErrorMarker(), tcell.ColorRed
	case 2: // warning
		return ui.Style.WarningMarker(), tcell.ColorYellow
	default: // info/hint
		return ui.Style.InfoMarker(), tcell.ColorBlue
	}
}

// drawGutterForLine draws the gutter (line number, breakpoint, diagnostic) for a given logical line.
func (e *Editor) drawGutterForLine(screen tcell.Screen, x, y, gutterW, lineIdx int, tab *Tab) {
	gutterStr := FormatGutterLine(lineIdx+1, tab.Buffer.LineCount())
	gutterStyle := tcell.StyleDefault.Foreground(ui.ColorGutterText).Background(ui.ColorGutterBg)

	// Check for breakpoint marker
	if e.hasBreakpoint != nil && tab.FilePath != "" && e.hasBreakpoint(tab.FilePath, lineIdx+1) {
		screen.SetContent(x, y, '*', nil,
			tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ui.ColorGutterBg).Bold(true))
		for i, ch := range gutterStr {
			if i > 0 && x+i < x+gutterW {
				screen.SetContent(x+i, y, ch, nil, gutterStyle)
			}
		}
		return
	}

	// Check for diagnostic marker (build errors take precedence over LSP)
	if tab.FilePath != "" {
		diag, hasDiag := DiagnosticInfo{}, false
		if bd, ok := e.buildDiags[tab.FilePath]; ok {
			diag, hasDiag = bd[lineIdx]
		}
		if !hasDiag {
			if ld, ok := e.diagnostics[tab.FilePath]; ok {
				diag, hasDiag = ld[lineIdx]
			}
		}
		if hasDiag {
			markerCh, markerFg := diagnosticMarker(diag.Severity)
			screen.SetContent(x, y, markerCh, nil,
				tcell.StyleDefault.Foreground(markerFg).Background(ui.ColorGutterBg).Bold(true))
			for i, ch := range gutterStr {
				if i > 0 && x+i < x+gutterW {
					screen.SetContent(x+i, y, ch, nil, gutterStyle)
				}
			}
			return
		}
	}

	// Check for bookmark marker
	if tab.Bookmarks[lineIdx] {
		screen.SetContent(x, y, '#', nil,
			tcell.StyleDefault.Foreground(tcell.ColorAqua).Background(ui.ColorGutterBg).Bold(true))
		for i, ch := range gutterStr {
			if i > 0 && x+i < x+gutterW {
				screen.SetContent(x+i, y, ch, nil, gutterStyle)
			}
		}
		return
	}

	for i, ch := range gutterStr {
		if x+i < x+gutterW {
			screen.SetContent(x+i, y, ch, nil, gutterStyle)
		}
	}
}

// drawHighlightedLineSegment draws a segment [startCol, endCol) of a highlighted line.
// startCol and endCol are byte offsets. Selection uses global byte offset i via
// isInSelection, so selection highlighting works correctly across wrap boundaries.
func (e *Editor) drawHighlightedLineSegment(screen tcell.Screen, x, y, maxWidth int, rawLine string, hl HighlightedLine, lineIdx int, tab *Tab, startCol, endCol int) {
	startScreenCol := byteOffsetToScreenCol(rawLine, startCol, e.tabSize)
	screenCol := startScreenCol
	for i, ch := range rawLine {
		if i < startCol {
			continue
		}
		if i >= endCol {
			break
		}
		if ch == '\t' {
			tabW := e.tabSize - (screenCol % e.tabSize)
			for t := 0; t < tabW; t++ {
				sx := x + screenCol - startScreenCol + t
				if sx >= x+maxWidth {
					break
				}
				style := e.hlStyleAt(hl, i, tab, lineIdx)
				screen.SetContent(sx, y, ' ', nil, style)
			}
			screenCol += tabW
			continue
		}
		sx := x + screenCol - startScreenCol
		if sx >= x+maxWidth {
			break
		}
		style := e.hlStyleAt(hl, i, tab, lineIdx)
		screen.SetContent(sx, y, ch, nil, style)
		screenCol++
	}
}

// drawPlainLineSegment draws a segment [startCol, endCol) of a plain line.
// startCol and endCol are byte offsets. Selection uses global byte offset i via
// isInSelection, so selection highlighting works correctly across wrap boundaries.
func (e *Editor) drawPlainLineSegment(screen tcell.Screen, x, y, maxWidth int, line string, lineIdx int, tab *Tab, startCol, endCol int) {
	startScreenCol := byteOffsetToScreenCol(line, startCol, e.tabSize)
	screenCol := startScreenCol
	for i, ch := range line {
		if i < startCol {
			continue
		}
		if i >= endCol {
			break
		}
		if ch == '\t' {
			tabW := e.tabSize - (screenCol % e.tabSize)
			for t := 0; t < tabW; t++ {
				sx := x + screenCol - startScreenCol + t
				if sx >= x+maxWidth {
					break
				}
				style := e.plainStyleAt(tab, lineIdx, i)
				screen.SetContent(sx, y, ' ', nil, style)
			}
			screenCol += tabW
			continue
		}
		sx := x + screenCol - startScreenCol
		if sx >= x+maxWidth {
			break
		}
		style := e.plainStyleAt(tab, lineIdx, i)
		screen.SetContent(sx, y, ch, nil, style)
		screenCol++
	}
}

// handleWrappedClick maps a visual click position to a logical cursor position in wrap mode.
// clickX is in screen columns; we convert it to a byte offset within the segment.
func (e *Editor) handleWrappedClick(tab *Tab, clickX, clickY, width, gutterW int) {
	editorWidth := width - gutterW
	if editorWidth < 1 {
		editorWidth = 1
	}

	visualRow := 0
	for lineIdx := tab.ScrollRow; lineIdx < tab.Buffer.LineCount(); lineIdx++ {
		line := tab.Buffer.Line(lineIdx)
		segs := wrapLine(line, editorWidth, e.tabSize)
		for _, seg := range segs {
			if visualRow == clickY {
				tab.CursorRow = lineIdx
				// Convert clickX screen columns to byte offset within segment
				segStartScreenCol := byteOffsetToScreenCol(line, seg.startCol, e.tabSize)
				targetScreenCol := segStartScreenCol + clickX
				tab.CursorCol = screenColToByteOffset(line, targetScreenCol, e.tabSize)
				if tab.CursorCol > seg.endCol {
					tab.CursorCol = seg.endCol
				}
				e.clampCursor(tab)
				e.clearSelection(tab)
				e.notifyChange()
				return
			}
			visualRow++
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
		fg := ui.ColorTabInactive
		bg := ui.ColorTabBarBg
		if i == e.activeTab {
			fg = ui.ColorTabActive
			bg = ui.ColorTabActiveBg
		}

		// Build label: [icon] name [modified]
		var label string
		if ui.Style.Modern {
			icon := ui.Style.FileIcon(tab.Name)
			if tab.Buffer.Modified() {
				label = " " + icon + " " + tab.Name + " " + ui.Style.ModifiedDot() + " "
			} else {
				label = " " + icon + " " + tab.Name + " "
			}
		} else {
			name := tab.Name
			if tab.Buffer.Modified() {
				name = "*" + name
			}
			label = " " + name + " "
		}

		for _, ch := range label {
			if cx < x+width {
				screen.SetContent(cx, y, ch, nil, tcell.StyleDefault.Foreground(fg).Background(bg))
				cx++
			}
		}
		// Separator
		if cx < x+width {
			screen.SetContent(cx, y, ui.Style.TabSeparator(), nil, tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(ui.ColorTabBarBg))
			cx++
		}
	}
}

func (e *Editor) drawHighlightedLine(screen tcell.Screen, x, y, maxWidth int, rawLine string, hl HighlightedLine, lineIdx int, tab *Tab) {
	scrollScreenCol := byteOffsetToScreenCol(rawLine, tab.ScrollCol, e.tabSize)
	screenCol := 0
	for i, ch := range rawLine {
		if ch == '\t' {
			tabW := e.tabSize - (screenCol % e.tabSize)
			for t := 0; t < tabW; t++ {
				sx := x + screenCol - scrollScreenCol + t
				if sx >= x && sx < x+maxWidth {
					style := e.hlStyleAt(hl, i, tab, lineIdx)
					screen.SetContent(sx, y, ' ', nil, style)
				}
			}
			screenCol += tabW
			continue
		}
		sx := x + screenCol - scrollScreenCol
		if sx >= x && sx < x+maxWidth {
			style := e.hlStyleAt(hl, i, tab, lineIdx)
			screen.SetContent(sx, y, ch, nil, style)
		}
		screenCol++
	}
}

func (e *Editor) drawPlainLine(screen tcell.Screen, x, y, maxWidth int, line string, lineIdx int, tab *Tab) {
	scrollScreenCol := byteOffsetToScreenCol(line, tab.ScrollCol, e.tabSize)
	screenCol := 0
	for i, ch := range line {
		if ch == '\t' {
			tabW := e.tabSize - (screenCol % e.tabSize)
			for t := 0; t < tabW; t++ {
				sx := x + screenCol - scrollScreenCol + t
				if sx >= x && sx < x+maxWidth {
					style := e.plainStyleAt(tab, lineIdx, i)
					screen.SetContent(sx, y, ' ', nil, style)
				}
			}
			screenCol += tabW
			continue
		}
		sx := x + screenCol - scrollScreenCol
		if sx >= x && sx < x+maxWidth {
			style := e.plainStyleAt(tab, lineIdx, i)
			screen.SetContent(sx, y, ch, nil, style)
		}
		screenCol++
	}
}

// plainStyleAt returns the style for a byte offset in a plain (non-highlighted) line,
// including selection and bracket match highlighting.
func (e *Editor) plainStyleAt(tab *Tab, lineIdx, byteOff int) tcell.Style {
	style := tcell.StyleDefault.Foreground(ui.ColorText).Background(ui.ColorBg)
	if tab.HasSelect && e.isInSelection(tab, lineIdx, byteOff) {
		style = style.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
	}
	if e.bracketMatch.FoundBracket {
		if lineIdx == e.bracketMatch.StartLine && byteOff == e.bracketMatch.StartCol {
			if e.bracketMatch.HasMatch {
				style = tcell.StyleDefault.Foreground(ui.ColorBracketMatchFg).Background(ui.ColorBracketMatch).Bold(true)
			} else {
				style = tcell.StyleDefault.Foreground(ui.ColorBracketErrorFg).Background(ui.ColorBracketError).Bold(true)
			}
		}
		if e.bracketMatch.HasMatch && lineIdx == e.bracketMatch.MatchLine && byteOff == e.bracketMatch.MatchCol {
			style = tcell.StyleDefault.Foreground(ui.ColorBracketMatchFg).Background(ui.ColorBracketMatch).Bold(true)
		}
	}
	return style
}

// hlStyleAt returns the tcell style for a byte offset in a highlighted line.
func (e *Editor) hlStyleAt(hl HighlightedLine, byteOff int, tab *Tab, lineIdx int) tcell.Style {
	fg := ui.ColorText
	bold := false
	if byteOff < len(hl.Styles) {
		fg = hl.Styles[byteOff].Fg
		bold = hl.Styles[byteOff].Bold
	}
	style := tcell.StyleDefault.Foreground(fg).Background(ui.ColorBg)
	if bold {
		style = style.Bold(true)
	}
	if tab.HasSelect && e.isInSelection(tab, lineIdx, byteOff) {
		style = tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
	}
	// Bracket match highlighting (overrides selection for bracket positions)
	if e.bracketMatch.FoundBracket {
		if lineIdx == e.bracketMatch.StartLine && byteOff == e.bracketMatch.StartCol {
			if e.bracketMatch.HasMatch {
				style = tcell.StyleDefault.Foreground(ui.ColorBracketMatchFg).Background(ui.ColorBracketMatch).Bold(true)
			} else {
				style = tcell.StyleDefault.Foreground(ui.ColorBracketErrorFg).Background(ui.ColorBracketError).Bold(true)
			}
		}
		if e.bracketMatch.HasMatch && lineIdx == e.bracketMatch.MatchLine && byteOff == e.bracketMatch.MatchCol {
			style = tcell.StyleDefault.Foreground(ui.ColorBracketMatchFg).Background(ui.ColorBracketMatch).Bold(true)
		}
	}
	return style
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
		// Handle completion popup keys first
		if e.completion.Visible() {
			switch event.Key() {
			case tcell.KeyDown:
				e.completion.MoveDown()
				return
			case tcell.KeyUp:
				e.completion.MoveUp()
				return
			case tcell.KeyEnter, tcell.KeyTab:
				e.acceptCompletion()
				return
			case tcell.KeyEscape:
				e.completion.Hide()
				return
			}
		}

		// Alt+Left/Right: horizontal viewport scroll (no cursor move)
		if event.Modifiers()&tcell.ModAlt != 0 && !e.wordWrap {
			tab := e.ActiveTab()
			if tab != nil {
				switch event.Key() {
				case tcell.KeyLeft:
					if tab.ScrollCol > 0 {
						tab.ScrollCol -= 3
						if tab.ScrollCol < 0 {
							tab.ScrollCol = 0
						}
					}
					return
				case tcell.KeyRight:
					tab.ScrollCol += 3
					return
				}
			}
		}

		// Use the key mapper to process the event
		ctx := e.KeyModeContext()
		result := e.keyMode.ProcessKey(event, ctx)

		if result.Handled {
			if len(result.Actions) > 0 {
				// Execute compound actions
				for _, act := range result.Actions {
					e.HandleAction(Action(act), result.Char)
				}
			} else if result.Action != int(ActionNone) {
				e.HandleAction(Action(result.Action), result.Char)
			}
		} else {
			// Fallback to direct MapKey for unhandled keys
			action := MapKey(event)
			if action != ActionNone {
				e.HandleAction(action, event.Rune())
				result.Action = int(action)
				result.Char = event.Rune()
			}
		}

		// After inserting a character, check for completion triggers
		if Action(result.Action) == ActionInsertChar {
			e.maybeRequestCompletion(result.Char)
		}
		// If typing while popup visible, update the filter
		if e.completion.Visible() && Action(result.Action) == ActionInsertChar {
			e.updateCompletionPrefix()
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
		bx, by, bw, _ := e.GetInnerRect()

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

		gutterW := 0
		if e.showLineNumbers {
			gutterW = GutterWidth(tab.Buffer.LineCount())
		}
		editorX := mx - bx - gutterW
		editorY := my - by - 1 // -1 for tab bar

		// Adjust for breadcrumb bar in modern mode
		gutterClickY := editorY
		if ui.Style.Modern {
			gutterClickY = editorY - 1 // breadcrumb takes a row
		}

		// Gutter click: toggle bookmark
		if editorX < 0 && gutterW > 0 && action == tview.MouseLeftClick {
			lineIdx := tab.ScrollRow + gutterClickY
			if lineIdx >= 0 && lineIdx < tab.Buffer.LineCount() {
				e.ToggleBookmarkAtLine(lineIdx)
				e.notifyChange()
			}
			return true, nil
		}

		if editorX < 0 {
			return false, nil
		}

		switch action {
		case tview.MouseLeftClick:
			setFocus(e)
			if e.wordWrap {
				e.handleWrappedClick(tab, editorX, editorY, bw, gutterW)
			} else {
				row := tab.ScrollRow + editorY
				col := editorX + tab.ScrollCol
				if row >= 0 && row < tab.Buffer.LineCount() {
					tab.CursorRow = row
					tab.CursorCol = col
					e.clampCursor(tab)
					e.clearSelection(tab)
					e.notifyChange()
				}
			}
			return true, nil
		case tview.MouseScrollUp:
			// Shift+Scroll = horizontal scroll left
			if event.Modifiers()&tcell.ModShift != 0 {
				if !e.wordWrap && tab.ScrollCol > 0 {
					tab.ScrollCol -= 3
					if tab.ScrollCol < 0 {
						tab.ScrollCol = 0
					}
					e.notifyChange()
				}
			} else {
				if tab.ScrollRow > 0 {
					tab.ScrollRow -= 3
					if tab.ScrollRow < 0 {
						tab.ScrollRow = 0
					}
					e.notifyChange()
				}
			}
			return true, nil
		case tview.MouseScrollDown:
			// Shift+Scroll = horizontal scroll right
			if event.Modifiers()&tcell.ModShift != 0 {
				if !e.wordWrap {
					tab.ScrollCol += 3
					e.notifyChange()
				}
			} else {
				if tab.ScrollRow < tab.Buffer.LineCount()-1 {
					tab.ScrollRow += 3
					e.notifyChange()
				}
			}
			return true, nil
		case tview.MouseScrollLeft:
			if !e.wordWrap && tab.ScrollCol > 0 {
				tab.ScrollCol -= 3
				if tab.ScrollCol < 0 {
					tab.ScrollCol = 0
				}
				e.notifyChange()
			}
			return true, nil
		case tview.MouseScrollRight:
			if !e.wordWrap {
				tab.ScrollCol += 3
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
