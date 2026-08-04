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

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/version"
)

// newTransportWithProxy creates an http.Transport with the given proxy.
// If proxyURL is empty, no proxy is configured.
// TLS verification is enabled by default for security. Set insecureSkipVerify
// to true only for self-signed serve instances.
func newTransportWithProxy(proxyURL string) *http.Transport {
	return newTransportWithProxyInsecure(proxyURL, false)
}

// newTransportWithProxyInsecure creates an http.Transport with optional TLS
// verification control. When insecureSkipVerify is true, TLS certificate
// validation is skipped (for self-signed serve instances only).
func newTransportWithProxyInsecure(proxyURL string, insecureSkipVerify bool) *http.Transport {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
	}

	if proxyURL != "" {
		parsed, err := neturl.Parse(proxyURL)
		if err != nil {
			// Fall back to no proxy instead of crashing the process.
			bmlog.Warn("[source] 代理 URL 无效，已回退为直连: %s: %v", proxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(parsed)
		}
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

	// Checksum is the expected checksum hash with algo prefix (e.g. "sha256:hash").
	// Empty if unknown — the checksum can be fetched on-demand via GetChecksum.
	Checksum string

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
	quiet   bool // when true, suppress [source]-level logs (used by serve adapter)
}

// NewMultiSource creates a new MultiSource from the given sources.
func NewMultiSource(sources ...Source) *MultiSource {
	return &MultiSource{sources: sources}
}

// SetQuiet controls whether the [source]-level query logs are emitted.
// When true (used by the serve adapter), MultiSource.List skips its own
// "开始查询源" / "查询成功" debug lines to avoid redundant logging —
// the caller (e.g. OnlineCacheManager) already logs at a higher level.
func (m *MultiSource) SetQuiet(q bool) {
	m.quiet = q
}

// refreshableSource 是一个可选接口，支持强制刷新缓存的源应实现此接口。
type refreshableSource interface {
	SetForceRefresh(bool)
}

// ForceRefresh 对所有支持缓存刷新的底层源设置强制刷新标志。
// 下一次 List() 调用将跳过缓存，直接从网络获取最新数据。
func (m *MultiSource) ForceRefresh() {
	for _, s := range m.sources {
		if r, ok := s.(refreshableSource); ok {
			r.SetForceRefresh(true)
		}
	}
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
		if !m.quiet {
			bmlog.Debug("[source] 开始查询源: %s (browser=%s channel=%s platform=%s arch=%s)",
				src.Name(), filter.Browser, filter.Channel, filter.Platform, filter.Arch)
		}
		versions, err := src.List(ctx, filter)
		if err != nil {
			// 记录失败日志，避免源错误被静默吞掉
			bmlog.Debug("[source] 源 %s 查询失败: %v", src.Name(), err)
			continue
		}
		if !m.quiet {
			bmlog.Debug("[source] 源 %s 查询成功: 返回 %d 个版本", src.Name(), len(versions))
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
	version = normalizeVersionAlias(browser, version)
	sources := m.sourcesForBrowser(browser)
	for _, src := range sources {
		v, err := src.Resolve(ctx, browser, version, platform, arch)
		if err == nil {
			return v, nil
		}
	}
	return VersionInfo{}, fmt.Errorf("version %s@%s not found in any source", browser, version)
}

// normalizeVersionAlias maps user-facing version aliases to the canonical form
// understood by each browser's underlying source. This provides a unified
// user experience: all aliases (latest, stable, beta, dev, canary, nightly,
// esr, release) work consistently regardless of which Source handles the
// request. Sources keep their own alias parsing as-is; this layer only
// fills the gaps where a source does not recognize a particular alias.
func normalizeVersionAlias(browser, version string) string {
	v := strings.TrimSpace(version)

	// Empty version means "latest"
	if v == "" {
		return "latest"
	}

	// If it's not a known alias, pass through as-is (version number or prefix)
	if !isKnownVersionAlias(v) {
		return v
	}

	v = strings.ToLower(v)
	b := strings.ToLower(browser)

	switch b {
	case "chrome", "chromium":
		// Chrome/Chromium (no online source with historical versions)
		// Aliases are mapped to meaningful values for local/manifest sources.
		switch v {
		case "release", "stable":
			return "stable"
		case "beta":
			return "beta"
		case "dev":
			return "dev"
		case "canary":
			return "canary"
		case "nightly", "esr":
			return "stable" // Chrome has no nightly or ESR channels
		}

	case "firefox":
		// FirefoxSource already handles: latest, beta, esr, devedition/dev
		switch v {
		case "stable", "release":
			return "latest" // Firefox "stable" is just "latest"
		case "canary":
			return "latest" // Firefox uses "nightly", not "canary"
		case "nightly":
			return "latest" // nightly builds are not in the releases directory
		}

	default:
		// All other browsers (edge, brave, opera, etc.)
		// These are served by HTTPSource which doesn't support channel aliases.
		// Map every alias to "latest" (HTTPSource returns the highest version).
		if v != "latest" {
			return "latest"
		}
	}

	return v
}

// isKnownVersionAlias reports whether s is a user-facing version alias
// as opposed to a version number or prefix.
func isKnownVersionAlias(s string) bool {
	switch strings.ToLower(s) {
	case "latest", "stable", "beta", "dev", "canary",
		"esr", "release", "nightly", "devedition":
		return true
	}
	return false
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

// FilterVersions filters a version list by the given filter criteria.
// This is the shared filtering logic used by all Source implementations
// (FirefoxSource, HTTPSource, etc.) so that direct online access and
// serve-cached access apply identical filtering rules.
//
// Empty/unknown filter fields are treated as wildcards (no filtering).
func FilterVersions(versions []VersionInfo, filter *Filter) []VersionInfo {
	if filter == nil {
		return versions
	}
	var result []VersionInfo
	for _, v := range versions {
		if filter.Browser != "" && !strings.EqualFold(filter.Browser, v.Browser) {
			continue
		}
		if filter.Channel != "" && filter.Channel != ChannelUnknown && filter.Channel != v.Channel {
			continue
		}
		if filter.Platform != "" && filter.Platform != PlatformUnknown && filter.Platform != v.Platform {
			continue
		}
		if filter.Arch != "" && filter.Arch != ArchUnknown && filter.Arch != v.Arch {
			continue
		}
		if filter.VersionPrefix != "" && !strings.HasPrefix(v.Version, filter.VersionPrefix) {
			continue
		}
		result = append(result, v)
	}
	return result
}

// compareVersions compares two version strings.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
// Delegates to version.Compare for a single consistent implementation.
func compareVersions(a, b string) int {
	return version.Compare(a, b)
}
