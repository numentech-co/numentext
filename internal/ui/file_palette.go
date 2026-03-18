package ui

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FileEntry represents a file found during directory walking.
type FileEntry struct {
	Name     string // filename only (e.g. "editor.go")
	RelPath  string // relative path from workDir (e.g. "internal/editor/editor.go")
	FullPath string // absolute path
}

// FilePalette is an overlay widget for quick file open with fuzzy filename matching.
// It renders directly to tcell.Screen, following the same pattern as CommandPalette.
type FilePalette struct {
	*tview.Box
	input    string
	files    []FileEntry
	filtered []FileEntry
	selected int
	onSelect func(entry FileEntry)
	onClose  func()
}

// NewFilePalette creates a FilePalette with the given file list.
// onSelect is called when the user presses Enter on a highlighted file.
// onClose is called when the palette is dismissed without selecting.
func NewFilePalette(files []FileEntry, onSelect func(FileEntry), onClose func()) *FilePalette {
	p := &FilePalette{
		Box:      tview.NewBox(),
		files:    files,
		onSelect: onSelect,
		onClose:  onClose,
	}
	p.refilter()
	return p
}

// refilter rebuilds the filtered slice based on the current input.
func (p *FilePalette) refilter() {
	if p.input == "" {
		p.filtered = make([]FileEntry, len(p.files))
		copy(p.filtered, p.files)
	} else {
		needle := strings.ToLower(p.input)
		type scored struct {
			entry FileEntry
			score int
		}
		var matches []scored
		for _, f := range p.files {
			score := fileMatchScore(needle, f)
			if score > 0 {
				matches = append(matches, scored{f, score})
			}
		}
		// Sort by score descending (higher = better match)
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		p.filtered = p.filtered[:0]
		for _, m := range matches {
			p.filtered = append(p.filtered, m.entry)
		}
	}
	if p.selected >= len(p.filtered) {
		p.selected = 0
	}
}

// fileMatchScore returns a positive score if needle matches the file entry,
// or 0 if there is no match. Higher scores indicate better matches.
//
// Matching strategies (in order of score weight):
//  1. Exact subsequence match on filename (case-insensitive) — score 10 + position bonus
//  2. CamelCase initials: uppercase letters from filename match needle as initials — score 8
//  3. Kebab/snake-case initials: first chars of dash/underscore-separated words — score 8
//  4. Subsequence match on full relative path — score 5
func fileMatchScore(needle string, entry FileEntry) int {
	lName := strings.ToLower(entry.Name)
	lPath := strings.ToLower(entry.RelPath)

	// Strategy 1: subsequence on filename
	if fuzzyMatch(needle, lName) {
		// Bonus: shorter filename relative to needle length means tighter match
		bonus := 10 + max(0, 20-len(lName))
		return bonus
	}

	// Strategy 2: CamelCase initials match
	if initialsMatch(needle, entry.Name, false) {
		return 8
	}

	// Strategy 3: Kebab/snake initials match
	if initialsMatch(needle, entry.Name, true) {
		return 8
	}

	// Strategy 4: subsequence on full relative path
	if fuzzyMatch(needle, lPath) {
		return 5
	}

	return 0
}

