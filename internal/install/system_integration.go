package install

import (
	"path/filepath"
	"runtime"

	"github.com/bws/bws/internal/system"
	"github.com/bws/bws/internal/version"
)

// SystemDetector is the interface for system browser detection.
// Allows injecting a mock for testing.
type SystemDetector interface {
	DetectAll() []system.BrowserInfo
	DetectAllForBrowser(browserName string) []system.BrowserInfo
	Detect(browserName string) (system.BrowserInfo, bool)
	Refresh() []system.BrowserInfo
	InvalidateCache()
}

// AttachSystem attaches a system detector to the install manager.
// When attached, methods like ListWithSystem will include system browsers.
func (m *Manager) AttachSystem(detector SystemDetector) {
	m.systemDetector = detector
}

// HasSystem returns true if a system detector is attached.
func (m *Manager) HasSystem() bool {
	return m.systemDetector != nil
}

// ListWithSystem returns all installed versions plus system-detected versions.
// System versions are marked with IsSystem=true and Source="system".
func (m *Manager) ListWithSystem() (version.List, error) {
	installed, err := m.ListInstalled()
	if err != nil {
		return nil, err
	}

	if m.systemDetector == nil {
		return installed, nil
	}

	// Get system browsers
	systemBrowsers := m.systemDetector.DetectAll()
	for _, sb := range systemBrowsers {
		installed = append(installed, systemBrowserToVersion(sb))
	}

	return installed, nil
}

// ListWithSystemByBrowser returns installed + system versions for one browser.
func (m *Manager) ListWithSystemByBrowser(browserName string) (version.List, error) {
	all, err := m.ListWithSystem()
	if err != nil {
		return nil, err
	}
	return all.Filter(version.Filter{Browser: browserName}), nil
}

// GetRecordWithSystem returns an install record for a version,
// checking both installed and system browsers.
// For system browsers, the record is synthesized on the fly.
func (m *Manager) GetRecordWithSystem(browserName string, ver string) (*version.InstallRecord, bool) {
	// First check locally installed versions
	if m.IsInstalled(browserName, ver) {
		record, err := m.GetRecord(browserName, ver)
		if err == nil {
			return record, true
		}
	}

	// Then check system browsers
	if m.systemDetector != nil {
		systemBrowsers := m.systemDetector.DetectAllForBrowser(browserName)
		for _, sb := range systemBrowsers {
			if sb.Version == ver || (ver == "system" && sb.Channel == "stable") {
				return systemBrowserToRecord(sb), true
			}
		}
	}

	return nil, false
}

// FindSystemByVersion finds a system browser by browser name and version.
func (m *Manager) FindSystemByVersion(browserName string, ver string) (system.BrowserInfo, bool) {
	if m.systemDetector == nil {
		return system.BrowserInfo{}, false
	}

	systemBrowsers := m.systemDetector.DetectAllForBrowser(browserName)
	for _, sb := range systemBrowsers {
		if sb.Version == ver {
			return sb, true
		}
	}
	return system.BrowserInfo{}, false
}

// GetSystemDefault returns the default (stable) system browser.
func (m *Manager) GetSystemDefault(browserName string) (system.BrowserInfo, bool) {
	if m.systemDetector == nil {
		return system.BrowserInfo{}, false
	}
	return m.systemDetector.Detect(browserName)
}

// IsSystemVersion checks if a version is a system-installed browser.
// Returns false if version is locally installed or not found.
func (m *Manager) IsSystemVersion(browserName string, ver string) bool {
	if m.systemDetector == nil {
		return false
	}

	// If it's locally installed, it's not a system version
	if m.IsInstalled(browserName, ver) {
		return false
	}

	systemBrowsers := m.systemDetector.DetectAllForBrowser(browserName)
	for _, sb := range systemBrowsers {
		if sb.Version == ver {
			return true
		}
	}
	return false
}

// GetExecutableWithSystem returns the executable path for a version,
// supporting both locally installed and system browsers.
func (m *Manager) GetExecutableWithSystem(browserName string, ver string) (string, bool) {
	// Check locally installed first
	if m.IsInstalled(browserName, ver) {
		path, err := m.GetExecutablePath(browserName, ver)
		if err == nil {
			return path, true
		}
	}

	// Check system browsers
	if m.systemDetector != nil {
		sb, found := m.FindSystemByVersion(browserName, ver)
		if found {
			return sb.Executable, true
		}
	}

	return "", false
}

// --- Helpers ---

// systemBrowserToVersion converts a system.BrowserInfo to a version.Version.
func systemBrowserToVersion(sb system.BrowserInfo) version.Version {
	return version.Version{
		Browser:      sb.Browser,
		Version:      sb.Version,
		MajorVersion: version.Major(sb.Version),
		Channel:      sb.Channel,
		Platform:     platformFromArch(sb.Architecture),
		Arch:         sb.Architecture,
		Source:       "system",
		IsSystem:     true,
	}
}

// systemBrowserToRecord converts a system.BrowserInfo to an InstallRecord.
// The record is synthetic — it represents a read-only system installation.
func systemBrowserToRecord(sb system.BrowserInfo) *version.InstallRecord {
	execRel := filepath.Base(sb.Executable)
	return &version.InstallRecord{
		Browser:        sb.Browser,
		Version:        sb.Version,
		Platform:       platformFromArch(sb.Architecture),
		Arch:           sb.Architecture,
		InstallDir:     sb.InstallPath,
		ExecutablePath: execRel,
		Source:         "system",
		IsSystem:       true,
		Channel:        sb.Channel,
	}
}

// platformFromArch returns the current platform string.
// Since system detection runs on the current machine, the platform is always
// the current OS.
func platformFromArch(arch string) string {
	return runtime.GOOS
}
