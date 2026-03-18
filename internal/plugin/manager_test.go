package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockHost is a test implementation of PluginHost.
type mockHost struct {
	statusMsg    string
	outputBuf    string
	openedFile   string
	insertedText string
	cursorRow    int
	cursorCol    int
	commands     map[string]func()
	menuItems    map[string]func()
	panelContent map[string]string
	shownPanels  map[string]bool
}

func newMockHost() *mockHost {
	return &mockHost{
		commands:     make(map[string]func()),
		menuItems:    make(map[string]func()),
		panelContent: make(map[string]string),
		shownPanels:  make(map[string]bool),
	}
}

func (h *mockHost) OpenFile(path string) error    { h.openedFile = path; return nil }
func (h *mockHost) ActiveFilePath() string         { return "/test/file.go" }
func (h *mockHost) ActiveFileContent() string      { return "package main\n" }
func (h *mockHost) CursorPosition() (int, int)     { return h.cursorRow, h.cursorCol }
func (h *mockHost) SetCursor(row, col int)         { h.cursorRow = row; h.cursorCol = col }
func (h *mockHost) InsertText(text string)         { h.insertedText = text }
func (h *mockHost) SelectedText() string           { return "" }
func (h *mockHost) ReplaceSelection(text string)   { h.insertedText = text }
func (h *mockHost) SetContent(text string)         {}
func (h *mockHost) SetStatusMessage(msg string)    { h.statusMsg = msg }
func (h *mockHost) AppendOutput(text string)       { h.outputBuf += text }
func (h *mockHost) ShowPanel(name string)          { h.shownPanels[name] = true }
func (h *mockHost) HidePanel(name string)          { delete(h.shownPanels, name) }
func (h *mockHost) SetPanelContent(name, t string) { h.panelContent[name] = t }
func (h *mockHost) AppendPanelContent(name, t string) {
	h.panelContent[name] += t
}
func (h *mockHost) AddMenuItem(menuName, label string, action func()) {
	h.menuItems[menuName+"/"+label] = action
}
func (h *mockHost) AddCommand(id, title string, action func()) {
	h.commands[id] = action
}
func (h *mockHost) Exec(command string, args []string, workDir string) (string, error) {
	return "", nil
}
func (h *mockHost) SetGutterMarkers(filePath string, markers map[int]string) {}
func (h *mockHost) RenderMarkdownToPanel(name, markdown string) {
	h.panelContent[name] = RenderMarkdownToTview(markdown)
}
func (h *mockHost) RenderCodeToPanel(name, code, language string) {
	h.panelContent[name] = RenderCodeToTview(code, language)
}

