// Package system provides detection and launching of system-installed browsers.
// System browsers are read-only — bm detects them and launches them directly
// without copying or managing their installation.
package system

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bws/bws/internal/browser"
)

// BrowserInfo represents a detected system-installed browser.
type BrowserInfo struct {
	Browser      string // canonical name: chrome, firefox, etc.
	DisplayName  string
	Version      string
	InstallPath  string // directory containing the executable
	Executable   string // full path to executable
	Channel      string // stable, beta, dev, canary, etc.
	IsSystem     bool   // always true for system browsers
	Architecture string
}

// Detector finds system-installed browsers with caching.
type Detector struct {
	browsers   *browser.Registry
	cache      []BrowserInfo
	cachedAt   time.Time
	cacheTTL   time.Duration
	mu         sync.RWMutex
}

// DetectorOption configures a detector.
type DetectorOption func(*Detector)

// WithCacheTTL sets the cache duration for detection results.
func WithCacheTTL(ttl time.Duration) DetectorOption {
	return func(d *Detector) {
		d.cacheTTL = ttl
	}
}

// NewDetector creates a new system browser detector.
func NewDetector(br *browser.Registry, opts ...DetectorOption) *Detector {
	d := &Detector{
		browsers: br,
		cacheTTL: 1 * time.Hour, // default: cache for 1 hour
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DetectAll scans the system for all known browsers.
// Results are cached for the duration of cacheTTL.
func (d *Detector) DetectAll() []BrowserInfo {
	// Check cache first
	d.mu.RLock()
	if d.cache != nil && time.Since(d.cachedAt) < d.cacheTTL {
		result := make([]BrowserInfo, len(d.cache))
		copy(result, d.cache)
		d.mu.RUnlock()
		return result
	}
	d.mu.RUnlock()

	// Perform actual detection
	results := make([]BrowserInfo, 0)
	for _, desc := range d.browsers.List() {
		infos := d.detectBrowser(desc)
		results = append(results, infos...)
	}

	// Update cache
	d.mu.Lock()
	d.cache = results
	d.cachedAt = time.Now()
	d.mu.Unlock()

	return results
}

// Refresh clears the cache and re-detects.
func (d *Detector) Refresh() []BrowserInfo {
	d.mu.Lock()
	d.cache = nil
	d.cachedAt = time.Time{}
	d.mu.Unlock()
	return d.DetectAll()
}

// InvalidateCache clears the detection cache.
func (d *Detector) InvalidateCache() {
	d.mu.Lock()
	d.cache = nil
	d.cachedAt = time.Time{}
	d.mu.Unlock()
}

// Detect finds a specific browser on the system (stable channel by default).
func (d *Detector) Detect(browserName string) (BrowserInfo, bool) {
	all := d.DetectAll()
	for _, info := range all {
		if info.Browser == browserName && info.Channel == "stable" {
			return info, true
		}
	}
	// Fallback: return any channel for this browser
	for _, info := range all {
		if info.Browser == browserName {
			return info, true
		}
	}
	return BrowserInfo{}, false
}

// DetectAllForBrowser returns all detected channels for a specific browser.
func (d *Detector) DetectAllForBrowser(browserName string) []BrowserInfo {
	all := d.DetectAll()
	var result []BrowserInfo
	for _, info := range all {
		if info.Browser == browserName {
			result = append(result, info)
		}
	}
	return result
}

// detectBrowser finds all installations of a browser by checking known paths.
// A browser may have multiple installations (stable, beta, dev, canary).
func (d *Detector) detectBrowser(desc *browser.BrowserDescriptor) []BrowserInfo {
	platform := runtime.GOOS
	arch := runtime.GOARCH

	candidates := getInstallPaths(desc.Name, platform)

	var results []BrowserInfo
	seen := make(map[string]bool)

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		// Deduplicate by executable path
		if seen[path] {
			continue
		}
		seen[path] = true

		version := readVersion(path, desc.Name)
		installDir := filepath.Dir(path)
		channel := detectChannel(path, desc.Name)

		results = append(results, BrowserInfo{
			Browser:      desc.Name,
			DisplayName:  desc.DisplayName,
			Version:      version,
			InstallPath:  installDir,
			Executable:   path,
			Channel:      channel,
			IsSystem:     true,
			Architecture: arch,
		})
	}

	return results
}

// getInstallPaths returns common installation paths for a browser on a platform.
func getInstallPaths(browserName string, platform string) []string {
	var paths []string

	switch platform {
	case "windows":
		paths = getWindowsPaths(browserName)
	case "darwin":
		paths = getMacPaths(browserName)
	case "linux":
		paths = getLinuxPaths(browserName)
	}

	return paths
}

func getWindowsPaths(browser string) []string {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	localAppData := os.Getenv("LOCALAPPDATA")

	var paths []string

	switch browser {
	case "chrome":
		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"))
			paths = append(paths, filepath.Join(programFiles, "Google", "Chrome Beta", "Application", "chrome.exe"))
			paths = append(paths, filepath.Join(programFiles, "Google", "Chrome Dev", "Application", "chrome.exe"))
			paths = append(paths, filepath.Join(programFiles, "Google", "Chrome SxS", "Application", "chrome.exe"))
		}
		if programFilesX86 != "" {
			paths = append(paths, filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"))
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"))
		}

	case "firefox":
		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "Mozilla Firefox", "firefox.exe"))
			paths = append(paths, filepath.Join(programFiles, "Firefox Developer Edition", "firefox.exe"))
			paths = append(paths, filepath.Join(programFiles, "Firefox Nightly", "firefox.exe"))
		}
		if programFilesX86 != "" {
			paths = append(paths, filepath.Join(programFilesX86, "Mozilla Firefox", "firefox.exe"))
		}

	case "chromium":
		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"))
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, "Chromium", "Application", "chrome.exe"))
		}

	case "edge":
		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"))
			paths = append(paths, filepath.Join(programFiles, "Microsoft", "Edge Beta", "Application", "msedge.exe"))
			paths = append(paths, filepath.Join(programFiles, "Microsoft", "Edge Dev", "Application", "msedge.exe"))
			paths = append(paths, filepath.Join(programFiles, "Microsoft", "Edge SxS", "Application", "msedge.exe"))
		}
	}

	return paths
}

