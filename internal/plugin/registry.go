package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRegistryURL is the official plugin registry.
const DefaultRegistryURL = "https://gitee.com/hyjiacan/browser-workshop-plugins/raw/master/registry.json"

// cacheTTL is the registry cache validity duration (24 hours).
const cacheTTL = 24 * time.Hour

// RegistryEntry describes a plugin in the remote registry.
type RegistryEntry struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Source      string                 `json:"source"`
	Type        string                 `json:"type"`
	Latest      string                 `json:"latest"`
	Versions    map[string]VersionInfo `json:"versions"`
	Tags        []string               `json:"tags"`
}

// VersionInfo describes a single plugin version.
type VersionInfo struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

// Registry is the remote plugin index.
type Registry struct {
	Version string                   `json:"version"`
	Plugins map[string]RegistryEntry `json:"plugins"`
}

// RegistryClient fetches and caches the registry.
type RegistryClient struct {
	URL      string
	CacheDir string
	client   *http.Client
}

// NewRegistryClient creates a registry client.
func NewRegistryClient(cacheDir string) *RegistryClient {
	return &RegistryClient{
		URL:      DefaultRegistryURL,
		CacheDir: cacheDir,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// loadCache attempts to load the registry from the local cache file.
// Returns nil (with no error) if the cache does not exist or has expired.
func (c *RegistryClient) loadCache() (*Registry, error) {
	if c.CacheDir == "" {
		return nil, nil
	}

	cachePath := filepath.Join(c.CacheDir, "registry.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, nil // cache file not found
	}

	// Check modification time for TTL
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, nil
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil, nil // cache expired
	}

	reg := &Registry{}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, nil // cache corrupt, re-fetch
	}
	return reg, nil
}

// Fetch downloads the registry JSON, using cache if still valid.
func (c *RegistryClient) Fetch() (*Registry, error) {
	// Try cache first
	if reg, err := c.loadCache(); err == nil && reg != nil {
		return reg, nil
	}

	resp, err := c.client.Get(c.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = os.MkdirAll(c.CacheDir, 0o755)
	if cacheErr := os.WriteFile(filepath.Join(c.CacheDir, "registry.json"), data, 0o644); cacheErr != nil {
		// Cache write failure is non-fatal, but log it for debugging.
		fmt.Fprintf(os.Stderr, "警告: 写入注册表缓存失败: %v\n", cacheErr)
	}

	reg := &Registry{}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}
	return reg, nil
}

// Search finds plugins matching a query.
func (c *RegistryClient) Search(query string) ([]RegistryEntry, error) {
	reg, err := c.Fetch()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []RegistryEntry
	for _, entry := range reg.Plugins {
		if strings.Contains(strings.ToLower(entry.Name), query) ||
			strings.Contains(strings.ToLower(entry.Description), query) {
			results = append(results, entry)
		}
	}
	return results, nil
}

// Get returns a specific plugin entry.
func (c *RegistryClient) Get(name string) (*RegistryEntry, error) {
	reg, err := c.Fetch()
	if err != nil {
		return nil, err
	}
	entry, ok := reg.Plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in registry", name)
	}
	return &entry, nil
}

// Download fetches a plugin file from a URL.
func (c *RegistryClient) Download(url string) ([]byte, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// DownloadWithHash fetches a plugin file and verifies its SHA256 hash.
// If expectedHash is empty, the hash check is skipped.
func (c *RegistryClient) DownloadWithHash(url, expectedHash string) ([]byte, error) {
	data, err := c.Download(url)
	if err != nil {
		return nil, err
	}

	if expectedHash != "" {
		sum := sha256.Sum256(data)
		actual := hex.EncodeToString(sum[:])
		if actual != expectedHash {
			return nil, fmt.Errorf("SHA256 校验失败: 期望 %s, 实际 %s", expectedHash, actual)
		}
	}

	return data, nil
}
