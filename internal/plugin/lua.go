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

// NewLuaRuntime creates a new Lua runtime with a restricted sandbox.
// Only safe standard libraries (base, string, table, math) are loaded.
// Dangerous libraries (os, io, package) are NOT available to plugins.
func NewLuaRuntime() *LuaRuntime {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true, // Do NOT auto-load any libraries
	})
	// Whitelist: only load safe libraries
	lua.OpenBase(L)
	lua.OpenString(L)
	lua.OpenTable(L)
	lua.OpenMath(L)
	// NOT loaded: os, io, package, debug, channel, bit32, coroutine, encoding
	return &LuaRuntime{L: L}
}

// Close 在关闭 Lua 状态前调用 on_exit 钩子，然后释放底层资源。
// on_exit 钩子的错误会被忽略，因为此时无法再进行有意义的恢复。
func (r *LuaRuntime) Close() {
	if r == nil || r.L == nil {
		return
	}
	// 关闭前尽力调用 on_exit 钩子（忽略错误）
	_ = r.callHook(string(HookOnExit))
	r.L.Close()
}

// RunScript executes a Lua plugin script with the given context.
// 该方法仅负责加载脚本并调用 pre_run 钩子。
// post_run 和 on_exit 等其他生命周期钩子由插件管理器通过 CallHook
// 在合适的时机调用（on_exit 会在 Close 时自动调用）。
func (r *LuaRuntime) RunScript(scriptPath string, ctx *ScriptContext) error {
	registerCtx(r.L, ctx)

	if err := r.L.DoFile(scriptPath); err != nil {
		return fmt.Errorf("running plugin %s: %w", filepath.Base(scriptPath), err)
	}

	if err := r.callHook(string(HookPreRun)); err != nil {
		return err
	}

	return nil
}

// CallHook 调用指定名称的 Lua 钩子函数（若存在）。
// 这是 callHook 的公开接口，供插件管理器在生命周期节点
// 调用 post_run 和 on_exit 等钩子。若钩子未定义则返回 nil。
func (r *LuaRuntime) CallHook(name string) error {
	return r.callHook(name)
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

// ScriptContext holds data passed to plugins (Lua and IPC).
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
