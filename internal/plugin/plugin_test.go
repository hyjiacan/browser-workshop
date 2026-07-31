package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest_Empty(t *testing.T) {
	dir := t.TempDir()
	m, err := LoadManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Plugins) != 0 {
		t.Errorf("expected empty manifest, got %d plugins", len(m.Plugins))
	}
}

func TestLoadManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	m := &Manifest{Version: "1", Plugins: map[string]ManifestEntry{
		"test": {Name: "test", Version: "1.0", Type: "lua"},
	}}
	if err := SaveManifest(m, path); err != nil {
		t.Fatal(err)
	}
	m2, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Plugins["test"].Version != "1.0" {
		t.Error("version mismatch after round-trip")
	}
}

func TestManager_Discover(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "test.lua"), []byte("-- test"), 0o644)
	plugins, err := mgr.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "test" {
		t.Errorf("expected 1 plugin named test, got %+v", plugins)
	}
}

func TestManager_InstallUninstall(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Install
	entry := ManifestEntry{Name: "test-plugin", Version: "1.0", Type: "lua", Path: filepath.Join(dir, "test-plugin.lua")}
	if err := mgr.Install(entry); err != nil {
		t.Fatal(err)
	}
	list := mgr.List()
	if len(list) != 1 || list[0].Name != "test-plugin" {
		t.Errorf("expected test-plugin installed, got %+v", list)
	}

	// Uninstall
	if err := mgr.Uninstall("test-plugin"); err != nil {
		t.Fatal(err)
	}
	list = mgr.List()
	if len(list) != 0 {
		t.Errorf("expected empty after uninstall, got %+v", list)
	}

	// Uninstall non-existent
	err = mgr.Uninstall("nonexistent")
	if err == nil {
		t.Error("expected error uninstalling nonexistent plugin")
	}
}

func TestLuaRuntime_RunScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "test.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	ctx.add_arg("--test-flag")
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	var added []string
	ctx := &ScriptContext{
		Browser: "chrome",
		Version: "120",
		AddArg: func(arg string) {
			added = append(added, arg)
		},
	}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0] != "--test-flag" {
		t.Errorf("expected --test-flag added, got %v", added)
	}
}

func TestLuaRuntime_Config(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "config_test.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	local val = ctx.config("test_key")
	if val ~= "test_value" then
		error("config mismatch: " .. tostring(val))
	end
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{
		Config: func(key string) string {
			if key == "test_key" {
				return "test_value"
			}
			return ""
		},
	}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
}

func TestLuaRuntime_WriteFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	script := filepath.Join(dir, "write_test.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	local err = ctx.write_file("`+strings.ReplaceAll(outFile, `\`, `/`)+`", "hello")
	if err ~= nil then
		error("write failed: " .. err)
	end
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{
		WriteFile: func(path, content string) error {
			return os.WriteFile(path, []byte(content), 0o644)
		},
	}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected hello, got %s", string(data))
	}
}

