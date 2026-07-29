package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// HTTPSource provides browser versions from a bm serve HTTP endpoint.
// It queries the manifest API of a bm serve instance.
// Supports both the new API v1 format and the legacy format.
type HTTPSource struct {
	baseURL    string
	name       string
	httpClient *http.Client
}

// --- New API v1 types ---

// manifestV1Response is the JSON response from the /api/v1/manifest endpoint.
type manifestV1Response struct {
	Status string            `json:"status"`
	Data   []manifestV1File  `json:"data"`
	Server manifestV1Server  `json:"server"`
}

type manifestV1File struct {
	Filename     string `json:"filename"`
	Version      string `json:"version"`
	MajorVersion string `json:"major_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
}

type manifestV1Server struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	FileCount int    `json:"file_count"`
}

// browserKeywords maps browser name keywords to canonical browser names.
// Used to detect browser name from filenames in the v1 API.
var browserKeywords = []struct {
	keyword string
	name    string
}{
	{"chrome", "chrome"},
	{"chromium", "chromium"},
	{"firefox", "firefox"},
	{"edge", "edge"},
	{"brave", "brave"},
	{"opera", "opera"},
	{"safari", "safari"},
	{"vivaldi", "vivaldi"},
	{"thorium", "thorium"},
	{"ungoogled", "ungoogled-chromium"},
}

// detectBrowserFromFilename extracts the browser name from a filename
// using keyword matching. Returns empty string if unrecognized.
func detectBrowserFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	for _, kw := range browserKeywords {
		if strings.Contains(lower, kw.keyword) {
			return kw.name
		}
	}
	return ""
}

// detectChannelFromFilename extracts the channel from a filename.
func detectChannelFromFilename(filename string) Channel {
	lower := strings.ToLower(filename)
	switch {
	case strings.Contains(lower, "canary"):
		return ChannelCanary
	case strings.Contains(lower, "dev"):
		return ChannelDev
	case strings.Contains(lower, "beta"):
		return ChannelBeta
	case strings.Contains(lower, "esr"):
		return ChannelESR
	default:
		return ChannelStable
	}
}

// normalizePlatform normalizes platform name to our Platform type.
func normalizePlatform(p string) Platform {
	switch strings.ToLower(p) {
	case "windows", "win":
		return PlatformWindows
	case "darwin", "macos", "mac":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

// normalizeArch normalizes architecture name to our Arch type.
func normalizeArch(a string) Arch {
	switch strings.ToLower(a) {
	case "x64", "amd64", "x86_64", "win64", "64":
		return ArchAMD64
	case "x86", "386", "x86_32", "win32", "32":
		return Arch386
	case "arm64", "aarch64":
		return ArchARM64
	default:
		return ArchUnknown
	}
}

// NewHTTPSource creates a new HTTP source from a bm serve base URL.
func NewHTTPSource(baseURL string) *HTTPSource {
	return NewHTTPSourceWithProxy(baseURL, "")
}

// NewHTTPSourceWithProxy creates an HTTPSource that uses the given proxy.
// proxyURL can be empty (direct), "http://host:port", "socks5://host:port", etc.
func NewHTTPSourceWithProxy(baseURL string, proxyURL string) *HTTPSource {
	// Normalize base URL - remove trailing slash
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPSource{
		baseURL:    baseURL,
		name:       "http:" + baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second, Transport: newTransportWithProxy(proxyURL)},
	}
}

// Name returns the name of this source.
func (s *HTTPSource) Name() string {
	return s.name
}

// SupportsBrowser reports whether this source supports the given browser.
// HTTPSource is a generic serve endpoint that can host any browser type.
func (s *HTTPSource) SupportsBrowser(browser string) bool {
	return true // serve endpoint supports all browser types
}

// List returns all available versions matching the filter.
func (s *HTTPSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	v1Manifest, err := s.fetchManifest(ctx, filter)
	if err != nil {
		return nil, err
	}

	var results []VersionInfo

	if v1Manifest != nil {
		results = s.processV1Manifest(v1Manifest, filter)
	}

	return results, nil
}

// processV1Manifest processes the new v1 API response and returns filtered version info.
func (s *HTTPSource) processV1Manifest(m *manifestV1Response, filter *Filter) []VersionInfo {
	var results []VersionInfo

	for _, f := range m.Data {
		// Detect browser from filename
		browser := detectBrowserFromFilename(f.Filename)
		if browser == "" {
			continue
		}

		// Filter by browser
		if filter.Browser != "" && !strings.EqualFold(filter.Browser, browser) {
			continue
		}

		// Determine channel (from filename, since v1 API doesn't include it directly)
		channel := detectChannelFromFilename(f.Filename)

		// Filter by channel
		if filter.Channel != "" && filter.Channel != ChannelUnknown && filter.Channel != channel {
			continue
		}

		// Normalize platform
		platform := normalizePlatform(f.Platform)

		// Filter by platform
		if filter.Platform != "" && filter.Platform != PlatformUnknown && filter.Platform != platform {
			continue
		}

		// Normalize arch
		arch := normalizeArch(f.Architecture)

		// Filter by arch
		if filter.Arch != "" && filter.Arch != ArchUnknown && filter.Arch != arch {
			continue
		}

		// Filter by version prefix
		if filter.VersionPrefix != "" && !strings.HasPrefix(f.Version, filter.VersionPrefix) {
			continue
		}

		downloadURL := fmt.Sprintf("%s/api/v1/download/%s", s.baseURL, neturl.PathEscape(f.Filename))

		results = append(results, VersionInfo{
			Browser:     browser,
			Version:     f.Version,
			Channel:     channel,
			Platform:    platform,
			Arch:        arch,
			DownloadURL: downloadURL,
			Size:        f.Size,
			SHA256:      f.Checksum,
		})
	}

	return results
}

// Latest returns the latest version matching the filter.
func (s *HTTPSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := s.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found matching the given criteria")
	}

	// Find the latest version with the highest version number
	latest := versions[0]
	for _, v := range versions[1:] {
		if compareVersions(v.Version, latest.Version) > 0 {
			latest = v
		}
	}
	return latest, nil
}

// Resolve finds a specific version by version string.
// Supports "latest" and partial version prefixes.
func (s *HTTPSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	browser = strings.ToLower(browser)

	// Handle "latest"
	if version == "" || strings.ToLower(version) == "latest" {
		return s.Latest(ctx, &Filter{
			Browser:  browser,
			Platform: platform,
			Arch:     arch,
		})
	}

	versions, err := s.List(ctx, &Filter{
		Browser:  browser,
		Platform: platform,
		Arch:     arch,
	})
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found for %s", browser)
	}

	// Exact match
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// Prefix match - find the latest matching version
	// Use "." separator to avoid "12" matching "120.x.x.x"
	prefix := version + "."
	var matches []VersionInfo
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return VersionInfo{}, fmt.Errorf("version %s not found for %s", version, browser)
	}

	// Return the highest matching version
	latest := matches[0]
	for _, v := range matches[1:] {
		if compareVersions(v.Version, latest.Version) > 0 {
			latest = v
		}
	}
	return latest, nil
}

// fetchManifest fetches the manifest from the server.
// When filter is non-nil, browser/platform/arch/channel query parameters are
// forwarded to the server so it can narrow the manifest (and, for online
// fallback servers, only query the requested browser from online sources).
// Returns (v1Manifest, error).
func (s *HTTPSource) fetchManifest(ctx context.Context, filter *Filter) (*manifestV1Response, error) {
	v1URL := s.baseURL + "/api/v1/manifest"
	if params := buildManifestQuery(filter); len(params) > 0 {
		v1URL += "?" + params.Encode()
	}

	v1Resp, v1Body, err := s.fetchJSON(ctx, v1URL)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	if v1Resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest request returned status %d", v1Resp.StatusCode)
	}

	var v1 manifestV1Response
	if err := json.Unmarshal(v1Body, &v1); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	if v1.Status == "ok" && v1.Data != nil {
		return &v1, nil
	}

	return nil, fmt.Errorf("unrecognized manifest format")
}

// buildManifestQuery builds the query parameters for the manifest API request
// based on the filter. Returns an empty Values if no filter criteria apply.
func buildManifestQuery(filter *Filter) neturl.Values {
	params := neturl.Values{}
	if filter == nil {
		return params
	}
	if filter.Browser != "" {
		params.Set("browser", filter.Browser)
	}
	if filter.Platform != "" && filter.Platform != PlatformUnknown {
		params.Set("platform", string(filter.Platform))
	}
	if filter.Arch != "" && filter.Arch != ArchUnknown {
		params.Set("arch", string(filter.Arch))
	}
	if filter.Channel != "" && filter.Channel != ChannelUnknown {
		params.Set("channel", string(filter.Channel))
	}
	return params
}

// maxResponseBodySize is the maximum HTTP response body size (100MB).
const maxResponseBodySize = 100 << 20

// fetchJSON fetches a URL and returns the response and body bytes.
func (s *HTTPSource) fetchJSON(ctx context.Context, url string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return resp, nil, fmt.Errorf("reading response body: %w", err)
	}

	return resp, body, nil
}
