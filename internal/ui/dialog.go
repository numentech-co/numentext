package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// wrapDialogWithShadow wraps a dialog primitive to draw a shadow behind it in modern mode.
func wrapDialogWithShadow(inner tview.Primitive, width, height int) tview.Primitive {
	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(&shadowBox{inner: inner, w: width, h: height}, height+1, 0, true).
			AddItem(nil, 0, 1, false),
			width+1, 0, true).
		AddItem(nil, 0, 1, false)
	return wrapper
}

// shadowBox wraps a tview.Primitive to draw a shadow around it.
type shadowBox struct {
	tview.Box
	inner tview.Primitive
	w, h  int
}

func (s *shadowBox) Draw(screen tcell.Screen) {
	x, y, _, _ := s.GetRect()
	// Draw inner at (x, y) with size (s.w, s.h)
	s.inner.SetRect(x, y, s.w, s.h)
	s.inner.Draw(screen)
	// Draw shadow
	if Style.Modern {
		DrawShadow(screen, x, y, s.w, s.h)
	}
}

func (s *shadowBox) Focus(delegate func(p tview.Primitive)) {
	s.inner.Focus(delegate)
}

func (s *shadowBox) HasFocus() bool {
	return s.inner.HasFocus()
}

func (s *shadowBox) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.inner.InputHandler()
}

func (s *shadowBox) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.inner.MouseHandler()
}

// setModernTitle sets a dialog title with modern bracket style.
func setModernTitle(box interface{ SetTitle(string) *tview.Box }, title string) {
	if Style.Modern {
		box.SetTitle(Style.TitleLeft() + title + Style.TitleRight())
	} else {
		box.SetTitle(" " + title + " ")
	}
}

// Dialog types
type DialogType int

const (
	DialogOpen DialogType = iota
	DialogSave
	DialogFind
	DialogReplace
	DialogGoToLine
	DialogConfirm
	DialogAbout
)

// DialogResult holds the result of a dialog interaction
type DialogResult struct {
	Confirmed bool
	Text      string
	Text2     string // For replace dialog (replace with)
	FilePath  string
}

// OpenFileDialog creates a file open dialog
func OpenFileDialog(app *tview.Application, startDir string, onResult func(DialogResult)) tview.Primitive {
	if startDir == "" {
		startDir, _ = os.Getwd()
	}

	currentDir := startDir
	selectedFile := ""

	// Create form
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBg)
	form.SetFieldTextColor(ColorTextWhite)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorTextWhite)
	form.SetLabelColor(ColorStatusText)
	form.SetTitle(" Open File ")
	form.SetTitleColor(ColorStatusText)
	form.SetBorder(true)
	form.SetBorderColor(ColorStatusText)

	// File list
	fileList := tview.NewList()
	fileList.SetBackgroundColor(ColorBg)
	fileList.SetMainTextColor(ColorTextWhite)
	fileList.SetSecondaryTextColor(ColorTextGray)
	fileList.SetSelectedTextColor(ColorSelectedText)
	fileList.SetSelectedBackgroundColor(ColorSelected)
	fileList.ShowSecondaryText(false)

	pathInput := tview.NewInputField()
	pathInput.SetLabel("Path: ")
	pathInput.SetText(currentDir)
	pathInput.SetBackgroundColor(ColorDialogBg)
	pathInput.SetFieldBackgroundColor(ColorBg)
	pathInput.SetFieldTextColor(ColorTextWhite)
	pathInput.SetLabelColor(ColorStatusText)

	var populateFiles func(dir string)
	populateFiles = func(dir string) {
		fileList.Clear()
		currentDir = dir
		pathInput.SetText(dir)

		// Add parent directory
		fileList.AddItem("+ ..", "", 0, func() {
			parent := filepath.Dir(dir)
			populateFiles(parent)
		})

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		// Sort: dirs first
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			entryPath := filepath.Join(dir, name)
			if entry.IsDir() {
				p := entryPath
				fileList.AddItem("+ "+name, "", 0, func() {
					populateFiles(p)
				})
			} else {
				p := entryPath
				fileList.AddItem("  "+name, "", 0, func() {
					selectedFile = p
					onResult(DialogResult{Confirmed: true, FilePath: selectedFile})
				})
			}
		}
	}

	populateFiles(currentDir)

	pathInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			newPath := pathInput.GetText()
			info, err := os.Stat(newPath)
			if err == nil {
				if info.IsDir() {
					populateFiles(newPath)
					app.SetFocus(fileList)
				} else {
					onResult(DialogResult{Confirmed: true, FilePath: newPath})
				}
			}
		} else if key == tcell.KeyEscape {
			onResult(DialogResult{Confirmed: false})
		}
	})

	// Layout
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pathInput, 1, 0, false).
		AddItem(fileList, 0, 1, true)

	layout.SetBackgroundColor(ColorDialogBg)
	layout.SetBorder(true)
	layout.SetBorderColor(ColorStatusText)
	setModernTitle(layout, "Open File")
	layout.SetTitleColor(ColorStatusText)

	// Handle escape on file list
	fileList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onResult(DialogResult{Confirmed: false})
			return nil
		}
		if event.Key() == tcell.KeyTab {
			app.SetFocus(pathInput)
			return nil
		}
		return event
	})

	// Center the dialog with shadow
	return wrapDialogWithShadow(layout, 60, 20)
}

