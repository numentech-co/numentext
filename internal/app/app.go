package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/config"
	"numentext/internal/dap"
	"numentext/internal/editor"
	"numentext/internal/editor/keymode"
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
	terms        []*terminal.Terminal
	activeTermIdx int
	termVisible  bool
	bottomFlex   *tview.Flex

	// Panel focus tracking
	focusedPanel string // "filetree", "editor", "output", "terminal"

	// LSP
	lspManager *lsp.Manager

	// DAP
	dapManager *dap.Manager

	// Mouse drag state for panel resizing
	dragging    string // "", "vertical", "horizontal"
	dragStartX  int
	dragStartY  int
	dragStartW  int // file tree width at drag start
	dragStartH  int // output height at drag start

	// Build error navigation
	buildErrors      []runner.BuildError
	buildErrorIdx    int  // current error index (0-based), -1 if none
	navigatingErrors bool // true while jumping to an error (suppresses clear)

	// Cross-file search results
	searchResults   []editor.SearchResult
	searchResultIdx int
}

func New() *App {
	a := &App{
		tviewApp:      tview.NewApplication(),
		runner:        runner.New(),
		config:        config.Load(),
		buildErrorIdx:   -1,
		searchResultIdx: -1,
	}

	a.workDir, _ = os.Getwd()

	// Initialize UI style and theme from config
	ui.InitStyle(a.config.UIStyle, a.config.IconSet)
	ui.ApplyTheme(a.config.Theme)
	a.applyBorderStyle()

	a.setupUI()
	a.setupMenus()
	a.setupKeybindings()
	a.setupMouse()
	a.setupLSP()
	a.setupDAP()

	return a
}

func (a *App) setupUI() {
	// Create components
	a.editor = editor.NewEditor()
	a.editor.SetShowLineNumbers(a.config.ShowLineNum)
	a.editor.SetWordWrap(a.config.WordWrap)
	a.editor.SetTabSize(a.config.TabSize)
	a.menuBar = ui.NewMenuBar()
	a.statusBar = ui.NewStatusBar()
	a.statusBar.SetWordWrap(a.config.WordWrap)
	a.fileTree = filetree.New(a.workDir)
	a.output = output.New()
	a.termPanel = terminal.NewPanel()
	a.termPanel.SetOnStatus(func(msg string) {
		a.statusBar.SetMessage(msg)
	})

	// Wire callbacks
	a.editor.SetOnChange(func() {
		a.updateStatusBar()
		// Clear build error markers when user edits a file (not during error navigation)
		if len(a.buildErrors) > 0 && !a.navigatingErrors {
			a.clearBuildErrors()
		}
	})
	a.editor.SetOnTabChange(func() {
		a.updateStatusBar()
	})

	// Status message callback for editor
	a.editor.SetOnStatusMessage(func(msg string) {
		a.statusBar.SetMessage(msg)
	})

	// Search callbacks for Vi/Helix modes
	a.editor.SetOnSearchForward(func() {
		a.showFind()
	})
	a.editor.SetOnSearchNext(func() {
		if !a.editor.Find("", true) {
			a.statusBar.SetMessage("No matches")
		}
	})
	a.editor.SetOnSearchPrev(func() {
		if !a.editor.Find("", false) {
			a.statusBar.SetMessage("No matches")
		}
	})

	a.fileTree.SetOnFileOpen(func(path string) {
		err := a.editor.OpenFile(path)
		if err != nil {
			a.output.AppendError("Error opening file: " + err.Error())
		}
		a.focusPanel("editor")
	})

	a.menuBar.SetOnAction(func() {
		a.tviewApp.SetFocus(a.editor)
	})

	// Auto-show/hide output panel based on content
	a.output.SetOnChange(func(hasContent bool) {
		if hasContent {
			a.layout.SetOutputVisible(true, a.config.OutputHeight)
		} else if !a.termVisible {
			a.layout.SetOutputVisible(false, 0)
		}
	})

	// Bottom panel: output by default, can switch to terminal
	a.bottomFlex = tview.NewFlex()
	a.bottomFlex.AddItem(a.output, 0, 1, false)

	// Create layout
	a.layout = ui.NewLayout(a.menuBar, a.fileTree, a.editor, a.bottomFlex, a.statusBar)

	// Apply persisted panel sizes from config
	if a.config.FileTreeWidth > 0 {
		a.layout.SetFileTreeWidth(a.config.FileTreeWidth)
	}

	a.tviewApp.SetRoot(a.layout.Pages, true)
	a.tviewApp.SetFocus(a.editor)
	a.tviewApp.EnableMouse(true)

	// Init panel focus tracking — editor is focused by default
	a.focusedPanel = "editor"
	a.statusBar.SetFocusedPanel("Editor")
	a.updatePanelBorders()

	// Init keyboard mode from config
	a.setKeyboardMode(a.config.KeyboardMode)
}

