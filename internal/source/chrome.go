package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ChromeSource provides Chrome version information using the OmahaProxy API.
// It queries https://omahaproxy.appspot.com for version data.
type ChromeSource struct {
	baseURL    string
	httpClient *http.Client
}

// NewChromeSource creates a new Chrome version source.
func NewChromeSource() *ChromeSource {
	return NewChromeSourceWithProxy("")
}

// NewChromeSourceWithProxy creates a new Chrome version source that uses the given proxy.
func NewChromeSourceWithProxy(proxyURL string) *ChromeSource {
	return &ChromeSource{
		baseURL:    "https://omahaproxy.appspot.com",
		httpClient: &http.Client{Timeout: 30 * time.Second, Transport: newTransportWithProxy(proxyURL)},
	}
}

// Name returns the name of this source.
func (s *ChromeSource) Name() string {
	return "chrome-omahaproxy"
}

// SupportsBrowser reports whether this source supports the given browser.
// ChromeSource supports Google Chrome and Chromium.
func (s *ChromeSource) SupportsBrowser(browser string) bool {
	b := strings.ToLower(browser)
	return b == "chrome" || b == "chromium"
}

// omahaVersion represents a version entry in the OmahaProxy JSON response.
type omahaVersion struct {
	Channel    string `json:"channel"`
	Version    string `json:"version"`
	CurrentRelDate string `json:"current_reldate"`
}

// omahaEntry represents a platform entry in the OmahaProxy JSON response.
type omahaEntry struct {
	OS       string         `json:"os"`
	Versions []omahaVersion `json:"versions"`
}

// List returns all available Chrome/Chromium versions matching the filter.
func (s *ChromeSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	browser := strings.ToLower(filter.Browser)
	if browser != "" && browser != "chrome" && browser != "chromium" {
		return nil, nil
	}
	if browser == "" {
		browser = "chrome"
	}

	// Fetch all versions from omahaproxy
	entries, err := s.fetchAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching %s versions: %w", browser, err)
	}

	var result []VersionInfo
	for _, entry := range entries {
		platform := mapOmahaPlatform(entry.OS)
		if filter.Platform != "" && platform != filter.Platform {
			continue
		}

		for _, v := range entry.Versions {
			channel := ParseChannel(v.Channel)
			if filter.Channel != "" && channel != filter.Channel {
				continue
			}
			if filter.VersionPrefix != "" && !strings.HasPrefix(v.Version, filter.VersionPrefix) {
				continue
			}

			arch := filter.Arch
			if arch == "" {
				arch = CurrentArch()
			}

			downloadURL := s.buildDownloadURL(v.Version, platform, arch, channel)

			result = append(result, VersionInfo{
				Browser:     browser,
				Version:     v.Version,
				Channel:     channel,
				Platform:    platform,
				Arch:        arch,
				DownloadURL: downloadURL,
			})
		}
	}

	// Sort by version descending
	sortVersionsDesc(result)

	return result, nil
}

// Latest returns the latest Chrome version matching the filter.
func (s *ChromeSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := s.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no chrome versions found matching filter")
	}
	return versions[0], nil
}

