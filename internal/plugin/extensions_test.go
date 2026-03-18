package plugin

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestExtensionRegistry_RegisterCommand(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function noop() end`)
	fn := lr.State().GetGlobal("noop").(*lua.LFunction)

	r.RegisterCommand(Command{
		ID:         "test.hello",
		Title:      "Hello World",
		PluginName: "test",
		Callback:   fn,
		Runtime:    lr,
	})

	cmds := r.Commands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].ID != "test.hello" {
		t.Errorf("expected id 'test.hello', got %q", cmds[0].ID)
	}
}

func TestExtensionRegistry_RegisterFileHandler(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function handle_ipynb(path, content) end`)
	fn := lr.State().GetGlobal("handle_ipynb").(*lua.LFunction)

	r.RegisterFileHandler(FileHandler{
		Extension:  ".ipynb",
		PluginName: "notebook",
		Callback:   fn,
		Runtime:    lr,
	})

	h, ok := r.FileHandler(".ipynb")
	if !ok {
		t.Fatal("expected file handler for .ipynb")
	}
	if h.PluginName != "notebook" {
		t.Errorf("expected plugin 'notebook', got %q", h.PluginName)
	}

	_, ok = r.FileHandler(".xyz")
	if ok {
		t.Error("expected no handler for .xyz")
	}
}

func TestExtensionRegistry_RegisterPanel(t *testing.T) {
	r := NewExtensionRegistry()

	r.RegisterPanel(PanelInfo{
		Name:       "preview",
		Position:   "bottom",
		PluginName: "previewer",
	})

	p, ok := r.Panel("preview")
	if !ok {
		t.Fatal("expected panel 'preview'")
	}
	if p.Position != "bottom" {
		t.Errorf("expected position 'bottom', got %q", p.Position)
	}
}

func TestExtensionRegistry_UnregisterPlugin(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function noop() end`)
	fn := lr.State().GetGlobal("noop").(*lua.LFunction)

	r.RegisterCommand(Command{ID: "a.cmd", Title: "A", PluginName: "plugin-a", Callback: fn, Runtime: lr})
	r.RegisterCommand(Command{ID: "b.cmd", Title: "B", PluginName: "plugin-b", Callback: fn, Runtime: lr})
	r.RegisterMenuEntry(MenuEntry{MenuName: "Tools", Label: "A Tool", PluginName: "plugin-a", Callback: fn, Runtime: lr})
	r.RegisterFileHandler(FileHandler{Extension: ".abc", PluginName: "plugin-a", Callback: fn, Runtime: lr})
	r.RegisterPanel(PanelInfo{Name: "a-panel", Position: "bottom", PluginName: "plugin-a"})
	r.RegisterShortcut(ShortcutEntry{KeyDesc: "Ctrl+J", PluginName: "plugin-a", Callback: fn, Runtime: lr})

	r.UnregisterPlugin("plugin-a")

	cmds := r.Commands()
	if len(cmds) != 1 || cmds[0].PluginName != "plugin-b" {
		t.Errorf("expected only plugin-b command after unregister, got %v", cmds)
	}
	if len(r.MenuEntries()) != 0 {
		t.Error("expected 0 menu entries after unregister")
	}
	if _, ok := r.FileHandler(".abc"); ok {
		t.Error("expected file handler removed after unregister")
	}
	if _, ok := r.Panel("a-panel"); ok {
		t.Error("expected panel removed after unregister")
	}
	if len(r.Shortcuts()) != 0 {
		t.Error("expected 0 shortcuts after unregister")
	}
}

func TestExtensionRegistry_PanelSelectCallback(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function on_select(idx, text) end`)
	fn := lr.State().GetGlobal("on_select").(*lua.LFunction)

	r.RegisterPanel(PanelInfo{
		Name:       "selectable",
		Position:   "bottom",
		PluginName: "test",
	})

	r.SetPanelSelectCallback("selectable", fn, lr)

	p, ok := r.Panel("selectable")
	if !ok {
		t.Fatal("expected panel 'selectable'")
	}
	if p.SelectCallback == nil {
		t.Error("expected select callback to be set")
	}
	if p.SelectRuntime != lr {
		t.Error("expected select runtime to match")
	}
}

func TestExtensionRegistry_PanelKeyCallback(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function on_key(key) return true end`)
	fn := lr.State().GetGlobal("on_key").(*lua.LFunction)

	r.RegisterPanel(PanelInfo{
		Name:       "keyed",
		Position:   "bottom",
		PluginName: "test",
	})

	r.SetPanelKeyCallback("keyed", fn, lr)

	p, ok := r.Panel("keyed")
	if !ok {
		t.Fatal("expected panel 'keyed'")
	}
	if p.KeyCallback == nil {
		t.Error("expected key callback to be set")
	}
}

func TestExtensionRegistry_PanelSelectedRow(t *testing.T) {
	r := NewExtensionRegistry()
	r.RegisterPanel(PanelInfo{
		Name:       "nav",
		Position:   "bottom",
		PluginName: "test",
	})

	r.SetPanelSelectedRow("nav", 5)

	p, _ := r.Panel("nav")
	if p.SelectedRow != 5 {
		t.Errorf("expected selected row 5, got %d", p.SelectedRow)
	}
}

func TestExtensionRegistry_PanelCells(t *testing.T) {
	r := NewExtensionRegistry()
	r.RegisterPanel(PanelInfo{
		Name:       "notebook",
		Position:   "bottom",
		PluginName: "test",
	})

	r.AddPanelCell("notebook", PanelCell{Type: "code", Language: "Go", Content: "package main"})
	r.AddPanelCell("notebook", PanelCell{Type: "markdown", Content: "# Hello"})
	r.AddPanelCell("notebook", PanelCell{Type: "output", Content: "success"})

	cells := r.PanelCells("notebook")
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(cells))
	}
	if cells[0].Type != "code" || cells[0].Language != "Go" {
		t.Errorf("unexpected first cell: %+v", cells[0])
	}
	if cells[1].Type != "markdown" {
		t.Errorf("unexpected second cell: %+v", cells[1])
	}
	if cells[2].Type != "output" || cells[2].Content != "success" {
		t.Errorf("unexpected third cell: %+v", cells[2])
	}

	r.ClearPanelCells("notebook")
	cells = r.PanelCells("notebook")
	if len(cells) != 0 {
		t.Errorf("expected 0 cells after clear, got %d", len(cells))
	}
}

func TestExtensionRegistry_PanelCells_NonexistentPanel(t *testing.T) {
	r := NewExtensionRegistry()
	cells := r.PanelCells("nonexistent")
	if cells != nil {
		t.Errorf("expected nil for nonexistent panel, got %v", cells)
	}
}

func TestExtensionRegistry_SetCallbackOnNonexistentPanel(t *testing.T) {
	r := NewExtensionRegistry()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function noop() end`)
	fn := lr.State().GetGlobal("noop").(*lua.LFunction)

	// Should not panic on nonexistent panel
	r.SetPanelSelectCallback("nonexistent", fn, lr)
	r.SetPanelKeyCallback("nonexistent", fn, lr)
	r.SetPanelSelectedRow("nonexistent", 0)
	r.AddPanelCell("nonexistent", PanelCell{Type: "raw", Content: "test"})
	r.ClearPanelCells("nonexistent")
}
