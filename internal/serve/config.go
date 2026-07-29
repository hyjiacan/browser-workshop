package serve

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bws/bws/internal/paths"
)

// ServeConfig holds the configuration for the serve command.
type ServeConfig struct {
	// Host is the listen host address.
	Host string

	// Port is the listen port.
	Port string

	// PackagesDir is the directory containing browser packages.
	PackagesDir string

	// BinDir is the directory containing client binary files.
	BinDir string

	// SyncEnabled controls whether auto-sync is enabled.
	SyncEnabled bool

	// SyncInterval is how often to run sync (e.g. "24h", "30d").
	SyncInterval string

	// SyncBrowsers is the list of browsers to sync (comma-separated).
	SyncBrowsers string

	// SyncChannels is the list of channels to sync (comma-separated).
	SyncChannels string

	// OnlineFallback controls whether serve fetches packages from the online
	// source in real-time when a requested file is not present locally.
	// When enabled, missing packages are downloaded on demand and served to
	// the client, and the manifest lists all online-available versions.
	OnlineFallback bool

	// LogLevel is the console log level for the serve module.
	// Valid values: trace, debug, info, warn, error, fatal.
	// Default: info
	LogLevel string

	// FileLogLevel is the file log level for the serve module.
	// Valid values: trace, debug, info, warn, error, fatal.
	// Default: debug
	FileLogLevel string

	// LogMaxSizeMB is the maximum size in MB for a single log file before rotation.
	// 0 means no size limit (infinite append).
	// Default: 10
	LogMaxSizeMB int

	// LogMaxBackups is the maximum number of backup log files to keep.
	// 0 means no backups are kept when rotating.
	// Default: 5
	LogMaxBackups int

	// ScanWorkers is the number of worker goroutines used for parallel
	// checksum computation during package scanning. 0 means auto (runtime.NumCPU()).
	// Values are clamped to the range [1, 32].
	// Default: 0 (auto)
	ScanWorkers int
}

// DefaultServeConfig returns the default serve configuration.
func DefaultServeConfig() ServeConfig {
	return ServeConfig{
		Host:           "0.0.0.0",
		Port:           "8080",
		PackagesDir:    "",
		BinDir:         "",
		SyncEnabled:    false,
		SyncInterval:   "24h",
		SyncBrowsers:   "",
		SyncChannels:   "stable",
		OnlineFallback: false,
		LogLevel:       "info",
		FileLogLevel:   "debug",
		LogMaxSizeMB:   10,
		LogMaxBackups:  5,
		ScanWorkers:    0,
	}
}

// ConfigPath returns the path to the bws-serve.ini config file.
// It's located in the bws-data directory (next to config.json and logs/).
// If baseDir is empty, the default bws-data directory is used.
func ConfigPath(baseDir string) string {
	if baseDir == "" {
		baseDir = paths.Default().Root
	}
	return filepath.Join(baseDir, "bws-serve.ini")
}

// LoadServeConfig loads the serve configuration from bws-serve.ini.
// If the file doesn't exist, it returns the default config.
func LoadServeConfig(baseDir string) (ServeConfig, error) {
	cfg := DefaultServeConfig()

	configPath := ConfigPath(baseDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return cfg, fmt.Errorf("opening config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}

		// Key=value pair
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(line[:eqIdx]))
		value := strings.TrimSpace(line[eqIdx+1:])
		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Only parse [serve] section for now
		if section != "serve" && section != "" {
			continue
		}

		switch key {
		case "host":
			cfg.Host = value
		case "port":
			cfg.Port = value
		case "packages-dir", "packagesdir":
			cfg.PackagesDir = value
		case "bin-dir", "bindir":
			cfg.BinDir = value
		case "sync", "sync-enabled", "syncenabled":
			cfg.SyncEnabled = parseBool(value)
		case "sync-interval", "syncinterval", "schedule":
			cfg.SyncInterval = value
		case "sync-browsers", "syncbrowsers":
			cfg.SyncBrowsers = value
		case "sync-channels", "syncchannels":
			cfg.SyncChannels = value
		case "online-fallback", "onlinefallback":
			cfg.OnlineFallback = parseBool(value)
		case "log-level", "loglevel":
			cfg.LogLevel = value
		case "file-log-level", "fileloglevel":
			cfg.FileLogLevel = value
		case "log-max-size-mb", "logmaxsizemb", "log-max-size":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.LogMaxSizeMB = v
			}
		case "log-max-backups", "logmaxbackups":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.LogMaxBackups = v
			}
		case "scan-workers", "scanworkers":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.ScanWorkers = v
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}

	return cfg, nil
}

