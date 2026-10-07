package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/driver"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

const (
	// driverAPIBase is the API prefix for driver endpoints.
	driverAPIBase = "/api/v1/driver"

	// driverIndexFileName persists the mapping between a hosted driver archive
	// filename and the upstream URL it was resolved from, so serve can keep
	// proxying archives after a restart.
	driverIndexFileName = "drivers-index.json"

	// driverResolveTimeout bounds a single upstream driver resolution.
	driverResolveTimeout = 2 * time.Minute
)

// driverIndexEntry records how a hosted driver archive maps to its upstream
// source.
type driverIndexEntry struct {
	Filename     string    `json:"filename"`
	Version      string    `json:"version"`
	MajorVersion string    `json:"major_version"`
	Platform     string    `json:"platform"`
	Arch         string    `json:"arch"`
	DownloadURL  string    `json:"download_url"`
	ResolvedAt   time.Time `json:"resolved_at"`
}

// driverIndexFile is the on-disk form of the driver index.
type driverIndexFile struct {
	Version int                         `json:"version"`
	Entries map[string]driverIndexEntry `json:"entries"`
}

// DriverResolver resolves chromedriver builds and serves their archives,
// proxying the upstream Chrome for Testing storage.
//
// Clients query the manifest endpoint to map a Chrome version to a driver
// build, then download the archive from this instance. That keeps automation
// drivers usable on intranet/air-gapped deployments, mirroring how browser
// packages are distributed.
type DriverResolver struct {
	mgr       *driver.Manager
	dir       string
	indexPath string
	logger    *bmlog.Logger

	mu       sync.Mutex
	index    map[string]driverIndexEntry
	inflight map[string]chan struct{}

	// saveMu serializes index persistence. Without it, concurrent manifest
	// requests would interleave writes to the same temporary file and could
	// publish a truncated index.
	saveMu sync.Mutex
}

// newDriverResolver creates a resolver storing archives under cacheDir/drivers.
// p roots the driver manager's own paths (e.g. the Chrome for Testing manifest
// cache) at the serve data directory rather than the process-wide default.
func newDriverResolver(cacheDir string, p *paths.Paths, logger *bmlog.Logger) *DriverResolver {
	dir := filepath.Join(cacheDir, "drivers")
	r := &DriverResolver{
		mgr:       driver.NewManager(p, driver.Options{}),
		dir:       dir,
		indexPath: filepath.Join(cacheDir, driverIndexFileName),
		logger:    logger,
		index:     make(map[string]driverIndexEntry),
		inflight:  make(map[string]chan struct{}),
	}
	r.loadIndex()
	return r
}

// remember records the mapping for a resolved driver build and persists it.
// Persisting is skipped when the mapping is unchanged, so repeated manifest
// queries for the same build do not rewrite the index on every request.
func (r *DriverResolver) remember(info *driver.Info) {
	if info == nil || info.Filename == "" {
		return
	}
	entry := driverIndexEntry{
		Filename:     info.Filename,
		Version:      info.Version,
		MajorVersion: info.MajorVersion,
		Platform:     info.Platform,
		Arch:         info.Arch,
		DownloadURL:  info.DownloadURL,
		ResolvedAt:   time.Now(),
	}

	r.mu.Lock()
	prev, existed := r.index[entry.Filename]
	r.index[entry.Filename] = entry
	r.mu.Unlock()

	if existed && prev.Version == entry.Version && prev.MajorVersion == entry.MajorVersion &&
		prev.Platform == entry.Platform && prev.Arch == entry.Arch && prev.DownloadURL == entry.DownloadURL {
		return
	}

	r.saveIndex()
}

// lookup returns the index entry for a filename.
func (r *DriverResolver) lookup(filename string) (driverIndexEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.index[filename]
	return entry, ok
}

