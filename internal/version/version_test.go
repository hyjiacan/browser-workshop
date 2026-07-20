package version

import (
	"reflect"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		wantErr  bool
	}{
		{"120.0.6099.109", []int{120, 0, 6099, 109}, false},
		{"121.0", []int{121, 0}, false},
		{"120", []int{120}, false},
		{"v120.0.6099.109", []int{120, 0, 6099, 109}, false},
		{"V121.0", []int{121, 0}, false},
		{"115.6.0esr", []int{115, 6, 0}, false},
		{"122.0.6261.9beta", []int{122, 0, 6261, 9}, false},
		{"", nil, true},
		{"abc", nil, true},
		{"12.34.xy", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMajor(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"120.0.6099.109", 120},
		{"121.0", 121},
		{"95.0.4638.69", 95},
		{"v115.6.0esr", 115},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Major(tt.input)
			if result != tt.expected {
				t.Errorf("Major(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b  string
		want  int
	}{
		{"120.0.6099.109", "120.0.6099.109", 0},
		{"121.0.6167.85", "120.0.6099.109", 1},
		{"120.0.6099.109", "121.0.6167.85", -1},
		{"120.0.6099.200", "120.0.6099.109", 1},
		{"120.1.0.0", "120.0.6099.109", 1},
		{"120.0", "120.0.0.0", 0},
		{"121", "120.999.999.999", 1},
		{"115.6.0esr", "115.5.0", 1},
		{"abc", "def", 0}, // both invalid, string compare
		{"123", "abc", 1}, // numeric > invalid
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := Compare(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.want)
			}
		})
	}
}

func TestLessGreaterEqual(t *testing.T) {
	if !Less("1.0", "2.0") {
		t.Error("Less(1.0, 2.0) should be true")
	}
	if Less("2.0", "1.0") {
		t.Error("Less(2.0, 1.0) should be false")
	}
	if !Greater("2.0", "1.0") {
		t.Error("Greater(2.0, 1.0) should be true")
	}
	if Greater("1.0", "2.0") {
		t.Error("Greater(1.0, 2.0) should be false")
	}
	if !Equal("1.0.0", "1.0") {
		t.Error("Equal(1.0.0, 1.0) should be true")
	}
	if Equal("1.0", "2.0") {
		t.Error("Equal(1.0, 2.0) should be false")
	}
}

func TestParseSpec(t *testing.T) {
	tests := []struct {
		input          string
		defaultBrowser string
		expected       Spec
	}{
		{
			input:          "chrome@120.0.6099.109",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chrome", Version: "120.0.6099.109", IsAlias: false},
		},
		{
			input:          "firefox@beta",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "firefox", Version: "beta", IsAlias: true},
		},
		{
			input:          "120",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chrome", Version: "120", IsAlias: false},
		},
		{
			input:          "v121.0",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chrome", Version: "v121.0", IsAlias: false},
		},
		{
			input:          "firefox",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "firefox", Version: "latest", IsAlias: true},
		},
		{
			input:          "",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chrome", Version: "latest", IsAlias: true},
		},
		{
			input:          "latest",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chrome", Version: "latest", IsAlias: true},
		},
		{
			input:          "chromium@latest",
			defaultBrowser: "chrome",
			expected:       Spec{Browser: "chromium", Version: "latest", IsAlias: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseSpec(tt.input, tt.defaultBrowser)
			if result.Browser != tt.expected.Browser {
				t.Errorf("Browser = %q, want %q", result.Browser, tt.expected.Browser)
			}
			if result.Version != tt.expected.Version {
				t.Errorf("Version = %q, want %q", result.Version, tt.expected.Version)
			}
			if result.IsAlias != tt.expected.IsAlias {
				t.Errorf("IsAlias = %v, want %v", result.IsAlias, tt.expected.IsAlias)
			}
		})
	}
}

