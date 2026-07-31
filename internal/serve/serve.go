// Package serve provides an HTTP server for hosting browser version packages.
// It serves a directory of browser installers following a standard layout
// and provides a manifest API for clients to discover available versions.
//
// API v1:
//   - GET /api/v1/manifest     - 文件清单（本地 + 在线缓存，含 XXH3 校验和）
//   - GET /api/v1/download/{filename} - 文件下载（支持 Range 断点续传）
//   - GET /api/v1/status       - 服务状态
//   - GET /api/v1/bin/{filename} - 客户端二进制下载
//   - GET /                    - HTML 帮助页
package serve

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/browser"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/repo"
	"github.com/bws/bws/internal/util"
	"github.com/bws/bws/internal/version"
	"github.com/zeebo/xxh3"
)

//go:embed page.html
var pageHTML embed.FS

const (
	serverName   = "bws-serve"
	cacheVersion = 1

	// onlineCacheTTL is how long the online-version cache is considered fresh.
	// Default: 24 hours so that serve doesn't hammer online sources.
	onlineCacheTTL = 24 * time.Hour

	// onlineListTimeout is the per-source timeout when listing online versions.
	onlineListTimeout = 30 * time.Second
)

// onlinePackage describes a package available from the online fallback source.
type onlinePackage struct {
	url      string
	browser  string
	version  string
	platform string
	arch     string
	size     int64
	sha256   string
}

