// Package repo provides local binary repository scanning and importing.
package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
)

// MatchResult represents a matched entry in a local repository.
type MatchResult struct {
	// Path is the full path to the matched directory or file
	Path string

	// DirName is the name of the matched directory (or full file name for files)
	DirName string

	// FileName is the name of the file without extension, used for pattern matching.
	// Empty for directories.
	FileName string

	// IsFile indicates whether the match is a file (true) or a directory (false)
	IsFile bool

	// Browser is the detected browser name
	Browser string

	// Version is the detected version string
	Version string

	// Arch is the detected architecture (may be empty)
	Arch string

	// Channel is the detected channel (may be empty)
	Channel string

	// Platform is the detected platform (may be empty)
	Platform string

	// Pattern describes what matched (for debugging)
	Pattern string

	// Status indicates the match quality
	Status MatchStatus

	// Detail provides additional info about the match
	Detail string
}

// MatchStatus indicates the quality of a match.
type MatchStatus int

const (
	// MatchOK means full match with all info
	MatchOK MatchStatus = iota

	// MatchPartial means some info is missing (e.g. no arch)
	MatchPartial

	// MatchUnrecognized means the directory couldn't be identified
	MatchUnrecognized

	// MatchNoExecutable means matched but no executable found
	MatchNoExecutable
)

func (s MatchStatus) String() string {
	switch s {
	case MatchOK:
		return "ok"
	case MatchPartial:
		return "partial"
	case MatchUnrecognized:
		return "unrecognized"
	case MatchNoExecutable:
		return "no-executable"
	default:
		return "unknown"
	}
}

// Scanner scans a local repository directory and identifies browser versions.
type Scanner struct {
	path     string
	browsers *browser.Registry
}

// ScannerOptions configures a scanner.
type ScannerOptions struct {
	// Reserved for future use
}

// NewScanner creates a new repository scanner for the given path.
func NewScanner(path string, br *browser.Registry, opts ...ScannerOptions) (*Scanner, error) {
	if br == nil {
		return nil, errors.New("browser registry is required")
	}

	return &Scanner{
		path:     path,
		browsers: br,
	}, nil
}

// Path returns the repository path this scanner is configured for.
func (s *Scanner) Path() string {
	return s.path
}

// --- Browser keyword detection ---

// browserKeywords maps browser names to their keyword variations.
// Order matters: more specific keywords come first.
var browserKeywords = []struct {
	name     string
	keywords []string
}{
	{"chrome", []string{"googlechrome", "google-chrome", "google chrome", "chrome"}},
	{"chromium", []string{"googlechromium", "google-chromium", "chromium"}},
	{"firefox", []string{"mozillafirefox", "mozilla-firefox", "mozilla firefox", "firefox"}},
	{"edge", []string{"microsoftedge", "microsoft-edge", "msedge", "edge"}},
	{"brave", []string{"brave-browser", "brave browser", "brave"}},
	{"opera", []string{"opera"}},
}

// detectBrowser detects the browser name from a string.
// Returns the canonical browser name and the matched keyword.
func detectBrowser(name string) (string, string) {
	lower := strings.ToLower(name)

	for _, b := range browserKeywords {
		for _, kw := range b.keywords {
			if strings.Contains(lower, kw) {
				return b.name, kw
			}
		}
	}

	return "", ""
}

// --- Version detection ---

// versionRegex matches version numbers like 120.0.6099.109, 120.0, 115.6.0esr, etc.
var versionRegex = regexp.MustCompile(`(\d+\.\d+(?:\.\d+){0,2}(?:[a-zA-Z]+)?)`)

// detectVersion extracts a version number from a string.
func detectVersion(name string) string {
	matches := versionRegex.FindAllString(name, -1)
	if len(matches) == 0 {
		return ""
	}

	// Prefer the longest match (most specific version number)
	best := matches[0]
	for _, m := range matches[1:] {
		if len(m) > len(best) {
			best = m
		}
	}

	return best
}

// --- Platform detection ---

var platformKeywords = []struct {
	platform string
	keywords []string
}{
	{"windows", []string{"windows", "win64", "win32", "win"}},
	{"darwin", []string{"macos", "mac-os", "mac os", "mac64", "macarm64", "mac"}},
	{"linux", []string{"linux64", "linux"}},
}

func detectPlatform(name string) string {
	lower := strings.ToLower(name)

	for _, p := range platformKeywords {
		for _, kw := range p.keywords {
			if strings.Contains(lower, kw) {
				return p.platform
			}
		}
	}

	return ""
}

// --- Architecture detection ---

var archKeywords = []struct {
	arch     string
	keywords []string
}{
	{"arm64", []string{"arm64", "aarch64", "macarm64"}},
	{"amd64", []string{"x86_64", "x64", "amd64", "win64", "64", "chrome64", "firefox64", "mac64", "linux64"}},
	{"386", []string{"x86", "win32", "386", "i386", "32", "chrome32", "firefox32"}},
}

func detectArch(name string) string {
	lower := strings.ToLower(name)

	for _, a := range archKeywords {
		for _, kw := range a.keywords {
			if strings.Contains(lower, kw) {
				return a.arch
			}
		}
	}

	return ""
}

// --- Channel detection ---

var channelKeywords = []struct {
	channel  string
	keywords []string
}{
	{"canary", []string{"canary"}},
	{"dev", []string{"dev", "developer"}},
	{"beta", []string{"beta"}},
	{"esr", []string{"esr"}},
	{"stable", []string{"stable", "official", "release"}},
}

func detectChannel(name string) string {
	lower := strings.ToLower(name)

	for _, c := range channelKeywords {
		for _, kw := range c.keywords {
			if strings.Contains(lower, kw) {
				return c.channel
			}
		}
	}

	return ""
}

