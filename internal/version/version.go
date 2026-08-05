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

// ClientVersion is the version of the bws client binary.
// It is set from main.go (or via ldflags at build time).
var ClientVersion = "0.0.0-dev"

// Version represents a browser version with all its metadata.
type Version struct {
	Browser      string            `json:"browser"`
	Version      string            `json:"version"`
	MajorVersion int               `json:"majorVersion"`
	Channel      string            `json:"channel,omitempty"`
	ReleaseDate  string            `json:"releaseDate,omitempty"` // ISO date string
	Platform     string            `json:"platform,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Source       string            `json:"source,omitempty"`
	IsSystem     bool              `json:"isSystem,omitempty"` // true for system-installed browsers
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

// stripBetaSuffix removes the Firefox beta numeric suffix from a version string.
// "141.0b1" → "141.0", "141.0.2b3" → "141.0.2", "141.0" → "141.0" (unchanged).
func stripBetaSuffix(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == 'b' && i > 0 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			return s[:i]
		}
	}
	return s
}

// Parse parses a version string like "120.0.6099.109" into its numeric segments.
// Returns the segments as []int.
func Parse(version string) ([]int, error) {
	// Strip any channel suffix like "esr", "beta", etc.
	clean := strings.TrimSpace(version)
	clean = strings.TrimSuffix(clean, "esr")
	clean = strings.TrimSuffix(clean, "beta")
	clean = strings.TrimSuffix(clean, "dev")
	clean = strings.TrimSuffix(clean, "canary")
	// Strip Firefox beta suffix: "141.0b1" → "141.0"
	clean = stripBetaSuffix(clean)
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimPrefix(clean, "V")

	if clean == "" {
		return nil, errors.New("empty version string")
	}

	parts := strings.Split(clean, ".")

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

// stripGitHash removes a trailing git describe hash suffix ("-g<hex>").
// e.g. "1.0.0-beta-45-gec99964" → "1.0.0-beta-45"
func stripGitHash(s string) string {
	idx := strings.LastIndex(s, "-g")
	if idx < 0 {
		return s
	}
	hexPart := s[idx+2:]
	if len(hexPart) < 6 {
		return s
	}
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return s
		}
	}
	return s[:idx]
}

// splitCorePre splits a version like "1.0.0-beta-45" into core "1.0.0" and prerelease "beta-45".
func splitCorePre(v string) (core, pre string) {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	v = stripGitHash(v)
	if idx := strings.Index(v, "-"); idx >= 0 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

// comparePreRelease compares two prerelease strings using semver-like rules.
// - A version with prerelease is lower than one without.
// - Prerelease identifiers are split by "-" or ".".
// - Numeric identifiers are compared numerically, strings lexicographically.
// - Shorter prerelease (fewer identifiers) is considered earlier.
func comparePreRelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1 // no prerelease > with prerelease
	}
	if b == "" {
		return -1
	}

	partsA := strings.FieldsFunc(a, func(r rune) bool { return r == '-' || r == '.' })
	partsB := strings.FieldsFunc(b, func(r rune) bool { return r == '-' || r == '.' })

	n := len(partsA)
	if len(partsB) < n {
		n = len(partsB)
	}

	for i := 0; i < n; i++ {
		numA, errA := strconv.Atoi(partsA[i])
		numB, errB := strconv.Atoi(partsB[i])
		if errA == nil && errB == nil {
			if numA != numB {
				if numA < numB {
					return -1
				}
				return 1
			}
		} else {
			if partsA[i] != partsB[i] {
				if partsA[i] < partsB[i] {
					return -1
				}
				return 1
			}
		}
	}

	if len(partsA) < len(partsB) {
		return -1
	}
	if len(partsA) > len(partsB) {
		return 1
	}
	return 0
}

// Compare compares two version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
//
// Supports semver-like pre-release versions:
//
//	1.0.0 < 1.0.0-beta       (release > pre-release)
//	1.0.0-beta < 1.0.0-beta-45  (higher number = newer)
//	1.0.0-beta-45-rec99964  → git hash stripped automatically
func Compare(a, b string) int {
	coreA, preA := splitCorePre(a)
	coreB, preB := splitCorePre(b)

	segA, errA := Parse(coreA)
	segB, errB := Parse(coreB)

	if errA != nil && errB != nil {
		// Both cores are non-numeric; treat as equal
		return 0
	}
	if errA != nil {
		return -1
	}
	if errB != nil {
		return 1
	}

	// Compare numeric core segments
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

	// Same core; compare prerelease
	return comparePreRelease(preA, preB)
}

// Greater returns true if a > b.
func Greater(a, b string) bool {
	return Compare(a, b) > 0
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
