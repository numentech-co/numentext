package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/config"
	"numentext/internal/dap"
	"numentext/internal/editor"
	"numentext/internal/filetree"
	"numentext/internal/lsp"
	"numentext/internal/output"
	"numentext/internal/runner"
	"numentext/internal/terminal"
	"numentext/internal/ui"
)

// App is the main application
type App struct {
	tviewApp  *tview.Application
	layout    *ui.Layout
	editor    *editor.Editor
	menuBar   *ui.MenuBar
	statusBar *ui.StatusBar
	fileTree  *filetree.FileTree
	output    *output.Panel
	runner    *runner.Runner
	config    *config.Config
	workDir   string

	// Terminal
	termPanel    *terminal.Panel
	term         *terminal.Terminal
	termVisible  bool
	bottomFlex   *tview.Flex

	// LSP
	lspManager *lsp.Manager

	// DAP
	dapManager *dap.Manager
}

func New() *App {
	a := &App{
		tviewApp: tview.NewApplication(),
		runner:   runner.New(),
		config:   config.Load(),
	}

	a.workDir, _ = os.Getwd()

	a.setupUI()
	a.setupMenus()
	a.setupKeybindings()
	a.setupLSP()
	a.setupDAP()

	return a
}

func (a *App) setupUI() {
	// Create components
	a.editor = editor.NewEditor()
	a.editor.SetShowLineNumbers(a.config.ShowLineNum)
	a.menuBar = ui.NewMenuBar()
	a.statusBar = ui.NewStatusBar()
	a.fileTree = filetree.New(a.workDir)
	a.output = output.New()
	a.termPanel = terminal.NewPanel()

	// Wire callbacks
	a.editor.SetOnChange(func() {
		a.updateStatusBar()
	})
	a.editor.SetOnTabChange(func() {
		a.updateStatusBar()
	})

	a.fileTree.SetOnFileOpen(func(path string) {
		err := a.editor.OpenFile(path)
		if err != nil {
			a.output.AppendError("Error opening file: " + err.Error())
		}
		a.tviewApp.SetFocus(a.editor)
	})

	a.menuBar.SetOnAction(func() {
		a.tviewApp.SetFocus(a.editor)
	})

	// Bottom panel: output by default, can switch to terminal
	a.bottomFlex = tview.NewFlex()
	a.bottomFlex.AddItem(a.output, 0, 1, false)

	// Create layout
	a.layout = ui.NewLayout(a.menuBar, a.fileTree, a.editor, a.bottomFlex, a.statusBar)

	a.tviewApp.SetRoot(a.layout.Pages, true)
	a.tviewApp.SetFocus(a.editor)
	a.tviewApp.EnableMouse(true)
}

