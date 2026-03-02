package ui

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SearchResult represents a single line match found during a project-wide text search.
type SearchResult struct {
	FilePath string // absolute path
	RelPath  string // relative path from workDir
	Line     int    // 1-based line number
	Content  string // the matching line text (trimmed)
}

// SearchPalette is an overlay widget for searching text across all project files.
// It renders directly to tcell.Screen, following the same pattern as FilePalette.
type SearchPalette struct {
	*tview.Box
	input    string
	results  []SearchResult
	selected int
	workDir  string
	onSelect func(result SearchResult)
	onClose  func()
}

// NewSearchPalette creates a SearchPalette rooted at workDir.
// onSelect is called when the user presses Enter on a result.
// onClose is called when the palette is dismissed.
func NewSearchPalette(workDir string, onSelect func(SearchResult), onClose func()) *SearchPalette {
	return &SearchPalette{
		Box:      tview.NewBox(),
		workDir:  workDir,
		onSelect: onSelect,
		onClose:  onClose,
	}
}

// search performs a synchronous case-insensitive text search across project files.
// It reuses the same skip logic as WalkProjectFiles and caps results at 100.
func (p *SearchPalette) search(query string) {
	p.results = nil
	p.selected = 0
	if len([]rune(query)) < 3 {
		return
	}

	needle := strings.ToLower(query)

	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".idea":        true,
		".vscode":      true,
		"__pycache__":  true,
		"dist":         true,
		"build":        true,
		"target":       true,
	}
	skipExts := map[string]bool{
		".exe": true, ".bin": true, ".o": true, ".a": true,
		".so": true, ".dylib": true, ".dll": true,
		".class": true, ".jar": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".svg": true, ".pdf": true,
		".db": true, ".sqlite": true,
	}

	const maxResults = 100

	_ = filepath.WalkDir(p.workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(p.results) >= maxResults {
			return filepath.SkipAll
		}

		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files and binary extensions
		if strings.HasPrefix(name, ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if skipExts[ext] {
			return nil
		}

		rel, relErr := filepath.Rel(p.workDir, path)
		if relErr != nil {
			rel = path
		}

		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if len(p.results) >= maxResults {
				break
			}
			lineText := scanner.Text()
			if strings.Contains(strings.ToLower(lineText), needle) {
				p.results = append(p.results, SearchResult{
					FilePath: path,
					RelPath:  rel,
					Line:     lineNum,
					Content:  strings.TrimSpace(lineText),
				})
			}
		}
		return nil
	})
}

