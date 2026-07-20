package install

import (
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/system"
	"github.com/bws/bws/internal/version"
)

// mockSystemDetector is a mock implementation of SystemDetector for testing.
type mockSystemDetector struct {
	browsers []system.BrowserInfo
}

func (m *mockSystemDetector) DetectAll() []system.BrowserInfo {
	return m.browsers
}

func (m *mockSystemDetector) DetectAllForBrowser(browserName string) []system.BrowserInfo {
	var result []system.BrowserInfo
	for _, b := range m.browsers {
		if b.Browser == browserName {
			result = append(result, b)
		}
	}
	return result
}

func (m *mockSystemDetector) Detect(browserName string) (system.BrowserInfo, bool) {
	for _, b := range m.browsers {
		if b.Browser == browserName && b.Channel == "stable" {
			return b, true
		}
	}
	for _, b := range m.browsers {
		if b.Browser == browserName {
			return b, true
		}
	}
	return system.BrowserInfo{}, false
}

func (m *mockSystemDetector) Refresh() []system.BrowserInfo {
	return m.browsers
}

func (m *mockSystemDetector) InvalidateCache() {}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	p := paths.New(tmpDir)
	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name:        "chrome",
		DisplayName: "Google Chrome",
	})
	return NewManager(p, reg)
}

func TestHasSystem_NoDetector(t *testing.T) {
	m := newTestManager(t)
	if m.HasSystem() {
		t.Error("HasSystem() should be false without detector")
	}
}

func TestAttachSystem(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{}
	m.AttachSystem(mock)

	if !m.HasSystem() {
		t.Error("HasSystem() should be true after attaching detector")
	}
}

func TestListWithSystem_NoDetector(t *testing.T) {
	m := newTestManager(t)
	list, err := m.ListWithSystem()
	if err != nil {
		t.Fatalf("ListWithSystem() error: %v", err)
	}
	// No installed versions, no system detector → empty list
	if len(list) != 0 {
		t.Errorf("ListWithSystem() = %d items, want 0", len(list))
	}
}

func TestListWithSystem_SystemOnly(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:      "chrome",
				DisplayName:  "Google Chrome",
				Version:      "120.0.6099.109",
				InstallPath:  "/fake/path/chrome",
				Executable:   "/fake/path/chrome/chrome",
				Channel:      "stable",
				IsSystem:     true,
				Architecture: "amd64",
			},
		},
	}
	m.AttachSystem(mock)

	list, err := m.ListWithSystem()
	if err != nil {
		t.Fatalf("ListWithSystem() error: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("ListWithSystem() = %d items, want 1", len(list))
	}

	v := list[0]
	if v.Browser != "chrome" {
		t.Errorf("Browser = %q, want chrome", v.Browser)
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", v.Version)
	}
	if !v.IsSystem {
		t.Error("IsSystem should be true")
	}
	if v.Source != "system" {
		t.Errorf("Source = %q, want system", v.Source)
	}
	if v.Channel != "stable" {
		t.Errorf("Channel = %q, want stable", v.Channel)
	}
}

func TestListWithSystemByBrowser(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:  "chrome",
				Version:  "120.0.0.0",
				Channel:  "stable",
				IsSystem: true,
			},
			{
				Browser:  "chrome",
				Version:  "121.0.0.0",
				Channel:  "beta",
				IsSystem: true,
			},
		},
	}
	m.AttachSystem(mock)

	list, err := m.ListWithSystemByBrowser("chrome")
	if err != nil {
		t.Fatalf("ListWithSystemByBrowser() error: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("got %d versions, want 2", len(list))
	}
}

func TestIsSystemVersion(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{Browser: "chrome", Version: "120.0.0.0", Channel: "stable", IsSystem: true},
		},
	}
	m.AttachSystem(mock)

	if !m.IsSystemVersion("chrome", "120.0.0.0") {
		t.Error("IsSystemVersion should be true for system browser")
	}
	if m.IsSystemVersion("chrome", "999.0.0.0") {
		t.Error("IsSystemVersion should be false for non-existent version")
	}
	if m.IsSystemVersion("firefox", "120.0.0.0") {
		t.Error("IsSystemVersion should be false for wrong browser")
	}
}

func TestIsSystemVersion_NoDetector(t *testing.T) {
	m := newTestManager(t)
	if m.IsSystemVersion("chrome", "120.0.0.0") {
		t.Error("IsSystemVersion should be false without detector")
	}
}

func TestFindSystemByVersion(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:    "chrome",
				Version:    "120.0.0.0",
				Channel:    "stable",
				Executable: "/usr/bin/chrome",
				IsSystem:   true,
			},
		},
	}
	m.AttachSystem(mock)

	info, found := m.FindSystemByVersion("chrome", "120.0.0.0")
	if !found {
		t.Fatal("FindSystemByVersion should find the system browser")
	}
	if info.Executable != "/usr/bin/chrome" {
		t.Errorf("Executable = %q", info.Executable)
	}

	_, found = m.FindSystemByVersion("chrome", "999.0.0.0")
	if found {
		t.Error("FindSystemByVersion should not find non-existent version")
	}
}

func TestFindSystemByVersion_NoDetector(t *testing.T) {
	m := newTestManager(t)
	_, found := m.FindSystemByVersion("chrome", "120.0.0.0")
	if found {
		t.Error("FindSystemByVersion should return false without detector")
	}
}

