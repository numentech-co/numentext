package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

// PluginHost is the interface that the application must implement
// to provide services to plugins.
type PluginHost interface {
	OpenFile(path string) error
	ActiveFilePath() string
	ActiveFileContent() string
	CursorPosition() (int, int)
	SetCursor(row, col int)
	InsertText(text string)
	SelectedText() string
	ReplaceSelection(text string)
	SetContent(text string)
	SetStatusMessage(msg string)
	AppendOutput(text string)
	AddMenuItem(menuName, label string, action func())
	AddCommand(id, title string, action func())
	ShowPanel(name string)
	HidePanel(name string)
	SetPanelContent(name, text string)
	AppendPanelContent(name, text string)
	Exec(command string, args []string, workDir string) (string, error)
	SetGutterMarkers(filePath string, markers map[int]string)
}

// registerHostAPI registers the "numen" global table on the Lua state
// with all host API functions bound to the given host and plugin context.
func registerHostAPI(lr *LuaRuntime, host PluginHost, pluginName string, registry *ExtensionRegistry, events *EventDispatcher) {
	L := lr.State()
	numen := L.NewTable()

	// numen.open_file(path)
	numen.RawSetString("open_file", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		err := host.OpenFile(path)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	// numen.active_file() -> path
	numen.RawSetString("active_file", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(host.ActiveFilePath()))
		return 1
	}))

	// numen.active_content() -> content
	numen.RawSetString("active_content", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(host.ActiveFileContent()))
		return 1
	}))

	// numen.status_message(text)
	numen.RawSetString("status_message", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		host.SetStatusMessage(text)
		return 0
	}))

	// numen.output_append(text)
	numen.RawSetString("output_append", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		host.AppendOutput(text)
		return 0
	}))

	// numen.cursor_position() -> row, col
	numen.RawSetString("cursor_position", L.NewFunction(func(L *lua.LState) int {
		row, col := host.CursorPosition()
		L.Push(lua.LNumber(row))
		L.Push(lua.LNumber(col))
		return 2
	}))

	// numen.set_cursor(row, col)
	numen.RawSetString("set_cursor", L.NewFunction(func(L *lua.LState) int {
		row := L.CheckInt(1)
		col := L.CheckInt(2)
		host.SetCursor(row, col)
		return 0
	}))

	// numen.insert_text(text)
	numen.RawSetString("insert_text", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		host.InsertText(text)
		return 0
	}))

	// numen.selected_text() -> text
	numen.RawSetString("selected_text", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(host.SelectedText()))
		return 1
	}))

	// numen.replace_selection(text) -- replaces selected text, or inserts at cursor if no selection
	numen.RawSetString("replace_selection", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		host.ReplaceSelection(text)
		return 0
	}))

	// numen.set_content(text) -- replaces the entire buffer content
	numen.RawSetString("set_content", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		host.SetContent(text)
		return 0
	}))

	// numen.register_command(id, title, callback)
	numen.RawSetString("register_command", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		title := L.CheckString(2)
		fn := L.CheckFunction(3)
		registry.RegisterCommand(Command{
			ID:         id,
			Title:      title,
			PluginName: pluginName,
			Callback:   fn,
			Runtime:    lr,
		})
		// Also register with host so it appears in the command palette
		host.AddCommand(id, title, func() {
			_ = lr.CallFunction(fn)
		})
		return 0
	}))

	// numen.register_menu_item(menu_name, label, callback)
	numen.RawSetString("register_menu_item", L.NewFunction(func(L *lua.LState) int {
		menuName := L.CheckString(1)
		label := L.CheckString(2)
		fn := L.CheckFunction(3)
		registry.RegisterMenuEntry(MenuEntry{
			MenuName:   menuName,
			Label:      label,
			PluginName: pluginName,
			Callback:   fn,
			Runtime:    lr,
		})
		host.AddMenuItem(menuName, label, func() {
			_ = lr.CallFunction(fn)
		})
		return 0
	}))

	// numen.register_shortcut(key_desc, callback)
	numen.RawSetString("register_shortcut", L.NewFunction(func(L *lua.LState) int {
		keyDesc := L.CheckString(1)
		fn := L.CheckFunction(2)
		registry.RegisterShortcut(ShortcutEntry{
			KeyDesc:    keyDesc,
			PluginName: pluginName,
			Callback:   fn,
			Runtime:    lr,
		})
		return 0
	}))

	// numen.register_file_handler(extension, open_callback)
	numen.RawSetString("register_file_handler", L.NewFunction(func(L *lua.LState) int {
		ext := L.CheckString(1)
		fn := L.CheckFunction(2)
		registry.RegisterFileHandler(FileHandler{
			Extension:  ext,
			PluginName: pluginName,
			Callback:   fn,
			Runtime:    lr,
		})
		return 0
	}))

	// numen.register_panel(name, position)
	numen.RawSetString("register_panel", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		position := L.CheckString(2)
		registry.RegisterPanel(PanelInfo{
			Name:       name,
			Position:   position,
			PluginName: pluginName,
		})
		return 0
	}))

	// numen.panel_set_content(name, text)
	numen.RawSetString("panel_set_content", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		text := L.CheckString(2)
		host.SetPanelContent(name, text)
		return 0
	}))

	// numen.panel_append(name, text)
	numen.RawSetString("panel_append", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		text := L.CheckString(2)
		host.AppendPanelContent(name, text)
		return 0
	}))

	// numen.panel_show(name)
	numen.RawSetString("panel_show", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		host.ShowPanel(name)
		return 0
	}))

	// numen.panel_hide(name)
	numen.RawSetString("panel_hide", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		host.HidePanel(name)
		return 0
	}))

	// numen.on(event, callback)
	numen.RawSetString("on", L.NewFunction(func(L *lua.LState) int {
		event := L.CheckString(1)
		fn := L.CheckFunction(2)
		events.Subscribe(event, pluginName, fn, lr)
		return 0
	}))

	// numen.exec(command, args_table, work_dir) -> output, err
	numen.RawSetString("exec", L.NewFunction(func(L *lua.LState) int {
		command := L.CheckString(1)
		argsTable := L.OptTable(2, L.NewTable())
		workDir := L.OptString(3, "")

		var args []string
		argsTable.ForEach(func(_, v lua.LValue) {
			args = append(args, v.String())
		})

		output, err := host.Exec(command, args, workDir)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(output))
		return 1
	}))

	// numen.set_gutter_markers(file_path, markers_table)
	// markers_table is {[line_number] = "added"|"modified"|"deleted", ...}
	numen.RawSetString("set_gutter_markers", L.NewFunction(func(L *lua.LState) int {
		filePath := L.CheckString(1)
		markersTable := L.CheckTable(2)

		markers := make(map[int]string)
		markersTable.ForEach(func(k, v lua.LValue) {
			if kn, ok := k.(lua.LNumber); ok {
				markers[int(kn)] = v.String()
			}
		})

		host.SetGutterMarkers(filePath, markers)
		return 0
	}))

	L.SetGlobal("numen", numen)
}
