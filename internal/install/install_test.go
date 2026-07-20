package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	p := paths.New(root)
	if err := p.EnsureAll(); err != nil {
		t.Fatal(err)
	}

	// Use a test registry with a simple test browser
	reg := browser.NewRegistry()
	exeName := "test-browser"
	if runtime.GOOS == "windows" {
		exeName = "test-browser.exe"
	}
	reg.Register(&browser.BrowserDescriptor{
		Name: "test",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {
				runtime.GOARCH: {exeName},
			},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})

	return NewManager(p, reg), root
}

func createFakeBrowser(t *testing.T, dir string) string {
	t.Helper()
	exeName := "test-browser"
	if runtime.GOOS == "windows" {
		exeName = "test-browser.exe"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(dir, exeName)
	if err := os.WriteFile(exePath, []byte("fake browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exePath
}

func TestIsInstalled(t *testing.T) {
	m, _ := setupTestManager(t)

	if m.IsInstalled("test", "1.0.0") {
		t.Error("IsInstalled should return false for non-existent version")
	}
}

func TestInstallFromDir(t *testing.T) {
	m, _ := setupTestManager(t)

	// Create a fake browser source directory
	srcDir := filepath.Join(t.TempDir(), "source-browser")
	createFakeBrowser(t, srcDir)

	// Also add some extra files
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("some data"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}

	var progressCalls []float64
	record, err := m.InstallFromDir(opts, func(p float64, msg string) {
		progressCalls = append(progressCalls, p)
	})
	if err != nil {
		t.Fatalf("InstallFromDir() error = %v", err)
	}

	// Verify record
	if record.Browser != "test" {
		t.Errorf("record.Browser = %q, want 'test'", record.Browser)
	}
	if record.Version != "1.0.0" {
		t.Errorf("record.Version = %q, want '1.0.0'", record.Version)
	}
	if record.Source != "test" {
		t.Errorf("record.Source = %q, want 'test'", record.Source)
	}
	if record.Platform != paths.Platform() {
		t.Errorf("record.Platform = %q, want %q", record.Platform, paths.Platform())
	}
	if record.Size == 0 {
		t.Error("record.Size should be > 0")
	}

	// Verify is installed
	if !m.IsInstalled("test", "1.0.0") {
		t.Error("IsInstalled should return true after installation")
	}

	// Verify files were copied
	verDir := m.paths.VersionDir("test", "1.0.0")
	exeName := "test-browser"
	if runtime.GOOS == "windows" {
		exeName = "test-browser.exe"
	}
	if _, err := os.Stat(filepath.Join(verDir, exeName)); err != nil {
		t.Errorf("executable not found at destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(verDir, "data.txt")); err != nil {
		t.Errorf("data file not copied: %v", err)
	}

	// Verify .bws.json exists
	metaPath := m.paths.VersionMetaFile("test", "1.0.0")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("metadata file not found: %v", err)
	}

	// Progress should have been called
	if len(progressCalls) == 0 {
		t.Error("progress callback was never called")
	}
}

func TestInstallFromDir_AlreadyInstalled(t *testing.T) {
	m, _ := setupTestManager(t)

	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)

	opts := InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}

	// First install
	_, err := m.InstallFromDir(opts, nil)
	if err != nil {
		t.Fatalf("first install error = %v", err)
	}

	// Second install should fail
	_, err = m.InstallFromDir(opts, nil)
	if err == nil {
		t.Error("second install should fail with 'already installed'")
	}
}

func TestInstallFromDir_InvalidSource(t *testing.T) {
	m, _ := setupTestManager(t)

	// Non-existent source directory
	opts := InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: "/nonexistent/path",
	}

	_, err := m.InstallFromDir(opts, nil)
	if err == nil {
		t.Error("should fail with non-existent source directory")
	}

	// Source without executable
	srcDir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	opts.SourceDir = srcDir
	_, err = m.InstallFromDir(opts, nil)
	if err == nil {
		t.Error("should fail when no executable in source")
	}
}

func TestInstallFromDir_UnsupportedBrowser(t *testing.T) {
	m, _ := setupTestManager(t)

	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)

	opts := InstallOptions{
		Browser:   "unknown",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}

	_, err := m.InstallFromDir(opts, nil)
	if err == nil {
		t.Error("should fail for unsupported browser")
	}
}

func TestUninstall(t *testing.T) {
	m, _ := setupTestManager(t)

	// Install first
	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)

	opts := InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}

	_, err := m.InstallFromDir(opts, nil)
	if err != nil {
		t.Fatalf("install error = %v", err)
	}

	if !m.IsInstalled("test", "1.0.0") {
		t.Fatal("should be installed before uninstall")
	}

	// Uninstall
	if err := m.Uninstall("test", "1.0.0"); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	if m.IsInstalled("test", "1.0.0") {
		t.Error("should not be installed after uninstall")
	}

	// Verify directory is gone
	verDir := m.paths.VersionDir("test", "1.0.0")
	if _, err := os.Stat(verDir); !os.IsNotExist(err) {
		t.Error("version directory should be removed after uninstall")
	}
}

func TestUninstall_NotInstalled(t *testing.T) {
	m, _ := setupTestManager(t)

	err := m.Uninstall("test", "9.9.9")
	if err == nil {
		t.Error("Uninstall() for non-existent version should fail")
	}
}

func TestListInstalled(t *testing.T) {
	m, _ := setupTestManager(t)

	// Install two versions
	for _, v := range []string{"1.0.0", "2.0.0"} {
		srcDir := filepath.Join(t.TempDir(), "source-"+v)
		createFakeBrowser(t, srcDir)
		_, err := m.InstallFromDir(InstallOptions{
			Browser:   "test",
			Version:   v,
			Source:    "test",
			SourceDir: srcDir,
		}, nil)
		if err != nil {
			t.Fatalf("install %s error = %v", v, err)
		}
	}

	list, err := m.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListInstalled() = %d versions, want 2", len(list))
	}
}

