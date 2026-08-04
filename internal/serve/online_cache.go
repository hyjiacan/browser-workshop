package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/version"
)

// onlineCacheFile stores the on-disk cache for a single browser type.
// The URLs map preserves download URLs so that online fallback downloads
// work immediately after restart without re-fetching from the online source.
type onlineCacheFile struct {
	Browser string            `json:"browser"`
	Files   []PackageFile     `json:"files"`
	URLs    map[string]string `json:"urls,omitempty"` // filename -> download URL
	Updated time.Time         `json:"updated"`
}

// browserCacheEntry is the in-memory cache entry for one browser.
type browserCacheEntry struct {
	files []PackageFile
	time  time.Time
}

// OnlineCacheManager manages per-browser online version caches.
// Each browser type gets its own cache file (e.g. online-cache-firefox.json),
// so they can be loaded/saved/refreshed independently.
type OnlineCacheManager struct {
	cacheDir  string     // directory for cache files (e.g. bws-data/cache/serve)
	src       SyncSource // online source for refreshing
	logger    LoggerLike // logger for debug output
	browsers  []string   // which browsers to cache

	mu        sync.RWMutex
	caches    map[string]*browserCacheEntry // key: browser name
	filesMap  map[string]onlinePackage      // filename -> download info (all browsers)
	refreshMu sync.Mutex                    // serializes refreshes per browser
}

// LoggerLike is a minimal logger interface to avoid hard-coupling to bmlog.
type LoggerLike interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NewOnlineCacheManager creates a new per-browser online cache manager.
// browsers lists which browser types to cache; if empty, defaults to [firefox].
func NewOnlineCacheManager(cacheDir string, src SyncSource, logger LoggerLike, browsers []string) *OnlineCacheManager {
	if len(browsers) == 0 {
		browsers = []string{"firefox"}
	}
	return &OnlineCacheManager{
		cacheDir:  cacheDir,
		src:       src,
		logger:    logger,
		browsers:  browsers,
		caches:    make(map[string]*browserCacheEntry),
		filesMap: make(map[string]onlinePackage),
	}
}

// cacheFilePath returns the cache file path for a browser.
// e.g. bws-data/cache/serve/online-cache-firefox.json
func (m *OnlineCacheManager) cacheFilePath(browser string) string {
	return filepath.Join(m.cacheDir, fmt.Sprintf("online-cache-%s.json", browser))
}

// Get returns the cached package list for a browser, refreshing from the
// online source if the cache is stale or missing.
func (m *OnlineCacheManager) Get(browser string) []PackageFile {
	// Fast path: check if cache is fresh.
	m.mu.RLock()
	if entry, ok := m.caches[browser]; ok && time.Since(entry.time) < onlineCacheTTL {
		out := make([]PackageFile, len(entry.files))
		copy(out, entry.files)
		m.mu.RUnlock()
		return out
	}
	m.mu.RUnlock()

	// Cache miss — refresh.
	m.refresh(browser)

	// Return whatever is now in the cache (may be empty on failure).
	m.mu.RLock()
	defer m.mu.RUnlock()
	if entry, ok := m.caches[browser]; ok {
		out := make([]PackageFile, len(entry.files))
		copy(out, entry.files)
		return out
	}
	return nil
}

// GetAll returns all cached entries across all configured browsers,
// refreshing any stale caches. This is used by the manifest handler to
// return a merged view of all online-cached versions.
func (m *OnlineCacheManager) GetAll() []PackageFile {
	var all []PackageFile
	for _, b := range m.browsers {
		all = append(all, m.Get(b)...)
	}
	return all
}

// AllEntries returns all currently cached entries without triggering
// refreshes. Used when a quick snapshot is sufficient (e.g. startup logging).
func (m *OnlineCacheManager) AllEntries() []PackageFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []PackageFile
	for _, entry := range m.caches {
		all = append(all, entry.files...)
	}
	return all
}

// RefreshAll refreshes caches for all configured browsers.
// This is typically called from a background goroutine at startup.
func (m *OnlineCacheManager) RefreshAll() {
	for _, b := range m.browsers {
		m.refresh(b)
	}
}

