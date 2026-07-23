// Package paths provides unified path management for bm.
// All directory and file paths used by bm are centralized here
// to ensure consistency across platforms.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Paths holds all directory and file paths used by bm.
type Paths struct {
	// Root is the base directory for all bm data
	Root string

	// Config is the path to the main config file
	Config string

	// LogDir is the directory for log files
	LogDir string

	// LogFile is the path to the main log file
	LogFile string

	// VersionsDir is where installed browser versions are stored
	VersionsDir string

	// CacheDir is the base cache directory
	CacheDir string

	// ManifestCacheDir stores cached version manifests from remote sources
	ManifestCacheDir string

	// DownloadCacheDir stores downloaded installation packages
	DownloadCacheDir string

	// RuntimeDir stores runtime data like browser profiles
	RuntimeDir string

	// Plugin scripts directory
	PluginsDir string
}

var (
	instance *Paths
	once     sync.Once
)

// Default returns the default Paths instance, initialized lazily.
// It first checks for portable mode (config file in exe directory),
// then falls back to the user home directory (~/.bm).
func Default() *Paths {
	once.Do(func() {
		instance = New(defaultRoot())
	})
	return instance
}

// New creates a Paths instance with the given root directory.
func New(root string) *Paths {
	p := &Paths{
		Root:             root,
		Config:           filepath.Join(root, "config.json"),
		LogDir:           filepath.Join(root, "logs"),
		LogFile:          filepath.Join(root, "logs", "bws.log"),
		VersionsDir:      filepath.Join(root, "versions"),
		CacheDir:         filepath.Join(root, "cache"),
		ManifestCacheDir: filepath.Join(root, "cache", "manifests"),
		DownloadCacheDir: filepath.Join(root, "cache", "downloads"),
		RuntimeDir:       filepath.Join(root, "runtime"),
		PluginsDir:       filepath.Join(root, "plugins"),
	}
	return p
}

// EnsurePluginsDir creates the plugins directory if it doesn't exist.
func (p *Paths) EnsurePluginsDir() error {
	return os.MkdirAll(p.PluginsDir, 0o755)
}

// EnsureAll creates all required directories if they don't exist.
func (p *Paths) EnsureAll() error {
	dirs := []string{
		p.Root,
		p.LogDir,
		p.VersionsDir,
		p.CacheDir,
		p.ManifestCacheDir,
		p.DownloadCacheDir,
		p.RuntimeDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// VersionDir returns the installation directory for a specific browser version.
func (p *Paths) VersionDir(browser string, version string) string {
	return filepath.Join(p.VersionsDir, browser, version)
}

// VersionMetaFile returns the path to the .bws.json metadata file for a version.
func (p *Paths) VersionMetaFile(browser string, version string) string {
	return filepath.Join(p.VersionDir(browser, version), ".bws.json")
}

// DownloadDir returns the download cache directory for a browser version.
func (p *Paths) DownloadDir(browser string, version string) string {
	return filepath.Join(p.DownloadCacheDir, browser, version)
}

// ProfileDir returns the profile directory for a browser version.
func (p *Paths) ProfileDir(browser string, version string) string {
	return filepath.Join(p.RuntimeDir, browser, version, "profile")
}

// ManifestFile returns the path to the cached manifest file for a browser.
func (p *Paths) ManifestFile(browser string) string {
	return filepath.Join(p.ManifestCacheDir, browser+".json")
}

// Platform returns the current platform name in bm's canonical form.
func Platform() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

// Arch returns the current architecture in bm's canonical form.
func Arch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "386":
		return "386"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// ArchCompatible checks whether a given architecture can run on the current system.
// Returns true if the arch is empty (unknown = assume compatible).
//
// Compatibility rules:
//   - amd64 system can run amd64 and 386 (x86)
//   - 386 system can only run 386
//   - arm64 system can run arm64 (Rosetta on macOS not checked here)
func ArchCompatible(arch string) bool {
	if arch == "" {
		return true
	}
	current := Arch()
	if current == arch {
		return true
	}
	// amd64 can also run 386
	if current == "amd64" && arch == "386" {
		return true
	}
	return false
}

// ExeDir returns the directory containing the current executable.
func ExeDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

// IsPortable returns true if running in portable mode.
// bm is always portable by default - data is stored in the exe directory.
func IsPortable() bool {
	return true
}

// defaultRoot returns the default root directory for bws data.
// Portable mode: bws-data subdirectory next to the executable.
// Fallback: ~/.bws in user home directory.
func defaultRoot() string {
	exeDir, err := ExeDir()
	if err == nil {
		return filepath.Join(exeDir, "bws-data")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir can't be determined
		wd, err := os.Getwd()
		if err == nil {
			return wd
		}
		return "."
	}
	return filepath.Join(home, ".bws")
}

// PortableRoot returns the root path for portable mode (exe directory).
func PortableRoot() (string, error) {
	return ExeDir()
}
