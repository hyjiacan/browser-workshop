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
	HookPreRun  Hook = "pre_run"
	HookPostRun Hook = "post_run"
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
		_ = os.Remove(entry.Path)
	}
	delete(mgr.manifest.Plugins, name)
	return SaveManifest(mgr.manifest, mgr.manifestPath)
}

// PluginsDir returns the plugins directory path.
func (mgr *Manager) PluginsDir() string {
	return mgr.pluginsDir
}
