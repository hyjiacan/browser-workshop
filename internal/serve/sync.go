package serve

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/util"
)

// maxDownloadSize is the maximum allowed download size (2GB).
const maxDownloadSize = 2 << 30

// SyncVersionInfo describes a version available from an online source.
type SyncVersionInfo struct {
	Browser     string
	Version     string
	Channel     string
	Platform    string
	Arch        string
	DownloadURL string
	Size        int64
	Filename    string // optional: preferred filename
	Checksum    string // expected checksum hash (empty if unknown); prefixed with algo e.g. "sha256:hash" or "sha1:hash"
}

// SyncSource provides version listing and download capability for sync.
type SyncSource interface {
	// ListVersions returns all available versions for the given browser/channel/platform/arch.
	// The ctx is used to cancel long-running queries (e.g. HTTP requests to online sources).
	ListVersions(ctx context.Context, browser string, channel string, platform string, arch string) ([]SyncVersionInfo, error)

	// Download downloads a file from url to destDir. Returns the final file path.
	Download(url string, destDir string, onProgress func(downloaded, total int64)) (string, error)

	// GetChecksum returns the checksum hash for a specific version/platform/arch.
	// Returns a prefixed string like "sha256:hash" or "sha1:hash".
	// Returns empty string if the hash is unknown (e.g. old versions without checksum files).
	// Implementations should cache per-version checksum data to avoid repeated fetches.
	GetChecksum(ctx context.Context, browser, version, platform, arch string) (string, error)
}

// SyncConfig configures the auto-sync behavior.
type SyncConfig struct {
	// Enabled controls whether auto-sync is active.
	Enabled bool

	// Interval is how often to run sync. Default: 24h.
	Interval time.Duration

	// Browsers is the list of browsers to sync (e.g. ["chrome", "firefox"]).
	// If empty, syncs all registered browsers.
	Browsers []string

	// Channels is the list of channels to sync (e.g. ["stable", "beta"]).
	// Default: ["stable"].
	Channels []string

	// Platforms is the list of platforms to sync (e.g. ["windows", "darwin", "linux"]).
	// Default: current platform.
	Platforms []string

	// Arches is the list of architectures to sync (e.g. ["amd64", "386", "arm64"]).
	// Default: current arch.
	Arches []string

	// MaxVersionsPerBrowser is the max number of latest versions to keep per browser/channel/platform/arch.
	// 0 = unlimited (keep all).
	MaxVersionsPerBrowser int
}

// defaultSyncConfig returns the default sync configuration.
func defaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:               true,
		Interval:              24 * time.Hour,
		Channels:              []string{"stable"},
		MaxVersionsPerBrowser: 0, // keep all
	}
}

// SyncStatus describes the current state of the sync system.
type SyncStatus struct {
	Running     bool      `json:"running"`
	LastSync    time.Time `json:"last_sync"`
	LastError   string    `json:"last_error,omitempty"`
	NextSync    time.Time `json:"next_sync"`
	Progress    string    `json:"progress,omitempty"`
	TotalFiles  int       `json:"total_files"`
	SyncedFiles int       `json:"synced_files"`
}

// syncManager handles scheduled and manual sync operations.
type syncManager struct {
	server   *Server
	source   SyncSource
	config   SyncConfig
	status   SyncStatus
	mu       sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
	trigger  chan struct{}
	running  bool
}

// newSyncManager creates a new sync manager for the server.
func newSyncManager(srv *Server, source SyncSource, config SyncConfig) *syncManager {
	if config.Interval == 0 {
		config.Interval = 24 * time.Hour
	}
	if len(config.Channels) == 0 {
		config.Channels = []string{"stable"}
	}
	return &syncManager{
		server:  srv,
		source:  source,
		config:  config,
		stopCh:  make(chan struct{}),
		trigger: make(chan struct{}, 1), // buffered so trigger doesn't block
	}
}

// Start starts the sync scheduler.
func (sm *syncManager) Start() {
	if sm.source == nil || !sm.config.Enabled {
		return
	}

	go sm.run()
}

// Stop stops the sync scheduler.
func (sm *syncManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.stopCh)
	})
}