func (a *App) setupMenus() {
	// File menu — rebuilt on each open to include recent files
	fileMenu := &ui.Menu{
		Label: "File",
		Accel: 'f',
		Items: a.buildFileMenuItems(),
	}
	fileMenu.OnOpen = func() {
		fileMenu.Items = a.buildFileMenuItems()
	}

	// Edit menu
	editMenu := &ui.Menu{
		Label: "Edit",
		Accel: 'e',
		Items: []*ui.MenuItem{
			{Label: "Undo", Shortcut: "Ctrl+Z", Action: func() { a.editor.HandleAction(editor.ActionUndo, 0) }},
			{Label: "Redo", Shortcut: "Ctrl+Y", Action: func() { a.editor.HandleAction(editor.ActionRedo, 0) }},
			{Label: "Cut", Shortcut: "Ctrl+X", Action: func() { a.editor.HandleAction(editor.ActionCut, 0) }},
			{Label: "Copy", Shortcut: "Ctrl+C", Action: func() { a.editor.HandleAction(editor.ActionCopy, 0) }},
			{Label: "Paste", Shortcut: "Ctrl+V", Action: func() { a.editor.HandleAction(editor.ActionPaste, 0) }},
			{Label: "Select All", Shortcut: "Ctrl+A", Accel: 'a', Action: func() { a.editor.HandleAction(editor.ActionSelectAll, 0) }},
			{Label: "---"},
			{Label: "Line Endings: LF", Action: func() {
				a.editor.SetLineEnding(editor.LineEndingLF)
				a.updateStatusBar()
			}},
			{Label: "Line Endings: CRLF", Action: func() {
				a.editor.SetLineEnding(editor.LineEndingCRLF)
				a.updateStatusBar()
			}},
			{Label: "Line Endings: CR", Action: func() {
				a.editor.SetLineEnding(editor.LineEndingCR)
				a.updateStatusBar()
			}},
			{Label: "---"},
			{Label: "Toggle UTF-8 BOM", Action: func() {
				a.editor.SetBOM(!a.editor.HasBOM())
				a.updateStatusBar()
			}},
			{Label: "---"},
			{Label: "Convert Tabs to Spaces", Action: func() {
				a.editor.ConvertTabsToSpaces()
			}},
			{Label: "Convert Spaces to Tabs", Action: func() {
				a.editor.ConvertSpacesToTabs()
			}},
		},
	}

	// Search menu
	searchMenu := &ui.Menu{
		Label: "Search",
		Accel: 's',
		Items: []*ui.MenuItem{
			{Label: "Find...", Shortcut: "Ctrl+F", Action: a.showFind},
			{Label: "Replace...", Shortcut: "Ctrl+H", Action: a.showReplace},
			{Label: "Search in Files...", Shortcut: "Ctrl+Shift+F", Accel: 'i', Action: a.showSearchPalette},
			{Label: "Go to Line...", Shortcut: "Ctrl+G", Accel: 'l', Action: a.showGoToLine},
			{Label: "Go to Definition", Shortcut: "F12", Accel: 'd', Action: a.goToDefinition},
			{Label: "Hover Info", Shortcut: "F11", Action: a.showHover},
			{Label: "---", Disabled: true},
			{Label: "Toggle Bookmark", Shortcut: "Ctrl+F2", Accel: 'b', Action: a.toggleBookmark},
			{Label: "Next Bookmark", Shortcut: "F2", Accel: 'n', Action: a.nextBookmark},
			{Label: "Prev Bookmark", Shortcut: "Shift+F2", Action: a.prevBookmark},
			{Label: "All Bookmarks...", Shortcut: "Ctrl+Shift+F2", Action: a.showBookmarksPanel},
		},
	}

	// Run menu
	runMenu := &ui.Menu{
		Label: "Run",
		Accel: 'r',
		Items: []*ui.MenuItem{
			{Label: "Run", Shortcut: "F5", Action: a.runFile},
			{Label: "Build", Shortcut: "F9", Action: a.buildFile},
			{Label: "Stop", Action: a.stopRun},
		},
	}

	// Debug menu
	debugMenu := &ui.Menu{
		Label: "Debug",
		Accel: 'd',
		Items: []*ui.MenuItem{
			{Label: "Start Debug", Shortcut: "F5", Action: a.startDebug},
			{Label: "Toggle Breakpoint", Shortcut: "F8", Action: a.toggleBreakpoint},
			{Label: "Continue", Shortcut: "F6", Action: a.debugContinue},
			{Label: "Step Over", Shortcut: "F7", Accel: 'v', Action: a.debugStepOver},
			{Label: "Step In", Shortcut: "", Accel: 'i', Action: a.debugStepIn},
			{Label: "Step Out", Accel: 'o', Action: a.debugStepOut},
			{Label: "Stop Debug", Accel: 'p', Action: a.stopDebug},
		},
	}

	// Tools menu
	toolsMenu := &ui.Menu{
		Label: "Tools",
		Accel: 't',
		Items: []*ui.MenuItem{
			{Label: "Terminal", Shortcut: "Ctrl+`", Action: a.toggleTerminal},
			{Label: "Format File", Shortcut: "Ctrl+Shift+I", Accel: 'o', Action: a.formatFile},
			{Label: "Lint File", Shortcut: "Ctrl+Shift+L", Action: a.lintFile},
			{Label: "Toggle Block Mode", Accel: 'b', Action: a.toggleBlockMode},
			{Label: "Restart LSP", Action: a.restartLSP},
			{Label: "Clear Output", Action: func() {
				a.output.Clear()
			}},
			{Label: "Refresh File Tree", Accel: 'f', Action: func() { a.fileTree.Refresh() }},
		},
	}

	// Options menu
	optionsMenu := &ui.Menu{
		Label: "Options",
		Accel: 'o',
		Items: []*ui.MenuItem{
			{Label: "Toggle Line Numbers", Action: func() {
				a.config.ShowLineNum = !a.config.ShowLineNum
				a.editor.SetShowLineNumbers(a.config.ShowLineNum)
				_ = a.config.Save()
			}},
			{Label: "Toggle Word Wrap", Action: func() {
				a.config.WordWrap = !a.config.WordWrap
				a.editor.SetWordWrap(a.config.WordWrap)
				a.statusBar.SetWordWrap(a.config.WordWrap)
				_ = a.config.Save()
			}},
			{Label: "Keyboard: Default", Shortcut: "Ctrl+Shift+M", Action: func() {
				a.setKeyboardMode("default")
				_ = a.config.Save()
			}},
			{Label: "Keyboard: Vi", Action: func() {
				a.setKeyboardMode("vi")
				_ = a.config.Save()
			}},
			{Label: "Keyboard: Helix", Action: func() {
				a.setKeyboardMode("helix")
				_ = a.config.Save()
			}},
			{Label: "UI Style: Modern", Accel: 'm', Action: func() {
				a.config.UIStyle = "modern"
				ui.InitStyle(a.config.UIStyle, a.config.IconSet)
				a.applyBorderStyle()
				_ = a.config.Save()
			}},
			{Label: "UI Style: Classic", Accel: 'c', Action: func() {
				a.config.UIStyle = "classic"
				ui.InitStyle(a.config.UIStyle, a.config.IconSet)
				a.applyBorderStyle()
				_ = a.config.Save()
			}},
			{Label: "Icons: Unicode", Action: func() {
				a.config.IconSet = "unicode"
				ui.InitStyle(a.config.UIStyle, a.config.IconSet)
				a.fileTree.Refresh()
				_ = a.config.Save()
			}},
			{Label: "Icons: ASCII", Action: func() {
				a.config.IconSet = "ascii"
				ui.InitStyle(a.config.UIStyle, a.config.IconSet)
				a.fileTree.Refresh()
				_ = a.config.Save()
			}},
			{Label: "Icons: Nerd Font", Action: func() {
				a.config.IconSet = "nerd-font"
				ui.InitStyle(a.config.UIStyle, a.config.IconSet)
				a.fileTree.Refresh()
				_ = a.config.Save()
			}},
			{Label: "Theme: Borland", Action: func() {
				a.config.Theme = "borland"
				ui.ApplyTheme("borland")
				_ = a.config.Save()
			}},
			{Label: "Theme: Modern Dark", Action: func() {
				a.config.Theme = "modern-dark"
				ui.ApplyTheme("modern-dark")
				_ = a.config.Save()
			}},
			{Label: "Theme: Modern Light", Action: func() {
				a.config.Theme = "modern-light"
				ui.ApplyTheme("modern-light")
				_ = a.config.Save()
			}},
			{Label: "Theme: Solarized Dark", Action: func() {
				a.config.Theme = "solarized-dark"
				ui.ApplyTheme("solarized-dark")
				_ = a.config.Save()
			}},
			{Label: "Formatters/Linters", Accel: 'r', Action: a.showToolsConfig},
		},
	}

	// Window menu
	windowMenu := &ui.Menu{
		Label: "Window",
		Accel: 'w',
		Items: []*ui.MenuItem{
			{Label: "Next Tab", Shortcut: "Ctrl+Tab", Action: a.nextTab},
			{Label: "Prev Tab", Shortcut: "Ctrl+Shift+Tab", Action: a.prevTab},
			{Label: "Close Tab", Shortcut: "Ctrl+W", Action: a.closeTab},
			{Label: "New Terminal Session", Shortcut: "Ctrl+Shift+T", Action: a.createTerminal},
		},
	}

	// Help menu
	helpMenu := &ui.Menu{
		Label: "Help",
		Accel: 'h',
		Items: []*ui.MenuItem{
			{Label: "Keyboard Shortcuts", Shortcut: "F1", Accel: 'k', Action: a.showHelpDialog},
			{Label: "Word Help", Shortcut: "Ctrl+F1", Accel: 'w', Action: a.showCtrlF1Help},
			{Label: "About NumenText", Action: a.showAbout},
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
		{Label: "Save As...", Accel: 'a', Action: a.saveFileAs},
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
	items = append(items, &ui.MenuItem{Label: "Exit", Shortcut: "Ctrl+Q", Accel: 'x', Action: a.quit})

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

		// Alt+letter: open or switch menus
		// Detect via ModAlt (Linux/iTerm2 with Esc+) or macOS Option Unicode chars
		accelRune := rune(0)
		if key == tcell.KeyRune && mod&tcell.ModAlt != 0 {
			accelRune = event.Rune()
		} else if key == tcell.KeyRune && mod == 0 && runtime.GOOS == "darwin" {
			accelRune = macOptionRune(event.Rune())
		}
		if accelRune != 0 {
			idx := a.menuBar.MenuForAccel(accelRune)
			if idx >= 0 {
				if a.menuBar.IsOpen() {
					// Synthesize an Alt+letter event for the menubar's InputHandler
					altEvent := tcell.NewEventKey(tcell.KeyRune, accelRune, tcell.ModAlt)
					a.menuBar.InputHandler()(altEvent, func(p tview.Primitive) {
						a.tviewApp.SetFocus(p)
					})
					return nil
				}
				a.menuBar.Open(idx)
				a.tviewApp.SetFocus(a.menuBar)
				return nil
			}
		}

		// If menu is open, handle menu-specific keys
		if a.menuBar.IsOpen() && key != tcell.KeyEscape {
			return event
		}

		// Ctrl+Shift+Arrow: resize panels
		shift := mod&tcell.ModShift != 0
		if ctrl && shift {
			switch key {
			case tcell.KeyLeft:
				a.layout.SetFileTreeWidth(a.layout.FileTreeWidth() - 2)
				a.config.FileTreeWidth = a.layout.FileTreeWidth()
				_ = a.config.Save()
				return nil
			case tcell.KeyRight:
				a.layout.SetFileTreeWidth(a.layout.FileTreeWidth() + 2)
				a.config.FileTreeWidth = a.layout.FileTreeWidth()
				_ = a.config.Save()
				return nil
			case tcell.KeyUp:
				_, _, _, screenH := a.layout.MainGrid.GetRect()
				maxH := screenH / 2
				a.layout.SetOutputHeight(a.layout.OutputHeight()+2, maxH)
				a.config.OutputHeight = a.layout.OutputHeight()
				_ = a.config.Save()
				return nil
			case tcell.KeyDown:
				_, _, _, screenH := a.layout.MainGrid.GetRect()
				maxH := screenH / 2
				a.layout.SetOutputHeight(a.layout.OutputHeight()-2, maxH)
				a.config.OutputHeight = a.layout.OutputHeight()
				_ = a.config.Save()
				return nil
			}
		}

		// Ctrl+letter handling. Terminals send these in various ways:
		// as KeyCtrl* constants, as KeyRune with ModCtrl, or as raw
		// key codes with ModCtrl. Normalize by checking ctrl flag + rune.
		ctrlRune := rune(0)
		if ctrl {
			ctrlRune = event.Rune()
		}
		// Some terminals send Ctrl+letter as KeyCtrl* (key 1-26) with rune 0.
		// Map those back to the corresponding letter.
		if ctrlRune == 0 && key >= 1 && key <= 26 {
			ctrlRune = rune('a' + key - 1)
			ctrl = true
		}
		if ctrl && ctrlRune != 0 {
			switch ctrlRune {
			case 's':
				if shift {
					a.saveFileAs()
				} else {
					a.saveFile()
				}
				return nil
			case 'n':
				a.newFile()
				return nil
			case 'o':
				a.openFile()
				return nil
			case 'q':
				a.quit()
				return nil
			case 'w':
				if a.focusedPanel == "terminal" {
					a.closeActiveTerminal()
				} else {
					a.closeTab()
				}
				return nil
			case 'f':
				if shift {
					a.showSearchPalette()
				} else {
					a.showFind()
				}
				return nil
			case 'g':
				a.showGoToLine()
				return nil
			case 'h':
				a.showReplace()
				return nil
			case 'b':
				a.editor.HandleAction(editor.ActionMatchBracket, 0)
				return nil
			case 'e':
				if len(a.buildErrors) > 0 {
					if shift {
						a.prevBuildError()
					} else {
						a.nextBuildError()
					}
				} else if len(a.searchResults) > 0 {
					if shift {
						a.prevSearchResult()
					} else {
						a.nextSearchResult()
					}
				} else {
					a.statusBar.SetMessage("No errors or search results")
				}
				return nil
			}
		}

		switch key {
		case tcell.KeyEscape:
			if a.menuBar.IsOpen() {
				a.menuBar.Close()
				a.focusPanel("editor")
				return nil
			}
			if hasDialog {
				// Let the dialog handle Escape
				return event
			}
			// Return focus to editor from file tree/output/terminal
			a.focusPanel("editor")
			return nil
		case tcell.KeyF1:
			if ctrl {
				// Ctrl+F1: word-under-cursor help (same as F11 hover)
				a.showCtrlF1Help()
			} else {
				// F1: show help dialog
				a.showHelpDialog()
			}
			return nil
		case tcell.KeyF2:
			if ctrl && shift {
				// Ctrl+Shift+F2: show bookmarks panel
				a.showBookmarksPanel()
			} else if ctrl {
				// Ctrl+F2: toggle bookmark
				a.toggleBookmark()
			} else if shift {
				// Shift+F2: previous bookmark
				a.prevBookmark()
			} else {
				// F2: next bookmark
				a.nextBookmark()
			}
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
		case tcell.KeyF4:
			if shift {
				a.prevBuildError()
			} else {
				a.nextBuildError()
			}
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
				case 'm':
					if mod&tcell.ModShift != 0 {
						a.cycleKeyboardMode()
						return nil
					}
				case 'p':
					if mod&tcell.ModShift != 0 {
						a.showCommandPalette()
					} else {
						a.showFilePalette()
					}
					return nil
				case 't':
					if mod&tcell.ModShift != 0 && a.focusedPanel == "terminal" {
						a.createTerminal()
						return nil
					}
				case 'i':
					if mod&tcell.ModShift != 0 {
						a.formatFile()
						return nil
					}
				case 'l':
					if mod&tcell.ModShift != 0 {
						a.lintFile()
						return nil
					}
				}
			}

			// When Vi/Helix Normal mode is active and editor has focus,
			// don't intercept plain rune keys — let them pass to the editor's InputHandler
			if !ctrl && a.editor.HasFocus() {
				km := a.editor.KeyMode()
				if km.SubMode() == keymode.SubModeNormal || km.SubMode() == keymode.SubModeVisual || km.SubMode() == keymode.SubModeVisualLine || km.SubMode() == keymode.SubModeCommand {
					return event // Let editor handle it
				}
			}
		case tcell.KeyCtrlRightSq:
			// Ctrl+] — cycle focus to the next visible panel (wraps around)
			a.nextPanel()
			return nil
		case tcell.KeyTab:
			if ctrl {
				if mod&tcell.ModShift != 0 {
					if a.focusedPanel == "terminal" {
						a.prevTerminal()
					} else {
						a.prevTab()
					}
				} else {
					if a.focusedPanel == "terminal" {
						a.nextTerminal()
					} else {
						a.nextTab()
					}
				}
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

func (a *App) setupMouse() {
	a.tviewApp.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if event == nil {
			return event, action
		}
		x, y := event.Position()

		switch action {
		case tview.MouseLeftDown:
			// Determine layout geometry from MainGrid's rect
			gx, gy, _, gh := a.layout.MainGrid.GetRect()

			// Vertical splitter: the column at fileTreeWidth (relative to grid)
			splitCol := gx + a.layout.FileTreeWidth()
			// The vertical splitter spans from row 1 (below menu) to bottom panel boundary
			topRow := gy + 1 // below menu bar
			bottomRow := gy + gh - 1 // above status bar
			if a.layout.OutputVisible() {
				bottomRow -= a.layout.OutputHeight()
			}

			if x >= splitCol-1 && x <= splitCol+1 && y >= topRow && y < bottomRow {
				a.dragging = "vertical"
				a.dragStartX = x
				a.dragStartW = a.layout.FileTreeWidth()
				return nil, tview.MouseConsumed
			}

			// Horizontal splitter: the row where the bottom panel starts
			if a.layout.OutputVisible() {
				splitRow := gy + gh - 1 - a.layout.OutputHeight() // 1 for status bar
				if y >= splitRow-1 && y <= splitRow+1 && x >= gx {
					a.dragging = "horizontal"
					a.dragStartY = y
					a.dragStartH = a.layout.OutputHeight()
					return nil, tview.MouseConsumed
				}
			}

		case tview.MouseMove:
			if a.dragging == "vertical" {
				delta := x - a.dragStartX
				newW := a.dragStartW + delta
				a.layout.SetFileTreeWidth(newW)
				a.config.FileTreeWidth = a.layout.FileTreeWidth()
				return nil, tview.MouseConsumed
			}
			if a.dragging == "horizontal" {
				delta := a.dragStartY - y // dragging up = grow
				_, _, _, screenH := a.layout.MainGrid.GetRect()
				maxH := screenH / 2
				newH := a.dragStartH + delta
				a.layout.SetOutputHeight(newH, maxH)
				a.config.OutputHeight = a.layout.OutputHeight()
				return nil, tview.MouseConsumed
			}

		case tview.MouseLeftUp:
			if a.dragging != "" {
				a.dragging = ""
				_ = a.config.Save()
				return nil, tview.MouseConsumed
			}

		case tview.MouseScrollUp, tview.MouseScrollDown:
			// Handle scroll directly for the terminal panel
			if a.termVisible && a.termPanel.InRect(x, y) {
				if action == tview.MouseScrollUp {
					a.termPanel.ScrollBy(3)
				} else {
					a.termPanel.ScrollBy(-3)
				}
				return nil, tview.MouseConsumed
			}
			// Forward scroll events to the output panel if mouse is over it
			if a.layout.OutputVisible() && a.output.InRect(x, y) {
				handler := a.output.MouseHandler()
				handler(action, event, func(p tview.Primitive) {
					a.tviewApp.SetFocus(p)
				})
				return nil, tview.MouseConsumed
			}
		}

		return event, action
	})
}

// visiblePanels returns the ordered list of currently visible panel names.
func (a *App) visiblePanels() []string {
	panels := []string{"filetree", "editor"}
	if a.layout.OutputVisible() {
		if a.termVisible {
			panels = append(panels, "terminal")
		} else {
			panels = append(panels, "output")
		}
	}
	return panels
}

// panelDisplayName returns a human-readable name for the status bar.
func panelDisplayName(name string) string {
	switch name {
	case "filetree":
		return "File Tree"
	case "editor":
		return "Editor"
	case "output":
		return "Output"
	case "terminal":
		return "Terminal"
	}
	return name
}

// focusPanel sets focus to the named panel, updates the border visuals and status bar.
func (a *App) focusPanel(name string) {
	a.focusedPanel = name
	switch name {
	case "filetree":
		a.tviewApp.SetFocus(a.fileTree)
	case "editor":
		a.tviewApp.SetFocus(a.editor)
	case "output":
		a.tviewApp.SetFocus(a.output)
	case "terminal":
		a.tviewApp.SetFocus(a.termPanel)
	}
	a.updatePanelBorders()
	a.statusBar.SetFocusedPanel(panelDisplayName(name))
}

// updatePanelBorders adjusts title colors to highlight the focused panel.
// The editor uses its own tab bar as a focus indicator; no border title is needed for it.
func (a *App) updatePanelBorders() {
	if a.focusedPanel == "filetree" {
		a.fileTree.SetTitleColor(ui.ColorPanelFocused)
	} else {
		a.fileTree.SetTitleColor(ui.ColorPanelBlurred)
	}

	if a.focusedPanel == "output" {
		a.output.SetTitleColor(ui.ColorPanelFocused)
	} else {
		a.output.SetTitleColor(ui.ColorPanelBlurred)
	}

	if a.focusedPanel == "terminal" {
		a.termPanel.SetTitleColor(ui.ColorPanelFocused)
	} else {
		a.termPanel.SetTitleColor(ui.ColorPanelBlurred)
	}
}

// nextPanel cycles focus forward through visible panels.
func (a *App) nextPanel() {
	panels := a.visiblePanels()
	if len(panels) == 0 {
		return
	}
	current := a.focusedPanel
	idx := 0
	for i, p := range panels {
		if p == current {
			idx = i
			break
		}
	}
	next := panels[(idx+1)%len(panels)]
	a.focusPanel(next)
}

func (a *App) updateStatusBar() {
	tab := a.editor.ActiveTab()
	if tab != nil {
		a.statusBar.Update(tab.Name, tab.CursorRow, tab.CursorCol, tab.Highlighter.Language(), tab.Buffer.Modified())
		a.statusBar.SetLineEnding(a.editor.LineEndingLabel())
		a.statusBar.SetHasBOM(a.editor.HasBOM())
		// Show build error info if navigating errors, otherwise show diagnostic
		if len(a.buildErrors) > 0 && a.buildErrorIdx >= 0 && a.buildErrorIdx < len(a.buildErrors) {
			be := a.buildErrors[a.buildErrorIdx]
			a.statusBar.SetMessage(fmt.Sprintf("Error %d of %d: %s", a.buildErrorIdx+1, len(a.buildErrors), be.Message))
		} else if diag, ok := a.editor.DiagnosticAtLine(tab.CursorRow); ok {
			a.statusBar.SetMessage(diag.Message)
		}
	} else {
		a.statusBar.Update("", 0, 0, "", false)
		a.statusBar.SetMessage("NumenText - Press Ctrl+N for new file, Ctrl+O to open")
	}
	// Update mode indicator
	km := a.editor.KeyMode()
	a.statusBar.SetModeInfo(km.SubModeLabel(), km.PendingDisplay())
}

// showCommandPalette opens the command palette overlay.
func (a *App) showCommandPalette() {
	commands := a.buildPaletteCommands()
	palette := ui.NewCommandPalette(commands, func() {
		// onExecute: close palette, refocus editor
		a.layout.HideDialog("palette")
		a.tviewApp.SetFocus(a.editor)
	}, func() {
		// onClose: dismiss without executing
		a.layout.HideDialog("palette")
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("palette", palette)
	a.tviewApp.SetFocus(palette)
}

// showFilePalette opens the file finder overlay (Ctrl+P).
func (a *App) showFilePalette() {
	files := ui.WalkProjectFiles(a.workDir)
	palette := ui.NewFilePalette(files, func(entry ui.FileEntry) {
		// onSelect: open the file in a new tab, close palette, refocus editor
		a.layout.HideDialog("filepalette")
		err := a.editor.OpenFile(entry.FullPath)
		if err != nil {
			a.output.AppendError("Error opening file: " + err.Error())
		} else {
			a.config.AddRecentFile(entry.FullPath)
			_ = a.config.Save()
		}
		a.tviewApp.SetFocus(a.editor)
	}, func() {
		// onClose: dismiss without opening
		a.layout.HideDialog("filepalette")
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("filepalette", palette)
	a.tviewApp.SetFocus(palette)
}

// showSearchPalette opens the project-wide text search overlay (Ctrl+Shift+F).
func (a *App) showSearchPalette() {
	palette := ui.NewSearchPalette(a.workDir, func(result ui.SearchResult) {
		// onSelect: open the file and jump to the matching line
		a.layout.HideDialog("searchpalette")
		err := a.editor.OpenFile(result.FilePath)
		if err != nil {
			a.output.AppendError("Error opening file: " + err.Error())
		} else {
			a.editor.GoToLine(result.Line)
		}
		a.tviewApp.SetFocus(a.editor)
	}, func() {
		// onClose: dismiss without opening
		a.layout.HideDialog("searchpalette")
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("searchpalette", palette)
	a.tviewApp.SetFocus(palette)
}

// buildPaletteCommands collects all menu items into a flat list of PaletteCommands.
func (a *App) buildPaletteCommands() []ui.PaletteCommand {
	var commands []ui.PaletteCommand
	for _, menu := range a.menuBar.Menus() {
		if menu.OnOpen != nil {
			menu.OnOpen()
		}
		for _, item := range menu.Items {
			if item.Disabled || item.Action == nil {
				continue
			}
			commands = append(commands, ui.PaletteCommand{
				Label:    menu.Label + ": " + item.Label,
				Shortcut: item.Shortcut,
				Action:   item.Action,
			})
		}
	}
	return commands
}

// OpenFileByPath opens a file by path (used for CLI arguments).
func (a *App) OpenFileByPath(filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	if err := a.editor.OpenFile(absPath); err != nil {
		a.output.AppendError("Error opening " + filePath + ": " + err.Error())
	} else {
		a.config.AddRecentFile(absPath)
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
				_ = a.config.Save()
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

	// Determine language tools config
	langID := lsp.LanguageIDForFile(tab.FilePath)
	toolsCfg := a.config.ToolsForLanguage(langID)

	// Format on save: run external formatters before writing
	if toolsCfg.FormatOnSave && len(toolsCfg.Formatters) > 0 {
		// Save to disk first so formatters can operate on the file
		err := a.editor.SaveCurrentFile()
		if err != nil {
			a.output.AppendError("Error saving: " + err.Error())
			return
		}

		// Remember cursor position
		cursorRow := tab.CursorRow
		cursorCol := tab.CursorCol

		result := editor.RunFormatters(tab.FilePath, toolsCfg.Formatters)
		if result.Error != nil {
			a.statusBar.SetMessage("Saved (format skipped: syntax errors)")
			// File was already saved with original content (rollback happened in RunFormatters)
		} else if result.Changed {
			// Reload buffer from disk
			a.editor.ReloadCurrentFile()
			// Restore cursor as closely as possible
			a.editor.SetCursorPos(cursorRow, cursorCol)
			a.statusBar.SetMessage("Saved and formatted: " + tab.FilePath)
		} else {
			a.statusBar.SetMessage("File saved: " + tab.FilePath)
		}
	} else {
		err := a.editor.SaveCurrentFile()
		if err != nil {
			a.output.AppendError("Error saving: " + err.Error())
			return
		}
		a.statusBar.SetMessage("File saved: " + tab.FilePath)
	}

	// Lint on save: run linters asynchronously after save
	if toolsCfg.LintOnSave && len(toolsCfg.Linters) > 0 {
		filePath := tab.FilePath
		linters := toolsCfg.Linters
		go func() {
			result := editor.RunLinters(filePath, linters)
			a.tviewApp.QueueUpdateDraw(func() {
				a.applyLintDiagnostics(filePath, result)
			})
		}()
	}

	// Refresh file tree in case a new file was saved
	a.fileTree.Refresh()
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
				_ = a.config.Save()
				a.statusBar.SetMessage("File saved: " + result.FilePath)
				a.fileTree.Refresh()
			}
		}
		a.tviewApp.SetFocus(a.editor)
	})
	a.layout.ShowDialog("saveas", dialog)
}

// formatFile runs formatters on the current file (on-demand).
// Falls back to LSP textDocument/formatting if no external formatters configured.
func (a *App) formatFile() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	langID := lsp.LanguageIDForFile(tab.FilePath)
	toolsCfg := a.config.ToolsForLanguage(langID)

	if len(toolsCfg.Formatters) > 0 {
		// Save first so formatters can work on the file
		if err := a.editor.SaveCurrentFile(); err != nil {
			a.output.AppendError("Error saving before format: " + err.Error())
			return
		}
		cursorRow := tab.CursorRow
		cursorCol := tab.CursorCol

		result := editor.RunFormatters(tab.FilePath, toolsCfg.Formatters)
		if result.Error != nil {
			a.statusBar.SetMessage("Format error: " + result.Error.Error())
		} else if result.Changed {
			a.editor.ReloadCurrentFile()
			a.editor.SetCursorPos(cursorRow, cursorCol)
			a.statusBar.SetMessage("File formatted")
		} else {
			a.statusBar.SetMessage("File already formatted")
		}
		return
	}

	// Fallback: try LSP formatting
	client := a.lspManager.ClientForFile(tab.FilePath)
	if client != nil {
		go func() {
			edits, err := client.Format(tab.FilePath, a.config.TabSize, true)
			a.tviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.statusBar.SetMessage("LSP format error: " + err.Error())
					return
				}
				if len(edits) == 0 {
					a.statusBar.SetMessage("File already formatted")
					return
				}
				a.applyLSPEdits(tab, edits)
				a.statusBar.SetMessage("File formatted via LSP")
			})
		}()
		return
	}

	a.statusBar.SetMessage("No formatter configured for this language")
}