// SaveServeConfig saves the serve configuration to bws-serve.ini.
func SaveServeConfig(baseDir string, cfg ServeConfig) error {
	configPath := ConfigPath(baseDir)

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Build the ini content with detailed comments
	var sb strings.Builder
	sb.WriteString("# ===============================================================\n")
	sb.WriteString("# bws serve 配置文件\n")
	sb.WriteString("# 位置: bws-data/bws-serve.ini（bws-data 为程序所在目录下的数据目录）\n")
	sb.WriteString("# ===============================================================\n")
	sb.WriteString("#\n")
	sb.WriteString("# 修改此文件后，重新运行 bws serve 即可生效。\n")
	sb.WriteString("#\n")
	sb.WriteString("\n")
	sb.WriteString("[serve]\n")
	sb.WriteString("\n")
	sb.WriteString("# 监听地址\n")
	sb.WriteString("# 0.0.0.0 = 监听所有网络接口（局域网内可访问）\n")
	sb.WriteString("# 127.0.0.1 = 仅本机访问\n")
	sb.WriteString(fmt.Sprintf("host = %s\n", cfg.Host))
	sb.WriteString("\n")
	sb.WriteString("# 监听端口\n")
	sb.WriteString(fmt.Sprintf("port = %s\n", cfg.Port))
	sb.WriteString("\n")
	sb.WriteString("# 浏览器安装包存放目录\n")
	sb.WriteString("# serve 将从此目录提供浏览器安装包的下载\n")
	sb.WriteString("# 留空 = 使用程序所在目录下的 packages 子目录\n")
	sb.WriteString("# 支持绝对路径和相对路径（相对于程序所在目录）\n")
	sb.WriteString("# 示例: D:\\bws-packages  或  ..\\shared\\packages\n")
	sb.WriteString(fmt.Sprintf("packages-dir = %s\n", cfg.PackagesDir))
	sb.WriteString("\n")
	sb.WriteString("# 客户端二进制文件存放目录\n")
	sb.WriteString("# serve 将从此目录提供 bws 客户端二进制的下载\n")
	sb.WriteString("# 留空 = 使用程序所在目录下的 bin 子目录\n")
	sb.WriteString("# 支持绝对路径和相对路径（相对于程序所在目录）\n")
	sb.WriteString("# 示例: D:\\bws-bin  或  ..\\shared\\bin\n")
	sb.WriteString(fmt.Sprintf("bin-dir = %s\n", cfg.BinDir))
	sb.WriteString("\n")
	sb.WriteString("# 自动同步开关\n")
	sb.WriteString("# true  = 启用定时同步，从在线源下载最新版本\n")
	sb.WriteString("# false = 仅使用本地 packages/ 中的文件\n")
	sb.WriteString(fmt.Sprintf("sync = %s\n", boolStr(cfg.SyncEnabled)))
	sb.WriteString("\n")
	sb.WriteString("# 同步间隔\n")
	sb.WriteString("# 支持格式: 30d（天）、24h（小时）、30m（分钟）、1h30m（组合）\n")
	sb.WriteString(fmt.Sprintf("sync-interval = %s\n", cfg.SyncInterval))
	sb.WriteString("\n")
	sb.WriteString("# 同步的浏览器列表（逗号分隔）\n")
	sb.WriteString("# 留空 = 同步所有支持的浏览器\n")
	sb.WriteString("# 可选值: chrome, firefox, chromium, edge\n")
	sb.WriteString("# 示例: chrome,firefox\n")
	sb.WriteString(fmt.Sprintf("sync-browsers = %s\n", cfg.SyncBrowsers))
	sb.WriteString("\n")
	sb.WriteString("# 同步的发布渠道（逗号分隔）\n")
	sb.WriteString("# 可选值: stable, beta, dev, canary, esr\n")
	sb.WriteString("# 默认: stable\n")
	sb.WriteString(fmt.Sprintf("sync-channels = %s\n", cfg.SyncChannels))
	sb.WriteString("\n")
	sb.WriteString("# 在线回退开关\n")
	sb.WriteString("# true  = 当请求的软件包在本地不存在时，自动从在线源实时下载并提供\n")
	sb.WriteString("#         同时清单（manifest）会列出在线源中所有可用的版本\n")
	sb.WriteString("# false = 仅提供本地 packages/ 中已有的文件，缺失时返回 404\n")
	sb.WriteString(fmt.Sprintf("online-fallback = %s\n", boolStr(cfg.OnlineFallback)))
	sb.WriteString("\n")
	sb.WriteString("# 日志级别（控制台输出）\n")
	sb.WriteString("# 可选值: trace, debug, info, warn, error, fatal\n")
	sb.WriteString("# 默认: info\n")
	sb.WriteString(fmt.Sprintf("log-level = %s\n", cfg.LogLevel))
	sb.WriteString("\n")
	sb.WriteString("# 文件日志级别\n")
	sb.WriteString("# 可选值: trace, debug, info, warn, error, fatal\n")
	sb.WriteString("# 默认: debug\n")
	sb.WriteString("# 文件日志记录到 logs/serve.log\n")
	sb.WriteString(fmt.Sprintf("file-log-level = %s\n", cfg.FileLogLevel))
	sb.WriteString("\n")
	sb.WriteString("# 单个日志文件最大大小（MB）\n")
	sb.WriteString("# 超过此大小时自动轮转，旧文件保存为 .1, .2, .3 等\n")
	sb.WriteString("# 0 = 不限制大小（无限追加）\n")
	sb.WriteString("# 默认: 10\n")
	sb.WriteString(fmt.Sprintf("log-max-size-mb = %d\n", cfg.LogMaxSizeMB))
	sb.WriteString("\n")
	sb.WriteString("# 保留的备份日志文件数量\n")
	sb.WriteString("# 轮转时保留的旧文件数量，超过此数量的最旧文件将被删除\n")
	sb.WriteString("# 0 = 不保留备份\n")
	sb.WriteString("# 默认: 5\n")
	sb.WriteString(fmt.Sprintf("log-max-backups = %d\n", cfg.LogMaxBackups))
	sb.WriteString("\n")
	sb.WriteString("# 扫描软件包时并行计算校验和的线程数\n")
	sb.WriteString("# 0 = 自动（根据 CPU 核心数自动设置）\n")
	sb.WriteString("# 有效范围: 1 到 32\n")
	sb.WriteString("# 默认: 0（自动）\n")
	sb.WriteString(fmt.Sprintf("scan-workers = %d\n", cfg.ScanWorkers))

	return os.WriteFile(configPath, []byte(sb.String()), 0o644)
}

