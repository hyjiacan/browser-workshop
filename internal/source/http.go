package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
)

// HTTPSource provides browser versions from a bm serve HTTP endpoint.
// It queries the manifest API of a bm serve instance using the API v1 format.
type HTTPSource struct {
	baseURL    string
	name       string
	httpClient *http.Client

	// manifestCache holds a short-lived cache of the manifest response so
	// that multiple List() calls within the same query session (e.g. when
	// the client iterates over channels) don't re-fetch the same 1.7 MB
	// payload 5 times. The cache is valid for manifestCacheTTL.
	manifestCache    *manifestV1Response
	manifestCacheAge time.Time
	// manifestFetchCh implements single-flight: when non-nil, a fetch is
	// in progress and concurrent callers should wait on it instead of
	// initiating their own HTTP request.
	manifestFetchCh chan struct{}
	manifestCacheMu sync.Mutex

	// forceRefresh, when true, causes the next fetchManifest to skip all
	// caches (memory + disk) and request serve to refresh its online cache
	// before merging. The flag is reset to false automatically after use.
	forceRefresh bool
}

// manifestCacheTTL is how long a cached manifest is considered fresh.
// 30 seconds is enough for a single query session (which typically
// completes in <5 seconds) while still allowing rapid re-queries.
const manifestCacheTTL = 30 * time.Second

// --- New API v1 types ---

// manifestV1Response is the JSON response from the /api/v1/manifest endpoint.
type manifestV1Response struct {
	Status string            `json:"status"`
	Data   []manifestV1File  `json:"data"`
	Server manifestV1Server  `json:"server"`
}

type manifestV1File struct {
	Filename         string `json:"filename"`
	Version          string `json:"version"`
	Browser          string `json:"browser"`
	Channel          string `json:"channel"`
	MajorVersion     string `json:"major_version"`
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	Size             int64  `json:"size"`
	Checksum         string `json:"checksum"`                    // locally computed XXH3 checksum
	UpstreamChecksum string `json:"upstream_checksum,omitempty"` // upstream checksum with algo prefix (e.g. "sha256:hash")
}

type manifestV1Server struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	FileCount int    `json:"file_count"`
}

// browserKeywords maps browser name keywords to canonical browser names.
// Used to detect browser name from filenames in the v1 API.
var browserKeywords = []struct {
	keyword string
	name    string
}{
	{"chrome", "chrome"},
	{"chromium", "chromium"},
	{"firefox", "firefox"},
	{"edge", "edge"},
	{"brave", "brave"},
	{"opera", "opera"},
	{"safari", "safari"},
	{"vivaldi", "vivaldi"},
	{"thorium", "thorium"},
	{"ungoogled", "ungoogled-chromium"},
}

// detectBrowserFromFilename extracts the browser name from a filename
// using keyword matching. Returns empty string if unrecognized.
func detectBrowserFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	for _, kw := range browserKeywords {
		if strings.Contains(lower, kw.keyword) {
			return kw.name
		}
	}
	return ""
}

// detectChannelFromFilename extracts the channel from a filename.
// 关键词匹配顺序很重要：更具体的关键词（canary、dev、beta、esr）
// 必须在更通用的 stable（默认分支）之前检查，以避免误检测。
// 当前顺序 canary -> dev -> beta -> esr -> stable 是正确的：
// - canary 最为特殊，不会与其他频道关键词冲突
// - dev 在 beta 之前检查，因为 dev 文件名不会包含 "beta"
// - esr 仅用于 Firefox，不会与 Chromium 系列冲突
func detectChannelFromFilename(filename string) Channel {
	lower := strings.ToLower(filename)
	switch {
	case strings.Contains(lower, "canary"):
		return ChannelCanary
	case strings.Contains(lower, "dev"):
		return ChannelDev
	case strings.Contains(lower, "beta"):
		return ChannelBeta
	case strings.Contains(lower, "esr"):
		return ChannelESR
	default:
		return ChannelStable
	}
}

