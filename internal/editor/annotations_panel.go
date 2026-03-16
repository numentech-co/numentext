package editor

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// AnnotationsPanel displays TODO/FIXME/etc. annotations from open files.
type AnnotationsPanel struct {
	*tview.Table
	annotations []Annotation
	onSelect    func(filePath string, line int) // callback when user selects an entry
}

// NewAnnotationsPanel creates a new annotations panel.
func NewAnnotationsPanel() *AnnotationsPanel {
	p := &AnnotationsPanel{
		Table: tview.NewTable(),
	}

	p.SetBackgroundColor(ui.ColorOutputBg)
	p.SetBorder(true)
	p.SetTitle(" Annotations (0) ")
	p.SetTitleColor(ui.ColorPanelBlurred)
	p.SetBorderColor(ui.ColorBorder)
	p.SetSelectable(true, false) // selectable rows, not columns
	p.SetFixed(1, 0)            // header row is fixed

	// Set up header row
	p.buildHeader()

	// Handle selection
	p.SetSelectedFunc(func(row, col int) {
		if row < 1 || row > len(p.annotations) {
			return
		}
		ann := p.annotations[row-1]
		if p.onSelect != nil {
			p.onSelect(ann.File, ann.Line)
		}
	})

	return p
}

// SetOnSelect sets the callback for when the user selects an annotation entry.
func (p *AnnotationsPanel) SetOnSelect(fn func(filePath string, line int)) {
	p.onSelect = fn
}

// Update refreshes the panel with the given annotations.
func (p *AnnotationsPanel) Update(annotations []Annotation) {
	// Sort by file then line
	sort.Slice(annotations, func(i, j int) bool {
		if annotations[i].File != annotations[j].File {
			return annotations[i].File < annotations[j].File
		}
		return annotations[i].Line < annotations[j].Line
	})

	p.annotations = annotations
	p.Clear()
	p.buildHeader()

	for i, ann := range annotations {
		row := i + 1
		shortFile := filepath.Base(ann.File)
		location := fmt.Sprintf("%s:%d", shortFile, ann.Line)

		locCell := tview.NewTableCell(tview.Escape(location)).
			SetTextColor(ui.ColorTextGray).
			SetMaxWidth(30).
			SetExpansion(0)

		tagColor := tagColor(ann.Tag)
		tagCell := tview.NewTableCell(tview.Escape(ann.Tag)).
			SetTextColor(tagColor).
			SetMaxWidth(10).
			SetExpansion(0)

		descCell := tview.NewTableCell(tview.Escape(ann.Text)).
			SetTextColor(ui.ColorTextWhite).
			SetExpansion(1)

		p.SetCell(row, 0, locCell)
		p.SetCell(row, 1, tagCell)
		p.SetCell(row, 2, descCell)
	}

	p.SetTitle(fmt.Sprintf(" Annotations (%d) ", len(annotations)))

	// Select the first data row if any
	if len(annotations) > 0 {
		p.Select(1, 0)
	}
}

func (p *AnnotationsPanel) buildHeader() {
	headerStyle := tcell.StyleDefault.
		Foreground(ui.ColorStatusText).
		Background(ui.ColorStatusBg).
		Bold(true)

	fileHeader := tview.NewTableCell("File:Line").
		SetStyle(headerStyle).
		SetSelectable(false).
		SetMaxWidth(30).
		SetExpansion(0)

	tagHeader := tview.NewTableCell("Tag").
		SetStyle(headerStyle).
		SetSelectable(false).
		SetMaxWidth(10).
		SetExpansion(0)

	descHeader := tview.NewTableCell("Description").
		SetStyle(headerStyle).
		SetSelectable(false).
		SetExpansion(1)

	p.SetCell(0, 0, fileHeader)
	p.SetCell(0, 1, tagHeader)
	p.SetCell(0, 2, descHeader)
}

// tagColor returns the display color for an annotation tag.
func tagColor(tag string) tcell.Color {
	switch tag {
	case "TODO":
		return tcell.NewRGBColor(0, 255, 255) // Cyan
	case "FIXME":
		return tcell.NewRGBColor(255, 255, 85) // Yellow
	case "BUG":
		return tcell.NewRGBColor(255, 85, 85) // Red
	case "HACK":
		return tcell.NewRGBColor(255, 170, 0) // Orange
	case "WONTFIX":
		return tcell.NewRGBColor(170, 170, 170) // Gray
	case "XXX":
		return tcell.NewRGBColor(255, 85, 255) // Magenta
	case "NOTE":
		return tcell.NewRGBColor(85, 255, 85) // Green
	default:
		return tcell.NewRGBColor(255, 255, 255) // White
	}
}