// --- Scanner core logic ---

// ScanEntry scans a single entry name (file or directory).
// entryName is the name to match against (for files, this should be the name without extension).
// isFile indicates whether the entry is a file.
// fullName is the original full name (with extension for files).
// Returns the match result, never nil.
func (s *Scanner) ScanEntry(entryName string, fullName string, isFile bool, defaultBrowser string, defaultArch string) *MatchResult {
	result := &MatchResult{
		DirName: fullName,
		IsFile:  isFile,
		Status:  MatchUnrecognized,
		Detail:  "no browser name detected",
	}

	if isFile {
		result.FileName = entryName
	}

	// Step 1: Detect browser
	browserName, matchedKw := detectBrowser(entryName)
	if browserName == "" {
		// Try with default browser hint
		if defaultBrowser != "" {
			// Check if there's a version number at least
			ver := detectVersion(entryName)
			if ver != "" {
				result.Browser = defaultBrowser
				result.Version = ver
				result.Status = MatchPartial
				result.Detail = "browser inferred from default"
				result.Pattern = "default-browser + version"
				fillDefaults(result, defaultBrowser, defaultArch)
				return result
			}
		}
		return result
	}

	result.Browser = browserName
	result.Pattern = fmt.Sprintf("keyword:%s", matchedKw)

	// Step 2: Detect version
	version := detectVersion(entryName)
	if version == "" {
		result.Status = MatchUnrecognized
		result.Detail = "no version number detected"
		return result
	}
	result.Version = version

	// Step 3: Detect platform
	platform := detectPlatform(entryName)
	if platform != "" {
		result.Platform = platform
	}

	// Step 4: Detect architecture
	arch := detectArch(entryName)
	if arch != "" {
		result.Arch = arch
	}

	// Step 5: Detect channel
	channel := detectChannel(entryName)
	if channel != "" {
		result.Channel = channel
	}

	// Step 6: Determine match quality
	if result.Arch != "" && result.Platform != "" {
		result.Status = MatchOK
	} else {
		result.Status = MatchPartial
		var missing []string
		if result.Arch == "" {
			missing = append(missing, "arch")
		}
		if result.Platform == "" {
			missing = append(missing, "platform")
		}
		result.Detail = fmt.Sprintf("missing: %s", strings.Join(missing, ", "))
	}

	// Apply defaults for missing fields
	fillDefaults(result, defaultBrowser, defaultArch)

	return result
}

func fillDefaults(result *MatchResult, defaultBrowser string, defaultArch string) {
	// Apply default arch if missing and default provided
	if result.Arch == "" && defaultArch != "" {
		if result.Status == MatchOK {
			result.Status = MatchPartial
			if result.Detail == "" {
				result.Detail = "arch inferred from default"
			}
		}
		result.Arch = defaultArch
	}

	// Apply default platform if missing (assume current platform)
	if result.Platform == "" {
		result.Platform = paths.Platform()
	}
}

// ScanDir scans a single directory name against all patterns.
// Deprecated: Use ScanEntry instead.
func (s *Scanner) ScanDir(dirName string, defaultBrowser string, defaultArch string) *MatchResult {
	return s.ScanEntry(dirName, dirName, false, defaultBrowser, defaultArch)
}

// Scan scans the configured repository directory and returns all matches.
func (s *Scanner) Scan() ([]MatchResult, error) {
	return s.ScanRepository(s.path, "", "")
}

// ScanRepository scans an entire repository directory and returns all matches.
func (s *Scanner) ScanRepository(repoPath string, defaultBrowser string, defaultArch string) ([]MatchResult, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, fmt.Errorf("reading repository directory: %w", err)
	}

	var results []MatchResult

	for _, entry := range entries {
		fullPath := joinPath(repoPath, entry.Name())

		var match *MatchResult

		if entry.IsDir() {
			// Directory: use the directory name directly for matching
			match = s.ScanEntry(entry.Name(), entry.Name(), false, defaultBrowser, defaultArch)
			match.Path = fullPath

			// If matched, check for executable
			if match.Status == MatchOK || match.Status == MatchPartial {
				if match.Browser != "" {
					_, err := s.browsers.FindExecutable(match.Browser, fullPath, paths.Platform(), paths.Arch())
					if err != nil {
						match.Status = MatchNoExecutable
						match.Detail = "executable not found"
					}
				}
			}
		} else {
			// File: strip extension before matching
			fileName := stripExtension(entry.Name())
			match = s.ScanEntry(fileName, entry.Name(), true, defaultBrowser, defaultArch)
			match.Path = fullPath
			// For installer files, we don't check for executable inside
		}

		results = append(results, *match)
	}

	return results, nil
}

// --- Helpers ---

// installerExtensions lists known installer/archive extensions that should be stripped.
// Order matters: compound extensions like .tar.gz must come before .gz.
var installerExtensions = []string{
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".exe",
	".msi",
	".zip",
	".7z",
	".rar",
	".dmg",
	".pkg",
	".deb",
	".rpm",
	".apk",
	".gz",
	".bz2",
	".xz",
}

// stripExtension removes known installer/archive extensions from a filename.
// If no known extension is found, it removes the last extension using filepath.Ext.
func stripExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range installerExtensions {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	// Fallback: remove last extension
	ext := filepath.Ext(name)
	if ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

func filepathIsAbs(path string) bool {
	return len(path) > 0 && (path[0] == '/' || path[0] == '\\' || (len(path) >= 2 && path[1] == ':'))
}

func joinPath(base, name string) string {
	base = strings.TrimRight(base, "/\\")
	return base + string(os.PathSeparator) + name
}
