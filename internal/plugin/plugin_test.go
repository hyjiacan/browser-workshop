package plugin

import (
	"os"
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