// findLocal resolves a driver from the index without touching the network.
// It is the offline fallback when the upstream manifest is unreachable.
func (r *DriverResolver) findLocal(chromeVersion, platform, arch string) (driverIndexEntry, bool) {
	major := majorOf(chromeVersion)

	r.mu.Lock()
	defer r.mu.Unlock()

	var best driverIndexEntry
	found := false
	for _, entry := range r.index {
		if entry.Platform != platform || entry.Arch != arch {
			continue
		}
		if major != "" && entry.MajorVersion != major {
			continue
		}
		if !found || version.Greater(entry.Version, best.Version) {
			best = entry
			found = true
		}
	}
	return best, found
}

// loadIndex reads the persisted index, tolerating a missing or corrupt file.
func (r *DriverResolver) loadIndex() {
	data, err := os.ReadFile(r.indexPath)
	if err != nil {
		return
	}
	var f driverIndexFile
	if err := json.Unmarshal(data, &f); err != nil {
		r.logger.Warn("[driver] 驱动索引损坏，将重建: %v", err)
		return
	}
	for name, entry := range f.Entries {
		r.index[name] = entry
	}
	r.logger.Debug("[driver] 已加载驱动索引 (%d 项)", len(r.index))
}

// saveIndex writes the index atomically. saveMu keeps concurrent writers from
// interleaving on the shared temporary file.
func (r *DriverResolver) saveIndex() {
	r.saveMu.Lock()
	defer r.saveMu.Unlock()

	r.mu.Lock()
	snapshot := make(map[string]driverIndexEntry, len(r.index))
	for k, v := range r.index {
		snapshot[k] = v
	}
	r.mu.Unlock()

	data, err := json.MarshalIndent(driverIndexFile{Version: 1, Entries: snapshot}, "", "  ")
	if err != nil {
		r.logger.Warn("[driver] 序列化驱动索引失败: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.indexPath), 0o755); err != nil {
		r.logger.Warn("[driver] 创建驱动索引目录失败: %v", err)
		return
	}
	tmp := r.indexPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		r.logger.Warn("[driver] 写入驱动索引失败: %v", err)
		return
	}
	if err := os.Rename(tmp, r.indexPath); err != nil {
		r.logger.Warn("[driver] 提交驱动索引失败: %v", err)
	}
}

