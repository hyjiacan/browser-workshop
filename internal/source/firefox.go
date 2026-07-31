package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
)

const (
	// firefoxFTPBaseURL 是 Mozilla 官方版本归档的 HTTPS 目录列表地址。
	// 虽然域名前缀为 ftp，但 FTP 协议（端口21）已被 Mozilla 关闭，
	// 现在仅提供 HTTPS 访问，返回 HTML 格式的目录列表。
	firefoxFTPBaseURL = "https://ftp.mozilla.org/pub/firefox/releases/"
	// firefoxFTPLang 是下载时使用的默认语言。
	firefoxFTPLang = "en-US"
)

// versionDirRegex 匹配有效的 Firefox 版本目录名。
// 有效示例: 141.0, 141.0.1, 141.0b1, 140.0esr, 128.10.0esr
// 无效示例: 13.0.1-funnelcake11, 1.0rc1, 3.0.16-real, 1.cdn_test
var versionDirRegex = regexp.MustCompile(`^\d+\.\d+(\.\d+)*(esr|b\d+|a\d+)?$`)

// dirLinkRegex 匹配 HTML 目录列表中的 <a href="..."> 链接。
var dirLinkRegex = regexp.MustCompile(`<a href="([^"]*)">([^<]*)</a>`)

// FirefoxSource 通过解析 ftp.mozilla.org 的 HTTPS 目录列表
// 提供 Firefox 版本数据。
//
// 目录结构:
//   /pub/firefox/releases/
//     ├── 141.0/                    (版本目录)
//     │   ├── win64/                (平台目录)
//     │   │   └── en-US/            (语言目录)
//     │   │       ├── Firefox Setup 141.0.exe
//     │   │       └── Firefox Setup 141.0.msi
//     │   ├── linux-x86_64/
//     │   │   └── en-US/
//     │   │       ├── firefox-141.0.tar.xz
//     │   │       └── firefox-141.0.deb
//     │   └── mac/
//     │       └── en-US/
//     │           ├── Firefox 141.0.dmg
//     │           └── Firefox 141.0.pkg
//     ├── 141.0b1/                  (Beta 版本)
//     └── 140.0esr/                 (ESR 版本)
type FirefoxSource struct {
	baseURL      string
	httpClient   *http.Client
	cacheDir     string        // 缓存文件所在目录（来自 paths.ManifestCacheDir）
	cacheTTL     time.Duration // 缓存有效期（默认 24 小时）
	forceRefresh bool          // 为 true 时跳过缓存，强制从网络抓取
	mu           sync.Mutex    // 保护缓存文件访问及 forceRefresh 标志
}

// NewFirefoxSource creates a new FirefoxSource.
func NewFirefoxSource() *FirefoxSource {
	return NewFirefoxSourceWithProxy("")
}

// NewFirefoxSourceWithProxy creates a new FirefoxSource that uses the given proxy.
func NewFirefoxSourceWithProxy(proxyURL string) *FirefoxSource {
	return NewFirefoxSourceWithOptions(proxyURL, paths.Default().ManifestCacheDir, 24*time.Hour)
}

// NewFirefoxSourceWithOptions creates a new FirefoxSource with explicit cache
// configuration. cacheDir is the directory where the cache file is stored;
// cacheTTL is the cache time-to-live. Pass an empty cacheDir to disable caching.
func NewFirefoxSourceWithOptions(proxyURL, cacheDir string, cacheTTL time.Duration) *FirefoxSource {
	return &FirefoxSource{
		baseURL:    firefoxFTPBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second, Transport: newTransportWithProxy(proxyURL)},
		cacheDir:   cacheDir,
		cacheTTL:   cacheTTL,
	}
}

// SetForceRefresh sets whether the next List() call should bypass the cache
// and fetch fresh data from the network.
func (s *FirefoxSource) SetForceRefresh(force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forceRefresh = force
}

// Name returns the name of this source.
func (s *FirefoxSource) Name() string {
	return "firefox-ftp"
}

// SupportsBrowser reports whether this source supports the given browser.
func (s *FirefoxSource) SupportsBrowser(browser string) bool {
	return strings.ToLower(browser) == "firefox"
}

// List 返回所有匹配过滤条件的 Firefox 版本。
// 它抓取 ftp.mozilla.org 顶层目录列表，解析版本目录名，
// 并根据版本名中的后缀(esr/b)判断渠道。
func (s *FirefoxSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	chLog := string(filter.Channel)
	if chLog == "" {
		chLog = "all"
	}
	bmlog.Debug("[firefox-ftp] 查询版本: browser=firefox channel=%s platform=%s arch=%s",
		chLog, filter.Platform, filter.Arch)

	// 抓取顶层目录列表（带缓存）
	dirs, err := s.fetchDirsWithCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("抓取 Firefox 版本目录失败: %w", err)
	}

	plat := filter.Platform
	if plat == "" {
		plat = CurrentPlatform()
	}
	arch := filter.Arch
	if arch == "" {
		arch = CurrentArch()
	}

	// 构建所有版本列表（不做过滤），交由共享的 FilterVersions 统一处理
	var allVersions []VersionInfo
	for _, dir := range dirs {
		ver := strings.TrimSuffix(dir, "/")
		if !versionDirRegex.MatchString(ver) {
			continue
		}
		channel := classifyFirefoxChannel(ver)
		allVersions = append(allVersions, VersionInfo{
			Browser:     "firefox",
			Version:     ver,
			Channel:     channel,
			Platform:    plat,
			Arch:        arch,
			DownloadURL: s.buildDownloadURL(ver, plat, arch),
		})
	}

	// 使用共享过滤逻辑
	results := FilterVersions(allVersions, filter)

	// 按版本号降序排序
	sort.Slice(results, func(i, j int) bool {
		return compareVersions(results[i].Version, results[j].Version) > 0
	})

	bmlog.Debug("[firefox-ftp] 返回 %d 个匹配版本", len(results))
	return results, nil
}