// Draw renders the search palette overlay centered near the top of the screen.
func (p *SearchPalette) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)

	sw, sh := screen.Size()

	maxVisible := 16
	if maxVisible > len(p.results) {
		maxVisible = len(p.results)
	}
	paletteW := 80
	if paletteW > sw-4 {
		paletteW = sw - 4
	}
	paletteH := 2 + maxVisible
	if paletteH < 3 {
		paletteH = 3
	}

	px := (sw - paletteW) / 2
	py := 2
	if py+paletteH > sh {
		py = 0
	}

	bgStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)
	hlStyle := tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)
	dimStyle := tcell.StyleDefault.Foreground(ColorTextGray).Background(ColorDialogBg)
	dimHlStyle := tcell.StyleDefault.Foreground(ColorTextGray).Background(ColorMenuHighlight)
	borderStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)
	lineNumStyle := tcell.StyleDefault.Foreground(ColorMenuHighlight).Background(ColorDialogBg)
	lineNumHlStyle := tcell.StyleDefault.Foreground(ColorTextWhite).Background(ColorMenuHighlight)
	contentStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)
	contentHlStyle := tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)

	// Top border with title
	p.drawHLine(screen, px, py, paletteW, borderStyle)
	title := " Text Search (Ctrl+Shift+F) "
	titleX := px + (paletteW-len(title))/2
	if titleX < px+1 {
		titleX = px + 1
	}
	for i, ch := range title {
		if titleX+i < px+paletteW-1 {
			screen.SetContent(titleX+i, py, ch, nil, borderStyle)
		}
	}

	// Input row
	inputY := py + 1
	screen.SetContent(px, inputY, '|', nil, borderStyle)
	screen.SetContent(px+paletteW-1, inputY, '|', nil, borderStyle)
	for cx := px + 1; cx < px+paletteW-1; cx++ {
		screen.SetContent(cx, inputY, ' ', nil, bgStyle)
	}
	prompt := "> "
	for i, ch := range prompt {
		if px+1+i < px+paletteW-2 {
			screen.SetContent(px+1+i, inputY, ch, nil, bgStyle)
		}
	}
	inputStart := px + 1 + len(prompt)
	displayInput := p.input
	maxInputLen := paletteW - 2 - len(prompt) - 1
	if len(displayInput) > maxInputLen {
		displayInput = displayInput[len(displayInput)-maxInputLen:]
	}
	for i, ch := range displayInput {
		if inputStart+i < px+paletteW-1 {
			screen.SetContent(inputStart+i, inputY, ch, nil,
				tcell.StyleDefault.Foreground(ColorTextWhite).Background(ColorDialogBg))
		}
	}
	cursorX := inputStart + len([]rune(displayInput))
	if cursorX < px+paletteW-1 {
		screen.SetContent(cursorX, inputY, '_', nil,
			tcell.StyleDefault.Foreground(ColorTextWhite).Background(ColorDialogBg))
	}
	// Hint to the right of cursor if input is short
	if len([]rune(p.input)) < 3 && len([]rune(p.input)) > 0 {
		hint := " (min 3 chars)"
		hx := cursorX + 1
		for _, ch := range hint {
			if hx >= px+paletteW-1 {
				break
			}
			screen.SetContent(hx, inputY, ch, nil, dimStyle)
			hx++
		}
	}

	// Separator
	sepY := inputY + 1
	p.drawHLine(screen, px, sepY, paletteW, borderStyle)

	// Results list
	scrollOffset := 0
	if p.selected >= maxVisible {
		scrollOffset = p.selected - maxVisible + 1
	}

	for i := 0; i < maxVisible; i++ {
		idx := scrollOffset + i
		if idx >= len(p.results) {
			break
		}
		r := p.results[idx]
		iy := sepY + 1 + i

		isSelected := idx == p.selected
		dStyle := dimStyle
		lnStyle := lineNumStyle
		cStyle := contentStyle
		if isSelected {
			dStyle = dimHlStyle
			lnStyle = lineNumHlStyle
			cStyle = contentHlStyle
		}

		// Clear line background
		screen.SetContent(px, iy, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, iy, '|', nil, borderStyle)
		rowBg := bgStyle
		if isSelected {
			rowBg = hlStyle
		}
		for cx := px + 1; cx < px+paletteW-1; cx++ {
			screen.SetContent(cx, iy, ' ', nil, rowBg)
		}

		// Layout: "relpath:linenum: content"
		// relpath is dimmed, linenum is colored, content is normal
		cx := px + 2

		// Draw relative path (dimmed)
		for _, ch := range r.RelPath {
			if cx >= px+paletteW-2 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, dStyle)
			cx++
		}

		// Draw colon separator
		if cx < px+paletteW-2 {
			screen.SetContent(cx, iy, ':', nil, dStyle)
			cx++
		}

		// Draw line number (colored)
		lineNumStr := itoa(r.Line)
		for _, ch := range lineNumStr {
			if cx >= px+paletteW-2 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, lnStyle)
			cx++
		}

		// Draw colon + space before content
		for _, ch := range ": " {
			if cx >= px+paletteW-2 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, dStyle)
			cx++
		}

		// Draw content (trimmed line text)
		for _, ch := range r.Content {
			if cx >= px+paletteW-1 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, cStyle)
			cx++
		}
	}

	// Bottom border (or "no results" row)
	bottomY := sepY + 1 + maxVisible
	if maxVisible == 0 {
		noY := sepY + 1
		screen.SetContent(px, noY, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, noY, '|', nil, borderStyle)
		var msg string
		if len([]rune(p.input)) < 3 {
			msg = "  Type at least 3 characters to search"
		} else {
			msg = "  No matches found"
		}
		for i, ch := range msg {
			cx := px + 1 + i
			if cx < px+paletteW-1 {
				screen.SetContent(cx, noY, ch, nil, bgStyle)
			}
		}
		bottomY = noY + 1
	}
	p.drawHLine(screen, px, bottomY, paletteW, borderStyle)

	// Result count in bottom border
	if len(p.results) > 0 {
		var countStr string
		if len(p.results) >= 100 {
			countStr = " 100+ matches "
		} else {
			countStr = " " + itoa(len(p.results)) + " matches "
		}
		countX := px + paletteW - 1 - len(countStr) - 1
		if countX > px+1 {
			for i, ch := range countStr {
				if countX+i < px+paletteW-1 {
					screen.SetContent(countX+i, bottomY, ch, nil, borderStyle)
				}
			}
		}
	}
}

func (p *SearchPalette) drawHLine(screen tcell.Screen, x, y, w int, style tcell.Style) {
	screen.SetContent(x, y, '+', nil, style)
	for cx := x + 1; cx < x+w-1; cx++ {
		screen.SetContent(cx, y, '-', nil, style)
	}
	screen.SetContent(x+w-1, y, '+', nil, style)
}

// InputHandler handles all keyboard input for the search palette.
func (p *SearchPalette) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEscape:
			if p.onClose != nil {
				p.onClose()
			}

		case tcell.KeyEnter:
			if len(p.results) > 0 {
				result := p.results[p.selected]
				if p.onSelect != nil {
					p.onSelect(result)
				}
			} else {
				if p.onClose != nil {
					p.onClose()
				}
			}

		case tcell.KeyUp:
			if len(p.results) > 0 {
				p.selected--
				if p.selected < 0 {
					p.selected = len(p.results) - 1
				}
			}

		case tcell.KeyDown:
			if len(p.results) > 0 {
				p.selected++
				if p.selected >= len(p.results) {
					p.selected = 0
				}
			}

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(p.input) > 0 {
				runes := []rune(p.input)
				p.input = string(runes[:len(runes)-1])
				p.search(p.input)
			}

		case tcell.KeyRune:
			p.input += string(event.Rune())
			p.search(p.input)
		}
	})
}

// itoa converts an int to its decimal string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
