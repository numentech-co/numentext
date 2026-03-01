# Epic 1: Core Editor
The fundamental text editing engine that supports multi-file editing with full cursor control, text manipulation, and buffer management.

## Story 1.1: Text Buffer Management
- The editor shall maintain a line-based text buffer that supports insert, delete, and replace operations at arbitrary positions within the document
- The buffer shall correctly handle multi-line insertions and deletions including line splitting and merging across line boundaries
- The buffer shall normalize all line endings to LF on load, handling CR, CRLF, and LF input transparently
- The buffer shall track its modified state, returning true when content differs from last save and false after a successful save

Technical notes:
- Current implementation uses a line-based string slice with undo stack
- Consider migrating to piece table or rope for large files in the future

## Story 1.2: Undo and Redo
- The editor shall maintain an undo stack that records every insert and delete operation with cursor position before and after the change
- Pressing Ctrl+Z shall reverse the most recent operation and restore the cursor to its position before that operation was performed
- Pressing Ctrl+Y shall re-apply the most recently undone operation and restore the cursor to its position after that operation
- Performing a new edit after undoing shall clear the redo stack, making the undone operations no longer recoverable via redo

Technical notes:
- Operations stored as EditOp structs with type, text, and cursor positions
- Redo stack cleared on any new edit to maintain linear history

## Story 1.3: Cursor Movement
- Arrow keys shall move the cursor one character or one line in the corresponding direction, wrapping to adjacent lines at line boundaries
- Home and End keys shall move the cursor to the beginning and end of the current line respectively
- Ctrl+Home and Ctrl+End shall move the cursor to the beginning and end of the entire document respectively
- Ctrl+Left and Ctrl+Right shall move the cursor to the previous or next word boundary, skipping whitespace between words
- Page Up and Page Down shall move the cursor by one visible page height, clamping to document boundaries

## Story 1.4: Text Selection
- Holding Shift with any cursor movement key shall extend or create a selection from the anchor point to the new cursor position
- Ctrl+A shall select the entire document contents, placing the cursor at the end of the selection
- Any typing or paste operation while a selection is active shall first delete the selected text then insert the new content
- Moving the cursor without Shift held shall clear the current selection and place the cursor at the new position

## Story 1.5: Clipboard Integration
- Ctrl+C shall copy the currently selected text to the operating system clipboard using platform-native commands
- Ctrl+X shall copy the selected text to the clipboard and then delete it from the buffer in a single undoable operation
- Ctrl+V shall insert the operating system clipboard contents at the cursor position, replacing any active selection first
- Clipboard operations shall work on macOS via pbcopy/pbpaste and on Linux via xclip or xsel

Technical notes:
- Uses os/exec to shell out to platform clipboard commands
- No Windows support currently planned

## Story 1.6: Multi-Tab Editing
- The editor shall support multiple open files as tabs displayed in a tab bar above the editing area
- Clicking a tab or pressing Ctrl+1 through Ctrl+9 shall switch to the corresponding tab, preserving cursor and scroll position per tab
- Ctrl+W shall close the current tab, prompting to save if the buffer has unsaved modifications
- Opening a file that is already open in another tab shall switch to that existing tab instead of creating a duplicate
- The tab bar shall display an asterisk prefix on tab names with unsaved modifications

## Story 1.7: Basic Text Editing Operations
- Typing a printable character shall insert it at the cursor position, advancing the cursor one position to the right
- Enter shall split the current line at the cursor position, creating a new line and moving the cursor to its beginning
- Backspace shall delete the character before the cursor, or merge with the previous line if the cursor is at column zero
- Delete shall remove the character at the cursor position, or merge with the next line if the cursor is at the end of the line
- Tab shall insert four spaces at the cursor position rather than a literal tab character
- Ctrl+D shall delete the entire current line, shifting subsequent lines up to fill the gap

# Epic 2: Syntax Highlighting
Language-aware colorization of source code using lexical analysis to visually distinguish keywords, strings, comments, and other token types.

## Story 2.1: Language Detection
- The highlighter shall detect the programming language from the file extension, supporting c, cpp, py, rs, go, js, ts, java, and their variants
- Opening a file with an unrecognized extension shall fall back to plain text mode with no syntax coloring applied
- Changing a file's name via Save As shall re-detect the language and re-apply highlighting for the new file extension
- The detected language name shall be displayed in the status bar for the currently active tab