// hostedFiles returns the driver archives already present on disk.
func (r *DriverResolver) hostedFiles() []string {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// --- HTTP handlers ---

// driverManifestResponse mirrors the API v1 envelope used by other endpoints.
type driverManifestResponse struct {
	Status string       `json:"status"`
	Data   *driver.Info `json:"data,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// handleDriverManifest resolves a driver build for a Chrome version.
//
// GET /api/v1/driver/manifest?chrome=<version>&platform=<p>&arch=<a>
func (s *Server) handleDriverManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != driverAPIBase+"/manifest" {
		http.NotFound(w, r)
		return
	}
	if s.driverResolver == nil {
		writeDriverError(w, http.StatusServiceUnavailable, "驱动服务未启用")
		return
	}

	q := r.URL.Query()
	chromeVersion := strings.TrimSpace(q.Get("chrome"))
	if chromeVersion == "" {
		writeDriverError(w, http.StatusBadRequest, "缺少 chrome 参数")
		return
	}
	platform := strings.TrimSpace(q.Get("platform"))
	if platform == "" {
		platform = paths.Platform()
	}
	arch := strings.TrimSpace(q.Get("arch"))
	if arch == "" {
		arch = paths.Arch()
	}

	ctx, cancel := context.WithTimeout(r.Context(), driverResolveTimeout)
	defer cancel()

	info, err := s.driverResolver.mgr.Resolve(ctx, chromeVersion, platform, arch)
	if err != nil {
		s.logger.Warn("[driver] 在线解析失败 (%s, %s/%s): %v，尝试本地索引", chromeVersion, platform, arch, err)
		if entry, ok := s.driverResolver.findLocal(chromeVersion, platform, arch); ok {
			info = &driver.Info{
				Name:         driver.NameChromedriver,
				Version:      entry.Version,
				MajorVersion: entry.MajorVersion,
				Platform:     entry.Platform,
				Arch:         entry.Arch,
				DownloadURL:  entry.DownloadURL,
				Filename:     entry.Filename,
				Source:       "serve-cache",
			}
			s.logger.Info("[driver] 使用本地索引命中驱动: %s", entry.Filename)
		} else {
			writeDriverError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	s.driverResolver.remember(info)

	// Rewrite the download URL to a relative path on this instance so the
	// client fetches the archive through serve rather than upstream.
	proxied := *info
	proxied.DownloadURL = driverAPIBase + "/download/" + url.PathEscape(info.Filename)
	proxied.Source = "serve"

	s.logger.Debug("[driver] 返回驱动清单: %s %s (%s/%s)", info.Name, info.Version, info.Platform, info.Arch)
	writeDriverJSON(w, http.StatusOK, driverManifestResponse{Status: "ok", Data: &proxied})
}

// handleDriverDownload serves a driver archive. A locally hosted archive is
// served directly; otherwise the archive is fetched from upstream, cached and
// then served.
//
// GET /api/v1/driver/download/{filename}
func (s *Server) handleDriverDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.driverResolver == nil {
		http.Error(w, "driver service disabled", http.StatusServiceUnavailable)
		return
	}

	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	filename := strings.TrimPrefix(r.URL.Path, driverAPIBase+"/download/")
	filename = strings.TrimSpace(filename)
	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(filename, `/\`) {
		s.logger.Warn("[driver] 非法文件名: %s (来自 %s)", filename, r.RemoteAddr)
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	fullPath, err := safeJoin(s.driverResolver.dir, filename)
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	if fi, statErr := os.Stat(fullPath); statErr == nil && !fi.IsDir() {
		s.logger.Debug("[driver] 本地命中驱动归档: %s", filename)
		servePackageFile(w, r, fullPath, s.logger)
		return
	}

	entry, ok := s.driverResolver.lookup(filename)
	if !ok {
		s.logger.Debug("[driver] 未找到驱动归档: %s", filename)
		http.Error(w, "driver not found", http.StatusNotFound)
		return
	}

	if err := s.fetchDriverArchive(filename, entry, fullPath); err != nil {
		s.logger.Warn("[driver] 代理下载驱动失败: %s: %v", filename, err)
		http.Error(w, "driver download failed", http.StatusBadGateway)
		return
	}

	servePackageFile(w, r, fullPath, s.logger)
}

// fetchDriverArchive downloads an upstream driver archive into the hosted
// directory, deduplicating concurrent requests for the same file.
func (s *Server) fetchDriverArchive(filename string, entry driverIndexEntry, destPath string) error {
	r := s.driverResolver

	r.mu.Lock()
	if waitCh, ok := r.inflight[filename]; ok {
		r.mu.Unlock()
		s.logger.Debug("[driver] 等待并发下载完成: %s", filename)
		<-waitCh
		if fi, err := os.Stat(destPath); err == nil && !fi.IsDir() {
			return nil
		}
		return fmt.Errorf("并发下载失败")
	}
	waitCh := make(chan struct{})
	r.inflight[filename] = waitCh
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.inflight, filename)
		r.mu.Unlock()
		close(waitCh)
	}()

	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("创建驱动目录失败: %w", err)
	}

	s.logger.Info("[driver] 正在从上游下载驱动: %s", filename)
	if _, err := DefaultDownload(entry.DownloadURL, r.dir, nil); err != nil {
		return err
	}
	return nil
}

// --- helpers ---

// majorOf extracts the numeric major version from a version string.
func majorOf(v string) string {
	if m := version.Major(v); m > 0 {
		return strconv.Itoa(m)
	}
	return strings.TrimSpace(v)
}

func writeDriverJSON(w http.ResponseWriter, status int, payload driverManifestResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func writeDriverError(w http.ResponseWriter, status int, message string) {
	writeDriverJSON(w, status, driverManifestResponse{Status: "error", Error: message})
}
