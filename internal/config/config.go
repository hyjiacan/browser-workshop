// Package config provides configuration loading and management for bws.
package config

import (
	"encoding/json"
	"os"
	"sort"
	"time"
)

// Config is the top-level configuration for bws.
type Config struct {
	// DefaultBrowser is the default browser to use when not specified.
	DefaultBrowser string `json:"defaultBrowser"`

	// DefaultChannel is the default release channel.
	DefaultChannel string `json:"defaultChannel"`

	// Aliases maps short names to browser@version specs.
	Aliases map[string]string `json:"aliases"`

	// Sources configures data sources per browser.
	Sources map[string][]SourceConfig `json:"sources"`

	// RepoPath is the path to the local binary repository directory.
	RepoPath string `json:"repoPath"`

	// RemoteSource is the URL of a remote bws serve instance (offline distribution).
	// Empty means no remote source is configured.
	RemoteSource string `json:"remoteSource"`

	// LogLevel is the minimum log level to write.
	LogLevel string `json:"logLevel"`

	// DataDir is the directory where bws stores its data (versions, cache, etc.).
	// If empty, defaults to the directory containing the config file.
	DataDir string `json:"dataDir"`

	// Download configures download behavior.
	Download DownloadConfig `json:"download"`

	// Cache configures caching behavior.
	Cache CacheConfig `json:"cache"`

	// Source switches control which data sources are active.
	// All default to true for backward compatibility.
	EnableServeSource bool `json:"enableServeSource"` // serve HTTP source
	EnableOmahaSource bool `json:"enableOmahaSource"` // Chrome Omaha protocol
	EnableFirefoxFTP  bool `json:"enableFirefoxFTP"`  // Firefox FTP releases (reserved)

	// DiskSpaceThresholdGB is the minimum free space (in GB) required before
	// warning the user. Default is 5 GB.
	DiskSpaceThresholdGB int `json:"diskSpaceThresholdGB"`

	// Proxy is the proxy URL used for both bws downloads and browser launching.
	// Supported schemes: http, https, socks5, socks5h.
	// Empty means no proxy (direct connection).
	// Can be overridden per-launch with --proxy flag.
	Proxy string `json:"proxy"`

	// Language is the UI language. Supported: "zh" (default), "en".
	// Empty means auto-detect from environment.
	Language string `json:"language"`
}

// SourceConfig describes a remote data source for a browser.
type SourceConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	BaseURL  string `json:"baseURL"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// DownloadConfig controls download behavior.
type DownloadConfig struct {
	MaxConcurrency int           `json:"maxConcurrency"`
	RetryCount     int           `json:"retryCount"`
	RetryDelay     time.Duration `json:"-"`
	RetryDelayStr  string        `json:"retryDelay"`
	Timeout        time.Duration `json:"-"`
	TimeoutStr     string        `json:"timeout"`
}

// CacheConfig controls caching behavior.
type CacheConfig struct {
	ManifestTTL    time.Duration `json:"-"`
	ManifestTTLStr string        `json:"manifestTTL"`
	DownloadTTL    time.Duration `json:"-"`
	DownloadTTLStr string        `json:"downloadTTL"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		DefaultBrowser: "chrome",
		DefaultChannel: "stable",
		LogLevel:       "info",
		Aliases: map[string]string{
			"stable": "chrome@latest",
			"beta":   "chrome@beta",
		},
		Sources: map[string][]SourceConfig{
			"firefox": {
				{
					Name:     "mozilla-ftp",
					Type:     "firefox-ftp",
					BaseURL:  "https://ftp.mozilla.org/pub/firefox/releases/",
					Enabled:  true,
					Priority: 10,
				},
			},
			"chromium": {
				{
					Name:     "chromium-snapshots",
					Type:     "chromium-gcs",
					BaseURL:  "https://commondatastorage.googleapis.com/chromium-browser-snapshots/",
					Enabled:  true,
					Priority: 10,
				},
			},
		},
		Download: DownloadConfig{
			MaxConcurrency: 3,
			RetryCount:     3,
			RetryDelayStr:  "2s",
			RetryDelay:     2 * time.Second,
			TimeoutStr:     "30m",
			Timeout:        30 * time.Minute,
		},
		Cache: CacheConfig{
			ManifestTTLStr: "24h",
			ManifestTTL:    24 * time.Hour,
			DownloadTTLStr: "168h",
			DownloadTTL:    168 * time.Hour,
		},
		EnableServeSource:    true,
		EnableOmahaSource:    true,
		EnableFirefoxFTP:     true,
		DiskSpaceThresholdGB: 5,
	}
}

// Load reads a config file from the given path.
// If the file doesn't exist, it returns a default config and no error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Parse duration strings
	if err := cfg.parseDurations(); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	cfg.applyDefaults()

	return cfg, nil
}