func getMacPaths(browser string) []string {
	var paths []string
	applications := "/Applications"

	switch browser {
	case "chrome":
		paths = append(paths,
			filepath.Join(applications, "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
			filepath.Join(applications, "Google Chrome Beta.app", "Contents", "MacOS", "Google Chrome Beta"),
			filepath.Join(applications, "Google Chrome Dev.app", "Contents", "MacOS", "Google Chrome Dev"),
			filepath.Join(applications, "Google Chrome Canary.app", "Contents", "MacOS", "Google Chrome Canary"),
		)
	case "firefox":
		paths = append(paths,
			filepath.Join(applications, "Firefox.app", "Contents", "MacOS", "firefox"),
			filepath.Join(applications, "Firefox Developer Edition.app", "Contents", "MacOS", "firefox"),
			filepath.Join(applications, "Firefox Nightly.app", "Contents", "MacOS", "firefox"),
		)
	case "chromium":
		paths = append(paths,
			filepath.Join(applications, "Chromium.app", "Contents", "MacOS", "Chromium"),
		)
	case "edge":
		paths = append(paths,
			filepath.Join(applications, "Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge"),
			filepath.Join(applications, "Microsoft Edge Beta.app", "Contents", "MacOS", "Microsoft Edge Beta"),
		)
	}

	// Also check user's Applications folder
	home, err := os.UserHomeDir()
	if err == nil {
		userApps := filepath.Join(home, "Applications")
		switch browser {
		case "chrome":
			paths = append(paths, filepath.Join(userApps, "Google Chrome.app", "Contents", "MacOS", "Google Chrome"))
		case "firefox":
			paths = append(paths, filepath.Join(userApps, "Firefox.app", "Contents", "MacOS", "firefox"))
		}
	}

	return paths
}

func getLinuxPaths(browser string) []string {
	var paths []string

	switch browser {
	case "chrome":
		paths = append(paths,
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome-beta",
			"/usr/bin/google-chrome-unstable",
			"/opt/google/chrome/chrome",
			"/opt/google/chrome-beta/chrome",
		)
	case "chromium":
		paths = append(paths,
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium-stable",
			"/snap/bin/chromium",
			"/usr/lib/chromium-browser/chromium-browser",
		)
	case "firefox":
		paths = append(paths,
			"/usr/bin/firefox",
			"/usr/bin/firefox-esr",
			"/opt/firefox/firefox",
			"/snap/bin/firefox",
		)
	case "edge":
		paths = append(paths,
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/usr/bin/microsoft-edge-beta",
			"/usr/bin/microsoft-edge-dev",
			"/opt/microsoft/msedge/msedge",
		)
	}

	return paths
}

// readVersion attempts to read the version of a browser executable.
// On Windows, it reads the file version info.
// On other platforms, it tries to parse from the directory structure or
// runs the browser with --version flag.
func readVersion(execPath string, browserName string) string {
	// Try platform-specific version reading
	if v := readFileVersion(execPath); v != "" {
		return v
	}

	// Try to detect from directory structure
	if v := detectVersionFromPath(execPath); v != "" {
		return v
	}

	// Fallback: unknown
	return "unknown"
}

// detectVersionFromPath tries to extract version from the install path.
// On Windows, Chrome has version directories next to chrome.exe.
func detectVersionFromPath(execPath string) string {
	dir := filepath.Dir(execPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Look for version-numbered directories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check if it looks like a version number
		if isVersionDir(name) {
			return name
		}
	}

	return ""
}

// isVersionDir checks if a directory name looks like a version number.
func isVersionDir(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Must start with a digit
	if name[0] < '0' || name[0] > '9' {
		return false
	}
	// Must end with a digit
	if name[len(name)-1] < '0' || name[len(name)-1] > '9' {
		return false
	}
	// Must contain at least one dot
	if !strings.Contains(name, ".") {
		return false
	}
	// All characters should be digits or dots
	for _, c := range name {
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}

// detectChannel tries to determine the release channel from the install path.
func detectChannel(execPath string, browserName string) string {
	lowerPath := strings.ToLower(execPath)

	switch {
	case strings.Contains(lowerPath, "canary") || strings.Contains(lowerPath, "sxs"):
		return "canary"
	case strings.Contains(lowerPath, "developer edition"):
		// Firefox Developer Edition is based on the beta channel
		return "beta"
	case strings.Contains(lowerPath, "nightly"):
		return "nightly"
	case strings.Contains(lowerPath, "beta"):
		return "beta"
	case strings.Contains(lowerPath, "dev"):
		return "dev"
	case strings.Contains(lowerPath, "esr"):
		return "esr"
	case strings.Contains(lowerPath, "unstable"):
		return "dev"
	default:
		return "stable"
	}
}
