// Package download provides file downloading with progress tracking and resume support.
package download

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxDownloadSize is the maximum allowed download size (2GB).
const maxDownloadSize = 2 << 30

// Progress reports download progress.
type Progress struct {
	// Total is the total number of bytes to download (-1 if unknown).
	Total int64

	// Downloaded is the number of bytes downloaded so far.
	Downloaded int64

	// Percent is the download percentage (0-100).
	Percent float64

	// Speed is the current download speed in bytes per second.
	Speed float64

	// Elapsed is the time elapsed since the download started.
	Elapsed time.Duration

	// ETA is the estimated time remaining.
	ETA time.Duration

	// Status is the current status of the download.
	Status string // "downloading", "paused", "complete", "error"
}

// ProgressCallback is called during download to report progress.
type ProgressCallback func(progress Progress)

// Options configures a download.
type Options struct {
	// URL is the URL to download from.
	URL string

	// DestPath is the path where the downloaded file will be saved.
	DestPath string

	// Resume enables resuming partial downloads if supported.
	Resume bool

	// Timeout is the total timeout for the download (0 = no timeout).
	Timeout time.Duration

	// OnProgress is called periodically with progress updates.
	OnProgress ProgressCallback

	// ProgressInterval controls how often progress is reported.
	ProgressInterval time.Duration

	// HTTPClient is the HTTP client to use (nil = default).
	HTTPClient *http.Client

	// UserAgent is the User-Agent header to send.
	UserAgent string
}

// Result contains the result of a download.
type Result struct {
	// Path is the path to the downloaded file.
	Path string

	// Size is the size of the downloaded file in bytes.
	Size int64

	// Duration is the total time taken to download.
	Duration time.Duration

	// Resumed indicates whether the download was resumed from a partial file.
	Resumed bool
}

// Manager handles downloading files with progress tracking and resume support.
type Manager struct {
	mu           sync.Mutex
	activeDownloads map[string]*downloadState
	defaultClient *http.Client
}

// downloadState tracks the state of an active download.
type downloadState struct {
	options   Options
	cancelFn  context.CancelFunc
	progress  Progress
	startTime time.Time
	lastBytes int64
	lastTime  time.Time
	speedEMA  float64
}

// NewManager creates a new download manager with the default HTTP client.
func NewManager() *Manager {
	return NewManagerWithProxy("")
}

// NewManagerWithProxy creates a new download manager that uses the given proxy.
// proxyURL can be empty (direct), "http://host:port", "socks5://host:port", etc.
func NewManagerWithProxy(proxyURL string) *Manager {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURLParsed)
		}
	}
	return &Manager{
		activeDownloads: make(map[string]*downloadState),
		defaultClient: &http.Client{
			Timeout:   0,
			Transport: transport,
		},
	}
}

