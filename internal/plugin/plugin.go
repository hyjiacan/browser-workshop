package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Hook defines lifecycle events where plugins can intervene.
type Hook string

const (
	HookPreRun     Hook = "pre_run"
	HookPostRun    Hook = "post_run"
	HookPreInstall  Hook = "pre_install"
	HookPostInstall Hook = "post_install"
	HookOnExit     Hook = "on_exit"
)

// Plugin represents a discovered plugin on disk.
type Plugin struct {
	Name     string // e.g. "fingerprint-enhanced"
	Path     string // absolute path to plugin file
	Type     string // "lua" or "binary"
	Manifest *ManifestEntry
}

// Manager discovers and loads plugins.
type Manager struct {
	pluginsDir   string
	manifest     *Manifest
	manifestPath string
	mu           sync.RWMutex
	// loaded 保存当前已加载的 Lua 运行时，按插件名索引。
	// 这些运行时需要保持存活，以便在浏览器退出后调用 post_run，
	// 并在插件卸载时调用 on_exit。
	loaded map[string]*LuaRuntime
}

// NewManager creates a plugin manager.
func NewManager(pluginsDir string) (*Manager, error) {
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating plugins dir: %w", err)
	}
	manifestPath := filepath.Join(pluginsDir, "manifest.json")
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	return &Manager{
		pluginsDir:   pluginsDir,
		manifest:     m,
		manifestPath: manifestPath,
		loaded:       make(map[string]*LuaRuntime),
	}, nil
}

// Discover scans the plugins directory and returns all valid plugins.
// It searches for .lua files and also checks the manifest for binary plugins.
func (mgr *Manager) Discover() ([]Plugin, error) {
	entries, err := os.ReadDir(mgr.pluginsDir)
	if err != nil {
		return nil, err
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var plugins []Plugin
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Lua plugins
		if strings.HasSuffix(name, ".lua") {
			pluginName := strings.TrimSuffix(name, ".lua")
			plugins = append(plugins, Plugin{
				Name: pluginName,
				Path: filepath.Join(mgr.pluginsDir, name),
				Type: "lua",
			})
			continue
		}

		// Binary plugins: check manifest for type info and executable permission
		pluginName := strings.TrimSuffix(name, filepath.Ext(name))
		if pluginName == "" {
			continue
		}
		if me, ok := mgr.manifest.Plugins[pluginName]; ok && me.Type == "binary" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			// Not executable, skip. On Windows, file permissions don't work the same way,
			// so we rely on the manifest type being "binary".
			continue
		}
			plugins = append(plugins, Plugin{
				Name:     pluginName,
				Path:     filepath.Join(mgr.pluginsDir, name),
				Type:     "binary",
				Manifest: &me,
			})
		}
	}
	return plugins, nil
}

// GetManifestEntry returns the manifest entry for a plugin by name.
func (mgr *Manager) GetManifestEntry(name string) (*ManifestEntry, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	entry, ok := mgr.manifest.Plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in manifest", name)
	}
	return &entry, nil
}

// List returns installed plugins from the manifest.
func (mgr *Manager) List() []ManifestEntry {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var result []ManifestEntry
	for _, entry := range mgr.manifest.Plugins {
		result = append(result, entry)
	}
	return result
}

// Install records a plugin in the manifest.
func (mgr *Manager) Install(entry ManifestEntry) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.manifest.Plugins[entry.Name] = entry
	return SaveManifest(mgr.manifest, mgr.manifestPath)
}

// Uninstall removes a plugin from the manifest and disk.
func (mgr *Manager) Uninstall(name string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	entry, ok := mgr.manifest.Plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not installed", name)
	}
	if entry.Path != "" {
		// Validate that the plugin path is within the plugins directory
		// to prevent deletion of arbitrary system files.
		absPath, err := filepath.Abs(entry.Path)
		if err != nil {
			return fmt.Errorf("resolving plugin path: %w", err)
		}
		absPluginsDir, err := filepath.Abs(mgr.pluginsDir)
		if err != nil {
			return fmt.Errorf("resolving plugins dir: %w", err)
		}
		rel, err := filepath.Rel(absPluginsDir, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			return fmt.Errorf("plugin path %q is outside plugins directory", entry.Path)
		}
		_ = os.Remove(absPath)
	}
	delete(mgr.manifest.Plugins, name)
	return SaveManifest(mgr.manifest, mgr.manifestPath)
}

// PluginsDir returns the plugins directory path.
func (mgr *Manager) PluginsDir() string {
	return mgr.pluginsDir
}

// RunLuaPlugin 加载并执行指定名称的 Lua 插件脚本。
// RunScript 内部会调用 pre_run 钩子（在浏览器启动前执行）。
// 运行时会保留在管理器中，以便后续在浏览器退出后调用 post_run，
// 并在卸载时调用 on_exit。
// 如果同名插件此前已加载，会先卸载旧的运行时（触发其 on_exit）。
func (mgr *Manager) RunLuaPlugin(name, scriptPath string, ctx *ScriptContext) error {
	rt := NewLuaRuntime()
	if err := rt.RunScript(scriptPath, ctx); err != nil {
		// 加载或 pre_run 失败，直接释放运行时（会尝试调用 on_exit）
		rt.Close()
		return err
	}

	mgr.mu.Lock()
	// 若同名插件已加载，先关闭旧的运行时
	if old, ok := mgr.loaded[name]; ok {
		old.Close()
	}
	mgr.loaded[name] = rt
	mgr.mu.Unlock()
	return nil
}

// PostRunPlugins 在浏览器退出后调用所有已加载 Lua 插件的 post_run 钩子。
// 返回每个插件调用时产生的错误列表；单个插件的失败不会中断其他插件。
// 若没有任何错误，返回 nil。
func (mgr *Manager) PostRunPlugins() []error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var errs []error
	for name, rt := range mgr.loaded {
		if err := rt.CallHook(string(HookPostRun)); err != nil {
			errs = append(errs, fmt.Errorf("插件 %q post_run 失败: %w", name, err))
		}
	}
	return errs
}

// UnloadPlugins 卸载所有已加载的 Lua 插件。
// 每个运行时的 Close 会自动调用 on_exit 钩子，然后释放 Lua 状态。
func (mgr *Manager) UnloadPlugins() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for name, rt := range mgr.loaded {
		rt.Close() // Close 内部会调用 on_exit 钩子
		delete(mgr.loaded, name)
	}
}

// Close 卸载所有已加载的插件并释放资源。
// 调用后管理器不再持有任何运行时，但清单与发现功能仍可使用。
func (mgr *Manager) Close() {
	mgr.UnloadPlugins()
}
