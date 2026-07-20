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
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/repo"
	"github.com/bws/bws/internal/version"
	"github.com/zeebo/xxh3"
)

const (
	serverName   = "Browser Workshop"
	cacheVersion = 1
)

// pageTemplate is the HTML template for the root help page.
const pageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.ServerName}} v{{.Version}}</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; color: #333; line-height: 1.6; }
        h1 { font-size: 28px; margin-bottom: 8px; color: #1a1a1a; }
        h2 { font-size: 20px; margin-top: 32px; margin-bottom: 12px; color: #2c2c2c; border-bottom: 1px solid #e5e5e5; padding-bottom: 6px; }
        .subtitle { color: #666; margin-bottom: 24px; font-size: 14px; }
        ul { list-style: none; padding: 0; }
        li { padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
        li:last-child { border-bottom: none; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        pre { background: #f6f8fa; padding: 16px; border-radius: 6px; overflow-x: auto; font-size: 13px; line-height: 1.5; }
        code { background: #f6f8fa; padding: 2px 6px; border-radius: 4px; font-size: 13px; }
        pre code { background: none; padding: 0; }
        .stats { display: flex; gap: 24px; margin-bottom: 16px; flex-wrap: wrap; }
        .stat-item { background: #f6f8fa; padding: 12px 20px; border-radius: 8px; }
        .stat-label { font-size: 12px; color: #666; text-transform: uppercase; letter-spacing: 0.5px; }
        .stat-value { font-size: 20px; font-weight: 600; color: #1a1a1a; }
        .api-list li { font-family: "SF Mono", Monaco, Consolas, monospace; font-size: 13px; }
        .api-method { display: inline-block; width: 50px; font-weight: 600; color: #0550ae; }
        .empty { color: #999; font-style: italic; }
        .platform-tag { display: inline-block; padding: 2px 8px; background: #e8f0fe; color: #1967d2; border-radius: 12px; font-size: 12px; margin-left: 8px; }
        .sync-section { background: #f6f8fa; padding: 16px; border-radius: 8px; margin-bottom: 16px; }
        .sync-status { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
        .sync-indicator { width: 10px; height: 10px; border-radius: 50%; background: #ccc; display: inline-block; }
        .sync-indicator.running { background: #f59e0b; animation: pulse 1.5s infinite; }
        .sync-indicator.idle { background: #10b981; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .sync-info { font-size: 13px; color: #666; }
        .sync-btn { background: #0066cc; color: white; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-size: 14px; }
        .sync-btn:hover { background: #0052a3; }
        .sync-btn:disabled { background: #999; cursor: not-allowed; }
        .sync-progress { font-size: 13px; color: #555; margin-top: 8px; }
        .sync-time { font-size: 12px; color: #888; }
    </style>
</head>
<body>
    <h1>{{.ServerName}}</h1>
    <p class="subtitle">服务端 v{{.Version}} &middot; API v1</p>

    <div class="stats">
        <div class="stat-item">
            <div class="stat-label">软件包</div>
            <div class="stat-value">{{.FileCount}}</div>
        </div>
        <div class="stat-item">
            <div class="stat-label">总大小</div>
            <div class="stat-value">{{.TotalSize}}</div>
        </div>
    </div>

    <div class="sync-section" id="syncSection" style="display:none;">
        <div class="sync-status">
            <span class="sync-indicator" id="syncIndicator"></span>
            <span id="syncStatusText">正在加载...</span>
            <button class="sync-btn" id="syncBtn" onclick="triggerSync()">立即同步</button>
        </div>
        <div class="sync-progress" id="syncProgress"></div>
        <div class="sync-time" id="syncTime"></div>
    </div>

    <h2>下载客户端</h2>
    {{if .BinFiles}}
    <ul>
        {{range .BinFiles}}
        <li><a href="/bin/{{.File}}">{{.Name}}</a><span class="platform-tag">{{.PlatformLabel}}</span> <span style="color:#999;font-size:12px;">({{.Size}})</span></li>
        {{end}}
    </ul>
    {{else}}
    <p class="empty">暂无可用的客户端二进制文件。</p>
    {{end}}

    <h2>配置</h2>
    <p>将此服务器添加为 Browser Workshop 的远程源：</p>
    <pre><code>bws config set source {{.BaseURL}}</code></pre>

    <h2>开始使用</h2>
    <p>查看可用的浏览器版本：</p>
    <pre><code>bws ls --remote chrome

# 安装指定版本
bws install chrome@120.0.6099.109

# 运行浏览器
bws run chrome@120</code></pre>

    <h2>API 参考</h2>
    <ul class="api-list">
        <li><span class="api-method">GET</span><a href="/api/v1/manifest">/api/v1/manifest</a> &mdash; 文件清单（含校验和）</li>
        <li><span class="api-method">GET</span>/api/v1/download/{filename} &mdash; 文件下载（支持断点续传）</li>
        <li><span class="api-method">GET</span><a href="/api/v1/status">/api/v1/status</a> &mdash; 服务状态</li>
        <li><span class="api-method">GET</span><a href="/api/v1/sync/status">/api/v1/sync/status</a> &mdash; 同步状态</li>
        <li><span class="api-method">POST</span>/api/v1/sync/trigger &mdash; 触发同步</li>
        <li><span class="api-method">GET</span><a href="/">/</a> &mdash; 本页面</li>
    </ul>
</body>
<script>
(function() {
    var syncSection = document.getElementById('syncSection');
    var syncIndicator = document.getElementById('syncIndicator');
    var syncStatusText = document.getElementById('syncStatusText');
    var syncProgress = document.getElementById('syncProgress');
    var syncTime = document.getElementById('syncTime');
    var syncBtn = document.getElementById('syncBtn');

    function formatTime(iso) {
        if (!iso) return '-';
        try {
            var d = new Date(iso);
            return d.toLocaleString('zh-CN');
        } catch(e) { return iso; }
    }

    function updateSyncStatus() {
        fetch('/api/v1/sync/status')
            .then(function(r) { return r.json(); })
            .then(function(data) {
                var s = data.data;
                if (!s || (!s.running && !s.last_sync && !s.progress)) {
                    // Sync not enabled
                    syncSection.style.display = 'none';
                    return;
                }
                syncSection.style.display = 'block';

                if (s.running) {
                    syncIndicator.className = 'sync-indicator running';
                    syncStatusText.textContent = '同步中...';
                    syncBtn.disabled = true;
                } else {
                    syncIndicator.className = 'sync-indicator idle';
                    syncStatusText.textContent = '同步空闲';
                    syncBtn.disabled = false;
                }

                if (s.progress) {
                    if (s.synced_files > 0 || s.total_files > 0) {
                        syncProgress.textContent = s.progress + ' (' + s.synced_files + '/' + s.total_files + ')';
                    } else {
                        syncProgress.textContent = s.progress;
                    }
                } else {
                    syncProgress.textContent = '';
                }

                var timeInfo = [];
                if (s.last_sync) {
                    timeInfo.push('上次同步: ' + formatTime(s.last_sync));
                }
                if (s.next_sync && !s.running) {
                    timeInfo.push('下次: ' + formatTime(s.next_sync));
                }
                if (s.last_error) {
                    timeInfo.push('错误: ' + s.last_error);
                }
                syncTime.textContent = timeInfo.join('  |  ');
            })
            .catch(function() {
                syncSection.style.display = 'none';
            });
    }

    window.triggerSync = function() {
        syncBtn.disabled = true;
        fetch('/api/v1/sync/trigger', { method: 'POST' })
            .then(function() {
                setTimeout(updateSyncStatus, 500);
            });
    };

    updateSyncStatus();
    setInterval(updateSyncStatus, 3000);
})();
</script>
</html>`

// pageData holds the data for rendering the HTML page template.
type pageData struct {
	Version    string
	ServerName string
	FileCount  int
	TotalSize  string
	BinFiles   []binFileView
	BaseURL    string
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

	startTime time.Time
	httpSrv   *http.Server
	mu        sync.RWMutex
	files     []PackageFile
	totalSize int64

	syncMgr *syncManager
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
	Version int                    `json:"version"`
	Files   map[string]cacheEntry  `json:"files"`
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

	// PackagesDir is the directory containing browser packages.
	// If empty, the executable directory + "packages" is used.
	PackagesDir string

	// BinDir is the directory containing client binary files.
	// If empty, the executable directory + "bin" is used.
	BinDir string

	// SyncSource is the source for auto-syncing packages from online sources.
	// If nil, auto-sync is disabled.
	SyncSource SyncSource

	// SyncConfig configures the auto-sync behavior.
	// If zero-value defaults are used, sync is enabled with 24h interval
	// (only if SyncSource is set).
	SyncConfig SyncConfig
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

	baseDir := exeDir // baseDir kept for cache path

	srv := &Server{
		addr:        opts.Addr,
		version:     opts.Version,
		baseDir:     baseDir,
		packagesDir: packagesDir,
		binDir:      binDir,
		cachePath:   filepath.Join(baseDir, ".serve-cache.json"),
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
	// Ensure directories exist
	os.MkdirAll(s.packagesDir, 0o755)
	os.MkdirAll(s.binDir, 0o755)

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
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
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
		Handler:      mux,
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
	fmt.Printf("  基础目录:     %s\n", s.baseDir)
	fmt.Printf("  软件包目录:   %s\n", s.packagesDir)
	fmt.Printf("  客户端目录:   %s\n", s.binDir)
	fmt.Printf("  监听地址:     %s\n", s.addr)
	fmt.Printf("  软件包数量:   %d 个 (%s)\n", fileCount, formatSize(totalSize))
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

// scanPackages scans the packages directory recursively and builds the file list.
// Uses a worker pool for parallel checksum computation.
func (s *Server) scanPackages(cache map[string]cacheEntry) error {
	fmt.Printf("  正在扫描软件包目录: %s\n", s.packagesDir)

	// Create a scanner for filename parsing
	scanner, err := repo.NewScanner(s.packagesDir, browser.DefaultRegistry)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}

	// Phase 1: walk directory recursively to collect all files
	type fileEntry struct {
		relPath string
		absPath string
		info    os.FileInfo
	}
	var allFiles []fileEntry
	seenFiles := make(map[string]bool)

	err = filepath.WalkDir(s.packagesDir, func(absPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
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
			fmt.Println("  软件包目录不存在，跳过扫描")
			return nil
		}
		return fmt.Errorf("scanning packages directory: %w", err)
	}

	if len(allFiles) == 0 {
		fmt.Println("  未找到任何文件")
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
		numWorkers := runtime.NumCPU()
		if numWorkers < 1 {
			numWorkers = 1
		}
		if len(misses) < numWorkers {
			numWorkers = len(misses)
		}

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
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					fmt.Printf("  计算校验和: %s\n", j.relPath)
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

		// Collect results
		for r := range results {
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "  警告: 计算 %s 校验和失败: %v\n", r.relPath, r.err)
				continue
			}
			// Find the matching miss entry to get file info
			for _, m := range misses {
				if m.idx == r.idx {
					pkg := s.parsePackageFile(scanner, r.relPath, m.info.Size(), r.checksum)
					files[r.idx] = pkg
					totalSize += m.info.Size()
					// Update cache
					cache[r.relPath] = cacheEntry{
						Mtime:    m.info.ModTime(),
						Checksum: r.checksum,
						Size:     m.info.Size(),
					}
					break
				}
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
	fmt.Printf("  扫描完成: %d 个文件 (缓存命中 %d, 新计算 %d, 线程数 %d)\n", len(files), cacheHits, computedSuccessfully, runtime.NumCPU())

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

	return os.WriteFile(s.cachePath, data, 0o644)
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

	s.mu.RLock()
	files := make([]PackageFile, len(s.files))
	copy(files, s.files)
	fileCount := len(s.files)
	s.mu.RUnlock()

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
	enc.Encode(resp)
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
	enc.Encode(resp)
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
		Version:    s.version,
		ServerName: serverName,
		FileCount:  fileCount,
		TotalSize:  formatSize(totalSize),
		BinFiles:   binFileViews,
		BaseURL:    baseURL,
	}

	// Parse and execute template
	tmpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		// Headers already written, just log the error
		fmt.Fprintf(os.Stderr, "Warning: template execution error: %v\n", err)
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

	// Platform detection
	platform := "unknown"
	switch {
	case strings.Contains(lower, ".exe") || strings.Contains(lower, "win"):
		platform = "windows"
	case strings.Contains(lower, "mac") || strings.Contains(lower, "darwin") || strings.Contains(lower, "macos"):
		platform = "macos"
	case strings.Contains(lower, "linux"):
		platform = "linux"
	}

	// Arch detection
	arch := ""
	switch {
	case strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64"):
		arch = "arm64"
	case strings.Contains(lower, "x64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "64"):
		arch = "x64"
	case strings.Contains(lower, "x86") || strings.Contains(lower, "386") || strings.Contains(lower, "32"):
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
			Running: false,
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