## Story 2.2: Token Colorization
- Keywords and reserved words shall render in white bold text to distinguish them from identifiers and other tokens
- String literals including single-quoted, double-quoted, backtick, and heredoc variants shall render in cyan
- Comments including single-line, multi-line, and documentation comments shall render in gray
- Numeric literals including integers, floats, hex, octal, and binary shall render in magenta
- Type names, class names, and decorators shall render in green to distinguish them from regular identifiers
- Function names at their definition site shall render in white bold

Technical notes:
- Uses chroma library for lexical analysis
- Per-character CharStyle array cached per line, invalidated on edit
- Consider tree-sitter migration for structural accuracy in the future

## Story 2.3: Highlighting Performance
- Syntax highlighting results shall be cached and only recomputed when the buffer content or active tab changes
- The cache shall use a version counter that increments on every edit, avoiding unnecessary re-tokenization on redraws
- Highlighting shall process the entire file as a single unit to ensure multi-line tokens like block comments are handled correctly
- Highlighting shall not block user input; rendering must remain responsive even for files with thousands of lines

# Epic 3: File Management
All operations related to opening, saving, browsing, and managing files and directories within the IDE.

## Story 3.1: File Open
- Ctrl+O shall open a modal file browser dialog showing the current directory contents with directories listed before files
- The dialog shall include a path input field where users can type or paste an absolute path and press Enter to navigate
- Selecting a file in the browser shall open it in a new editor tab and close the dialog, returning focus to the editor
- Selecting a directory shall navigate into it, refreshing the file list to show the new directory contents
- Hidden files and directories whose names begin with a dot shall be excluded from the file browser listing

## Story 3.2: File Save
- Ctrl+S shall save the current tab's buffer contents to its associated file path, clearing the modified flag on success
- If the current tab has no file path (untitled), Ctrl+S shall behave as Save As, prompting for a file path first
- Save errors such as permission denied or disk full shall display an error message in the output panel without losing buffer content
- After a successful save, the status bar shall briefly display a confirmation message showing the saved file path

## Story 3.3: Save As
- Save As shall open a dialog with an input field pre-filled with the current file path or a default path for untitled files
- Confirming the dialog shall write the buffer to the specified path, update the tab name, and re-detect the file language
- The recently opened files list shall be updated to include the new file path after a successful Save As operation

## Story 3.4: File Tree Browser
- A persistent file tree panel shall occupy the left side of the layout, showing the working directory structure as a collapsible tree
- Clicking a file node shall open that file in a new editor tab or switch to its existing tab if already open
- Clicking a directory node shall toggle its expanded or collapsed state, loading children lazily on first expansion
- The file tree shall display a single-character type indicator before each file name based on its extension
- A Refresh option in the Tools menu shall reload the entire file tree to reflect external filesystem changes

## Story 3.5: New File
- Ctrl+N shall create a new empty tab labeled untitled with no associated file path and an empty buffer
- The new tab shall become the active tab immediately and receive keyboard focus for editing
- Multiple untitled tabs shall be supported simultaneously without naming conflicts

## Story 3.6: Recent Files
- The application shall maintain a list of the 20 most recently opened or saved file paths in its configuration file
- The File menu shall include a Recent Files submenu listing these paths, with the most recent file listed first
- Selecting a recent file entry shall open that file in a new tab, or switch to its tab if already open

Technical notes:
- Stored in ~/.numentext/config.json
- Duplicates removed, list capped at 20 entries

# Epic 4: Search and Navigation
Finding text, replacing text, and navigating to specific locations within the editor buffer.

## Story 4.1: Find
- Ctrl+F shall open a Find dialog with a text input field and a Find Next button
- Find shall perform case-insensitive search forward from the current cursor position, wrapping to the beginning if no match is found
- A successful match shall move the cursor to the match position and select the matched text to make it visually obvious
- If no match is found, the status bar shall display a "Not found" message with the search term

## Story 4.2: Replace
- Ctrl+H shall open a Replace dialog with Find and Replace input fields plus Find, Replace, and Replace All buttons
- Replace shall substitute the currently selected match with the replacement text and then advance to the next match
- Replace All shall substitute every occurrence of the search term throughout the entire buffer in a single undoable operation
- The status bar shall report the number of replacements made after a Replace All operation

## Story 4.3: Go to Line
- Ctrl+G shall open a dialog with a numeric input field accepting a line number
- Entering a valid line number shall move the cursor to the beginning of that line and scroll to make it visible
- Line numbers outside the valid range shall be clamped to the first or last line of the document without showing an error

