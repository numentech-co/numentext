package plugin

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestEventDispatcher_SubscribeAndDispatch(t *testing.T) {
	ed := NewEventDispatcher()
	lr := NewLuaRuntime()
	defer lr.Close()

	// Create a Lua function that sets a global
	err := lr.DoString(`
		received_path = ""
		function on_file_open(path)
			received_path = path
		end
	`)
	if err != nil {
		t.Fatal(err)
	}

	fn := lr.State().GetGlobal("on_file_open").(*lua.LFunction)
	ed.Subscribe("file_open", "test-plugin", fn, lr)

	if ed.ListenerCount("file_open") != 1 {
		t.Errorf("expected 1 listener, got %d", ed.ListenerCount("file_open"))
	}

	ed.Dispatch("file_open", lua.LString("/tmp/test.go"))

	val := lr.State().GetGlobal("received_path")
	if val.String() != "/tmp/test.go" {
		t.Errorf("expected '/tmp/test.go', got %q", val.String())
	}
}

func TestEventDispatcher_MultipleListeners(t *testing.T) {
	ed := NewEventDispatcher()

	lr1 := NewLuaRuntime()
	defer lr1.Close()
	lr2 := NewLuaRuntime()
	defer lr2.Close()

	lr1.DoString(`count1 = 0; function handler1() count1 = count1 + 1 end`)
	lr2.DoString(`count2 = 0; function handler2() count2 = count2 + 1 end`)

	fn1 := lr1.State().GetGlobal("handler1").(*lua.LFunction)
	fn2 := lr2.State().GetGlobal("handler2").(*lua.LFunction)

	ed.Subscribe("file_save", "plugin-a", fn1, lr1)
	ed.Subscribe("file_save", "plugin-b", fn2, lr2)

	ed.Dispatch("file_save")

	v1 := lr1.State().GetGlobal("count1")
	v2 := lr2.State().GetGlobal("count2")
	if v1.String() != "1" {
		t.Errorf("expected count1=1, got %s", v1.String())
	}
	if v2.String() != "1" {
		t.Errorf("expected count2=1, got %s", v2.String())
	}
}

func TestEventDispatcher_Unsubscribe(t *testing.T) {
	ed := NewEventDispatcher()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`count = 0; function handler() count = count + 1 end`)
	fn := lr.State().GetGlobal("handler").(*lua.LFunction)

	ed.Subscribe("file_open", "my-plugin", fn, lr)
	ed.Unsubscribe("my-plugin")

	if ed.ListenerCount("file_open") != 0 {
		t.Errorf("expected 0 listeners after unsubscribe, got %d", ed.ListenerCount("file_open"))
	}

	ed.Dispatch("file_open")
	v := lr.State().GetGlobal("count")
	if v.String() != "0" {
		t.Errorf("expected count=0 (no dispatch), got %s", v.String())
	}
}

func TestEventDispatcher_ErrorCallback(t *testing.T) {
	ed := NewEventDispatcher()
	lr := NewLuaRuntime()
	defer lr.Close()

	lr.DoString(`function bad_handler() error("oops") end`)
	fn := lr.State().GetGlobal("bad_handler").(*lua.LFunction)

	var gotErr bool
	ed.SetOnError(func(pluginName, event string, err error) {
		gotErr = true
		if pluginName != "broken-plugin" {
			t.Errorf("expected plugin 'broken-plugin', got %q", pluginName)
		}
	})

	ed.Subscribe("file_open", "broken-plugin", fn, lr)
	ed.Dispatch("file_open")

	if !gotErr {
		t.Error("expected error callback to be called")
	}
}