// initialsMatch checks whether needle matches the initials extracted from name.
// If splitOnSeparators is true, it splits on '-' and '_' (kebab/snake).
// If false, it extracts uppercase letters (CamelCase).
func initialsMatch(needle, name string, splitOnSeparators bool) bool {
	var initials strings.Builder
	if splitOnSeparators {
		// Strip extension for initial extraction
		base := strings.TrimSuffix(name, filepath.Ext(name))
		parts := strings.FieldsFunc(base, func(r rune) bool {
			return r == '-' || r == '_'
		})
		for _, p := range parts {
			if len(p) > 0 {
				initials.WriteRune(unicode.ToLower(rune(p[0])))
			}
		}
	} else {
		// Extract uppercase letters from CamelCase (skip first char to allow lowercase start)
		for i, r := range name {
			if i == 0 {
				initials.WriteRune(unicode.ToLower(r))
				continue
			}
			if unicode.IsUpper(r) {
				initials.WriteRune(unicode.ToLower(r))
			}
		}
	}
	return strings.HasPrefix(initials.String(), needle) || strings.Contains(initials.String(), needle)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Draw renders the file palette overlay centered near the top of the screen.
func (p *FilePalette) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)

	sw, sh := screen.Size()

	maxVisible := 16
	if maxVisible > len(p.filtered) {
		maxVisible = len(p.filtered)
	}
	paletteW := 72
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
	dimStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorDialogBg)
	dimHlStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorMenuHighlight)
	borderStyle := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorDialogBg)
	nameStyle := tcell.StyleDefault.Foreground(ColorTextPrimary).Background(ColorDialogBg)
	nameHlStyle := tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)

	// Top border with title
	p.drawHLine(screen, px, py, paletteW, borderStyle)
	title := " File Finder (Ctrl+P) "
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
				tcell.StyleDefault.Foreground(ColorTextPrimary).Background(ColorDialogBg))
		}
	}
	cursorX := inputStart + len([]rune(displayInput))
	if cursorX < px+paletteW-1 {
		screen.SetContent(cursorX, inputY, '_', nil,
			tcell.StyleDefault.Foreground(ColorTextPrimary).Background(ColorDialogBg))
	}

	// Separator
	sepY := inputY + 1
	p.drawHLine(screen, px, sepY, paletteW, borderStyle)

	// File list
	scrollOffset := 0
	if p.selected >= maxVisible {
		scrollOffset = p.selected - maxVisible + 1
	}

	for i := 0; i < maxVisible; i++ {
		idx := scrollOffset + i
		if idx >= len(p.filtered) {
			break
		}
		entry := p.filtered[idx]
		iy := sepY + 1 + i

		isSelected := idx == p.selected
		rowBg := bgStyle
		nStyle := nameStyle
		dStyle := dimStyle
		if isSelected {
			rowBg = hlStyle
			nStyle = nameHlStyle
			dStyle = dimHlStyle
		}

		// Clear line background
		screen.SetContent(px, iy, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, iy, '|', nil, borderStyle)
		for cx := px + 1; cx < px+paletteW-1; cx++ {
			screen.SetContent(cx, iy, ' ', nil, rowBg)
		}

		// Draw filename prominently (left side, with padding)
		nameRunes := []rune(entry.Name)
		cx := px + 2
		for _, ch := range nameRunes {
			if cx >= px+paletteW-2 {
				break
			}
			screen.SetContent(cx, iy, ch, nil, nStyle)
			cx++
		}

		// Draw relative directory path dimmed (right-aligned, showing dir portion)
		dir := filepath.Dir(entry.RelPath)
		if dir == "." {
			dir = ""
		}
		if dir != "" {
			dir = "  " + dir
			dirRunes := []rune(dir)
			// Available width after filename
			nameWidth := len(nameRunes) + 2 // 2 for left padding
			available := paletteW - 2 - nameWidth - 1
			if len(dirRunes) <= available {
				startX := px + 2 + len(nameRunes) + 1
				for j, ch := range dirRunes {
					dcx := startX + j
					if dcx >= px+paletteW-1 {
						break
					}
					screen.SetContent(dcx, iy, ch, nil, dStyle)
				}
			} else if available > 4 {
				// Truncate from left: "...foo/bar"
				truncated := dirRunes[len(dirRunes)-available+3:]
				dots := []rune("...")
				startX := px + 2 + len(nameRunes) + 1
				for j, ch := range dots {
					screen.SetContent(startX+j, iy, ch, nil, dStyle)
				}
				for j, ch := range truncated {
					dcx := startX + len(dots) + j
					if dcx >= px+paletteW-1 {
						break
					}
					screen.SetContent(dcx, iy, ch, nil, dStyle)
				}
			}
		}
	}

	// Bottom border (or "no results" row)
	bottomY := sepY + 1 + maxVisible
	if maxVisible == 0 {
		noY := sepY + 1
		screen.SetContent(px, noY, '|', nil, borderStyle)
		screen.SetContent(px+paletteW-1, noY, '|', nil, borderStyle)
		msg := "  No matching files"
		for i, ch := range msg {
			cx := px + 1 + i
			if cx < px+paletteW-1 {
				screen.SetContent(cx, noY, ch, nil, bgStyle)
			}
		}
		bottomY = noY + 1
	}
	p.drawHLine(screen, px, bottomY, paletteW, borderStyle)
}

func (p *FilePalette) drawHLine(screen tcell.Screen, x, y, w int, style tcell.Style) {
	screen.SetContent(x, y, '+', nil, style)
	for cx := x + 1; cx < x+w-1; cx++ {
		screen.SetContent(cx, y, '-', nil, style)
	}
	screen.SetContent(x+w-1, y, '+', nil, style)
}

// InputHandler handles all keyboard input for the file palette.
func (p *FilePalette) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEscape:
			if p.onClose != nil {
				p.onClose()
			}

		case tcell.KeyEnter:
			if len(p.filtered) > 0 {
				entry := p.filtered[p.selected]
				if p.onSelect != nil {
					p.onSelect(entry)
				}
			} else {
				if p.onClose != nil {
					p.onClose()
				}
			}

		case tcell.KeyUp:
			if len(p.filtered) > 0 {
				p.selected--
				if p.selected < 0 {
					p.selected = len(p.filtered) - 1
				}
			}

		case tcell.KeyDown:
			if len(p.filtered) > 0 {
				p.selected++
				if p.selected >= len(p.filtered) {
					p.selected = 0
				}
			}

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(p.input) > 0 {
				runes := []rune(p.input)
				p.input = string(runes[:len(runes)-1])
				p.selected = 0
				p.refilter()
			}

		case tcell.KeyRune:
			p.input += string(event.Rune())
			p.selected = 0
			p.refilter()
		}
	})
}

// WalkProjectFiles walks workDir recursively and returns all files as FileEntry
// values. It skips hidden directories and common non-source directories
// (.git, node_modules, vendor, .DS_Store) as well as binary file extensions.
func WalkProjectFiles(workDir string) []FileEntry {
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
		".so":    true,
		".dylib": true,
		".dll":   true,
		".class": true,
		".jar":   true,
		".zip":   true,
		".tar":   true,
		".gz":    true,
		".bz2":   true,
		".xz":    true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".ico":   true,
		".svg":   true,
		".pdf":   true,
		".db":    true,
		".sqlite": true,
	}

	var entries []FileEntry
	_ = filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		name := d.Name()

		if d.IsDir() {
			// Skip hidden dirs and known non-source dirs
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Skip binary extensions
		ext := strings.ToLower(filepath.Ext(name))
		if skipExts[ext] {
			return nil
		}

		rel, relErr := filepath.Rel(workDir, path)
		if relErr != nil {
			rel = path
		}

		entries = append(entries, FileEntry{
			Name:     name,
			RelPath:  rel,
			FullPath: path,
		})
		return nil
	})

	// Sort by filename for stable display
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries
}