func createTestPlugin(t *testing.T, dir, name, initLua string) string {
	t.Helper()
	pluginDir := filepath.Join(dir, name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name": "` + name + `", "version": "1.0.0", "description": "test"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(initLua), 0o644); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func TestManager_LoadPlugin(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "hello", `
		numen.status_message("Hello from plugin!")
	`)

	err := mgr.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	plugins := mgr.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	entry := plugins["hello"]
	if entry == nil {
		t.Fatal("expected plugin 'hello'")
	}
	if !entry.Enabled {
		t.Error("expected plugin to be enabled")
	}
	if host.statusMsg != "Hello from plugin!" {
		t.Errorf("expected status message, got %q", host.statusMsg)
	}
}

func TestManager_DisabledPlugin(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)
	mgr.SetDisabledPlugins([]string{"hello"})

	createTestPlugin(t, dir, "hello", `numen.status_message("should not run")`)

	err := mgr.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	entry := mgr.Plugins()["hello"]
	if entry == nil {
		t.Fatal("expected entry for disabled plugin")
	}
	if entry.Enabled {
		t.Error("expected plugin to be disabled")
	}
	if host.statusMsg != "" {
		t.Errorf("disabled plugin should not set status, got %q", host.statusMsg)
	}
}

func TestManager_HostAPI(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "api-test", `
		-- Test active_file
		local path = numen.active_file()
		numen.output_append("file:" .. path)

		-- Test active_content
		local content = numen.active_content()
		numen.output_append("|content:" .. content)

		-- Test cursor
		numen.set_cursor(5, 10)
		local r, c = numen.cursor_position()
		numen.output_append("|cursor:" .. r .. "," .. c)

		-- Test insert_text
		numen.insert_text("hello world")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	if host.outputBuf != "file:/test/file.go|content:package main\n|cursor:5,10" {
		t.Errorf("unexpected output: %q", host.outputBuf)
	}
	if host.insertedText != "hello world" {
		t.Errorf("unexpected inserted text: %q", host.insertedText)
	}
	if host.cursorRow != 5 || host.cursorCol != 10 {
		t.Errorf("unexpected cursor: %d,%d", host.cursorRow, host.cursorCol)
	}
}

func TestManager_CommandRegistration(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "cmd-test", `
		numen.register_command("test.greet", "Greet", function()
			numen.status_message("Greetings!")
		end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	cmds := mgr.Registry().Commands()
	if len(cmds) != 1 || cmds[0].ID != "test.greet" {
		t.Errorf("unexpected commands: %v", cmds)
	}

	// Execute the command through host
	if action, ok := host.commands["test.greet"]; ok {
		action()
	} else {
		t.Fatal("command not registered with host")
	}
	if host.statusMsg != "Greetings!" {
		t.Errorf("expected 'Greetings!', got %q", host.statusMsg)
	}
}

func TestManager_EventDispatching(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "event-test", `
		numen.on("file_open", function(path)
			numen.status_message("opened: " .. path)
		end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	mgr.HandleFileOpen("/tmp/test.txt")
	if host.statusMsg != "opened: /tmp/test.txt" {
		t.Errorf("expected event result, got %q", host.statusMsg)
	}
}

func TestManager_UnloadPlugin(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "unload-test", `
		numen.register_command("unload.cmd", "Unload Test", function() end)
		numen.on("file_open", function() end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	if err := mgr.UnloadPlugin("unload-test"); err != nil {
		t.Fatal(err)
	}

	if len(mgr.Plugins()) != 0 {
		t.Error("expected 0 plugins after unload")
	}
	if len(mgr.Registry().Commands()) != 0 {
		t.Error("expected 0 commands after unload")
	}
	if mgr.Events().ListenerCount("file_open") != 0 {
		t.Error("expected 0 listeners after unload")
	}
}

func TestManager_FileHandler(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "ipynb-viewer", `
		numen.register_file_handler(".ipynb", function(path, content)
			numen.status_message("handling: " .. path)
		end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	if !mgr.HasFileHandler(".ipynb") {
		t.Error("expected file handler for .ipynb")
	}

	err := mgr.InvokeFileHandler(".ipynb", "/test/notebook.ipynb", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if host.statusMsg != "handling: /test/notebook.ipynb" {
		t.Errorf("unexpected status: %q", host.statusMsg)
	}
}

func TestManager_PanelOperations(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "panel-test", `
		numen.register_panel("preview", "bottom")
		numen.panel_set_content("preview", "Hello")
		numen.panel_append("preview", " World")
		numen.panel_show("preview")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	if host.panelContent["preview"] != "Hello World" {
		t.Errorf("expected panel content 'Hello World', got %q", host.panelContent["preview"])
	}
	if !host.shownPanels["preview"] {
		t.Error("expected panel to be shown")
	}
}

func TestManager_PanelOnSelect(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "select-test", `
		numen.register_panel("items", "bottom")
		numen.panel_set_content("items", "apple\nbanana\ncherry")
		numen.panel_on_select("items", function(idx, text)
			numen.status_message("selected:" .. idx .. ":" .. text)
		end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	p, ok := mgr.Registry().Panel("items")
	if !ok {
		t.Fatal("expected panel 'items'")
	}
	if p.SelectCallback == nil {
		t.Fatal("expected select callback to be set")
	}
}

func TestManager_PanelOnKey(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "key-test", `
		numen.register_panel("interactive", "bottom")
		numen.panel_on_key("interactive", function(key)
			numen.status_message("key:" .. key)
			return true
		end)
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	p, ok := mgr.Registry().Panel("interactive")
	if !ok {
		t.Fatal("expected panel 'interactive'")
	}
	if p.KeyCallback == nil {
		t.Fatal("expected key callback to be set")
	}
}

func TestManager_RenderMarkdown(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "md-test", `
		numen.register_panel("doc", "bottom")
		numen.render_markdown("doc", "# Hello\nSome **bold** text")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	content := host.panelContent["doc"]
	if content == "" {
		t.Fatal("expected panel content to be set")
	}
	if !strings.Contains(content, "Hello") {
		t.Errorf("expected 'Hello' in rendered content, got %q", content)
	}
	if !strings.Contains(content, "bold") {
		t.Errorf("expected 'bold' in rendered content, got %q", content)
	}
}

func TestManager_RenderCode(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "code-test", `
		numen.register_panel("source", "bottom")
		numen.render_code("source", "def hello():\n    pass", "Python")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	content := host.panelContent["source"]
	if content == "" {
		t.Fatal("expected panel content to be set")
	}
	if !strings.Contains(content, "def") {
		t.Errorf("expected 'def' in rendered content, got %q", content)
	}
}

func TestManager_PanelCellOperations(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "cell-test", `
		numen.register_panel("notebook", "bottom")
		numen.panel_add_cell("notebook", "code", "Go", "package main")
		numen.panel_add_cell("notebook", "markdown", "", "# Title")
		numen.panel_add_cell("notebook", "output", "", "result here")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	cells := mgr.Registry().PanelCells("notebook")
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(cells))
	}
	if cells[0].Type != "code" || cells[0].Language != "Go" || cells[0].Content != "package main" {
		t.Errorf("unexpected cell 0: %+v", cells[0])
	}
	if cells[1].Type != "markdown" || cells[1].Content != "# Title" {
		t.Errorf("unexpected cell 1: %+v", cells[1])
	}
	if cells[2].Type != "output" || cells[2].Content != "result here" {
		t.Errorf("unexpected cell 2: %+v", cells[2])
	}
}

func TestManager_PanelClearCells(t *testing.T) {
	dir := t.TempDir()
	host := newMockHost()
	mgr := NewManager(host)
	mgr.SetPluginDir(dir)

	createTestPlugin(t, dir, "clear-test", `
		numen.register_panel("nb", "bottom")
		numen.panel_add_cell("nb", "code", "Go", "x := 1")
		numen.panel_add_cell("nb", "raw", "", "text")
		numen.panel_clear_cells("nb")
	`)

	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}

	cells := mgr.Registry().PanelCells("nb")
	if len(cells) != 0 {
		t.Errorf("expected 0 cells after clear, got %d", len(cells))
	}
}

func TestManager_EmptyPluginDir(t *testing.T) {
	mgr := NewManager(newMockHost())
	mgr.SetPluginDir("/nonexistent/path")

	err := mgr.LoadAll()
	if err != nil {
		t.Errorf("LoadAll should not fail for missing plugin dir, got: %v", err)
	}
}

func TestManager_MissingInitLua(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "no-init")
	os.MkdirAll(pluginDir, 0o755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"),
		[]byte(`{"name":"no-init","version":"1.0.0"}`), 0o644)

	mgr := NewManager(newMockHost())
	mgr.SetPluginDir(dir)

	var logged string
	mgr.SetOnLog(func(msg string) { logged = msg })

	mgr.LoadAll()

	if !strings.Contains(logged, "init.lua not found") {
		t.Errorf("expected log about missing init.lua, got: %q", logged)
	}
}
