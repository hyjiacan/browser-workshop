// Package config provides configuration loading and management for bws.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config is the top-level configuration for bws.
type Config struct {
	mu sync.RWMutex

	// DefaultBrowser is the default browser to use when not specified.
	DefaultBrowser string

	// DefaultVersion is the default version for the default browser.
	// Empty means no specific version is set (use latest/system).
	DefaultVersion string

	// DefaultChannel is the default release channel.
	DefaultChannel string

	// Aliases maps short names to browser@version specs.
	Aliases map[string]string

	// Sources configures data sources per browser.
	Sources map[string][]SourceConfig

	// RepoPath is the path to the local binary repository directory.
	RepoPath string

	// RemoteSource is the URL of a remote bws serve instance (offline distribution).
	// Empty means no remote source is configured.
	RemoteSource string

	// Log configures logging behavior.
	Log LogConfig

	// DataDir is the directory where bws stores its data (versions, cache, etc.).
	// If empty, defaults to the directory containing the config file.
	DataDir string

	// Download configures download behavior.
	Download DownloadConfig

	// Cache configures caching behavior.
	Cache CacheConfig

	// Source switches control which data sources are active.
	EnableServeSource bool // serve HTTP source
	EnableFirefoxFTP  bool // Firefox FTP releases (reserved)

	// DiskSpaceThresholdGB is the minimum free space (in GB) required before
	// warning the user. Default is 5 GB.
	DiskSpaceThresholdGB int

	// Proxy is the proxy URL used for both bws downloads and browser launching.
	// Supported schemes: http, https, socks5, socks5h.
	// Empty means no proxy (direct connection).
	// Can be overridden per-launch with --proxy flag.
	Proxy string

	// Language is the UI language. Supported: "zh" (default), "en".
	// Empty means auto-detect from environment.
	Language string
}

// LogConfig configures logging behavior.
type LogConfig struct {
	// ConsoleLevel is the log level for console (stderr) output.
	// Valid values: trace, debug, info, warn, error, fatal.
	ConsoleLevel string

	// FileLevel is the log level for file output.
	// Valid values: trace, debug, info, warn, error, fatal.
	FileLevel string

	// MaxSizeMB is the maximum size in MB for a single log file before rotation.
	// 0 means no size limit (infinite append).
	MaxSizeMB int

	// MaxBackups is the maximum number of backup log files to keep.
	// 0 means no backups are kept when rotating.
	MaxBackups int
}

// SourceConfig describes a remote data source for a browser.
type SourceConfig struct {
	Name     string
	Type     string
	BaseURL  string
	Enabled  bool
	Priority int
}

// DownloadConfig controls download behavior.
type DownloadConfig struct {
	MaxConcurrency int
	RetryCount     int
	RetryDelay     time.Duration
	RetryDelayStr  string
	Timeout        time.Duration
	TimeoutStr     string
}

// CacheConfig controls caching behavior.
type CacheConfig struct {
	ManifestTTL    time.Duration
	ManifestTTLStr string
	DownloadTTL    time.Duration
	DownloadTTLStr string
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		DefaultBrowser: "chrome",
		DefaultChannel: "stable",
		Log: LogConfig{
			ConsoleLevel: "info",
			FileLevel:    "debug",
			MaxSizeMB:    10,
			MaxBackups:   5,
		},
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

	return loadINI(data)
}

// loadINI loads the INI format.
func loadINI(data []byte) (*Config, error) {
	cfg := Default()

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}

		// Key=value pair.
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(line[:eqIdx]))
		value := strings.TrimSpace(line[eqIdx+1:])
		// Remove surrounding quotes if present.
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		cfg.setINIValue(section, key, value)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := cfg.parseDurations(); err != nil {
		return nil, err
	}
	cfg.applyDefaults()

	return cfg, nil
}

