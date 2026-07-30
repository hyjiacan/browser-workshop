// Package serve provides an HTTP server for hosting browser version packages.
// It serves a directory of browser installers following a standard layout
// and provides a manifest API for clients to discover available versions.
//
// API v1:
//   - GET /api/v1/manifest     - 文件清单（含 XXH3 校验和）
//   - GET /api/v1/download/{filename} - 文件下载（支持 Range 断点续传）
//   - GET /api/v1/status       - 服务状态
//   - GET /                    - HTML 帮助页
//   - GET /bin/{filename}      - 客户端二进制下载
package serve

import (
	"context"
	"embed"
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
	"github.com/bws/bws/internal/version"
	"github.com/zeebo/xxh3"
)

//go:embed page.html
var pageHTML embed.FS

const (
	serverName   = "bws-serve"
	cacheVersion = 1

	// onlineCacheTTL is how long the online-version cache is considered fresh.
	onlineCacheTTL = 10 * time.Minute

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
	onlineBrowsers []string
	onlineChannels []string

	// onlineMu protects the online cache fields below.
	onlineMu        sync.RWMutex
	onlineFiles     map[string]onlinePackage // filename -> download info
	onlineList      []PackageFile            // cached manifest entries from online source
	onlineCacheTime time.Time
	// onlineRefreshMu serializes cache refreshes so concurrent requests
	// don't all hit the online source at once.
	onlineRefreshMu sync.Mutex

	// dlMu + dlInflight deduplicate concurrent on-demand downloads of the
	// same filename: the first requester downloads, others wait.
	dlMu       sync.Mutex
	dlInflight map[string]chan struct{}
}