// Trigger requests an immediate sync run. Returns immediately.
// If a sync is already running, this is a no-op.
func (sm *syncManager) Trigger() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.running {
		return
	}
	select {
	case sm.trigger <- struct{}{}:
	default:
		// already a trigger pending
	}
}

// Status returns the current sync status.
func (sm *syncManager) Status() SyncStatus {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.status
}

// run is the main sync loop.
func (sm *syncManager) run() {
	// Run initial sync shortly after startup
	initialDelay := 5 * time.Second
	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	sm.setNextSync(time.Now().Add(initialDelay))

	for {
		select {
		case <-sm.stopCh:
			return
		case <-sm.trigger:
			sm.doSync()
			// Reset timer for next scheduled sync
			timer.Stop()
			timer = time.NewTimer(sm.config.Interval)
			sm.setNextSync(time.Now().Add(sm.config.Interval))
		case <-timer.C:
			sm.doSync()
			timer.Reset(sm.config.Interval)
			sm.setNextSync(time.Now().Add(sm.config.Interval))
		}
	}
}

func (sm *syncManager) setNextSync(t time.Time) {
	sm.mu.Lock()
	sm.status.NextSync = t
	sm.mu.Unlock()
}

// doSync performs the actual synchronization.
func (sm *syncManager) doSync() {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = true
	sm.status.Running = true
	sm.status.Progress = "正在启动同步..."
	sm.mu.Unlock()

	syncStart := time.Now()
	sm.server.logger.Info("[sync] 开始同步任务")

	defer func() {
		sm.mu.Lock()
		sm.running = false
		sm.status.Running = false
		sm.status.LastSync = time.Now()
		sm.mu.Unlock()
		sm.server.logger.Info("[sync] 同步任务结束 (总耗时 %v)", time.Since(syncStart).Round(time.Millisecond))
	}()

	if sm.source == nil {
		sm.setError(fmt.Errorf("no sync source configured"))
		sm.server.logger.Warn("[sync] 同步源未配置")
		return
	}

	// Determine what to sync
	browsers := sm.config.Browsers
	if len(browsers) == 0 {
		browsers = []string{"chrome", "firefox", "chromium"}
	}

	platforms := sm.config.Platforms
	if len(platforms) == 0 {
		platforms = []string{"windows"}
	}

	arches := sm.config.Arches
	if len(arches) == 0 {
		arches = []string{"amd64"}
	}

	channels := sm.config.Channels
	if len(channels) == 0 {
		channels = []string{"stable"}
	}

	sm.server.logger.Debug("[sync] 同步配置: browsers=%v channels=%v platforms=%v arches=%v",
		browsers, channels, platforms, arches)

	totalFiles := 0
	syncedFiles := 0
	skippedExisting := 0
	failedDownloads := 0
	sm.setProgressCount(totalFiles, syncedFiles)

	for _, browser := range browsers {
		for _, ch := range channels {
			for _, platform := range platforms {
				for _, arch := range arches {
					key := fmt.Sprintf("%s/%s/%s/%s", browser, ch, platform, arch)
					sm.setProgress("正在获取 " + key + " 的版本列表...")
					sm.server.logger.Debug("[sync] 正在获取 %s 的版本列表", key)

					// 为每次查询设置超时，避免在线源无响应时同步任务卡住
					listCtx, listCancel := context.WithTimeout(context.Background(), 2*time.Minute)
					versions, err := sm.source.ListVersions(listCtx, browser, ch, platform, arch)
					listCancel()
					if err != nil {
						sm.setError(fmt.Errorf("listing %s: %w", key, err))
						sm.server.logger.Warn("[sync] 获取 %s 版本列表失败: %v", key, err)
						continue
					}

					sm.server.logger.Debug("[sync] 获取 %s 版本列表成功: %d 个版本", key, len(versions))
					totalFiles += len(versions)
					sm.setProgressCount(totalFiles, syncedFiles)

					// Download each version
					for _, v := range versions {
						// Check if already exists
						filename := v.Filename
						if filename == "" {
							// Generate filename from URL
							filename = filepath.Base(v.DownloadURL)
						}
						destPath := filepath.Join(sm.server.packagesDir, filename)

						if _, err := os.Stat(destPath); err == nil {
							skippedExisting++
							sm.server.logger.Debug("[sync] 文件已存在，跳过: %s", filename)
							syncedFiles++
							sm.setProgressCount(totalFiles, syncedFiles)
							continue
						}

						sizeStr := "未知"
						if v.Size > 0 {
							sizeStr = util.FormatSize(v.Size)
						}
						sm.setProgress(fmt.Sprintf("正在下载 %s %s (%s/%s)...",
							browser, v.Version, platform, arch))
						sm.server.logger.Debug("[sync] 开始下载: %s %s (%s/%s) 大小=%s url=%s",
							browser, v.Version, platform, arch, sizeStr, v.DownloadURL)

						dlStart := time.Now()
						// Download to temp file first
						dlPath, err := sm.source.Download(v.DownloadURL, sm.server.packagesDir,
							func(downloaded, total int64) {
								// Progress updates could be more granular, but we keep it simple
							})
						if err != nil {
							failedDownloads++
							sm.setError(fmt.Errorf("downloading %s@%s: %w", browser, v.Version, err))
							sm.server.logger.Warn("[sync] 下载失败: %s@%s (%s/%s): %v (耗时 %v)",
								browser, v.Version, platform, arch, err, time.Since(dlStart).Round(time.Millisecond))
							continue
						}

						// Verify download integrity: check file size if the source provided one.
						if v.Size > 0 {
							fi, statErr := os.Stat(dlPath)
							if statErr != nil {
								failedDownloads++
								sm.setError(fmt.Errorf("stat downloaded %s@%s: %w", browser, v.Version, statErr))
								sm.server.logger.Warn("[sync] 下载后无法获取文件信息: %s@%s: %v", browser, v.Version, statErr)
								_ = os.Remove(dlPath)
								continue
							}
							if fi.Size() != v.Size {
								failedDownloads++
								sm.setError(fmt.Errorf("downloaded %s@%s size mismatch: got %d, expected %d", browser, v.Version, fi.Size(), v.Size))
								sm.server.logger.Warn("[sync] 下载文件大小不匹配: %s@%s: 实际=%d 预期=%d", browser, v.Version, fi.Size(), v.Size)
								_ = os.Remove(dlPath)
								continue
							}
						}

						// 校验和验证（支持 SHA256 和 SHA1 两种算法）。
					// 校验和来源优先级：
					// 1. SyncVersionInfo.Checksum（来自版本列表，Chrome/Chromium 源提供）
					// 2. 按需 GetChecksum（Firefox 源在下载后按需获取，避免列表阶段逐版本请求）
					// 校验和格式为带算法前缀的字符串，如 "sha256:hash" 或 "sha1:hash"。
					expectedChecksum := v.Checksum
					if expectedChecksum == "" && sm.source != nil {
						chkCtx, chkCancel := context.WithTimeout(context.Background(), 60*time.Second)
						chk, chkErr := sm.source.GetChecksum(chkCtx, browser, v.Version, platform, arch)
						chkCancel()
						if chkErr != nil {
							sm.server.logger.Debug("[sync] 获取校验和失败: %s@%s: %v", browser, v.Version, chkErr)
						}
						expectedChecksum = chk
					}

					if expectedChecksum != "" {
						algo, expectedHash, ok := strings.Cut(expectedChecksum, ":")
						if !ok || expectedHash == "" {
							sm.server.logger.Debug("[sync] 校验和格式无效，跳过校验: %s@%s: %s", browser, v.Version, expectedChecksum)
						} else {
							var h hash.Hash
							switch algo {
							case "sha256":
								h = sha256.New()
							case "sha1":
								h = sha1.New()
							default:
								sm.server.logger.Warn("[sync] 不支持的校验算法: %s@%s: %s", browser, v.Version, algo)
							}
							if h != nil {
								f, err := os.Open(dlPath)
								if err != nil {
									failedDownloads++
									sm.setError(fmt.Errorf("opening downloaded %s@%s for %s: %w", browser, v.Version, algo, err))
									sm.server.logger.Warn("[sync] 打开已下载文件失败: %s@%s: %v", browser, v.Version, err)
									_ = os.Remove(dlPath)
									continue
								}
								if _, err := io.Copy(h, f); err != nil {
									f.Close()
									failedDownloads++
									sm.setError(fmt.Errorf("computing %s for %s@%s: %w", algo, browser, v.Version, err))
									sm.server.logger.Warn("[sync] 计算 %s 失败: %s@%s: %v", algo, browser, v.Version, err)
									_ = os.Remove(dlPath)
									continue
								}
								f.Close()
								actualHash := hex.EncodeToString(h.Sum(nil))
								if actualHash != expectedHash {
									failedDownloads++
									sm.setError(fmt.Errorf("%s mismatch for %s@%s: expected %s, got %s", algo, browser, v.Version, expectedHash, actualHash))
									sm.server.logger.Warn("[sync] %s 校验失败: %s@%s: 预期=%s 实际=%s", algo, browser, v.Version, expectedHash, actualHash)
									_ = os.Remove(dlPath)
									continue
								}
								sm.server.logger.Debug("[sync] %s 校验通过: %s@%s", algo, browser, v.Version)
							}
						}
					}

						sm.server.logger.Debug("[sync] 下载完成: %s@%s (%s/%s) (耗时 %v)",
							browser, v.Version, platform, arch, time.Since(dlStart).Round(time.Millisecond))

						syncedFiles++
						sm.setProgressCount(totalFiles, syncedFiles)
					}
				}
			}
		}
	}

	// Rescan packages after sync
	sm.setProgress("正在刷新文件清单...")
	sm.server.logger.Debug("[sync] 正在刷新文件清单...")
	cache, _ := sm.server.loadCache()
	if err := sm.server.scanPackages(cache); err != nil {
		sm.setError(fmt.Errorf("rescanning packages: %w", err))
		sm.server.logger.Warn("[sync] 同步后刷新文件清单失败: %v", err)
		return
	}
	sm.server.saveCache(cache)

	sm.setProgress("同步完成")
	sm.mu.Lock()
	sm.status.LastError = ""
	total := sm.status.TotalFiles
	synced := sm.status.SyncedFiles
	sm.mu.Unlock()
	sm.server.logger.Info("[sync] 同步完成: 共 %d 个文件，已同步 %d 个 (跳过已存在 %d, 失败 %d, 总耗时 %v)",
		total, synced, skippedExisting, failedDownloads, time.Since(syncStart).Round(time.Millisecond))
}