func (a *App) setupMenus() {
	// File menu — rebuilt on each open to include recent files
	fileMenu := &ui.Menu{
		Label: "File",
		Items: a.buildFileMenuItems(),
	}
	fileMenu.OnOpen = func() {
		fileMenu.Items = a.buildFileMenuItems()
	}

	// Edit menu
	editMenu := &ui.Menu{
		Label: "Edit",
		Items: []*ui.MenuItem{
			{Label: "Undo", Shortcut: "Ctrl+Z", Action: func() { a.editor.HandleAction(editor.ActionUndo, 0) }},
			{Label: "Redo", Shortcut: "Ctrl+Y", Action: func() { a.editor.HandleAction(editor.ActionRedo, 0) }},
			{Label: "Cut", Shortcut: "Ctrl+X", Action: func() { a.editor.HandleAction(editor.ActionCut, 0) }},
			{Label: "Copy", Shortcut: "Ctrl+C", Action: func() { a.editor.HandleAction(editor.ActionCopy, 0) }},
			{Label: "Paste", Shortcut: "Ctrl+V", Action: func() { a.editor.HandleAction(editor.ActionPaste, 0) }},
			{Label: "Select All", Shortcut: "Ctrl+A", Action: func() { a.editor.HandleAction(editor.ActionSelectAll, 0) }},
		},
	}

	// Search menu
	searchMenu := &ui.Menu{
		Label: "Search",
		Items: []*ui.MenuItem{
			{Label: "Find...", Shortcut: "Ctrl+F", Action: a.showFind},
			{Label: "Replace...", Shortcut: "Ctrl+H", Action: a.showReplace},
			{Label: "Go to Line...", Shortcut: "Ctrl+G", Action: a.showGoToLine},
			{Label: "Go to Definition", Shortcut: "F12", Action: a.goToDefinition},
			{Label: "Hover Info", Shortcut: "F11", Action: a.showHover},
		},
	}

	// Run menu
	runMenu := &ui.Menu{
		Label: "Run",
		Items: []*ui.MenuItem{
			{Label: "Run", Shortcut: "F5", Action: a.runFile},
			{Label: "Build", Shortcut: "F9", Action: a.buildFile},
			{Label: "Stop", Action: a.stopRun},
		},
	}

	// Debug menu
	debugMenu := &ui.Menu{
		Label: "Debug",
		Items: []*ui.MenuItem{
			{Label: "Start Debug", Shortcut: "F5", Action: a.startDebug},
			{Label: "Toggle Breakpoint", Shortcut: "F8", Action: a.toggleBreakpoint},
			{Label: "Continue", Shortcut: "F6", Action: a.debugContinue},
			{Label: "Step Over", Shortcut: "F7", Action: a.debugStepOver},
			{Label: "Step In", Action: a.debugStepIn},
			{Label: "Step Out", Action: a.debugStepOut},
			{Label: "Stop Debug", Action: a.stopDebug},
		},
	}

	// Tools menu
	toolsMenu := &ui.Menu{
		Label: "Tools",
		Items: []*ui.MenuItem{
			{Label: "Terminal", Shortcut: "Ctrl+`", Action: a.toggleTerminal},
			{Label: "Restart LSP", Action: a.restartLSP},
			{Label: "Clear Output", Action: func() { a.output.Clear() }},
			{Label: "Refresh File Tree", Action: func() { a.fileTree.Refresh() }},
		},
	}

	// Options menu
	optionsMenu := &ui.Menu{
		Label: "Options",
		Items: []*ui.MenuItem{
			{Label: "Toggle Line Numbers", Action: func() {
				a.config.ShowLineNum = !a.config.ShowLineNum
				a.editor.SetShowLineNumbers(a.config.ShowLineNum)
				a.config.Save()
			}},
		},
	}

	// Window menu
	windowMenu := &ui.Menu{
		Label: "Window",
		Items: []*ui.MenuItem{
			{Label: "Next Tab", Shortcut: "Ctrl+Tab", Action: a.nextTab},
			{Label: "Close Tab", Shortcut: "Ctrl+W", Action: a.closeTab},
		},
	}

	// Help menu
	helpMenu := &ui.Menu{
		Label: "Help",
		Items: []*ui.MenuItem{
			{Label: "About NumenText", Action: a.showAbout},
			{Label: "Keyboard Shortcuts", Action: a.showShortcuts},
		},
	}

	a.menuBar.AddMenu(fileMenu)
	a.menuBar.AddMenu(editMenu)
	a.menuBar.AddMenu(searchMenu)
	a.menuBar.AddMenu(runMenu)
	a.menuBar.AddMenu(debugMenu)
	a.menuBar.AddMenu(toolsMenu)
	a.menuBar.AddMenu(optionsMenu)
	a.menuBar.AddMenu(windowMenu)
	a.menuBar.AddMenu(helpMenu)
}

func (a *App) buildFileMenuItems() []*ui.MenuItem {
	items := []*ui.MenuItem{
		{Label: "New", Shortcut: "Ctrl+N", Action: a.newFile},
		{Label: "Open...", Shortcut: "Ctrl+O", Action: a.openFile},
		{Label: "Save", Shortcut: "Ctrl+S", Action: a.saveFile},
		{Label: "Save As...", Action: a.saveFileAs},
		{Label: "Close Tab", Shortcut: "Ctrl+W", Action: a.closeTab},
	}

	// Add recent files if any
	if len(a.config.RecentFiles) > 0 {
		items = append(items, &ui.MenuItem{Label: "---", Disabled: true})
		max := len(a.config.RecentFiles)
		if max > 10 {
			max = 10
		}
		for i := 0; i < max; i++ {
			path := a.config.RecentFiles[i]
			// Extract just the filename for display
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]
			p := path // capture for closure
			items = append(items, &ui.MenuItem{
				Label:  name,
				Action: func() { a.openRecentFile(p) },
			})
		}
	}

	items = append(items, &ui.MenuItem{Label: "---", Disabled: true})
	items = append(items, &ui.MenuItem{Label: "Exit", Shortcut: "Ctrl+Q", Action: a.quit})

	return items
}