func TestGetSystemDefault(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:    "chrome",
				Version:    "120.0.0.0",
				Channel:    "stable",
				Executable: "/usr/bin/chrome",
				IsSystem:   true,
			},
		},
	}
	m.AttachSystem(mock)

	info, found := m.GetSystemDefault("chrome")
	if !found {
		t.Fatal("GetSystemDefault should find stable channel")
	}
	if info.Channel != "stable" {
		t.Errorf("Channel = %q, want stable", info.Channel)
	}
}

func TestGetSystemDefault_NoDetector(t *testing.T) {
	m := newTestManager(t)
	_, found := m.GetSystemDefault("chrome")
	if found {
		t.Error("GetSystemDefault should return false without detector")
	}
}

func TestGetExecutableWithSystem_SystemBrowser(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:    "chrome",
				Version:    "120.0.0.0",
				Channel:    "stable",
				Executable: "/usr/bin/google-chrome",
				IsSystem:   true,
			},
		},
	}
	m.AttachSystem(mock)

	path, found := m.GetExecutableWithSystem("chrome", "120.0.0.0")
	if !found {
		t.Fatal("GetExecutableWithSystem should find system browser")
	}
	if path != "/usr/bin/google-chrome" {
		t.Errorf("path = %q", path)
	}
}

func TestGetExecutableWithSystem_NotFound(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{}
	m.AttachSystem(mock)

	_, found := m.GetExecutableWithSystem("chrome", "999.0.0.0")
	if found {
		t.Error("GetExecutableWithSystem should return false for non-existent version")
	}
}

func TestGetRecordWithSystem_SystemBrowser(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:     "chrome",
				Version:     "120.0.0.0",
				Channel:     "stable",
				InstallPath: "/usr/bin",
				Executable:  "/usr/bin/google-chrome",
				IsSystem:    true,
			},
		},
	}
	m.AttachSystem(mock)

	record, found := m.GetRecordWithSystem("chrome", "120.0.0.0")
	if !found {
		t.Fatal("GetRecordWithSystem should find system browser")
	}
	if !record.IsSystem {
		t.Error("record.IsSystem should be true")
	}
	if record.Channel != "stable" {
		t.Errorf("Channel = %q", record.Channel)
	}
	if record.Source != "system" {
		t.Errorf("Source = %q", record.Source)
	}
}

func TestGetRecordWithSystem_NotFound(t *testing.T) {
	m := newTestManager(t)
	mock := &mockSystemDetector{}
	m.AttachSystem(mock)

	_, found := m.GetRecordWithSystem("chrome", "999.0.0.0")
	if found {
		t.Error("GetRecordWithSystem should return false for non-existent version")
	}
}

func TestSystemBrowserToVersion(t *testing.T) {
	sb := system.BrowserInfo{
		Browser:      "chrome",
		DisplayName:  "Google Chrome",
		Version:      "120.0.6099.109",
		InstallPath:  "/opt/chrome",
		Executable:   "/opt/chrome/chrome",
		Channel:      "stable",
		IsSystem:     true,
		Architecture: "amd64",
	}

	v := systemBrowserToVersion(sb)

	if v.Browser != "chrome" {
		t.Errorf("Browser = %q", v.Browser)
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", v.Version)
	}
	if v.Channel != "stable" {
		t.Errorf("Channel = %q", v.Channel)
	}
	if v.Arch != "amd64" {
		t.Errorf("Arch = %q", v.Arch)
	}
	if v.Source != "system" {
		t.Errorf("Source = %q", v.Source)
	}
	if !v.IsSystem {
		t.Error("IsSystem should be true")
	}
	if v.MajorVersion != 120 {
		t.Errorf("MajorVersion = %d, want 120", v.MajorVersion)
	}
}

func TestSystemBrowserToRecord(t *testing.T) {
	sb := system.BrowserInfo{
		Browser:     "chrome",
		Version:     "120.0.6099.109",
		InstallPath: "/opt/chrome",
		Executable:  "/opt/chrome/chrome",
		Channel:     "stable",
		IsSystem:    true,
	}

	r := systemBrowserToRecord(sb)

	if r.Browser != "chrome" {
		t.Errorf("Browser = %q", r.Browser)
	}
	if r.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", r.Version)
	}
	if r.InstallDir != "/opt/chrome" {
		t.Errorf("InstallDir = %q", r.InstallDir)
	}
	if r.ExecutablePath != "chrome" {
		t.Errorf("ExecutablePath = %q, want chrome", r.ExecutablePath)
	}
	if !r.IsSystem {
		t.Error("IsSystem should be true")
	}
	if r.Channel != "stable" {
		t.Errorf("Channel = %q", r.Channel)
	}
	if r.Source != "system" {
		t.Errorf("Source = %q", r.Source)
	}
}

func TestVersionIsSystemConsistency(t *testing.T) {
	// Verify that a non-system version has IsSystem=false by default
	v := version.Version{
		Browser: "chrome",
		Version: "120.0.0.0",
	}
	if v.IsSystem {
		t.Error("default Version.IsSystem should be false")
	}

	record := version.InstallRecord{
		Browser: "chrome",
		Version: "120.0.0.0",
	}
	if record.IsSystem {
		t.Error("default InstallRecord.IsSystem should be false")
	}
}