// EnsureDefaultConfig creates a default bws-serve.ini if it doesn't exist.
// Returns the config path, whether it was newly created, and any error.
func EnsureDefaultConfig(baseDir string) (string, bool, error) {
	configPath := ConfigPath(baseDir)
	if _, err := os.Stat(configPath); err == nil {
		return configPath, false, nil
	}
	cfg := DefaultServeConfig()
	if err := SaveServeConfig(baseDir, cfg); err != nil {
		return configPath, false, err
	}
	return configPath, true, nil
}

// SetConfigKey sets a single configuration key and saves the config file.
// Returns the updated config.
func SetConfigKey(baseDir string, key string, value string) (ServeConfig, error) {
	cfg, err := LoadServeConfig(baseDir)
	if err != nil {
		return cfg, err
	}

	key = strings.ToLower(strings.TrimSpace(key))

	switch key {
	case "host":
		cfg.Host = value
	case "port":
		if _, err := strconv.Atoi(value); err != nil {
			return cfg, fmt.Errorf("无效的端口号: %s", value)
		}
		cfg.Port = value
	case "packages-dir", "packagesdir":
		cfg.PackagesDir = value
	case "bin-dir", "bindir":
		cfg.BinDir = value
	case "sync", "sync-enabled", "syncenabled":
		cfg.SyncEnabled = parseBool(value)
	case "sync-interval", "syncinterval", "schedule":
		// Validate duration format
		if _, err := parseDuration(value); err != nil {
			return cfg, fmt.Errorf("无效的时间间隔格式: %s（例如 24h、30m、7d）", value)
		}
		cfg.SyncInterval = value
	case "sync-browsers", "syncbrowsers":
		cfg.SyncBrowsers = value
	case "sync-channels", "syncchannels":
		cfg.SyncChannels = value
	case "online-fallback", "onlinefallback":
		cfg.OnlineFallback = parseBool(value)
	case "log-level", "loglevel":
		// Validate log level
		normalized := strings.ToLower(strings.TrimSpace(value))
		validLevels := map[string]bool{
			"trace": true, "debug": true, "info": true,
			"warn": true, "warning": true, "error": true, "fatal": true,
		}
		if !validLevels[normalized] {
			return cfg, fmt.Errorf("无效的日志级别: %s（可选值: trace, debug, info, warn, error, fatal）", value)
		}
		cfg.LogLevel = value
	case "file-log-level", "fileloglevel":
		// Validate file log level
		normalized := strings.ToLower(strings.TrimSpace(value))
		validLevels := map[string]bool{
			"trace": true, "debug": true, "info": true,
			"warn": true, "warning": true, "error": true, "fatal": true,
		}
		if !validLevels[normalized] {
			return cfg, fmt.Errorf("无效的文件日志级别: %s（可选值: trace, debug, info, warn, error, fatal）", value)
		}
		cfg.FileLogLevel = value
	case "log-max-size-mb", "logmaxsizemb", "log-max-size":
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 {
			return cfg, fmt.Errorf("无效的日志文件大小: %s（必须为非负整数，单位 MB）", value)
		}
		cfg.LogMaxSizeMB = v
	case "log-max-backups", "logmaxbackups":
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 {
			return cfg, fmt.Errorf("无效的备份文件数量: %s（必须为非负整数）", value)
		}
		cfg.LogMaxBackups = v
	case "scan-workers", "scanworkers":
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 || v > 32 {
			return cfg, fmt.Errorf("无效的扫描线程数: %s（必须为 0 到 32 之间的整数，0 表示自动）", value)
		}
		cfg.ScanWorkers = v
	default:
		return cfg, fmt.Errorf("未知的配置项: %s", key)
	}

	if err := SaveServeConfig(baseDir, cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// GetConfigKey gets the value of a single configuration key.
func GetConfigKey(baseDir string, key string) (string, error) {
	cfg, err := LoadServeConfig(baseDir)
	if err != nil {
		return "", err
	}

	key = strings.ToLower(strings.TrimSpace(key))

	switch key {
	case "host":
		return cfg.Host, nil
	case "port":
		return cfg.Port, nil
	case "packages-dir", "packagesdir":
		return cfg.PackagesDir, nil
	case "bin-dir", "bindir":
		return cfg.BinDir, nil
	case "sync", "sync-enabled", "syncenabled":
		return boolStr(cfg.SyncEnabled), nil
	case "sync-interval", "syncinterval", "schedule":
		return cfg.SyncInterval, nil
	case "sync-browsers", "syncbrowsers":
		return cfg.SyncBrowsers, nil
	case "sync-channels", "syncchannels":
		return cfg.SyncChannels, nil
	case "online-fallback", "onlinefallback":
		return boolStr(cfg.OnlineFallback), nil
	case "log-level", "loglevel":
		return cfg.LogLevel, nil
	case "file-log-level", "fileloglevel":
		return cfg.FileLogLevel, nil
	case "log-max-size-mb", "logmaxsizemb", "log-max-size":
		return strconv.Itoa(cfg.LogMaxSizeMB), nil
	case "log-max-backups", "logmaxbackups":
		return strconv.Itoa(cfg.LogMaxBackups), nil
	case "scan-workers", "scanworkers":
		return strconv.Itoa(cfg.ScanWorkers), nil
	default:
		return "", fmt.Errorf("未知的配置项: %s", key)
	}
}

// Addr returns the listen address (host:port).
func (c ServeConfig) Addr() string {
	host := c.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := c.Port
	if port == "" {
		port = "8080"
	}
	return host + ":" + port
}

// SyncDuration parses the sync interval and returns a time.Duration.
func (c ServeConfig) SyncDuration() (time.Duration, error) {
	return parseDuration(c.SyncInterval)
}

// SyncBrowsersList returns the sync browsers as a slice.
func (c ServeConfig) SyncBrowsersList() []string {
	if c.SyncBrowsers == "" {
		return nil
	}
	parts := strings.Split(c.SyncBrowsers, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// SyncChannelsList returns the sync channels as a slice.
func (c ServeConfig) SyncChannelsList() []string {
	if c.SyncChannels == "" {
		return []string{"stable"}
	}
	parts := strings.Split(c.SyncChannels, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{"stable"}
	}
	return result
}

// --- Helpers ---

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

// parseDuration parses a duration string, supporting "d" for days.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	// Handle days
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
