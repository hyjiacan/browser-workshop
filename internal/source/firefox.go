package source

import (
	"bufio"
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
//
//	/pub/firefox/releases/
//	  ├── 141.0/                    (版本目录)
//	  │   ├── win64/                (平台目录)
//	  │   │   └── en-US/            (语言目录)
//	  │   │       ├── Firefox Setup 141.0.exe
//	  │   │       └── Firefox Setup 141.0.msi
//	  │   ├── linux-x86_64/
//	  │   │   └── en-US/
//	  │   │       ├── firefox-141.0.tar.xz
//	  │   │       └── firefox-141.0.deb
//	  │   └── mac/
//	  │       └── en-US/
//	  │           ├── Firefox 141.0.dmg
//	  │           └── Firefox 141.0.pkg
//	  ├── 141.0b1/                  (Beta 版本)
//	  └── 140.0esr/                 (ESR 版本)
type FirefoxSource struct {
	baseURL      string
	httpClient   *http.Client
	cacheDir     string        // 缓存文件所在目录（来自 paths.ManifestCacheDir）
	cacheTTL     time.Duration // 缓存有效期（默认 24 小时）
	forceRefresh bool          // 为 true 时跳过缓存，强制从网络抓取
	mu           sync.Mutex    // 保护缓存文件访问及 forceRefresh 标志

	// urlCache 缓存已解析的下载 URL 和 SHA256，避免对同一版本重复抓取目录列表。
	// key 格式: "<version>|<platform>|<arch>"
	urlCache   map[string]*urlCacheEntry
	urlCacheMu sync.RWMutex
}

// urlCacheEntry 缓存 ResolveDownloadURL 的结果。
type urlCacheEntry struct {
	url    string
	sha256 string
	size   int64
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
// 找到匹配版本后，会调用 ResolveDownloadURL 获取实际下载地址和 SHA256 校验和。
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
			return s.enrichVersionInfo(ctx, v)
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
		return s.enrichVersionInfo(ctx, latest)
	}

	return VersionInfo{}, fmt.Errorf("未找到 firefox 版本 %s", version)
}

// enrichVersionInfo 使用 ResolveDownloadURL 更新 VersionInfo 的 DownloadURL 和 SHA256 字段。
// 如果解析失败，保留原始（基于模式构造的）URL，不返回错误。
func (s *FirefoxSource) enrichVersionInfo(ctx context.Context, v VersionInfo) (VersionInfo, error) {
	dlURL, sha256, err := s.ResolveDownloadURL(ctx, v.Version, v.Platform, v.Arch)
	if err != nil {
		// 解析失败时保留原始 URL，不阻断流程
		bmlog.Debug("[firefox-ftp] 解析下载 URL 失败，保留模式构造的 URL: %v", err)
		return v, nil
	}
	v.DownloadURL = dlURL
	v.SHA256 = sha256
	return v, nil
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

// parseFileEntries 从 HTML 目录列表中提取文件名（非目录条目）。
// 返回不以 "/" 结尾的条目，跳过父目录链接。
func parseFileEntries(html string) []string {
	matches := dirLinkRegex.FindAllStringSubmatch(html, -1)
	var files []string
	for _, m := range matches {
		name := m[2]
		// 跳过父目录
		if name == ".." || name == "../" {
			continue
		}
		// 跳过目录（以 / 结尾）
		if strings.HasSuffix(name, "/") {
			continue
		}
		files = append(files, name)
	}
	return files
}

// fetchFileEntries 抓取 HTTPS 目录列表页面并提取文件名（非目录条目）。
// 与 fetchDirectoryListing 不同，此方法返回文件而非目录。
func (s *FirefoxSource) fetchFileEntries(ctx context.Context, pageURL string) ([]string, error) {
	bmlog.Debug("[firefox-ftp] 请求文件列表: %s", pageURL)

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
		return nil, fmt.Errorf("文件列表返回状态码 %d", resp.StatusCode)
	}

	// 限制响应体大小为 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	files := parseFileEntries(string(body))
	bmlog.Debug("[firefox-ftp] 解析到 %d 个文件条目", len(files))
	return files, nil
}

