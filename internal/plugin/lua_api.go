package plugin

import (
	"os"

	lua "github.com/yuin/gopher-lua"
)

// registerCtx creates the global `ctx` table in the Lua state.
func registerCtx(L *lua.LState, ctx *ScriptContext) {
	tbl := L.NewTable()

	// Read-only fields
	tbl.RawSetString("browser", lua.LString(ctx.Browser))
	tbl.RawSetString("version", lua.LString(ctx.Version))
	tbl.RawSetString("profile", lua.LString(ctx.Profile))
	tbl.RawSetString("profile_dir", lua.LString(ctx.ProfileDir))

	// ctx.config(key) -> string
	L.SetField(tbl, "config", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := ""
		if ctx.Config != nil {
			val = ctx.Config(key)
		}
		L.Push(lua.LString(val))
		return 1
	}))

	// ctx.add_arg(arg)
	L.SetField(tbl, "add_arg", L.NewFunction(func(L *lua.LState) int {
		arg := L.CheckString(1)
		if ctx.AddArg != nil {
			ctx.AddArg(arg)
		}
		return 0
	}))

	// ctx.set_env(key, value)
	L.SetField(tbl, "set_env", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.CheckString(2)
		if ctx.SetEnv != nil {
			ctx.SetEnv(key, value)
		}
		return 0
	}))

	// ctx.write_file(path, content) -> error?
	L.SetField(tbl, "write_file", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		content := L.CheckString(2)
		if ctx.WriteFile != nil {
			if err := ctx.WriteFile(path, content); err != nil {
				L.Push(lua.LString(err.Error()))
				return 1
			}
		}
		L.Push(lua.LNil)
		return 1
	}))

	// ctx.read_file(path) -> content, error?
	L.SetField(tbl, "read_file", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		if ctx.ReadFile != nil {
			content, err := ctx.ReadFile(path)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LString(content))
			L.Push(lua.LNil)
			return 2
		}
		L.Push(lua.LNil)
		L.Push(lua.LString("read_file not available"))
		return 2
	}))

	// ctx.log(message)
	L.SetField(tbl, "log", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		os.Stderr.WriteString("[plugin] " + msg + "\n")
		return 0
	}))

	L.SetGlobal("ctx", tbl)
}
