package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChromeSource_List(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/all.json" {
			data := []omahaEntry{
				{
					OS: "win64",
					Versions: []omahaVersion{
						{Channel: "stable", Version: "120.0.6099.109"},
						{Channel: "beta", Version: "121.0.6167.57"},
						{Channel: "dev", Version: "122.0.6238.12"},
						{Channel: "canary", Version: "123.0.6294.0"},
					},
				},
				{
					OS: "mac",
					Versions: []omahaVersion{
						{Channel: "stable", Version: "120.0.6099.109"},
						{Channel: "beta", Version: "121.0.6167.57"},
					},
				},
			}
			json.NewEncoder(w).Encode(data)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	src := &ChromeSource{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	// Test listing all versions for current platform
	ctx := context.Background()
	versions, err := src.List(ctx, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have at least some versions
	if len(versions) == 0 {
		t.Fatal("expected at least some versions")
	}

	// All should be chrome
	for _, v := range versions {
		if v.Browser != "chrome" {
			t.Errorf("expected browser chrome, got %s", v.Browser)
		}
	}
}

func TestChromeSource_Latest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []omahaEntry{
			{
				OS: "win64",
				Versions: []omahaVersion{
					{Channel: "stable", Version: "120.0.6099.109"},
					{Channel: "beta", Version: "121.0.6167.57"},
				},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	src := &ChromeSource{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// Test latest stable
	latest, err := src.Latest(ctx, &Filter{Channel: ChannelStable})
	if err != nil {
		t.Fatalf("Latest stable failed: %v", err)
	}
	if latest.Version != "120.0.6099.109" {
		t.Errorf("latest stable version = %s, want 120.0.6099.109", latest.Version)
	}
	if latest.Channel != ChannelStable {
		t.Errorf("latest stable channel = %s, want stable", latest.Channel)
	}

	// Test latest beta
	latestBeta, err := src.Latest(ctx, &Filter{Channel: ChannelBeta})
	if err != nil {
		t.Fatalf("Latest beta failed: %v", err)
	}
	if latestBeta.Version != "121.0.6167.57" {
		t.Errorf("latest beta version = %s, want 121.0.6167.57", latestBeta.Version)
	}
}

func TestChromeSource_Resolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []omahaEntry{
			{
				OS: "win64",
				Versions: []omahaVersion{
					{Channel: "stable", Version: "120.0.6099.109"},
					{Channel: "stable", Version: "119.0.6045.199"},
					{Channel: "beta", Version: "121.0.6167.57"},
				},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	src := &ChromeSource{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// Test exact version
	v, err := src.Resolve(ctx, "chrome", "120.0.6099.109", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("Resolve exact version failed: %v", err)
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("resolved version = %s, want 120.0.6099.109", v.Version)
	}

	// Test "latest" alias
	v, err = src.Resolve(ctx, "chrome", "latest", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("Resolve latest failed: %v", err)
	}
	if v.Channel != ChannelStable {
		t.Errorf("latest channel = %s, want stable", v.Channel)
	}

	// Test partial version (should return latest matching)
	v, err = src.Resolve(ctx, "chrome", "119", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("Resolve partial version failed: %v", err)
	}
	if v.Version != "119.0.6045.199" {
		t.Errorf("partial version resolved to %s, want 119.0.6045.199", v.Version)
	}

	// Test wrong browser
	_, err = src.Resolve(ctx, "firefox", "120.0", PlatformWindows, ArchAMD64)
	if err == nil {
		t.Error("expected error for non-chrome browser")
	}
}

func TestMultiSource(t *testing.T) {
	// Create two mock sources
	source1 := &mockSource{
		name: "source1",
		versions: []VersionInfo{
			{Browser: "chrome", Version: "120.0.6099.109", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			{Browser: "chrome", Version: "119.0.6045.199", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}
	source2 := &mockSource{
		name: "source2",
		versions: []VersionInfo{
			{Browser: "firefox", Version: "121.0", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
			// Duplicate of source1's chrome 120 (should be deduplicated)
			{Browser: "chrome", Version: "120.0.6099.109", Channel: ChannelStable, Platform: PlatformWindows, Arch: ArchAMD64},
		},
	}

	multi := NewMultiSource(source1, source2)

	if multi.Name() == "" {
		t.Error("MultiSource.Name() returned empty string")
	}

	ctx := context.Background()
	versions, err := multi.List(ctx, &Filter{Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("MultiSource.List failed: %v", err)
	}

	// Should have 3 unique versions (2 chrome + 1 firefox, duplicate removed)
	if len(versions) != 3 {
		t.Errorf("expected 3 unique versions, got %d", len(versions))
	}

	// Test Latest
	latest, err := multi.Latest(ctx, &Filter{Browser: "chrome", Platform: PlatformWindows, Arch: ArchAMD64})
	if err != nil {
		t.Fatalf("MultiSource.Latest failed: %v", err)
	}
	if latest.Version != "120.0.6099.109" {
		t.Errorf("latest version = %s, want 120.0.6099.109", latest.Version)
	}
}

// mockSource is a mock implementation of Source for testing.
type mockSource struct {
	name     string
	versions []VersionInfo
}

func (m *mockSource) Name() string { return m.name }

func (m *mockSource) SupportsBrowser(browser string) bool { return true }

func (m *mockSource) List(ctx context.Context, filter *Filter) ([]VersionInfo, error) {
	filter = applyDefaults(filter)
	var result []VersionInfo
	for _, v := range m.versions {
		if filter.Browser != "" && v.Browser != filter.Browser {
			continue
		}
		if filter.Channel != "" && v.Channel != filter.Channel {
			continue
		}
		if filter.Platform != "" && v.Platform != filter.Platform {
			continue
		}
		if filter.Arch != "" && v.Arch != filter.Arch {
			continue
		}
		result = append(result, v)
	}
	return result, nil
}

func (m *mockSource) Latest(ctx context.Context, filter *Filter) (VersionInfo, error) {
	versions, err := m.List(ctx, filter)
	if err != nil {
		return VersionInfo{}, err
	}
	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions found")
	}
	return versions[0], nil
}

func (m *mockSource) Resolve(ctx context.Context, browser string, version string, platform Platform, arch Arch) (VersionInfo, error) {
	for _, v := range m.versions {
		if v.Browser == browser && v.Version == version && v.Platform == platform && v.Arch == arch {
			return v, nil
		}
	}
	return VersionInfo{}, fmt.Errorf("not found")
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"120.0.6099.109", "119.0.6045.199", 1},
		{"119.0.6045.199", "120.0.6099.109", -1},
		{"120.0.6099.109", "120.0.6099.109", 0},
		{"120.0.6099.109", "120.0.6099.110", -1},
		{"120.0", "120.0.6099.109", -1},
		{"121", "120.999.999.999", 1},
	}

	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseChannel(t *testing.T) {
	tests := []struct {
		input string
		want  Channel
	}{
		{"stable", ChannelStable},
		{"beta", ChannelBeta},
		{"dev", ChannelDev},
		{"canary", ChannelCanary},
		{"esr", ChannelESR},
		{"STABLE", ChannelStable},
		{"Beta", ChannelBeta},
		{"unknown", ChannelUnknown},
		{"", ChannelUnknown},
	}

	for _, tt := range tests {
		got := ParseChannel(tt.input)
		if got != tt.want {
			t.Errorf("ParseChannel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapOmahaPlatform(t *testing.T) {
	tests := []struct {
		input string
		want  Platform
	}{
		{"win64", PlatformWindows},
		{"win", PlatformWindows},
		{"windows", PlatformWindows},
		{"mac", PlatformMacOS},
		{"macos", PlatformMacOS},
		{"linux", PlatformLinux},
		{"linux64", PlatformLinux},
		{"unknown", PlatformUnknown},
	}

	for _, tt := range tests {
		got := mapOmahaPlatform(tt.input)
		if got != tt.want {
			t.Errorf("mapOmahaPlatform(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	// Nil filter should get defaults
	f := applyDefaults(nil)
	if f.Platform == "" {
		t.Error("applyDefaults(nil) should set default platform")
	}
	if f.Arch == "" {
		t.Error("applyDefaults(nil) should set default arch")
	}

	// Partial filter should keep specified values
	f = applyDefaults(&Filter{Browser: "chrome"})
	if f.Browser != "chrome" {
		t.Errorf("browser should be preserved, got %s", f.Browser)
	}
}
