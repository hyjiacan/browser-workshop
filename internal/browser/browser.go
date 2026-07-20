// Package browser provides browser descriptors and registry.
// All browser-specific logic is centralized in BrowserDescriptor objects,
// so core modules only depend on the descriptor interface.
package browser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// BrowserFeatures describes which features a browser supports.
type BrowserFeatures struct {
	SupportsHeadless  bool
	SupportsIncognito bool
	SupportsProfile   bool
	CanMultiInstance  bool
	HasUserDirArg     bool
}

// BrowserDescriptor encapsulates all browser-specific characteristics.
// Core modules only interact with browsers through this descriptor.
type BrowserDescriptor struct {
	// Basic info
	Name        string // Unique identifier, e.g. "chrome", "firefox"
	DisplayName string // Human-readable name, e.g. "Google Chrome"
	Icon        string // Optional emoji/icon for TUI display

	// Executable file candidates per platform and architecture.
	// Ordered by priority — first match wins.
	// Map structure: platform → arch → []candidatePaths
	ExecutableCandidates map[string]map[string][]string

	// Profile / user data directory argument
	ProfileArg    string // e.g. "--user-data-dir=" or "-profile"
	ProfileSeparate bool // true if profile arg and path are separate arguments

	// Standard startup arguments
	MultiInstanceArgs []string // Args to allow multiple instances
	DisableUpdateArgs []string // Args to disable auto-update
	FirstRunSkipArgs  []string // Args to skip first-run wizard

	// Supported package formats, in priority order
	PackageFormats []string // e.g. ["zip", "exe", "msi"]

	// Release channels
	Channels       []string // e.g. ["stable", "beta", "dev", "canary"]
	DefaultChannel string

	// Expected version segment count (for validation)
	VersionSegments int

	// Feature flags
	Features BrowserFeatures
}

// Registry holds all registered browser descriptors.
type Registry struct {
	browsers map[string]*BrowserDescriptor
	aliases  map[string]string // alias -> canonical name
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		browsers: make(map[string]*BrowserDescriptor),
		aliases:  make(map[string]string),
	}
}

// Register adds a browser descriptor to the registry.
func (r *Registry) Register(desc *BrowserDescriptor) {
	r.browsers[desc.Name] = desc
}

// RegisterAlias adds a short alias for a browser name.
// The alias is case-insensitive.
func (r *Registry) RegisterAlias(alias string, canonicalName string) {
	r.aliases[strings.ToLower(alias)] = strings.ToLower(canonicalName)
}

// ResolveName resolves a browser name or alias to its canonical name.
// Returns the canonical name and true if found, otherwise returns the input and false.
func (r *Registry) ResolveName(name string) (string, bool) {
	lower := strings.ToLower(name)
	// Check if it's already a canonical name
	if _, ok := r.browsers[lower]; ok {
		return lower, true
	}
	// Check aliases
	if canonical, ok := r.aliases[lower]; ok {
		return canonical, true
	}
	return name, false
}

// Get returns the descriptor for the given browser name (or alias), or nil if not found.
func (r *Registry) Get(name string) *BrowserDescriptor {
	canonical, _ := r.ResolveName(name)
	return r.browsers[canonical]
}

// List returns all registered browser descriptors.
func (r *Registry) List() []*BrowserDescriptor {
	result := make([]*BrowserDescriptor, 0, len(r.browsers))
	for _, b := range r.browsers {
		result = append(result, b)
	}
	return result
}

// Names returns the names of all registered browsers.
func (r *Registry) Names() []string {
	result := make([]string, 0, len(r.browsers))
	for name := range r.browsers {
		result = append(result, name)
	}
	return result
}

// Has checks if a browser is registered (supports aliases).
func (r *Registry) Has(name string) bool {
	_, ok := r.ResolveName(name)
	return ok
}