func TestLuaRuntime_ReadFile(t *testing.T) {
	dir := t.TempDir()
	readFile := filepath.Join(dir, "input.txt")
	_ = os.WriteFile(readFile, []byte("world"), 0o644)

	script := filepath.Join(dir, "read_test.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	local content, err = ctx.read_file("`+strings.ReplaceAll(readFile, `\`, `/`)+`")
	if err ~= nil then
		error("read failed: " .. err)
	end
	if content ~= "world" then
		error("content mismatch: " .. tostring(content))
	end
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{
		ReadFile: func(path string) (string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
}

func TestLuaRuntime_SetEnv(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "env_test.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	ctx.set_env("TEST_KEY", "test_value")
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	env := make(map[string]string)
	ctx := &ScriptContext{
		SetEnv: func(k, v string) {
			env[k] = v
		},
	}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
	if env["TEST_KEY"] != "test_value" {
		t.Errorf("expected TEST_KEY=test_value, got %s", env["TEST_KEY"])
	}
}

func TestLuaRuntime_NoHook(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "no_hook.lua")
	// Script without pre_run - should not error
	_ = os.WriteFile(script, []byte(`
-- no hooks defined
local x = 1 + 2
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{Browser: "chrome"}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
}

func TestLuaRuntime_ScriptError(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "error.lua")
	_ = os.WriteFile(script, []byte(`
function pre_run()
	error("intentional error")
end
`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{Browser: "chrome"}
	err := rt.RunScript(script, ctx)
	if err == nil {
		t.Error("expected error from script with intentional error")
	}
}

func TestLuaRuntime_MissingScript(t *testing.T) {
	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{Browser: "chrome"}
	err := rt.RunScript("/nonexistent/path.lua", ctx)
	if err == nil {
		t.Error("expected error for missing script")
	}
}

func TestManager_DiscoverEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := mgr.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected no plugins in empty dir, got %d", len(plugins))
	}
}

func TestManager_PluginsDir(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if mgr.PluginsDir() != dir {
		t.Errorf("expected %s, got %s", dir, mgr.PluginsDir())
	}
}

func TestManager_DiscoverBinary(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write a binary plugin file
	binPath := filepath.Join(dir, "my-plugin.exe")
	_ = os.WriteFile(binPath, []byte("fake binary"), 0o755)

	// Install it in manifest as binary type
	if err := mgr.Install(ManifestEntry{
		Name:    "my-plugin",
		Version: "1.0",
		Type:    "binary",
		Path:    binPath,
	}); err != nil {
		t.Fatal(err)
	}

	plugins, err := mgr.Discover()
	if err != nil {
		t.Fatal(err)
	}

	// Should find the binary plugin
	found := false
	for _, p := range plugins {
		if p.Name == "my-plugin" && p.Type == "binary" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected binary plugin my-plugin, got %+v", plugins)
	}
}

func TestManager_GetManifestEntry(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Install(ManifestEntry{
		Name:    "ipc-test",
		Version: "2.0",
		Type:    "binary",
		Path:    filepath.Join(dir, "ipc-test.exe"),
	}); err != nil {
		t.Fatal(err)
	}

	entry, err := mgr.GetManifestEntry("ipc-test")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != "binary" || entry.Version != "2.0" {
		t.Errorf("expected binary v2.0, got type=%s version=%s", entry.Type, entry.Version)
	}

	// Non-existent entry
	_, err = mgr.GetManifestEntry("no-such-plugin")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestRunIPCPlugin(t *testing.T) {
	// Build a simple IPC plugin test helper in Go
	dir := t.TempDir()
	helperSrc := filepath.Join(dir, "helper.go")
	helperBin := filepath.Join(dir, "helper.exe")

	_ = os.WriteFile(helperSrc, []byte(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var req map[string]interface{}
	json.Unmarshal(data, &req)

	resp := map[string]interface{}{
		"extraArgs": []string{"--from-" + req["browser"].(string)},
		"env":       map[string]string{"PLUGIN_RAN": "1"},
	}
	out, _ := json.Marshal(resp)
	fmt.Print(string(out))
}
`), 0o644)

	// Compile the helper
	cmd := execCmd("go", "build", "-o", helperBin, helperSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping IPC test: cannot compile helper: %v\n%s", err, out)
	}

	ctx := &ScriptContext{
		Browser:    "chrome",
		Version:    "120",
		Profile:    "test-profile",
		ProfileDir: "/tmp/profiles/test",
	}

	resp, err := RunIPCPlugin(helperBin, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ExtraArgs) != 1 || resp.ExtraArgs[0] != "--from-chrome" {
		t.Errorf("expected --from-chrome, got %v", resp.ExtraArgs)
	}
	if resp.Env["PLUGIN_RAN"] != "1" {
		t.Errorf("expected PLUGIN_RAN=1, got %v", resp.Env)
	}
}

func TestRunIPCPlugin_Error(t *testing.T) {
	dir := t.TempDir()
	helperSrc := filepath.Join(dir, "helper_err.go")
	helperBin := filepath.Join(dir, "helper_err.exe")

	_ = os.WriteFile(helperSrc, []byte(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var req map[string]interface{}
	json.Unmarshal(data, &req)

	resp := map[string]interface{}{
		"error": "plugin failed intentionally",
	}
	out, _ := json.Marshal(resp)
	fmt.Print(string(out))
}
`), 0o644)

	cmd := execCmd("go", "build", "-o", helperBin, helperSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping IPC error test: cannot compile helper: %v\n%s", err, out)
	}

	ctx := &ScriptContext{Browser: "firefox"}
	_, err := RunIPCPlugin(helperBin, ctx)
	if err == nil {
		t.Error("expected error from plugin that returns error field")
	}
}

func TestRunIPCPlugin_Timeout(t *testing.T) {
	dir := t.TempDir()
	helperSrc := filepath.Join(dir, "helper_slow.go")
	helperBin := filepath.Join(dir, "helper_slow.exe")

	_ = os.WriteFile(helperSrc, []byte(`package main

import (
	"time"
)

func main() {
	time.Sleep(30 * time.Second)
}
`), 0o644)

	cmd := execCmd("go", "build", "-o", helperBin, helperSrc)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping IPC timeout test: cannot compile helper: %v\n%s", err, out)
	}

	ctx := &ScriptContext{Browser: "chrome"}
	_, err := RunIPCPlugin(helperBin, ctx)
	if err == nil {
		t.Error("expected timeout error from slow plugin")
	}
}

// execCmd is a test helper that runs a command.
func execCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	return cmd
}

// newHookRecorderCtx 构造一个 ScriptContext，用 env map 记录钩子调用情况。
// 配合下方脚本可追踪 pre_run/post_run/on_exit 是否被调用。
func newHookRecorderCtx(env map[string]string) *ScriptContext {
	return &ScriptContext{
		Browser: "chrome",
		Version: "120",
		SetEnv: func(k, v string) {
			env[k] = v
		},
	}
}

// hookScript 定义了 pre_run、post_run、on_exit 三个钩子，
// 每个钩子通过 ctx.set_env 记录自己被调用过。
const hookScript = `
function pre_run()
	ctx.set_env("PRE_RUN", "1")
end

function post_run()
	ctx.set_env("POST_RUN", "1")
end

function on_exit()
	ctx.set_env("ON_EXIT", "1")
end
`

func TestLuaRuntime_CallHook(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hooks.lua")
	_ = os.WriteFile(script, []byte(hookScript), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	env := make(map[string]string)
	ctx := newHookRecorderCtx(env)
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
	// pre_run 应在 RunScript 中被调用
	if env["PRE_RUN"] != "1" {
		t.Errorf("期望 pre_run 已被调用，env=%v", env)
	}

	// 通过公开的 CallHook 调用 post_run
	if err := rt.CallHook(string(HookPostRun)); err != nil {
		t.Fatalf("CallHook post_run 失败: %v", err)
	}
	if env["POST_RUN"] != "1" {
		t.Errorf("期望 post_run 已被调用，env=%v", env)
	}
}

func TestLuaRuntime_CallHook_Undefined(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "no_hooks.lua")
	_ = os.WriteFile(script, []byte(`-- 没有定义任何钩子`), 0o644)

	rt := NewLuaRuntime()
	defer rt.Close()

	ctx := &ScriptContext{Browser: "chrome"}
	if err := rt.RunScript(script, ctx); err != nil {
		t.Fatal(err)
	}
	// 调用未定义的钩子应返回 nil（不报错）
	if err := rt.CallHook(string(HookPostRun)); err != nil {
		t.Errorf("调用未定义的钩子应返回 nil，实际: %v", err)
	}
	if err := rt.CallHook(string(HookOnExit)); err != nil {
		t.Errorf("调用未定义的钩子应返回 nil，实际: %v", err)
	}
}

func TestLuaRuntime_CloseCallsOnExit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "on_exit.lua")
	_ = os.WriteFile(script, []byte(hookScript), 0o644)

	env := make(map[string]string)
	ctx := newHookRecorderCtx(env)

	rt := NewLuaRuntime()
	if err := rt.RunScript(script, ctx); err != nil {
		rt.Close()
		t.Fatal(err)
	}
	// 此时 on_exit 尚未被调用
	if env["ON_EXIT"] != "" {
		t.Errorf("on_exit 不应在 Close 前被调用，env=%v", env)
	}
	// Close 应触发 on_exit
	rt.Close()
	if env["ON_EXIT"] != "1" {
		t.Errorf("期望 Close 触发 on_exit，env=%v", env)
	}
}

func TestLuaRuntime_CloseNilSafe(t *testing.T) {
	// 确保 nil 接收者或 nil 状态下 Close 不会 panic
	var rt *LuaRuntime
	rt.Close() // 不应 panic

	rt = &LuaRuntime{} // L 为 nil
	rt.Close()         // 不应 panic
}

func TestManager_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "lifecycle.lua")
	_ = os.WriteFile(script, []byte(hookScript), 0o644)

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	env := make(map[string]string)
	ctx := newHookRecorderCtx(env)

	// 1. 加载并运行插件（触发 pre_run）
	if err := mgr.RunLuaPlugin("lifecycle", script, ctx); err != nil {
		t.Fatal(err)
	}
	if env["PRE_RUN"] != "1" {
		t.Errorf("期望 pre_run 已被调用，env=%v", env)
	}

	// 2. 浏览器退出后调用 post_run
	if errs := mgr.PostRunPlugins(); errs != nil {
		t.Fatalf("PostRunPlugins 返回错误: %v", errs)
	}
	if env["POST_RUN"] != "1" {
		t.Errorf("期望 post_run 已被调用，env=%v", env)
	}

	// on_exit 此时不应被调用
	if env["ON_EXIT"] != "" {
		t.Errorf("on_exit 不应在卸载前被调用，env=%v", env)
	}

	// 3. 卸载插件（触发 on_exit）
	mgr.UnloadPlugins()
	if env["ON_EXIT"] != "1" {
		t.Errorf("期望 on_exit 在卸载时被调用，env=%v", env)
	}
}

func TestManager_PostRunPlugins_NoPlugins(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	// 没有已加载插件时应返回 nil
	if errs := mgr.PostRunPlugins(); errs != nil {
		t.Errorf("期望 nil，实际: %v", errs)
	}
}

func TestManager_CloseTriggersOnExit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "close.lua")
	_ = os.WriteFile(script, []byte(hookScript), 0o644)

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	env := make(map[string]string)
	ctx := newHookRecorderCtx(env)

	if err := mgr.RunLuaPlugin("close", script, ctx); err != nil {
		mgr.Close()
		t.Fatal(err)
	}
	// Close 应触发所有已加载插件的 on_exit
	mgr.Close()
	if env["ON_EXIT"] != "1" {
		t.Errorf("期望 Close 触发 on_exit，env=%v", env)
	}
}