func TestIsAlias(t *testing.T) {
	aliases := []string{"latest", "stable", "beta", "dev", "canary", "esr", "release", "nightly"}
	for _, a := range aliases {
		if !isAlias(a) {
			t.Errorf("isAlias(%q) should be true", a)
		}
		// case insensitive
		if !isAlias("BETA") {
			t.Error("isAlias('BETA') should be true (case insensitive)")
		}
	}

	if isAlias("120") {
		t.Error("isAlias('120') should be false")
	}
	if isAlias("chrome") {
		t.Error("isAlias('chrome') should be false")
	}
}

func TestListSort(t *testing.T) {
	list := List{
		{Version: "120.0.6099.109"},
		{Version: "121.0.6167.85"},
		{Version: "119.0.6045.199"},
	}

	// Descending (newest first)
	sorted := list.Sort(true)
	if sorted[0].Version != "121.0.6167.85" {
		t.Errorf("descending first = %q, want 121.0.6167.85", sorted[0].Version)
	}
	if sorted[2].Version != "119.0.6045.199" {
		t.Errorf("descending last = %q, want 119.0.6045.199", sorted[2].Version)
	}

	// Ascending (oldest first)
	sorted = list.Sort(false)
	if sorted[0].Version != "119.0.6045.199" {
		t.Errorf("ascending first = %q, want 119.0.6045.199", sorted[0].Version)
	}
}

func TestListFilter(t *testing.T) {
	list := List{
		{Browser: "chrome", Version: "120.0.6099.109", MajorVersion: 120, Channel: "stable", Platform: "windows", Arch: "amd64"},
		{Browser: "chrome", Version: "121.0.6167.85", MajorVersion: 121, Channel: "stable", Platform: "windows", Arch: "amd64"},
		{Browser: "firefox", Version: "121.0", MajorVersion: 121, Channel: "release", Platform: "windows", Arch: "amd64"},
		{Browser: "chrome", Version: "122.0.6261.9", MajorVersion: 122, Channel: "beta", Platform: "windows", Arch: "amd64"},
		{Browser: "chrome", Version: "120.0.6099.71", MajorVersion: 120, Channel: "stable", Platform: "darwin", Arch: "amd64"},
	}

	t.Run("filter by browser", func(t *testing.T) {
		filtered := list.Filter(Filter{Browser: "chrome"})
		if len(filtered) != 4 {
			t.Errorf("filtered by chrome = %d items, want 4", len(filtered))
		}
	})

	t.Run("filter by channel", func(t *testing.T) {
		filtered := list.Filter(Filter{Channel: "beta"})
		if len(filtered) != 1 {
			t.Errorf("filtered by beta = %d items, want 1", len(filtered))
		}
	})

	t.Run("filter by major version", func(t *testing.T) {
		filtered := list.Filter(Filter{Major: 120})
		if len(filtered) != 2 {
			t.Errorf("filtered by major 120 = %d items, want 2", len(filtered))
		}
	})

	t.Run("filter by platform", func(t *testing.T) {
		filtered := list.Filter(Filter{Platform: "darwin"})
		if len(filtered) != 1 {
			t.Errorf("filtered by darwin = %d items, want 1", len(filtered))
		}
	})

	t.Run("filter with query", func(t *testing.T) {
		filtered := list.Filter(Filter{Query: "120"})
		if len(filtered) != 2 {
			t.Errorf("filtered by query '120' = %d items, want 2", len(filtered))
		}
	})

	t.Run("filter with limit", func(t *testing.T) {
		filtered := list.Filter(Filter{Limit: 2})
		if len(filtered) != 2 {
			t.Errorf("filtered with limit 2 = %d items, want 2", len(filtered))
		}
	})

	t.Run("combined filter", func(t *testing.T) {
		filtered := list.Filter(Filter{Browser: "chrome", Major: 120, Platform: "windows"})
		if len(filtered) != 1 {
			t.Errorf("combined filter = %d items, want 1", len(filtered))
		}
	})
}