# Epic 5: Build and Run System
Compiling source code and executing programs with output captured and displayed within the IDE.

## Story 5.1: Language Build Commands
- The system shall detect the programming language from the file extension and select the appropriate compiler or interpreter command
- C files shall build with gcc using the -Wall flag, C++ with g++ using -Wall and -std=c++17
- Rust files shall compile with rustc, Go files with go build, and Java files with javac
- Python, JavaScript, and TypeScript files shall be treated as interpreted and require no separate build step

## Story 5.2: Run Execution
- F5 shall save the current file, compile it if the language requires compilation, and then execute the resulting program
- F9 shall save and compile only, without executing, displaying build success or error messages in the output panel
- The output panel shall display the command being executed, its stdout and stderr output, and the exit code with elapsed time
- Build and run operations shall execute asynchronously so the IDE interface remains responsive during long-running processes
- A 30-second timeout shall terminate processes that run too long, displaying a timeout message in the output panel

## Story 5.3: Process Control
- The Run menu shall include a Stop option that terminates any currently running build or execution process
- The output panel shall display a confirmation message when a process is manually stopped by the user
- Starting a new run while a process is active shall terminate the existing process before launching the new one

## Story 5.4: Output Panel
- The output panel shall occupy the bottom portion of the layout, displaying build and run results with scrollback
- Commands shall be displayed in cyan, error output in red, and success messages in green for visual distinction
- The output panel shall auto-scroll to the latest output and support manual scrolling through previous output
- The Tools menu shall include a Clear Output option that erases all output panel content

# Epic 6: User Interface
The visual layout, menus, status indicators, and dialog system that provide the IDE's interactive surface.

## Story 6.1: Application Layout
- The layout shall consist of a menu bar at top, file tree on the left, editor area in the center, output panel at the bottom, and status bar at the very bottom
- The file tree shall have a fixed width of 20 columns, the output panel a fixed height of 8 rows, and the editor shall fill remaining space
- All panels shall render with the Borland blue color scheme using dark blue backgrounds, cyan accents, and yellow text
- The application shall enable mouse support at startup, allowing click, scroll, and drag interactions throughout the interface

## Story 6.2: Menu Bar
- The menu bar shall display eight menus: File, Edit, Search, Run, Tools, Options, Window, and Help
- Clicking a menu label shall open a dropdown showing its items with labels on the left and keyboard shortcuts on the right
- Arrow keys shall navigate between menus horizontally and between items vertically when a dropdown is open
- Pressing Enter on a menu item shall execute its action and close the dropdown, returning focus to the editor
- Pressing Escape or clicking outside the dropdown shall close it without executing any action
- F10 shall open the first menu and focus the menu bar for keyboard-driven navigation

## Story 6.3: Status Bar
- The status bar shall display the current filename, cursor line and column numbers, detected language, and file encoding
- Modified files shall show a modified indicator next to the filename in the status bar
- The right side of the status bar shall display keyboard shortcut hints: F5 Run, F9 Build, and F10 Menu
- When no file is open, the status bar shall display the application name and a hint about opening or creating files

## Story 6.4: Modal Dialogs
- Dialogs shall appear as centered overlays on top of the main layout, capturing all keyboard and mouse input
- All dialogs shall close when the user presses Escape, returning focus to the previously focused component
- Dialog input fields shall use the editor's blue background with white text, and buttons shall use blue background with white text
- A confirmation dialog shall appear when closing a tab with unsaved changes, offering Yes and No options

## Story 6.5: Welcome Screen
- When no tabs are open, the editor area shall display an ASCII art NumenText logo centered in the available space
- The welcome screen shall list the most important keyboard shortcuts: Ctrl+N, Ctrl+O, Ctrl+S, Ctrl+Q, F5, F9, Ctrl+F, Ctrl+G, and F10
- The welcome screen shall include the tagline "A Modern Terminal IDE" and "Inspired by Borland C++"

