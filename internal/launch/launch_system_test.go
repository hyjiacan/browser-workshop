package launch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/system"
)

// mockSystemDetector implements install.SystemDetector for testing.
type mockSystemDetector struct {
	browsers []system.BrowserInfo
}

func (m *mockSystemDetector) DetectAll() []system.BrowserInfo { return m.browsers }
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
func (m *mockSystemDetector) Refresh() []system.BrowserInfo { return m.browsers }
func (m *mockSystemDetector) InvalidateCache()               {}

func setupSystemTestLauncher(t *testing.T) (*Manager, *mockSystemDetector, string) {
	t.Helper()
	root := t.TempDir()
	p := paths.New(root)
	if err := p.EnsureAll(); err != nil {
		t.Fatal(err)
	}

	exeName := "test-browser"
	if runtime.GOOS == "windows" {
		exeName = "test-browser.exe"
	}

	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "test",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {exeName}},
		},
		ProfileArg:      "--profile=",
		ProfileSeparate: false,
		MultiInstanceArgs: []string{
			"--no-default-browser-check",
			"--no-first-run",
		},
		DisableUpdateArgs: []string{"--disable-update"},
		FirstRunSkipArgs:  []string{"--no-first-run"},
		Features: browser.BrowserFeatures{
			SupportsHeadless:  true,
			SupportsIncognito: true,
			SupportsProfile:   true,
			CanMultiInstance:  true,
		},
	})

	inst := install.NewManager(p, reg)

	// Create a fake system browser executable
	sysExeDir := filepath.Join(t.TempDir(), "system-browser")
	os.MkdirAll(sysExeDir, 0o755)
	sysExePath := filepath.Join(sysExeDir, exeName)
	os.WriteFile(sysExePath, []byte("fake"), 0o755)

	mock := &mockSystemDetector{
		browsers: []system.BrowserInfo{
			{
				Browser:      "test",
				DisplayName:  "Test Browser",
				Version:      "2.0.0",
				InstallPath:  sysExeDir,
				Executable:   sysExePath,
				Channel:      "stable",
				IsSystem:     true,
				Architecture: runtime.GOARCH,
			},
		},
	}
	inst.AttachSystem(mock)

	return NewManager(p, reg, inst), mock, root
}

func TestBuildCommandPreview_SystemBrowser(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	exe, args, err := m.BuildCommandPreview(Options{
		Browser: "test",
		Version: "2.0.0",
	})
	if err != nil {
		t.Fatalf("BuildCommandPreview() error = %v", err)
	}
	if exe == "" {
		t.Error("executable path is empty")
	}

	argStr := strings.Join(args, " ")

	// System browser in native mode should NOT have isolation flags
	if strings.Contains(argStr, "--no-default-browser-check") {
		t.Errorf("native mode should not have --no-default-browser-check: %v", args)
	}
	if strings.Contains(argStr, "--disable-update") {
		t.Errorf("native mode should not have --disable-update: %v", args)
	}
	if strings.Contains(argStr, "--profile=") {
		t.Errorf("native mode should not have profile arg: %v", args)
	}
}

func TestBuildCommandPreview_SystemBrowserWithURLs(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser: "test",
		Version: "2.0.0",
		URLs:    []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "https://example.com") {
		t.Errorf("args missing URL: %v", args)
	}
}

func TestBuildCommandPreview_SystemBrowserHeadless(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser:  "test",
		Version:  "2.0.0",
		Headless: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	// Headless should still work in native mode
	if !strings.Contains(argStr, "--headless") {
		t.Errorf("args missing --headless: %v", args)
	}
}

func TestBuildCommandPreview_SystemBrowserIncognito(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser:   "test",
		Version:   "2.0.0",
		Incognito: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "--incognito") {
		t.Errorf("args missing --incognito: %v", args)
	}
}

func TestBuildCommandPreview_SystemBrowserWithExtraArgs(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser:   "test",
		Version:   "2.0.0",
		ExtraArgs: []string{"--custom-flag", "value"},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "--custom-flag") {
		t.Errorf("args missing custom flag: %v", args)
	}
	if !strings.Contains(argStr, "value") {
		t.Errorf("args missing value: %v", args)
	}
}

func TestBuildCommandPreview_LocalVersionNormalMode(t *testing.T) {
	m, _ := setupTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser: "test",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	// Local version should have isolation flags
	if !strings.Contains(argStr, "--no-default-browser-check") {
		t.Errorf("local version should have --no-default-browser-check: %v", args)
	}
	if !strings.Contains(argStr, "--profile=") {
		t.Errorf("local version should have profile arg: %v", args)
	}
}

func TestBuildCommandPreview_LocalVersionNativeMode(t *testing.T) {
	m, _ := setupTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser:    "test",
		Version:    "1.0.0",
		NativeMode: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	// Explicit native mode for local version should skip isolation flags
	if strings.Contains(argStr, "--no-default-browser-check") {
		t.Errorf("native mode should skip --no-default-browser-check: %v", args)
	}
	if strings.Contains(argStr, "--profile=") {
		t.Errorf("native mode should skip profile arg: %v", args)
	}
}

func TestBuildCommandPreview_SystemBrowserNotFound(t *testing.T) {
	m, mock, _ := setupSystemTestLauncher(t)
	mock.browsers = nil // remove all system browsers

	_, _, err := m.BuildCommandPreview(Options{
		Browser: "test",
		Version: "2.0.0",
	})
	if err == nil {
		t.Error("expected error for non-existent system browser version")
	}
}

func TestBuildCommandPreview_NewWindow(t *testing.T) {
	m, _, _ := setupSystemTestLauncher(t)

	_, args, err := m.BuildCommandPreview(Options{
		Browser:   "test",
		Version:   "2.0.0",
		NewWindow: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "--new-window") {
		t.Errorf("args missing --new-window: %v", args)
	}
}

func TestLaunchOptions_Defaults(t *testing.T) {
	opts := Options{
		Browser: "test",
		Version: "1.0.0",
	}

	if opts.NativeMode {
		t.Error("NativeMode should default to false")
	}
	if opts.Detached {
		t.Error("Detached should default to false")
	}
	if len(opts.ExtraArgs) != 0 {
		t.Error("ExtraArgs should default to empty")
	}
}

func TestProcess_Defaults(t *testing.T) {
	p := &Process{}

	if p.IsSystem {
		t.Error("IsSystem should default to false")
	}
	if p.NativeMode {
		t.Error("NativeMode should default to false")
	}
}
