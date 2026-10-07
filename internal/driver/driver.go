// Package driver manages automation drivers — currently chromedriver — that
// must match the exact browser version being launched.
//
// The driver version is dynamic: it depends on the Chrome build the user
// starts. For Chrome >= 115 the mapping comes from the Chrome for Testing
// "known-good-versions-with-downloads" manifest; older builds fall back to the
// legacy chromedriver storage bucket.
package driver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

// NameChromedriver is the canonical name of the Chrome WebDriver.
const NameChromedriver = "chromedriver"

// recordFileName is the metadata file written inside each driver version dir.
const recordFileName = ".bws-driver.json"

// Info describes a driver build that can be downloaded.
type Info struct {
	// Name is the driver name (e.g. "chromedriver").
	Name string `json:"name"`

	// Version is the exact driver version (e.g. "120.0.6099.109").
	Version string `json:"version"`

	// MajorVersion is the directory key used for installation (e.g. "120").
	MajorVersion string `json:"major_version"`

	// Platform is the bws platform name (windows/darwin/linux).
	Platform string `json:"platform"`

	// Arch is the bws architecture name (amd64/386/arm64).
	Arch string `json:"arch"`

	// DownloadURL is the archive download URL.
	DownloadURL string `json:"download_url"`

	// Filename is the archive file name derived from DownloadURL.
	Filename string `json:"filename"`

	// Size is the expected archive size in bytes (0 if unknown).
	Size int64 `json:"size,omitempty"`

	// Source identifies where the build was resolved from
	// ("chrome-for-testing", "chromedriver-storage" or "serve").
	Source string `json:"source"`
}

// Record describes a driver installed on disk.
type Record struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	MajorVersion string    `json:"major_version"`
	Platform     string    `json:"platform"`
	Arch         string    `json:"arch"`
	Dir          string    `json:"dir"`
	Executable   string    `json:"executable"` // relative to Dir
	Size         int64     `json:"size"`
	Source       string    `json:"source"`
	InstalledAt  time.Time `json:"installed_at"`
}

// ExecutablePath returns the absolute path to the driver binary.
func (r *Record) ExecutablePath() string {
	return filepath.Join(r.Dir, r.Executable)
}

// Options configures a driver Manager.
type Options struct {
	// ProxyURL is the proxy used for manifest queries and downloads.
	ProxyURL string

	// ServeURL, when non-empty, makes the manager resolve and download
	// drivers through a bws serve instance (which may itself proxy the
	// upstream Chrome for Testing endpoints). Direct upstream access is
	// kept as a fallback.
	ServeURL string
}

// Manager resolves, installs and runs automation drivers.
type Manager struct {
	paths    *paths.Paths
	client   *http.Client
	sources  []Source
	proxyURL string
}

// NewManager creates a driver manager.
func NewManager(p *paths.Paths, opts Options) *Manager {
	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: newTransport(opts.ProxyURL),
	}

	m := &Manager{
		paths:    p,
		client:   client,
		proxyURL: opts.ProxyURL,
	}

	// A configured serve instance is preferred: it can host drivers and
	// proxy upstream downloads for offline/intranet deployments.
	if strings.TrimSpace(opts.ServeURL) != "" {
		m.sources = append(m.sources, NewServeSource(opts.ServeURL, client))
	}
	m.sources = append(m.sources, NewChromeForTestingSource(p, client))

	return m
}

// Sources returns the configured resolution sources (serve first, then
// direct upstream access).
func (m *Manager) Sources() []Source {
	return m.sources
}

// newTransport builds an http.Transport honouring the given proxy URL.
func newTransport(proxyURL string) *http.Transport {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			bmlog.Warn("[driver] 代理 URL 无效，已回退为直连: %s: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return transport
}

// CFTPlatform maps a bws platform/arch pair to a Chrome for Testing platform
// key. Returns "" when the combination is not published.
func CFTPlatform(platform, arch string) string {
	switch platform {
	case "windows":
		switch arch {
		case "amd64":
			return "win64"
		case "386":
			return "win32"
		}
	case "darwin":
		switch arch {
		case "amd64":
			return "mac-x64"
		case "arm64":
			return "mac-arm64"
		}
	case "linux":
		if arch == "amd64" {
			return "linux64"
		}
	}
	return ""
}

// LegacyPlatform maps a bws platform/arch pair to the platform token used by
// the legacy chromedriver storage filenames (chromedriver_<token>.zip).
// Returns "" when unsupported.
func LegacyPlatform(platform, arch string) string {
	switch platform {
	case "windows":
		// Only a 32-bit build was ever published; it runs on 64-bit Windows.
		if arch == "amd64" || arch == "386" {
			return "win32"
		}
	case "darwin":
		switch arch {
		case "amd64":
			return "mac64"
		case "arm64":
			return "mac64_m1"
		}
	case "linux":
		if arch == "amd64" {
			return "linux64"
		}
	}
	return ""
}

// DriverBinaryName returns the executable file name for a driver on the
// current platform.
func DriverBinaryName(name string) string {
	if paths.Platform() == "windows" {
		return name + ".exe"
	}
	return name
}

// majorKey returns the directory key for a driver version. The major version
// is used so that one directory serves every patch release of the same Chrome
// major; the exact version is tracked in the metadata file.
func majorKey(driverVersion string) string {
	if m := version.Major(driverVersion); m > 0 {
		return strconv.Itoa(m)
	}
	// Fall back to the raw string when the version is not numeric.
	return strings.TrimSpace(driverVersion)
}

// --- metadata persistence ---

// RecordPath returns the metadata file path for an installed driver.
func (m *Manager) RecordPath(name, major string) string {
	return filepath.Join(m.paths.DriverDir(name, major), recordFileName)
}

// readRecord loads the install record for a driver major version.
func (m *Manager) readRecord(name, major string) (*Record, error) {
	data, err := os.ReadFile(m.RecordPath(name, major))
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	if rec.Dir == "" {
		rec.Dir = m.paths.DriverDir(name, major)
	}
	return &rec, nil
}

// writeRecord persists the install record.
func (m *Manager) writeRecord(rec *Record) error {
	dir := m.paths.DriverDir(rec.Name, rec.MajorVersion)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, recordFileName), data, 0o644)
}

// resolveSources queries every configured source in order and returns the
// first successful resolution.
func (m *Manager) resolveSources(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	var lastErr error
	for _, src := range m.sources {
		info, err := src.Resolve(ctx, chromeVersion, platform, arch)
		if err == nil {
			bmlog.Debug("[driver] 通过源 %s 解析到 %s (Chrome %s)", src.Name(), info.Version, chromeVersion)
			return info, nil
		}
		bmlog.Debug("[driver] 源 %s 解析失败: %v", src.Name(), err)
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的驱动数据源")
	}
	return nil, lastErr
}