// defaultOnlineCombos lists the platform/arch combinations queried from the
// online source when building the fallback manifest. It covers the common
// client platforms so that clients on any OS can see servable versions.
var defaultOnlineCombos = []struct {
	platform string
	arch     string
}{
	{"windows", "amd64"},
	{"windows", "386"},
	{"windows", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
}

// pageData holds the data for rendering the HTML page template.
type pageData struct {
	Version        string
	ServerName     string
	Description    string
	Features       []string
	FileCount      int
	TotalSize      string
	BinFiles       []binFileView
	BaseURL        string
	SyncEnabled    bool
	OnlineFallback bool
}

// binFileView represents a bin file for template rendering.
type binFileView struct {
	Name          string
	File          string
	Platform      string
	Arch          string
	PlatformLabel string
	Size          string
}

// Server serves packages over HTTP with API v1.
type Server struct {
	addr        string
	version     string
	baseDir     string // 程序所在目录
	packagesDir string // baseDir/packages
	binDir      string // baseDir/bin
	cachePath   string // baseDir/.serve-cache.json
	configPath  string // bws-serve.ini path (for startup logging)

	logger    *bmlog.Logger
	startTime time.Time
	httpSrv   *http.Server
	mu        sync.RWMutex
	files     []PackageFile
	totalSize int64

	// scanWorkers is the configured number of worker goroutines for parallel
	// checksum computation. 0 means auto (runtime.NumCPU()).
	scanWorkers int

	syncMgr *syncManager

	// Online fallback: fetch packages from an online source when not cached locally.
	onlineSrc      SyncSource
	onlineFallback bool
	onlineCacheMgr *OnlineCacheManager

	// dlMu + dlInflight deduplicate concurrent on-demand downloads of the
	// same filename: the first requester downloads, others wait.
	dlMu       sync.Mutex
	dlInflight map[string]chan struct{}

	// cacheMu protects saveCache from concurrent writes to the cache file.
	cacheMu sync.Mutex

	// scanMu serializes concurrent scanPackages calls so that multiple
	// on-demand online fallback downloads don't trigger parallel scans.
	scanMu sync.Mutex

	// authToken is the optional bearer token for API authentication.
	// When non-empty, /api/ requests must carry "Authorization: Bearer <token>".
	authToken string
}

// PackageFile represents a single package file with its metadata.
type PackageFile struct {
	Filename     string `json:"filename"`
	Version      string `json:"version"`
	Browser      string `json:"browser"`
	Channel      string `json:"channel"`
	MajorVersion string `json:"major_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	SHA256       string `json:"sha256,omitempty"` // expected SHA-256 hash from upstream (empty if unknown)
}

// cacheFile represents the on-disk checksum cache.
type cacheFile struct {
	Version int                   `json:"version"`
	Files   map[string]cacheEntry `json:"files"`
}

// cacheEntry stores cached checksum info for a single file.
type cacheEntry struct {
	Mtime    time.Time `json:"mtime"`
	Checksum string    `json:"checksum"`
	Size     int64     `json:"size"`
}

// ManifestResponse is the API v1 manifest response.
type ManifestResponse struct {
	Status string        `json:"status"`
	Data   []PackageFile `json:"data"`
	Server struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		FileCount int    `json:"file_count"`
	} `json:"server"`
}

// StatusResponse is the API v1 status response.
type StatusResponse struct {
	Status string `json:"status"`
	Server struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Uptime    int64  `json:"uptime"`
		FileCount int    `json:"file_count"`
		TotalSize int64  `json:"total_size"`
	} `json:"server"`
}

// ServerOptions configures a new Server.
type ServerOptions struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string

	// Version is the server/program version string.
	Version string

	// BaseDir is the base directory for all serve-related paths.
	// If empty, the executable directory is used.
	// All relative paths (packages, bin, cache) are resolved relative to BaseDir.
	BaseDir string

	// PackagesDir is the directory containing browser packages.
	// If empty, defaults to baseDir/packages.
	// If relative, it is resolved relative to BaseDir.
	// If absolute, it is used as-is.
	PackagesDir string

	// BinDir is the directory containing client binary files.
	// If empty, defaults to baseDir/bin.
	// If relative, it is resolved relative to BaseDir.
	// If absolute, it is used as-is.
	BinDir string

	// SyncSource is the source for auto-syncing packages from online sources.
	// If nil, auto-sync is disabled.
	SyncSource SyncSource

	// SyncConfig configures the auto-sync behavior.
	// If zero-value defaults are used, sync is enabled with 24h interval
	// (only if SyncSource is set).
	SyncConfig SyncConfig

	// OnlineSource is the online source used for on-demand fallback fetching.
	// When a requested package is not present locally and OnlineFallback is
	// enabled, the package is downloaded from this source and served to the
	// client. It uses the same SyncSource interface as SyncSource.
	OnlineSource SyncSource

	// OnlineFallback enables on-demand fetching from OnlineSource when a
	// requested package file is not found locally. When enabled, the manifest
	// also lists all versions available from the online source.
	OnlineFallback bool

	// OnlineBrowsers is the list of browsers to query from OnlineSource when
	// building the fallback manifest. If empty, defaults to [firefox].
	OnlineBrowsers []string

	// ScanWorkers is the number of worker goroutines used for parallel
	// checksum computation during package scanning. 0 means auto
	// (runtime.NumCPU()). Values are clamped to [1, 32].
	ScanWorkers int

	// ConfigPath is the path to the bws-serve.ini config file. It is only
	// used for informational startup logging. If empty, it is derived from
	// BaseDir.
	ConfigPath string

	// AuthToken is an optional bearer token for API authentication.
	// When set, all /api/ requests must include "Authorization: Bearer <token>".
	// When empty, the API is open (no authentication).
	// The root HTML page is always accessible without authentication.
	AuthToken string

	// Logger is the logger used by the server. If nil, the default logger is used.
	Logger *bmlog.Logger
}

// NewServer creates a new serve server.
// addr is the listen address (e.g. ":8080").
// version is the program version string.
// The packages directory defaults to exeDir/packages.
func NewServer(addr string, version string) *Server {
	return NewServerWithOptions(ServerOptions{
		Addr:    addr,
		Version: version,
	})
}

// NewServerWithOptions creates a new serve server with full options.
func NewServerWithOptions(opts ServerOptions) *Server {
	// baseDir is where serve's internal files (cache, etc.) are stored.
	// Default: bws-data directory (shared with client data).
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = paths.Default().Root
	}

	// packagesDir and binDir default to exe directory subdirectories.
	// They can be absolute paths or relative to the exe directory.
	exeDir, err := paths.ExeDir()
	if err != nil {
		wd, _ := os.Getwd()
		exeDir = wd
	}

	packagesDir := opts.PackagesDir
	if packagesDir == "" {
		packagesDir = filepath.Join(exeDir, "packages")
	}

	binDir := opts.BinDir
	if binDir == "" {
		binDir = filepath.Join(exeDir, "bin")
	}

	logger := opts.Logger
	if logger == nil {
		logger = bmlog.Default()
	}

	// configPath is used only for startup logging. Default to bws-serve.ini in exe dir.
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = ConfigPath("")
	}

	srv := &Server{
		addr:           opts.Addr,
		version:        opts.Version,
		baseDir:        baseDir,
		packagesDir:    packagesDir,
		binDir:         binDir,
		cachePath:      filepath.Join(baseDir, ".serve-cache.json"),
		configPath:     configPath,
		logger:         logger,
		onlineSrc:      opts.OnlineSource,
		onlineFallback: opts.OnlineFallback,
		scanWorkers:    opts.ScanWorkers,
		dlInflight:     make(map[string]chan struct{}),
		authToken:      opts.AuthToken,
	}

	// Create online cache manager when online source is configured.
	// The manager handles per-browser caching of online version data.
	if opts.OnlineSource != nil {
		srv.onlineCacheMgr = NewOnlineCacheManager(baseDir, opts.OnlineSource, logger, opts.OnlineBrowsers)
	}

	// Set up sync manager if source is provided
	if opts.SyncSource != nil {
		cfg := opts.SyncConfig
		if cfg.Interval == 0 {
			cfg.Interval = 24 * time.Hour
		}
		if len(cfg.Channels) == 0 {
			cfg.Channels = []string{"stable"}
		}
		cfg.Enabled = true // enabled by default when source is provided
		srv.syncMgr = newSyncManager(srv, opts.SyncSource, cfg)
	}

	return srv
}

// Start starts the HTTP server. It blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Info("[serve] 正在启动...")

	// Ensure directories exist
	if err := os.MkdirAll(s.packagesDir, 0o755); err != nil {
		s.logger.Error("[serve] 创建软件包目录失败: %v", err)
		return fmt.Errorf("creating packages directory: %w", err)
	}
	if err := os.MkdirAll(s.binDir, 0o755); err != nil {
		s.logger.Error("[serve] 创建客户端二进制目录失败: %v", err)
		return fmt.Errorf("creating bin directory: %w", err)
	}

	// Load checksum cache
	cache, err := s.loadCache()
	if err != nil {
		s.logger.Debug("[serve] 加载缓存失败（将使用空缓存）: %v", err)
	} else if len(cache) > 0 {
		s.logger.Debug("[serve] 已加载缓存: %d 个条目", len(cache))
	}

	// Scan packages and compute checksums
	if err := s.scanPackages(cache); err != nil {
		s.logger.Error("[serve] 扫描软件包失败: %v", err)
		return fmt.Errorf("scanning packages: %w", err)
	}

	// Save updated cache
	if err := s.saveCache(cache); err != nil {
		s.logger.Warn("[serve] 保存缓存失败: %v", err)
	}

	s.startTime = time.Now()

	// Start sync manager
	if s.syncMgr != nil {
		s.logger.Info("[serve] 自动同步已启用 (间隔 %v)", s.syncMgr.config.Interval)
		s.logger.Debug("[serve] 同步配置: browsers=%v channels=%v", s.syncMgr.config.Browsers, s.syncMgr.config.Channels)
		s.syncMgr.Start()
	}

	// Online fallback
	if s.onlineFallback && s.onlineSrc != nil {
		s.logger.Info("[serve] 在线回退已启用 (browsers=%v)", s.onlineCacheMgr.Browsers())

		// Load existing cache files from disk first (fast, no network).
		s.onlineCacheMgr.LoadFromDisk()
	} else if s.onlineFallback && s.onlineSrc == nil {
		s.logger.Warn("[serve] 在线回退已启用但未配置在线源，功能不可用")
	}

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/manifest", s.handleManifest)
	mux.HandleFunc("/api/v1/download/", s.handleDownload)
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/sync/status", s.handleSyncStatus)
	mux.HandleFunc("/api/v1/sync/trigger", s.handleSyncTrigger)
	mux.HandleFunc("/api/v1/bin/", s.handleBin)
	mux.HandleFunc("/", s.handleRoot)

	// Wrap mux with optional auth middleware
	handler := http.Handler(mux)
	if s.authToken != "" {
		handler = &authMiddleware{next: handler, token: s.authToken, logger: s.logger}
	}

	s.httpSrv = &http.Server{
		Addr:         s.addr,
		Handler:      &loggingHandler{next: handler, logger: s.logger, name: serverName},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.printStartupInfo()

	// Start online cache preload AFTER printing startup info so logs don't interleave.
	if s.onlineFallback && s.onlineSrc != nil {
		go func() {
			s.logger.Info("[online] 正在后台预加载在线缓存...")
			preloadStart := time.Now()
			s.onlineCacheMgr.RefreshAll()
			s.logger.Info("[online] 在线缓存预加载完成 (耗时 %v)", time.Since(preloadStart).Round(time.Millisecond))
		}()
	}

	s.logger.Info("[serve] 服务器开始监听: %s", s.addr)
	return s.httpSrv.ListenAndServe()
}

// Stop gracefully stops the server with a default timeout.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("[serve] 正在关闭服务器...")

	// Stop sync manager first
	if s.syncMgr != nil {
		s.logger.Debug("[sync] 正在停止同步管理器...")
		s.syncMgr.Stop()
	}
	if s.httpSrv == nil {
		return nil
	}

	s.logger.Debug("[serve] 正在等待HTTP请求完成（超时 %v）...", ctx)
	err := s.httpSrv.Shutdown(ctx)
	if err != nil {
		s.logger.Error("[serve] 服务器关闭失败: %v", err)
	} else {
		s.logger.Info("[serve] 服务器已安全关闭")
	}
	return err
}

// printStartupInfo prints server startup information to stdout.
func (s *Server) printStartupInfo() {
	s.mu.RLock()
	fileCount := len(s.files)
	totalSize := s.totalSize
	s.mu.RUnlock()

	fmt.Println("========================================")
	fmt.Printf("  %s v%s\n", serverName, s.version)
	fmt.Println("========================================")
	fmt.Printf("  数据目录:     %s\n", s.baseDir)
	fmt.Printf("  软件包目录:   %s\n", s.packagesDir)
	fmt.Printf("  客户端目录:   %s\n", s.binDir)
	fmt.Printf("  监听地址:     %s\n", s.addr)
	fmt.Printf("  软件包数量:   %d 个 (%s)\n", fileCount, util.FormatSize(totalSize))
	if s.onlineFallback && s.onlineSrc != nil {
		fmt.Printf("  在线回退:     已启用\n")
	}
	if s.syncMgr != nil {
		fmt.Printf("  自动同步:     已启用 (间隔 %v)\n", s.syncMgr.config.Interval)
	}
	fmt.Println()
	fmt.Println("  API 接口:")
	fmt.Printf("    GET /                    - HTML 帮助页面\n")
	fmt.Printf("    GET /api/v1/manifest     - 软件包清单 (含在线缓存)\n")
	fmt.Printf("    GET /api/v1/download/    - 软件包下载\n")
	fmt.Printf("    GET /api/v1/status       - 服务状态\n")
	fmt.Printf("    GET /api/v1/bin/         - 客户端二进制文件\n")
	fmt.Println()
	fmt.Println("  客户端配置:")
	fmt.Printf("    bws config set source http://<服务器地址>:<端口>\n")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止服务。")
	fmt.Println()

	// Debug: detailed config for troubleshooting
	s.logger.Debug("[serve] 配置: addr=%s packages=%s bin=%s cache=%s config=%s workers=%d",
		s.addr, s.packagesDir, s.binDir, s.cachePath, s.configPath, s.scanWorkers)
}

// supportedPackageExtensions lists file extensions (lowercase, leading dot) for
// browser installer/archive packages that should be served. Compound
// extensions (e.g. ".tar.gz") are checked first by isSupportedExtension so the
// single-ext filepath.Ext fallback (".gz") does not reject them.
var supportedPackageExtensions = map[string]bool{
	".exe":     true,
	".msi":     true, // Windows 安装包
	".zip":     true,
	".7z":      true,
	".tar.gz":  true,
	".tgz":     true,
	".tar.bz2": true,
	".tbz2":    true,
	".tar.xz":  true,
	".txz":     true,
	".tar.zst": true,
	".dmg":     true, // macOS 安装包
	".deb":     true, // Linux Debian
	".rpm":     true, // Linux RPM
	".apk":     true, // Android
}

// supportedCompoundExtensions lists multi-dot extensions that filepath.Ext
// cannot detect on its own (it only returns the last dot segment). These are
// checked against the lowercased base name before falling back to the single
// extension.
var supportedCompoundExtensions = []string{
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
}

// isSupportedExtension reports whether the given file (by its base name) has a
// supported installer/archive extension. It first checks compound extensions
// (e.g. ".tar.gz") against the full base name, then falls back to the single
// trailing extension via filepath.Ext.
func isSupportedExtension(base string) bool {
	lower := strings.ToLower(base)
	// Check compound extensions first (e.g. ".tar.gz" before ".gz").
	for _, ext := range supportedCompoundExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return false
	}
	return supportedPackageExtensions[ext]
}

// scanPackages scans the packages directory recursively and builds the file list.
// Uses a worker pool for parallel checksum computation. The number of workers is
// determined by s.scanWorkers (0 = runtime.NumCPU(), clamped to [1, 32]).
func (s *Server) scanPackages(cache map[string]cacheEntry) error {
	// Serialize concurrent scan calls — multiple on-demand downloads may
	// trigger scans simultaneously.
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	scanStart := time.Now()
	if cache == nil {
		cache = make(map[string]cacheEntry)
	}
	s.logger.Info("[scan] 正在扫描目录: %s", s.packagesDir)

	// Create a scanner for filename parsing
	scanner, err := repo.NewScanner(s.packagesDir, browser.DefaultRegistry)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}

	// Determine the number of checksum workers. 0 = auto (NumCPU), clamped to [1, 32].
	numWorkers := s.scanWorkers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 32 {
		numWorkers = 32
	}

	// Phase 1: walk directory recursively to collect all files
	type fileEntry struct {
		relPath string
		absPath string
		info    os.FileInfo
	}
	var allFiles []fileEntry
	seenFiles := make(map[string]bool)
	skippedUnsupported := 0

	err = filepath.WalkDir(s.packagesDir, func(absPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		// Filter by supported installer/archive extension. This skips
		// non-package files like .txt, .json, .html, .md, .png, .jpg, etc.
		if !isSupportedExtension(d.Name()) {
			skippedUnsupported++
			s.logger.Debug("[scan] 跳过不支持的文件类型: %s", d.Name())
			return nil
		}
		relPath, err := filepath.Rel(s.packagesDir, absPath)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		seenFiles[relPath] = true
		allFiles = append(allFiles, fileEntry{
			relPath: relPath,
			absPath: absPath,
			info:    info,
		})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Debug("[scan] 软件包目录不存在，跳过扫描")
			return nil
		}
		return fmt.Errorf("scanning packages directory: %w", err)
	}

	if len(allFiles) == 0 {
		s.logger.Debug("[scan] 未找到任何文件 (跳过 %d 个非安装包文件)", skippedUnsupported)
		s.mu.Lock()
		s.files = nil
		s.totalSize = 0
		s.mu.Unlock()
		return nil
	}

	s.logger.Debug("[scan] 目录遍历完成: %d 个有效文件, 跳过 %d 个非安装包文件 (耗时 %v)",
		len(allFiles), skippedUnsupported, time.Since(scanStart).Round(time.Millisecond))

	// Phase 2: separate cache hits and misses
	type missEntry struct {
		idx     int
		relPath string
		absPath string
		info    os.FileInfo
	}
	var files []PackageFile
	var totalSize int64
	cacheHits := 0
	misses := make([]missEntry, 0)
	files = make([]PackageFile, len(allFiles))

	for i, fe := range allFiles {
		cached, ok := cache[fe.relPath]
		if ok && cached.Mtime.Equal(fe.info.ModTime()) && cached.Size == fe.info.Size() {
			// Cache hit
			pkg := s.parsePackageFile(scanner, fe.relPath, fe.info.Size(), cached.Checksum)
			files[i] = pkg
			totalSize += fe.info.Size()
			cacheHits++
		} else {
			// Cache miss - needs checksum computation
			misses = append(misses, missEntry{
				idx:     i,
				relPath: fe.relPath,
				absPath: fe.absPath,
				info:    fe.info,
			})
		}
	}

	// Phase 3: compute checksums in parallel using worker pool
	if len(misses) > 0 {
		workers := numWorkers
		if len(misses) < workers {
			workers = len(misses)
		}
		s.logger.Debug("[scan] 校验和计算: 缓存命中 %d, 需计算 %d (线程 %d)",
			cacheHits, len(misses), workers)

		type job struct {
			idx     int
			relPath string
			absPath string
		}
		type result struct {
			idx      int
			relPath  string
			checksum string
			err      error
		}

		jobs := make(chan job, len(misses))
		results := make(chan result, len(misses))

		// Start workers
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					cs, err := computeXXH3(j.absPath)
					results <- result{
						idx:      j.idx,
						relPath:  j.relPath,
						checksum: cs,
						err:      err,
					}
				}
			}()
		}

		// Submit jobs
		for _, m := range misses {
			jobs <- job{
				idx:     m.idx,
				relPath: m.relPath,
				absPath: m.absPath,
			}
		}
		close(jobs)

		// Wait for all workers to finish
		go func() {
			wg.Wait()
			close(results)
		}()

		// Build miss lookup map for O(1) access
		missMap := make(map[int]missEntry, len(misses))
		for _, m := range misses {
			missMap[m.idx] = m
		}

		// Collect results
		for r := range results {
			if r.err != nil {
				s.logger.Warn("[scan] 计算校验和失败: %s: %v", r.relPath, r.err)
				continue
			}
			m, ok := missMap[r.idx]
			if !ok {
				continue
			}
			pkg := s.parsePackageFile(scanner, r.relPath, m.info.Size(), r.checksum)
			files[r.idx] = pkg
			totalSize += m.info.Size()
			// Update cache
			cache[r.relPath] = cacheEntry{
				Mtime:    m.info.ModTime(),
				Checksum: r.checksum,
				Size:     m.info.Size(),
			}
		}
	}

	// Phase 4: remove nil entries (failed checksums) and cleanup
	validFiles := files[:0]
	for _, f := range files {
		if f.Filename != "" {
			validFiles = append(validFiles, f)
		}
	}
	files = validFiles

	// Clean up cache entries for deleted files
	for name := range cache {
		if !seenFiles[name] {
			delete(cache, name)
		}
	}

	// Sort files by filename for consistent output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})

	// Print discovery log for each valid file (jm.exe style).
	for _, f := range files {
		s.logger.Info("[scan] 发现: %s（%s/%s, 版本 %s, %s, %s）",
			f.Filename, f.Platform, f.Architecture, f.Version, f.Browser, util.FormatSize(f.Size))
	}

	s.mu.Lock()
	s.files = files
	s.totalSize = totalSize
	s.mu.Unlock()

	computedSuccessfully := len(files) - cacheHits

	s.logger.Info("[scan] 扫描完成: %d 个文件 (缓存命中 %d, 新计算 %d, 跳过 %d), 总大小 %s (耗时 %v)",
		len(files), cacheHits, computedSuccessfully, skippedUnsupported, util.FormatSize(totalSize), time.Since(scanStart).Round(time.Millisecond))

	return nil
}

// parsePackageFile parses a filename to extract metadata and returns a PackageFile.
func (s *Server) parsePackageFile(scanner *repo.Scanner, filename string, size int64, checksum string) PackageFile {
	// Strip extension for matching
	nameNoExt := util.StripExtension(filename)

	// Use scanner to detect metadata
	match := scanner.ScanEntry(nameNoExt, filename, true, "", "")

	pkg := PackageFile{
		Filename: filename,
		Size:     size,
		Checksum: "xxh3:" + checksum,
	}

	pkg.Browser = match.Browser
	pkg.Channel = match.Channel

	if match.Version != "" {
		pkg.Version = match.Version
		pkg.MajorVersion = strconv.Itoa(version.Major(match.Version))
	} else {
		pkg.Version = "unknown"
		pkg.MajorVersion = "0"
	}

	if match.Platform != "" {
		pkg.Platform = match.Platform
	} else {
		pkg.Platform = "unknown"
	}

	if match.Arch != "" {
		pkg.Architecture = match.Arch
	} else {
		pkg.Architecture = "unknown"
	}

	return pkg
}

// computeXXH3 computes the XXH3 64-bit hash of a file and returns it as a hex string.
func computeXXH3(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxh3.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}

// loadCache loads the checksum cache from disk.
func (s *Server) loadCache() (map[string]cacheEntry, error) {
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]cacheEntry), nil
		}
		return nil, err
	}

	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}

	if cf.Version != cacheVersion {
		return make(map[string]cacheEntry), nil
	}

	if cf.Files == nil {
		return make(map[string]cacheEntry), nil
	}

	return cf.Files, nil
}

// saveCache saves the checksum cache to disk.
func (s *Server) saveCache(cache map[string]cacheEntry) error {
	cf := cacheFile{
		Version: cacheVersion,
		Files:   cache,
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return os.WriteFile(s.cachePath, data, 0o600)
}

// --- HTTP Handlers ---

// handleManifest returns the merged manifest of local packages and online-cached
// versions as JSON. When online fallback is enabled, the manifest includes both
// locally hosted files (with real checksums) and online-cached version entries
// (without checksums until downloaded). Filtering is done client-side.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/v1/manifest" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	localFiles := make([]PackageFile, len(s.files))
	copy(localFiles, s.files)
	s.mu.RUnlock()

	totalLocal := len(localFiles)

	// Merge online cache entries when online fallback is enabled.
	var merged []PackageFile
	if s.onlineFallback && s.onlineCacheMgr != nil {
		onlineFiles := s.onlineCacheMgr.GetAll()
		merged = mergeManifest(localFiles, onlineFiles)
		s.logger.Debug("[manifest] 返回 %d 个文件 (本地 %d + 在线缓存 %d, 合并去重后 %d)",
			len(merged), totalLocal, len(onlineFiles), len(merged))
	} else {
		merged = localFiles
		s.logger.Debug("[manifest] 返回 %d 个文件 (仅本地)", totalLocal)
	}

	resp := ManifestResponse{
		Status: "ok",
		Data:   merged,
	}
	resp.Server.Name = serverName
	resp.Server.Version = s.version
	resp.Server.FileCount = len(merged)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		s.logger.Warn("[manifest] 编码响应失败: %v", err)
	}
}

// handleDownload serves a package file for download.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path is /api/v1/download/{filename}
	filename := strings.TrimPrefix(r.URL.Path, "/api/v1/download/")
	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	// Prevent path traversal
	fullPath, err := safeJoin(s.packagesDir, filename)
	if err != nil {
		s.logger.Warn("[download] 路径遍历检测: %s (来自 %s)", filename, r.RemoteAddr)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Check file exists and is not a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Online fallback: fetch the package on demand when not cached locally.
			if s.onlineFallback && s.onlineSrc != nil {
				s.logger.Info("[download] 本地未找到，触发在线回退: %s", filename)
				if localPath, ferr := s.fetchOnlinePackage(filename); ferr == nil {
					if dlInfo, statErr := os.Stat(localPath); statErr == nil {
						s.logger.Info("[online] 在线回退成功: %s (%s)", filename, util.FormatSize(dlInfo.Size()))
					} else {
						s.logger.Info("[online] 在线回退成功: %s", filename)
					}
					servePackageFile(w, r, localPath)
					return
				} else {
					s.logger.Warn("[online] 在线回退失败: %s: %v", filename, ferr)
					http.Error(w, "file not found", http.StatusNotFound)
					return
				}
			}
			s.logger.Debug("[download] 文件不存在: %s", filename)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		s.logger.Error("[download] 检查文件失败: %s: %v", filename, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		s.logger.Warn("[download] 请求的是目录: %s", filename)
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

	s.logger.Debug("[download] 本地命中: %s (%s)", filename, util.FormatSize(info.Size()))
	// Serve the file with Range support
	servePackageFile(w, r, fullPath)
}

// servePackageFile serves a package file for download with Range support.
func servePackageFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

// handleStatus returns the server status.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/v1/status" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	fileCount := len(s.files)
	totalSize := s.totalSize
	s.mu.RUnlock()

	uptime := int64(time.Since(s.startTime).Seconds())

	s.logger.Debug("[status] 返回服务状态: uptime=%ds files=%d size=%s", uptime, fileCount, util.FormatSize(totalSize))

	resp := StatusResponse{
		Status: "ok",
	}
	resp.Server.Name = serverName
	resp.Server.Version = s.version
	resp.Server.Uptime = uptime
	resp.Server.FileCount = fileCount
	resp.Server.TotalSize = totalSize

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		s.logger.Warn("[status] 编码响应失败: %v", err)
	}
}

// handleBin serves client binary files from the bin/ directory.
func (s *Server) handleBin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path is /api/v1/bin/{filename}
	filename := strings.TrimPrefix(r.URL.Path, "/api/v1/bin/")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	// Prevent path traversal
	fullPath, err := safeJoin(s.binDir, filename)
	if err != nil {
		s.logger.Warn("[bin] 路径遍历检测: %s", filename)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Check file exists and is not a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Debug("[bin] 文件不存在: %s", filename)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		s.logger.Error("[bin] 检查文件失败: %s: %v", filename, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		s.logger.Warn("[bin] 请求的是目录: %s", filename)
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

	s.logger.Debug("[bin] 发送: %s (%s)", filename, util.FormatSize(info.Size()))

	// Serve the file
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

// handleRoot returns the HTML help page.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" {
		s.logger.Debug("[root] 未找到路径: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// List bin directory contents
	binFiles := listBinFiles(s.binDir)

	s.mu.RLock()
	fileCount := len(s.files)
	totalSize := s.totalSize
	s.mu.RUnlock()

	// Build base URL
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, host)

	// Build template data
	binFileViews := make([]binFileView, len(binFiles))
	for i, bf := range binFiles {
		platLabel := bf.Platform
		if bf.Arch != "" {
			platLabel += " " + bf.Arch
		}
		binFileViews[i] = binFileView{
			Name:          bf.Filename,
			File:          bf.Filename,
			Platform:      bf.Platform,
			Arch:          bf.Arch,
			PlatformLabel: platLabel,
			Size:          util.FormatSize(bf.Size),
		}
	}

	data := pageData{
		Version:     s.version,
		ServerName:  serverName,
		Description: "多版本浏览器管理工具，支持本地导入、远程下载、版本切换、隔离运行。",
		Features: []string{
			"多版本管理：同时安装和管理多个浏览器版本，支持版本前缀快速筛选",
			"本地导入：支持 zip、7z、tar.gz 等多种格式自动识别导入",
			"远程下载：从官方源下载指定版本（Firefox FTP）",
			"离线分发：局域网浏览器版本分发服务，支持自动同步",
			"隔离运行：每个版本独立 Profile，互不干扰",
			"便携模式：数据存储在 bws-data/ 子目录，U 盘随身携带",
		},
		FileCount:      fileCount,
		TotalSize:      util.FormatSize(totalSize),
		BinFiles:       binFileViews,
		BaseURL:        baseURL,
		SyncEnabled:    s.syncMgr != nil,
		OnlineFallback: s.onlineFallback,
	}

	// Parse and execute template
	tmpl, err := template.ParseFS(pageHTML, "page.html")
	if err != nil {
		s.logger.Error("[root] 模板解析失败: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		// Headers already written, just log the error
		s.logger.Warn("[root] 模板渲染错误: %v", err)
	}
}

// --- Helpers ---

// binFile describes a file in the bin/ directory.
type binFile struct {
	Filename string
	Platform string
	Arch     string
	Size     int64
}

// listBinFiles returns a list of files in the bin/ directory with detected platform/arch.
func listBinFiles(binDir string) []binFile {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil
	}

	var files []binFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		filename := entry.Name()
		platform, arch := detectPlatformArch(filename)
		files = append(files, binFile{
			Filename: filename,
			Platform: platform,
			Arch:     arch,
			Size:     info.Size(),
		})
	}

	// Sort by filename
	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})

	return files
}

// detectPlatformArch detects platform and architecture from a binary filename.
// Returns canonical names: windows/darwin/linux and amd64/386/arm64.
func detectPlatformArch(filename string) (string, string) {
	lower := strings.ToLower(filename)

	// Platform detection — specific patterns first, then generic.
	// "darwin" contains substring "win", so darwin/macos must be checked before windows.
	platform := "unknown"
	switch {
	case strings.Contains(lower, "darwin") || strings.Contains(lower, "macos") || strings.Contains(lower, "_mac") || strings.Contains(lower, "-mac"):
		platform = "darwin"
	case strings.Contains(lower, "linux"):
		platform = "linux"
	case strings.Contains(lower, ".exe") || strings.Contains(lower, "win") || strings.Contains(lower, "windows"):
		platform = "windows"
	}

	// Arch detection — returns canonical arch names.
	arch := ""
	switch {
	case strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64"):
		arch = "arm64"
	case strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64"):
		arch = "amd64"
	case strings.Contains(lower, "x86") || strings.Contains(lower, "i386") || strings.Contains(lower, "386"):
		arch = "386"
	}

	return platform, arch
}

// safeJoin joins baseDir and name, ensuring the result is within baseDir.
// Returns an error if path traversal is detected.
func safeJoin(baseDir, name string) (string, error) {
	// Quick check for obvious traversal patterns
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("path traversal detected")
	}

	fullPath := filepath.Join(baseDir, name)

	// Resolve to absolute paths for comparison
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	// Ensure the resolved path is within the base directory
	// Use a separator to avoid prefix match issues (e.g. /base vs /base-other)
	rel, err := filepath.Rel(absBase, absFull)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path traversal detected")
	}

	return absFull, nil
}

// --- Online fallback ---

// mergeManifest merges local and online package lists, deduplicating by
// filename. Local entries take priority because they carry real checksums.
// However, when a local entry has incomplete metadata (e.g. platform/arch
// could not be parsed from the filename), the missing fields are filled in
// from the corresponding online entry so that client-side filtering works.
func mergeManifest(local, online []PackageFile) []PackageFile {
	// Build a lookup of online entries by filename for metadata backfill.
	onlineByName := make(map[string]PackageFile, len(online))
	for _, f := range online {
		if f.Filename != "" {
			onlineByName[f.Filename] = f
		}
	}

	seen := make(map[string]bool, len(local)+len(online))
	merged := make([]PackageFile, 0, len(local)+len(online))
	for _, f := range local {
		if f.Filename == "" || seen[f.Filename] {
			continue
		}
		seen[f.Filename] = true
		// Backfill incomplete metadata from the online entry.
		if online, ok := onlineByName[f.Filename]; ok {
			if f.Platform == "" || f.Platform == "unknown" {
				f.Platform = online.Platform
			}
			if f.Architecture == "" || f.Architecture == "unknown" {
				f.Architecture = online.Architecture
			}
			if f.Channel == "" {
				f.Channel = online.Channel
			}
			if f.Browser == "" {
				f.Browser = online.Browser
			}
			if f.Version == "" || f.Version == "unknown" {
				f.Version = online.Version
				f.MajorVersion = online.MajorVersion
			}
		}
		merged = append(merged, f)
	}
	for _, f := range online {
		if f.Filename == "" || seen[f.Filename] {
			continue
		}
		seen[f.Filename] = true
		merged = append(merged, f)
	}
	return merged
}

// onlineFilename derives the local filename for an online version. It prefers
// the explicit Filename field and falls back to the URL-decoded base name of
// the download URL (so URL-encoded names like "Firefox%20Setup%20..." become
// the real "Firefox Setup ...").
func onlineFilename(v SyncVersionInfo) string {
	if v.Filename != "" {
		return v.Filename
	}
	base := filepath.Base(v.DownloadURL)
	if decoded, err := url.PathUnescape(base); err == nil && decoded != "" {
		return decoded
	}
	return base
}

// fetchOnlinePackage downloads a package on demand from the online source and
// returns its local path. Concurrent requests for the same filename are
// deduplicated: only the first requester performs the download, the rest wait
// and reuse the result.
func (s *Server) fetchOnlinePackage(filename string) (string, error) {
	// Deduplicate concurrent downloads of the same filename.
	s.dlMu.Lock()
	if ch, ok := s.dlInflight[filename]; ok {
		s.dlMu.Unlock()
		s.logger.Debug("[online] 等待并发下载完成: %s", filename)
		<-ch // wait for the in-flight download to finish
		destPath := filepath.Join(s.packagesDir, filename)
		if info, err := os.Stat(destPath); err == nil && !info.IsDir() {
			s.logger.Debug("[online] 并发下载已完成，复用文件: %s", filename)
			return destPath, nil
		}
		return "", fmt.Errorf("在线获取失败（并发请求未完成下载）")
	}
	ch := make(chan struct{})
	s.dlInflight[filename] = ch
	s.dlMu.Unlock()

	err := s.doOnlineDownload(filename)

	// Signal waiters and clean up regardless of outcome.
	s.dlMu.Lock()
	delete(s.dlInflight, filename)
	s.dlMu.Unlock()
	close(ch)

	if err != nil {
		return "", err
	}
	return filepath.Join(s.packagesDir, filename), nil
}

// doOnlineDownload resolves the download URL for filename and downloads it
// into the local packages directory, then refreshes the manifest so the new
// file (with its checksum) becomes visible.
func (s *Server) doOnlineDownload(filename string) error {
	info, err := s.findOnlinePackage(filename)
	if err != nil {
		return err
	}

	s.logger.Info("[online] 开始在线下载: %s", filename)
	s.logger.Debug("[online] 下载详情: url=%s browser=%s version=%s platform=%s arch=%s size=%s",
		info.url, info.browser, info.version, info.platform, info.arch, util.FormatSize(info.size))

	// SyncSource.Download has its own 30-minute timeout via DefaultDownload.
	downloadStart := time.Now()
	downloadedPath, err := s.onlineSrc.Download(info.url, s.packagesDir, nil)
	if err != nil {
		s.logger.Warn("[online] 下载失败: %s: %v (耗时 %v)", filename, err, time.Since(downloadStart).Round(time.Millisecond))
		return fmt.Errorf("下载失败: %w", err)
	}

	s.logger.Info("[online] 下载完成: %s (耗时 %v)", filename, time.Since(downloadStart).Round(time.Millisecond))

	// The downloaded file may be named after the (possibly URL-encoded) URL
	// base. Rename it to the canonical filename clients request by.
	destPath := filepath.Join(s.packagesDir, filename)
	if downloadedPath != destPath {
		s.logger.Debug("[online] 重命名文件: %s -> %s", filepath.Base(downloadedPath), filename)
		// Remove a stale destination if it exists, then rename.
		_ = os.Remove(destPath)
		if err := os.Rename(downloadedPath, destPath); err != nil {
			// Rename can fail across volumes/devices; fall back to a copy.
			s.logger.Debug("[online] 重命名失败，尝试复制: %s -> %s", downloadedPath, destPath)
			if _, copyErr := util.CopyFile(downloadedPath, destPath); copyErr != nil {
				return fmt.Errorf("重命名下载文件失败: %w", copyErr)
			}
			_ = os.Remove(downloadedPath)
		}
	}

	// Verify SHA256 checksum if the online source provided one.
	if info.sha256 != "" {
		f, err := os.Open(destPath)
		if err != nil {
			s.logger.Warn("[online] SHA256 校验失败: 无法打开文件: %s: %v", filename, err)
			_ = os.Remove(destPath)
			return fmt.Errorf("SHA256 校验失败: 无法打开文件: %w", err)
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			s.logger.Warn("[online] SHA256 校验失败: 读取文件错误: %s: %v", filename, err)
			_ = os.Remove(destPath)
			return fmt.Errorf("SHA256 校验失败: 读取文件错误: %w", err)
		}
		f.Close()
		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != info.sha256 {
			s.logger.Warn("[online] SHA256 校验失败: %s: 预期=%s 实际=%s", filename, info.sha256, actualHash)
			_ = os.Remove(destPath)
			return fmt.Errorf("SHA256 校验失败: %s: 预期=%s 实际=%s", filename, info.sha256, actualHash)
		}
		s.logger.Debug("[online] SHA256 校验通过: %s", filename)
	}

	// Log final file size.
	if fi, statErr := os.Stat(destPath); statErr == nil {
		s.logger.Debug("[online] 文件已就绪: %s (%s)", destPath, util.FormatSize(fi.Size()))
	}

	// Re-scan packages so the new file gets a checksum and is added to the
	// manifest. The checksum cache makes this cheap for unchanged files.
	s.logger.Debug("[online] 重新扫描软件包目录以更新清单...")
	cache, _ := s.loadCache()
	if err := s.scanPackages(cache); err != nil {
		s.logger.Warn("[online] 重新扫描软件包失败: %v", err)
	}
	_ = s.saveCache(cache)
	s.logger.Debug("[online] 清单已更新")

	return nil
}

// findOnlinePackage looks up the download info for a filename, refreshing the
// online cache on miss or when the download URL is missing (e.g. cache was
// loaded from an older disk file without URL data).
func (s *Server) findOnlinePackage(filename string) (onlinePackage, error) {
	info, ok := s.onlineCacheMgr.FindPackage(filename)
	if ok && info.url != "" {
		return info, nil
	}

	// Not in cache or URL missing: refresh all browsers so we can resolve
	// any filename regardless of which browser/platform/arch it belongs to.
	s.onlineCacheMgr.RefreshAll()

	info, ok = s.onlineCacheMgr.FindPackage(filename)
	if !ok {
		return onlinePackage{}, fmt.Errorf("在线源中未找到文件: %s", filename)
	}
	if info.url == "" {
		return onlinePackage{}, fmt.Errorf("在线源中未找到 %s 的下载地址: %s", filename, filename)
	}
	return info, nil
}

// --- Sync API Handlers ---

// handleSyncStatus returns the current sync status.
func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/v1/sync/status" {
		http.NotFound(w, r)
		return
	}

	s.logger.Debug("[sync] 查询同步状态 (来自 %s)", r.RemoteAddr)

	var status SyncStatus
	if s.syncMgr != nil {
		status = s.syncMgr.Status()
		s.logger.Debug("[sync] 状态: running=%v synced=%d/%d lastSync=%s",
			status.Running, status.SyncedFiles, status.TotalFiles, status.LastSync.Format("2006-01-02 15:04:05"))
	} else {
		status = SyncStatus{
			Running:  false,
			Progress: "同步未启用（未配置同步源）",
		}
		s.logger.Debug("[sync] 同步未启用")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(struct {
		Status string     `json:"status"`
		Data   SyncStatus `json:"data"`
	}{
		Status: "ok",
		Data:   status,
	})
}

// handleSyncTrigger triggers an immediate sync.
func (s *Server) handleSyncTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/v1/sync/trigger" {
		http.NotFound(w, r)
		return
	}

	if s.syncMgr == nil {
		s.logger.Warn("[sync] 触发同步失败：同步未启用 (来自 %s)", r.RemoteAddr)
		http.Error(w, "sync not enabled", http.StatusServiceUnavailable)
		return
	}

	s.logger.Info("[sync] 收到手动触发同步请求 (来自 %s)", r.RemoteAddr)
	s.syncMgr.Trigger()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "ok",
		Message: "同步已触发",
	})
}