// lintFile runs linters on the current file (on-demand).
func (a *App) lintFile() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		return
	}
	langID := lsp.LanguageIDForFile(tab.FilePath)
	toolsCfg := a.config.ToolsForLanguage(langID)

	if len(toolsCfg.Linters) == 0 {
		a.statusBar.SetMessage("No linter configured for this language")
		return
	}

	// Save first so linters work on current content
	if err := a.editor.SaveCurrentFile(); err != nil {
		a.output.AppendError("Error saving before lint: " + err.Error())
		return
	}

	filePath := tab.FilePath
	linters := toolsCfg.Linters
	a.statusBar.SetMessage("Running linter...")
	go func() {
		result := editor.RunLinters(filePath, linters)
		a.tviewApp.QueueUpdateDraw(func() {
			a.applyLintDiagnostics(filePath, result)
		})
	}()
}

// applyLintDiagnostics converts linter results to editor diagnostics.
func (a *App) applyLintDiagnostics(filePath string, result editor.LintResult) {
	if result.Error != nil {
		a.statusBar.SetMessage("Lint error: " + result.Error.Error())
		return
	}

	// Convert to editor diagnostics (keyed by 0-based line)
	diags := make(map[int]editor.DiagnosticInfo)
	for _, d := range result.Diagnostics {
		line := d.Line - 1 // convert to 0-based
		if line < 0 {
			line = 0
		}
		// If multiple diagnostics on same line, keep the more severe one
		if existing, ok := diags[line]; ok && existing.Severity < d.Severity {
			continue
		}
		source := "linter"
		diags[line] = editor.DiagnosticInfo{
			Severity: d.Severity,
			Message:  source + ": " + d.Message,
		}
	}

	a.editor.SetDiagnostics(filePath, diags)

	count := len(result.Diagnostics)
	if count > 0 {
		a.statusBar.SetMessage(fmt.Sprintf("Lint: %d issue(s) found", count))
	} else {
		a.statusBar.SetMessage("Lint: no issues found")
	}
}

