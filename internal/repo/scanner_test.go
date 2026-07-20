package repo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
)

func TestNewScanner(t *testing.T) {
	reg := browser.NewRegistry()

	t.Run("basic creation", func(t *testing.T) {
		s, err := NewScanner("/tmp/repo", reg)
		if err != nil {
			t.Fatalf("NewScanner() error = %v", err)
		}
		if s.Path() != "/tmp/repo" {
			t.Errorf("Path() = %q, want '/tmp/repo'", s.Path())
		}
	})

	t.Run("nil registry", func(t *testing.T) {
		_, err := NewScanner("/tmp/repo", nil)
		if err == nil {
			t.Error("expected error for nil registry")
		}
	})
}

func TestDetectBrowser(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantBrowser string
		wantKeyword string
	}{
		{"chrome simple", "chrome", "chrome", "chrome"},
		{"chrome capitalized", "Chrome", "chrome", "chrome"},
		{"google chrome", "Google Chrome", "chrome", "google chrome"},
		{"google-chrome", "google-chrome", "chrome", "google-chrome"},
		{"googlechrome", "googlechrome", "chrome", "googlechrome"},
		{"chromium", "chromium", "chromium", "chromium"},
		{"google-chromium", "google-chromium", "chromium", "google-chromium"},
		{"firefox simple", "firefox", "firefox", "firefox"},
		{"mozilla firefox", "Mozilla Firefox", "firefox", "mozilla firefox"},
		{"mozilla-firefox", "mozilla-firefox", "firefox", "mozilla-firefox"},
		{"edge simple", "edge", "edge", "edge"},
		{"microsoft edge", "Microsoft Edge", "edge", "edge"},
		{"microsoft-edge", "microsoft-edge", "edge", "microsoft-edge"},
		{"msedge", "msedge", "edge", "msedge"},
		{"brave simple", "brave", "brave", "brave"},
		{"brave-browser", "brave-browser", "brave", "brave-browser"},
		{"opera simple", "opera", "opera", "opera"},
		{"chrome in version string", "chrome_120.0.6099.109", "chrome", "chrome"},
		{"firefox in version string", "firefox-121.0", "firefox", "firefox"},
		{"unrecognized", "random-browser", "", ""},
		{"empty string", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browserName, keyword := detectBrowser(tt.input)
			if browserName != tt.wantBrowser {
				t.Errorf("detectBrowser(%q) browser = %q, want %q", tt.input, browserName, tt.wantBrowser)
			}
			if keyword != tt.wantKeyword {
				t.Errorf("detectBrowser(%q) keyword = %q, want %q", tt.input, keyword, tt.wantKeyword)
			}
		})
	}
}

func TestDetectVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVer string
	}{
		{"full version", "chrome_120.0.6099.109_win64", "120.0.6099.109"},
		{"two-part version", "firefox-121.0", "121.0"},
		{"three-part version", "chromium_120.0.0", "120.0.0"},
		{"esr version", "firefox_115.6.0esr", "115.6.0esr"},
		{"version only", "120.0.6099.109", "120.0.6099.109"},
		{"version at start", "108.0.5359.48_chrome64_beta", "108.0.5359.48"},
		{"no version", "chrome_setup", ""},
		{"empty string", "", ""},
		{"single number not matched", "chrome123", ""},
		{"v-prefixed version", "v120.0.6099.109", "120.0.6099.109"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := detectVersion(tt.input)
			if ver != tt.wantVer {
				t.Errorf("detectVersion(%q) = %q, want %q", tt.input, ver, tt.wantVer)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"windows full", "chrome_windows_amd64", "windows"},
		{"win64", "chrome_120_win64", "windows"},
		{"win32", "chrome_120_win32", "windows"},
		{"win", "chrome_120_win", "windows"},
		{"macos", "firefox_macos_arm64", "darwin"},
		{"mac64", "firefox_mac64", "darwin"},
		{"mac", "firefox_mac_installer", "darwin"},
		{"mac os", "chrome mac os version", "darwin"},
		{"linux64", "chromium_linux64", "linux"},
		{"linux", "chromium_linux_amd64", "linux"},
		{"no platform", "chrome_120.0.6099.109", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := detectPlatform(tt.input)
			if p != tt.want {
				t.Errorf("detectPlatform(%q) = %q, want %q", tt.input, p, tt.want)
			}
		})
	}
}

