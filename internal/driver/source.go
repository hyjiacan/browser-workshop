package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Source resolves a driver build matching a given browser version.
type Source interface {
	// Name returns the source identifier used in logs.
	Name() string

	// Resolve returns the driver build matching chromeVersion for the given
	// platform/arch. platform and arch use bws canonical names
	// (windows/darwin/linux, amd64/386/arm64).
	Resolve(ctx context.Context, chromeVersion, platform, arch string) (*Info, error)
}

const (
	// cftManifestURL is the Chrome for Testing manifest listing every
	// published build together with its download URLs.
	cftManifestURL = "https://googlechromelabs.github.io/chrome-for-testing/known-good-versions-with-downloads.json"

	// legacyBaseURL is the pre-115 chromedriver storage bucket.
	legacyBaseURL = "https://chromedriver.storage.googleapis.com"

	// cftMinMajor is the first Chrome major served by Chrome for Testing.
	cftMinMajor = 115

	// cftManifestTTL is how long the cached Chrome for Testing manifest is
	// considered fresh.
	cftManifestTTL = 24 * time.Hour

	// maxManifestSize caps the manifest response size (64MB).
	maxManifestSize = 64 << 20
)

// --- Chrome for Testing manifest model ---

type cftManifest struct {
	Timestamp string       `json:"timestamp"`
	Versions  []cftVersion `json:"versions"`
}

type cftVersion struct {
	Version   string                   `json:"version"`
	Revision  string                   `json:"revision"`
	Downloads map[string][]cftDownload `json:"downloads"`
}

type cftDownload struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// ChromeForTestingSource resolves chromedriver builds from the Chrome for
// Testing manifest, falling back to the legacy storage bucket for older
// Chrome majors.
type ChromeForTestingSource struct {
	paths  *paths.Paths
	client *http.Client

	// manifestURL and legacyBase are overridable for tests.
	manifestURL string
	legacyBase  string
}

// NewChromeForTestingSource creates the upstream resolution source.
func NewChromeForTestingSource(p *paths.Paths, client *http.Client) *ChromeForTestingSource {
	return &ChromeForTestingSource{
		paths:       p,
		client:      client,
		manifestURL: cftManifestURL,
		legacyBase:  legacyBaseURL,
	}
}

// Name returns the source identifier.
func (s *ChromeForTestingSource) Name() string { return "chrome-for-testing" }

// Resolve resolves the driver matching chromeVersion.
//
// Chrome >= 115 is served by the Chrome for Testing manifest; older builds
// use the legacy chromedriver storage bucket.
func (s *ChromeForTestingSource) Resolve(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	major := version.Major(chromeVersion)
	if major == 0 {
		return nil, fmt.Errorf("无法解析 Chrome 版本: %q", chromeVersion)
	}

	if major >= cftMinMajor {
		return s.resolveCFT(ctx, chromeVersion, platform, arch)
	}
	return s.resolveLegacy(ctx, chromeVersion, platform, arch)
}

// resolveCFT looks up the chromedriver build in the Chrome for Testing manifest.
func (s *ChromeForTestingSource) resolveCFT(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	platKey := CFTPlatform(platform, arch)
	if platKey == "" {
		return nil, fmt.Errorf("Chrome for Testing 未提供 %s/%s 平台的 chromedriver", platform, arch)
	}

	manifest, err := s.loadManifest(ctx)
	if err != nil {
		return nil, err
	}

	major := version.Major(chromeVersion)
	var exact, best *cftVersion
	for i := range manifest.Versions {
		v := &manifest.Versions[i]
		if v.Version == chromeVersion {
			exact = v
			break
		}
		if version.Major(v.Version) == major {
			if best == nil || version.Greater(v.Version, best.Version) {
				best = v
			}
		}
	}

	target := exact
	if target == nil {
		target = best
	}
	if target == nil {
		return nil, fmt.Errorf("Chrome for Testing 清单中未找到 Chrome %s 对应的 chromedriver", chromeVersion)
	}
	if exact == nil {
		bmlog.Warn("[driver] 未找到 Chrome %s 的精确匹配，使用同主版本最新的 chromedriver %s（可能不完全兼容）",
			chromeVersion, target.Version)
	}

	downloadURL := findDownload(target, "chromedriver", platKey)
	if downloadURL == "" {
		return nil, fmt.Errorf("Chrome for Testing 清单中未找到 %s 平台的 chromedriver 下载地址", platKey)
	}

	return &Info{
		Name:         NameChromedriver,
		Version:      target.Version,
		MajorVersion: strconv.Itoa(major),
		Platform:     platform,
		Arch:         arch,
		DownloadURL:  downloadURL,
		Filename:     filepath.Base(downloadURL),
		Source:       s.Name(),
	}, nil
}

// resolveLegacy resolves a driver through the pre-115 chromedriver storage
// bucket: LATEST_RELEASE_<major> yields the version, from which the download
// URL is deterministic.
func (s *ChromeForTestingSource) resolveLegacy(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	platKey := LegacyPlatform(platform, arch)
	if platKey == "" {
		return nil, fmt.Errorf("旧版 chromedriver 源未提供 %s/%s 平台的构建", platform, arch)
	}

	major := version.Major(chromeVersion)
	releaseURL := fmt.Sprintf("%s/LATEST_RELEASE_%d", s.legacyBase, major)
	body, err := s.fetchText(ctx, releaseURL)
	if err != nil {
		return nil, fmt.Errorf("查询 Chrome %d 的 chromedriver 版本失败: %w", major, err)
	}
	driverVersion := strings.TrimSpace(body)
	if driverVersion == "" {
		return nil, fmt.Errorf("旧版源未返回 Chrome %d 的 chromedriver 版本", major)
	}

	filename := fmt.Sprintf("chromedriver_%s.zip", platKey)
	downloadURL := fmt.Sprintf("%s/%s/%s", s.legacyBase, driverVersion, filename)

	return &Info{
		Name:         NameChromedriver,
		Version:      driverVersion,
		MajorVersion: strconv.Itoa(major),
		Platform:     platform,
		Arch:         arch,
		DownloadURL:  downloadURL,
		Filename:     filename,
		Source:       "chromedriver-storage",
	}, nil
}