// applyLSPEdits applies LSP text edits to the active tab buffer.
func (a *App) applyLSPEdits(tab *editor.Tab, edits []lsp.TextEdit) {
	if tab == nil {
		return
	}
	// For simplicity, if we have edits, save the file, apply via LSP full-replace approach:
	// Get current text, apply edits in reverse order (to preserve positions), update buffer
	lines := tab.Buffer.Lines()
	// Apply edits in reverse order of position to avoid offset issues
	for i := len(edits) - 1; i >= 0; i-- {
		edit := edits[i]
		startLine := edit.Range.Start.Line
		startChar := edit.Range.Start.Character
		endLine := edit.Range.End.Line
		endChar := edit.Range.End.Character

		// Clamp to valid range
		if startLine < 0 {
			startLine = 0
		}
		if endLine >= len(lines) {
			endLine = len(lines) - 1
		}
		if startLine >= len(lines) {
			continue
		}

		// Build the text before and after the edit range
		prefix := ""
		if startChar <= len(lines[startLine]) {
			prefix = lines[startLine][:startChar]
		}
		suffix := ""
		if endLine < len(lines) && endChar <= len(lines[endLine]) {
			suffix = lines[endLine][endChar:]
		}

		// Replace the range with new text
		newText := prefix + edit.NewText + suffix
		newLines := strings.Split(newText, "\n")

		// Splice into the lines array
		result := make([]string, 0, len(lines)-endLine+startLine+len(newLines))
		result = append(result, lines[:startLine]...)
		result = append(result, newLines...)
		if endLine+1 < len(lines) {
			result = append(result, lines[endLine+1:]...)
		}
		lines = result
	}

	tab.Buffer.SetText(strings.Join(lines, "\n"))
	tab.Buffer.SetModified(true)
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
			if result.UseRegex {
				if err := a.editor.FindRegexError(result.Text); err != nil {
					a.statusBar.SetMessage("Regex error: " + err.Error())
					return
				}
			}
			found := a.editor.FindWithOptions(result.Text, true, result.UseRegex)
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
			if result.UseRegex {
				if err := a.editor.FindRegexError(result.Text); err != nil {
					a.statusBar.SetMessage("Regex error: " + err.Error())
					return
				}
			}
			if result.AllFiles {
				a.findAllFiles(result.Text, result.UseRegex)
			} else {
				found := a.editor.FindWithOptions(result.Text, true, result.UseRegex)
				if !found {
					a.statusBar.SetMessage("Not found: " + result.Text)
				}
			}
		},
		func(result ui.DialogResult) {
			// Replace
			a.editor.ReplaceWithOptions(result.Text, result.Text2, result.UseRegex)
		},
		func(result ui.DialogResult) {
			// Replace All
			if result.AllFiles {
				counts := a.editor.ReplaceAllInAllFiles(result.Text, result.Text2, result.UseRegex)
				total := 0
				for _, c := range counts {
					total += c
				}
				if total == 0 {
					a.statusBar.SetMessage("No matches found")
				} else {
					a.statusBar.SetMessage(fmt.Sprintf("Replaced %d occurrences in %d files", total, len(counts)))
				}
			} else {
				count := a.editor.ReplaceAllWithOptions(result.Text, result.Text2, result.UseRegex)
				a.statusBar.SetMessage(fmt.Sprintf("Replaced %d occurrences", count))
			}
		},
		func() {
			// Close
			a.layout.HideDialog("replace")
			a.tviewApp.SetFocus(a.editor)
		},
	)
	a.layout.ShowDialog("replace", dialog)
}