// Browsers returns the list of browsers this manager tracks.
func (m *OnlineCacheManager) Browsers() []string {
	return m.browsers
}

// FindPackage looks up the download info for a filename across all browsers.
// Returns ok=false if not found.
func (m *OnlineCacheManager) FindPackage(filename string) (onlinePackage, bool) {
	m.mu.RLock()
	info, ok := m.filesMap[filename]
	m.mu.RUnlock()
	return info, ok
}

// AddPackage adds a download info entry (used after on-demand download).
func (m *OnlineCacheManager) AddPackage(filename string, info onlinePackage) {
	m.mu.Lock()
	if _, exists := m.filesMap[filename]; !exists {
		m.filesMap[filename] = info
	}
	m.mu.Unlock()
}

// refresh queries the online source for a single browser across all
// platform/arch combos and updates the cache. Failures are logged but
// don't clear existing cache data.
func (m *OnlineCacheManager) refresh(browser string) {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()

	// Double-check after acquiring lock.
	m.mu.RLock()
	if entry, ok := m.caches[browser]; ok && time.Since(entry.time) < onlineCacheTTL {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	m.logger.Debug("[online] 刷新缓存: browser=%s", browser)

	refreshStart := time.Now()
	var list []PackageFile
	var newFiles []onlinePackage
	successQueries := 0
	seen := make(map[string]bool) // deduplicate by filename across platform/arch combos

	for _, combo := range defaultOnlineCombos {
		m.logger.Debug("[online] 查询在线源: browser=%s channel=all platform=%s arch=%s",
			browser, combo.platform, combo.arch)

		versions, err := m.listVersionsWithTimeout(browser, "", combo.platform, combo.arch)
		if err != nil {
			m.logger.Warn("[online] 查询失败: browser=%s platform=%s arch=%s: %v",
				browser, combo.platform, combo.arch, err)
			continue
		}
		successQueries++

		m.logger.Debug("[online] 查询成功: browser=%s platform=%s arch=%s 返回 %d 个版本",
			browser, combo.platform, combo.arch, len(versions))

		for _, v := range versions {
			fname := onlineFilename(v)
			if fname == "" || v.DownloadURL == "" {
				continue
			}
			// Skip duplicate filenames (same file returned for different
			// platform/arch combos). Keep the first occurrence.
			if seen[fname] {
				continue
			}
			seen[fname] = true
			list = append(list, buildOnlinePackageFile(fname, v, combo.platform, combo.arch))
			newFiles = append(newFiles, onlinePackage{
				url:      v.DownloadURL,
				browser:  browser,
				version:  v.Version,
				platform: combo.platform,
				arch:     combo.arch,
				size:     v.Size,
				checksum: v.Checksum,
			})
		}
	}

	m.logger.Debug("[online] 刷新完成: browser=%s 成功 %d 次查询, 获取 %d 个版本 (耗时 %v)",
		browser, successQueries, len(list), time.Since(refreshStart).Round(time.Millisecond))

	// Sort list and newFiles together by filename so that indices stay
	// synchronized. Without this, the filesMap and URL map would reference
	// the wrong download URL for each filename.
	type cacheEntry struct {
		pkg  PackageFile
		info onlinePackage
	}
	entries := make([]cacheEntry, len(list))
	for i := range list {
		entries[i] = cacheEntry{pkg: list[i], info: newFiles[i]}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pkg.Filename < entries[j].pkg.Filename
	})
	for i, e := range entries {
		list[i] = e.pkg
		newFiles[i] = e.info
	}

	// Update in-memory cache and filesMap.
	// Always overwrite entries for the current browser so that refreshed
	// data (including download URLs) replaces stale disk-loaded entries.
	m.mu.Lock()
	m.caches[browser] = &browserCacheEntry{files: list, time: time.Now()}
	for i, f := range list {
		if existing, exists := m.filesMap[f.Filename]; exists && existing.browser != browser {
			continue // Don't overwrite entries belonging to a different browser
		}
		m.filesMap[f.Filename] = newFiles[i]
	}
	m.mu.Unlock()

	// Persist to disk (best-effort).
	urls := make(map[string]string, len(newFiles))
	for i, nf := range newFiles {
		if nf.url != "" && i < len(list) {
			urls[list[i].Filename] = nf.url
		}
	}
	m.saveToDisk(browser, list, urls)
}