// setINIValue sets a single INI key-value pair on the config.
func (c *Config) setINIValue(section, key, value string) {
	switch section {
	case "client":
		switch key {
		case "default-browser", "defaultbrowser":
			c.DefaultBrowser = value
		case "default-version", "defaultversion":
			c.DefaultVersion = value
		case "default-channel", "defaultchannel":
			c.DefaultChannel = value
		case "language":
			c.Language = value
		case "data-dir", "datadir":
			c.DataDir = value
		case "repo-path", "repopath":
			c.RepoPath = value
		case "remote-source", "remotesource":
			c.RemoteSource = value
		}
	case "alias":
		if c.Aliases == nil {
			c.Aliases = make(map[string]string)
		}
		c.Aliases[key] = value
	case "log":
		switch key {
		case "console-level", "consolelevel":
			c.Log.ConsoleLevel = value
		case "file-level", "filelevel":
			c.Log.FileLevel = value
		case "max-size-mb", "maxsizemb", "max-size":
			if v, err := strconv.Atoi(value); err == nil {
				c.Log.MaxSizeMB = v
			}
		case "max-backups", "maxbackups":
			if v, err := strconv.Atoi(value); err == nil {
				c.Log.MaxBackups = v
			}
		}
	case "download":
		switch key {
		case "max-concurrency", "maxconcurrency":
			if v, err := strconv.Atoi(value); err == nil {
				c.Download.MaxConcurrency = v
			}
		case "retry-count", "retrycount":
			if v, err := strconv.Atoi(value); err == nil {
				c.Download.RetryCount = v
			}
		case "retry-delay", "retrydelay":
			c.Download.RetryDelayStr = value
		case "timeout":
			c.Download.TimeoutStr = value
		}
	case "cache":
		switch key {
		case "manifest-ttl", "manifestttl":
			c.Cache.ManifestTTLStr = value
		case "download-ttl", "downloadttl":
			c.Cache.DownloadTTLStr = value
		}
	case "source-switches", "sourceswitches", "source":
		switch key {
		case "enable-serve-source", "enableservesource":
			c.EnableServeSource = parseBool(value)
		case "enable-firefox-ftp", "enablefirefoxftp":
			c.EnableFirefoxFTP = parseBool(value)
		}
	case "network":
		switch key {
		case "proxy":
			c.Proxy = value
		case "disk-space-threshold-gb", "diskspacethresholdgb", "disk-space-threshold":
			if v, err := strconv.Atoi(value); err == nil {
				c.DiskSpaceThresholdGB = v
			}
		}
	}

	// Handle [source.browser.index] sections.
	if strings.HasPrefix(section, "source.") {
		parts := strings.Split(section, ".")
		if len(parts) == 3 {
			browser := parts[1]
			idxStr := parts[2]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return
			}
			if c.Sources == nil {
				c.Sources = make(map[string][]SourceConfig)
			}
			// Ensure slice is long enough.
			for len(c.Sources[browser]) <= idx {
				c.Sources[browser] = append(c.Sources[browser], SourceConfig{})
			}
			src := c.Sources[browser][idx]
			switch key {
			case "name":
				src.Name = value
			case "type":
				src.Type = value
			case "base-url", "baseurl":
				src.BaseURL = value
			case "enabled":
				src.Enabled = parseBool(value)
			case "priority":
				if v, err := strconv.Atoi(value); err == nil {
					src.Priority = v
				}
			}
			c.Sources[browser][idx] = src
		}
	}
}