func (a *App) openRecentFile(path string) {
	err := a.editor.OpenFile(path)
	if err != nil {
		a.output.AppendError("Error opening file: " + err.Error())
	}
	a.tviewApp.SetFocus(a.editor)
}

func (a *App) setupKeybindings() {
	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		mod := event.Modifiers()
		ctrl := mod&tcell.ModCtrl != 0

		// Check if a dialog is currently showing
		frontPage, _ := a.layout.Pages.GetFrontPage()
		hasDialog := frontPage != "main"

		// If a dialog is open, only intercept Escape (let dialog handle it)
		if hasDialog {
			return event
		}

		// If menu is open, handle menu-specific keys
		if a.menuBar.IsOpen() && key != tcell.KeyEscape {
			return event
		}

		switch key {
		case tcell.KeyEscape:
			if a.menuBar.IsOpen() {
				a.menuBar.Close()
				a.tviewApp.SetFocus(a.editor)
				return nil
			}
			if hasDialog {
				// Let the dialog handle Escape
				return event
			}
			// If terminal has focus, return to editor
			if a.termPanel.HasFocus() {
				a.tviewApp.SetFocus(a.editor)
				return nil
			}
			// Return focus to editor from file tree/output
			a.tviewApp.SetFocus(a.editor)
			return nil
		case tcell.KeyF6:
			a.debugContinue()
			return nil
		case tcell.KeyF7:
			a.debugStepOver()
			return nil
		case tcell.KeyF8:
			a.toggleBreakpoint()
			return nil
		case tcell.KeyF11:
			a.showHover()
			return nil
		case tcell.KeyF12:
			a.goToDefinition()
			return nil
		case tcell.KeyF5:
			if a.termPanel.HasFocus() {
				return event // let terminal handle it
			}
			a.runFile()
			return nil
		case tcell.KeyF9:
			if a.termPanel.HasFocus() {
				return event
			}
			a.buildFile()
			return nil
		case tcell.KeyF10:
			a.menuBar.Open(0)
			a.tviewApp.SetFocus(a.menuBar)
			return nil
		case tcell.KeyRune:
			if ctrl {
				switch event.Rune() {
				case '`':
					a.toggleTerminal()
					return nil
				case 'n':
					a.newFile()
					return nil
				case 'o':
					a.openFile()
					return nil
				case 's':
					if mod&tcell.ModShift != 0 {
						a.saveFileAs()
					} else {
						a.saveFile()
					}
					return nil
				case 'w':
					a.closeTab()
					return nil
				case 'q':
					a.quit()
					return nil
				case 'f':
					a.showFind()
					return nil
				case 'h':
					a.showReplace()
					return nil
				case 'g':
					a.showGoToLine()
					return nil
				}
			}
		case tcell.KeyTab:
			if ctrl {
				a.nextTab()
				return nil
			}
		}

		// Ctrl+1 through Ctrl+9 for tab switching
		if ctrl && key == tcell.KeyRune {
			r := event.Rune()
			if r >= '1' && r <= '9' {
				idx := int(r - '1')
				if idx < a.editor.TabCount() {
					a.editor.SetActiveTab(idx)
					return nil
				}
			}
		}

		return event
	})
}

func (a *App) updateStatusBar() {
	tab := a.editor.ActiveTab()
	if tab != nil {
		a.statusBar.Update(tab.Name, tab.CursorRow, tab.CursorCol, tab.Highlighter.Language(), tab.Buffer.Modified())
		// Show diagnostic message if cursor is on a diagnostic line
		if diag, ok := a.editor.DiagnosticAtLine(tab.CursorRow); ok {
			a.statusBar.SetMessage(diag.Message)
		}
	} else {
		a.statusBar.Update("", 0, 0, "", false)
		a.statusBar.SetMessage("NumenText - Press Ctrl+N for new file, Ctrl+O to open")
	}
}

