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

	// BaseDir is the base directory containing packages/ and bin/.
	BaseDir string

	// SyncEnabled controls whether auto-sync is enabled.
	SyncEnabled bool

	// SyncInterval is how often to run sync (e.g. "24h", "30d").
	SyncInterval string

	// SyncBrowsers is the list of browsers to sync (comma-separated).
	SyncBrowsers string

	// SyncChannels is the list of channels to sync (comma-separated).
	SyncChannels string
}

// DefaultServeConfig returns the default serve configuration.
func DefaultServeConfig() ServeConfig {
	return ServeConfig{
		Host:         "0.0.0.0",
		Port:         "8080",
		BaseDir:      "",
		SyncEnabled:  false,
		SyncInterval: "24h",
		SyncBrowsers: "",
		SyncChannels: "stable",
	}
}

// ConfigPath returns the path to the bws-serve.ini config file.
// It's located in the program directory (next to packages/ and bin/).
func ConfigPath(baseDir string) string {
	if baseDir == "" {
		dir, err := paths.ExeDir()
		if err == nil {
			baseDir = dir
		} else {
			wd, _ := os.Getwd()
			baseDir = wd
		}
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
		case "base-dir", "basedir", "dir":
			cfg.BaseDir = value
		case "sync", "sync-enabled", "syncenabled":
			cfg.SyncEnabled = parseBool(value)
		case "sync-interval", "syncinterval", "schedule":
			cfg.SyncInterval = value
		case "sync-browsers", "syncbrowsers":
			cfg.SyncBrowsers = value
		case "sync-channels", "syncchannels":
			cfg.SyncChannels = value
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
	sb.WriteString("# 位置: bws-serve.ini（与 bws.exe 同目录，或通过 -d 指定）\n")
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
	sb.WriteString("# 数据存储目录\n")
	sb.WriteString("# serve 会在此目录下自动创建 packages/ 和 bin/ 子目录\n")
	sb.WriteString("# 留空 = 使用程序所在目录\n")
	sb.WriteString("# 支持相对路径和绝对路径，支持多级子目录\n")
	sb.WriteString("# 示例: D:\\bws-data  或  ..\\data\\browsers\n")
	sb.WriteString(fmt.Sprintf("base-dir = %s\n", cfg.BaseDir))
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
	case "base-dir", "basedir", "dir":
		absPath, err := filepath.Abs(value)
		if err == nil {
			cfg.BaseDir = absPath
		} else {
			cfg.BaseDir = value
		}
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
	case "base-dir", "basedir", "dir":
		return cfg.BaseDir, nil
	case "sync", "sync-enabled", "syncenabled":
		return boolStr(cfg.SyncEnabled), nil
	case "sync-interval", "syncinterval", "schedule":
		return cfg.SyncInterval, nil
	case "sync-browsers", "syncbrowsers":
		return cfg.SyncBrowsers, nil
	case "sync-channels", "syncchannels":
		return cfg.SyncChannels, nil
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
