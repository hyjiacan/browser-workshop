package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry

	// Should have 3 built-in browsers
	names := r.Names()
	if len(names) != 3 {
		t.Errorf("DefaultRegistry has %d browsers, want 3", len(names))
	}

	// Each built-in browser should exist
	for _, name := range []string{"chrome", "firefox", "chromium"} {
		if !r.Has(name) {
			t.Errorf("DefaultRegistry missing browser: %s", name)
		}
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	desc := &BrowserDescriptor{
		Name:        "test-browser",
		DisplayName: "Test Browser",
	}

	r.Register(desc)

	if !r.Has("test-browser") {
		t.Fatal("Has() returned false after Register")
	}

	got := r.Get("test-browser")
	if got == nil {
		t.Fatal("Get() returned nil after Register")
	}
	if got.Name != "test-browser" {
		t.Errorf("Get().Name = %q, want 'test-browser'", got.Name)
	}
}

func TestGet_NonExistent(t *testing.T) {
	r := NewRegistry()
	if r.Get("nonexistent") != nil {
		t.Error("Get() for non-existent browser should return nil")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()

	r.Register(&BrowserDescriptor{Name: "a"})
	r.Register(&BrowserDescriptor{Name: "b"})
	r.Register(&BrowserDescriptor{Name: "c"})

	list := r.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d items, want 3", len(list))
	}
}

func TestNames(t *testing.T) {
	r := NewRegistry()

	r.Register(&BrowserDescriptor{Name: "alpha"})
	r.Register(&BrowserDescriptor{Name: "beta"})

	names := r.Names()
	if len(names) != 2 {
		t.Errorf("Names() returned %d items, want 2", len(names))
	}

	// Check both names are present
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("Names() = %v, missing expected names", names)
	}
}

func TestHas(t *testing.T) {
	r := NewRegistry()

	if r.Has("x") {
		t.Error("Has() returned true for unregistered browser")
	}

	r.Register(&BrowserDescriptor{Name: "x"})
	if !r.Has("x") {
		t.Error("Has() returned false after registration")
	}
}

func TestFindExecutable(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("skipping on unsupported platform")
	}

	r := NewRegistry()
	r.Register(&BrowserDescriptor{
		Name: "test",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {
				runtime.GOARCH: {"mybrowser" + exeExt()},
			},
		},
		Features: BrowserFeatures{SupportsProfile: true},
	})

	dir := t.TempDir()

	// No executable yet
	_, err := r.FindExecutable("test", dir, runtime.GOOS, runtime.GOARCH)
	if err == nil {
		t.Error("FindExecutable() should error when no executable exists")
	}

	// Create the executable
	exePath := filepath.Join(dir, "mybrowser"+exeExt())
	if err := os.WriteFile(exePath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Should find it now
	relPath, err := r.FindExecutable("test", dir, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("FindExecutable() error = %v", err)
	}
	if relPath != "mybrowser"+exeExt() {
		t.Errorf("FindExecutable() = %q, want 'mybrowser%s'", relPath, exeExt())
	}
}

func TestFindExecutable_Recursive(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("skipping on unsupported platform")
	}

	r := NewRegistry()
	r.Register(&BrowserDescriptor{
		Name: "test",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {
				runtime.GOARCH: {"deepbrowser" + exeExt()},
			},
		},
		Features: BrowserFeatures{SupportsProfile: true},
	})

	dir := t.TempDir()

	// Create executable in a subdirectory
	subDir := filepath.Join(dir, "sub", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(subDir, "deepbrowser"+exeExt())
	if err := os.WriteFile(exePath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	relPath, err := r.FindExecutable("test", dir, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("FindExecutable() recursive error = %v", err)
	}
	expected := filepath.Join("sub", "nested", "deepbrowser"+exeExt())
	if relPath != expected {
		t.Errorf("FindExecutable() = %q, want %q", relPath, expected)
	}
}

func TestFindExecutable_UnknownBrowser(t *testing.T) {
	r := NewRegistry()
	_, err := r.FindExecutable("unknown", ".", "windows", "amd64")
	if err == nil {
		t.Error("FindExecutable() for unknown browser should error")
	}
}

func TestDetectBrowser(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("skipping on unsupported platform")
	}

	r := NewRegistry()
	r.Register(&BrowserDescriptor{
		Name: "alpha",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {
				runtime.GOARCH: {"alpha-browser" + exeExt()},
			},
		},
	})
	r.Register(&BrowserDescriptor{
		Name: "beta",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {
				runtime.GOARCH: {"beta-browser" + exeExt()},
			},
		},
	})

	// Create a directory with alpha's executable
	dir := t.TempDir()
	exePath := filepath.Join(dir, "alpha-browser"+exeExt())
	if err := os.WriteFile(exePath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	detected, err := r.DetectBrowser(dir, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("DetectBrowser() error = %v", err)
	}
	if detected != "alpha" {
		t.Errorf("DetectBrowser() = %q, want 'alpha'", detected)
	}
}