// normalizePlatform normalizes platform name to our Platform type.
func normalizePlatform(p string) Platform {
	switch strings.ToLower(p) {
	case "windows", "win":
		return PlatformWindows
	case "darwin", "macos", "mac":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

// normalizeArch normalizes architecture name to our Arch type.
func normalizeArch(a string) Arch {
	switch strings.ToLower(a) {
	case "x64", "amd64", "x86_64", "win64", "64":
		return ArchAMD64
	case "x86", "386", "x86_32", "win32", "32":
		return Arch386
	case "arm64", "aarch64":
		return ArchARM64
	default:
		return ArchUnknown
	}
}

// NewHTTPSource creates a new HTTP source from a bm serve base URL.
func NewHTTPSource(baseURL string) *HTTPSource {
	return NewHTTPSourceWithProxy(baseURL, "")
}

// NewHTTPSourceWithProxy creates an HTTPSource that uses the given proxy.
// proxyURL can be empty (direct), "http://host:port", "socks5://host:port", etc.
func NewHTTPSourceWithProxy(baseURL string, proxyURL string) *HTTPSource {
	// Normalize base URL - remove trailing slash
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPSource{
		baseURL:    baseURL,
		name:       "http:" + baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second, Transport: newTransportWithProxy(proxyURL)},
	}
}

// Name returns the name of this source.
func (s *HTTPSource) Name() string {
	return s.name
}

// SetForceRefresh enables or disables force-refresh mode for the next manifest fetch.
// When enabled, the next fetchManifest call will skip all local caches (memory + disk)
// and instruct the serve instance to refresh its online cache via ?refresh=true.
// The flag is automatically reset to false after the fetch completes.
// This implements the refreshableSource interface used by MultiSource.ForceRefresh().
func (s *HTTPSource) SetForceRefresh(force bool) {
	s.manifestCacheMu.Lock()
	s.forceRefresh = force
	s.manifestCacheMu.Unlock()
}

// SupportsBrowser reports whether this source supports the given browser.
// HTTPSource is a generic serve endpoint that can host any browser type.
func (s *HTTPSource) SupportsBrowser(browser string) bool {
	return true // serve endpoint supports all browser types
}

// List returns all available versions matching the filter.
// It fetches the full manifest (all browsers/platforms/archs) from the server
// and filters client-side via FilterVersions.
func (s *HTTPSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	v1Manifest, err := s.fetchManifest(ctx)
	if err != nil {
		return nil, err
	}

	if v1Manifest == nil {
		return nil, nil
	}

	// Convert manifest entries to VersionInfo, then apply shared filtering.
	all := s.manifestToVersions(v1Manifest)
	return FilterVersions(all, filter), nil
}

// manifestToVersions converts the v1 manifest response into a slice of
// VersionInfo. It uses the explicit Browser/Channel fields from the manifest
// when available, falling back to filename-based detection for older serve
// instances that don't include those fields.
func (s *HTTPSource) manifestToVersions(m *manifestV1Response) []VersionInfo {
	var results []VersionInfo
	for _, f := range m.Data {
		// Use explicit browser field if present, otherwise detect from filename.
		browser := f.Browser
		if browser == "" {
			browser = detectBrowserFromFilename(f.Filename)
		}
		if browser == "" {
			continue
		}

		// Use explicit channel field if present, otherwise detect from filename.
		channel := detectChannelFromFilename(f.Filename)
		if f.Channel != "" {
			channel = Channel(f.Channel)
		}

		platform := normalizePlatform(f.Platform)
		arch := normalizeArch(f.Architecture)
		downloadURL := fmt.Sprintf("%s/api/v1/download/%s", s.baseURL, neturl.PathEscape(f.Filename))

		results = append(results, VersionInfo{
			Browser:     browser,
			Version:     f.Version,
			Channel:     channel,
			Platform:    platform,
			Arch:        arch,
			DownloadURL: downloadURL,
			Size:        f.Size,
			Checksum:    f.UpstreamChecksum,
		})
	}
	return results
}

// Latest returns the latest version matching the filter.
func (s *HTTPSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := s.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found matching the given criteria")
	}

	// Find the latest version with the highest version number
	latest := versions[0]
	for _, v := range versions[1:] {
		if compareVersions(v.Version, latest.Version) > 0 {
			latest = v
		}
	}
	return latest, nil
}