// Latest 返回匹配过滤条件的最新 Firefox 版本。
func (s *FirefoxSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	filter = applyDefaults(filter)

	channel := filter.Channel
	if channel == "" {
		channel = ChannelStable
	}

	versions, err := s.List(ctx, &Filter{
		Browser:  "firefox",
		Channel:  channel,
		Platform: filter.Platform,
		Arch:     filter.Arch,
	})
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("未找到 firefox %s 版本", channel)
	}

	return versions[0], nil
}

// Resolve 查找指定版本的 Firefox。
// 支持别名(latest/beta/esr)、精确匹配和前缀匹配。
func (s *FirefoxSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	// 处理别名
	if version == "latest" || version == "" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch})
	}
	if version == "beta" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelBeta})
	}
	if version == "esr" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelESR})
	}
	// devedition/dev 在 FTP releases 目录中没有单独的渠道，回退到 beta
	if version == "devedition" || version == "dev" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelBeta})
	}
	// nightly 构建不在 releases 目录下，在 /pub/firefox/nightly/ 目录
	if version == "nightly" {
		return VersionInfo{}, fmt.Errorf("nightly 版本不在 releases 目录中，请使用 /pub/firefox/nightly/ 目录")
	}

	list, err := s.List(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch})
	if err != nil {
		return VersionInfo{}, err
	}

	// 精确匹配
	for _, v := range list {
		if v.Version == version {
			return v, nil
		}
	}

	// 前缀匹配 — 收集所有匹配项并返回最高版本。
	// 使用 "." 分隔符避免 "12" 匹配 "120.x"。
	prefix := version + "."
	var matches []VersionInfo
	for _, v := range list {
		if strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}
	if len(matches) > 0 {
		latest := matches[0]
		for _, v := range matches[1:] {
			if compareVersions(v.Version, latest.Version) > 0 {
				latest = v
			}
		}
		return latest, nil
	}

	return VersionInfo{}, fmt.Errorf("未找到 firefox 版本 %s", version)
}

// --- internal helpers ---

// firefoxCacheEntry 是缓存文件的 JSON 结构。
type firefoxCacheEntry struct {
	FetchedAt time.Time `json:"fetched_at"`
	Dirs      []string  `json:"dirs"`
}

// cachePath 返回 Firefox FTP 缓存文件的完整路径。
func (s *FirefoxSource) cachePath() string {
	return filepath.Join(s.cacheDir, "firefox-ftp-cache.json")
}

// loadCache 读取缓存文件，返回缓存的目录条目及抓取时间。
func (s *FirefoxSource) loadCache() ([]string, time.Time, error) {
	data, err := os.ReadFile(s.cachePath())
	if err != nil {
		return nil, time.Time{}, err
	}
	var entry firefoxCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, time.Time{}, err
	}
	return entry.Dirs, entry.FetchedAt, nil
}

// saveCache 将目录条目写入缓存文件。
func (s *FirefoxSource) saveCache(dirs []string) error {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return err
	}
	entry := firefoxCacheEntry{
		FetchedAt: time.Now().UTC(),
		Dirs:      dirs,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.cachePath(), data, 0o644)
}

// isCacheExpired 判断缓存是否已超过 TTL。
func (s *FirefoxSource) isCacheExpired(fetchedAt time.Time) bool {
	return time.Since(fetchedAt) > s.cacheTTL
}

