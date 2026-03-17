package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// DefaultTimeout is the maximum duration for a single Lua call.
const DefaultTimeout = 5 * time.Second

// LuaRuntime wraps a sandboxed gopher-lua VM.
type LuaRuntime struct {
	state   *lua.LState
	mu      sync.Mutex
	timeout time.Duration
}

// dangerousModules are removed from the Lua global scope for sandboxing.
var dangerousModules = []string{"os", "io", "debug", "loadfile", "dofile"}

// NewLuaRuntime creates a sandboxed Lua VM with dangerous modules removed.
func NewLuaRuntime() *LuaRuntime {
	opts := lua.Options{
		SkipOpenLibs: false,
	}
	L := lua.NewState(opts)

	// Remove dangerous globals/modules
	for _, name := range dangerousModules {
		L.SetGlobal(name, lua.LNil)
	}
	// Remove package.loadlib for extra safety
	pkg := L.GetGlobal("package")
	if tbl, ok := pkg.(*lua.LTable); ok {
		tbl.RawSetString("loadlib", lua.LNil)
	}

	return &LuaRuntime{
		state:   L,
		timeout: DefaultTimeout,
	}
}

// Close shuts down the Lua VM.
func (lr *LuaRuntime) Close() {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.state.Close()
}

// State returns the underlying lua.LState for registering functions.
func (lr *LuaRuntime) State() *lua.LState {
	return lr.state
}

// SetTimeout sets the per-call timeout duration.
func (lr *LuaRuntime) SetTimeout(d time.Duration) {
	lr.timeout = d
}

// DoString executes Lua code with timeout protection.
func (lr *LuaRuntime) DoString(code string) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), lr.timeout)
	defer cancel()
	lr.state.SetContext(ctx)
	defer lr.state.RemoveContext()

	err := lr.state.DoString(code)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("lua execution timed out after %v", lr.timeout)
		}
		return err
	}
	return nil
}

// DoFile executes a Lua file with timeout protection.
func (lr *LuaRuntime) DoFile(path string) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), lr.timeout)
	defer cancel()
	lr.state.SetContext(ctx)
	defer lr.state.RemoveContext()

	err := lr.state.DoFile(path)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("lua execution timed out after %v", lr.timeout)
		}
		return err
	}
	return nil
}

// CallFunction calls a Lua function by reference with timeout protection.
func (lr *LuaRuntime) CallFunction(fn *lua.LFunction, args ...lua.LValue) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), lr.timeout)
	defer cancel()
	lr.state.SetContext(ctx)
	defer lr.state.RemoveContext()

	err := lr.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, args...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("lua execution timed out after %v", lr.timeout)
		}
		return err
	}
	return nil
}

// CallFunctionWithReturn calls a Lua function and returns the first result.
func (lr *LuaRuntime) CallFunctionWithReturn(fn *lua.LFunction, nret int, args ...lua.LValue) ([]lua.LValue, error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), lr.timeout)
	defer cancel()
	lr.state.SetContext(ctx)
	defer lr.state.RemoveContext()

	err := lr.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    nret,
		Protect: true,
	}, args...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("lua execution timed out after %v", lr.timeout)
		}
		return nil, err
	}

	results := make([]lua.LValue, nret)
	for i := nret - 1; i >= 0; i-- {
		results[i] = lr.state.Get(-1)
		lr.state.Pop(1)
	}
	return results, nil
}

// SetGlobal sets a global value in the Lua state.
func (lr *LuaRuntime) SetGlobal(name string, val lua.LValue) {
	lr.state.SetGlobal(name, val)
}