## Story 6.6: Color Theme
- The default theme shall use dark blue (#000080) for editor and file tree backgrounds, cyan for menu bar and status bar
- Editor text shall default to yellow, with white bold for keywords, cyan for strings, gray for comments, and magenta for numbers
- Selected text shall render as black text on a cyan background to provide clear visual contrast
- Active tabs shall display white text on blue, inactive tabs gray text on darker blue, with the tab bar in the darkest blue

# Epic 7: Configuration and Persistence
Saving user preferences, application state, and settings across sessions.

## Story 7.1: Configuration File
- The application shall store configuration as JSON in ~/.numentext/config.json, creating the directory if it does not exist
- Configuration shall include tab size, theme name, line number visibility, word wrap preference, and the recent files list
- The configuration shall be loaded at startup and saved whenever a preference changes or a file is opened or saved
- Missing or corrupt configuration files shall be silently replaced with default values without displaying errors to the user

## Story 7.2: Preference Options
- The Options menu shall include a Toggle Line Numbers item that shows or hides the line number gutter in the editor
- Preference changes shall take effect immediately without requiring a restart or manual reload of the configuration
- All preference values shall persist across sessions by writing to the configuration file after each change

# Epic 8: Keyboard Shortcuts
Comprehensive keyboard shortcut system providing efficient access to all major IDE functions.

## Story 8.1: Global Shortcuts
- Ctrl+N, Ctrl+O, Ctrl+S, Ctrl+W, and Ctrl+Q shall work from any focused component to create, open, save, close, and quit
- F5, F9, and F10 shall work globally to run the current file, build the current file, and open the menu bar
- Ctrl+F, Ctrl+H, and Ctrl+G shall open the Find, Replace, and Go to Line dialogs from any context
- Escape shall close the current dialog or menu if one is open, otherwise return focus to the editor from any panel

## Story 8.2: Editor Shortcuts
- Ctrl+Z and Ctrl+Y shall undo and redo within the active editor tab's buffer
- Ctrl+X, Ctrl+C, and Ctrl+V shall cut, copy, and paste using the operating system clipboard
- Ctrl+A shall select all text in the active buffer
- Ctrl+D shall delete the current line
- Ctrl+Tab shall cycle to the next open tab, wrapping from the last tab to the first

## Story 8.3: Tab Switching
- Ctrl+1 through Ctrl+9 shall switch directly to the tab at that numeric position if it exists
- Ctrl+Tab shall switch to the next tab in order, wrapping around from the last tab to the first tab

# Epic 9: Mouse Support
Full mouse interaction for cursor placement, text selection, scrolling, and UI navigation.

## Story 9.1: Editor Mouse Interaction
- Left-clicking in the editor area shall position the cursor at the clicked row and column, clearing any active selection
- Scroll wheel shall scroll the editor view vertically by three lines per scroll increment without moving the cursor
- Left-clicking on a tab in the tab bar shall switch to that tab, making it active and displaying its content

## Story 9.2: File Tree Mouse Interaction
- Left-clicking a file in the file tree shall open that file in the editor, creating a new tab or switching to an existing one
- Left-clicking a directory node shall toggle its expanded or collapsed state in the tree view
- The file tree shall support scroll wheel navigation when it contains more entries than the visible area

## Story 9.3: Menu Mouse Interaction
- Left-clicking a menu label in the menu bar shall open its dropdown, or close it if already open
- Left-clicking a menu item in an open dropdown shall execute that item's action and close the dropdown
- Left-clicking outside an open dropdown shall close the dropdown without executing any action

# Epic 10: Cross-Platform and Deployment
Ensuring the IDE runs reliably across operating systems and can be easily deployed to remote machines.

## Story 10.1: Single Binary Distribution
- The application shall compile to a single static binary with no external runtime dependencies using standard Go build
- Cross-compilation shall be supported via GOOS and GOARCH environment variables for at least linux/amd64 and darwin/arm64
- The binary shall be functional immediately after copying to a target machine with no installation steps or configuration required

## Story 10.2: Platform Compatibility
- The application shall run correctly in common terminal emulators including iTerm2, Terminal.app, GNOME Terminal, and Alacritty
- All user interface elements shall use only ASCII characters to ensure correct rendering across all terminal configurations
- The application shall detect the operating system at runtime to select the correct clipboard command for macOS or Linux

# Epic 11: Embedded Terminal
A real interactive terminal emulator embedded within the IDE for running shell commands without leaving the editor.

## Story 11.1: PTY Integration
- The IDE shall spawn a real pseudo-terminal using the creack/pty library, running the user's default shell
- The embedded terminal shall support interactive programs, ANSI escape sequences, and cursor addressing for full terminal compatibility
- Terminal sessions shall persist across tab switches and panel resizes, maintaining scroll position and shell state
- The terminal shall properly handle window resize events, sending SIGWINCH to update the PTY dimensions

Technical notes:
- Use github.com/creack/pty for PTY spawning
- Need a VT100/ANSI state machine to parse escape sequences into a cell grid
- Render the cell grid using tcell like the editor does

## Story 11.2: Terminal Panel
- The terminal shall replace the current output panel as the bottom panel, togglable between terminal mode and output-only mode
- The user shall be able to switch between the terminal and the editor using a keyboard shortcut without losing terminal state
- Build and run output shall optionally be sent to the embedded terminal instead of the output panel for interactive program support
- The terminal panel shall support scrollback of at least 1000 lines of previous output

# Epic 12: Language Server Protocol
Integration with LSP servers to provide intelligent code assistance including completions, diagnostics, and navigation.

## Story 12.1: LSP Client Core
- The IDE shall include an LSP client that communicates with language servers over stdin/stdout using the JSON-RPC protocol
- The client shall automatically detect and launch the appropriate language server based on the current file's language
- The client shall handle the LSP initialization handshake, capabilities negotiation, and document synchronization lifecycle
- The client shall gracefully handle language server crashes by displaying an error and allowing manual restart

Technical notes:
- Common servers: gopls (Go), clangd (C/C++), pyright (Python), rust-analyzer (Rust), typescript-language-server (JS/TS)
- Consider using an existing Go LSP client library rather than implementing the protocol from scratch

## Story 12.2: Autocomplete
- The editor shall request completions from the language server as the user types, displaying suggestions in a popup near the cursor
- Arrow keys shall navigate the completion list, Enter or Tab shall insert the selected completion, and Escape shall dismiss it
- Completions shall include function signatures, variable names, type names, and module members as provided by the language server
- The completion popup shall appear automatically after typing a trigger character such as a dot or double colon

## Story 12.3: Diagnostics
- The editor shall display language server diagnostics as colored markers in the gutter: red for errors, yellow for warnings
- Hovering or moving the cursor to a line with a diagnostic shall display the diagnostic message in the status bar
- Diagnostics shall update in real time as the user edits, reflecting the latest analysis from the language server
- The output panel shall provide a summary list of all current diagnostics with file, line, and message

## Story 12.4: Go to Definition
- A keyboard shortcut shall send a go-to-definition request to the language server for the symbol under the cursor
- If the definition is in the current file, the cursor shall jump to that location with the source line centered on screen
- If the definition is in a different file, that file shall open in a new tab with the cursor positioned at the definition
- If no definition is found, the status bar shall display a message indicating no definition was available

## Story 12.5: Hover Information
- A keyboard shortcut shall request hover information from the language server for the symbol under the cursor
- Hover results shall display in a tooltip popup near the cursor showing type signatures, documentation, or other metadata
- The popup shall dismiss when the user presses Escape, moves the cursor, or begins typing

# Epic 13: Debug Adapter Protocol
Integration with DAP servers to provide interactive debugging with breakpoints, stepping, and variable inspection.

## Story 13.1: DAP Client Core
- The IDE shall include a DAP client that communicates with debug adapters over stdin/stdout using the JSON protocol
- The client shall support the launch and attach request types for starting or connecting to debug sessions
- The client shall handle the DAP initialization sequence, capabilities exchange, and session lifecycle events
- The client shall support at minimum Go (delve), Python (debugpy), C/C++ (gdb/lldb via cpptools), and Rust (codelldb) adapters

Technical notes:
- DAP is simpler than LSP, fewer message types
- Debug adapters are separate binaries, similar architecture to language servers

## Story 13.2: Breakpoints
- The user shall be able to toggle a breakpoint on the current line using a keyboard shortcut, displaying a red marker in the gutter
- Breakpoints shall be sent to the debug adapter when a debug session starts and updated in real time during the session
- The editor shall visually highlight the current execution line during a debug pause with a distinct background color
- Breakpoints shall persist within a session but do not need to be saved across application restarts

## Story 13.3: Stepping and Control
- The debug menu or toolbar shall provide Continue, Step Over, Step Into, Step Out, and Stop controls for debug sessions
- Each stepping action shall update the current execution line indicator and refresh the variable display
- The status bar shall indicate when a debug session is active and whether the program is running or paused at a breakpoint

## Story 13.4: Variable Inspection
- When paused at a breakpoint, a variables panel shall display local variables, function arguments, and their current values
- Variables with complex types shall be expandable to show their fields or elements in a tree structure
- The variable display shall update automatically after each stepping operation to reflect the current program state