// Resolve finds a specific version by version string.
// Supports "latest" and partial version prefixes.
func (s *HTTPSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	browser = strings.ToLower(browser)

	// Handle "latest"
	if version == "" || strings.ToLower(version) == "latest" {
		return s.Latest(ctx, &Filter{
			Browser:  browser,
			Platform: platform,
			Arch:     arch,
		})
	}

	versions, err := s.List(ctx, &Filter{
		Browser:  browser,
		Platform: platform,
		Arch:     arch,
	})
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found for %s", browser)
	}

	// Exact match
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// Prefix match - find the latest matching version
	// Use "." separator to avoid "12" matching "120.x.x.x"
	prefix := version + "."
	var matches []VersionInfo
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return VersionInfo{}, fmt.Errorf("version %s not found for %s", version, browser)
	}

	// Return the highest matching version
	latest := matches[0]
	for _, v := range matches[1:] {
		if compareVersions(v.Version, latest.Version) > 0 {
			latest = v
		}
	}
	return latest, nil
}

// fetchManifest fetches the manifest from the server, using a single-flight
// pattern with a short-lived cache. When the client iterates over multiple
// channels (each triggering a separate List() call), only the first call
// performs the HTTP request; concurrent calls wait and reuse the result.
// The cache is valid for manifestCacheTTL (30s) so rapid re-queries also
// benefit.
func (s *HTTPSource) fetchManifest(ctx context.Context) (*manifestV1Response, error) {
	s.manifestCacheMu.Lock()

	// Fast path: in-memory cache hit (skip when force-refreshing).
	if !s.forceRefresh {
		if s.manifestCache != nil && time.Since(s.manifestCacheAge) < manifestCacheTTL {
			cached := s.manifestCache
			s.manifestCacheMu.Unlock()
			bmlog.Debug("[http-source] manifest 内存缓存命中 (条目数=%d, 缓存年龄=%v)",
				len(cached.Data), time.Since(s.manifestCacheAge).Round(time.Millisecond))
			return cached, nil
		}

		// Single-flight: if a fetch is already in progress, wait for it.
		if s.manifestFetchCh != nil {
			ch := s.manifestFetchCh
			s.manifestCacheMu.Unlock()
			bmlog.Debug("[http-source] 等待并发 manifest 请求完成...")
			<-ch
			// Re-check cache after the in-flight fetch completes.
			s.manifestCacheMu.Lock()
			cached := s.manifestCache
			s.manifestCacheMu.Unlock()
			if cached != nil {
				bmlog.Debug("[http-source] 复用并发请求结果 (条目数=%d)", len(cached.Data))
				return cached, nil
			}
			// In-flight fetch failed — retry.
			bmlog.Debug("[http-source] 并发请求失败，重试...")
			return s.fetchManifest(ctx)
		}
	}

	// Acquire ownership of the fetch.
	// Read and reset forceRefresh under the lock for atomicity.
	fetchCh := make(chan struct{})
	s.manifestFetchCh = fetchCh
	force := s.forceRefresh
	s.forceRefresh = false
	s.manifestCacheMu.Unlock()

	if force {
		bmlog.Debug("[http-source] 强制刷新模式: 跳过磁盘缓存，通知 serve 刷新在线源")
	}

	// Ensure we signal completion and clean up regardless of outcome.
	defer func() {
		s.manifestCacheMu.Lock()
		s.manifestFetchCh = nil
		s.manifestCacheMu.Unlock()
		close(fetchCh)
	}()

	// Try disk cache before hitting the network (skip in force-refresh mode).
	if !force {
		if diskResp, err := s.loadManifestFromDisk(); err == nil {
			s.manifestCacheMu.Lock()
			s.manifestCache = diskResp
			s.manifestCacheAge = time.Now()
			s.manifestCacheMu.Unlock()
			bmlog.Debug("[http-source] manifest 磁盘缓存命中 (条目数=%d)", len(diskResp.Data))
			return diskResp, nil
		}
	}

	// Fetch the full manifest from the server — no filter parameters.
	// All filtering is done client-side via FilterVersions.
	// In force-refresh mode, instruct serve to refresh its online cache.
	v1URL := s.baseURL + "/api/v1/manifest"
	if force {
		v1URL += "?refresh=true"
	}
	bmlog.Debug("[http-source] 请求 manifest: %s", v1URL)

	v1Resp, v1Body, err := s.fetchJSON(ctx, v1URL)
	if err != nil {
		bmlog.Debug("[http-source] 请求失败: %v", err)
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	bmlog.Debug("[http-source] 响应状态码: %d 大小=%d", v1Resp.StatusCode, len(v1Body))

	if v1Resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest request returned status %d", v1Resp.StatusCode)
	}

	var v1 manifestV1Response
	if err := json.Unmarshal(v1Body, &v1); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	if v1.Status != "ok" || v1.Data == nil {
		return nil, fmt.Errorf("unrecognized manifest format")
	}

	bmlog.Debug("[http-source] manifest 解析成功: %d 个文件", len(v1.Data))

	// Persist to disk cache for reuse across sessions.
	if err := s.saveManifestToDisk(&v1); err != nil {
		bmlog.Debug("[http-source] 保存 manifest 到磁盘缓存失败: %v", err)
	}

	// Store in memory for reuse within the current session.
	s.manifestCacheMu.Lock()
	s.manifestCache = &v1
	s.manifestCacheAge = time.Now()
	s.manifestCacheMu.Unlock()

	return &v1, nil
}