func TestDetectArch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"arm64 direct", "chrome_arm64", "arm64"},
		{"aarch64", "chrome_aarch64", "arm64"},
		{"macarm64", "chrome_macarm64", "arm64"},
		{"amd64 direct", "firefox_amd64", "amd64"},
		{"x86_64", "firefox_x86_64", "amd64"},
		{"x64", "firefox_x64", "amd64"},
		{"win64", "chrome_win64", "amd64"},
		{"chrome64", "108.0.5359.48_chrome64_beta", "amd64"},
		{"firefox64", "firefox64_stable", "amd64"},
		{"mac64", "firefox_mac64", "amd64"},
		{"linux64", "chromium_linux64", "amd64"},
		{"386 direct", "chrome_386", "386"},
		{"x86", "firefox_x86", "386"},
		{"win32", "chrome_win32", "386"},
		{"i386", "chrome_i386", "386"},
		{"chrome32", "chrome32_installer", "386"},
		{"firefox32", "firefox32_stable", "386"},
		{"no arch", "chrome_120.0.6099.109", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := detectArch(tt.input)
			if a != tt.want {
				t.Errorf("detectArch(%q) = %q, want %q", tt.input, a, tt.want)
			}
		})
	}
}

func TestDetectChannel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"canary", "chrome_canary_120", "canary"},
		{"dev", "chrome_dev_120", "dev"},
		{"developer", "firefox_developer_121", "dev"},
		{"beta", "chrome_beta_120", "beta"},
		{"esr", "firefox_115.6.0esr", "esr"},
		{"stable", "chrome_stable_120", "stable"},
		{"official", "chrome_official_120", "stable"},
		{"release", "firefox_release_121", "stable"},
		{"no channel", "chrome_120.0.6099.109", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := detectChannel(tt.input)
			if c != tt.want {
				t.Errorf("detectChannel(%q) = %q, want %q", tt.input, c, tt.want)
			}
		})
	}
}

func TestScanDir(t *testing.T) {
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"chrome.exe"}},
		},
	})
	reg.Register(&browser.BrowserDescriptor{
		Name: "firefox",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {"firefox.exe"}},
		},
	})

	s, err := NewScanner("/tmp/repo", reg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		dirName        string
		defaultBrowser string
		defaultArch    string
		wantBrowser    string
		wantVersion    string
		wantArch       string
		wantStatus     MatchStatus
	}{
		{
			name:        "underscore format with win64",
			dirName:     "chrome_120.0.6099.109_win64",
			wantBrowser: "chrome",
			wantVersion: "120.0.6099.109",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
		},
		{
			name:        "dash format with win64",
			dirName:     "firefox-121.0-win64",
			wantBrowser: "firefox",
			wantVersion: "121.0",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
		},
		{
			name:        "space format with x64",
			dirName:     "Chrome 120.0.6099.109 x64",
			wantBrowser: "chrome",
			wantVersion: "120.0.6099.109",
			wantArch:    "amd64",
			wantStatus:  MatchPartial,
		},
		{
			name:        "no separator",
			dirName:     "chrome120.0.6099.109",
			wantBrowser: "chrome",
			wantVersion: "120.0.6099.109",
			wantStatus:  MatchPartial,
		},
		{
			name:           "version only with default browser",
			dirName:        "120.0.6099.109",
			defaultBrowser: "chrome",
			defaultArch:    "amd64",
			wantBrowser:    "chrome",
			wantVersion:    "120.0.6099.109",
			wantArch:       "amd64",
			wantStatus:     MatchPartial,
		},
		{
			name:        "esr version with win64",
			dirName:     "firefox_115.6.0esr_win64",
			wantBrowser: "firefox",
			wantVersion: "115.6.0esr",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
		},
		{
			name:        "linux64 format",
			dirName:     "chromium_120.0.0.1_linux64",
			wantBrowser: "chromium",
			wantVersion: "120.0.0.1",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
		},
		{
			name:        "with channel and platform",
			dirName:     "chrome-beta_120.0.6099.109_win64",
			wantBrowser: "chrome",
			wantVersion: "120.0.6099.109",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
		},
		{
			name:       "completely unrecognized",
			dirName:    "random-folder",
			wantStatus: MatchUnrecognized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ScanDir(tt.dirName, tt.defaultBrowser, tt.defaultArch)
			if result == nil {
				t.Fatal("ScanDir returned nil")
			}
			if result.IsFile {
				t.Errorf("IsFile = true, want false for ScanDir")
			}
			if tt.wantBrowser != "" && result.Browser != tt.wantBrowser {
				t.Errorf("Browser = %q, want %q", result.Browser, tt.wantBrowser)
			}
			if tt.wantVersion != "" && result.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", result.Version, tt.wantVersion)
			}
			if tt.wantArch != "" && result.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", result.Arch, tt.wantArch)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (%s)", result.Status, tt.wantStatus, tt.wantStatus.String())
			}
		})
	}
}