// fetchFileListing 获取指定版本/平台/语言目录下的文件列表。
// platDir 是 Mozilla FTP 平台目录名（如 "linux-x86_64"、"win64"）。
// lang 是语言代码（如 "en-US"）。
func (s *FirefoxSource) fetchFileListing(ctx context.Context, version, platDir, lang string) ([]string, error) {
	pageURL := fmt.Sprintf("%s%s/%s/%s/", s.baseURL, version, platDir, lang)
	return s.fetchFileEntries(ctx, pageURL)
}

// fetchSHA256Sums 获取指定版本的 SHA256SUMS 文件并解析为 path→hash 映射。
// SHA256SUMS 文件格式: 每行 "<64字符十六进制哈希>  <相对路径>\n"
// 相对路径示例: "linux-x86_64/en-US/firefox-68.9.0esr.tar.bz2"
// 如果文件不存在（旧版本可能没有），返回空映射和 nil 错误。
func (s *FirefoxSource) fetchSHA256Sums(ctx context.Context, version string) (map[string]string, error) {
	sumsURL := fmt.Sprintf("%s%s/SHA256SUMS", s.baseURL, version)
	bmlog.Debug("[firefox-ftp] 请求 SHA256SUMS: %s", sumsURL)

	req, err := http.NewRequestWithContext(ctx, "GET", sumsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		bmlog.Debug("[firefox-ftp] 请求 SHA256SUMS 失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// 旧版本可能没有 SHA256SUMS 文件，优雅处理
		bmlog.Debug("[firefox-ftp] SHA256SUMS 不存在 (版本 %s)", version)
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SHA256SUMS 返回状态码 %d", resp.StatusCode)
	}

	// 限制响应体大小为 1MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	sums := parseSHA256Sums(string(body))
	bmlog.Debug("[firefox-ftp] SHA256SUMS 解析到 %d 个条目 (版本 %s)", len(sums), version)
	return sums, nil
}

// parseSHA256Sums 解析 SHA256SUMS 文件内容。
// 每行格式: "<64字符十六进制哈希>  <相对路径>"
// 路径可能包含空格（如 "win64/en-US/Firefox Setup 141.0.exe"），
// 因此不能简单使用 strings.Fields 分割。
// 返回 path→hash 映射，path 保持原始大小写。
func parseSHA256Sums(content string) map[string]string {
	sums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过空行和注释
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 格式: <hash>  <path> （两个或多个空格分隔）
		// 哈希固定为 64 个十六进制字符，之后是空白符，剩余部分为路径
		if len(line) < 66 { // 至少 64 hash + 1 space + 1 path
			continue
		}
		hash := line[:64]
		// 验证哈希长度（SHA256 = 64 个十六进制字符）
		if !isHexHash(hash) {
			continue
		}
		// 跳过哈希后的空白字符，剩余部分为路径
		path := strings.TrimLeft(line[64:], " \t")
		if path == "" {
			continue
		}
		sums[path] = hash
	}
	return sums
}