func (sm *syncManager) setProgress(msg string) {
	sm.mu.Lock()
	sm.status.Progress = msg
	sm.mu.Unlock()
}

func (sm *syncManager) setProgressCount(total, synced int) {
	sm.mu.Lock()
	sm.status.TotalFiles = total
	sm.status.SyncedFiles = synced
	sm.mu.Unlock()
}

func (sm *syncManager) setError(err error) {
	sm.mu.Lock()
	sm.status.LastError = err.Error()
	sm.mu.Unlock()
}

// --- Default download implementation ---

// DefaultDownload is a simple HTTP download used when SyncSource doesn't provide its own.
func DefaultDownload(url string, destDir string, onProgress func(downloaded, total int64)) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("creating dest dir: %w", err)
	}

	filename := filepath.Base(url)
	destPath := filepath.Join(destDir, filename)

	// Download to temp file
	tmpPath := destPath + ".tmp"

	// Clean up temp file on any error
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	// Use a client with timeout
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Enforce size limit
	if resp.ContentLength > maxDownloadSize {
		return "", fmt.Errorf("文件大小超过限制 (%d > %d)", resp.ContentLength, maxDownloadSize)
	}

	total := resp.ContentLength

	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return "", fmt.Errorf("writing: %w", werr)
			}
			downloaded += int64(n)
			if downloaded > maxDownloadSize {
				out.Close()
				return "", fmt.Errorf("下载大小超过限制 (%d > %d)", downloaded, maxDownloadSize)
			}
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			out.Close()
			return "", fmt.Errorf("reading: %w", err)
		}
	}

	if err := out.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	// Rename to final
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", fmt.Errorf("renaming: %w", err)
	}

	cleanup = false // rename succeeded, don't remove
	return destPath, nil
}