// findAllFiles searches across all open tabs and displays results in the output panel
func (a *App) findAllFiles(query string, useRegex bool) {
	results, err := a.editor.FindAllFiles(query, useRegex)
	if err != nil {
		a.statusBar.SetMessage("Regex error: " + err.Error())
		return
	}
	if len(results) == 0 {
		a.statusBar.SetMessage("No matches found across files")
		return
	}

	// Store results for navigation
	a.searchResults = results
	a.searchResultIdx = -1

	// Display results in output panel
	a.output.Clear()
	a.output.AppendText(fmt.Sprintf("[#00ffff]Search results for: %s (%d matches)[-]", tview.Escape(query), len(results)))

	currentFile := ""
	for i, r := range results {
		if r.FilePath != currentFile {
			currentFile = r.FilePath
			a.output.AppendText(fmt.Sprintf("\n[yellow]%s[-]", tview.Escape(currentFile)))
		}
		linePreview := r.LineText
		if len(linePreview) > 80 {
			linePreview = linePreview[:80] + "..."
		}
		a.output.AppendText(fmt.Sprintf("  [white]%d:[-] Ln %d, Col %d: %s",
			i+1, r.Line+1, r.Col+1, tview.Escape(linePreview)))
	}

	a.output.AppendText("\n[#00ffff]Press Ctrl+E / Ctrl+Shift+E to navigate results[-]")
	a.layout.SetOutputVisible(true, 10)
	a.statusBar.SetMessage(fmt.Sprintf("Found %d matches across files", len(results)))
}

// nextSearchResult jumps to the next search result
func (a *App) nextSearchResult() {
	if len(a.searchResults) == 0 {
		a.statusBar.SetMessage("No search results")
		return
	}
	a.searchResultIdx++
	if a.searchResultIdx >= len(a.searchResults) {
		a.searchResultIdx = 0
	}
	a.jumpToSearchResult(a.searchResultIdx)
}

// prevSearchResult jumps to the previous search result
func (a *App) prevSearchResult() {
	if len(a.searchResults) == 0 {
		a.statusBar.SetMessage("No search results")
		return
	}
	a.searchResultIdx--
	if a.searchResultIdx < 0 {
		a.searchResultIdx = len(a.searchResults) - 1
	}
	a.jumpToSearchResult(a.searchResultIdx)
}

// jumpToSearchResult navigates to a specific search result
func (a *App) jumpToSearchResult(idx int) {
	if idx < 0 || idx >= len(a.searchResults) {
		return
	}
	r := a.searchResults[idx]

	// Switch to the correct tab
	if r.TabIndex >= 0 && r.TabIndex < a.editor.TabCount() {
		a.editor.SetActiveTab(r.TabIndex)
	}

	// Go to the line and column
	a.editor.GoToLine(r.Line + 1) // GoToLine is 1-based
	tab := a.editor.ActiveTab()
	if tab != nil {
		tab.CursorCol = r.Col
		tab.HasSelect = true
		tab.SelectStart = [2]int{r.Line, r.Col}
		tab.SelectEnd = [2]int{r.Line, r.Col + r.MatchLen}
	}

	a.statusBar.SetMessage(fmt.Sprintf("Result %d of %d: %s:%d",
		idx+1, len(a.searchResults), filepath.Base(r.FilePath), r.Line+1))
	a.focusPanel("editor")
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

func (a *App) prevTab() {
	count := a.editor.TabCount()
	if count <= 1 {
		return
	}
	prev := (a.editor.ActiveTabIndex() - 1 + count) % count
	a.editor.SetActiveTab(prev)
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

	// Clear previous build errors
	a.clearBuildErrors()

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
				a.handleBuildErrors(result)
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

	// Clear previous build errors
	a.clearBuildErrors()

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
			} else {
				a.handleBuildErrors(result)
			}
		})
	}()
}

