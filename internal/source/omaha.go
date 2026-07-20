package source

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChromeOmahaSource provides Chrome version information using the Google Update
// (Omaha) protocol. It queries https://tools.google.com/service/update2 with
// XML requests to obtain version info and direct download URLs.
type ChromeOmahaSource struct {
	updateURL  string
	httpClient *http.Client
}

// NewChromeOmahaSource creates a new Chrome Omaha source.
func NewChromeOmahaSource() *ChromeOmahaSource {
	return &ChromeOmahaSource{
		updateURL: "https://tools.google.com/service/update2",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

// Name returns the name of this source.
func (s *ChromeOmahaSource) Name() string {
	return "chrome-omaha"
}

// SupportsBrowser reports whether this source supports the given browser.
// ChromeOmahaSource supports Google Chrome and Chromium (they share the same Omaha protocol).
func (s *ChromeOmahaSource) SupportsBrowser(browser string) bool {
	b := strings.ToLower(browser)
	return b == "chrome" || b == "chromium"
}

// --- Omaha XML request structures ---

type omahaRequest struct {
	XMLName   xml.Name `xml:"request"`
	Protocol  string   `xml:"protocol,attr"`
	Version   string   `xml:"version,attr"`
	IsMachine string   `xml:"ismachine,attr"`
	OS        omahaOS  `xml:"os"`
	App       omahaApp `xml:"app"`
}

type omahaOS struct {
	Platform string `xml:"platform,attr"`
	Version  string `xml:"version,attr"`
	SP       string `xml:"sp,attr"`
	Arch     string `xml:"arch,attr"`
}

type omahaApp struct {
	AppID      string          `xml:"appid,attr"`
	Version    string          `xml:"version,attr"`
	NextVersion string         `xml:"nextversion,attr"`
	Lang       string          `xml:"lang,attr"`
	Brand      string          `xml:"brand,attr"`
	Client     string          `xml:"client,attr"`
	InstallAge string          `xml:"installage,attr"`
	AP         string          `xml:"ap,attr,omitempty"`
	UpdateCheck omahaUpdateCheck `xml:"updatecheck"`
}

type omahaUpdateCheck struct{}

// --- Omaha XML response structures ---

type omahaResponse struct {
	XMLName xml.Name            `xml:"response"`
	Apps    []omahaResponseApp  `xml:"app"`
}

type omahaResponseApp struct {
	AppID       string             `xml:"appid,attr"`
	Status      string             `xml:"status,attr"`
	UpdateCheck omahaRespUpdateCheck `xml:"updatecheck"`
}

type omahaRespUpdateCheck struct {
	Status   string        `xml:"status,attr"`
	URLs     omahaURLs     `xml:"urls"`
	Manifest omahaManifest `xml:"manifest"`
}

type omahaURLs struct {
	URLs []omahaURL `xml:"url"`
}

type omahaURL struct {
	CodeBase string `xml:"codebase,attr"`
}

type omahaManifest struct {
	Version  string        `xml:"version,attr"`
	Packages omahaPackages `xml:"packages"`
}

type omahaPackages struct {
	Packages []omahaPackage `xml:"package"`
}

type omahaPackage struct {
	Name string `xml:"name,attr"`
	Size int64  `xml:"size,attr"`
	SHA1 string `xml:"hash,attr"`
}

// --- App ID and AP configuration ---

// Chrome app IDs for the Omaha protocol.
const (
	chromeAppIDStable = "{8A69D345-D564-463c-AFF1-A69D9E530F96}"
	chromeAppIDCanary = "{4EA16AC7-FD5A-47C3-875B-DBF4A2008C20}"
)

// buildAP returns the "ap" (additional parameters) value for a given channel
// and architecture. Canary uses a different app ID and does not need ap.
func buildAP(channel Channel, arch Arch) string {
	archPrefix := ""
	switch arch {
	case ArchAMD64:
		archPrefix = "x64-"
	case Arch386:
		archPrefix = ""
	default:
		archPrefix = "x64-"
	}
	return archPrefix + string(channel)
}

// getAppID returns the Omaha app ID for the given channel.
func getAppID(channel Channel) string {
	if channel == ChannelCanary {
		return chromeAppIDCanary
	}
	return chromeAppIDStable
}

// mapPlatformToOmahaPlatform maps our Platform type to Omaha's platform string.
func mapPlatformToOmahaPlatform(p Platform) string {
	switch p {
	case PlatformWindows:
		return "win"
	case PlatformMacOS:
		return "mac"
	default:
		return "win"
	}
}

// mapPlatformToOSVersion returns the OS version string for Omaha requests.
func mapPlatformToOSVersion(p Platform) string {
	switch p {
	case PlatformWindows:
		return "10.0"
	case PlatformMacOS:
		return "13.0"
	default:
		return "10.0"
	}
}

// mapArchToOmahaArch maps our Arch type to Omaha's arch string.
func mapArchToOmahaArch(a Arch) string {
	switch a {
	case ArchAMD64:
		return "x64"
	case Arch386:
		return "x86"
	default:
		return "x64"
	}
}

// --- Request building ---

// buildRequest builds the Omaha XML request body for a specific combination.
func (s *ChromeOmahaSource) buildRequest(platform Platform, arch Arch, channel Channel) ([]byte, error) {
	app := omahaApp{
		AppID:       getAppID(channel),
		Version:     "",
		NextVersion: "",
		Lang:        "en",
		Brand:       "GGLS",
		Client:      "someclientid",
		InstallAge:  "-1",
		UpdateCheck: omahaUpdateCheck{},
	}

	// Canary uses a different appid and does not need the ap parameter.
	// Other channels use ap to distinguish.
	if channel != ChannelCanary {
		app.AP = buildAP(channel, arch)
	}

	req := omahaRequest{
		Protocol:  "3.0",
		Version:   "1.3.23.0",
		IsMachine: "0",
		OS: omahaOS{
			Platform: mapPlatformToOmahaPlatform(platform),
			Version:  mapPlatformToOSVersion(platform),
			SP:       "",
			Arch:     mapArchToOmahaArch(arch),
		},
		App: app,
	}

	data, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling omaha request: %w", err)
	}

	// Add XML declaration
	result := []byte(xml.Header + string(data))
	return result, nil
}

// --- HTTP query ---

// queryOmaha sends an Omaha request and returns the parsed response.
func (s *ChromeOmahaSource) queryOmaha(ctx context.Context, platform Platform, arch Arch, channel Channel) (*omahaResponse, error) {
	body, err := s.buildRequest(platform, arch, channel)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.updateURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("omaha request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omaha request returned status %d", resp.StatusCode)
	}

	var result omahaResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding omaha response: %w", err)
	}

	return &result, nil
}

