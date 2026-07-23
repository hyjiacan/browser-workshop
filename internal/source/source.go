// Package source provides browser version data source adapters.
// It defines a common interface for querying available browser versions
// and their download URLs from various upstream sources.
package source

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	neturl "net/url"
	"runtime"
	"sort"
	"strings"
)

// newTransportWithProxy creates an http.Transport with the given proxy.
// If proxyURL is empty, no proxy is configured.
// TLS verification is skipped for compatibility with self-signed serve instances.
func newTransportWithProxy(proxyURL string) *http.Transport {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // TLS verification skipped for self-signed serve instances
		},
	}

	if proxyURL != "" {
		parsed, err := neturl.Parse(proxyURL)
		if err != nil {
			// Should not happen: proxy URL is validated at config time.
			// Panic with a clear message instead of silently falling back to no proxy.
			panic(fmt.Sprintf("invalid proxy URL %q (should have been validated): %v", proxyURL, err))
		}
		transport.Proxy = http.ProxyURL(parsed)
	}

	return transport
}

// Channel represents a browser release channel.
type Channel string

const (
	ChannelStable  Channel = "stable"
	ChannelBeta    Channel = "beta"
	ChannelDev     Channel = "dev"
	ChannelCanary  Channel = "canary"
	ChannelESR     Channel = "esr"
	ChannelUnknown Channel = ""
)

// ParseChannel parses a channel string into a Channel type.
func ParseChannel(s string) Channel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stable":
		return ChannelStable
	case "beta":
		return ChannelBeta
	case "dev":
		return ChannelDev
	case "canary":
		return ChannelCanary
	case "esr":
		return ChannelESR
	default:
		return ChannelUnknown
	}
}

// Platform represents a target platform.
type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "darwin"
	PlatformLinux   Platform = "linux"
	PlatformUnknown Platform = ""
)

// Arch represents a target architecture.
type Arch string

const (
	ArchAMD64  Arch = "amd64"
	Arch386    Arch = "386"
	ArchARM64  Arch = "arm64"
	ArchUnknown Arch = ""
)

// VersionInfo describes an available browser version from a data source.
type VersionInfo struct {
	// Browser is the browser name (e.g. "chrome", "firefox")
	Browser string

	// Version is the version string (e.g. "120.0.6099.109")
	Version string

	// Channel is the release channel
	Channel Channel

	// Platform is the target platform
	Platform Platform

	// Arch is the target architecture
	Arch Arch

	// DownloadURL is the URL to download this version
	DownloadURL string

	// Size is the expected file size in bytes (0 if unknown)
	Size int64

	// SHA256 is the expected SHA-256 hash of the download (empty if unknown)
	SHA256 string

	// ReleaseNotes is a URL to release notes (empty if unknown)
	ReleaseNotes string
}

// Filter specifies criteria for filtering available versions.
type Filter struct {
	// Browser filters by browser name (empty = all)
	Browser string

	// Channel filters by release channel (empty = all)
	Channel Channel

	// Platform filters by platform (empty = current platform)
	Platform Platform

	// Arch filters by architecture (empty = current arch)
	Arch Arch

	// VersionPrefix filters by version prefix (e.g. "120." matches 120.x.x.x)
	VersionPrefix string
}

// Source is the interface that all browser version data sources must implement.
// A source provides information about available browser versions and their
// download URLs.
type Source interface {
	// Name returns the name of this data source.
	Name() string

	// SupportsBrowser reports whether this source supports the given browser.
	// A source should return true for browsers it can provide versions for.
	SupportsBrowser(browser string) bool

	// List returns all available versions matching the filter.
	// If filter is nil, returns all available versions for the current platform/arch.
	List(ctx context.Context, filter *Filter) ([]VersionInfo, error)

	// Latest returns the latest version matching the filter.
	// Returns the most recent version available.
	Latest(ctx context.Context, filter *Filter) (VersionInfo, error)

	// Resolve finds a specific version by version string.
	// Supports partial version matching (e.g. "120" -> latest 120.x.x.x).
	Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error)
}

// MultiSource combines multiple sources into one.
// It queries all sources and deduplicates results.
type MultiSource struct {
	sources []Source
}

// NewMultiSource creates a new MultiSource from the given sources.
func NewMultiSource(sources ...Source) *MultiSource {
	return &MultiSource{sources: sources}
}

// Name returns the name of this source.
func (m *MultiSource) Name() string {
	names := make([]string, 0, len(m.sources))
	for _, s := range m.sources {
		names = append(names, s.Name())
	}
	return "multi(" + strings.Join(names, ",") + ")"
}

// SupportsBrowser reports whether any of the underlying sources support the browser.
func (m *MultiSource) SupportsBrowser(browser string) bool {
	for _, s := range m.sources {
		if s.SupportsBrowser(browser) {
			return true
		}
	}
	return false
}

// sourcesForBrowser returns only the sources that support the given browser.
func (m *MultiSource) sourcesForBrowser(browser string) []Source {
	var result []Source
	for _, s := range m.sources {
		if s.SupportsBrowser(browser) {
			result = append(result, s)
		}
	}
	return result
}

// List returns all available versions from relevant sources.
// If filter specifies a browser, only sources that support that browser are queried.
// Results are deduplicated by (browser, version, platform, arch).
// Earlier sources take priority for duplicates.
func (m *MultiSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	sources := m.sources
	if filter != nil && filter.Browser != "" {
		sources = m.sourcesForBrowser(filter.Browser)
	}

	type key struct {
		browser  string
		version  string
		platform Platform
		arch     Arch
	}

	seen := make(map[key]bool)
	var result []VersionInfo

	for _, src := range sources {
		versions, err := src.List(ctx, filter)
		if err != nil {
			// Skip sources that fail, continue with others
			continue
		}
		for _, v := range versions {
			k := key{v.Browser, v.Version, v.Platform, v.Arch}
			if seen[k] {
				continue
			}
			seen[k] = true
			result = append(result, v)
		}
	}

	// Sort by browser, then version descending
	sort.Slice(result, func(i, j int) bool {
		if result[i].Browser != result[j].Browser {
			return result[i].Browser < result[j].Browser
		}
		return compareVersions(result[i].Version, result[j].Version) > 0
	})

	return result, nil
}

// Latest returns the latest version across relevant sources.
func (m *MultiSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := m.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found matching filter")
	}
	return versions[0], nil
}

// Resolve finds a specific version from any relevant source.
func (m *MultiSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	sources := m.sourcesForBrowser(browser)
	for _, src := range sources {
		v, err := src.Resolve(ctx, browser, version, platform, arch)
		if err == nil {
			return v, nil
		}
	}
	return VersionInfo{}, fmt.Errorf("version %s@%s not found in any source", browser, version)
}

// CurrentPlatform returns the current platform.
func CurrentPlatform() Platform {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindows
	case "darwin":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

// CurrentArch returns the current architecture.
func CurrentArch() Arch {
	switch runtime.GOARCH {
	case "amd64":
		return ArchAMD64
	case "386":
		return Arch386
	case "arm64":
		return ArchARM64
	default:
		return ArchUnknown
	}
}

// applyDefaults fills in default values for a filter.
func applyDefaults(filter *Filter) *Filter {
	if filter == nil {
		filter = &Filter{}
	}
	f := *filter
	if f.Platform == "" {
		f.Platform = CurrentPlatform()
	}
	if f.Arch == "" {
		f.Arch = CurrentArch()
	}
	return &f
}

// compareVersions compares two version strings.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum > bNum {
			return 1
		}
		if aNum < bNum {
			return -1
		}
	}
	return 0
}
