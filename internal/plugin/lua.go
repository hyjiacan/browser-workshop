package plugin

import (
	"fmt"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// LuaRuntime wraps a gopher-lua state for plugin execution.
type LuaRuntime struct {
	L *lua.LState
}

// NewLuaRuntime creates a new Lua runtime.
func NewLuaRuntime() *LuaRuntime {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: false,
	})
	return &LuaRuntime{L: L}
}

// Close releases the Lua state.
func (r *LuaRuntime) Close() {
	r.L.Close()
}

// RunScript executes a Lua plugin script with the given context.
func (r *LuaRuntime) RunScript(scriptPath string, ctx *ScriptContext) error {
	registerCtx(r.L, ctx)

	if err := r.L.DoFile(scriptPath); err != nil {
		return fmt.Errorf("running plugin %s: %w", filepath.Base(scriptPath), err)
	}

	if err := r.callHook("pre_run"); err != nil {
		return err
	}

	return nil
}

// callHook invokes a named Lua function if it exists.
func (r *LuaRuntime) callHook(name string) error {
	fn := r.L.GetGlobal(name)
	if fn == lua.LNil {
		return nil
	}
	if err := r.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}); err != nil {
		return fmt.Errorf("hook %s: %w", name, err)
	}
	return nil
}

// ScriptContext holds data passed to Lua plugins.
type ScriptContext struct {
	Browser    string
	Version    string
	Profile    string
	ProfileDir string
	Args       []string
	Env        map[string]string
	Config     func(key string) string
	AddArg     func(arg string)
	SetEnv     func(key, value string)
	WriteFile  func(path, content string) error
	ReadFile   func(path string) (string, error)
}