// FindExecutable searches for the browser's executable within dir.
// It checks all candidate paths for the given platform/arch.
// Returns the relative path to the executable within dir.
func (r *Registry) FindExecutable(browser string, dir string, platform string, arch string) (string, error) {
	desc := r.Get(browser)
	if desc == nil {
		return "", errors.New("browser not registered: " + browser)
	}

	candidates := desc.ExecutableCandidates[platform]
	if candidates == nil {
		return "", errors.New("no executable candidates for platform: " + platform)
	}

	archCandidates := candidates[arch]
	if len(archCandidates) == 0 {
		// Fallback: try all arches for this platform
		for _, ac := range candidates {
			archCandidates = append(archCandidates, ac...)
		}
	}

	for _, candidate := range archCandidates {
		fullPath := filepath.Join(dir, candidate)
		if fileIsExecutable(fullPath) {
			return candidate, nil
		}
	}

	// Last resort: search recursively up to 2 levels deep for any matching name
	for _, candidate := range archCandidates {
		baseName := filepath.Base(candidate)
		found := searchRecursive(dir, baseName, 2)
		if found != "" {
			return found, nil
		}
	}

	return "", errors.New("executable not found in " + dir)
}

// DetectBrowser tries to identify which browser is installed in dir
// by checking executable files. Returns the browser name.
func (r *Registry) DetectBrowser(dir string, platform string, arch string) (string, error) {
	for _, desc := range r.browsers {
		candidates := desc.ExecutableCandidates[platform]
		if candidates == nil {
			continue
		}

		archCandidates := candidates[arch]
		if len(archCandidates) == 0 {
			for _, ac := range candidates {
				archCandidates = append(archCandidates, ac...)
			}
		}

		for _, candidate := range archCandidates {
			fullPath := filepath.Join(dir, candidate)
			if fileIsExecutable(fullPath) {
				return desc.Name, nil
			}
		}

		// Recursive search
		for _, candidate := range archCandidates {
			baseName := filepath.Base(candidate)
			if searchRecursive(dir, baseName, 2) != "" {
				return desc.Name, nil
			}
		}
	}

	return "", errors.New("unrecognized browser directory: " + dir)
}

// BuildProfileArgs returns the command-line arguments for setting the profile directory.
func (desc *BrowserDescriptor) BuildProfileArgs(profilePath string) []string {
	if !desc.Features.SupportsProfile {
		return nil
	}

	if desc.ProfileSeparate {
		return []string{desc.ProfileArg, profilePath}
	}
	return []string{desc.ProfileArg + profilePath}
}

// BuildStandardArgs returns the standard set of arguments for isolated launching.
func (desc *BrowserDescriptor) BuildStandardArgs() []string {
	var args []string
	args = append(args, desc.MultiInstanceArgs...)
	args = append(args, desc.DisableUpdateArgs...)
	args = append(args, desc.FirstRunSkipArgs...)
	return args
}

// DefaultRegistry is the global default registry with built-in browsers.
var DefaultRegistry = NewRegistry()

func init() {
	DefaultRegistry.Register(Chrome)
	DefaultRegistry.Register(Chromium)
	DefaultRegistry.Register(Firefox)

	// 注册短别名
	DefaultRegistry.RegisterAlias("gc", "chrome")
	DefaultRegistry.RegisterAlias("googlechrome", "chrome")
	DefaultRegistry.RegisterAlias("google-chrome", "chrome")
	DefaultRegistry.RegisterAlias("cm", "chromium")
	DefaultRegistry.RegisterAlias("ff", "firefox")
}

// fileIsExecutable checks if a file exists and is executable.
// On Windows, we just check existence of .exe files.
func fileIsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return true
}

// searchRecursive searches for a file by name up to maxDepth levels deep.
// Returns the relative path from root if found, empty string otherwise.
func searchRecursive(root string, target string, maxDepth int) string {
	var result string

	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		if depth > maxDepth {
			return false
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}

		for _, entry := range entries {
			fullPath := filepath.Join(dir, entry.Name())

			if entry.Name() == target && !entry.IsDir() {
				rel, err := filepath.Rel(root, fullPath)
				if err == nil {
					result = rel
					return true
				}
			}

			if entry.IsDir() {
				if walk(fullPath, depth+1) {
					return true
				}
			}
		}
		return false
	}

	walk(root, 0)
	return result
}