// isHexHash 检查字符串是否为 64 个十六进制字符（SHA256 哈希）。
func isHexHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ResolveDownloadURL 解析指定版本的实际下载 URL 和 SHA256 哈希。
// 它从 FTP 目录列表获取真实文件名（而非依赖硬编码模式），
// 并从 SHA256SUMS 文件获取对应的校验和。
//
// 匹配规则:
//   - Linux: firefox-{version}.tar.* （匹配 .tar.xz 和 .tar.bz2）
//   - Windows: Firefox Setup {version}.exe 或 .msi
//   - macOS: Firefox {version}.dmg
//
// 如果无法获取目录列表或找不到匹配文件，回退到 buildDownloadURL 的模式构造。
// 如果 SHA256SUMS 不可用，sha256 返回空字符串。
func (s *FirefoxSource) ResolveDownloadURL(ctx context.Context, version string, platform Platform, arch Arch) (dlURL, sha256 string, err error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s|%s|%s", version, platform, arch)
	s.urlCacheMu.RLock()
	if s.urlCache != nil {
		if entry, ok := s.urlCache[cacheKey]; ok {
			s.urlCacheMu.RUnlock()
			bmlog.Debug("[firefox-ftp] URL 缓存命中: %s", cacheKey)
			return entry.url, entry.sha256, nil
		}
	}
	s.urlCacheMu.RUnlock()

	platDir := mapToMozillaFTPPlatform(platform, arch)

	// 尝试从目录列表获取实际文件名
	files, listErr := s.fetchFileListing(ctx, version, platDir, firefoxFTPLang)

	var actualFilename string
	if listErr != nil {
		bmlog.Debug("[firefox-ftp] 获取文件列表失败，回退到模式构造: %v", listErr)
		actualFilename = buildFirefoxFilename(version, platform)
	} else {
		actualFilename = findMatchingFile(files, version, platform)
		if actualFilename == "" {
			bmlog.Debug("[firefox-ftp] 未在目录列表中找到匹配文件，回退到模式构造")
			actualFilename = buildFirefoxFilename(version, platform)
		}
	}

	// 构造下载 URL
	encodedFilename := url.PathEscape(actualFilename)
	dlURL = fmt.Sprintf("%s%s/%s/%s/%s",
		s.baseURL, version, platDir, firefoxFTPLang, encodedFilename)

	// 尝试获取 SHA256
	sums, sumsErr := s.fetchSHA256Sums(ctx, version)
	if sumsErr != nil {
		bmlog.Debug("[firefox-ftp] 获取 SHA256SUMS 失败: %v", sumsErr)
	} else if len(sums) > 0 {
		// SHA256SUMS 中的路径格式: <platDir>/<lang>/<filename>
		sumsPath := fmt.Sprintf("%s/%s/%s", platDir, firefoxFTPLang, actualFilename)
		if h, ok := sums[sumsPath]; ok {
			sha256 = h
		} else {
			bmlog.Debug("[firefox-ftp] SHA256SUMS 中未找到路径: %s", sumsPath)
		}
	}

	// 写入缓存
	s.urlCacheMu.Lock()
	if s.urlCache == nil {
		s.urlCache = make(map[string]*urlCacheEntry)
	}
	s.urlCache[cacheKey] = &urlCacheEntry{
		url:    dlURL,
		sha256: sha256,
	}
	s.urlCacheMu.Unlock()

	return dlURL, sha256, nil
}

// findMatchingFile 从文件列表中查找匹配版本和平台的安装包文件名。
// 匹配规则:
//   - Linux: firefox-{version}.tar.* （优先 .tar.xz，其次 .tar.bz2）
//   - Windows: Firefox Setup {version}.exe （优先 .exe，其次 .msi）
//   - macOS: Firefox {version}.dmg
//
// 返回空字符串表示未找到匹配文件。
func findMatchingFile(files []string, version string, platform Platform) string {
	switch platform {
	case PlatformLinux:
		// 优先 .tar.xz，其次 .tar.bz2
		xzName := fmt.Sprintf("firefox-%s.tar.xz", version)
		bz2Name := fmt.Sprintf("firefox-%s.tar.bz2", version)
		for _, f := range files {
			if f == xzName {
				return f
			}
		}
		for _, f := range files {
			if f == bz2Name {
				return f
			}
		}
		// 通配匹配 firefox-{version}.tar.*
		prefix := fmt.Sprintf("firefox-%s.tar.", version)
		for _, f := range files {
			if strings.HasPrefix(f, prefix) {
				return f
			}
		}
	case PlatformWindows:
		// 优先 .exe，其次 .msi
		exeName := fmt.Sprintf("Firefox Setup %s.exe", version)
		msiName := fmt.Sprintf("Firefox Setup %s.msi", version)
		for _, f := range files {
			if f == exeName {
				return f
			}
		}
		for _, f := range files {
			if f == msiName {
				return f
			}
		}
	case PlatformMacOS:
		dmgName := fmt.Sprintf("Firefox %s.dmg", version)
		for _, f := range files {
			if f == dmgName {
				return f
			}
		}
	}
	return ""
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
// 注意: 这是一个基于已知命名模式的最佳猜测，并非从 FTP 目录列表获取的真实文件名。
// 对于旧版本，实际文件名可能不同（如 Linux 旧版使用 .tar.bz2 而非 .tar.xz）。
// 如需获取真实文件名，请使用 ResolveDownloadURL 方法。
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
