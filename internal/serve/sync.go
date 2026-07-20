package serve

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
}

// SyncSource provides version listing and download capability for sync.
type SyncSource interface {
	// ListVersions returns all available versions for the given browser/channel/platform/arch.
	ListVersions(browser string, channel string, platform string, arch string) ([]SyncVersionInfo, error)

	// Download downloads a file from url to destDir. Returns the final file path.
	Download(url string, destDir string, onProgress func(downloaded, total int64)) (string, error)
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

	// Platforms is the list of platforms to sync (e.g. ["windows", "macos", "linux"]).
	// Default: current platform.
	Platforms []string

	// Arches is the list of architectures to sync (e.g. ["x64", "x86"]).
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
	close(sm.stopCh)
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

	defer func() {
		sm.mu.Lock()
		sm.running = false
		sm.status.Running = false
		sm.status.LastSync = time.Now()
		sm.mu.Unlock()
	}()

	if sm.source == nil {
		sm.setError(fmt.Errorf("no sync source configured"))
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
		arches = []string{"x64"}
	}

	channels := sm.config.Channels
	if len(channels) == 0 {
		channels = []string{"stable"}
	}

	totalFiles := 0
	syncedFiles := 0
	sm.setProgressCount(totalFiles, syncedFiles)

	for _, browser := range browsers {
		for _, ch := range channels {
			for _, platform := range platforms {
				for _, arch := range arches {
					key := fmt.Sprintf("%s/%s/%s/%s", browser, ch, platform, arch)
					sm.setProgress("正在获取 " + key + " 的版本列表...")

					versions, err := sm.source.ListVersions(browser, ch, platform, arch)
					if err != nil {
						sm.setError(fmt.Errorf("listing %s: %w", key, err))
						continue
					}

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
							syncedFiles++
							sm.setProgressCount(totalFiles, syncedFiles)
							continue
						}

						sm.setProgress(fmt.Sprintf("正在下载 %s %s (%s/%s)...",
							browser, v.Version, platform, arch))

						// Download to temp file first
						_, err := sm.source.Download(v.DownloadURL, sm.server.packagesDir,
							func(downloaded, total int64) {
								// Progress updates could be more granular, but we keep it simple
							})
						if err != nil {
							sm.setError(fmt.Errorf("downloading %s@%s: %w", browser, v.Version, err))
							continue
						}

						syncedFiles++
						sm.setProgressCount(totalFiles, syncedFiles)
					}
				}
			}
		}
	}

	// Rescan packages after sync
	sm.setProgress("正在刷新文件清单...")
	cache, _ := sm.server.loadCache()
	if err := sm.server.scanPackages(cache); err != nil {
		sm.setError(fmt.Errorf("rescanning packages: %w", err))
		return
	}
	sm.server.saveCache(cache)

	sm.setProgress("同步完成")
	sm.mu.Lock()
	sm.status.LastError = ""
	sm.mu.Unlock()
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

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
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

	out.Close()

	// Rename to final
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", fmt.Errorf("renaming: %w", err)
	}

	return destPath, nil
}
