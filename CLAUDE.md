# NumenText

Terminal-based IDE written in Go, inspired by Borland C++/Turbo C.

## Development Workflow

This project uses the [Artifex](https://github.com/numentech-co/artifex) structured development toolkit. Requirements are written as markdown, converted to tickets, and implemented through the ticket-implementation agent workflow with validation gates. Use `/next-ticket`, `/mark-done`, and `/ticket-status` slash commands to manage work.

## Build & Run

```
go build -o numentext .
./numentext [file...]
```

Verify changes compile with `go build ./...`, `go vet ./...`, and `go test ./...`.

## Architecture

Go module `numentext` using tview (TUI), tcell (terminal), chroma (syntax highlighting), gopher-lua (plugin system).

```
main.go                        Entry point, CLI flags (--merge, --plugin-*)
internal/
  app/app.go                   Wires all components, menus, global keybindings, dialog lifecycle
  editor/
    editor.go                  Multi-tab editor: cursor, selection, clipboard, Draw to tcell.Screen
    buffer.go                  Line-based text buffer with undo/redo stack
    highlight.go               Chroma integration, per-character styling
    keymap.go                  tcell.EventKey -> Action enum mapping
    gutter.go                  Line number formatting
    bracket.go                 Bracket matching and highlighting
    markdown.go                Inline markdown rendering (bold, italic, headers, links, code)
    markdown_blocks.go         Block markdown rendering (code blocks, tables, blockquotes, lists)
    markdown_blocks_draw.go    Block element Draw methods
    tools.go                   External formatter/linter execution
    diffmarkers.go             Diff change type enum for gutter markers
  ui/
    layout.go                  Main grid: menu + (filetree | editor) + output + status
    menubar.go                 Horizontal menu with dropdown submenus
    statusbar.go               Bottom bar: file info, cursor pos, shortcuts
    dialog.go                  Modal dialogs: Open, Save As, Find, Replace, Go to Line, Confirm
    theme.go                   Color constants (semantic names)
    themes.go                  4 built-in themes + ApplyTheme
    style.go                   Style registry (modern/classic, icon sets, terminal detection)
  filetree/filetree.go         Directory tree via tview.TreeView
  output/output.go             Build/run output panel with mouse selection
  terminal/
    terminal.go                VT100 state machine + PTY via creack/pty
    panel.go                   Terminal panel widget with mouse selection
    block.go                   Command block detection and rendering
  runner/
    runner.go                  Async compile & run orchestrator
    languages.go               Per-language build/run commands (C, C++, Python, Go, Rust, JS, TS, Java, Kotlin)
    testrunner.go              Unit test runner with framework detection
    errorparse.go              Build error parser (gcc, go, python, rustc, javac, tsc, node, kotlin)
  lsp/                         JSON-RPC 2.0 LSP client
  dap/                         DAP client for debugging
  hexview/hexview.go           Binary hex viewer/editor (two-pane)
  diffview/                    Side-by-side diff viewer with word-level diffs
  graphics/                    Image loading, Sixel/Kitty encoding, capability detection
  plugin/                      Lua plugin system (gopher-lua)
    manager.go                 Plugin loading, manifest parsing, lifecycle
    luaruntime.go              Sandboxed Lua VM with timeout
    hostapi.go                 numen.* API exposed to Lua plugins
    extensions.go              Command/menu/panel/event registration
    render.go                  Markdown and code rendering for plugin panels
  config/
    config.go                  ~/.numentext/config.json persistence
    defaults.go                Default tool configs per language
    venv.go                    Python virtual environment detection
```

## Key Patterns

- Editor renders directly to `tcell.Screen` in its `Draw()` method, not via tview widgets
- Syntax highlighting is cached; invalidated via `hlVersion` counter on content/tab changes
- Global input capture in app.go handles shortcuts via unified Ctrl+letter handler that normalizes across terminal key encodings
- Clipboard uses `os/exec` to pbcopy/pbpaste (macOS) or xclip (Linux)
- Run/build executes async in a goroutine, updates UI via `QueueUpdateDraw`
- Dialog overlays use `tview.Pages` layered on top of the "main" page
- Plugins use embedded Lua (gopher-lua) with sandboxed API exposure via numen.* table
- Color constants use semantic names (ColorBgAlt, ColorTextPrimary, ColorTextMuted) not appearance names

## Conventions

- No emoji in code or UI
- Style-aware UI: all UI characters (borders, icons, indicators) come from the style registry (`ui.Style`). Classic mode uses ASCII; modern mode uses Unicode.
- All user-facing text primitives must avoid `[` and `]` literals -- tview interprets them as color tags. Use `tview.Escape()`.
- Don't call `app.Draw()` from within input handlers -- tview redraws automatically after input. Calling it causes re-entrancy freezes.
- Color constants must use semantic names describing their role, not their appearance (e.g., `ColorBgAlt` not `ColorBgDarker`).
- Temporary/intermediate artifacts (requirements JSON files, planning docs) go in `.tmp/` which is gitignored. Never commit these to the repo.
- Ticket database (`tickets.db`) is a local dev artifact, gitignored. Use the ticket-manager skill to manage it.

## Plugins

Plugins live in `~/.numentext/plugins/<name>/` with `plugin.json` + `init.lua`.
Install: `numentext --plugin-install github.com/org/repo`

Official plugins (under numentech-co):
- numentext-html -- HTML encode/decode, entity picker
- numentext-todo -- TODO/FIXME annotation scanner
- numentext-git -- Git diff markers, diff view, merge tool
- numentext-jupyter -- Jupyter notebook viewer/editor

Plugin API (numen.* table): open_file, active_file, active_content, selected_text, replace_selection, set_content, cursor_position, set_cursor, insert_text, status_message, output_append, register_command, register_menu_item, register_shortcut, register_panel, panel_set_content, panel_show, panel_hide, panel_on_select, panel_on_key, render_markdown, render_code, panel_add_cell, panel_clear_cells, on (events), exec, set_gutter_markers.