func TestScanEntry_FilePatterns(t *testing.T) {
	reg := browser.NewRegistry()
	s, err := NewScanner("/tmp/repo", reg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		entryName    string
		fullName     string
		isFile       bool
		wantBrowser  string
		wantVersion  string
		wantArch     string
		wantChannel  string
		wantPlatform string
		wantStatus   MatchStatus
		wantIsFile   bool
	}{
		// Version-first format: 108.0.5359.48_chrome64_beta_windows_installer
		{
			name:         "installer version_browser64_channel_platform",
			entryName:    "108.0.5359.48_chrome64_beta_windows_installer",
			fullName:     "108.0.5359.48_chrome64_beta_windows_installer.exe",
			isFile:       true,
			wantBrowser:  "chrome",
			wantVersion:  "108.0.5359.48",
			wantArch:     "amd64",
			wantChannel:  "beta",
			wantPlatform: "windows",
			wantStatus:   MatchOK,
			wantIsFile:   true,
		},
		{
			name:         "installer firefox32_stable_mac",
			entryName:    "115.0_firefox32_stable_mac_installer",
			fullName:     "115.0_firefox32_stable_mac_installer.dmg",
			isFile:       true,
			wantBrowser:  "firefox",
			wantVersion:  "115.0",
			wantArch:     "386",
			wantChannel:  "stable",
			wantPlatform: "darwin",
			wantStatus:   MatchOK,
			wantIsFile:   true,
		},
		{
			name:         "installer edge64_dev_linux",
			entryName:    "120.0.0.1_edge64_dev_linux_installer",
			fullName:     "120.0.0.1_edge64_dev_linux_installer.tar.gz",
			isFile:       true,
			wantBrowser:  "edge",
			wantVersion:  "120.0.0.1",
			wantArch:     "amd64",
			wantChannel:  "dev",
			wantPlatform: "linux",
			wantStatus:   MatchOK,
			wantIsFile:   true,
		},
		// Browser-first format: chrome_108.0.5359.48_win64_setup
		{
			name:        "setup browser_version_arch_setup",
			entryName:   "chrome_108.0.5359.48_win64_setup",
			fullName:    "chrome_108.0.5359.48_win64_setup.exe",
			isFile:      true,
			wantBrowser: "chrome",
			wantVersion: "108.0.5359.48",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
			wantIsFile:  true,
		},
		{
			name:        "setup browser_version_amd64_installer",
			entryName:   "firefox_115.0_amd64_installer",
			fullName:    "firefox_115.0_amd64_installer.msi",
			isFile:      true,
			wantBrowser: "firefox",
			wantVersion: "115.0",
			wantArch:    "amd64",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		{
			name:        "setup chromium_arm64_portable",
			entryName:   "chromium_120.0.0.1_arm64_portable",
			fullName:    "chromium_120.0.0.1_arm64_portable.zip",
			isFile:      true,
			wantBrowser: "chromium",
			wantVersion: "120.0.0.1",
			wantArch:    "arm64",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		{
			name:        "setup edge_x64_installer",
			entryName:   "edge_120.0.0.1_x64_installer",
			fullName:    "edge_120.0.0.1_x64_installer.7z",
			isFile:      true,
			wantBrowser: "edge",
			wantVersion: "120.0.0.1",
			wantArch:    "amd64",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		// BrowserSetup format: ChromeSetup_108.0.5359.48
		{
			name:        "ChromeSetup pattern",
			entryName:   "ChromeSetup_108.0.5359.48",
			fullName:    "ChromeSetup_108.0.5359.48.exe",
			isFile:      true,
			wantBrowser: "chrome",
			wantVersion: "108.0.5359.48",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		{
			name:        "FirefoxSetup pattern with dash",
			entryName:   "FirefoxSetup-v115.0",
			fullName:    "FirefoxSetup-v115.0.exe",
			isFile:      true,
			wantBrowser: "firefox",
			wantVersion: "115.0",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		{
			name:        "EdgeSetup pattern",
			entryName:   "EdgeSetup_120.0.0.1",
			fullName:    "EdgeSetup_120.0.0.1.exe",
			isFile:      true,
			wantBrowser: "edge",
			wantVersion: "120.0.0.1",
			wantStatus:  MatchPartial,
			wantIsFile:  true,
		},
		// Brave browser
		{
			name:        "brave browser installer",
			entryName:   "brave-browser_120.0.0.1_win64_setup",
			fullName:    "brave-browser_120.0.0.1_win64_setup.exe",
			isFile:      true,
			wantBrowser: "brave",
			wantVersion: "120.0.0.1",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
			wantIsFile:  true,
		},
		// Opera browser
		{
			name:        "opera browser installer",
			entryName:   "opera_100.0.0.1_win64_installer",
			fullName:    "opera_100.0.0.1_win64_installer.exe",
			isFile:      true,
			wantBrowser: "opera",
			wantVersion: "100.0.0.1",
			wantArch:    "amd64",
			wantStatus:  MatchOK,
			wantIsFile:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ScanEntry(tt.entryName, tt.fullName, tt.isFile, "", "")
			if result == nil {
				t.Fatal("ScanEntry returned nil")
			}
			if result.IsFile != tt.wantIsFile {
				t.Errorf("IsFile = %v, want %v", result.IsFile, tt.wantIsFile)
			}
			if tt.isFile && result.FileName != tt.entryName {
				t.Errorf("FileName = %q, want %q", result.FileName, tt.entryName)
			}
			if result.DirName != tt.fullName {
				t.Errorf("DirName = %q, want %q", result.DirName, tt.fullName)
			}
			if tt.wantBrowser != "" && result.Browser != tt.wantBrowser {
				t.Errorf("Browser = %q, want %q", result.Browser, tt.wantBrowser)
			}
			if tt.wantVersion != "" && result.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", result.Version, tt.wantVersion)
			}
			if tt.wantArch != "" && result.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", result.Arch, tt.wantArch)
			}
			if tt.wantChannel != "" && result.Channel != tt.wantChannel {
				t.Errorf("Channel = %q, want %q", result.Channel, tt.wantChannel)
			}
			if tt.wantPlatform != "" && result.Platform != tt.wantPlatform {
				t.Errorf("Platform = %q, want %q", result.Platform, tt.wantPlatform)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (%s)", result.Status, tt.wantStatus, tt.wantStatus.String())
			}
		})
	}
}

func TestStripExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"chrome_setup.exe", "chrome_setup"},
		{"firefox_115.0.zip", "firefox_115.0"},
		{"chromium_120.7z", "chromium_120"},
		{"EdgeSetup.dmg", "EdgeSetup"},
		{"chrome_120.tar.gz", "chrome_120"},
		{"firefox_115.tar.bz2", "firefox_115"},
		{"chrome_120.tar.xz", "chrome_120"},
		{"edge_120.msi", "edge_120"},
		{"chrome.rpm", "chrome"},
		{"firefox.deb", "firefox"},
		{"chrome.rar", "chrome"},
		{"noextension", "noextension"},
		{"ChromeSetup_108.0.5359.48.exe", "ChromeSetup_108.0.5359.48"},
		{"108.0.5359.48_chrome64_beta_windows_installer.exe", "108.0.5359.48_chrome64_beta_windows_installer"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripExtension(tt.input)
			if result != tt.expected {
				t.Errorf("stripExtension(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// platformArchDirSuffix returns a directory name suffix that contains both platform and arch keywords
// appropriate for the current platform, so that keyword detection yields MatchOK.
func platformArchDirSuffix() string {
	switch paths.Platform() {
	case "windows":
		if paths.Arch() == "amd64" {
			return "win64"
		}
		return "win32"
	case "darwin":
		if paths.Arch() == "arm64" {
			return "macarm64"
		}
		return "mac64"
	case "linux":
		if paths.Arch() == "arm64" {
			return "linux-arm64"
		}
		return "linux64"
	default:
		return paths.Arch()
	}
}

func TestScanRepository_WithFiles(t *testing.T) {
	reg := browser.NewRegistry()
	exeName := "chrome"
	if runtime.GOOS == "windows" {
		exeName = "chrome.exe"
	}
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {exeName}},
		},
	})

	// Create test repository structure
	repoDir := t.TempDir()
	suffix := platformArchDirSuffix()

	// Create a properly named directory with executable
	goodDir := filepath.Join(repoDir, "chrome_1.0.0_"+suffix)
	os.MkdirAll(goodDir, 0o755)
	os.WriteFile(filepath.Join(goodDir, exeName), []byte("fake"), 0o755)

	// Create a directory without executable
	noExeDir := filepath.Join(repoDir, "chrome_2.0.0_"+suffix)
	os.MkdirAll(noExeDir, 0o755)

	// Create an unrecognized directory
	unrecognizedDir := filepath.Join(repoDir, "random-stuff")
	os.MkdirAll(unrecognizedDir, 0o755)

	// Create installer files
	os.WriteFile(filepath.Join(repoDir, "chrome_3.0.0_win64_setup.exe"), []byte("fake"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "ChromeSetup_4.0.0.exe"), []byte("fake"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "random-file.txt"), []byte("fake"), 0o644)

	s, err := NewScanner("", reg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := s.ScanRepository(repoDir, "", "")
	if err != nil {
		t.Fatalf("ScanRepository() error = %v", err)
	}

	// Should have 3 dirs + 3 files = 6 entries
	if len(results) != 6 {
		t.Errorf("got %d results, want 6 (3 dirs + 3 files)", len(results))
	}

	// Count by type and status
	fileCount := 0
	dirCount := 0
	statusCounts := make(map[MatchStatus]int)
	for _, r := range results {
		statusCounts[r.Status]++
		if r.IsFile {
			fileCount++
		} else {
			dirCount++
		}
	}

	if fileCount != 3 {
		t.Errorf("expected 3 files, got %d", fileCount)
	}
	if dirCount != 3 {
		t.Errorf("expected 3 directories, got %d", dirCount)
	}

	// Check that files have FileName set
	for _, r := range results {
		if r.IsFile && r.FileName == "" {
			t.Errorf("file match has empty FileName: %s", r.DirName)
		}
		if !r.IsFile && r.FileName != "" {
			t.Errorf("directory match has non-empty FileName: %s", r.DirName)
		}
	}

	// Verify specific file matches
	var foundSetupExe, foundChromeSetupExe bool
	for _, r := range results {
		if r.IsFile && r.DirName == "chrome_3.0.0_win64_setup.exe" {
			foundSetupExe = true
			if r.Browser != "chrome" {
				t.Errorf("chrome_3.0.0_win64_setup.exe: Browser = %q, want 'chrome'", r.Browser)
			}
			if r.Version != "3.0.0" {
				t.Errorf("chrome_3.0.0_win64_setup.exe: Version = %q, want '3.0.0'", r.Version)
			}
			if r.Status != MatchOK {
				t.Errorf("chrome_3.0.0_win64_setup.exe: Status = %v, want MatchOK", r.Status)
			}
		}
		if r.IsFile && r.DirName == "ChromeSetup_4.0.0.exe" {
			foundChromeSetupExe = true
			if r.Browser != "chrome" {
				t.Errorf("ChromeSetup_4.0.0.exe: Browser = %q, want 'chrome'", r.Browser)
			}
			if r.Version != "4.0.0" {
				t.Errorf("ChromeSetup_4.0.0.exe: Version = %q, want '4.0.0'", r.Version)
			}
		}
	}
	if !foundSetupExe {
		t.Error("did not find chrome_3.0.0_win64_setup.exe in results")
	}
	if !foundChromeSetupExe {
		t.Error("did not find ChromeSetup_4.0.0.exe in results")
	}
}

func TestScan(t *testing.T) {
	reg := browser.NewRegistry()
	exeName := "chrome"
	if runtime.GOOS == "windows" {
		exeName = "chrome.exe"
	}
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {exeName}},
		},
	})

	// Create test repository structure
	repoDir := t.TempDir()
	suffix := platformArchDirSuffix()

	// Create a properly named directory with executable
	goodDir := filepath.Join(repoDir, "chrome_1.0.0_"+suffix)
	os.MkdirAll(goodDir, 0o755)
	os.WriteFile(filepath.Join(goodDir, exeName), []byte("fake"), 0o755)

	// Create a directory without executable
	noExeDir := filepath.Join(repoDir, "chrome_2.0.0_"+suffix)
	os.MkdirAll(noExeDir, 0o755)

	// Create an unrecognized directory
	unrecognizedDir := filepath.Join(repoDir, "random-stuff")
	os.MkdirAll(unrecognizedDir, 0o755)

	s, err := NewScanner(repoDir, reg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}

	// Count by status
	statusCounts := make(map[MatchStatus]int)
	for _, r := range results {
		statusCounts[r.Status]++
	}

	if statusCounts[MatchOK] < 1 {
		t.Errorf("expected at least 1 MatchOK, got %d", statusCounts[MatchOK])
	}
	if statusCounts[MatchNoExecutable] < 1 {
		t.Errorf("expected at least 1 MatchNoExecutable, got %d", statusCounts[MatchNoExecutable])
	}
	if statusCounts[MatchUnrecognized] < 1 {
		t.Errorf("expected at least 1 MatchUnrecognized, got %d", statusCounts[MatchUnrecognized])
	}
}