// PackageFile represents a single package file with its metadata.
type PackageFile struct {
	Filename     string `json:"filename"`
	Version      string `json:"version"`
	MajorVersion string `json:"major_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
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
	// building the fallback manifest. If empty, defaults to [chrome, firefox].
	OnlineBrowsers []string

	// OnlineChannels is the list of channels to query from OnlineSource when
	// building the fallback manifest. If empty, defaults to [stable].
	OnlineChannels []string

	// ScanWorkers is the number of worker goroutines used for parallel
	// checksum computation during package scanning. 0 means auto
	// (runtime.NumCPU()). Values are clamped to [1, 32].
	ScanWorkers int

	// ConfigPath is the path to the bws-serve.ini config file. It is only
	// used for informational startup logging. If empty, it is derived from
	// BaseDir.
	ConfigPath string

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

	// configPath is used only for startup logging. Default to bws-serve.ini in baseDir.
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(baseDir, "bws-serve.ini")
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
		onlineBrowsers: opts.OnlineBrowsers,
		onlineChannels: opts.OnlineChannels,
		scanWorkers:    opts.ScanWorkers,
		dlInflight:     make(map[string]chan struct{}),
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
	// Print startup banner before doing any heavy work so the user sees that
	// the server is coming up. scanPackages and printStartupInfo add the
	// detailed progress and summary afterwards.
	fmt.Println("bws serve 正在启动...")
	fmt.Printf("  正在加载配置: %s\n", s.configPath)
	fmt.Printf("  正在扫描软件包目录: %s\n", s.packagesDir)

	// Ensure directories exist
	if err := os.MkdirAll(s.packagesDir, 0o755); err != nil {
		return fmt.Errorf("creating packages directory: %w", err)
	}
	if err := os.MkdirAll(s.binDir, 0o755); err != nil {
		return fmt.Errorf("creating bin directory: %w", err)
	}

	// Load cache
	cache, err := s.loadCache()
	if err != nil {
		cache = make(map[string]cacheEntry)
	}

	// Scan packages and compute checksums
	if err := s.scanPackages(cache); err != nil {
		return fmt.Errorf("scanning packages: %w", err)
	}

	// Save updated cache
	if err := s.saveCache(cache); err != nil {
		s.logger.Warn("保存缓存失败: %v", err)
	}

	s.startTime = time.Now()

	// Start sync manager
	if s.syncMgr != nil {
		s.syncMgr.Start()
	}

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/manifest", s.handleManifest)
	mux.HandleFunc("/api/v1/download/", s.handleDownload)
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/sync/status", s.handleSyncStatus)
	mux.HandleFunc("/api/v1/sync/trigger", s.handleSyncTrigger)
	mux.HandleFunc("/bin/", s.handleBin)
	mux.HandleFunc("/", s.handleRoot)

	s.httpSrv = &http.Server{
		Addr:         s.addr,
		Handler:      &loggingHandler{next: mux, logger: s.logger, name: serverName},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.printStartupInfo()

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
	// Stop sync manager first
	if s.syncMgr != nil {
		s.syncMgr.Stop()
	}
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// printStartupInfo prints server startup information.
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
	fmt.Printf("  软件包数量:   %d 个 (%s)\n", fileCount, formatSize(totalSize))
	if s.onlineFallback && s.onlineSrc != nil {
		fmt.Printf("  在线回退:     已启用（本地缺失时自动从在线源获取）\n")
	}
	fmt.Println()
	fmt.Println("  API 接口:")
	fmt.Printf("    GET /                    - HTML 帮助页面\n")
	fmt.Printf("    GET /api/v1/manifest     - 软件包清单 (JSON)\n")
	fmt.Printf("    GET /api/v1/download/    - 软件包下载\n")
	fmt.Printf("    GET /api/v1/status       - 服务状态\n")
	fmt.Printf("    GET /bin/                - 客户端二进制文件\n")
	fmt.Println()
	fmt.Println("  客户端配置:")
	fmt.Printf("    bws config set source http://<服务器地址>:<端口>\n")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止服务。")
	fmt.Println()
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
	if cache == nil {
		cache = make(map[string]cacheEntry)
	}
	s.logger.Debug("正在扫描软件包目录: %s", s.packagesDir)

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

	fmt.Printf("  正在扫描文件...\n")

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
			s.logger.Debug("跳过不支持的文件类型: %s", d.Name())
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
			s.logger.Debug("软件包目录不存在，跳过扫描")
			return nil
		}
		return fmt.Errorf("scanning packages directory: %w", err)
	}

	if len(allFiles) == 0 {
		s.logger.Debug("未找到任何文件")
		s.mu.Lock()
		s.files = nil
		s.totalSize = 0
		s.mu.Unlock()
		return nil
	}

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
		fmt.Printf("  正在计算校验和（%d 个文件需要处理，使用 %d 个线程）...\n", len(misses), workers)

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
					s.logger.Debug("计算校验和: %s", j.relPath)
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
				s.logger.Warn("计算 %s 校验和失败: %v", r.relPath, r.err)
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

	s.mu.Lock()
	s.files = files
	s.totalSize = totalSize
	s.mu.Unlock()

	computedSuccessfully := len(files) - cacheHits
	fmt.Printf("  扫描完成: 共 %d 个文件 (缓存命中 %d, 新计算 %d, 线程数 %d", len(files), cacheHits, computedSuccessfully, numWorkers)
	if skippedUnsupported > 0 {
		fmt.Printf(", 跳过 %d 个非安装包文件", skippedUnsupported)
	}
	fmt.Println(")")

	return nil
}

// parsePackageFile parses a filename to extract metadata and returns a PackageFile.
func (s *Server) parsePackageFile(scanner *repo.Scanner, filename string, size int64, checksum string) PackageFile {
	// Strip extension for matching
	nameNoExt := stripExtension(filename)

	// Use scanner to detect metadata
	match := scanner.ScanEntry(nameNoExt, filename, true, "", "")

	pkg := PackageFile{
		Filename: filename,
		Size:     size,
		Checksum: "xxh3:" + checksum,
	}

	if match.Version != "" {
		pkg.Version = match.Version
		pkg.MajorVersion = strconv.Itoa(version.Major(match.Version))
	} else {
		pkg.Version = "unknown"
		pkg.MajorVersion = "0"
	}

	if match.Platform != "" {
		pkg.Platform = normalizePlatform(match.Platform)
	} else {
		pkg.Platform = "unknown"
	}

	if match.Arch != "" {
		pkg.Architecture = normalizeArch(match.Arch)
	} else {
		pkg.Architecture = "unknown"
	}

	return pkg
}

// normalizePlatform converts scanner platform names to serve API names.
func normalizePlatform(p string) string {
	switch p {
	case "darwin":
		return "macos"
	default:
		return p
	}
}

// normalizeArch converts scanner arch names to serve API names.
func normalizeArch(a string) string {
	switch a {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	default:
		return a
	}
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

	return os.WriteFile(s.cachePath, data, 0o600)
}

// --- HTTP Handlers ---

// handleManifest returns the package manifest as JSON.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/v1/manifest" {
		http.NotFound(w, r)
		return
	}

	// Parse optional filter query parameters: browser, platform, arch, channel.
	query := r.URL.Query()
	browserFilter := strings.ToLower(strings.TrimSpace(query.Get("browser")))
	platformFilter := strings.ToLower(strings.TrimSpace(query.Get("platform")))
	archFilter := strings.ToLower(strings.TrimSpace(query.Get("arch")))
	channelFilter := strings.ToLower(strings.TrimSpace(query.Get("channel")))

	s.mu.RLock()
	files := make([]PackageFile, len(s.files))
	copy(files, s.files)
	s.mu.RUnlock()

	// Apply local filtering based on the query parameters. Local files only
	// carry platform/arch metadata (no channel), so channel filtering only
	// affects the online portion.
	if browserFilter != "" || platformFilter != "" || archFilter != "" {
		filtered := files[:0]
		for _, f := range files {
			if browserFilter != "" {
				if !strings.Contains(strings.ToLower(f.Filename), browserFilter) {
					continue
				}
			}
			if platformFilter != "" {
				if f.Platform != platformFilter && !(platformFilter == "darwin" && f.Platform == "macos") && !(platformFilter == "macos" && f.Platform == "darwin") {
					continue
				}
			}
			if archFilter != "" {
				if f.Architecture != archFilter && !(archFilter == "amd64" && f.Architecture == "x64") && !(archFilter == "x64" && f.Architecture == "amd64") && !(archFilter == "386" && f.Architecture == "x86") && !(archFilter == "x86" && f.Architecture == "386") {
					continue
				}
			}
			filtered = append(filtered, f)
		}
		files = filtered
	}

	// When online fallback is enabled, merge in versions available from the
	// online source so clients can discover (and on-demand download) packages
	// that are not yet cached locally. Only the requested browser is queried
	// (when a browser filter is present) to avoid hitting every configured
	// browser on each request.
	fileCount := len(files)
	if s.onlineFallback && s.onlineSrc != nil {
		online := s.getOnlinePackages(browserFilter)
		// Apply platform/arch/channel filtering to the online list.
		if platformFilter != "" || archFilter != "" || channelFilter != "" {
			filtered := online[:0]
			for _, f := range online {
				if platformFilter != "" {
					if f.Platform != platformFilter && !(platformFilter == "darwin" && f.Platform == "macos") && !(platformFilter == "macos" && f.Platform == "darwin") {
						continue
					}
				}
				if archFilter != "" {
					if f.Architecture != archFilter && !(archFilter == "amd64" && f.Architecture == "x64") && !(archFilter == "x64" && f.Architecture == "amd64") && !(archFilter == "386" && f.Architecture == "x86") && !(archFilter == "x86" && f.Architecture == "386") {
						continue
					}
				}
				if channelFilter != "" {
					if !strings.Contains(strings.ToLower(f.Filename), channelFilter) {
						continue
					}
				}
				filtered = append(filtered, f)
			}
			online = filtered
		}
		files = mergeManifest(files, online)
		fileCount = len(files)
	}

	resp := ManifestResponse{
		Status: "ok",
		Data:   files,
	}
	resp.Server.Name = serverName
	resp.Server.Version = s.version
	resp.Server.FileCount = fileCount

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		s.logger.Warn("编码 manifest 响应失败: %v", err)
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
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Check file exists and is not a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Online fallback: fetch the package on demand when not cached locally.
			if s.onlineFallback && s.onlineSrc != nil {
				if localPath, ferr := s.fetchOnlinePackage(filename); ferr == nil {
					servePackageFile(w, r, localPath)
					return
				} else {
					s.logger.Warn("在线回退获取 %s 失败: %v", filename, ferr)
					http.Error(w, "file not found", http.StatusNotFound)
					return
				}
			}
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

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
		s.logger.Warn("编码 status 响应失败: %v", err)
	}
}

// handleBin serves client binary files from the bin/ directory.
func (s *Server) handleBin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path is /bin/{filename}
	filename := strings.TrimPrefix(r.URL.Path, "/bin/")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	// Prevent path traversal
	fullPath, err := safeJoin(s.binDir, filename)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Check file exists and is not a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

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
			Size:          formatSize(bf.Size),
		}
	}

	data := pageData{
		Version:        s.version,
		ServerName:     serverName,
		Description:    "多版本浏览器管理工具，支持本地导入、远程下载、版本切换、隔离运行。",
		Features: []string{
			"多版本管理：同时安装和管理多个浏览器版本，支持版本前缀快速筛选",
			"本地导入：支持 zip、7z、tar.gz 等多种格式自动识别导入",
			"远程下载：从官方源下载指定版本（Chrome Omaha、Firefox FTP）",
			"离线分发：局域网浏览器版本分发服务，支持自动同步",
			"隔离运行：每个版本独立 Profile，互不干扰",
			"便携模式：数据存储在 bws-data/ 子目录，U 盘随身携带",
		},
		FileCount:      fileCount,
		TotalSize:      formatSize(totalSize),
		BinFiles:       binFileViews,
		BaseURL:        baseURL,
		SyncEnabled:    s.syncMgr != nil,
		OnlineFallback: s.onlineFallback,
	}

	// Parse and execute template
	tmpl, err := template.ParseFS(pageHTML, "page.html")
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		// Headers already written, just log the error
		s.logger.Warn("模板渲染错误: %v", err)
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
func detectPlatformArch(filename string) (string, string) {
	lower := strings.ToLower(filename)

	// Platform detection — specific patterns first, then generic.
	// "darwin" contains substring "win", so darwin/macos must be checked before windows.
	platform := "unknown"
	switch {
	case strings.Contains(lower, "darwin") || strings.Contains(lower, "macos") || strings.Contains(lower, "_mac") || strings.Contains(lower, "-mac"):
		platform = "macos"
	case strings.Contains(lower, "linux"):
		platform = "linux"
	case strings.Contains(lower, ".exe") || strings.Contains(lower, "win") || strings.Contains(lower, "windows"):
		platform = "windows"
	}

	// Arch detection
	arch := ""
	switch {
	case strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64"):
		arch = "arm64"
	case strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64"):
		arch = "x64"
	case strings.Contains(lower, "x86") || strings.Contains(lower, "i386") || strings.Contains(lower, "386"):
		arch = "x86"
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

// installerExtensions lists known installer/archive extensions that should be stripped.
// Order matters: compound extensions like .tar.gz must come before .gz.
var installerExtensions = []string{
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".exe",
	".msi",
	".zip",
	".7z",
	".rar",
	".dmg",
	".pkg",
	".deb",
	".rpm",
	".apk",
	".gz",
	".bz2",
	".xz",
}

// stripExtension removes known installer/archive extensions from a filename.
// If no known extension is found, it removes the last extension using filepath.Ext.
func stripExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range installerExtensions {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	// Fallback: remove last extension
	ext := filepath.Ext(name)
	if ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

// formatSize formats a byte count for display.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// --- Online fallback ---

// mergeManifest merges local and online package lists, deduplicating by
// filename. Local entries take priority because they carry real checksums.
func mergeManifest(local, online []PackageFile) []PackageFile {
	seen := make(map[string]bool, len(local)+len(online))
	merged := make([]PackageFile, 0, len(local)+len(online))
	for _, f := range local {
		if f.Filename == "" || seen[f.Filename] {
			continue
		}
		seen[f.Filename] = true
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

// getOnlinePackages returns the cached online package entries, refreshing the
// cache from the online source if it is stale. Refreshes are serialized so
// concurrent requests share a single network query round.
//
// When browserFilter is non-empty, only that browser is queried from the
// online source (the cached result is also filtered to that browser). When
// empty, all configured browsers are queried (preserving the prior behavior).
// Note: the cache is keyed only by whether a filter is present, not by the
// specific browser value, so switching browsers within the TTL window does
// not trigger a re-query but returns a filtered view of the cached entries.
func (s *Server) getOnlinePackages(browserFilter string) []PackageFile {
	s.onlineMu.RLock()
	if !s.onlineCacheTime.IsZero() && time.Since(s.onlineCacheTime) < onlineCacheTTL {
		list := s.onlineList
		s.onlineMu.RUnlock()
		// Return a copy to avoid callers mutating the cached slice.
		out := filterOnlineListByBrowser(list, browserFilter)
		return out
	}
	s.onlineMu.RUnlock()

	// Serialize refreshes.
	s.onlineRefreshMu.Lock()
	defer s.onlineRefreshMu.Unlock()

	// Double-check after acquiring the refresh lock: another goroutine may
	// have just refreshed the cache.
	s.onlineMu.RLock()
	if !s.onlineCacheTime.IsZero() && time.Since(s.onlineCacheTime) < onlineCacheTTL {
		list := s.onlineList
		s.onlineMu.RUnlock()
		out := filterOnlineListByBrowser(list, browserFilter)
		return out
	}
	s.onlineMu.RUnlock()

	s.doRefreshOnlineCache(browserFilter)

	s.onlineMu.RLock()
	defer s.onlineMu.RUnlock()
	return filterOnlineListByBrowser(s.onlineList, browserFilter)
}

// filterOnlineListByBrowser returns a copy of list restricted to entries whose
// filename contains the given browser keyword. When browserFilter is empty,
// a full copy is returned. This is used so a single cached list (built from
// all configured browsers) can still serve browser-specific requests without
// a re-query.
func filterOnlineListByBrowser(list []PackageFile, browserFilter string) []PackageFile {
	if browserFilter == "" {
		out := make([]PackageFile, len(list))
		copy(out, list)
		return out
	}
	out := make([]PackageFile, 0, len(list))
	for _, f := range list {
		if strings.Contains(strings.ToLower(f.Filename), browserFilter) {
			out = append(out, f)
		}
	}
	return out
}

// doRefreshOnlineCache queries the online source for all configured
// browser/channel/platform/arch combinations and rebuilds the cache.
// Failures for individual combinations are logged and skipped so a single
// broken source does not invalidate the whole cache.
//
// When browserFilter is non-empty, only that browser is queried (instead of
// all configured browsers). This keeps a browser-specific request from
// triggering expensive queries for every other configured browser.
func (s *Server) doRefreshOnlineCache(browserFilter string) {
	browsers := s.onlineBrowsers
	if len(browsers) == 0 {
		browsers = []string{"chrome", "firefox"}
	}
	// If a specific browser filter is requested, only query that browser so
	// we avoid hitting every configured browser on each request.
	if browserFilter != "" {
		browsers = []string{browserFilter}
	}
	channels := s.onlineChannels
	if len(channels) == 0 {
		channels = []string{"stable"}
	}

	filesMap := make(map[string]onlinePackage)
	var list []PackageFile

	for _, b := range browsers {
		for _, ch := range channels {
			for _, combo := range defaultOnlineCombos {
				// ListVersions is not context-aware; bound it with a timeout
				// via a goroutine so a slow source cannot stall the request.
				versions, err := s.listVersionsWithTimeout(b, ch, combo.platform, combo.arch)
				if err != nil {
					s.logger.Warn("在线获取 %s/%s/%s/%s 版本失败: %v",
						b, ch, combo.platform, combo.arch, err)
					continue
				}
				for _, v := range versions {
					fname := onlineFilename(v)
					if fname == "" || v.DownloadURL == "" {
						continue
					}
					if _, exists := filesMap[fname]; exists {
						continue
					}
					filesMap[fname] = onlinePackage{
						url:      v.DownloadURL,
						browser:  b,
						version:  v.Version,
						platform: combo.platform,
						arch:     combo.arch,
						size:     v.Size,
					}
					list = append(list, s.onlinePackageFile(fname, v, combo.platform, combo.arch))
				}
			}
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Filename < list[j].Filename
	})

	s.onlineMu.Lock()
	s.onlineFiles = filesMap
	s.onlineList = list
	s.onlineCacheTime = time.Now()
	s.onlineMu.Unlock()
}

// listVersionsWithTimeout calls onlineSrc.ListVersions with a timeout so a
// hanging source cannot block the manifest/download handlers indefinitely.
func (s *Server) listVersionsWithTimeout(browser, channel, platform, arch string) ([]SyncVersionInfo, error) {
	type result struct {
		versions []SyncVersionInfo
		err      error
	}
	ch := make(chan result, 1)
	go func() {
		versions, err := s.onlineSrc.ListVersions(browser, channel, platform, arch)
		ch <- result{versions, err}
	}()
	select {
	case r := <-ch:
		return r.versions, r.err
	case <-time.After(onlineListTimeout):
		return nil, fmt.Errorf("查询超时（超过 %s）", onlineListTimeout)
	}
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

// onlinePackageFile builds a PackageFile entry for an online-available
// version. The checksum is empty until the file is actually downloaded
// locally; the size is taken from the source when known.
func (s *Server) onlinePackageFile(filename string, v SyncVersionInfo, platform, arch string) PackageFile {
	pkg := PackageFile{
		Filename: filename,
		Version:  v.Version,
		Size:     v.Size,
	}
	if v.Version != "" {
		pkg.MajorVersion = strconv.Itoa(version.Major(v.Version))
	} else {
		pkg.MajorVersion = "0"
	}
	pkg.Platform = normalizePlatform(platform)
	pkg.Architecture = normalizeArch(arch)
	return pkg
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
		<-ch // wait for the in-flight download to finish
		destPath := filepath.Join(s.packagesDir, filename)
		if info, err := os.Stat(destPath); err == nil && !info.IsDir() {
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

	s.logger.Info("在线回退: 正在下载 %s ...", filename)

	// SyncSource.Download has its own 30-minute timeout via DefaultDownload.
	downloadedPath, err := s.onlineSrc.Download(info.url, s.packagesDir, nil)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	// The downloaded file may be named after the (possibly URL-encoded) URL
	// base. Rename it to the canonical filename clients request by.
	destPath := filepath.Join(s.packagesDir, filename)
	if downloadedPath != destPath {
		// Remove a stale destination if it exists, then rename.
		_ = os.Remove(destPath)
		if err := os.Rename(downloadedPath, destPath); err != nil {
			// Rename can fail across volumes/devices; fall back to a copy.
			if copyErr := copyFile(downloadedPath, destPath); copyErr != nil {
				return fmt.Errorf("重命名下载文件失败: %w", copyErr)
			}
			_ = os.Remove(downloadedPath)
		}
	}

	// Re-scan packages so the new file gets a checksum and is added to the
	// manifest. The checksum cache makes this cheap for unchanged files.
	cache, _ := s.loadCache()
	if err := s.scanPackages(cache); err != nil {
		s.logger.Warn("重新扫描软件包失败: %v", err)
	}
	_ = s.saveCache(cache)

	return nil
}

// findOnlinePackage looks up the download info for a filename, refreshing the
// online cache on miss.
func (s *Server) findOnlinePackage(filename string) (onlinePackage, error) {
	s.onlineMu.RLock()
	info, ok := s.onlineFiles[filename]
	s.onlineMu.RUnlock()
	if ok {
		return info, nil
	}

	// Not in cache: refresh and try again. Pass an empty browser filter so we
	// query all configured browsers (the filename alone doesn't tell us which
	// browser it belongs to, and a download miss should still be resolvable).
	s.getOnlinePackages("")

	s.onlineMu.RLock()
	defer s.onlineMu.RUnlock()
	info, ok = s.onlineFiles[filename]
	if !ok {
		return onlinePackage{}, fmt.Errorf("在线源中未找到文件: %s", filename)
	}
	return info, nil
}

// copyFile copies src to dst. Used as a fallback when os.Rename fails (e.g.
// cross-device). It does not preserve permissions beyond what the source has.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
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

	var status SyncStatus
	if s.syncMgr != nil {
		status = s.syncMgr.Status()
	} else {
		status = SyncStatus{
			Running:  false,
			Progress: "同步未启用（未配置同步源）",
		}
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
		http.Error(w, "sync not enabled", http.StatusServiceUnavailable)
		return
	}

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