// fetchDirsWithCache 获取目录列表，支持缓存、强制刷新和网络失败回退。
//
// 流程:
//  1. 如果 forceRefresh 为 true，跳过缓存直接从网络抓取
//  2. 尝试读取缓存——如果存在且未过期，使用缓存数据
//  3. 缓存未命中或已过期时，从网络抓取新数据
//  4. 网络抓取失败但缓存存在（即使已过期）时，使用缓存作为回退
//  5. 抓取成功后更新缓存文件
//  6. 抓取完成后重置 forceRefresh 标志
func (s *FirefoxSource) fetchDirsWithCache(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	force := s.forceRefresh
	s.mu.Unlock()

	// 如果未配置缓存目录，直接从网络抓取（不缓存）。
	// 这也覆盖了测试中直接构造 FirefoxSource 的场景。
	if s.cacheDir == "" {
		dirs, err := s.fetchDirectoryListing(ctx, s.baseURL)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.forceRefresh = false
		s.mu.Unlock()
		return dirs, nil
	}

	// 读取缓存（加锁保护文件访问）
	s.mu.Lock()
	cachedDirs, fetchedAt, cacheErr := s.loadCache()
	s.mu.Unlock()
	cacheValid := cacheErr == nil && len(cachedDirs) > 0

	// 缓存有效且未过期且未强制刷新 → 直接使用缓存
	if !force && cacheValid && !s.isCacheExpired(fetchedAt) {
		return cachedDirs, nil
	}

	// 缓存未命中、已过期或强制刷新 → 从网络抓取
	dirs, fetchErr := s.fetchDirectoryListing(ctx, s.baseURL)
	if fetchErr != nil {
		// 网络抓取失败——如果缓存存在（即使已过期），作为回退使用
		if cacheValid {
			return cachedDirs, nil
		}
		return nil, fetchErr
	}

	// 抓取成功，更新缓存并重置 forceRefresh 标志
	s.mu.Lock()
	_ = s.saveCache(dirs)
	s.forceRefresh = false
	s.mu.Unlock()

	return dirs, nil
}

// fetchDirectoryListing 抓取 HTTPS 目录列表页面并提取目录名。
// 仅返回以 "/" 结尾的条目（即目录），跳过父目录链接 ".."。
func (s *FirefoxSource) fetchDirectoryListing(ctx context.Context, pageURL string) ([]string, error) {
	bmlog.Debug("[firefox-ftp] 请求目录列表: %s", pageURL)

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		bmlog.Debug("[firefox-ftp] 请求失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	bmlog.Debug("[firefox-ftp] 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("目录列表返回状态码 %d", resp.StatusCode)
	}

	// 限制响应体大小为 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	dirs := parseDirectoryEntries(string(body))
	bmlog.Debug("[firefox-ftp] 解析到 %d 个目录条目", len(dirs))
	return dirs, nil
}

// parseDirectoryEntries 从 HTML 目录列表中提取目录名。
// 仅返回以 "/" 结尾的条目，跳过父目录链接。
func parseDirectoryEntries(html string) []string {
	matches := dirLinkRegex.FindAllStringSubmatch(html, -1)
	var dirs []string
	for _, m := range matches {
		name := m[2]
		// 跳过父目录
		if name == ".." || name == "../" {
			continue
		}
		// 仅包含目录（以 / 结尾）
		if strings.HasSuffix(name, "/") {
			dirs = append(dirs, name)
		}
	}
	return dirs
}

// classifyFirefoxChannel 根据版本字符串判断渠道。
// "esr" 后缀 → ESR，"b" + 数字 → Beta，其余 → Stable。
func classifyFirefoxChannel(version string) Channel {
	lower := strings.ToLower(version)
	if strings.Contains(lower, "esr") {
		return ChannelESR
	}
	// 检查是否为 beta 版本（如 "141.0b1"）
	if idx := strings.IndexByte(lower, 'b'); idx > 0 {
		if idx+1 < len(lower) && lower[idx+1] >= '0' && lower[idx+1] <= '9' {
			return ChannelBeta
		}
	}
	return ChannelStable
}

// buildDownloadURL 构造 Firefox 版本的直接下载 URL。
// URL 格式: https://ftp.mozilla.org/pub/firefox/releases/<version>/<platform>/en-US/<filename>
func (s *FirefoxSource) buildDownloadURL(version string, platform Platform, arch Arch) string {
	platDir := mapToMozillaFTPPlatform(platform, arch)
	filename := buildFirefoxFilename(version, platform)

	// 对文件名进行 URL 编码（空格 → %20）
	encodedFilename := url.PathEscape(filename)

	return fmt.Sprintf("%s%s/%s/%s/%s",
		s.baseURL, version, platDir, firefoxFTPLang, encodedFilename)
}

// mapToMozillaFTPPlatform 将内部平台/架构映射为 Mozilla FTP 目录名。
func mapToMozillaFTPPlatform(platform Platform, arch Arch) string {
	switch platform {
	case PlatformWindows:
		switch arch {
		case Arch386:
			return "win32"
		case ArchARM64:
			return "win64-aarch64"
		default:
			return "win64"
		}
	case PlatformMacOS:
		return "mac"
	case PlatformLinux:
		switch arch {
		case Arch386:
			return "linux-i686"
		case ArchARM64:
			return "linux-aarch64"
		default:
			return "linux-x86_64"
		}
	default:
		return "win64"
	}
}

// buildFirefoxFilename 根据版本和平台构造安装包文件名。
// Windows: Firefox Setup <version>.exe
// macOS:   Firefox <version>.dmg
// Linux:   firefox-<version>.tar.xz
func buildFirefoxFilename(version string, platform Platform) string {
	switch platform {
	case PlatformWindows:
		return fmt.Sprintf("Firefox Setup %s.exe", version)
	case PlatformMacOS:
		return fmt.Sprintf("Firefox %s.dmg", version)
	case PlatformLinux:
		return fmt.Sprintf("firefox-%s.tar.xz", version)
	default:
		return fmt.Sprintf("Firefox Setup %s.exe", version)
	}
}