// Save writes the config to the given path as INI.
// Note: Save reads fields under the config's mutex to ensure thread safety.
func Save(cfg *Config, path string) error {
	cfg.mu.RLock()
	// Convert durations to strings for serialization (local copies, don't mutate cfg).
	retryDelayStr := cfg.Download.RetryDelay.String()
	timeoutStr := cfg.Download.Timeout.String()
	manifestTTLStr := cfg.Cache.ManifestTTL.String()
	downloadTTLStr := cfg.Cache.DownloadTTL.String()

	var sb strings.Builder
	sb.WriteString("# ===============================================================\n")
	sb.WriteString("# bws 客户端配置文件\n")
	sb.WriteString("# 位置: " + filepath.Base(path) + "\n")
	sb.WriteString("# ===============================================================\n")
	sb.WriteString("#\n")
	sb.WriteString("# 修改此文件后，下次运行 bws 命令时自动生效。\n")
	sb.WriteString("#\n")
	sb.WriteString("\n")

	// [client]
	sb.WriteString("[client]\n")
	sb.WriteString("\n")
	sb.WriteString("# 默认浏览器\n")
	sb.WriteString("# 可选值: chrome, firefox, chromium\n")
	sb.WriteString(fmt.Sprintf("default-browser = %s\n", cfg.DefaultBrowser))
	sb.WriteString("\n")
	sb.WriteString("# 默认版本（留空则使用最新或系统版本）\n")
	sb.WriteString(fmt.Sprintf("default-version = %s\n", cfg.DefaultVersion))
	sb.WriteString("\n")
	sb.WriteString("# 默认发布渠道\n")
	sb.WriteString("# 可选值: stable, beta, dev, canary, esr\n")
	sb.WriteString(fmt.Sprintf("default-channel = %s\n", cfg.DefaultChannel))
	sb.WriteString("\n")
	sb.WriteString("# 界面语言\n")
	sb.WriteString("# 可选值: zh（中文）, en（英文）\n")
	sb.WriteString(fmt.Sprintf("language = %s\n", cfg.Language))
	sb.WriteString("\n")
	sb.WriteString("# 数据目录（留空则使用配置文件所在目录）\n")
	sb.WriteString(fmt.Sprintf("data-dir = %s\n", cfg.DataDir))
	sb.WriteString("\n")
	sb.WriteString("# 本地仓库路径\n")
	sb.WriteString(fmt.Sprintf("repo-path = %s\n", cfg.RepoPath))
	sb.WriteString("\n")
	sb.WriteString("# 远程 bws serve 源地址\n")
	sb.WriteString("# 示例: http://192.168.1.100:8080\n")
	sb.WriteString(fmt.Sprintf("remote-source = %s\n", cfg.RemoteSource))
	sb.WriteString("\n")

	// [alias]
	if len(cfg.Aliases) > 0 {
		sb.WriteString("[alias]\n")
		sb.WriteString("\n")
		// Sort keys for stable output.
		var keys []string
		for k := range cfg.Aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("%s = %s\n", k, cfg.Aliases[k]))
		}
		sb.WriteString("\n")
	}

	// [log]
	sb.WriteString("[log]\n")
	sb.WriteString("\n")
	sb.WriteString("# 控制台日志级别\n")
	sb.WriteString("# 可选值: trace, debug, info, warn, error, fatal\n")
	sb.WriteString(fmt.Sprintf("console-level = %s\n", cfg.Log.ConsoleLevel))
	sb.WriteString("\n")
	sb.WriteString("# 文件日志级别\n")
	sb.WriteString("# 可选值: trace, debug, info, warn, error, fatal\n")
	sb.WriteString(fmt.Sprintf("file-level = %s\n", cfg.Log.FileLevel))
	sb.WriteString("\n")
	sb.WriteString("# 单个日志文件最大大小（MB）\n")
	sb.WriteString(fmt.Sprintf("max-size-mb = %d\n", cfg.Log.MaxSizeMB))
	sb.WriteString("\n")
	sb.WriteString("# 保留的备份日志文件数量\n")
	sb.WriteString(fmt.Sprintf("max-backups = %d\n", cfg.Log.MaxBackups))
	sb.WriteString("\n")

	// [download]
	sb.WriteString("[download]\n")
	sb.WriteString("\n")
	sb.WriteString("# 最大并发下载数\n")
	sb.WriteString(fmt.Sprintf("max-concurrency = %d\n", cfg.Download.MaxConcurrency))
	sb.WriteString("\n")
	sb.WriteString("# 下载失败重试次数\n")
	sb.WriteString(fmt.Sprintf("retry-count = %d\n", cfg.Download.RetryCount))
	sb.WriteString("\n")
	sb.WriteString("# 重试间隔（示例: 2s, 1m）\n")
	sb.WriteString(fmt.Sprintf("retry-delay = %s\n", retryDelayStr))
	sb.WriteString("\n")
	sb.WriteString("# 下载超时（示例: 30m, 1h）\n")
	sb.WriteString(fmt.Sprintf("timeout = %s\n", timeoutStr))
	sb.WriteString("\n")

	// [cache]
	sb.WriteString("[cache]\n")
	sb.WriteString("\n")
	sb.WriteString("# 清单缓存有效期\n")
	sb.WriteString(fmt.Sprintf("manifest-ttl = %s\n", manifestTTLStr))
	sb.WriteString("\n")
	sb.WriteString("# 下载缓存有效期\n")
	sb.WriteString(fmt.Sprintf("download-ttl = %s\n", downloadTTLStr))
	sb.WriteString("\n")

	// [source-switches]
	sb.WriteString("[source-switches]\n")
	sb.WriteString("\n")
	sb.WriteString("# 是否启用远程 serve 源\n")
	sb.WriteString(fmt.Sprintf("enable-serve-source = %s\n", boolStr(cfg.EnableServeSource)))
	sb.WriteString("\n")
	sb.WriteString("# 是否启用 Firefox FTP 源\n")
	sb.WriteString(fmt.Sprintf("enable-firefox-ftp = %s\n", boolStr(cfg.EnableFirefoxFTP)))
	sb.WriteString("\n")

	// [network]
	sb.WriteString("[network]\n")
	sb.WriteString("\n")
	sb.WriteString("# 代理服务器地址\n")
	sb.WriteString("# 支持: http://host:port, socks5://host:port\n")
	sb.WriteString("# 留空 = 直接连接\n")
	sb.WriteString(fmt.Sprintf("proxy = %s\n", cfg.Proxy))
	sb.WriteString("\n")
	sb.WriteString("# 磁盘空间阈值（GB）\n")
	sb.WriteString("# 低于此值时发出警告\n")
	sb.WriteString(fmt.Sprintf("disk-space-threshold-gb = %d\n", cfg.DiskSpaceThresholdGB))
	sb.WriteString("\n")

	// Sources
	if len(cfg.Sources) > 0 {
		// Sort browsers for stable output.
		var browsers []string
		for b := range cfg.Sources {
			browsers = append(browsers, b)
		}
		sort.Strings(browsers)
		for _, browser := range browsers {
			for i, src := range cfg.Sources[browser] {
				sb.WriteString(fmt.Sprintf("[source.%s.%d]\n", browser, i))
				sb.WriteString(fmt.Sprintf("name = %s\n", src.Name))
				sb.WriteString(fmt.Sprintf("type = %s\n", src.Type))
				sb.WriteString(fmt.Sprintf("base-url = %s\n", src.BaseURL))
				sb.WriteString(fmt.Sprintf("enabled = %s\n", boolStr(src.Enabled)))
				sb.WriteString(fmt.Sprintf("priority = %d\n", src.Priority))
				sb.WriteString("\n")
			}
		}
	}

	cfg.mu.RUnlock()

	// Ensure directory exists.
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// parseDurations parses duration strings from the serialized form.
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
	// Log config defaults
	if c.Log.ConsoleLevel == "" {
		c.Log.ConsoleLevel = def.Log.ConsoleLevel
	}
	if c.Log.FileLevel == "" {
		c.Log.FileLevel = def.Log.FileLevel
	}
	if c.Log.MaxSizeMB == 0 {
		c.Log.MaxSizeMB = def.Log.MaxSizeMB
	}
	if c.Log.MaxBackups == 0 {
		c.Log.MaxBackups = def.Log.MaxBackups
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RepoPath = path
}