// Actions
func (a *App) newFile() {
	a.editor.NewTab("untitled", "", "")
	a.tviewApp.SetFocus(a.editor)
}

func (a *App) openFile() {
	dialog := ui.OpenFileDialog(a.tviewApp, a.workDir, func(result ui.DialogResult) {
		a.layout.HideDialog("open")
		if result.Confirmed {
			err := a.editor.OpenFile(result.FilePath)
			if err != nil {
				a.output.AppendError("Error opening file: " + err.Error())
			} else {
				a.config.AddRecentFile(result.FilePath)
				a.config.Save()
			}
		}
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("open", dialog)
}

func (a *App) saveFile() {
	tab := a.editor.ActiveTab()
	if tab == nil {
		return
	}
	if tab.FilePath == "" {
		a.saveFileAs()
		return
	}
	err := a.editor.SaveCurrentFile()
	if err != nil {
		a.output.AppendError("Error saving: " + err.Error())
	} else {
		a.statusBar.SetMessage("File saved: " + tab.FilePath)
	}
}

func (a *App) saveFileAs() {
	tab := a.editor.ActiveTab()
	if tab == nil {
		return
	}
	currentPath := tab.FilePath
	if currentPath == "" {
		currentPath = a.workDir + "/untitled"
	}
	dialog := ui.SaveFileDialog(a.tviewApp, currentPath, func(result ui.DialogResult) {
		a.layout.HideDialog("saveas")
		if result.Confirmed {
			err := a.editor.SaveAs(result.FilePath)
			if err != nil {
				a.output.AppendError("Error saving: " + err.Error())
			} else {
				a.config.AddRecentFile(result.FilePath)
				a.config.Save()
				a.statusBar.SetMessage("File saved: " + result.FilePath)
			}
		}
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("saveas", dialog)
}

func (a *App) closeTab() {
	tab := a.editor.ActiveTab()
	if tab == nil {
		return
	}
	if tab.Buffer.Modified() {
		dialog := ui.ConfirmDialog(a.tviewApp, "Save changes to "+tab.Name+"?", func(yes bool) {
			a.layout.HideDialog("confirm")
			if yes {
				a.saveFile()
			}
			a.editor.CloseCurrentTab()
			a.tviewApp.SetFocus(a.editor)
		})
		a.layout.ShowDialog("confirm", dialog)
	} else {
		a.editor.CloseCurrentTab()
	}
}

func (a *App) quit() {
	// Check for unsaved files
	hasModified := false
	for _, tab := range a.editor.Tabs() {
		if tab.Buffer.Modified() {
			hasModified = true
			break
		}
	}

	if hasModified {
		dialog := ui.ConfirmDialog(a.tviewApp, "You have unsaved changes. Quit anyway?", func(yes bool) {
			if yes {
				a.tviewApp.Stop()
			}
			a.layout.HideDialog("quit")
			a.tviewApp.SetFocus(a.editor)
		})
		a.layout.ShowDialog("quit", dialog)
	} else {
		a.tviewApp.Stop()
	}
}

func (a *App) showFind() {
	dialog := ui.FindDialog(a.tviewApp, func(result ui.DialogResult) {
		if result.Confirmed {
			found := a.editor.Find(result.Text, true)
			if !found {
				a.statusBar.SetMessage("Not found: " + result.Text)
			}
		} else {
			a.layout.HideDialog("find")
			a.tviewApp.SetFocus(a.editor)
		}
	})
	a.layout.ShowDialog("find", dialog)
}

func (a *App) showReplace() {
	dialog := ui.ReplaceDialog(a.tviewApp,
		func(result ui.DialogResult) {
			// Find
			found := a.editor.Find(result.Text, true)
			if !found {
				a.statusBar.SetMessage("Not found: " + result.Text)
			}
		},
		func(result ui.DialogResult) {
			// Replace
			a.editor.Replace(result.Text, result.Text2)
		},
		func(result ui.DialogResult) {
			// Replace All
			count := a.editor.ReplaceAll(result.Text, result.Text2)
			a.statusBar.SetMessage(fmt.Sprintf("Replaced %d occurrences", count))
		},
		func() {
			// Close
			a.layout.HideDialog("replace")
			a.tviewApp.SetFocus(a.editor)
		},
	)
	a.layout.ShowDialog("replace", dialog)
}

func (a *App) showGoToLine() {
	dialog := ui.GoToLineDialog(a.tviewApp, func(result ui.DialogResult) {
		a.layout.HideDialog("gotoline")
		if result.Confirmed {
			lineNum, err := strconv.Atoi(result.Text)
			if err == nil {
				a.editor.GoToLine(lineNum)
			}
		}
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("gotoline", dialog)
}

func (a *App) nextTab() {
	count := a.editor.TabCount()
	if count <= 1 {
		return
	}
	next := (a.editor.ActiveTabIndex() + 1) % count
	a.editor.SetActiveTab(next)
}

func (a *App) runFile() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		a.output.AppendError("No file to run. Save the file first.")
		return
	}

	// Auto-save before running
	if tab.Buffer.Modified() {
		err := a.editor.SaveCurrentFile()
		if err != nil {
			a.output.AppendError("Error saving before run: " + err.Error())
			return
		}
	}

	a.output.Clear()
	a.output.AppendCommand(runner.FormatRunCommand(tab.FilePath))

	go func() {
		result := a.runner.Run(tab.FilePath)
		a.tviewApp.QueueUpdateDraw(func() {
			if result.Error != "" {
				a.output.AppendError(result.Error)
			}
			if result.Output != "" {
				a.output.AppendText(result.Output)
			}
			if result.ExitCode == 0 {
				a.output.AppendSuccess(fmt.Sprintf("\nProcess exited with code 0 (%.2fs)", result.Duration.Seconds()))
			} else {
				a.output.AppendError(fmt.Sprintf("\nProcess exited with code %d (%.2fs)", result.ExitCode, result.Duration.Seconds()))
			}
		})
	}()
}

func (a *App) buildFile() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		a.output.AppendError("No file to build. Save the file first.")
		return
	}

	// Auto-save before building
	if tab.Buffer.Modified() {
		err := a.editor.SaveCurrentFile()
		if err != nil {
			a.output.AppendError("Error saving before build: " + err.Error())
			return
		}
	}

	a.output.Clear()
	buildCmd := runner.FormatBuildCommand(tab.FilePath)
	if buildCmd == "" {
		a.output.AppendText("No build step required for this language.")
		return
	}
	a.output.AppendCommand(buildCmd)

	go func() {
		result := a.runner.Build(tab.FilePath)
		a.tviewApp.QueueUpdateDraw(func() {
			if result.Error != "" {
				a.output.AppendError(result.Error)
			}
			if result.Output != "" {
				a.output.AppendText(result.Output)
			}
			if result.ExitCode == 0 {
				a.output.AppendSuccess(fmt.Sprintf("Build successful (%.2fs)", result.Duration.Seconds()))
			}
		})
	}()
}

func (a *App) stopRun() {
	a.runner.Stop()
	a.output.AppendText("\nProcess stopped.")
}

func (a *App) showAbout() {
	dialog := ui.AboutDialog(a.tviewApp, func() {
		a.layout.HideDialog("about")
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("about", dialog)
}

func (a *App) showShortcuts() {
	text := tview.NewTextView()
	text.SetBackgroundColor(ui.ColorDialogBg)
	text.SetTextColor(ui.ColorStatusText)
	text.SetDynamicColors(true)
	text.SetBorder(true)
	text.SetBorderColor(ui.ColorStatusText)
	text.SetTitle(" Keyboard Shortcuts ")
	text.SetTitleColor(ui.ColorStatusText)

	content := `
 [white::b]File[-::-]
 Ctrl+N    New file
 Ctrl+O    Open file
 Ctrl+S    Save
 Ctrl+W    Close tab
 Ctrl+Q    Quit

 [white::b]Edit[-::-]
 Ctrl+Z    Undo
 Ctrl+Y    Redo
 Ctrl+X    Cut
 Ctrl+C    Copy
 Ctrl+V    Paste
 Ctrl+A    Select all
 Ctrl+D    Delete line

 [white::b]Search[-::-]
 Ctrl+F    Find
 Ctrl+H    Replace
 Ctrl+G    Go to line

 [white::b]Run[-::-]
 F5        Run
 F9        Build
 F10       Menu bar

 [white::b]Navigation[-::-]
 Ctrl+Tab       Next tab
 Ctrl+1-9       Switch tab
 Ctrl+Arrows    Word jump

 Press Escape to close
`
	text.SetText(content)
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.layout.HideDialog("shortcuts")
			a.tviewApp.SetFocus(a.editor)
			return nil
		}
		return event
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 30, 0, true).
			AddItem(nil, 0, 1, false),
			40, 0, true).
		AddItem(nil, 0, 1, false)

	a.layout.ShowDialog("shortcuts", modal)
}

func (a *App) setupLSP() {
	a.lspManager = lsp.NewManager(a.workDir)
	a.lspManager.OnStatus = func(msg string) {
		a.tviewApp.QueueUpdateDraw(func() {
			a.statusBar.SetMessage(msg)
		})
	}
	a.lspManager.OnDiagnostics = func(params lsp.PublishDiagnosticsParams) {
		a.tviewApp.QueueUpdateDraw(func() {
			// Convert LSP diagnostics to editor format
			filePath := lsp.URIToPath(params.URI)
			diags := make(map[int]editor.DiagnosticInfo)
			for _, d := range params.Diagnostics {
				diags[d.Range.Start.Line] = editor.DiagnosticInfo{
					Severity: d.Severity,
					Message:  d.Message,
				}
			}
			a.editor.SetDiagnostics(filePath, diags)

			count := len(params.Diagnostics)
			if count > 0 {
				a.statusBar.SetMessage(fmt.Sprintf("%d diagnostic(s)", count))
			}
		})
	}

	// Wire editor callbacks for LSP notifications
	a.editor.SetOnFileOpen(func(filePath, text string) {
		go a.lspManager.NotifyOpen(filePath, text)
	})
	a.editor.SetOnFileChange(func(filePath, text string) {
		go a.lspManager.NotifyChange(filePath, text)
	})
	a.editor.SetOnFileClose(func(filePath string) {
		go a.lspManager.NotifyClose(filePath)
	})

	// Completion
	a.editor.SetOnRequestComplete(func(filePath string, row, col int, callback func([]editor.CompletionItem)) {
		go func() {
			client := a.lspManager.ClientForFile(filePath)
			if client == nil {
				return
			}
			items, err := client.Completion(filePath, row, col)
			if err != nil || len(items) == 0 {
				return
			}
			// Convert LSP items to editor items
			editorItems := make([]editor.CompletionItem, len(items))
			for i, item := range items {
				insertText := item.InsertText
				if insertText == "" {
					insertText = item.Label
				}
				editorItems[i] = editor.CompletionItem{
					Label:      item.Label,
					Detail:     item.Detail,
					InsertText: insertText,
					Kind:       item.Kind,
				}
			}
			a.tviewApp.QueueUpdateDraw(func() {
				callback(editorItems)
			})
		}()
	})
}

func (a *App) goToDefinition() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	filePath := tab.FilePath
	row := tab.CursorRow
	col := tab.CursorCol

	go func() {
		client := a.lspManager.ClientForFile(filePath)
		if client == nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.statusBar.SetMessage("No language server available")
			})
			return
		}
		locs, err := client.Definition(filePath, row, col)
		if err != nil || len(locs) == 0 {
			a.tviewApp.QueueUpdateDraw(func() {
				a.statusBar.SetMessage("No definition found")
			})
			return
		}
		loc := locs[0]
		targetPath := lsp.URIToPath(loc.URI)
		targetLine := loc.Range.Start.Line

		a.tviewApp.QueueUpdateDraw(func() {
			if targetPath != filePath {
				if err := a.editor.OpenFile(targetPath); err != nil {
					a.statusBar.SetMessage("Cannot open: " + err.Error())
					return
				}
			}
			a.editor.GoToLine(targetLine + 1) // GoToLine is 1-based
			a.statusBar.SetMessage(fmt.Sprintf("Definition: %s:%d", targetPath, targetLine+1))
		})
	}()
}

