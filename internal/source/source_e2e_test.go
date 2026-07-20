package source

import (
	"context"
	"errors"
	"testing"
)

// TestMultiSource_filtersByBrowser verifies that MultiSource only queries
// sources that support the requested browser.
func TestMultiSource_filtersByBrowser(t *testing.T) {
	// Create sources with browser-specific support
	serveSrc := &mockBrowserSource{
		name:              "serve",
		supportedBrowsers: []string{"chrome", "chromium", "firefox", "edge"},
		versions: []VersionInfo{
			{Browser: "chrome", Version: "120.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			{Browser: "chromium", Version: "120.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			{Browser: "firefox", Version: "119.0", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			{Browser: "edge", Version: "120.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}
	omahaSrc := &mockBrowserSource{
		name:              "omaha",
		supportedBrowsers: []string{"chrome", "chromium"},
		versions: []VersionInfo{
			{Browser: "chrome", Version: "121.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			{Browser: "chromium", Version: "121.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}
	firefoxSrc := &mockBrowserSource{
		name:              "firefox",
		supportedBrowsers: []string{"firefox"},
		versions: []VersionInfo{
			{Browser: "firefox", Version: "120.0", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}

	multi := NewMultiSource(serveSrc, omahaSrc, firefoxSrc)
	ctx := context.Background()

	// --- Query chrome: all three sources should be queried ---
	serveSrc.queried = false
	omahaSrc.queried = false
	firefoxSrc.queried = false

	versions, err := multi.List(ctx, &Filter{Browser: "chrome", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List chrome failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("chrome: expected 2 versions, got %d", len(versions))
	}
	if !serveSrc.queried {
		t.Error("chrome: serve source should have been queried")
	}
	if !omahaSrc.queried {
		t.Error("chrome: omaha source should have been queried")
	}
	if firefoxSrc.queried {
		t.Error("chrome: firefox source should NOT have been queried")
	}

	// --- Query chromium: serve and omaha should be queried ---
	serveSrc.queried = false
	omahaSrc.queried = false
	firefoxSrc.queried = false

	versions, err = multi.List(ctx, &Filter{Browser: "chromium", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List chromium failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("chromium: expected 2 versions, got %d", len(versions))
	}
	if !serveSrc.queried {
		t.Error("chromium: serve source should have been queried")
	}
	if !omahaSrc.queried {
		t.Error("chromium: omaha source should have been queried")
	}
	if firefoxSrc.queried {
		t.Error("chromium: firefox source should NOT have been queried")
	}

	// --- Query firefox: only serve and firefox should be queried ---
	serveSrc.queried = false
	omahaSrc.queried = false
	firefoxSrc.queried = false

	versions, err = multi.List(ctx, &Filter{Browser: "firefox", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List firefox failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("firefox: expected 2 versions, got %d", len(versions))
	}
	if !serveSrc.queried {
		t.Error("firefox: serve source should have been queried")
	}
	if omahaSrc.queried {
		t.Error("firefox: omaha source should NOT have been queried")
	}
	if !firefoxSrc.queried {
		t.Error("firefox: firefox source should have been queried")
	}

	// --- Query edge: only serve should be queried ---
	serveSrc.queried = false
	omahaSrc.queried = false
	firefoxSrc.queried = false

	versions, err = multi.List(ctx, &Filter{Browser: "edge", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List edge failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("edge: expected 1 version, got %d", len(versions))
	}
	if !serveSrc.queried {
		t.Error("edge: serve source should have been queried")
	}
	if omahaSrc.queried {
		t.Error("edge: omaha source should NOT have been queried")
	}
	if firefoxSrc.queried {
		t.Error("edge: firefox source should NOT have been queried")
	}
}

// TestMultiSource_continuesOnEmptyResult verifies that if one source returns
// empty results, MultiSource continues to query other sources.
func TestMultiSource_continuesOnEmptyResult(t *testing.T) {
	emptySrc := &mockBrowserSource{
		name:              "empty",
		supportedBrowsers: []string{"chrome"},
		versions:          nil, // returns empty
	}
	hasVersionsSrc := &mockBrowserSource{
		name:              "has-versions",
		supportedBrowsers: []string{"chrome"},
		versions: []VersionInfo{
			{Browser: "chrome", Version: "120.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}

	multi := NewMultiSource(emptySrc, hasVersionsSrc)
	ctx := context.Background()

	versions, err := multi.List(ctx, &Filter{Browser: "chrome", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version from fallback source, got %d", len(versions))
	}
	if versions[0].Version != "120.0.0.1" {
		t.Errorf("version = %s, want 120.0.0.1", versions[0].Version)
	}
}

// TestMultiSource_continuesOnError verifies that if one source errors,
// MultiSource continues to query other sources.
func TestMultiSource_continuesOnError(t *testing.T) {
	errSrc := &mockBrowserSource{
		name:              "error",
		supportedBrowsers: []string{"chrome"},
		listErr:           errors.New("network error"),
	}
	goodSrc := &mockBrowserSource{
		name:              "good",
		supportedBrowsers: []string{"chrome"},
		versions: []VersionInfo{
			{Browser: "chrome", Version: "120.0.0.1", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}

	multi := NewMultiSource(errSrc, goodSrc)
	ctx := context.Background()

	versions, err := multi.List(ctx, &Filter{Browser: "chrome", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version from good source, got %d", len(versions))
	}
}

// TestChromeOmahaSource_SupportsBrowser verifies that ChromeOmahaSource
// supports both chrome and chromium.
func TestChromeOmahaSource_SupportsBrowser(t *testing.T) {
	src := NewChromeOmahaSource()

	if !src.SupportsBrowser("chrome") {
		t.Error("ChromeOmahaSource should support 'chrome'")
	}
	if !src.SupportsBrowser("chromium") {
		t.Error("ChromeOmahaSource should support 'chromium'")
	}
	if !src.SupportsBrowser("Chrome") {
		t.Error("ChromeOmahaSource should support 'Chrome' (case insensitive)")
	}
	if src.SupportsBrowser("firefox") {
		t.Error("ChromeOmahaSource should NOT support 'firefox'")
	}
	if src.SupportsBrowser("edge") {
		t.Error("ChromeOmahaSource should NOT support 'edge'")
	}
}

// TestChromeSource_SupportsBrowser verifies that ChromeSource supports
// both chrome and chromium.
func TestChromeSource_SupportsBrowser(t *testing.T) {
	src := NewChromeSource()

	if !src.SupportsBrowser("chrome") {
		t.Error("ChromeSource should support 'chrome'")
	}
	if !src.SupportsBrowser("chromium") {
		t.Error("ChromeSource should support 'chromium'")
	}
	if src.SupportsBrowser("firefox") {
		t.Error("ChromeSource should NOT support 'firefox'")
	}
}

// TestFirefoxSource_SupportsBrowser verifies that FirefoxSource only
// supports firefox.
func TestFirefoxSource_SupportsBrowser(t *testing.T) {
	src := NewFirefoxSource()

	if !src.SupportsBrowser("firefox") {
		t.Error("FirefoxSource should support 'firefox'")
	}
	if !src.SupportsBrowser("Firefox") {
		t.Error("FirefoxSource should support 'Firefox' (case insensitive)")
	}
	if src.SupportsBrowser("chrome") {
		t.Error("FirefoxSource should NOT support 'chrome'")
	}
	if src.SupportsBrowser("chromium") {
		t.Error("FirefoxSource should NOT support 'chromium'")
	}
}

// TestHTTPSource_SupportsBrowser verifies that HTTPSource supports all browsers.
func TestHTTPSource_SupportsBrowser(t *testing.T) {
	src := NewHTTPSource("http://example.com")

	if !src.SupportsBrowser("chrome") {
		t.Error("HTTPSource should support 'chrome'")
	}
	if !src.SupportsBrowser("firefox") {
		t.Error("HTTPSource should support 'firefox'")
	}
	if !src.SupportsBrowser("edge") {
		t.Error("HTTPSource should support 'edge'")
	}
}

// --- mockBrowserSource is a mock that supports browser filtering ---

type mockBrowserSource struct {
	name              string
	supportedBrowsers []string
	versions          []VersionInfo
	listErr           error
	queried           bool
}

func (m *mockBrowserSource) Name() string { return m.name }

func (m *mockBrowserSource) SupportsBrowser(browser string) bool {
	for _, b := range m.supportedBrowsers {
		if b == browser {
			return true
		}
	}
	return false
}

func (m *mockBrowserSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	m.queried = true
	if m.listErr != nil {
		return nil, m.listErr
	}
	if filter == nil {
		return m.versions, nil
	}
	var result []VersionInfo
	for _, v := range m.versions {
		if filter.Browser != "" && v.Browser != filter.Browser {
			continue
		}
		result = append(result, v)
	}
	return result, nil
}

func (m *mockBrowserSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := m.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, errors.New("no versions")
	}
	return versions[0], nil
}

func (m *mockBrowserSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	return VersionInfo{}, errors.New("not implemented")
}
