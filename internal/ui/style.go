package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// UIStyle controls the visual style of the application.
// Modern mode uses Unicode characters; classic mode uses ASCII only.
type UIStyle struct {
	Modern  bool
	IconSet string // "ascii", "unicode", "nerd-font"
}

// Style is the global style instance, initialized from config.
var Style = UIStyle{Modern: true, IconSet: "unicode"}

// InitStyle sets the global style from config values.
// If running on a basic terminal (TERM=linux, TERM=dumb), falls back to classic/ascii.
func InitStyle(uiStyle, iconSet string) {
	Style.Modern = uiStyle != "classic"
	switch iconSet {
	case "ascii", "unicode", "nerd-font":
		Style.IconSet = iconSet
	default:
		Style.IconSet = "unicode"
	}

	// Auto-detect basic terminals that can't render Unicode
	term := os.Getenv("TERM")
	if term == "linux" || term == "dumb" || term == "vt100" || term == "vt220" {
		Style.Modern = false
		if Style.IconSet == "unicode" || Style.IconSet == "nerd-font" {
			Style.IconSet = "ascii"
		}
	}
}

// --- Directory icons ---

func (s UIStyle) DirIconOpen() string {
	if s.IconSet == "nerd-font" {
		return "\uf07c" // nf-fa-folder_open
	}
	if s.Modern {
		return "\u25bc" // ▼
	}
	return "+"
}

func (s UIStyle) DirIconClosed() string {
	if s.IconSet == "nerd-font" {
		return "\uf07b" // nf-fa-folder
	}
	if s.Modern {
		return "\u25b6" // ▶
	}
	return "+"
}

// --- File icons ---

// fileIconsASCII maps extensions to single ASCII characters.
var fileIconsASCII = map[string]string{
	".go": "g", ".c": "c", ".h": "c", ".cpp": "c", ".cc": "c", ".cxx": "c", ".hpp": "c",
	".py": "p", ".rs": "r", ".js": "j", ".jsx": "j", ".ts": "t", ".tsx": "t",
	".java": "j", ".json": "~", ".md": "m", ".html": "h", ".htm": "h",
	".css": "#", ".sh": "$", ".bash": "$", ".yaml": "y", ".yml": "y",
}

// fileIconsUnicode maps extensions to Unicode symbols.
var fileIconsUnicode = map[string]string{
	".go": "\u25c6", ".c": "\u25a0", ".h": "\u25a0", ".cpp": "\u25a0", ".cc": "\u25a0", ".cxx": "\u25a0", ".hpp": "\u25a0",
	".py": "\u25c8", ".rs": "\u25a3", ".js": "\u25cb", ".jsx": "\u25cb", ".ts": "\u25ca", ".tsx": "\u25ca",
	".java": "\u25cf", ".json": "\u25e6", ".md": "\u25ab", ".html": "\u25a1", ".htm": "\u25a1",
	".css": "\u25a8", ".sh": "\u25b7", ".bash": "\u25b7", ".yaml": "\u25aa", ".yml": "\u25aa",
}

// fileIconsNerdFont maps extensions to Nerd Font glyphs.
var fileIconsNerdFont = map[string]string{
	".go": "\ue627", ".c": "\ue61e", ".h": "\ue61e", ".cpp": "\ue61d", ".cc": "\ue61d", ".cxx": "\ue61d", ".hpp": "\ue61d",
	".py": "\ue73c", ".rs": "\ue7a8", ".js": "\ue74e", ".jsx": "\ue7ba", ".ts": "\ue628", ".tsx": "\ue7ba",
	".java": "\ue738", ".json": "\ue60b", ".md": "\ue73e", ".html": "\ue736", ".htm": "\ue736",
	".css": "\ue749", ".sh": "\ue795", ".bash": "\ue795", ".yaml": "\ue6a8", ".yml": "\ue6a8",
}

func (s UIStyle) FileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	var icons map[string]string
	switch s.IconSet {
	case "nerd-font":
		icons = fileIconsNerdFont
	case "unicode":
		icons = fileIconsUnicode
	default:
		icons = fileIconsASCII
	}
	if icon, ok := icons[ext]; ok {
		return icon
	}
	if s.IconSet == "ascii" {
		return "-"
	}
	if s.IconSet == "nerd-font" {
		return "\uf15b" // nf-fa-file
	}
	return "\u25a2" // ▢
}

// --- Indicators ---

func (s UIStyle) ModifiedIndicator() string {
	if s.Modern {
		return "\u25cf" // ●
	}
	return "[modified]"
}

func (s UIStyle) ModifiedDot() string {
	if s.Modern {
		return "\u25cf" // ●
	}
	return "*"
}

func (s UIStyle) WrapIndicator() string {
	if s.Modern {
		return "\u21a9" // ↩
	}
	return "WRAP"
}

// --- Diagnostic markers ---

func (s UIStyle) ErrorMarker() rune {
	if s.Modern {
		return '\u2716' // ✖
	}
	return 'E'
}

func (s UIStyle) WarningMarker() rune {
	if s.Modern {
		return '\u26a0' // ⚠
	}
	return 'W'
}

func (s UIStyle) InfoMarker() rune {
	if s.Modern {
		return '\u2139' // ℹ
	}
	return '!'
}

// --- UI element characters ---

func (s UIStyle) MenuShadowChar() rune {
	if s.Modern {
		return '\u2591' // ░
	}
	return ' '
}

func (s UIStyle) TabSeparator() rune {
	if s.Modern {
		return '\u2502' // │
	}
	return '|'
}

func (s UIStyle) BreadcrumbSep() string {
	if s.Modern {
		return "\u25b8" // ▸
	}
	return ">"
}

func (s UIStyle) MenuSeparator() rune {
	if s.Modern {
		return '\u2500' // ─
	}
	return '-'
}

// --- Scrollbar characters ---

func (s UIStyle) ScrollTrack() rune {
	return '\u2591' // ░
}

func (s UIStyle) ScrollThumb() rune {
	return '\u2588' // █
}

// --- Dialog title brackets ---

func (s UIStyle) TitleLeft() string {
	if s.Modern {
		return "\u2524 " // ┤
	}
	return " "
}

func (s UIStyle) TitleRight() string {
	if s.Modern {
		return " \u251c" // ├
	}
	return " "
}

// DrawShadow draws a 1-cell shadow on the right and bottom edges of a rect.
// Used by dialogs and menus in modern mode.
func DrawShadow(screen tcell.Screen, x, y, w, h int) {
	if !Style.Modern {
		return
	}
	shadowStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack)
	ch := Style.MenuShadowChar()
	// Right edge shadow
	for row := y + 1; row < y+h+1; row++ {
		screen.SetContent(x+w, row, ch, nil, shadowStyle)
	}
	// Bottom edge shadow
	for col := x + 1; col < x+w+1; col++ {
		screen.SetContent(col, y+h, ch, nil, shadowStyle)
	}
}
