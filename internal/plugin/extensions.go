package plugin

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Command is a plugin-registered command for the command palette.
type Command struct {
	ID         string
	Title      string
	PluginName string
	Callback   *lua.LFunction
	Runtime    *LuaRuntime
}

// MenuEntry is a plugin-registered menu item.
type MenuEntry struct {
	MenuName   string
	Label      string
	PluginName string
	Callback   *lua.LFunction
	Runtime    *LuaRuntime
}

// ShortcutEntry is a plugin-registered keyboard shortcut.
type ShortcutEntry struct {
	KeyDesc    string // e.g. "Ctrl+Shift+J"
	PluginName string
	Callback   *lua.LFunction
	Runtime    *LuaRuntime
}

// FileHandler is a plugin-registered handler for a file extension.
type FileHandler struct {
	Extension  string // e.g. ".ipynb"
	PluginName string
	Callback   *lua.LFunction
	Runtime    *LuaRuntime
}

// PanelInfo describes a plugin-registered panel.
type PanelInfo struct {
	Name       string
	Position   string // "bottom" or "right"
	PluginName string
}

// ExtensionRegistry holds all plugin-registered commands, menus, shortcuts, etc.
type ExtensionRegistry struct {
	mu           sync.RWMutex
	commands     []Command
	menuEntries  []MenuEntry
	shortcuts    []ShortcutEntry
	fileHandlers map[string]FileHandler // extension -> handler
	panels       map[string]PanelInfo   // name -> panel info
}

// NewExtensionRegistry creates an empty extension registry.
func NewExtensionRegistry() *ExtensionRegistry {
	return &ExtensionRegistry{
		fileHandlers: make(map[string]FileHandler),
		panels:       make(map[string]PanelInfo),
	}
}

// RegisterCommand adds a command to the registry.
func (r *ExtensionRegistry) RegisterCommand(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, cmd)
}

// RegisterMenuEntry adds a menu item to the registry.
func (r *ExtensionRegistry) RegisterMenuEntry(entry MenuEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.menuEntries = append(r.menuEntries, entry)
}

// RegisterShortcut adds a keyboard shortcut to the registry.
func (r *ExtensionRegistry) RegisterShortcut(entry ShortcutEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shortcuts = append(r.shortcuts, entry)
}

// RegisterFileHandler registers a plugin handler for a file extension.
func (r *ExtensionRegistry) RegisterFileHandler(handler FileHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fileHandlers[handler.Extension] = handler
}

// RegisterPanel registers a plugin panel.
func (r *ExtensionRegistry) RegisterPanel(info PanelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.panels[info.Name] = info
}

// Commands returns all registered commands.
func (r *ExtensionRegistry) Commands() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Command, len(r.commands))
	copy(result, r.commands)
	return result
}

// MenuEntries returns all registered menu items.
func (r *ExtensionRegistry) MenuEntries() []MenuEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]MenuEntry, len(r.menuEntries))
	copy(result, r.menuEntries)
	return result
}

// Shortcuts returns all registered keyboard shortcuts.
func (r *ExtensionRegistry) Shortcuts() []ShortcutEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ShortcutEntry, len(r.shortcuts))
	copy(result, r.shortcuts)
	return result
}

// FileHandler returns the handler for a given extension, if any.
func (r *ExtensionRegistry) FileHandler(ext string) (FileHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.fileHandlers[ext]
	return h, ok
}

// Panels returns all registered panels.
func (r *ExtensionRegistry) Panels() map[string]PanelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]PanelInfo, len(r.panels))
	for k, v := range r.panels {
		result[k] = v
	}
	return result
}

// Panel returns a specific panel by name.
func (r *ExtensionRegistry) Panel(name string) (PanelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.panels[name]
	return p, ok
}

// UnregisterPlugin removes all registrations for a plugin.
func (r *ExtensionRegistry) UnregisterPlugin(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Filter commands
	filtered := make([]Command, 0, len(r.commands))
	for _, c := range r.commands {
		if c.PluginName != pluginName {
			filtered = append(filtered, c)
		}
	}
	r.commands = filtered

	// Filter menu entries
	filteredMenus := make([]MenuEntry, 0, len(r.menuEntries))
	for _, m := range r.menuEntries {
		if m.PluginName != pluginName {
			filteredMenus = append(filteredMenus, m)
		}
	}
	r.menuEntries = filteredMenus

	// Filter shortcuts
	filteredShortcuts := make([]ShortcutEntry, 0, len(r.shortcuts))
	for _, s := range r.shortcuts {
		if s.PluginName != pluginName {
			filteredShortcuts = append(filteredShortcuts, s)
		}
	}
	r.shortcuts = filteredShortcuts

	// Remove file handlers
	for ext, h := range r.fileHandlers {
		if h.PluginName == pluginName {
			delete(r.fileHandlers, ext)
		}
	}

	// Remove panels
	for name, p := range r.panels {
		if p.PluginName == pluginName {
			delete(r.panels, name)
		}
	}
}