func (a *App) showHover() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	filePath := tab.FilePath
	row := tab.CursorRow
	col := tab.CursorCol

	go func() {
		client := a.lspManager.ClientForFile(filePath)
		if client == nil {
			return
		}
		hover, err := client.Hover(filePath, row, col)
		if err != nil || hover == nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.statusBar.SetMessage("No hover information")
			})
			return
		}
		// Show hover content in status bar (single line)
		content := hover.Contents.Value
		// Strip markdown code fences
		content = strings.ReplaceAll(content, "```go\n", "")
		content = strings.ReplaceAll(content, "```\n", "")
		content = strings.ReplaceAll(content, "```", "")
		content = strings.TrimSpace(content)
		// Take first line only for status bar
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = content[:idx]
		}
		a.tviewApp.QueueUpdateDraw(func() {
			a.statusBar.SetMessage(content)
		})
	}()
}

func (a *App) setupDAP() {
	a.dapManager = dap.NewManager()
	a.dapManager.OnStatus = func(msg string) {
		a.tviewApp.QueueUpdateDraw(func() {
			a.statusBar.SetMessage(msg)
		})
	}
	a.dapManager.OnOutput = func(text string) {
		a.tviewApp.QueueUpdateDraw(func() {
			a.output.AppendText(text)
		})
	}
	a.dapManager.OnStopped = func(file string, line int, reason string) {
		a.tviewApp.QueueUpdateDraw(func() {
			if file != "" {
				tab := a.editor.ActiveTab()
				if tab == nil || tab.FilePath != file {
					_ = a.editor.OpenFile(file)
				}
				a.editor.GoToLine(line)
			}
			a.statusBar.SetMessage(fmt.Sprintf("Stopped: %s (line %d)", reason, line))
		})
	}
	a.dapManager.OnTerminated = func() {
		a.tviewApp.QueueUpdateDraw(func() {
			a.statusBar.SetMessage("Debug session ended")
		})
	}

	// Wire breakpoint display in editor gutter
	a.editor.SetHasBreakpoint(func(filePath string, line int) bool {
		return a.dapManager.HasBreakpoint(filePath, line)
	})
}