// Resolve finds a specific Chrome/Chromium version.
// Supports partial version matching (e.g. "120" -> latest 120.x.x.x).
func (s *ChromeSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	b := strings.ToLower(browser)
	if b != "chrome" && b != "chromium" {
		return VersionInfo{}, fmt.Errorf("chrome source only handles chrome/chromium browser")
	}

	// If it's a special keyword
	switch strings.ToLower(version) {
	case "latest", "stable":
		return s.Latest(ctx, &Filter{
			Browser:  b,
			Channel:  ChannelStable,
			Platform: platform,
			Arch:     arch,
		})
	case "beta":
		return s.Latest(ctx, &Filter{
			Browser:  b,
			Channel:  ChannelBeta,
			Platform: platform,
			Arch:     arch,
		})
	case "dev":
		return s.Latest(ctx, &Filter{
			Browser:  b,
			Channel:  ChannelDev,
			Platform: platform,
			Arch:     arch,
		})
	case "canary":
		return s.Latest(ctx, &Filter{
			Browser:  b,
			Channel:  ChannelCanary,
			Platform: platform,
			Arch:     arch,
		})
	}

	// Try exact match first
	versions, err := s.List(ctx, &Filter{
		Browser:  b,
		Platform: platform,
		Arch:     arch,
	})
	if err != nil {
		return VersionInfo{}, err
	}

	// Exact version match
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// Partial version match (e.g. "120" -> latest 120.x.x.x)
	prefix := version + "."
	var matches []VersionInfo
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) > 0 {
		sortVersionsDesc(matches)
		return matches[0], nil
	}

	return VersionInfo{}, fmt.Errorf("%s version %s not found", b, version)
}

// fetchAll fetches all version data from omahaproxy.
func (s *ChromeSource) fetchAll(ctx context.Context) ([]omahaEntry, error) {
	url := s.baseURL + "/all.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var entries []omahaEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return entries, nil
}

// buildDownloadURL constructs the download URL for a Chrome version.
// Chrome download URLs follow patterns like:
// - https://dl.google.com/release2/chrome/.../120.0.6099.109_chrome64_stable_windows_installer.exe
// Since we can't get the exact URL from omahaproxy alone, we use the known pattern
// for the standalone installer.
func (s *ChromeSource) buildDownloadURL(version string, platform Platform, arch Arch, channel Channel) string {
	// Chrome's official download page pattern
	// The exact URL requires additional info (the full path hash)
	// For now, return a best-effort URL pattern that can be used for reference
	// In practice, a full implementation would need to query Chrome's update API
	// or use a known mirror.

	// Known pattern for Google Chrome standalone enterprise installers:
	// https://dl.google.com/dl/chrome/install/googlechromestandaloneenterprise64.msi (latest)
	// But for specific versions, we need the Omaha update protocol to get exact URLs.

	// Return the version-specific download URL pattern
	// Note: This is a placeholder. The actual URL requires the Omaha update check
	// which gives the precise download path with hash.
	osPart := mapPlatformToOmahaOS(platform)
	archPart := mapArchToChromeArch(arch)
	chPart := string(channel)
	if chPart == "" {
		chPart = "stable"
	}

	// Format commonly seen in Chrome downloads
	return fmt.Sprintf("https://dl.google.com/release2/chrome/%s_%s_%s_%s_installer.exe",
		version, archPart, chPart, osPart)
}

// mapOmahaPlatform maps Omaha OS names to our Platform type.
func mapOmahaPlatform(os string) Platform {
	switch strings.ToLower(os) {
	case "win", "win64", "windows":
		return PlatformWindows
	case "mac", "macos", "darwin":
		return PlatformMacOS
	case "linux", "linux64":
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

// mapPlatformToOmahaOS maps our Platform type to Omaha OS name.
func mapPlatformToOmahaOS(p Platform) string {
	switch p {
	case PlatformWindows:
		return "windows"
	case PlatformMacOS:
		return "mac"
	case PlatformLinux:
		return "linux"
	default:
		return "windows"
	}
}

// mapArchToChromeArch maps our Arch type to Chrome's arch naming.
func mapArchToChromeArch(a Arch) string {
	switch a {
	case ArchAMD64:
		return "chrome64"
	case Arch386:
		return "chrome32"
	default:
		return "chrome64"
	}
}

// sortVersionsDesc sorts versions by version number descending.
func sortVersionsDesc(versions []VersionInfo) {
	sortVersions(versions, true)
}

// sortVersions sorts versions by version number.
func sortVersions(versions []VersionInfo, desc bool) {
	sort.Slice(versions, func(i, j int) bool {
		cmp := compareVersions(versions[i].Version, versions[j].Version)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}
