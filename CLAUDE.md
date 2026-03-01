# NumenText

Terminal-based IDE written in Go, inspired by Borland C++/Turbo C.

**Status: Paused** — functional prototype, user evaluating Helix editor as alternative.

## Build & Run

```
go build -o numentext .
./numentext
```

No test suite exists yet. Verify changes compile with `go build ./...` and `go vet ./...`.

## Architecture

Go module `numentext` using tview (TUI), tcell (terminal), chroma (syntax highlighting).

```
main.go                        Entry point
internal/
  app/app.go                   Wires all components, menus, global keybindings, dialog lifecycle
  editor/
    editor.go                  Multi-tab editor: cursor, selection, clipboard, Draw to tcell.Screen
    buffer.go                  Line-based text buffer with undo/redo stack
    highlight.go               Chroma integration, per-character styling (CharStyle array per line)
    keymap.go                  tcell.EventKey -> Action enum mapping
    gutter.go                  Line number formatting
  ui/
    layout.go                  Main grid: menu + (filetree | editor) + output + status
    menubar.go                 Horizontal menu with dropdown submenus (custom tview.Box)
    statusbar.go               Bottom bar: file info, cursor pos, shortcuts
    dialog.go                  Modal dialogs: Open, Save As, Find, Replace, Go to Line, Confirm, About
    theme.go                   Borland blue color constants
  filetree/filetree.go         Directory tree via tview.TreeView
  output/output.go             Build/run output panel (tview.TextView)
  runner/
    runner.go                  Async compile & run orchestrator (30s timeout)
    languages.go               Per-language build/run commands (C, C++, Python, Rust, Go, JS, TS, Java)
  config/config.go             ~/.numentext/config.json persistence
```

## Key Patterns

- Editor renders directly to `tcell.Screen` in its `Draw()` method, not via tview widgets
- Syntax highlighting is cached; invalidated via `hlVersion` counter on content/tab changes
- Global input capture in app.go handles shortcuts; dialogs get priority when front page != "main"
- Clipboard uses `os/exec` to pbcopy/pbpaste (macOS) or xclip (Linux)
- Run/build executes async in a goroutine, updates UI via `QueueUpdateDraw`
- Dialog overlays use `tview.Pages` layered on top of the "main" page

## Conventions

- No emoji or Unicode box-drawing characters in UI — terminals render them inconsistently. Use ASCII only.
- File tree icons are single ASCII chars (e.g., `g` for .go, `c` for .c, `+` for directories).
- All user-facing text primitives must avoid `[` and `]` literals — tview interprets them as color tags. Use `tview.Escape()` or avoid brackets.
- Don't call `app.Draw()` from within input handlers — tview redraws automatically after input. Calling it causes re-entrancy freezes.

## Known Issues

See `.claude/projects/-Users-rlogman-projects-bcpp/memory/known-issues.md` for full list. Key ones:
- Mouse drag selection not implemented
- Find dialog is modal (blocks editor while open)
- Ctrl+Shift+S unreliable in terminals (Save As only via menu)
- No auto-indent, bracket matching, or multi-cursor

## Future Direction (if resumed)

1. Embedded PTY terminal via `creack/pty` (replace output panel with real shell)
2. LSP client (autocomplete, go-to-definition, diagnostics)
3. DAP client (debugger integration)
4. Tree-sitter for highlighting instead of chroma

The strategic idea: stay a small, fast Go binary that delegates intelligence to protocols (LSP, DAP) rather than building everything in-house.