// findDownload returns the download URL for a product/platform pair.
func findDownload(v *cftVersion, product, platform string) string {
	for _, d := range v.Downloads[product] {
		if d.Platform == platform {
			return d.URL
		}
	}
	return ""
}

// manifestCacheFile returns the on-disk cache path for the CFT manifest.
func (s *ChromeForTestingSource) manifestCacheFile() string {
	return filepath.Join(s.paths.ManifestCacheDir, "chromedriver-cft.json")
}

// loadManifest returns the Chrome for Testing manifest, using the on-disk
// cache when it is still fresh.
func (s *ChromeForTestingSource) loadManifest(ctx context.Context) (*cftManifest, error) {
	cachePath := s.manifestCacheFile()
	if fi, err := os.Stat(cachePath); err == nil && time.Since(fi.ModTime()) < cftManifestTTL {
		if m, err := readCFTManifest(cachePath); err == nil {
			bmlog.Debug("[driver] Chrome for Testing 清单命中磁盘缓存 (%d 个版本)", len(m.Versions))
			return m, nil
		}
		bmlog.Debug("[driver] Chrome for Testing 清单缓存损坏，将重新下载")
	}

	bmlog.Debug("[driver] 正在下载 Chrome for Testing 清单: %s", s.manifestURL)
	body, err := s.fetchBytes(ctx, s.manifestURL)
	if err != nil {
		// Fall back to a stale cache rather than failing outright.
		if m, cerr := readCFTManifest(cachePath); cerr == nil {
			bmlog.Warn("[driver] 下载 Chrome for Testing 清单失败，改用过期缓存: %v", err)
			return m, nil
		}
		return nil, fmt.Errorf("下载 Chrome for Testing 清单失败: %w", err)
	}

	var m cftManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("解析 Chrome for Testing 清单失败: %w", err)
	}
	if len(m.Versions) == 0 {
		return nil, fmt.Errorf("Chrome for Testing 清单为空")
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
		if werr := os.WriteFile(cachePath, body, 0o644); werr != nil {
			bmlog.Debug("[driver] 写入 Chrome for Testing 清单缓存失败: %v", werr)
		}
	}

	bmlog.Debug("[driver] Chrome for Testing 清单已加载 (%d 个版本)", len(m.Versions))
	return &m, nil
}

// readCFTManifest reads a manifest from disk.
func readCFTManifest(path string) (*cftManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m cftManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if len(m.Versions) == 0 {
		return nil, fmt.Errorf("清单为空")
	}
	return &m, nil
}

// fetchBytes performs a GET and returns the full body.
func (s *ChromeForTestingSource) fetchBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxManifestSize))
}

// fetchText performs a GET and returns the trimmed body as a string.
func (s *ChromeForTestingSource) fetchText(ctx context.Context, rawURL string) (string, error) {
	body, err := s.fetchBytes(ctx, rawURL)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// --- serve source ---

// ServeSource resolves drivers through a bws serve instance, which may host
// driver archives or proxy the upstream Chrome for Testing endpoints. This is
// what enables offline/intranet driver distribution.
type ServeSource struct {
	baseURL string
	client  *http.Client
}

// NewServeSource creates a source backed by a bws serve instance.
func NewServeSource(baseURL string, client *http.Client) *ServeSource {
	return &ServeSource{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// Name returns the source identifier.
func (s *ServeSource) Name() string { return "serve" }

// serveDriverManifest mirrors the API v1 envelope used by the package manifest.
type serveDriverManifest struct {
	Status string `json:"status"`
	Data   *Info  `json:"data"`
	Error  string `json:"error,omitempty"`
}

// Resolve asks the serve instance for the driver matching chromeVersion.
func (s *ServeSource) Resolve(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	endpoint := fmt.Sprintf("%s/api/v1/driver/manifest?chrome=%s&platform=%s&arch=%s",
		s.baseURL, url.QueryEscape(chromeVersion), url.QueryEscape(platform), url.QueryEscape(arch))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serve 驱动清单请求返回 HTTP %d", resp.StatusCode)
	}

	var m serveDriverManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("解析 serve 驱动清单失败: %w", err)
	}
	if m.Status != "ok" || m.Data == nil {
		if m.Error != "" {
			return nil, fmt.Errorf("serve 未能解析驱动: %s", m.Error)
		}
		return nil, fmt.Errorf("serve 未返回可用的驱动信息")
	}

	info := *m.Data
	info.Source = s.Name()

	// serve returns a relative download path so archives are fetched through
	// the same instance (proxy download). Resolve it against the base URL.
	if info.DownloadURL != "" && !strings.Contains(info.DownloadURL, "://") {
		info.DownloadURL = s.baseURL + "/" + strings.TrimLeft(info.DownloadURL, "/")
	}
	if info.Filename == "" {
		info.Filename = filepath.Base(info.DownloadURL)
	}
	return &info, nil
}