func TestDetectBrowser_Unrecognized(t *testing.T) {
	r := NewRegistry()
	r.Register(&BrowserDescriptor{
		Name: "test",
		ExecutableCandidates: map[string]map[string][]string{
			"windows": {"amd64": {"nonexistent.exe"}},
		},
	})

	dir := t.TempDir()
	_, err := r.DetectBrowser(dir, "windows", "amd64")
	if err == nil {
		t.Error("DetectBrowser() should error for unrecognized directory")
	}
}

func TestBuildProfileArgs(t *testing.T) {
	tests := []struct {
		name     string
		desc     *BrowserDescriptor
		profile  string
		expected []string
	}{
		{
			name: "chrome style (= separator)",
			desc: &BrowserDescriptor{
				ProfileArg:      "--user-data-dir=",
				ProfileSeparate: false,
				Features:        BrowserFeatures{SupportsProfile: true},
			},
			profile:  "/tmp/profile",
			expected: []string{"--user-data-dir=/tmp/profile"},
		},
		{
			name: "firefox style (separate args)",
			desc: &BrowserDescriptor{
				ProfileArg:      "-profile",
				ProfileSeparate: true,
				Features:        BrowserFeatures{SupportsProfile: true},
			},
			profile:  "/tmp/profile",
			expected: []string{"-profile", "/tmp/profile"},
		},
		{
			name: "no profile support",
			desc: &BrowserDescriptor{
				Features: BrowserFeatures{SupportsProfile: false},
			},
			profile:  "/tmp/profile",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.desc.BuildProfileArgs(tt.profile)
			if len(result) != len(tt.expected) {
				t.Fatalf("got %d args, want %d: %v", len(result), len(tt.expected), result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("arg[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestBuildStandardArgs(t *testing.T) {
	desc := &BrowserDescriptor{
		MultiInstanceArgs: []string{"--no-default-browser-check", "--no-first-run"},
		DisableUpdateArgs: []string{"--disable-update"},
		FirstRunSkipArgs:  []string{"--no-first-run"},
	}

	args := desc.BuildStandardArgs()
	expectedCount := len(desc.MultiInstanceArgs) + len(desc.DisableUpdateArgs) + len(desc.FirstRunSkipArgs)
	if len(args) != expectedCount {
		t.Errorf("BuildStandardArgs() returned %d args, want %d", len(args), expectedCount)
	}
}

func TestChromeDescriptor(t *testing.T) {
	if Chrome.Name != "chrome" {
		t.Errorf("Chrome.Name = %q, want 'chrome'", Chrome.Name)
	}
	if Chrome.DefaultChannel != "stable" {
		t.Errorf("Chrome.DefaultChannel = %q, want 'stable'", Chrome.DefaultChannel)
	}
	if !Chrome.Features.SupportsHeadless {
		t.Error("Chrome should support headless")
	}
	if !Chrome.Features.SupportsIncognito {
		t.Error("Chrome should support incognito")
	}
}

func TestFirefoxDescriptor(t *testing.T) {
	if Firefox.Name != "firefox" {
		t.Errorf("Firefox.Name = %q, want 'firefox'", Firefox.Name)
	}
	if !Firefox.ProfileSeparate {
		t.Error("Firefox should use separate profile args")
	}
}

func TestChromiumDescriptor(t *testing.T) {
	if Chromium.Name != "chromium" {
		t.Errorf("Chromium.Name = %q, want 'chromium'", Chromium.Name)
	}
}

func TestIncognitoArg(t *testing.T) {
	if Chrome.IncognitoArg() != "--incognito" {
		t.Errorf("Chrome incognito arg = %q", Chrome.IncognitoArg())
	}
	if Firefox.IncognitoArg() != "-private" {
		t.Errorf("Firefox incognito arg = %q", Firefox.IncognitoArg())
	}
}

func TestHeadlessArgs(t *testing.T) {
	chromeArgs := Chrome.HeadlessArgs()
	if len(chromeArgs) < 2 {
		t.Errorf("Chrome headless should have at least 2 args, got %d", len(chromeArgs))
	}

	firefoxArgs := Firefox.HeadlessArgs()
	if len(firefoxArgs) < 1 {
		t.Errorf("Firefox headless should have at least 1 arg, got %d", len(firefoxArgs))
	}
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