// GetRepoPath returns the configured repository path.
func (c *Config) GetRepoPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RepoPath
}

// GetRemoteSource returns the configured remote source URL.
// Returns empty string if not configured.
func (c *Config) GetRemoteSource() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RemoteSource
}

// SetRemoteSource sets the remote source URL.
func (c *Config) SetRemoteSource(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RemoteSource = url
}

// ClearRemoteSource clears the remote source configuration.
func (c *Config) ClearRemoteSource() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RemoteSource = ""
}

// --- Source switches ---

func (c *Config) IsServeSourceEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.EnableServeSource
}
func (c *Config) SetServeSourceEnabled(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.EnableServeSource = v
}
func (c *Config) IsFirefoxFTPEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.EnableFirefoxFTP
}
func (c *Config) SetFirefoxFTPEnabled(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.EnableFirefoxFTP = v
}

func (c *Config) GetDiskSpaceThresholdGB() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.DiskSpaceThresholdGB <= 0 {
		return 5
	}
	return c.DiskSpaceThresholdGB
}
func (c *Config) SetDiskSpaceThresholdGB(v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DiskSpaceThresholdGB = v
}

// GetProxy returns the configured proxy URL.
// Returns empty string if no proxy is configured.
func (c *Config) GetProxy() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Proxy
}

// SetProxy sets the proxy URL.
// Pass empty string to clear the proxy.
func (c *Config) SetProxy(proxy string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Proxy = proxy
}

// GetLanguage returns the configured UI language.
func (c *Config) GetLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Language
}

// SetLanguage sets the UI language. Supported: "zh", "en".
func (c *Config) SetLanguage(lang string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Language = lang
}

// GetSources returns enabled source configs for the given browser, sorted by priority.
func (c *Config) GetSources(browser string) []SourceConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
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

// --- Helpers ---

// GetAlias returns the alias value for the given name.
func (c *Config) GetAlias(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.Aliases[name]
	return v, ok
}

// AddAlias adds or updates an alias.
func (c *Config) AddAlias(name, target string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Aliases == nil {
		c.Aliases = make(map[string]string)
	}
	c.Aliases[name] = target
}

// RemoveAlias removes an alias by name.
func (c *Config) RemoveAlias(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Aliases, name)
}

// ListAliases returns a copy of all aliases.
func (c *Config) ListAliases() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]string, len(c.Aliases))
	for k, v := range c.Aliases {
		result[k] = v
	}
	return result
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