// SaveFileDialog creates a save file dialog
func SaveFileDialog(app *tview.Application, currentPath string, onResult func(DialogResult)) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBg)
	form.SetFieldTextColor(ColorTextWhite)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorTextWhite)
	form.SetLabelColor(ColorStatusText)
	form.SetBorder(true)
	form.SetBorderColor(ColorStatusText)
	setModernTitle(form, "Save As")
	form.SetTitleColor(ColorStatusText)

	form.AddInputField("File path:", currentPath, 50, nil, nil)
	form.AddButton("Save", func() {
		path := form.GetFormItemByLabel("File path:").(*tview.InputField).GetText()
		onResult(DialogResult{Confirmed: true, FilePath: path})
	})
	form.AddButton("Cancel", func() {
		onResult(DialogResult{Confirmed: false})
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onResult(DialogResult{Confirmed: false})
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(form, 60, 7)
}

// FindDialog creates a find dialog
func FindDialog(app *tview.Application, onResult func(DialogResult)) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBg)
	form.SetFieldTextColor(ColorTextWhite)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorTextWhite)
	form.SetLabelColor(ColorStatusText)
	form.SetBorder(true)
	form.SetBorderColor(ColorStatusText)
	setModernTitle(form, "Find")
	form.SetTitleColor(ColorStatusText)

	form.AddInputField("Search:", "", 40, nil, nil)
	form.AddButton("Find Next", func() {
		text := form.GetFormItemByLabel("Search:").(*tview.InputField).GetText()
		onResult(DialogResult{Confirmed: true, Text: text})
	})
	form.AddButton("Close", func() {
		onResult(DialogResult{Confirmed: false})
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onResult(DialogResult{Confirmed: false})
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(form, 55, 7)
}

// ReplaceDialog creates a find & replace dialog
func ReplaceDialog(app *tview.Application, onFind func(DialogResult), onReplace func(DialogResult), onReplaceAll func(DialogResult), onClose func()) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBg)
	form.SetFieldTextColor(ColorTextWhite)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorTextWhite)
	form.SetLabelColor(ColorStatusText)
	form.SetBorder(true)
	form.SetBorderColor(ColorStatusText)
	setModernTitle(form, "Replace")
	form.SetTitleColor(ColorStatusText)

	form.AddInputField("Find:", "", 40, nil, nil)
	form.AddInputField("Replace:", "", 40, nil, nil)
	form.AddButton("Find", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		onFind(DialogResult{Confirmed: true, Text: find})
	})
	form.AddButton("Replace", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		replace := form.GetFormItemByLabel("Replace:").(*tview.InputField).GetText()
		onReplace(DialogResult{Confirmed: true, Text: find, Text2: replace})
	})
	form.AddButton("Replace All", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		replace := form.GetFormItemByLabel("Replace:").(*tview.InputField).GetText()
		onReplaceAll(DialogResult{Confirmed: true, Text: find, Text2: replace})
	})
	form.AddButton("Close", func() {
		onClose()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onClose()
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(form, 60, 9)
}

// GoToLineDialog creates a go-to-line dialog
func GoToLineDialog(app *tview.Application, onResult func(DialogResult)) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBg)
	form.SetFieldTextColor(ColorTextWhite)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorTextWhite)
	form.SetLabelColor(ColorStatusText)
	form.SetBorder(true)
	form.SetBorderColor(ColorStatusText)
	setModernTitle(form, "Go to Line")
	form.SetTitleColor(ColorStatusText)

	form.AddInputField("Line number:", "", 20, tview.InputFieldInteger, nil)
	form.AddButton("Go", func() {
		text := form.GetFormItemByLabel("Line number:").(*tview.InputField).GetText()
		onResult(DialogResult{Confirmed: true, Text: text})
	})
	form.AddButton("Cancel", func() {
		onResult(DialogResult{Confirmed: false})
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onResult(DialogResult{Confirmed: false})
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(form, 45, 7)
}

// ConfirmDialog creates a confirmation dialog
func ConfirmDialog(app *tview.Application, message string, onResult func(bool)) tview.Primitive {
	modal := tview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetBackgroundColor(ColorDialogBg)
	modal.SetTextColor(ColorStatusText)
	modal.SetButtonBackgroundColor(ColorMenuHighlight)
	modal.SetButtonTextColor(ColorTextWhite)
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		onResult(buttonLabel == "Yes")
	})
	return modal
}

// AboutDialog creates the about dialog
func AboutDialog(app *tview.Application, onClose func()) tview.Primitive {
	text := tview.NewTextView()
	text.SetBackgroundColor(ColorDialogBg)
	text.SetTextColor(ColorStatusText)
	text.SetTextAlign(tview.AlignCenter)
	text.SetDynamicColors(true)
	text.SetBorder(true)
	text.SetBorderColor(ColorStatusText)
	setModernTitle(text, "About NumenText")
	text.SetTitleColor(ColorStatusText)

	content := `
[white::b]NumenText[-::-]
A Modern Terminal IDE

Version 1.0.0

Inspired by Borland C++ / Turbo C

Supports: C, C++, Python, Rust,
Go, JavaScript, TypeScript, Java

Press Escape to close
`
	text.SetText(content)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter {
			onClose()
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(text, 45, 16)
}
