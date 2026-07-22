// Package version provides version model, parsing, comparison, and filtering.
package version

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version represents a browser version with all its metadata.
type Version struct {
	Browser      string            `json:"browser"`
	Version      string            `json:"version"`
	MajorVersion int               `json:"majorVersion"`
	Channel      string            `json:"channel,omitempty"`
	ReleaseDate  string            `json:"releaseDate,omitempty"` // ISO date string
	Platform     string            `json:"platform,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Downloads    map[string]DownloadInfo `json:"downloads,omitempty"` // key: platform_arch
	Source       string            `json:"source,omitempty"`
	IsSystem     bool              `json:"isSystem,omitempty"` // true for system-installed browsers
}

// DownloadInfo holds download metadata for a specific platform/arch.
type DownloadInfo struct {
	URL    string `json:"url"`
	Size   int64  `json:"size,omitempty"` // in bytes
	SHA256 string `json:"sha256,omitempty"`
	Format string `json:"format,omitempty"` // zip, tar.bz2, exe, msi, dmg
}

// Filter is used to query and filter version lists.
type Filter struct {
	Browser  string // empty = all browsers
	Channel  string // empty = all channels
	Platform string // empty = current platform
	Arch     string // empty = current arch
	Major    int    // 0 = all major versions
	Query    string // search keyword
	Limit    int    // 0 = unlimited
}

// List is a slice of Version with helper methods.
type List []Version

// --- Parsing ---

// Parse parses a version string like "120.0.6099.109" into its numeric segments.
// Returns the segments as []int.
func Parse(version string) ([]int, error) {
	// Strip any channel suffix like "esr", "beta", etc.
	clean := strings.TrimSpace(version)
	clean = strings.TrimSuffix(clean, "esr")
	clean = strings.TrimSuffix(clean, "beta")
	clean = strings.TrimSuffix(clean, "dev")
	clean = strings.TrimSuffix(clean, "canary")
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimPrefix(clean, "V")

	if clean == "" {
		return nil, errors.New("empty version string")
	}

	parts := strings.Split(clean, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid version: %s", version)
	}

	segments := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid version segment %q: %w", p, err)
		}
		segments[i] = n
	}

	return segments, nil
}

// Major extracts the major version number from a version string.
// Returns 0 if parsing fails.
func Major(version string) int {
	segments, err := Parse(version)
	if err != nil || len(segments) == 0 {
		return 0
	}
	return segments[0]
}

// --- Comparison ---

// Compare compares two version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func Compare(a, b string) int {
	segA, errA := Parse(a)
	segB, errB := Parse(b)

	if errA != nil && errB != nil {
		// Both are non-numeric versions; treat them as equal
		return 0
	}
	if errA != nil {
		return -1
	}
	if errB != nil {
		return 1
	}

	maxLen := len(segA)
	if len(segB) > maxLen {
		maxLen = len(segB)
	}

	for i := 0; i < maxLen; i++ {
		var aVal, bVal int
		if i < len(segA) {
			aVal = segA[i]
		}
		if i < len(segB) {
			bVal = segB[i]
		}
		if aVal < bVal {
			return -1
		}
		if aVal > bVal {
			return 1
		}
	}

	return 0
}

// Less returns true if a < b.
func Less(a, b string) bool {
	return Compare(a, b) < 0
}

// Greater returns true if a > b.
func Greater(a, b string) bool {
	return Compare(a, b) > 0
}

// Equal returns true if a == b.
func Equal(a, b string) bool {
	return Compare(a, b) == 0
}

// --- Version Spec Parsing ---

// Spec represents a parsed version specification from user input.
// e.g. "chrome@120", "firefox@latest", "120.0.6099.109"
type Spec struct {
	Browser string // empty if not specified
	Version string // raw version string, could be "120", "latest", "120.0.6099.109"
	IsAlias bool   // true if version is an alias like "latest", "stable", "beta"
}

// ParseSpec parses a user-provided version specification.
// Supported formats:
//   - "browser@version"  → browser + version
//   - "browser"          → browser + default/latest
//   - "version"          → default browser + version
func ParseSpec(input string, defaultBrowser string) Spec {
	input = strings.TrimSpace(input)

	if input == "" {
		return Spec{Browser: defaultBrowser, Version: "latest", IsAlias: true}
	}

	// Check for browser@version format
	if idx := strings.Index(input, "@"); idx > 0 {
		browser := strings.TrimSpace(input[:idx])
		ver := strings.TrimSpace(input[idx+1:])
		alias := isAlias(ver)
		return Spec{Browser: browser, Version: ver, IsAlias: alias}
	}

	// Check if input looks like a version (starts with digit or v+digit)
	if looksLikeVersion(input) {
		return Spec{Browser: defaultBrowser, Version: input, IsAlias: false}
	}

	// Check if input is a version alias (e.g. "latest", "beta")
	if isAlias(input) {
		return Spec{Browser: defaultBrowser, Version: input, IsAlias: true}
	}

	// Otherwise, treat as browser name
	return Spec{Browser: input, Version: "latest", IsAlias: true}
}

// isAlias checks if a version string is a known alias.
var aliasSet = map[string]bool{
	"latest":  true,
	"stable":  true,
	"beta":    true,
	"dev":     true,
	"canary":  true,
	"esr":     true,
	"release": true,
	"nightly": true,
}

func isAlias(v string) bool {
	return aliasSet[strings.ToLower(v)]
}

// IsAlias reports whether v is a known version alias (e.g. "latest", "stable").
func IsAlias(v string) bool {
	return isAlias(v)
}

// looksLikeVersion checks if a string looks like a version number.
func looksLikeVersion(s string) bool {
	if len(s) == 0 {
		return false
	}
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if len(s) == 0 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// --- List Operations ---

// Sort sorts the version list by version number (descending = newest first).
func (l List) Sort(descending bool) List {
	result := make(List, len(l))
	copy(result, l)

	sort.Slice(result, func(i, j int) bool {
		cmp := Compare(result[i].Version, result[j].Version)
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})

	return result
}

// Filter returns versions matching the given filter criteria.
func (l List) Filter(f Filter) List {
	var result List

	for _, v := range l {
		if f.Browser != "" && v.Browser != f.Browser {
			continue
		}
		if f.Channel != "" && v.Channel != f.Channel {
			continue
		}
		if f.Platform != "" && v.Platform != "" && v.Platform != f.Platform {
			continue
		}
		if f.Arch != "" && v.Arch != "" && v.Arch != f.Arch {
			continue
		}
		if f.Major > 0 && v.MajorVersion != f.Major {
			continue
		}
		if f.Query != "" && !matchesQuery(v, f.Query) {
			continue
		}
		result = append(result, v)
	}

	if f.Limit > 0 && len(result) > f.Limit {
		result = result[:f.Limit]
	}

	return result
}

// matchesQuery checks if a version matches a search query.
func matchesQuery(v Version, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(v.Version), q) {
		return true
	}
	if strings.Contains(strings.ToLower(v.Channel), q) {
		return true
	}
	if strings.Contains(strings.ToLower(v.Browser), q) {
		return true
	}
	return false
}

// Latest returns the latest (highest version number) version from the list.
func (l List) Latest() (Version, bool) {
	if len(l) == 0 {
		return Version{}, false
	}

	best := l[0]
	for _, v := range l[1:] {
		if Greater(v.Version, best.Version) {
			best = v
		}
	}
	return best, true
}

// LatestByMajor returns the latest version for each major version.
func (l List) LatestByMajor() map[int]Version {
	result := make(map[int]Version)
	for _, v := range l {
		major := v.MajorVersion
		if major == 0 {
			major = Major(v.Version)
		}
		existing, ok := result[major]
		if !ok || Greater(v.Version, existing.Version) {
			result[major] = v
		}
	}
	return result
}

// Find finds a version by exact version string.
func (l List) Find(version string) (Version, bool) {
	for _, v := range l {
		if Equal(v.Version, version) {
			return v, true
		}
	}
	return Version{}, false
}

// FindByMajor finds the latest version matching the given major version.
func (l List) FindByMajor(major int) (Version, bool) {
	filtered := l.Filter(Filter{Major: major})
	return filtered.Latest()
}

// Browsers returns the unique browser names in the list.
func (l List) Browsers() []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range l {
		if !seen[v.Browser] {
			seen[v.Browser] = true
			result = append(result, v.Browser)
		}
	}
	return result
}

// --- InstallRecord (shared model) ---

// InstallRecord represents an installed browser version.
type InstallRecord struct {
	Browser        string    `json:"browser"`
	Version        string    `json:"version"`
	InstalledAt    time.Time `json:"installedAt"`
	Platform       string    `json:"platform"`
	Arch           string    `json:"arch"`
	InstallDir     string    `json:"installDir"`
	ExecutablePath string    `json:"executablePath"` // relative to InstallDir
	Size           int64     `json:"size"` // in bytes
	Source         string    `json:"source"`
	IsSystem       bool      `json:"isSystem,omitempty"` // true for system browsers
	Channel        string    `json:"channel,omitempty"`  // release channel
}

// ToVersion converts an InstallRecord to a Version for list display.
func (r *InstallRecord) ToVersion() Version {
	return Version{
		Browser:      r.Browser,
		Version:      r.Version,
		MajorVersion: Major(r.Version),
		Channel:      r.Channel,
		Platform:     r.Platform,
		Arch:         r.Arch,
		Source:       r.Source,
		IsSystem:     r.IsSystem,
	}
}