func TestListInstalled_Empty(t *testing.T) {
	m, _ := setupTestManager(t)

	list, err := m.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("empty install list should have 0 items, got %d", len(list))
	}
}

func TestListInstalledByBrowser(t *testing.T) {
	m, _ := setupTestManager(t)

	// Install a version
	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)
	_, err := m.InstallFromDir(InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// List for our test browser
	list, err := m.ListInstalledByBrowser("test")
	if err != nil {
		t.Fatalf("ListInstalledByBrowser() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListInstalledByBrowser('test') = %d, want 1", len(list))
	}

	// List for non-existent browser
	list, err = m.ListInstalledByBrowser("nonexistent")
	if err != nil {
		t.Fatalf("ListInstalledByBrowser('nonexistent') error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListInstalledByBrowser('nonexistent') = %d, want 0", len(list))
	}
}

func TestGetRecord(t *testing.T) {
	m, _ := setupTestManager(t)

	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)
	_, err := m.InstallFromDir(InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	record, err := m.GetRecord("test", "1.0.0")
	if err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}
	if record.Browser != "test" {
		t.Errorf("record.Browser = %q", record.Browser)
	}
	if record.Version != "1.0.0" {
		t.Errorf("record.Version = %q", record.Version)
	}
	if record.InstalledAt.IsZero() {
		t.Error("record.InstalledAt should be set")
	}
}

func TestGetRecord_NotInstalled(t *testing.T) {
	m, _ := setupTestManager(t)

	_, err := m.GetRecord("test", "9.9.9")
	if err == nil {
		t.Error("GetRecord() for non-installed version should fail")
	}
}

func TestGetExecutablePath(t *testing.T) {
	m, _ := setupTestManager(t)

	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)
	_, err := m.InstallFromDir(InstallOptions{
		Browser:   "test",
		Version:   "1.0.0",
		Source:    "test",
		SourceDir: srcDir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	exePath, err := m.GetExecutablePath("test", "1.0.0")
	if err != nil {
		t.Fatalf("GetExecutablePath() error = %v", err)
	}

	if _, err := os.Stat(exePath); err != nil {
		t.Errorf("executable path does not exist: %v", err)
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 100), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), make([]byte, 200), 0o644)
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "c.txt"), make([]byte, 300), 0o644)

	size, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize() error = %v", err)
	}
	if size != 600 {
		t.Errorf("dirSize() = %d, want 600", size)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create source structure
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello"), 0o644)
	sub := filepath.Join(src, "subdir")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "file2.txt"), []byte("world"), 0o644)

	var totalCopied int64
	err := copyDir(src, dst, func(fileName string, n int64) {
		totalCopied += n
	})
	if err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	// Verify files
	if _, err := os.Stat(filepath.Join(dst, "file1.txt")); err != nil {
		t.Errorf("file1 not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "subdir", "file2.txt")); err != nil {
		t.Errorf("file2 not copied: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("file1 content = %q, want 'hello'", string(data))
	}

	// Verify callback
	if totalCopied != 10 { // "hello" (5) + "world" (5) = 10
		t.Errorf("total copied via callback = %d, want 10", totalCopied)
	}
}

func TestInstallFromDir_MultipleVersions(t *testing.T) {
	m, _ := setupTestManager(t)

	versions := []string{"1.0.0", "1.5.0", "2.0.0"}
	for _, v := range versions {
		srcDir := filepath.Join(t.TempDir(), "src-"+v)
		createFakeBrowser(t, srcDir)
		_, err := m.InstallFromDir(InstallOptions{
			Browser:   "test",
			Version:   v,
			Source:    "test",
			SourceDir: srcDir,
		}, nil)
		if err != nil {
			t.Fatalf("install %s error = %v", v, err)
		}
	}

	list, err := m.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("installed = %d versions, want 3", len(list))
	}

	// Check that version.Major is correctly set
	for _, v := range list {
		if v.MajorVersion == 0 {
			t.Errorf("MajorVersion should be set for version %q", v.Version)
		}
	}
}

func TestInstallRecord_Roundtrip(t *testing.T) {
	m, _ := setupTestManager(t)

	srcDir := filepath.Join(t.TempDir(), "source")
	createFakeBrowser(t, srcDir)

	original := &version.InstallRecord{
		Browser:        "test",
		Version:        "1.0.0",
		Platform:       paths.Platform(),
		Arch:           paths.Arch(),
		Source:         "unit-test",
		ExecutablePath: "test-browser",
	}

	// Write and read back
	metaPath := filepath.Join(t.TempDir(), ".bws.json")
	if err := writeMeta(metaPath, original); err != nil {
		t.Fatalf("writeMeta() error = %v", err)
	}

	// Read via manager
	// We need to set up the directory structure properly
	verDir := m.paths.VersionDir("test", "1.0.0")
	os.MkdirAll(verDir, 0o755)
	metaFile := m.paths.VersionMetaFile("test", "1.0.0")
	os.WriteFile(metaFile, []byte{}, 0o644)
	writeMeta(metaFile, original)

	record, err := m.GetRecord("test", "1.0.0")
	if err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}

	if record.Browser != original.Browser {
		t.Errorf("Browser = %q, want %q", record.Browser, original.Browser)
	}
	if record.Version != original.Version {
		t.Errorf("Version = %q, want %q", record.Version, original.Version)
	}
	if record.Source != original.Source {
		t.Errorf("Source = %q, want %q", record.Source, original.Source)
	}
}