// handleBuildErrors parses build output for errors, sets gutter markers, and jumps to first error.
func (a *App) handleBuildErrors(result *runner.Result) {
	output := result.Output
	if output == "" {
		output = result.Error
	}
	errors := runner.ParseBuildOutput(output)
	if len(errors) == 0 {
		return
	}

	a.buildErrors = errors
	a.buildErrorIdx = 0
	a.statusBar.SetHasErrors(true)

	// Set build diagnostic markers in the editor for each file
	fileErrors := make(map[string]map[int]editor.DiagnosticInfo)
	for _, be := range errors {
		if fileErrors[be.File] == nil {
			fileErrors[be.File] = make(map[int]editor.DiagnosticInfo)
		}
		lineIdx := be.Line - 1 // Convert to 0-based
		if lineIdx < 0 {
			lineIdx = 0
		}
		sev := 1 // error
		if be.Severity == "warning" {
			sev = 2
		} else if be.Severity == "note" {
			sev = 3
		}
		// Only store first error per line
		if _, exists := fileErrors[be.File][lineIdx]; !exists {
			fileErrors[be.File][lineIdx] = editor.DiagnosticInfo{
				Severity: sev,
				Message:  be.Message,
			}
		}
	}
	for file, diags := range fileErrors {
		a.editor.SetBuildDiagnostics(file, diags)
	}

	// Jump to first error
	a.jumpToBuildError(0)
}

// clearBuildErrors removes all build error state and gutter markers.
func (a *App) clearBuildErrors() {
	a.buildErrors = nil
	a.buildErrorIdx = -1
	a.statusBar.SetHasErrors(false)
	a.editor.ClearAllBuildDiagnostics()
}

// jumpToBuildError opens the file and jumps to the error at the given index.
func (a *App) jumpToBuildError(idx int) {
	if idx < 0 || idx >= len(a.buildErrors) {
		return
	}
	be := a.buildErrors[idx]
	a.buildErrorIdx = idx

	// Suppress clearing errors during navigation
	a.navigatingErrors = true
	defer func() { a.navigatingErrors = false }()

	// Resolve file path relative to working directory
	filePath := be.File
	if !strings.HasPrefix(filePath, "/") {
		filePath = a.workDir + "/" + filePath
	}

	// Open file (switches to tab if already open)
	err := a.editor.OpenFile(filePath)
	if err != nil {
		a.statusBar.SetMessage(fmt.Sprintf("Cannot open %s: %s", be.File, err.Error()))
		return
	}

	// Jump to error line (1-based)
	a.editor.GoToLine(be.Line)
	a.focusPanel("editor")

	// Show error info in status bar
	a.statusBar.SetMessage(fmt.Sprintf("Error %d of %d: %s", idx+1, len(a.buildErrors), be.Message))
}

// nextBuildError jumps to the next build error (wraps around).
func (a *App) nextBuildError() {
	if len(a.buildErrors) == 0 {
		a.statusBar.SetMessage("No build errors")
		return
	}
	next := a.buildErrorIdx + 1
	if next >= len(a.buildErrors) {
		next = 0
	}
	a.jumpToBuildError(next)
}

// prevBuildError jumps to the previous build error (wraps around).
func (a *App) prevBuildError() {
	if len(a.buildErrors) == 0 {
		a.statusBar.SetMessage("No build errors")
		return
	}
	prev := a.buildErrorIdx - 1
	if prev < 0 {
		prev = len(a.buildErrors) - 1
	}
	a.jumpToBuildError(prev)
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

func (a *App) showToolsConfig() {
	text := tview.NewTextView()
	text.SetBackgroundColor(ui.ColorDialogBg)
	text.SetTextColor(ui.ColorStatusText)
	text.SetDynamicColors(true)
	text.SetBorder(true)
	text.SetBorderColor(ui.ColorStatusText)
	text.SetTitle(" Formatters/Linters ")
	text.SetTitleColor(ui.ColorStatusText)

	var content strings.Builder
	content.WriteString("\n")

	if len(a.config.LanguageTools) == 0 {
		content.WriteString(" No language tools configured.\n")
		content.WriteString("\n Edit ~/.numentext/config.json to add tools.\n")
		content.WriteString(" Example:\n")
		content.WriteString("   \"language_tools\": {\n")
		content.WriteString("     \"python\": {\n")
		content.WriteString("       \"formatters\": [{\"command\":\"black\",\"args\":[\"--quiet\",\"{file}\"]}],\n")
		content.WriteString("       \"linters\": [{\"command\":\"flake8\",\"args\":[\"{file}\"]}],\n")
		content.WriteString("       \"format_on_save\": true,\n")
		content.WriteString("       \"lint_on_save\": true\n")
		content.WriteString("     }\n")
		content.WriteString("   }\n")
	} else {
		for lang, ltc := range a.config.LanguageTools {
			content.WriteString(fmt.Sprintf(" [white::b]%s[-::-]\n", lang))
			fmtStatus := "OFF"
			if ltc.FormatOnSave {
				fmtStatus = "ON"
			}
			content.WriteString(fmt.Sprintf("   Format on save: %s\n", fmtStatus))
			for _, f := range ltc.Formatters {
				content.WriteString(fmt.Sprintf("     - %s %s\n", f.Command, strings.Join(f.Args, " ")))
			}
			if len(ltc.Formatters) == 0 {
				content.WriteString("     (none)\n")
			}
			lintStatus := "OFF"
			if ltc.LintOnSave {
				lintStatus = "ON"
			}
			content.WriteString(fmt.Sprintf("   Lint on save: %s\n", lintStatus))
			for _, l := range ltc.Linters {
				content.WriteString(fmt.Sprintf("     - %s %s\n", l.Command, strings.Join(l.Args, " ")))
			}
			if len(ltc.Linters) == 0 {
				content.WriteString("     (none)\n")
			}
			content.WriteString("\n")
		}
	}
	content.WriteString("\n Press Escape to close\n")
	text.SetText(content.String())

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.layout.HideDialog("toolsconfig")
			a.tviewApp.SetFocus(a.editor)
			return nil
		}
		return event
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 24, 0, true).
			AddItem(nil, 0, 1, false),
			50, 0, true).
		AddItem(nil, 0, 1, false)

	a.layout.ShowDialog("toolsconfig", modal)
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
 Ctrl+B    Go to matching bracket

 [white::b]Run[-::-]
 Ctrl+E    Next build error
 Ctrl+Shift+E  Previous build error
 F5        Run
 F9        Build
 F10       Menu bar

 [white::b]Navigation[-::-]
 Ctrl+Tab            Next tab / Next terminal session
 Ctrl+Shift+Tab      Prev tab / Prev terminal session
 Ctrl+1-9            Switch tab
 Ctrl+Arrows         Word jump
 Ctrl+]              Next panel (File Tree -> Editor -> Output/Terminal)
 Ctrl+Shift+T        New terminal session (when terminal focused)
 Ctrl+W              Close tab / Close terminal session

 [white::b]Panel Resize[-::-]
 Ctrl+Shift+Left     Shrink file tree
 Ctrl+Shift+Right    Grow file tree
 Ctrl+Shift+Up       Grow bottom panel
 Ctrl+Shift+Down     Shrink bottom panel

 [white::b]Tools[-::-]
 Ctrl+Shift+I        Format file
 Ctrl+Shift+L        Lint file

 [white::b]Terminal Block Mode[-::-]
 Ctrl+Up/Down        Select prev/next command block
 Enter               Toggle expand/collapse (when block selected)
 y                   Copy block (cmd+output) to clipboard
 c                   Copy command only
 o                   Copy output only
 e                   Expand all blocks
 a                   Collapse all blocks
 Escape              Deselect block

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
		go func() {
			a.lspManager.NotifyOpen(filePath, text)
			a.refreshBreadcrumb(filePath)
		}()
	})
	a.editor.SetOnFileChange(func(filePath, text string) {
		go func() {
			a.lspManager.NotifyChange(filePath, text)
			a.refreshBreadcrumb(filePath)
		}()
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

// --- Bookmark methods ---

func (a *App) toggleBookmark() {
	result := a.editor.ToggleBookmark()
	switch result {
	case 1:
		a.statusBar.SetMessage("Bookmark added")
	case 0:
		a.statusBar.SetMessage("Bookmark removed")
	case -1:
		a.statusBar.SetMessage(fmt.Sprintf("Bookmark limit reached (%d)", editor.MaxBookmarks))
	}
}

func (a *App) nextBookmark() {
	line, idx, total := a.editor.NextBookmark()
	if total == 0 {
		a.statusBar.SetMessage("No bookmarks")
		return
	}
	_ = line
	a.statusBar.SetMessage(fmt.Sprintf("Bookmark %d of %d", idx, total))
}

func (a *App) prevBookmark() {
	line, idx, total := a.editor.PrevBookmark()
	if total == 0 {
		a.statusBar.SetMessage("No bookmarks")
		return
	}
	_ = line
	a.statusBar.SetMessage(fmt.Sprintf("Bookmark %d of %d", idx, total))
}

func (a *App) showBookmarksPanel() {
	entries := a.editor.AllBookmarks()
	if len(entries) == 0 {
		a.statusBar.SetMessage("No bookmarks")
		return
	}

	list := tview.NewList()
	list.SetBackgroundColor(ui.ColorDialogBg)
	list.SetMainTextColor(ui.ColorTextWhite)
	list.SetSecondaryTextColor(ui.ColorTextGray)
	list.SetSelectedTextColor(ui.ColorSelectedText)
	list.SetSelectedBackgroundColor(ui.ColorSelected)
	list.ShowSecondaryText(true)

	for _, entry := range entries {
		lineText := strings.TrimSpace(entry.Text)
		if len(lineText) > 60 {
			lineText = lineText[:60] + "..."
		}
		// Escape tview color tags
		lineText = tview.Escape(lineText)
		primary := fmt.Sprintf("%s:%d", entry.TabName, entry.Line+1)
		e := entry // capture
		list.AddItem(primary, "  "+lineText, 0, func() {
			a.editor.SetActiveTab(e.TabIndex)
			a.editor.GoToLine(e.Line + 1)
			a.layout.HideDialog("bookmarks")
			a.tviewApp.SetFocus(a.editor)
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.layout.HideDialog("bookmarks")
			a.tviewApp.SetFocus(a.editor)
			return nil
		}
		return event
	})

	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true)
	frame.SetBackgroundColor(ui.ColorDialogBg)
	frame.SetBorder(true)
	frame.SetBorderColor(ui.ColorStatusText)
	frame.SetTitle(" Bookmarks ")
	frame.SetTitleColor(ui.ColorStatusText)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, 20, 0, true).
			AddItem(nil, 0, 1, false),
			60, 0, true).
		AddItem(nil, 0, 1, false)

	a.layout.ShowDialog("bookmarks", modal)
	a.tviewApp.SetFocus(list)
}