// manifestDiskCacheTTL is how long the on-disk manifest cache is considered
// fresh. It's longer than the in-memory cache (30s) because the client stores
// the full manifest locally so that it can determine available versions without
// always hitting the server.
const manifestDiskCacheTTL = 5 * time.Minute

// manifestDiskCacheFile returns the disk cache file path scoped to the
// HTTPSource's base URL so that different servers don't share caches.
func (s *HTTPSource) manifestDiskCacheFile() string {
	cacheDir := paths.Default().ManifestCacheDir
	// Sanitize baseURL into a safe filename component.
	safe := strings.NewReplacer(
		"://", "_",
		":", "_",
		"/", "_",
		".", "_",
	).Replace(s.baseURL)
	return filepath.Join(cacheDir, "serve_"+safe+".json")
}

// loadManifestFromDisk attempts to load the cached manifest from disk.
// Returns an error if the cache is missing or expired.
func (s *HTTPSource) loadManifestFromDisk() (*manifestV1Response, error) {
	cachePath := s.manifestDiskCacheFile()

	fi, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}
	if time.Since(fi.ModTime()) > manifestDiskCacheTTL {
		return nil, fmt.Errorf("disk manifest cache expired")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var resp manifestV1Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding cached manifest: %w", err)
	}

	if resp.Status != "ok" || resp.Data == nil {
		return nil, fmt.Errorf("cached manifest has invalid format")
	}

	return &resp, nil
}

// saveManifestToDisk persists the manifest to disk for cross-session reuse.
// On failure it returns an error (the caller should log but not block).
func (s *HTTPSource) saveManifestToDisk(manifest *manifestV1Response) error {
	cacheDir := paths.Default().ManifestCacheDir
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	cachePath := s.manifestDiskCacheFile()
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}

// maxResponseBodySize is the maximum HTTP response body size (100MB).
const maxResponseBodySize = 100 << 20

// fetchJSON fetches a URL and returns the response and body bytes.
func (s *HTTPSource) fetchJSON(ctx context.Context, url string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return resp, nil, fmt.Errorf("reading response body: %w", err)
	}

	return resp, body, nil
}
