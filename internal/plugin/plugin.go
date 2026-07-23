package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
func (mgr *Manager) Discover() ([]Plugin, error) {
	entries, err := os.ReadDir(mgr.pluginsDir)
	if err != nil {
		return nil, err
	}

	var plugins []Plugin
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".lua") {
			pluginName := strings.TrimSuffix(name, ".lua")
			plugins = append(plugins, Plugin{
				Name: pluginName,
				Path: filepath.Join(mgr.pluginsDir, name),
				Type: "lua",
			})
		}
	}
	return plugins, nil
}

// List returns installed plugins from the manifest.
func (mgr *Manager) List() []ManifestEntry {
	var result []ManifestEntry
	for _, entry := range mgr.manifest.Plugins {
		result = append(result, entry)
	}
	return result
}

// Install records a plugin in the manifest.
func (mgr *Manager) Install(entry ManifestEntry) error {
	mgr.manifest.Plugins[entry.Name] = entry
	return SaveManifest(mgr.manifest, mgr.manifestPath)
}

// Uninstall removes a plugin from the manifest and disk.
func (mgr *Manager) Uninstall(name string) error {
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