// --- Help methods ---

func (a *App) showCtrlF1Help() {
	tab := a.editor.ActiveTab()
	if tab == nil || tab.FilePath == "" {
		a.statusBar.SetMessage("No help available for this file type")
		return
	}
	// Check if cursor is on whitespace
	line := tab.Buffer.Line(tab.CursorRow)
	col := tab.CursorCol
	if col >= len(line) || line[col] == ' ' || line[col] == '\t' {
		return
	}
	// Delegate to showHover (same as F11)
	a.showHover()
}

func (a *App) showHelpDialog() {
	// Define all shortcuts grouped by category
	type shortcutEntry struct {
		key  string
		desc string
	}
	type category struct {
		name    string
		entries []shortcutEntry
	}
	categories := []category{
		{"File", []shortcutEntry{
			{"Ctrl+N", "New file"},
			{"Ctrl+O", "Open file"},
			{"Ctrl+S", "Save"},
			{"Ctrl+W", "Close tab"},
			{"Ctrl+Q", "Quit"},
		}},
		{"Edit", []shortcutEntry{
			{"Ctrl+Z", "Undo"},
			{"Ctrl+Y", "Redo"},
			{"Ctrl+X", "Cut"},
			{"Ctrl+C", "Copy"},
			{"Ctrl+V", "Paste"},
			{"Ctrl+A", "Select all"},
			{"Ctrl+D", "Delete line"},
		}},
		{"Search", []shortcutEntry{
			{"Ctrl+F", "Find"},
			{"Ctrl+H", "Replace"},
			{"Ctrl+G", "Go to line"},
			{"Ctrl+B", "Go to matching bracket"},
			{"Ctrl+P", "File finder"},
			{"Ctrl+Shift+P", "Command palette"},
			{"Ctrl+Shift+F", "Search in files"},
		}},
		{"Build/Run", []shortcutEntry{
			{"F5", "Run"},
			{"F9", "Build"},
			{"Ctrl+E", "Next build error"},
			{"Ctrl+Shift+E", "Previous build error"},
			{"F4", "Next error"},
			{"Shift+F4", "Previous error"},
		}},
		{"Navigation", []shortcutEntry{
			{"Ctrl+Tab", "Next tab"},
			{"Ctrl+Shift+Tab", "Previous tab"},
			{"Ctrl+1-9", "Switch to tab N"},
			{"Ctrl+]", "Next panel"},
			{"F10", "Menu bar"},
		}},
		{"LSP", []shortcutEntry{
			{"F11", "Hover info"},
			{"F12", "Go to definition"},
			{"Ctrl+F1", "Word help (hover)"},
		}},
		{"Debug", []shortcutEntry{
			{"F5", "Start debug"},
			{"F6", "Continue"},
			{"F7", "Step over"},
			{"F8", "Toggle breakpoint"},
		}},
		{"Bookmarks", []shortcutEntry{
			{"Ctrl+F2", "Toggle bookmark"},
			{"F2", "Next bookmark"},
			{"Shift+F2", "Previous bookmark"},
			{"Ctrl+Shift+F2", "Bookmarks panel"},
		}},
		{"Tools", []shortcutEntry{
			{"Ctrl+`", "Toggle terminal"},
			{"Ctrl+Shift+I", "Format file"},
			{"Ctrl+Shift+L", "Lint file"},
			{"Ctrl+Shift+T", "New terminal session"},
		}},
		{"Panel Resize", []shortcutEntry{
			{"Ctrl+Shift+Left", "Shrink file tree"},
			{"Ctrl+Shift+Right", "Grow file tree"},
			{"Ctrl+Shift+Up", "Grow bottom panel"},
			{"Ctrl+Shift+Down", "Shrink bottom panel"},
		}},
		{"Help", []shortcutEntry{
			{"F1", "Keyboard shortcuts"},
			{"Ctrl+F1", "Word help"},
		}},
	}

	// Build flat list for filtering
	type flatEntry struct {
		catName string
		key     string
		desc    string
	}
	var allEntries []flatEntry
	for _, cat := range categories {
		for _, e := range cat.entries {
			allEntries = append(allEntries, flatEntry{cat.name, e.key, e.desc})
		}
	}

	searchInput := tview.NewInputField()
	searchInput.SetLabel("Search: ")
	searchInput.SetBackgroundColor(ui.ColorDialogBg)
	searchInput.SetFieldBackgroundColor(ui.ColorBg)
	searchInput.SetFieldTextColor(ui.ColorTextWhite)
	searchInput.SetLabelColor(ui.ColorStatusText)

	textView := tview.NewTextView()
	textView.SetBackgroundColor(ui.ColorDialogBg)
	textView.SetTextColor(ui.ColorStatusText)
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)

	renderEntries := func(filter string) {
		var sb strings.Builder
		filter = strings.ToLower(filter)
		lastCat := ""
		count := 0
		for _, e := range allEntries {
			if filter != "" {
				if !strings.Contains(strings.ToLower(e.key), filter) &&
					!strings.Contains(strings.ToLower(e.desc), filter) &&
					!strings.Contains(strings.ToLower(e.catName), filter) {
					continue
				}
			}
			if e.catName != lastCat {
				if lastCat != "" {
					sb.WriteString("\n")
				}
				sb.WriteString(" [white::b]")
				sb.WriteString(e.catName)
				sb.WriteString("[-::-]\n")
				lastCat = e.catName
			}
			sb.WriteString(fmt.Sprintf(" %-20s %s\n", e.key, e.desc))
			count++
		}
		if count == 0 {
			sb.WriteString("\n No matching shortcuts found.\n")
		}
		textView.SetText(sb.String())
		textView.ScrollToBeginning()
	}

	renderEntries("")

	searchInput.SetChangedFunc(func(text string) {
		renderEntries(text)
	})

	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			a.layout.HideDialog("help")
			a.tviewApp.SetFocus(a.editor)
			return nil
		case tcell.KeyDown:
			// Move focus to text view for scrolling
			a.tviewApp.SetFocus(textView)
			return nil
		}
		return event
	})

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			a.layout.HideDialog("help")
			a.tviewApp.SetFocus(a.editor)
			return nil
		case tcell.KeyTab:
			a.tviewApp.SetFocus(searchInput)
			return nil
		}
		return event
	})

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchInput, 1, 0, true).
		AddItem(textView, 0, 1, false)

	layout.SetBackgroundColor(ui.ColorDialogBg)
	layout.SetBorder(true)
	layout.SetBorderColor(ui.ColorStatusText)
	layout.SetTitle(" Help - Keyboard Shortcuts (F1) ")
	layout.SetTitleColor(ui.ColorStatusText)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(layout, 30, 0, true).
			AddItem(nil, 0, 1, false),
			50, 0, true).
		AddItem(nil, 0, 1, false)

	a.layout.ShowDialog("help", modal)
	a.tviewApp.SetFocus(searchInput)
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
	go func() { _ = a.dapManager.StartSession(tab.FilePath) }()
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

