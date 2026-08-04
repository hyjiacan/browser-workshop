package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/paths"
)

func setupTestLauncher(t *testing.T) (*Manager, string) {
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
			runtime.GOOS: {
				runtime.GOARCH: {exeName},
			},
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

	// Install a test version
	srcDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(srcDir, exeName)
	if err := os.WriteFile(exePath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := inst.InstallFromDir(install.InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}, nil)
	if err != nil {
		t.Fatalf("install test version error = %v", err)
	}

	return NewManager(p, reg, inst), root
}

func TestBuildCommandPreview(t *testing.T) {
	m, _ := setupTestLauncher(t)

	t.Run("basic launch", func(t *testing.T) {
		exe, args, err := m.BuildCommandPreview(Options{
			Browser: "test",
			Version: "1.0.0",
		})
		if err != nil {
			t.Fatalf("BuildCommandPreview() error = %v", err)
		}
		if exe == "" {
			t.Error("executable path is empty")
		}
		if len(args) == 0 {
			t.Error("args should not be empty")
		}

		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "--no-default-browser-check") {
			t.Errorf("args missing --no-default-browser-check: %v", args)
		}
		if !strings.Contains(argStr, "--disable-update") {
			t.Errorf("args missing --disable-update: %v", args)
		}
		if !strings.Contains(argStr, "--profile=") {
			t.Errorf("args missing profile arg: %v", args)
		}
	})

	t.Run("with URLs", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser: "test",
			Version: "1.0.0",
			URLs:    []string{"https://example.com", "https://test.com"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "https://example.com") {
			t.Errorf("args missing first URL: %v", args)
		}
		if !strings.Contains(argStr, "https://test.com") {
			t.Errorf("args missing second URL: %v", args)
		}
	})

	t.Run("headless mode", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser:  "test",
			Version:  "1.0.0",
			Headless: true,
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "--headless") {
			t.Errorf("headless args missing --headless: %v", args)
		}
	})

	t.Run("incognito mode", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser:   "test",
			Version:   "1.0.0",
			Incognito: true,
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "--incognito") {
			t.Errorf("incognito args missing flag: %v", args)
		}
	})

	t.Run("new window", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser:   "test",
			Version:   "1.0.0",
			NewWindow: true,
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "--new-window") {
			t.Errorf("args missing --new-window: %v", args)
		}
	})

	t.Run("extra args", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser:   "test",
			Version:   "1.0.0",
			ExtraArgs: []string{"--foo=bar", "--baz"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "--foo=bar") {
			t.Errorf("extra args missing: %v", args)
		}
		if !strings.Contains(argStr, "--baz") {
			t.Errorf("extra args missing: %v", args)
		}
	})

	t.Run("named profile", func(t *testing.T) {
		_, args, err := m.BuildCommandPreview(Options{
			Browser:     "test",
			Version:     "1.0.0",
			ProfileName: "my-profile",
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		argStr := strings.Join(args, " ")
		if !strings.Contains(argStr, "my-profile") {
			t.Errorf("named profile not in args: %v", args)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		_, _, err := m.BuildCommandPreview(Options{
			Browser: "test",
			Version: "9.9.9",
		})
		if err == nil {
			t.Error("should error for non-installed version")
		}
	})

	t.Run("unsupported browser", func(t *testing.T) {
		_, _, err := m.BuildCommandPreview(Options{
			Browser: "unknown",
			Version: "1.0.0",
		})
		if err == nil {
			t.Error("should error for unknown browser")
		}
	})

	t.Run("empty browser/version", func(t *testing.T) {
		_, _, err := m.BuildCommandPreview(Options{})
		if err == nil {
			t.Error("should error with empty options")
		}
	})
}

func TestGetProfileDir(t *testing.T) {
	m, root := setupTestLauncher(t)

	t.Run("default version profile", func(t *testing.T) {
		dir := m.getProfileDir(Options{Browser: "test", Version: "1.0.0"})
		expected := filepath.Join(root, "runtime", "test", "1.0.0", "profile")
		if dir != expected {
			t.Errorf("profile dir = %q, want %q", dir, expected)
		}
	})

	t.Run("named profile", func(t *testing.T) {
		dir := m.getProfileDir(Options{Browser: "test", Version: "1.0.0", ProfileName: "dev"})
		expected := filepath.Join(root, "runtime", "test", "profiles", "dev", "profile")
		if dir != expected {
			t.Errorf("named profile dir = %q, want %q", dir, expected)
		}
	})
}

func TestBuildArgs(t *testing.T) {
	m, _ := setupTestLauncher(t)
	desc := m.browsers.Get("test")
	profileDir := "/tmp/test-profile"

	t.Run("standard args", func(t *testing.T) {
		args, err := m.buildArgs(desc, Options{Browser: "test", Version: "1.0.0"}, profileDir, false)
		if err != nil {
			t.Fatal(err)
		}
		argStr := strings.Join(args, " ")

		if !strings.Contains(argStr, "--no-default-browser-check") {
			t.Errorf("missing multi-instance arg")
		}
		if !strings.Contains(argStr, "--disable-update") {
			t.Errorf("missing disable-update arg")
		}
		if !strings.Contains(argStr, "--profile="+profileDir) {
			t.Errorf("missing profile arg with path: %v", args)
		}
	})

	t.Run("all flags combined", func(t *testing.T) {
		args, err := m.buildArgs(desc, Options{
			Browser:   "test",
			Version:   "1.0.0",
			Headless:  true,
			Incognito: true,
			NewWindow: true,
			URLs:      []string{"https://a.com"},
			ExtraArgs: []string{"--extra"},
		}, profileDir, false)
		if err != nil {
			t.Fatal(err)
		}
		argStr := strings.Join(args, " ")

		checks := []string{
			"--headless",
			"--incognito",
			"--new-window",
			"https://a.com",
			"--extra",
			"--profile=",
		}
		for _, c := range checks {
			if !strings.Contains(argStr, c) {
				t.Errorf("missing expected arg/string: %s", c)
			}
		}
	})

	t.Run("extra args come last", func(t *testing.T) {
		args, err := m.buildArgs(desc, Options{
			Browser:   "test",
			Version:   "1.0.0",
			URLs:      []string{"https://example.com"},
			ExtraArgs: []string{"--last-arg"},
		}, profileDir, false)
		if err != nil {
			t.Fatal(err)
		}

		lastArg := args[len(args)-1]
		if lastArg != "--last-arg" {
			t.Errorf("last arg = %q, want '--last-arg' (extra args should be last)", lastArg)
		}
	})
}

func TestWriteStandardPrefs(t *testing.T) {
	desc := &browser.BrowserDescriptor{
		Name: "firefox",
		StandardPrefs: []string{
			`user_pref("app.update.enabled", false);`,
			`user_pref("browser.shell.checkDefaultBrowser", false);`,
		},
	}

	t.Run("writes prefs to user.js", func(t *testing.T) {
		dir := t.TempDir()
		err := writeStandardPrefs(desc, dir)
		if err != nil {
			t.Fatalf("writeStandardPrefs() error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "user.js"))
		if err != nil {
			t.Fatalf("reading user.js: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "Standard prefs written by bws") {
			t.Error("missing marker comment")
		}
		if !strings.Contains(content, `user_pref("app.update.enabled", false)`) {
			t.Error("missing app.update.enabled pref")
		}
		if !strings.Contains(content, `user_pref("browser.shell.checkDefaultBrowser", false)`) {
			t.Error("missing browser.shell.checkDefaultBrowser pref")
		}
	})

	t.Run("idempotent - does not duplicate", func(t *testing.T) {
		dir := t.TempDir()

		// First write
		if err := writeStandardPrefs(desc, dir); err != nil {
			t.Fatal(err)
		}
		data1, _ := os.ReadFile(filepath.Join(dir, "user.js"))

		// Second write should be a no-op
		if err := writeStandardPrefs(desc, dir); err != nil {
			t.Fatal(err)
		}
		data2, _ := os.ReadFile(filepath.Join(dir, "user.js"))

		if string(data1) != string(data2) {
			t.Error("second writeStandardPrefs should not modify the file")
		}
	})

	t.Run("preserves existing content", func(t *testing.T) {
		dir := t.TempDir()
		prefsPath := filepath.Join(dir, "user.js")
		existing := `user_pref("some.existing.pref", true);`
		os.WriteFile(prefsPath, []byte(existing), 0o644)

		if err := writeStandardPrefs(desc, dir); err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(prefsPath)
		content := string(data)
		if !strings.Contains(content, "some.existing.pref") {
			t.Error("existing prefs should be preserved")
		}
		if !strings.Contains(content, "Standard prefs written by bws") {
			t.Error("standard prefs should be prepended")
		}
	})

	t.Run("no prefs - no-op", func(t *testing.T) {
		dir := t.TempDir()
		emptyDesc := &browser.BrowserDescriptor{Name: "test"}
		err := writeStandardPrefs(emptyDesc, dir)
		if err != nil {
			t.Errorf("writeStandardPrefs with no prefs should not error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "user.js")); !os.IsNotExist(err) {
			t.Error("user.js should not be created when StandardPrefs is empty")
		}
	})
}

func TestFirefoxStandardPrefs(t *testing.T) {
	// Verify that Firefox descriptor has standard prefs for disabling
	// updates and default browser check.
	if len(browser.Firefox.StandardPrefs) == 0 {
		t.Fatal("Firefox should have StandardPrefs")
	}

	prefsStr := strings.Join(browser.Firefox.StandardPrefs, "\n")
	if !strings.Contains(prefsStr, "app.update.enabled") {
		t.Error("Firefox StandardPrefs should disable app.update.enabled")
	}
	if !strings.Contains(prefsStr, "browser.shell.checkDefaultBrowser") {
		t.Error("Firefox StandardPrefs should disable browser.shell.checkDefaultBrowser")
	}
}

func TestChromiumDisableUpdate(t *testing.T) {
	// Verify that Chromium has --disable-update in DisableUpdateArgs.
	found := false
	for _, arg := range browser.Chromium.DisableUpdateArgs {
		if arg == "--disable-update" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Chromium should have --disable-update in DisableUpdateArgs")
	}
}

func TestLaunch_NotInstalled(t *testing.T) {
	m, _ := setupTestLauncher(t)

	_, err := m.Launch(Options{
		Browser: "test",
		Version: "9.9.9",
	})
	if err == nil {
		t.Error("Launch() should fail for non-installed version")
	}
}

func TestLaunch_EmptyParams(t *testing.T) {
	m, _ := setupTestLauncher(t)

	_, err := m.Launch(Options{})
	if err == nil {
		t.Error("Launch() should fail with empty params")
	}
}

func TestProcessStruct(t *testing.T) {
	p := &Process{
		Pid:        12345,
		Executable: "/path/to/browser",
		Args:       []string{"--foo", "--bar"},
		ProfileDir: "/path/to/profile",
	}

	if p.Pid != 12345 {
		t.Errorf("Pid = %d", p.Pid)
	}
	if p.Executable != "/path/to/browser" {
		t.Errorf("Executable = %q", p.Executable)
	}
	if len(p.Args) != 2 {
		t.Errorf("Args count = %d", len(p.Args))
	}
	if p.ProfileDir != "/path/to/profile" {
		t.Errorf("ProfileDir = %q", p.ProfileDir)
	}

	// Test Kill and Wait with nil Cmd should error
	if err := p.Kill(); err == nil {
		t.Error("Kill() with nil Cmd should error")
	}
	if err := p.Wait(); err == nil {
		t.Error("Wait() with nil Cmd should error")
	}
}

func TestSetDetached(t *testing.T) {
	// Test that setDetached doesn't panic and sets SysProcAttr
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "echo", "test")
	} else {
		cmd = exec.Command("echo", "test")
	}

	setDetached(cmd)
	if cmd.SysProcAttr == nil {
		t.Error("SysProcAttr should be set after setDetached")
	}
}