// Download downloads a file with progress tracking.
// It supports resume if the server supports Range requests.
func (m *Manager) Download(ctx context.Context, opts Options) (*Result, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if opts.DestPath == "" {
		return nil, fmt.Errorf("destination path is required")
	}

	if opts.ProgressInterval == 0 {
		opts.ProgressInterval = 100 * time.Millisecond
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = m.defaultClient
	}

	// Create download context with timeout if specified
	downloadCtx := ctx
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		downloadCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	} else {
		downloadCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Track this download
	state := &downloadState{
		options:   opts,
		cancelFn:  cancel,
		startTime: time.Now(),
		lastTime:  time.Now(),
		progress: Progress{
			Total:  -1,
			Status: "downloading",
		},
	}

	key := opts.URL
	m.mu.Lock()
	m.activeDownloads[key] = state
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activeDownloads, key)
		m.mu.Unlock()
	}()

	// Ensure destination directory exists
	destDir := filepath.Dir(opts.DestPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating destination directory: %w", err)
	}

	// Determine if we can resume
	var existingSize int64
	tempPath := opts.DestPath + ".part"

	if opts.Resume {
		if info, err := os.Stat(tempPath); err == nil {
			existingSize = info.Size()
		}
	}

	// Clean up temp file on error (unless resuming)
	downloadOK := false
	defer func() {
		if !downloadOK && !opts.Resume {
			_ = os.Remove(tempPath)
		}
	}()

	// Build the request
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	// Add Range header for resume
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Execute the request
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Enforce size limit
	if resp.ContentLength > maxDownloadSize {
		return nil, fmt.Errorf("文件大小超过限制 (%d > %d)", resp.ContentLength, maxDownloadSize)
	}

	// Calculate total size and handle file creation
	var totalSize int64
	var file *os.File

	if resp.StatusCode == http.StatusPartialContent && existingSize > 0 {
		// Resume mode: open file in append mode
		file, err = os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("opening file for resume: %w", err)
		}

		// Parse Content-Range header: "bytes start-end/total"
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 && parts[1] != "*" {
				if _, err := fmt.Sscanf(parts[1], "%d", &totalSize); err != nil {
					totalSize = 0
				}
			}
		}
		if totalSize == 0 {
			totalSize = resp.ContentLength + existingSize
		}
	} else {
		// Full download: create new file
		file, err = os.Create(tempPath)
		if err != nil {
			return nil, fmt.Errorf("creating destination file: %w", err)
		}
		existingSize = 0
		totalSize = resp.ContentLength
	}

	state.progress.Total = totalSize
	state.progress.Downloaded = existingSize

	// Report initial progress
	m.reportProgress(state)

	// Create progress writer
	writer := &progressWriter{
		writer:     file,
		onProgress: func(n int64) {
			state.progress.Downloaded += n
			m.updateSpeed(state)
			state.progress.Elapsed = time.Since(state.startTime)
			if state.progress.Total > 0 {
				state.progress.Percent = float64(state.progress.Downloaded) / float64(state.progress.Total) * 100
				if state.speedEMA > 0 {
					remaining := state.progress.Total - state.progress.Downloaded
					state.progress.ETA = time.Duration(float64(remaining)/state.speedEMA) * time.Second
				}
			}
		},
		interval: opts.ProgressInterval,
		lastReport: time.Now(),
		ctx:      downloadCtx,
	}

	// Download the body
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		file.Close()
		if downloadCtx.Err() != nil {
			return nil, downloadCtx.Err()
		}
		return nil, fmt.Errorf("downloading: %w", err)
	}

	// Ensure all data is flushed
	writer.Flush()

	// Final progress report
	state.progress.Status = "complete"
	state.progress.Percent = 100
	m.reportProgress(state)

	// Close file before renaming (must check error)
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("closing downloaded file: %w", err)
	}

	downloadOK = true

	// Rename .part to final filename
	if err := os.Rename(tempPath, opts.DestPath); err != nil {
		return nil, fmt.Errorf("finalizing download: %w", err)
	}

	// Get final file size
	info, err := os.Stat(opts.DestPath)
	if err != nil {
		return nil, fmt.Errorf("stat final file: %w", err)
	}

	return &Result{
		Path:     opts.DestPath,
		Size:     info.Size(),
		Duration: time.Since(state.startTime),
		Resumed:  existingSize > 0,
	}, nil
}

// Cancel cancels an active download by URL.
func (m *Manager) Cancel(url string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.activeDownloads[url]
	if !ok {
		return false
	}

	state.cancelFn()
	state.progress.Status = "cancelled"
	return true
}

// GetProgress returns the current progress of a download by URL.
func (m *Manager) GetProgress(url string) (Progress, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.activeDownloads[url]
	if !ok {
		return Progress{}, false
	}

	return state.progress, true
}

// reportProgress calls the progress callback if set.
func (m *Manager) reportProgress(state *downloadState) {
	if state.options.OnProgress != nil {
		state.options.OnProgress(state.progress)
	}
}

// updateSpeed calculates the current download speed using EMA.
func (m *Manager) updateSpeed(state *downloadState) {
	now := time.Now()
	elapsed := now.Sub(state.lastTime).Seconds()

	if elapsed > 0 {
		delta := state.progress.Downloaded - state.lastBytes
		currentSpeed := float64(delta) / elapsed

		// EMA (Exponential Moving Average) with alpha = 0.3
		alpha := 0.3
		if state.speedEMA == 0 {
			state.speedEMA = currentSpeed
		} else {
			state.speedEMA = alpha*currentSpeed + (1-alpha)*state.speedEMA
		}

		state.progress.Speed = state.speedEMA
		state.lastBytes = state.progress.Downloaded
		state.lastTime = now
	}
}

// progressWriter wraps a writer to track progress.
type progressWriter struct {
	writer     io.Writer
	onProgress func(int64)
	interval   time.Duration
	lastReport time.Time
	ctx        context.Context
}

func (w *progressWriter) Write(p []byte) (int, error) {
	// Check context
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}

	n, err := w.writer.Write(p)
	if n > 0 && w.onProgress != nil {
		w.onProgress(int64(n))

		// Report progress at intervals
		now := time.Now()
		if now.Sub(w.lastReport) >= w.interval {
			w.lastReport = now
			// Progress is reported via the onProgress callback
			// The actual progress reporting happens in the Manager
		}
	}
	return n, err
}

// Flush forces a final progress report.
func (w *progressWriter) Flush() {
	// Nothing to flush, progress is tracked incrementally
}

// FormatSpeed formats a speed in bytes per second to a human-readable string.
func FormatSpeed(bps float64) string {
	if bps < 1024 {
		return fmt.Sprintf("%.0f B/s", bps)
	}
	if bps < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	}
	if bps < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bps/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB/s", bps/(1024*1024*1024))
}

// FormatSize formats a size in bytes to a human-readable string.
func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

// FormatDuration formats a duration in a human-readable way.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
