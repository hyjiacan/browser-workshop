package system

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bws/bws/internal/browser"
)

func TestNewDetector(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{Name: "test"})

	d := NewDetector(reg)
	if d == nil {
		t.Fatal("NewDetector returned nil")
	}
	if d.browsers == nil {
		t.Error("browsers registry is nil")
	}
	if d.cacheTTL != 1*time.Hour {
		t.Errorf("default cacheTTL = %v, want 1h", d.cacheTTL)
	}
}

func TestNewDetector_WithOptions(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{Name: "test"})

	d := NewDetector(reg, WithCacheTTL(30*time.Minute))
	if d.cacheTTL != 30*time.Minute {
		t.Errorf("cacheTTL = %v, want 30m", d.cacheTTL)
	}
}

func TestDetect_NotFound(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent-browser",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg)
	_, found := d.Detect("nonexistent-browser")
	if found {
		t.Error("Detect() should return false for non-existent browser")
	}
}

func TestDetectAll(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent-browser",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg)
	results := d.DetectAll()
	if len(results) != 0 {
		t.Errorf("DetectAll() = %d results, want 0", len(results))
	}
}

func TestDetectAll_Cache(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent-browser",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg, WithCacheTTL(1*time.Hour))

	// First call populates cache
	d.DetectAll()

	if d.cache == nil {
		t.Error("cache should be populated after DetectAll")
	}
	if d.cachedAt.IsZero() {
		t.Error("cachedAt should be set")
	}

	// Second call should use cache
	results := d.DetectAll()
	if len(results) != 0 {
		t.Errorf("cached results = %d, want 0", len(results))
	}
}

func TestInvalidateCache(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent-browser",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg)
	d.DetectAll()

	if d.cache == nil {
		t.Fatal("cache should not be nil")
	}

	d.InvalidateCache()
	if d.cache != nil {
		t.Error("cache should be nil after InvalidateCache")
	}
}

func TestRefresh(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent-browser",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg)
	results := d.Refresh()
	if len(results) != 0 {
		t.Errorf("Refresh() = %d results, want 0", len(results))
	}
}

func TestDetectAllForBrowser(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "nonexistent",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"nonexistent"}},
		},
	})

	d := NewDetector(reg)
	results := d.DetectAllForBrowser("nonexistent")
	if len(results) != 0 {
		t.Errorf("DetectAllForBrowser() = %d, want 0", len(results))
	}
}

func TestBrowserInfo(t *testing.T) {
	info := BrowserInfo{
		Browser:      "chrome",
		DisplayName:  "Google Chrome",
		Version:      "120.0.6099.109",
		InstallPath:  "/path/to/chrome",
		Executable:   "/path/to/chrome/chrome",
		Channel:      "stable",
		IsSystem:     true,
		Architecture: "amd64",
	}

	if info.Browser != "chrome" {
		t.Errorf("Browser = %q", info.Browser)
	}
	if !info.IsSystem {
		t.Error("IsSystem should be true")
	}
	if info.Channel != "stable" {
		t.Errorf("Channel = %q", info.Channel)
	}
}

func TestIsVersionDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"120.0.6099.109", true},
		{"121.0", true},
		{"120", false},
		{"v120.0", false},
		{"12.34.56.78", true},
		{"1.2.3", true},
		{"", false},
		{"random", false},
		{"12a.0", false},
		{".", false},
		{"12.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVersionDir(tt.name)
			if got != tt.want {
				t.Errorf("isVersionDir(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDetectChannel(t *testing.T) {
	tests := []struct {
		path    string
		browser string
		want    string
	}{
		{"/path/Google/Chrome/Application/chrome.exe", "chrome", "stable"},
		{"/path/Google/Chrome Beta/Application/chrome.exe", "chrome", "beta"},
		{"/path/Google/Chrome Dev/Application/chrome.exe", "chrome", "dev"},
		{"/path/Google/Chrome SxS/Application/chrome.exe", "chrome", "canary"},
		{"/path/Firefox Developer Edition/firefox", "firefox", "beta"},
		{"/path/Firefox Nightly/firefox", "firefox", "nightly"},
		{"/usr/bin/firefox-esr", "firefox", "esr"},
		{"/usr/bin/google-chrome-unstable", "chrome", "dev"},
		{"/usr/bin/google-chrome-stable", "chrome", "stable"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectChannel(tt.path, tt.browser)
			if got != tt.want {
				t.Errorf("detectChannel(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectVersionFromPath(t *testing.T) {
	dir := t.TempDir()
	chromeDir := filepath.Join(dir, "chrome")
	os.MkdirAll(chromeDir, 0o755)

	verDir := filepath.Join(chromeDir, "120.0.6099.109")
	os.MkdirAll(verDir, 0o755)

	exeName := "fake-chrome"
	if runtime.GOOS == "windows" {
		exeName = "fake-chrome.exe"
	}
	exePath := filepath.Join(chromeDir, exeName)
	os.WriteFile(exePath, []byte("fake"), 0o755)

	version := detectVersionFromPath(exePath)
	if version != "120.0.6099.109" {
		t.Errorf("detectVersionFromPath() = %q, want '120.0.6099.109'", version)
	}
}

func TestDetectVersionFromPath_NoVersionDir(t *testing.T) {
	dir := t.TempDir()
	exeName := "fake"
	if runtime.GOOS == "windows" {
		exeName = "fake.exe"
	}
	exePath := filepath.Join(dir, exeName)
	os.WriteFile(exePath, []byte("fake"), 0o755)

	version := detectVersionFromPath(exePath)
	if version != "" {
		t.Errorf("detectVersionFromPath() = %q, want empty string", version)
	}
}

func TestGetInstallPaths(t *testing.T) {
	tests := []struct {
		browser string
	}{
		{"chrome"},
		{"firefox"},
		{"chromium"},
		{"edge"},
	}

	for _, tt := range tests {
		t.Run(tt.browser, func(t *testing.T) {
			paths := getInstallPaths(tt.browser, runtime.GOOS)
			for _, p := range paths {
				if p == "" {
					t.Errorf("empty path in list")
				}
			}
		})
	}
}

func TestGetWindowsPaths(t *testing.T) {
	paths := getWindowsPaths("chrome")
	t.Logf("Windows Chrome paths: %d entries", len(paths))
}

func TestGetMacPaths(t *testing.T) {
	paths := getMacPaths("chrome")
	t.Logf("macOS Chrome paths: %d entries", len(paths))
}

func TestGetLinuxPaths(t *testing.T) {
	paths := getLinuxPaths("chrome")
	t.Logf("Linux Chrome paths: %d entries", len(paths))
}
