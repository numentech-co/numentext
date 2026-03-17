package plugin

import (
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestNewLuaRuntime_SandboxRemovesDangerousModules(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	for _, mod := range dangerousModules {
		val := lr.State().GetGlobal(mod)
		if val != lua.LNil {
			t.Errorf("dangerous module %q should be nil, got %v", mod, val.Type())
		}
	}
}

func TestLuaRuntime_DoString(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	err := lr.DoString(`x = 1 + 2`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	val := lr.State().GetGlobal("x")
	if val.String() != "3" {
		t.Errorf("expected x=3, got %s", val.String())
	}
}

func TestLuaRuntime_DoStringError(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	err := lr.DoString(`error("boom")`)
	if err == nil {
		t.Fatal("expected error from DoString")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error containing 'boom', got: %v", err)
	}
}

func TestLuaRuntime_Timeout(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()
	lr.SetTimeout(100 * time.Millisecond)

	err := lr.DoString(`while true do end`)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message, got: %v", err)
	}
}

func TestLuaRuntime_CallFunction(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	err := lr.DoString(`
		function add(a, b)
			result = a + b
		end
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	fn := lr.State().GetGlobal("add").(*lua.LFunction)
	err = lr.CallFunction(fn, lua.LNumber(3), lua.LNumber(4))
	if err != nil {
		t.Fatalf("CallFunction failed: %v", err)
	}

	val := lr.State().GetGlobal("result")
	if val.String() != "7" {
		t.Errorf("expected result=7, got %s", val.String())
	}
}

func TestLuaRuntime_CallFunctionWithReturn(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	err := lr.DoString(`
		function multiply(a, b)
			return a * b
		end
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	fn := lr.State().GetGlobal("multiply").(*lua.LFunction)
	results, err := lr.CallFunctionWithReturn(fn, 1, lua.LNumber(5), lua.LNumber(6))
	if err != nil {
		t.Fatalf("CallFunctionWithReturn failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].String() != "30" {
		t.Errorf("expected 30, got %s", results[0].String())
	}
}

func TestLuaRuntime_SandboxBlocksOsModule(t *testing.T) {
	lr := NewLuaRuntime()
	defer lr.Close()

	err := lr.DoString(`os.execute("echo pwned")`)
	if err == nil {
		t.Fatal("expected error when accessing sandboxed os module")
	}
}
