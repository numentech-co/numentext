# NumenText

A terminal-based IDE written in Go, inspired by Borland C++ and Turbo C.

NumenText is a non-modal, menu-driven editor for people who want a capable IDE in the terminal without learning vim or modal editing. Familiar shortcuts (Ctrl+S, Ctrl+C, F5 to run) work out of the box.

## Features

- **Multi-tab editor** with syntax highlighting (via Chroma) for 20+ languages
- **Integrated terminal** with PTY support and command block mode
- **LSP client** -- autocomplete, go-to-definition, hover info, diagnostics (auto-detects gopls, pyright, clangd, rust-analyzer, typescript-language-server)
- **DAP client** -- debugger integration with breakpoints, step over/in/out (dlv, debugpy, lldb-vscode)
- **Build and run** -- F5 to run, F9 to build (C, C++, Go, Rust, Python, JavaScript, TypeScript, Java)
- **File tree** with directory browsing
- **Find, replace, go-to-line, search in files**
- **Command palette** (Ctrl+Shift+P) and quick file open (Ctrl+P)
- **Resizable panels** -- keyboard (Ctrl+Shift+Arrows) or mouse drag
- **Vi and Helix keybinding modes** (Ctrl+Shift+M to cycle)
- **Persistent config** -- panel sizes, recent files, preferences saved to `~/.numentext/config.json`
- **Single binary**, no runtime dependencies

## Install

Requires Go 1.25 or later.

```
git clone https://github.com/numentech-co/numentext.git
cd numentext
go build -o numentext .
./numentext
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+N | New file |
| Ctrl+O | Open file |
| Ctrl+S | Save |
| Ctrl+W | Close tab |
| Ctrl+Q | Quit |
| Ctrl+Z / Ctrl+Y | Undo / Redo |
| Ctrl+X / Ctrl+C / Ctrl+V | Cut / Copy / Paste |
| Ctrl+F | Find |
| Ctrl+H | Replace |
| Ctrl+G | Go to line |
| Ctrl+P | Quick file open |
| Ctrl+Shift+P | Command palette |
| Ctrl+Shift+F | Search in files |
| F5 | Run |
| F9 | Build |
| F10 | Menu bar |
| F11 | Hover info (LSP) |
| F12 | Go to definition (LSP) |
| F8 | Toggle breakpoint |
| F6 | Debug continue |
| F7 | Step over |
| Ctrl+` | Toggle terminal |
| Ctrl+Tab | Next tab |
| Ctrl+] | Next panel |
| Ctrl+Shift+Arrows | Resize panels |
| Ctrl+Shift+M | Cycle keyboard mode |

## Architecture

```
main.go                        Entry point
internal/
  app/app.go                   Wires components, menus, keybindings, dialogs
  editor/                      Multi-tab editor with buffer, undo/redo, completion
  terminal/                    VT100 state machine + PTY via creack/pty
  lsp/                         JSON-RPC 2.0 LSP client
  dap/                         DAP client for debugging
  ui/                          MenuBar, StatusBar, Dialogs, Theme
  runner/                      Build/run orchestrator for multiple languages
  filetree/                    Directory tree panel
  config/                      Persistent config (~/.numentext/config.json)
```

The design philosophy: stay a small, fast Go binary that delegates intelligence to protocols (LSP, DAP) rather than reimplementing language features.

## Contributing

Contributions are welcome. Please follow these guidelines:

### Getting Started

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Ensure the build passes: `go build ./... && go vet ./...`
5. Submit a pull request

### Code Guidelines

- **Keep it simple.** NumenText is intentionally minimal. Don't add features that aren't needed yet.
- **Style-aware UI.** All UI characters (borders, icons, indicators) must come from the style registry (`ui.Style`), never hardcoded. Classic mode uses ASCII only; modern mode uses Unicode. Never use emoji.
- **Escape tview brackets.** All user-facing text must avoid literal `[` and `]` -- tview interprets them as color tags. Use `tview.Escape()`.
- **No `app.Draw()` in input handlers.** tview redraws automatically after input. Manual calls cause re-entrancy freezes.
- **Test your changes compile.** There is no full test suite yet, but `go build ./...` and `go vet ./...` must pass.
- **One concern per PR.** Keep pull requests focused on a single change.

### Reporting Issues

Open an issue with:
- What you expected to happen
- What actually happened
- Your terminal emulator and OS
- Steps to reproduce

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
