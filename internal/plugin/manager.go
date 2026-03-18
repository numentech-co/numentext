package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// PluginEntry holds a loaded plugin's state.
type PluginEntry struct {
	Manifest *Manifest
	Dir      string
	Runtime  *LuaRuntime
	Enabled  bool
}

// Manager handles plugin discovery, loading, and lifecycle.
type Manager struct {
	mu       sync.RWMutex
	plugins  map[string]*PluginEntry
	host     PluginHost
	events   *EventDispatcher
	registry *ExtensionRegistry
	pluginDir string
	disabled  map[string]bool // plugin names disabled in config
	onLog     func(msg string)
}

// NewManager creates a new plugin manager.
func NewManager(host PluginHost) *Manager {
	home, _ := os.UserHomeDir()
	return &Manager{
		plugins:   make(map[string]*PluginEntry),
		host:      host,
		events:    NewEventDispatcher(),
		registry:  NewExtensionRegistry(),
		pluginDir: filepath.Join(home, ".numentext", "plugins"),
		disabled:  make(map[string]bool),
	}
}

// SetPluginDir overrides the plugin directory (for testing).
func (m *Manager) SetPluginDir(dir string) {
	m.pluginDir = dir
}

// SetDisabledPlugins sets the list of disabled plugin names.
func (m *Manager) SetDisabledPlugins(names []string) {
	m.disabled = make(map[string]bool, len(names))
	for _, n := range names {
		m.disabled[n] = true
	}
}

// SetOnLog sets a callback for log messages during plugin loading.
func (m *Manager) SetOnLog(fn func(msg string)) {
	m.onLog = fn
}

func (m *Manager) log(msg string) {
	if m.onLog != nil {
		m.onLog(msg)
	}
}

// Events returns the event dispatcher for firing events from the app.
func (m *Manager) Events() *EventDispatcher {
	return m.events
}

// Registry returns the extension registry.
func (m *Manager) Registry() *ExtensionRegistry {
	return m.registry
}

// PluginDir returns the plugin directory path.
func (m *Manager) PluginDir() string {
	return m.pluginDir
}

// Plugins returns all loaded plugin entries.
func (m *Manager) Plugins() map[string]*PluginEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*PluginEntry, len(m.plugins))
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}

// LoadAll scans the plugin directory and loads all enabled plugins.
func (m *Manager) LoadAll() error {
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		// No plugins directory -- not an error
		return nil
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("reading plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(m.pluginDir, entry.Name())
		if err := m.LoadPlugin(dir); err != nil {
			m.log(fmt.Sprintf("Plugin %s: %v", entry.Name(), err))
		}
	}
	return nil
}

// LoadPlugin loads a single plugin from the given directory.
func (m *Manager) LoadPlugin(dir string) error {
	manifest, err := LoadManifest(dir)
	if err != nil {
		return err
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	if m.disabled[manifest.Name] {
		m.log(fmt.Sprintf("Plugin %s is disabled, skipping", manifest.Name))
		m.mu.Lock()
		m.plugins[manifest.Name] = &PluginEntry{
			Manifest: manifest,
			Dir:      dir,
			Enabled:  false,
		}
		m.mu.Unlock()
		return nil
	}

	initPath := filepath.Join(dir, "init.lua")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		return fmt.Errorf("init.lua not found in %s", dir)
	}

	lr := NewLuaRuntime()
	// Set package.path so require() finds modules in the plugin's directory
	lr.SetPackagePath(dir)
	registerHostAPI(lr, m.host, manifest.Name, m.registry, m.events)

	if err := lr.DoFile(initPath); err != nil {
		lr.Close()
		return fmt.Errorf("loading init.lua: %w", err)
	}

	m.mu.Lock()
	m.plugins[manifest.Name] = &PluginEntry{
		Manifest: manifest,
		Dir:      dir,
		Runtime:  lr,
		Enabled:  true,
	}
	m.mu.Unlock()

	m.log(fmt.Sprintf("Loaded plugin: %s v%s", manifest.Name, manifest.Version))
	return nil
}

// UnloadPlugin unloads and cleans up a plugin.
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	entry, ok := m.plugins[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %q not found", name)
	}
	delete(m.plugins, name)
	m.mu.Unlock()

	if entry.Runtime != nil {
		entry.Runtime.Close()
	}
	m.events.Unsubscribe(name)
	m.registry.UnregisterPlugin(name)
	return nil
}

// EnablePlugin enables a previously disabled plugin.
func (m *Manager) EnablePlugin(name string) error {
	m.mu.RLock()
	entry, ok := m.plugins[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	if entry.Enabled {
		return nil
	}

	delete(m.disabled, name)
	// Unload first (removes stale entry), then reload
	_ = m.UnloadPlugin(name)
	return m.LoadPlugin(entry.Dir)
}

// DisablePlugin disables a loaded plugin.
func (m *Manager) DisablePlugin(name string) error {
	m.disabled[name] = true
	m.mu.RLock()
	entry, ok := m.plugins[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	if !entry.Enabled {
		return nil
	}

	if entry.Runtime != nil {
		entry.Runtime.Close()
		entry.Runtime = nil
	}
	m.events.Unsubscribe(name)
	m.registry.UnregisterPlugin(name)

	m.mu.Lock()
	entry.Enabled = false
	m.mu.Unlock()

	return nil
}

// DispatchEvent fires an event to all subscribed plugins.
func (m *Manager) DispatchEvent(event string, args ...lua.LValue) {
	m.events.Dispatch(event, args...)
}

// HandleFileOpen fires the file_open event.
func (m *Manager) HandleFileOpen(path string) {
	m.events.Dispatch(EventFileOpen, lua.LString(path))
}

// HandleFileSave fires the file_save event.
func (m *Manager) HandleFileSave(path string) {
	m.events.Dispatch(EventFileSave, lua.LString(path))
}

// HandleFileClose fires the file_close event.
func (m *Manager) HandleFileClose(path string) {
	m.events.Dispatch(EventFileClose, lua.LString(path))
}

// HandleCursorMove fires the cursor_move event.
func (m *Manager) HandleCursorMove(row, col int) {
	m.events.Dispatch(EventCursorMove, lua.LNumber(row), lua.LNumber(col))
}

// HandleThemeChange fires the theme_change event.
func (m *Manager) HandleThemeChange(theme string) {
	m.events.Dispatch(EventThemeChange, lua.LString(theme))
}

// HasFileHandler checks if a plugin claims a file extension.
func (m *Manager) HasFileHandler(ext string) bool {
	_, ok := m.registry.FileHandler(ext)
	return ok
}

// InvokeFileHandler calls the plugin handler for a file extension.
func (m *Manager) InvokeFileHandler(ext, path, content string) error {
	handler, ok := m.registry.FileHandler(ext)
	if !ok {
		return fmt.Errorf("no handler for extension %q", ext)
	}
	return handler.Runtime.CallFunction(handler.Callback,
		lua.LString(path), lua.LString(content))
}

// DisabledPlugins returns the set of disabled plugin names.
func (m *Manager) DisabledPlugins() []string {
	result := make([]string, 0, len(m.disabled))
	for name := range m.disabled {
		result = append(result, name)
	}
	return result
}
