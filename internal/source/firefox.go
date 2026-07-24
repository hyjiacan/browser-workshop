package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	firefoxProductDetailsURL    = "https://product-details.mozilla.org/1.0/firefox_versions.json"
	firefoxHistoryMajorURL      = "https://product-details.mozilla.org/1.0/firefox_history_major_releases.json"
	firefoxHistoryStabilityURL = "https://product-details.mozilla.org/1.0/firefox_history_stability_releases.json"
	firefoxDownloadBase         = "https://download.mozilla.org/"
)

// FirefoxSource provides version data for Mozilla Firefox using Mozilla's
// Product Details API.
type FirefoxSource struct {
	httpClient *http.Client
}

// NewFirefoxSource creates a new FirefoxSource.
func NewFirefoxSource() *FirefoxSource {
	return NewFirefoxSourceWithProxy("")
}

// NewFirefoxSourceWithProxy creates a new FirefoxSource that uses the given proxy.
func NewFirefoxSourceWithProxy(proxyURL string) *FirefoxSource {
	return &FirefoxSource{
		httpClient: &http.Client{Timeout: 30 * time.Second, Transport: newTransportWithProxy(proxyURL)},
	}
}

// Name returns the name of this source.
func (s *FirefoxSource) Name() string {
	return "firefox-mozilla"
}

// SupportsBrowser reports whether this source supports the given browser.
func (s *FirefoxSource) SupportsBrowser(browser string) bool {
	return strings.ToLower(browser) == "firefox"
}

// List returns all available Firefox versions matching the filter.
// It combines data from:
// - firefox_versions.json: latest versions for each channel (stable, beta, esr, devedition, nightly)
// - firefox_history_major_releases.json: all historical major stable releases
// - firefox_history_stability_releases.json: historical patch releases for ESR branches
func (s *FirefoxSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	channel := filter.Channel
	if channel == "" {
		channel = ChannelStable
	}

	// Collect versions from multiple sources (parallel fetch)
	type fetchResult struct {
		latestMap         map[Channel]string
		historyVersions   []string
		stabilityVersions []string
	}

	var fr fetchResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fetchErr error

	// 1. Fetch latest channel versions (required)
	wg.Add(1)
	go func() {
		defer wg.Done()
		m, err := s.fetchLatestVersions(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			fetchErr = err
		} else {
			fr.latestMap = m
		}
	}()

	// 2. Fetch historical major releases (optional)
	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := s.fetchHistoryMajorVersions(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			fr.historyVersions = v
		}
	}()

	// 3. Fetch historical stability/patch releases (optional)
	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := s.fetchHistoryStabilityVersions(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			fr.stabilityVersions = v
		}
	}()

	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	var results []VersionInfo

	// Add latest versions for all channels
	for ch, ver := range fr.latestMap {
		if ver == "" {
			continue
		}
		plat := filter.Platform
		arch := filter.Arch
		if plat == "" {
			plat = CurrentPlatform()
		}
		if arch == "" {
			arch = CurrentArch()
		}

		results = append(results, VersionInfo{
			Browser:     "firefox",
			Version:     ver,
			Channel:     ch,
			Platform:    plat,
			Arch:        arch,
			DownloadURL: s.buildDownloadURL(ver, plat, arch, ch),
			Size:        0,
		})
	}

	// Build a set of known ESR base versions from latestMap
	esrBases := make(map[string]bool)
	for ch, ver := range fr.latestMap {
		if ch == ChannelESR && ver != "" {
			base := extractMajorVersion(ver)
			if base != "" {
				esrBases[base] = true
			}
		}
	}

	plat := filter.Platform
	if plat == "" {
		plat = CurrentPlatform()
	}
	arch := filter.Arch
	if arch == "" {
		arch = CurrentArch()
	}

	// Add historical stable major versions
	for _, ver := range fr.historyVersions {
		results = append(results, VersionInfo{
			Browser:     "firefox",
			Version:     ver,
			Channel:     ChannelStable,
			Platform:    plat,
			Arch:        arch,
			DownloadURL: s.buildDownloadURL(ver, plat, arch, ChannelStable),
			Size:        0,
		})
	}

	// Add stability patch releases, classifying them by channel
	for _, ver := range fr.stabilityVersions {
		ch := classifyFirefoxVersion(ver, esrBases)
		results = append(results, VersionInfo{
			Browser:     "firefox",
			Version:     ver,
			Channel:     ch,
			Platform:    plat,
			Arch:        arch,
			DownloadURL: s.buildDownloadURL(ver, plat, arch, ch),
			Size:        0,
		})
	}

	// Filter by channel
	if channel != "" {
		var filtered []VersionInfo
		for _, v := range results {
			if v.Channel == channel {
				filtered = append(filtered, v)
			}
		}
		results = filtered
	}

	// Filter by version prefix
	if filter.VersionPrefix != "" {
		var filtered []VersionInfo
		for _, v := range results {
			if strings.HasPrefix(v.Version, filter.VersionPrefix) {
				filtered = append(filtered, v)
			}
		}
		results = filtered
	}

	// Deduplicate by (version, platform, arch)
	seen := make(map[string]bool)
	var deduped []VersionInfo
	for _, v := range results {
		key := fmt.Sprintf("%s|%s|%s|%s", v.Version, v.Platform, v.Arch, v.Channel)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, v)
		}
	}
	results = deduped

	// Sort by version descending
	sort.Slice(results, func(i, j int) bool {
		return compareVersions(results[i].Version, results[j].Version) > 0
	})

	return results, nil
}

