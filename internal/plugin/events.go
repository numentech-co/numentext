package plugin

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Event names for the plugin event system.
const (
	EventFileOpen    = "file_open"
	EventFileSave    = "file_save"
	EventFileClose   = "file_close"
	EventCursorMove  = "cursor_move"
	EventThemeChange = "theme_change"
)

// eventListener holds a plugin name and callback for one event subscription.
type eventListener struct {
	pluginName string
	callback   *lua.LFunction
	runtime    *LuaRuntime
}

// EventDispatcher manages event subscriptions from plugins.
type EventDispatcher struct {
	mu        sync.RWMutex
	listeners map[string][]eventListener
	onError   func(pluginName, event string, err error)
}

// NewEventDispatcher creates a new event dispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		listeners: make(map[string][]eventListener),
	}
}

// SetOnError sets a callback for when a plugin event handler returns an error.
func (ed *EventDispatcher) SetOnError(fn func(pluginName, event string, err error)) {
	ed.onError = fn
}

// Subscribe registers a listener for the given event.
func (ed *EventDispatcher) Subscribe(event, pluginName string, callback *lua.LFunction, runtime *LuaRuntime) {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.listeners[event] = append(ed.listeners[event], eventListener{
		pluginName: pluginName,
		callback:   callback,
		runtime:    runtime,
	})
}

// Unsubscribe removes all listeners for a given plugin.
func (ed *EventDispatcher) Unsubscribe(pluginName string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	for event, list := range ed.listeners {
		filtered := make([]eventListener, 0, len(list))
		for _, l := range list {
			if l.pluginName != pluginName {
				filtered = append(filtered, l)
			}
		}
		ed.listeners[event] = filtered
	}
}

// Dispatch fires an event to all subscribers with the given Lua arguments.
func (ed *EventDispatcher) Dispatch(event string, args ...lua.LValue) {
	ed.mu.RLock()
	list := make([]eventListener, len(ed.listeners[event]))
	copy(list, ed.listeners[event])
	ed.mu.RUnlock()

	for _, l := range list {
		err := l.runtime.CallFunction(l.callback, args...)
		if err != nil && ed.onError != nil {
			ed.onError(l.pluginName, event, err)
		}
	}
}

// ListenerCount returns the number of listeners for the given event.
func (ed *EventDispatcher) ListenerCount(event string) int {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return len(ed.listeners[event])
}