func (a *App) startDebug() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		a.statusBar.SetMessage("No file to debug")
		return
	}
	if tab.Buffer.Modified() {
		_ = a.editor.SaveCurrentFile()
	}
	a.output.Clear()
	go a.dapManager.StartSession(tab.FilePath)
}

func (a *App) stopDebug() {
	a.dapManager.StopSession()
}

func (a *App) toggleBreakpoint() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	a.dapManager.ToggleBreakpoint(tab.FilePath, tab.CursorRow+1) // DAP uses 1-based lines
}

func (a *App) debugContinue() {
	a.dapManager.Continue()
}

func (a *App) debugStepOver() {
	a.dapManager.StepOver()
}

func (a *App) debugStepIn() {
	a.dapManager.StepIn()
}

func (a *App) debugStepOut() {
	a.dapManager.StepOut()
}

func (a *App) restartLSP() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		a.statusBar.SetMessage("No file open for LSP restart")
		return
	}
	go a.lspManager.RestartForFile(tab.FilePath)
}

func (a *App) toggleTerminal() {
	if a.termVisible {
		a.closeTerminal()
	} else {
		a.openTerminal()
	}
}

func (a *App) openTerminal() {
	if a.term == nil {
		a.term = terminal.NewTerminal(80, 24)
		a.termPanel.SetTerminal(a.term)
		a.term.SetOnData(func() {
			a.tviewApp.QueueUpdateDraw(func() {})
		})
		err := a.term.Start("")
		if err != nil {
			a.output.AppendError("Failed to start terminal: " + err.Error())
			return
		}
	}

	a.bottomFlex.Clear()
	a.bottomFlex.AddItem(a.termPanel, 0, 1, true)
	a.termVisible = true
	a.tviewApp.SetFocus(a.termPanel)
}

func (a *App) closeTerminal() {
	a.bottomFlex.Clear()
	a.bottomFlex.AddItem(a.output, 0, 1, false)
	a.termVisible = false
	a.tviewApp.SetFocus(a.editor)
}

// Run starts the application
func (a *App) Run() error {
	defer a.lspManager.StopAll()
	defer a.dapManager.StopSession()
	if a.term != nil {
		defer a.term.Stop()
	}
	return a.tviewApp.Run()
}