// Save writes the config to the given path as pretty-printed JSON.
func Save(cfg *Config, path string) error {
	// Convert durations to strings for serialization
	cfg.Download.RetryDelayStr = cfg.Download.RetryDelay.String()
	cfg.Download.TimeoutStr = cfg.Download.Timeout.String()
	cfg.Cache.ManifestTTLStr = cfg.Cache.ManifestTTL.String()
	cfg.Cache.DownloadTTLStr = cfg.Cache.DownloadTTL.String()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// parseDurations parses duration strings from the JSON-serialized form.
func (c *Config) parseDurations() error {
	var err error

	if c.Download.RetryDelayStr != "" {
		c.Download.RetryDelay, err = time.ParseDuration(c.Download.RetryDelayStr)
		if err != nil {
			return err
		}
	}
	if c.Download.TimeoutStr != "" {
		c.Download.Timeout, err = time.ParseDuration(c.Download.TimeoutStr)
		if err != nil {
			return err
		}
	}
	if c.Cache.ManifestTTLStr != "" {
		c.Cache.ManifestTTL, err = time.ParseDuration(c.Cache.ManifestTTLStr)
		if err != nil {
			return err
		}
	}
	if c.Cache.DownloadTTLStr != "" {
		c.Cache.DownloadTTL, err = time.ParseDuration(c.Cache.DownloadTTLStr)
		if err != nil {
			return err
		}
	}

	return nil
}

// applyDefaults fills in default values for fields that are zero-valued.
func (c *Config) applyDefaults() {
	def := Default()

	if c.DefaultBrowser == "" {
		c.DefaultBrowser = def.DefaultBrowser
	}
	if c.DefaultChannel == "" {
		c.DefaultChannel = def.DefaultChannel
	}
	if c.LogLevel == "" {
		c.LogLevel = def.LogLevel
	}
	if c.Aliases == nil {
		c.Aliases = def.Aliases
	}
	if c.Download.MaxConcurrency == 0 {
		c.Download.MaxConcurrency = def.Download.MaxConcurrency
	}
	if c.Download.RetryCount == 0 {
		c.Download.RetryCount = def.Download.RetryCount
	}
	if c.Download.RetryDelay == 0 {
		c.Download.RetryDelay = def.Download.RetryDelay
		c.Download.RetryDelayStr = def.Download.RetryDelayStr
	}
	if c.Download.Timeout == 0 {
		c.Download.Timeout = def.Download.Timeout
		c.Download.TimeoutStr = def.Download.TimeoutStr
	}
	if c.Cache.ManifestTTL == 0 {
		c.Cache.ManifestTTL = def.Cache.ManifestTTL
		c.Cache.ManifestTTLStr = def.Cache.ManifestTTLStr
	}
	if c.Cache.DownloadTTL == 0 {
		c.Cache.DownloadTTL = def.Cache.DownloadTTL
		c.Cache.DownloadTTLStr = def.Cache.DownloadTTLStr
	}
}

// SetRepoPath sets the repository path and persists the config.
func (c *Config) SetRepoPath(path string) {
	c.RepoPath = path
}

// GetRepoPath returns the configured repository path.
func (c *Config) GetRepoPath() string {
	return c.RepoPath
}

// GetRemoteSource returns the configured remote source URL.
// Returns empty string if not configured.
func (c *Config) GetRemoteSource() string {
	return c.RemoteSource
}

// SetRemoteSource sets the remote source URL.
func (c *Config) SetRemoteSource(url string) {
	c.RemoteSource = url
}

// ClearRemoteSource clears the remote source configuration.
func (c *Config) ClearRemoteSource() {
	c.RemoteSource = ""
}

// --- Source switches ---

func (c *Config) IsServeSourceEnabled() bool     { return c.EnableServeSource }
func (c *Config) SetServeSourceEnabled(v bool)   { c.EnableServeSource = v }
func (c *Config) IsOmahaSourceEnabled() bool      { return c.EnableOmahaSource }
func (c *Config) SetOmahaSourceEnabled(v bool)    { c.EnableOmahaSource = v }
func (c *Config) IsFirefoxFTPEnabled() bool       { return c.EnableFirefoxFTP }
func (c *Config) SetFirefoxFTPEnabled(v bool)    { c.EnableFirefoxFTP = v }

func (c *Config) GetDiskSpaceThresholdGB() int {
	if c.DiskSpaceThresholdGB <= 0 {
		return 5
	}
	return c.DiskSpaceThresholdGB
}
func (c *Config) SetDiskSpaceThresholdGB(v int) { c.DiskSpaceThresholdGB = v }

// GetProxy returns the configured proxy URL.
// Returns empty string if no proxy is configured.
func (c *Config) GetProxy() string {
	return c.Proxy
}

// SetProxy sets the proxy URL.
// Pass empty string to clear the proxy.
func (c *Config) SetProxy(proxy string) {
	c.Proxy = proxy
}

// GetLanguage returns the configured UI language.
func (c *Config) GetLanguage() string {
	return c.Language
}

// SetLanguage sets the UI language. Supported: "zh", "en".
func (c *Config) SetLanguage(lang string) {
	c.Language = lang
}

// GetSources returns enabled source configs for the given browser, sorted by priority.
func (c *Config) GetSources(browser string) []SourceConfig {
	srcs, ok := c.Sources[browser]
	if !ok {
		return nil
	}

	var result []SourceConfig
	for _, s := range srcs {
		if s.Enabled {
			result = append(result, s)
		}
	}

	// Sort by priority (ascending = lower number first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})

	return result
}
