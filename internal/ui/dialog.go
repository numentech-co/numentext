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
	UseRegex  bool   // Whether regex mode is enabled
	AllFiles  bool   // Whether to search/replace across all open files
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
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetTitle(" Open File ")
	form.SetTitleColor(ColorAccel)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)

	// File list
	fileList := tview.NewList()
	fileList.SetBackgroundColor(ColorBg)
	fileList.SetMainTextStyle(tcell.StyleDefault.Foreground(ColorText).Background(ColorBg))
	fileList.SetSelectedStyle(tcell.StyleDefault.Foreground(ColorSelectedText).Background(ColorSelected))
	fileList.ShowSecondaryText(false)

	pathInput := tview.NewInputField()
	pathInput.SetLabel("Path: ")
	pathInput.SetText(currentDir)
	pathInput.SetBackgroundColor(ColorDialogBg)
	pathInput.SetFieldBackgroundColor(ColorBgDarker)
	pathInput.SetFieldTextColor(ColorText)
	pathInput.SetLabelStyle(tcell.StyleDefault.Foreground(ColorText).Background(ColorDialogBg))

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
	layout.SetBorderColor(ColorBorder)
	setModernTitle(layout, "Open File")
	layout.SetTitleColor(ColorAccel)

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
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, "Save As")
	form.SetTitleColor(ColorAccel)

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
	useRegex := false

	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, "Find")
	form.SetTitleColor(ColorAccel)

	form.AddInputField("Search:", "", 40, nil, nil)
	form.AddCheckbox("Regex:", false, func(checked bool) {
		useRegex = checked
	})
	form.AddButton("Find Next", func() {
		text := form.GetFormItemByLabel("Search:").(*tview.InputField).GetText()
		onResult(DialogResult{Confirmed: true, Text: text, UseRegex: useRegex})
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

	return wrapDialogWithShadow(form, 55, 9)
}

// ReplaceDialog creates a find & replace dialog
func ReplaceDialog(app *tview.Application, onFind func(DialogResult), onReplace func(DialogResult), onReplaceAll func(DialogResult), onClose func()) tview.Primitive {
	useRegex := false
	allFiles := false

	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, "Replace")
	form.SetTitleColor(ColorAccel)

	form.AddInputField("Find:", "", 40, nil, nil)
	form.AddInputField("Replace:", "", 40, nil, nil)
	form.AddCheckbox("Regex:", false, func(checked bool) {
		useRegex = checked
	})
	form.AddCheckbox("All files:", false, func(checked bool) {
		allFiles = checked
	})
	form.AddButton("Find", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		onFind(DialogResult{Confirmed: true, Text: find, UseRegex: useRegex, AllFiles: allFiles})
	})
	form.AddButton("Replace", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		replace := form.GetFormItemByLabel("Replace:").(*tview.InputField).GetText()
		onReplace(DialogResult{Confirmed: true, Text: find, Text2: replace, UseRegex: useRegex, AllFiles: allFiles})
	})
	form.AddButton("Replace All", func() {
		find := form.GetFormItemByLabel("Find:").(*tview.InputField).GetText()
		replace := form.GetFormItemByLabel("Replace:").(*tview.InputField).GetText()
		onReplaceAll(DialogResult{Confirmed: true, Text: find, Text2: replace, UseRegex: useRegex, AllFiles: allFiles})
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

	return wrapDialogWithShadow(form, 60, 13)
}

// GoToLineDialog creates a go-to-line dialog
func GoToLineDialog(app *tview.Application, onResult func(DialogResult)) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, "Go to Line")
	form.SetTitleColor(ColorAccel)

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
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, message)
	form.SetTitleColor(ColorText)

	form.AddButton("Yes", func() {
		onResult(true)
	})
	form.AddButton("No", func() {
		onResult(false)
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onResult(false)
			return nil
		}
		return event
	})

	return wrapDialogWithShadow(form, 45, 5)
}

// AboutDialog creates the about dialog
func AboutDialog(app *tview.Application, onClose func()) tview.Primitive {
	text := tview.NewTextView()
	text.SetBackgroundColor(ColorDialogBg)
	text.SetTextColor(ColorText)
	text.SetTextAlign(tview.AlignCenter)
	text.SetDynamicColors(true)
	text.SetBorder(true)
	text.SetBorderColor(ColorBorder)
	setModernTitle(text, "About NumenText")
	text.SetTitleColor(ColorAccel)

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

// MessageDialog creates a simple informational dialog with a title and body text.
func MessageDialog(app *tview.Application, title, body string, onClose func()) tview.Primitive {
	text := tview.NewTextView()
	text.SetBackgroundColor(ColorDialogBg)
	text.SetTextColor(ColorStatusText)
	text.SetDynamicColors(true)
	text.SetScrollable(true)
	text.SetBorder(true)
	text.SetBorderColor(ColorBorder)
	setModernTitle(text, title)
	text.SetTitleColor(ColorAccel)
	text.SetText(tview.Escape(body))

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter {
			onClose()
			return nil
		}
		return event
	})

	// Calculate dialog size based on content
	lines := 1
	maxWidth := 0
	lineWidth := 0
	for _, ch := range body {
		if ch == '\n' {
			lines++
			if lineWidth > maxWidth {
				maxWidth = lineWidth
			}
			lineWidth = 0
		} else {
			lineWidth++
		}
	}
	if lineWidth > maxWidth {
		maxWidth = lineWidth
	}
	width := maxWidth + 4
	if width < 40 {
		width = 40
	}
	if width > 70 {
		width = 70
	}
	height := lines + 4
	if height > 20 {
		height = 20
	}

	return wrapDialogWithShadow(text, width, height)
}

// GoToAddressDialog creates a go-to-address dialog for hex view (accepts hex offset).
func GoToAddressDialog(app *tview.Application, onResult func(DialogResult)) tview.Primitive {
	form := tview.NewForm()
	form.SetBackgroundColor(ColorDialogBg)
	form.SetFieldBackgroundColor(ColorBgDarker)
	form.SetFieldTextColor(ColorText)
	form.SetButtonBackgroundColor(ColorMenuHighlight)
	form.SetButtonTextColor(ColorMenuHlText)
	form.SetLabelColor(ColorText)
	form.SetBorder(true)
	form.SetBorderColor(ColorBorder)
	setModernTitle(form, "Go to Address")
	form.SetTitleColor(ColorAccel)

	form.AddInputField("Hex offset:", "", 20, nil, nil)
	form.AddButton("Go", func() {
		text := form.GetFormItemByLabel("Hex offset:").(*tview.InputField).GetText()
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
