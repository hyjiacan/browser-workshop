package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest tracks installed plugins.
type Manifest struct {
	Version  string                   `json:"version"`
	Plugins  map[string]ManifestEntry `json:"plugins"`
	Modified time.Time                `json:"modified"`
}

// ManifestEntry records a single installed plugin.
type ManifestEntry struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	Type        string    `json:"type"` // "lua", "binary"
	InstalledAt time.Time `json:"installedAt"`
	Path        string    `json:"path"`
}

// LoadManifest reads the manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: "1", Plugins: make(map[string]ManifestEntry)}, nil
		}
		return nil, err
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if m.Plugins == nil {
		m.Plugins = make(map[string]ManifestEntry)
	}
	return m, nil
}

// SaveManifest writes the manifest to disk.
func SaveManifest(m *Manifest, path string) error {
	m.Modified = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
