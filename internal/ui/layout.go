package ui

import (
	"github.com/rivo/tview"
)

// Layout holds the main application layout
type Layout struct {
	Root     *tview.Flex
	MainGrid *tview.Flex

	MenuBar   *MenuBar
	FileTree  tview.Primitive
	Editor    tview.Primitive
	Output    tview.Primitive
	StatusBar *StatusBar

	Pages *tview.Pages
}

// NewLayout creates the main application layout
func NewLayout(menuBar *MenuBar, fileTree tview.Primitive, editor tview.Primitive, output tview.Primitive, statusBar *StatusBar) *Layout {
	l := &Layout{
		MenuBar:   menuBar,
		FileTree:  fileTree,
		Editor:    editor,
		Output:    output,
		StatusBar: statusBar,
	}

	// Middle section: file tree + editor
	middle := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(fileTree, 20, 0, false).
		AddItem(editor, 0, 1, true)

	// Main vertical layout: menu, middle, output, status
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(menuBar, 1, 0, false).
		AddItem(middle, 0, 3, true).
		AddItem(output, 8, 0, false).
		AddItem(statusBar, 1, 0, false)

	mainFlex.SetBackgroundColor(ColorBg)

	l.MainGrid = mainFlex

	// Pages for dialog overlays
	l.Pages = tview.NewPages()
	l.Pages.AddPage("main", mainFlex, true, true)

	l.Root = tview.NewFlex().AddItem(l.Pages, 0, 1, true)

	return l
}

// ShowDialog shows a modal dialog overlay
func (l *Layout) ShowDialog(name string, dialog tview.Primitive) {
	l.Pages.AddPage(name, dialog, true, true)
}

// HideDialog removes a dialog overlay
func (l *Layout) HideDialog(name string) {
	l.Pages.RemovePage(name)
}

// HasDialog checks if a dialog is showing
func (l *Layout) HasDialog(name string) bool {
	return l.Pages.HasPage(name)
}