// LoadFromDisk loads all existing cache files from disk at startup.
// This is non-blocking: failures are logged and skipped.
func (m *OnlineCacheManager) LoadFromDisk() {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match pattern: online-cache-<browser>.json
		if len(name) <= len("online-cache-.json") || !strings.HasPrefix(name, "online-cache-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		browser := name[len("online-cache-") : len(name)-len(".json")]
		if browser == "" {
			continue
		}
		m.loadOneFromDisk(browser)
	}
}

func (m *OnlineCacheManager) loadOneFromDisk(browser string) {
	path := m.cacheFilePath(browser)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cf onlineCacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		m.logger.Debug("[online] 加载缓存文件失败: %s: %v", path, err)
		return
	}
	// If the cache file predates URL persistence, mark it as stale so the
	// next access triggers a refresh that populates download URLs.
	cacheTime := cf.Updated
	if len(cf.URLs) == 0 {
		m.logger.Debug("[online] 缓存文件缺少下载地址，标记为过期以触发刷新: browser=%s", browser)
		cacheTime = time.Time{} // zero value → always stale
	}

	m.caches[browser] = &browserCacheEntry{
		files: cf.Files,
		time:  cacheTime,
	}
	// Populate filesMap from loaded data, restoring download URLs.
	for _, f := range cf.Files {
		if _, exists := m.filesMap[f.Filename]; !exists {
			pkg := onlinePackage{
				browser:  f.Browser,
				version:  f.Version,
				platform: f.Platform,
				arch:     f.Architecture,
				size:     f.Size,
				checksum: f.UpstreamChecksum,
			}
			if cf.URLs != nil {
				pkg.url = cf.URLs[f.Filename]
			}
			m.filesMap[f.Filename] = pkg
		}
	}
	m.logger.Debug("[online] 已加载缓存: browser=%s 条目数=%d", browser, len(cf.Files))
}

func (m *OnlineCacheManager) saveToDisk(browser string, files []PackageFile, urls map[string]string) {
	cf := onlineCacheFile{
		Browser: browser,
		Files:   files,
		URLs:    urls,
		Updated: time.Now(),
	}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return
	}
	path := m.cacheFilePath(browser)

	// Ensure cache directory exists (may be missing in test scenarios).
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	// Atomic write: write to a temp file first, then rename.
	// This prevents cache corruption if the process crashes mid-write.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		m.logger.Debug("[online] 写入临时缓存文件失败: %s: %v", tmpPath, err)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		m.logger.Debug("[online] 重命名缓存文件失败: %s -> %s: %v", tmpPath, path, err)
		_ = os.Remove(tmpPath) // clean up temp file on rename failure
	}
}

// listVersionsWithTimeout calls src.ListVersions with a timeout.
// The context is passed through to the underlying HTTP requests so that
// cancelled queries don't leak goroutines or hold locks.
func (m *OnlineCacheManager) listVersionsWithTimeout(browser, channel, platform, arch string) ([]SyncVersionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), onlineListTimeout)
	defer cancel()
	return m.src.ListVersions(ctx, browser, channel, platform, arch)
}

// --- helpers ---

// buildOnlinePackageFile builds a PackageFile entry for an online-available version.
// Platform and arch are used as-is because they come from defaultOnlineCombos
// which already uses canonical names (windows, darwin, linux, amd64, 386, arm64)
// consistent with the source package's Platform/Arch types and the client's
// FilterVersions expectations.
func buildOnlinePackageFile(filename string, v SyncVersionInfo, platform, arch string) PackageFile {
	pkg := PackageFile{
		Filename:         filename,
		Version:          v.Version,
		Browser:          v.Browser,
		Channel:          v.Channel,
		Size:             v.Size,
		Platform:         platform,
		Architecture:     arch,
		UpstreamChecksum: v.Checksum,
	}
	if v.Version != "" {
		pkg.MajorVersion = strconv.Itoa(version.Major(v.Version))
	} else {
		pkg.MajorVersion = "0"
	}
	return pkg
}