func TestScanRepository(t *testing.T) {
	reg := browser.NewRegistry()
	exeName := "chrome"
	if runtime.GOOS == "windows" {
		exeName = "chrome.exe"
	}
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {exeName}},
		},
	})

	// Create test repository structure
	repoDir := t.TempDir()
	suffix := platformArchDirSuffix()

	// Create a properly named directory with executable
	goodDir := filepath.Join(repoDir, "chrome_1.0.0_"+suffix)
	os.MkdirAll(goodDir, 0o755)
	os.WriteFile(filepath.Join(goodDir, exeName), []byte("fake"), 0o755)

	// Create a directory without executable
	noExeDir := filepath.Join(repoDir, "chrome_2.0.0_"+suffix)
	os.MkdirAll(noExeDir, 0o755)

	// Create an unrecognized directory
	unrecognizedDir := filepath.Join(repoDir, "random-stuff")
	os.MkdirAll(unrecognizedDir, 0o755)

	s, err := NewScanner("", reg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := s.ScanRepository(repoDir, "", "")
	if err != nil {
		t.Fatalf("ScanRepository() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}

	// Count by status
	statusCounts := make(map[MatchStatus]int)
	for _, r := range results {
		statusCounts[r.Status]++
	}

	if statusCounts[MatchOK] < 1 {
		t.Errorf("expected at least 1 MatchOK, got %d", statusCounts[MatchOK])
	}
	if statusCounts[MatchNoExecutable] < 1 {
		t.Errorf("expected at least 1 MatchNoExecutable, got %d", statusCounts[MatchNoExecutable])
	}
	if statusCounts[MatchUnrecognized] < 1 {
		t.Errorf("expected at least 1 MatchUnrecognized, got %d", statusCounts[MatchUnrecognized])
	}
}

func TestMatchStatusString(t *testing.T) {
	tests := []struct {
		status   MatchStatus
		expected string
	}{
		{MatchOK, "ok"},
		{MatchPartial, "partial"},
		{MatchUnrecognized, "unrecognized"},
		{MatchNoExecutable, "no-executable"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.status.String() != tt.expected {
				t.Errorf("String() = %q, want %q", tt.status.String(), tt.expected)
			}
		})
	}
}
