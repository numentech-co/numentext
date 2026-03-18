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

	// Interactivity callbacks (Epic 45)
	SelectCallback *lua.LFunction // called with (row_index, row_text) on Enter/click
	SelectRuntime  *LuaRuntime
	SelectedRow    int // currently highlighted row (0-based)

	// Key forwarding callback (Epic 48.4)
	KeyCallback *lua.LFunction // called with (key_name_string); returns true to consume
	KeyRuntime  *LuaRuntime

	// Cell-based layout (Epic 48.3)
	Cells []PanelCell
}

// PanelCell represents one cell in a cell-based panel layout.
type PanelCell struct {
	Type     string // "code", "markdown", "output", "raw"
	Language string // language for code cells
	Content  string
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

// SetPanelSelectCallback sets the select callback for a panel.
func (r *ExtensionRegistry) SetPanelSelectCallback(name string, fn *lua.LFunction, runtime *LuaRuntime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.panels[name]; ok {
		p.SelectCallback = fn
		p.SelectRuntime = runtime
		r.panels[name] = p
	}
}

// SetPanelKeyCallback sets the key event callback for a panel.
func (r *ExtensionRegistry) SetPanelKeyCallback(name string, fn *lua.LFunction, runtime *LuaRuntime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.panels[name]; ok {
		p.KeyCallback = fn
		p.KeyRuntime = runtime
		r.panels[name] = p
	}
}

// SetPanelSelectedRow updates the selected row index for a panel.
func (r *ExtensionRegistry) SetPanelSelectedRow(name string, row int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.panels[name]; ok {
		p.SelectedRow = row
		r.panels[name] = p
	}
}

// AddPanelCell appends a cell to a panel's cell list.
func (r *ExtensionRegistry) AddPanelCell(name string, cell PanelCell) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.panels[name]; ok {
		p.Cells = append(p.Cells, cell)
		r.panels[name] = p
	}
}

// ClearPanelCells removes all cells from a panel.
func (r *ExtensionRegistry) ClearPanelCells(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.panels[name]; ok {
		p.Cells = nil
		r.panels[name] = p
	}
}

// PanelCells returns a copy of the cells for a panel.
func (r *ExtensionRegistry) PanelCells(name string) []PanelCell {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.panels[name]
	if !ok {
		return nil
	}
	cells := make([]PanelCell, len(p.Cells))
	copy(cells, p.Cells)
	return cells
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