// termName returns the display label for a terminal at the given index.
func termName(idx int) string {
	return fmt.Sprintf("Term %d", idx+1)
}

// updateTermPanel syncs the panel's terminal and tab bar to the current terms list.
func (a *App) updateTermPanel() {
	if len(a.terms) == 0 {
		a.termPanel.SetTerminal(nil)
		a.termPanel.SetTabs(nil, 0)
		return
	}
	a.termPanel.SetTerminal(a.terms[a.activeTermIdx])
	names := make([]string, len(a.terms))
	for i := range a.terms {
		names[i] = termName(i)
	}
	a.termPanel.SetTabs(names, a.activeTermIdx)
}

// createTerminal starts a new terminal session and makes it active.
func (a *App) createTerminal() {
	t := terminal.NewTerminal(80, 24)
	t.SetOnData(func() {
		a.tviewApp.QueueUpdateDraw(func() {})
	})
	err := t.Start("")
	if err != nil {
		a.output.AppendError("Failed to start terminal: " + err.Error())
		return
	}
	a.terms = append(a.terms, t)
	a.activeTermIdx = len(a.terms) - 1
	a.updateTermPanel()
}

// closeActiveTerminal stops the active terminal and removes it from the list.
func (a *App) closeActiveTerminal() {
	if len(a.terms) == 0 {
		return
	}
	a.terms[a.activeTermIdx].Stop()
	a.terms = append(a.terms[:a.activeTermIdx], a.terms[a.activeTermIdx+1:]...)
	if len(a.terms) == 0 {
		// No sessions left — hide terminal panel
		a.closeTerminal()
		return
	}
	if a.activeTermIdx >= len(a.terms) {
		a.activeTermIdx = len(a.terms) - 1
	}
	a.updateTermPanel()
}

// nextTerminal cycles to the next terminal session.
func (a *App) nextTerminal() {
	if len(a.terms) <= 1 {
		return
	}
	a.activeTermIdx = (a.activeTermIdx + 1) % len(a.terms)
	a.updateTermPanel()
}

// prevTerminal cycles to the previous terminal session.
func (a *App) prevTerminal() {
	if len(a.terms) <= 1 {
		return
	}
	a.activeTermIdx = (a.activeTermIdx - 1 + len(a.terms)) % len(a.terms)
	a.updateTermPanel()
}

func (a *App) toggleBlockMode() {
	on := !a.termPanel.BoxMode()
	a.termPanel.SetBoxMode(on)
	if on {
		a.statusBar.SetMessage("Terminal block mode ON")
	} else {
		a.statusBar.SetMessage("Terminal block mode OFF")
	}
}

func (a *App) openTerminal() {
	if len(a.terms) == 0 {
		a.createTerminal()
		if len(a.terms) == 0 {
			return // creation failed
		}
	}

	a.bottomFlex.Clear()
	a.bottomFlex.AddItem(a.termPanel, 0, 1, true)
	a.termVisible = true
	a.layout.SetOutputVisible(true, a.config.OutputHeight)
	a.focusPanel("terminal")
}

func (a *App) closeTerminal() {
	a.bottomFlex.Clear()
	a.bottomFlex.AddItem(a.output, 0, 1, false)
	a.termVisible = false
	// Hide output panel if there's no output content
	if len(a.output.Lines()) == 0 {
		a.layout.SetOutputVisible(false, 0)
	}
	a.focusPanel("editor")
}

// macOptionRune maps macOS Option+letter Unicode characters back to their
// base ASCII letter. macOS Terminal.app sends these instead of ModAlt events.
// Returns 0 if the rune is not a recognized Option+letter character.
func macOptionRune(r rune) rune {
	switch r {
	case 0x0192: // ƒ = Option+F
		return 'f'
	case 0x00B4: // ´ = Option+E
		return 'e'
	case 0x00DF: // ß = Option+S
		return 's'
	case 0x00AE: // ® = Option+R
		return 'r'
	case 0x2202: // ∂ = Option+D
		return 'd'
	case 0x2020: // † = Option+T
		return 't'
	case 0x00F8: // ø = Option+O
		return 'o'
	case 0x2211: // ∑ = Option+W
		return 'w'
	case 0x02D9: // ˙ = Option+H
		return 'h'
	}
	return 0
}

func (a *App) setKeyboardMode(mode string) {
	switch mode {
	case "vi":
		vi := keymode.NewViMode()
		vi.Callbacks = &keymode.ViCommandCallback{
			OnSave:     a.saveFile,
			OnQuit:     a.quit,
			OnSaveQuit: func() { a.saveFile(); a.quit() },
			OnGoToLine: func(line int) { a.editor.GoToLine(line) },
		}
		vi.OnCommandStart = func(prompt string) {
			a.statusBar.SetCommandText(prompt)
		}
		vi.OnCommandUpdate = func(text string) {
			a.statusBar.SetCommandText(text)
		}
		vi.OnCommandEnd = func() {
			a.statusBar.SetCommandText("")
		}
		a.editor.SetKeyMode(vi)
	case "helix":
		a.editor.SetKeyMode(keymode.NewHelixMode())
	default:
		mode = "default"
		a.editor.SetKeyMode(keymode.NewDefaultMode())
	}
	a.config.KeyboardMode = mode
	a.updateStatusBar()
}

func (a *App) cycleKeyboardMode() {
	current := a.config.KeyboardMode
	var next string
	switch current {
	case "default":
		next = "vi"
	case "vi":
		next = "helix"
	default:
		next = "default"
	}
	a.setKeyboardMode(next)
	_ = a.config.Save()
	a.statusBar.SetMessage("Keyboard mode: " + a.editor.KeyMode().Mode())
}

// Run starts the application
func (a *App) Run() error {
	defer a.cleanup()
	return a.tviewApp.Run()
}

// cleanup stops all subprocesses with an overall timeout so the app never hangs on exit.
func (a *App) cleanup() {
	done := make(chan struct{})
	go func() {
		for _, t := range a.terms {
			t.Stop()
		}
		a.dapManager.StopSession()
		a.lspManager.StopAll()
		close(done)
	}()

	frames := []string{"|", "/", "-", "\\"}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(2 * time.Second)
	i := 0

	for {
		select {
		case <-done:
			fmt.Print("\r\033[K") // clear spinner line
			return
		case <-deadline:
			fmt.Print("\r\033[K")
			return
		case <-tick.C:
			fmt.Printf("\r  %s Shutting down...", frames[i%len(frames)])
			i++
		}
	}
}

// refreshBreadcrumb fetches document symbols from LSP and updates the editor breadcrumb.
func (a *App) refreshBreadcrumb(filePath string) {
	symbols, err := a.lspManager.DocumentSymbols(filePath)
	if err != nil || symbols == nil {
		return
	}
	bs := convertSymbols(symbols)
	a.tviewApp.QueueUpdateDraw(func() {
		a.editor.SetBreadcrumbSymbols(filePath, bs)
	})
}

func convertSymbols(symbols []lsp.DocumentSymbol) []editor.BreadcrumbSymbol {
	result := make([]editor.BreadcrumbSymbol, len(symbols))
	for i, s := range symbols {
		result[i] = editor.BreadcrumbSymbol{
			Name:      s.Name,
			Kind:      s.Kind,
			StartLine: s.Range.Start.Line,
			EndLine:   s.Range.End.Line,
		}
		if len(s.Children) > 0 {
			result[i].Children = convertSymbols(s.Children)
		}
	}
	return result
}

// applyBorderStyle sets tview global border characters based on UI style.
func (a *App) applyBorderStyle() {
	if a.config.UIStyle == "classic" {
		tview.Borders.Horizontal = '-'
		tview.Borders.Vertical = '|'
		tview.Borders.TopLeft = '+'
		tview.Borders.TopRight = '+'
		tview.Borders.BottomLeft = '+'
		tview.Borders.BottomRight = '+'
		tview.Borders.LeftT = '+'
		tview.Borders.RightT = '+'
		tview.Borders.TopT = '+'
		tview.Borders.BottomT = '+'
		tview.Borders.Cross = '+'
		tview.Borders.HorizontalFocus = '-'
		tview.Borders.VerticalFocus = '|'
		tview.Borders.TopLeftFocus = '+'
		tview.Borders.TopRightFocus = '+'
		tview.Borders.BottomLeftFocus = '+'
		tview.Borders.BottomRightFocus = '+'
	}
	// Modern mode uses tview defaults (Unicode box-drawing)
}
