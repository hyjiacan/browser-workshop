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

func TestGreater(t *testing.T) {
	if !Greater("2.0", "1.0") {
		t.Error("Greater(2.0, 1.0) should be true")
	}
	if Greater("1.0", "2.0") {
		t.Error("Greater(1.0, 2.0) should be false")
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