func TestListLatest(t *testing.T) {
	list := List{
		{Version: "120.0.6099.109"},
		{Version: "121.0.6167.85"},
		{Version: "119.0.6045.199"},
	}

	latest, ok := list.Latest()
	if !ok {
		t.Fatal("Latest() returned false")
	}
	if latest.Version != "121.0.6167.85" {
		t.Errorf("Latest() = %q, want 121.0.6167.85", latest.Version)
	}

	// Empty list
	var empty List
	_, ok = empty.Latest()
	if ok {
		t.Error("Latest() on empty list should return false")
	}
}

func TestListLatestByMajor(t *testing.T) {
	list := List{
		{Version: "120.0.6099.71", MajorVersion: 120},
		{Version: "120.0.6099.109", MajorVersion: 120},
		{Version: "121.0.6167.85", MajorVersion: 121},
		{Version: "121.0.6167.140", MajorVersion: 121},
		{Version: "119.0.6045.199", MajorVersion: 119},
	}

	result := list.LatestByMajor()

	if result[120].Version != "120.0.6099.109" {
		t.Errorf("latest major 120 = %q, want 120.0.6099.109", result[120].Version)
	}
	if result[121].Version != "121.0.6167.140" {
		t.Errorf("latest major 121 = %q, want 121.0.6167.140", result[121].Version)
	}
	if len(result) != 3 {
		t.Errorf("LatestByMajor() = %d entries, want 3", len(result))
	}
}

func TestListFind(t *testing.T) {
	list := List{
		{Version: "120.0.6099.109"},
		{Version: "121.0.6167.85"},
	}

	v, ok := list.Find("120.0.6099.109")
	if !ok {
		t.Fatal("Find() returned false")
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("Find() = %q", v.Version)
	}

	_, ok = list.Find("999.0.0.0")
	if ok {
		t.Error("Find() for non-existent should return false")
	}
}

func TestListFindByMajor(t *testing.T) {
	list := List{
		{Version: "120.0.6099.71", MajorVersion: 120},
		{Version: "120.0.6099.109", MajorVersion: 120},
		{Version: "121.0.6167.85", MajorVersion: 121},
	}

	v, ok := list.FindByMajor(120)
	if !ok {
		t.Fatal("FindByMajor(120) returned false")
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("FindByMajor(120) = %q, want latest 120.x", v.Version)
	}

	_, ok = list.FindByMajor(999)
	if ok {
		t.Error("FindByMajor(999) should return false")
	}
}

func TestListBrowsers(t *testing.T) {
	list := List{
		{Browser: "chrome"},
		{Browser: "firefox"},
		{Browser: "chrome"},
		{Browser: "chromium"},
	}

	browsers := list.Browsers()
	if len(browsers) != 3 {
		t.Errorf("Browsers() = %d items, want 3", len(browsers))
	}
}

func TestInstallRecordToVersion(t *testing.T) {
	record := &InstallRecord{
		Browser:     "chrome",
		Version:     "120.0.6099.109",
		InstalledAt: time.Now(),
		Platform:    "windows",
		Arch:        "amd64",
		Size:        123456789,
		Source:      "local-repo",
	}

	v := record.ToVersion()
	if v.Browser != "chrome" {
		t.Errorf("Browser = %q", v.Browser)
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", v.Version)
	}
	if v.MajorVersion != 120 {
		t.Errorf("MajorVersion = %d, want 120", v.MajorVersion)
	}
	if v.Source != "local-repo" {
		t.Errorf("Source = %q", v.Source)
	}
}

func TestMatchesQuery(t *testing.T) {
	v := Version{
		Browser: "chrome",
		Version: "120.0.6099.109",
		Channel: "stable",
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"120", true},
		{"chrome", true},
		{"stable", true},
		{"6099", true},
		{"firefox", false},
		{"xyz", false},
	}

	for _, tt := range tests {
		if got := matchesQuery(v, tt.query); got != tt.want {
			t.Errorf("matchesQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}
