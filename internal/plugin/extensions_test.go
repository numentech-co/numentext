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