// Latest returns the latest Firefox version matching the filter.
func (s *FirefoxSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	filter = applyDefaults(filter)

	channel := filter.Channel
	if channel == "" {
		channel = ChannelStable
	}

	// For latest, just query the current versions API
	latestMap, err := s.fetchLatestVersions(ctx)
	if err != nil {
		return VersionInfo{}, err
	}

	ver, ok := latestMap[channel]
	if !ok || ver == "" {
		return VersionInfo{}, fmt.Errorf("no firefox %s version found", channel)
	}

	plat := filter.Platform
	arch := filter.Arch
	if plat == "" {
		plat = CurrentPlatform()
	}
	if arch == "" {
		arch = CurrentArch()
	}

	return VersionInfo{
		Browser:     "firefox",
		Version:     ver,
		Channel:     channel,
		Platform:    plat,
		Arch:        arch,
		DownloadURL: s.buildDownloadURL(ver, plat, arch, channel),
		Size:        0,
	}, nil
}

// Resolve finds a specific Firefox version.
func (s *FirefoxSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	// Handle aliases
	if version == "latest" || version == "" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch})
	}
	if version == "beta" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelBeta})
	}
	if version == "esr" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelESR})
	}
	if version == "devedition" || version == "dev" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelDev})
	}
	if version == "nightly" {
		return s.Latest(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch, Channel: ChannelCanary})
	}

	// Try exact match first
	list, err := s.List(ctx, &Filter{Browser: browser, Platform: platform, Arch: arch})
	if err != nil {
		return VersionInfo{}, err
	}

	for _, v := range list {
		if v.Version == version {
			return v, nil
		}
	}

	// Try prefix match
	for _, v := range list {
		if strings.HasPrefix(v.Version, version) {
			return v, nil
		}
	}

	return VersionInfo{}, fmt.Errorf("firefox version %s not found", version)
}

// --- internal helpers ---

type firefoxVersionsResponse struct {
	LATEST_FIREFOX_VERSION              string `json:"LATEST_FIREFOX_VERSION"`
	LATEST_FIREFOX_DEVEL_VERSION       string `json:"LATEST_FIREFOX_DEVEL_VERSION"`
	FIREFOX_ESR                         string `json:"FIREFOX_ESR"`
	FIREFOX_ESR115                      string `json:"FIREFOX_ESR115"`
	FIREFOX_DEVEDITION                  string `json:"FIREFOX_DEVEDITION"`
	FIREFOX_NIGHTLY                     string `json:"FIREFOX_NIGHTLY"`
	LATEST_FIREFOX_RELEASED_DEVEL_VERSION string `json:"LATEST_FIREFOX_RELEASED_DEVEL_VERSION"`
}