// fetchVersionInfo queries Omaha for a single platform/arch/channel combination
// and returns the resulting VersionInfo, or nil if no update is available.
func (s *ChromeOmahaSource) fetchVersionInfo(ctx context.Context, browser string, platform Platform, arch Arch, channel Channel) (*VersionInfo, error) {
	resp, err := s.queryOmaha(ctx, platform, arch, channel)
	if err != nil {
		return nil, err
	}

	if len(resp.Apps) == 0 {
		return nil, fmt.Errorf("no app in omaha response")
	}

	app := resp.Apps[0]
	if app.Status != "ok" {
		return nil, fmt.Errorf("omaha app status: %s", app.Status)
	}

	uc := app.UpdateCheck
	if uc.Status != "ok" {
		// "noupdate" means no update available (e.g., already latest)
		// but since we send version="", we should always get an update.
		return nil, fmt.Errorf("omaha updatecheck status: %s", uc.Status)
	}

	manifest := uc.Manifest
	if manifest.Version == "" {
		return nil, fmt.Errorf("empty version in omaha response")
	}

	// Build download URL from first codebase + first package name
	downloadURL := ""
	size := int64(0)
	if len(uc.URLs.URLs) > 0 && len(manifest.Packages.Packages) > 0 {
		downloadURL = uc.URLs.URLs[0].CodeBase + manifest.Packages.Packages[0].Name
		size = manifest.Packages.Packages[0].Size
	}

	return &VersionInfo{
		Browser:     strings.ToLower(browser),
		Version:     manifest.Version,
		Channel:     channel,
		Platform:    platform,
		Arch:        arch,
		DownloadURL: downloadURL,
		Size:        size,
	}, nil
}

// --- Source interface implementation ---

