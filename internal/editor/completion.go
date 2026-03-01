package editor

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"numentext/internal/ui"
)

// CompletionItem represents a single completion suggestion
type CompletionItem struct {
	Label      string
	Detail     string
	InsertText string
	Kind       int
}

// CompletionPopup manages the autocomplete dropdown
type CompletionPopup struct {
	visible    bool
	items      []CompletionItem
	filtered   []CompletionItem
	selected   int
	prefix     string
	startCol   int // column where the prefix starts
	startRow   int // row where completion was triggered
}

func NewCompletionPopup() *CompletionPopup {
	return &CompletionPopup{}
}

// Show opens the popup with the given items and cursor context
func (cp *CompletionPopup) Show(items []CompletionItem, prefix string, row, col int) {
	cp.items = items
	cp.prefix = prefix
	cp.startRow = row
	cp.startCol = col - len(prefix)
	cp.selected = 0
	cp.filter()
	cp.visible = len(cp.filtered) > 0
}

// Hide closes the popup
func (cp *CompletionPopup) Hide() {
	cp.visible = false
	cp.items = nil
	cp.filtered = nil
	cp.selected = 0
	cp.prefix = ""
}

// Visible returns whether the popup is showing
func (cp *CompletionPopup) Visible() bool {
	return cp.visible
}

// UpdatePrefix updates the filter prefix as the user types
func (cp *CompletionPopup) UpdatePrefix(prefix string) {
	cp.prefix = prefix
	cp.selected = 0
	cp.filter()
	if len(cp.filtered) == 0 {
		cp.visible = false
	}
}

func (cp *CompletionPopup) filter() {
	if cp.prefix == "" {
		cp.filtered = cp.items
		return
	}
	lower := strings.ToLower(cp.prefix)
	cp.filtered = nil
	for _, item := range cp.items {
		if strings.HasPrefix(strings.ToLower(item.Label), lower) {
			cp.filtered = append(cp.filtered, item)
		}
	}
}

// MoveDown moves selection down
func (cp *CompletionPopup) MoveDown() {
	if len(cp.filtered) > 0 {
		cp.selected = (cp.selected + 1) % len(cp.filtered)
	}
}

// MoveUp moves selection up
func (cp *CompletionPopup) MoveUp() {
	if len(cp.filtered) > 0 {
		cp.selected--
		if cp.selected < 0 {
			cp.selected = len(cp.filtered) - 1
		}
	}
}

// Selected returns the currently selected item, or nil
func (cp *CompletionPopup) Selected() *CompletionItem {
	if cp.selected >= 0 && cp.selected < len(cp.filtered) {
		item := cp.filtered[cp.selected]
		return &item
	}
	return nil
}

// StartCol returns the column where the completion text starts
func (cp *CompletionPopup) StartCol() int {
	return cp.startCol
}

// Draw renders the completion popup onto the screen
func (cp *CompletionPopup) Draw(screen tcell.Screen, editorX, editorY, gutterWidth, scrollRow, scrollCol int) {
	if !cp.visible || len(cp.filtered) == 0 {
		return
	}

	// Position popup below the cursor
	popupX := editorX + gutterWidth + cp.startCol - scrollCol
	popupY := editorY + (cp.startRow - scrollRow) + 1

	// Calculate popup dimensions
	maxWidth := 0
	for _, item := range cp.filtered {
		w := len(item.Label)
		if item.Detail != "" {
			w += len(item.Detail) + 3
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += 4 // padding
	if maxWidth < 20 {
		maxWidth = 20
	}

	maxItems := len(cp.filtered)
	if maxItems > 10 {
		maxItems = 10
	}

	// Draw popup background and items
	bgStyle := tcell.StyleDefault.Background(ui.ColorMenuDropBg).Foreground(ui.ColorMenuDropText)
	selStyle := tcell.StyleDefault.Background(ui.ColorMenuHighlight).Foreground(ui.ColorMenuHlText)

	for i := 0; i < maxItems; i++ {
		item := cp.filtered[i]
		y := popupY + i
		style := bgStyle
		if i == cp.selected {
			style = selStyle
		}

		// Clear line
		for x := popupX; x < popupX+maxWidth; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}

		// Draw kind indicator
		kindCh := completionKindChar(item.Kind)
		screen.SetContent(popupX+1, y, kindCh, nil, style)

		// Draw label
		for j, ch := range item.Label {
			if popupX+3+j < popupX+maxWidth {
				screen.SetContent(popupX+3+j, y, ch, nil, style)
			}
		}

		// Draw detail on the right
		if item.Detail != "" {
			_, bg, _ := style.Decompose()
			detailStyle := tcell.StyleDefault.Foreground(ui.ColorTextGray).Background(bg)
			dx := popupX + maxWidth - len(item.Detail) - 1
			for j, ch := range item.Detail {
				screen.SetContent(dx+j, y, ch, nil, detailStyle)
			}
		}
	}
}

func completionKindChar(kind int) rune {
	switch kind {
	case 2: // Method
		return 'm'
	case 3: // Function
		return 'f'
	case 4: // Constructor
		return 'c'
	case 5: // Field
		return 'F'
	case 6: // Variable
		return 'v'
	case 7: // Class
		return 'C'
	case 8: // Interface
		return 'I'
	case 9: // Module
		return 'M'
	case 10: // Property
		return 'p'
	case 14: // Keyword
		return 'k'
	case 15: // Snippet
		return 's'
	default:
		return ' '
	}
}