// fetchLatestVersions returns a map of channel -> latest version string.
func (s *FirefoxSource) fetchLatestVersions(ctx context.Context) (map[Channel]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", firefoxProductDetailsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firefox versions API returned %d", resp.StatusCode)
	}

	var data firefoxVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	result := make(map[Channel]string)

	if data.LATEST_FIREFOX_VERSION != "" {
		result[ChannelStable] = data.LATEST_FIREFOX_VERSION
	}
	if data.LATEST_FIREFOX_DEVEL_VERSION != "" {
		result[ChannelBeta] = data.LATEST_FIREFOX_DEVEL_VERSION
	}
	if data.FIREFOX_ESR != "" {
		result[ChannelESR] = data.FIREFOX_ESR
	}
	// Also add older ESR if available and different
	if data.FIREFOX_ESR115 != "" && data.FIREFOX_ESR115 != data.FIREFOX_ESR {
		result[ChannelESR] = data.FIREFOX_ESR115 // Multiple ESR lines: keep latest, could extend
	}
	if data.FIREFOX_DEVEDITION != "" {
		result[ChannelDev] = data.FIREFOX_DEVEDITION
	}
	if data.FIREFOX_NIGHTLY != "" {
		result[ChannelCanary] = data.FIREFOX_NIGHTLY
	}

	return result, nil
}

// fetchHistoryMajorVersions fetches all historical major Firefox release version numbers.
func (s *FirefoxSource) fetchHistoryMajorVersions(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", firefoxHistoryMajorURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firefox history API returned %d", resp.StatusCode)
	}

	var history map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, err
	}

	var versions []string
	for ver := range history {
		versions = append(versions, ver)
	}

	return versions, nil
}

// fetchHistoryStabilityVersions fetches historical patch/stability release version numbers.
// These include point releases for both stable and ESR branches.
func (s *FirefoxSource) fetchHistoryStabilityVersions(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", firefoxHistoryStabilityURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firefox stability history API returned %d", resp.StatusCode)
	}

	var history map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, err
	}

	var versions []string
	for ver := range history {
		versions = append(versions, ver)
	}

	return versions, nil
}

// buildDownloadURL constructs the download URL for a Firefox version.
func (s *FirefoxSource) buildDownloadURL(version string, platform Platform, arch Arch, channel Channel) string {
	osParam := mapToMozillaOS(platform, arch)
	if osParam == "" {
		osParam = "win64"
	}

	// For specific versions, use the version-specific download URL
	product := "firefox"
	lang := "en-US"

	switch channel {
	case ChannelBeta:
		product = "firefox-beta"
	case ChannelESR:
		product = "firefox-esr"
	case ChannelDev:
		product = "firefox-devedition"
	case ChannelCanary:
		product = "firefox-nightly"
	default:
		product = "firefox"
	}

	// Use the standard Mozilla download URL pattern for specific versions:
	// https://download.mozilla.org/?product=firefox-<version>-SSL&os=<os>&lang=en-US
	// For "latest" style downloads, Mozilla uses product=firefox-latest-ssl
	// But for version-specific, we use the version number directly.

	// Clean version: remove esr/b suffixes for the product string
	cleanVer := strings.TrimSuffix(strings.TrimSuffix(version, "esr"), "b")
	cleanVer = strings.TrimSuffix(cleanVer, "a")

	return fmt.Sprintf("%s?product=%s-%s-SSL&os=%s&lang=%s",
		firefoxDownloadBase, product, cleanVer, osParam, lang)
}

func mapToMozillaOS(platform Platform, arch Arch) string {
	switch platform {
	case PlatformWindows:
		if arch == Arch386 {
			return "win"
		}
		return "win64"
	case PlatformMacOS:
		if arch == ArchARM64 {
			return "osx"
		}
		return "osx"
	case PlatformLinux:
		if arch == ArchARM64 {
			return "linux-aarch64"
		}
		return "linux64"
	default:
		return "win64"
	}
}

// extractMajorVersion extracts the major version number from a version string.
// e.g., "128.8.0esr" -> "128", "136.0.2" -> "136"
func extractMajorVersion(ver string) string {
	parts := strings.Split(ver, ".")
	if len(parts) == 0 {
		return ""
	}
	// Strip non-numeric suffix
	major := parts[0]
	for i, c := range major {
		if c < '0' || c > '9' {
			if i > 0 {
				return major[:i]
			}
			return ""
		}
	}
	return major
}

// classifyFirefoxVersion determines the channel of a version string.
// ESR versions contain "esr" suffix, others are stable.
func classifyFirefoxVersion(ver string, knownESRBases map[string]bool) Channel {
	lower := strings.ToLower(ver)
	if strings.Contains(lower, "esr") {
		return ChannelESR
	}
	// Check if this is a patch release for a known ESR base
	major := extractMajorVersion(ver)
	if major != "" && knownESRBases[major] {
		return ChannelESR
	}
	return ChannelStable
}