// List returns all available Chrome/Chromium versions matching the filter.
// It queries the Omaha server for each relevant channel/platform/arch combination.
func (s *ChromeOmahaSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)

	browser := strings.ToLower(filter.Browser)
	if browser != "" && browser != "chrome" && browser != "chromium" {
		return nil, nil
	}
	if browser == "" {
		browser = "chrome"
	}

	// Determine which combinations to query
	platforms := []Platform{filter.Platform}
	arches := []Arch{filter.Arch}

	// Determine channels
	var channels []Channel
	if filter.Channel != "" {
		channels = []Channel{filter.Channel}
	} else {
		channels = []Channel{ChannelStable, ChannelBeta, ChannelDev, ChannelCanary}
	}

	// Collect results concurrently for performance
	type result struct {
		vi  *VersionInfo
		err error
	}

	var wg sync.WaitGroup
	results := make(chan result, len(platforms)*len(arches)*len(channels))

	for _, p := range platforms {
		for _, a := range arches {
			for _, ch := range channels {
				wg.Add(1)
				go func(browser string, p Platform, a Arch, ch Channel) {
					defer wg.Done()
					vi, err := s.fetchVersionInfo(ctx, browser, p, a, ch)
					results <- result{vi: vi, err: err}
				}(browser, p, a, ch)
			}
		}
	}

	wg.Wait()
	close(results)

	var versions []VersionInfo
	for r := range results {
		if r.err != nil {
			// Skip failed queries; a single failure shouldn't break the whole list
			continue
		}
		if r.vi == nil {
			continue
		}
		// Apply version prefix filter
		if filter.VersionPrefix != "" && !strings.HasPrefix(r.vi.Version, filter.VersionPrefix) {
			continue
		}
		versions = append(versions, *r.vi)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no %s versions found from omaha", browser)
	}

	// Sort by version descending
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return versions, nil
}

// Latest returns the latest Chrome/Chromium version matching the filter.
func (s *ChromeOmahaSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	filter = applyDefaults(filter)

	browser := strings.ToLower(filter.Browser)
	if browser != "" && browser != "chrome" && browser != "chromium" {
		return VersionInfo{}, fmt.Errorf("chrome omaha source only handles chrome/chromium browser")
	}
	if browser == "" {
		browser = "chrome"
	}

	channel := filter.Channel
	if channel == "" {
		channel = ChannelStable
	}

	vi, err := s.fetchVersionInfo(ctx, browser, filter.Platform, filter.Arch, channel)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("fetching latest %s %s: %w", browser, channel, err)
	}
	if vi == nil {
		return VersionInfo{}, fmt.Errorf("no version found for %s %s", browser, channel)
	}

	return *vi, nil
}

// Resolve finds a specific Chrome/Chromium version.
// Supports "latest"/"stable"/"beta"/"dev"/"canary" keywords and partial version matching.
func (s *ChromeOmahaSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	b := strings.ToLower(browser)
	if b != "chrome" && b != "chromium" {
		return VersionInfo{}, fmt.Errorf("chrome omaha source only handles chrome/chromium browser")
	}

	// Handle special keywords
	switch strings.ToLower(version) {
	case "latest", "stable", "":
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

	// For a specific version number, try all channels to find a match.
	// The Omaha protocol only returns the latest version for each channel,
	// so we check each channel's latest version against the requested version.
	// If the version matches exactly, return it.
	// For partial matches (e.g. "120"), return the highest matching version.
	channels := []Channel{ChannelStable, ChannelBeta, ChannelDev, ChannelCanary}

	var matches []VersionInfo
	for _, ch := range channels {
		vi, err := s.fetchVersionInfo(ctx, b, platform, arch, ch)
		if err != nil {
			continue
		}
		if vi == nil {
			continue
		}

		// Exact match
		if vi.Version == version {
			return *vi, nil
		}

		// Partial match (e.g. "120" -> "120.x.x.x")
		prefix := version + "."
		if strings.HasPrefix(vi.Version, prefix) {
			matches = append(matches, *vi)
		}
	}

	if len(matches) > 0 {
		// Return the highest matching version
		sort.Slice(matches, func(i, j int) bool {
			return compareVersions(matches[i].Version, matches[j].Version) > 0
		})
		return matches[0], nil
	}

	return VersionInfo{}, fmt.Errorf("%s version %s not found in any channel", b, version)
}
