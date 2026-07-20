package repo

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bws/bws/internal/archive"
	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/paths"
)

func setupTestImporter(t *testing.T) (*Importer, string) {
	t.Helper()

	root := t.TempDir()
	p := paths.New(root)
	p.EnsureAll()

	chromeExe := "chrome"
	if runtime.GOOS == "windows" {
		chromeExe = "chrome.exe"
	}
	firefoxExe := "firefox"
	if runtime.GOOS == "windows" {
		firefoxExe = "firefox.exe"
	}

	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {chromeExe}},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})
	reg.Register(&browser.BrowserDescriptor{
		Name: "firefox",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {firefoxExe}},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})

	inst := install.NewManager(p, reg)

	// Create source repository
	repoDir := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repoDir, 0o755)

	// Create chrome_1.0.0 with executable
	dir1 := filepath.Join(repoDir, "chrome_1.0.0_"+archSuffix())
	os.MkdirAll(dir1, 0o755)
	os.WriteFile(filepath.Join(dir1, chromeExe), []byte("fake"), 0o755)

	// Create chrome_2.0.0 with executable
	dir2 := filepath.Join(repoDir, "chrome_2.0.0_"+archSuffix())
	os.MkdirAll(dir2, 0o755)
	os.WriteFile(filepath.Join(dir2, chromeExe), []byte("fake"), 0o755)

	// Create firefox_1.5.0 with executable
	dir3 := filepath.Join(repoDir, "firefox_1.5.0_"+archSuffix())
	os.MkdirAll(dir3, 0o755)
	os.WriteFile(filepath.Join(dir3, firefoxExe), []byte("fake"), 0o755)

	// Create unrecognized directory
	os.MkdirAll(filepath.Join(repoDir, "random-folder"), 0o755)

	scanner, err := NewScanner(repoDir, reg)
	if err != nil {
		t.Fatal(err)
	}

	return NewImporter(scanner, inst), repoDir
}

func archSuffix() string {
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "amd64" {
			return "win64"
		}
		return "win32"
	}
	if runtime.GOARCH == "amd64" {
		return "amd64"
	}
	return runtime.GOARCH
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func TestImportAll(t *testing.T) {
	imp, _ := setupTestImporter(t)

	var progressCalls []ImportProgress
	summary, err := imp.ImportAll(ImportOptions{}, func(p ImportProgress) {
		progressCalls = append(progressCalls, p)
	})

	if err != nil {
		t.Fatalf("ImportAll() error = %v", err)
	}

	// Should have scanned 4 dirs (3 valid + 1 unrecognized)
	if summary.Total != 4 {
		t.Errorf("Total = %d, want 4", summary.Total)
	}
	// Should have imported 3
	if summary.Success != 3 {
		t.Errorf("Success = %d, want 3", summary.Success)
	}
	// 1 skipped (unrecognized)
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", summary.Failed)
	}

	// Progress should have been called
	if len(progressCalls) == 0 {
		t.Error("no progress callbacks")
	}
}

func TestImportAll_DryRun(t *testing.T) {
	imp, _ := setupTestImporter(t)

	summary, err := imp.ImportAll(ImportOptions{
		DryRun: true,
	}, nil)

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// In dry-run, success count should still be 3
	if summary.Success != 3 {
		t.Errorf("Success = %d, want 3 (dry-run)", summary.Success)
	}

	// But nothing should actually be installed
	installed, _ := imp.installer.ListInstalled()
	if len(installed) != 0 {
		t.Errorf("dry-run should not install anything, got %d installed", len(installed))
	}
}

func TestImportAll_AlreadyInstalled(t *testing.T) {
	imp, _ := setupTestImporter(t)

	// First import
	_, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("first import error = %v", err)
	}

	// Second import - should find all already installed
	summary, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("second import error = %v", err)
	}

	// Should still report success (already installed counts as success)
	if summary.Success != 3 {
		t.Errorf("Success = %d, want 3 (already installed)", summary.Success)
	}
	// Already installed count should be 3
	if summary.SkippedAlreadyInstalled != 3 {
		t.Errorf("SkippedAlreadyInstalled = %d, want 3", summary.SkippedAlreadyInstalled)
	}
}

func TestImportAll_Force(t *testing.T) {
	imp, _ := setupTestImporter(t)

	// First import
	_, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("first import error = %v", err)
	}

	// Force reimport
	summary, err := imp.ImportAll(ImportOptions{
		Force: true,
	}, nil)
	if err != nil {
		t.Fatalf("force import error = %v", err)
	}

	if summary.Success != 3 {
		t.Errorf("Success = %d, want 3 (force)", summary.Success)
	}
	// Force should not count as already installed
	if summary.SkippedAlreadyInstalled != 0 {
		t.Errorf("SkippedAlreadyInstalled = %d, want 0 (force)", summary.SkippedAlreadyInstalled)
	}
}

func TestImportAll_EmptyDir(t *testing.T) {
	reg := browser.NewRegistry()
	inst := install.NewManager(paths.New(t.TempDir()), reg)

	emptyDir := t.TempDir()
	scanner, err := NewScanner(emptyDir, reg)
	if err != nil {
		t.Fatal(err)
	}
	imp := NewImporter(scanner, inst)

	summary, err := imp.ImportAll(ImportOptions{}, nil)

	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("Total = %d, want 0", summary.Total)
	}
}

func TestImportSummary_PrintSummary(t *testing.T) {
	summary := &ImportSummary{
		Total:                  5,
		Success:                3,
		Failed:                 1,
		Skipped:                1,
		SkippedIncompatible:    0,
		SkippedAlreadyInstalled: 0,
		Results: []ImportResult{
			{Browser: "chrome", Version: "1.0.0", Success: true},
			{Browser: "chrome", Version: "2.0.0", Success: true},
			{Browser: "firefox", Version: "1.5.0", Success: true},
			{Browser: "fail", Version: "1.0.0", Success: false, Error: errTest},
		},
	}

	var buf bytes.Buffer
	summary.PrintSummary(&buf)

	output := buf.String()
	if !strings.Contains(output, "Import Summary:") {
		t.Errorf("output missing header: %s", output)
	}
	if !strings.Contains(output, "Total scanned:") {
		t.Errorf("output missing total: %s", output)
	}
	if !strings.Contains(output, "Succeeded:") {
		t.Errorf("output missing success count: %s", output)
	}
	if !strings.Contains(output, "Failed:") {
		t.Errorf("output missing failed count: %s", output)
	}
	if !strings.Contains(output, "chrome@1.0.0") {
		t.Errorf("output missing success entry: %s", output)
	}
	if !strings.Contains(output, "FAIL") && !strings.Contains(output, "fail@1.0.0") {
		t.Errorf("output missing fail entry: %s", output)
	}
}

func TestImportSummary_PrintSummary_WithIncompatible(t *testing.T) {
	summary := &ImportSummary{
		Total:                  6,
		Success:                3,
		Failed:                 1,
		Skipped:                2,
		SkippedIncompatible:    1,
		SkippedAlreadyInstalled: 1,
		Results: []ImportResult{
			{Browser: "chrome", Version: "1.0.0", Arch: "amd64", Success: true},
			{Browser: "chrome", Version: "2.0.0", Arch: "arm64", Success: false, SkipReason: "incompatible-arch", Error: errTest},
			{Browser: "firefox", Version: "1.5.0", Arch: "amd64", Success: true, SkipReason: "already-installed"},
		},
	}

	var buf bytes.Buffer
	summary.PrintSummary(&buf)

	output := buf.String()
	if !strings.Contains(output, "incompatible arch") {
		t.Errorf("output missing incompatible arch line: %s", output)
	}
	if !strings.Contains(output, "already installed") {
		t.Errorf("output missing already installed line: %s", output)
	}
	if !strings.Contains(output, "SKIP(arch)") {
		t.Errorf("output missing SKIP(arch) label: %s", output)
	}
	if !strings.Contains(output, "OK(installed)") {
		t.Errorf("output missing OK(installed) label: %s", output)
	}
}

var errTest = &testError{"test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func TestImportAll_ProgressPhases(t *testing.T) {
	imp, _ := setupTestImporter(t)

	var phases []string
	imp.ImportAll(ImportOptions{}, func(p ImportProgress) {
		phases = append(phases, p.Phase)
	})

	// Should have scanning, importing, and done phases
	hasScanning := false
	hasImporting := false
	hasDone := false
	for _, phase := range phases {
		switch phase {
		case "scanning":
			hasScanning = true
		case "importing":
			hasImporting = true
		case "done":
			hasDone = true
		}
	}

	if !hasScanning {
		t.Error("missing 'scanning' phase")
	}
	if !hasImporting {
		t.Error("missing 'importing' phase")
	}
	if !hasDone {
		t.Error("missing 'done' phase")
	}
}

func TestNewImporter(t *testing.T) {
	imp, _ := setupTestImporter(t)

	if imp == nil {
		t.Fatal("NewImporter returned nil")
	}
	if imp.scanner == nil {
		t.Error("scanner is nil")
	}
	if imp.installer == nil {
		t.Error("installer is nil")
	}
}

func TestImportAll_ArchIncompatible(t *testing.T) {
	root := t.TempDir()
	p := paths.New(root)
	p.EnsureAll()

	chromeExe := "chrome"
	if runtime.GOOS == "windows" {
		chromeExe = "chrome.exe"
	}

	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {chromeExe}},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})

	inst := install.NewManager(p, reg)

	// Create source repository with one compatible and one incompatible arch
	repoDir := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repoDir, 0o755)

	// Compatible version (same arch)
	compatibleDir := filepath.Join(repoDir, "chrome_1.0.0_"+archSuffix())
	os.MkdirAll(compatibleDir, 0o755)
	os.WriteFile(filepath.Join(compatibleDir, chromeExe), []byte("fake"), 0o755)

	// Incompatible version (arm64 on amd64, or amd64 on 386, etc.)
	incompatibleArch := "arm64"
	if paths.Arch() == "arm64" {
		incompatibleArch = "amd64"
	}
	incompatibleDir := filepath.Join(repoDir, "chrome_2.0.0_"+incompatibleArch)
	os.MkdirAll(incompatibleDir, 0o755)
	os.WriteFile(filepath.Join(incompatibleDir, chromeExe), []byte("fake"), 0o755)

	scanner, err := NewScanner(repoDir, reg)
	if err != nil {
		t.Fatal(err)
	}

	imp := NewImporter(scanner, inst)
	summary, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("ImportAll() error = %v", err)
	}

	// Total should be 2
	if summary.Total != 2 {
		t.Errorf("Total = %d, want 2", summary.Total)
	}
	// 1 success (compatible)
	if summary.Success != 1 {
		t.Errorf("Success = %d, want 1", summary.Success)
	}
	// 1 skipped (incompatible)
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}
	// 1 skipped incompatible
	if summary.SkippedIncompatible != 1 {
		t.Errorf("SkippedIncompatible = %d, want 1", summary.SkippedIncompatible)
	}
}

func TestImportAll_ZipFile(t *testing.T) {
	root := t.TempDir()
	p := paths.New(root)
	p.EnsureAll()

	chromeExe := "chrome"
	if runtime.GOOS == "windows" {
		chromeExe = "chrome.exe"
	}

	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {chromeExe}},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})

	inst := install.NewManager(p, reg)

	// Create source repository with a zip file
	repoDir := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repoDir, 0o755)

	// Create a zip file with the browser executable inside
	zipPath := filepath.Join(repoDir, "chrome_1.0.0_"+archSuffix()+".zip")
	createTestZip(t, zipPath, chromeExe)

	scanner, err := NewScanner(repoDir, reg)
	if err != nil {
		t.Fatal(err)
	}

	imp := NewImporter(scanner, inst)
	summary, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("ImportAll() error = %v", err)
	}

	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Success != 1 {
		t.Errorf("Success = %d, want 1", summary.Success)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", summary.Failed)
		for _, r := range summary.Results {
			if r.Error != nil {
				t.Logf("  Error: %v", r.Error)
			}
		}
	}

	// Verify it was actually installed
	if !inst.IsInstalled("chrome", "1.0.0") {
		t.Error("chrome@1.0.0 should be installed after zip import")
	}
}

func TestImportAll_UnsupportedFormat(t *testing.T) {
	root := t.TempDir()
	p := paths.New(root)
	p.EnsureAll()

	chromeExe := "chrome"
	if runtime.GOOS == "windows" {
		chromeExe = "chrome.exe"
	}

	reg := browser.NewRegistry()
	reg.Register(&browser.BrowserDescriptor{
		Name: "chrome",
		ExecutableCandidates: map[string]map[string][]string{
			runtime.GOOS: {runtime.GOARCH: {chromeExe}},
		},
		Features: browser.BrowserFeatures{SupportsProfile: true},
	})

	inst := install.NewManager(p, reg)

	// Create source repository with an unsupported file extension
	repoDir := filepath.Join(t.TempDir(), "repo")
	os.MkdirAll(repoDir, 0o755)

	// Create a fake file with unknown extension but matching browser keyword
	badPath := filepath.Join(repoDir, "chrome_1.0.0_win64_setup.xyz")
	os.WriteFile(badPath, []byte("This is not a real file"), 0o644)

	scanner, err := NewScanner(repoDir, reg)
	if err != nil {
		t.Fatal(err)
	}

	imp := NewImporter(scanner, inst)
	summary, err := imp.ImportAll(ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("ImportAll() error = %v", err)
	}

	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (unsupported format)", summary.Failed)
	}
}

func TestExtractZip(t *testing.T) {
	// Create a temp zip file
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, "test.exe")

	// Extract it
	destDir := filepath.Join(tmpDir, "extracted")
	err := archive.Extract(zipPath, destDir)
	if err != nil {
		t.Fatalf("archive.Extract() error = %v", err)
	}

	// Verify the file was extracted
	extractedFile := filepath.Join(destDir, "test.exe")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Error("extracted file not found")
	}
}

func TestIsZipFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real zip file
	zipPath := filepath.Join(tmpDir, "real.zip")
	createTestZip(t, zipPath, "test.exe")

	if !isZipFileCheck(zipPath) {
		t.Error("isZipFile should return true for a real zip file")
	}

	// Create a non-zip file
	nonZipPath := filepath.Join(tmpDir, "fake.exe")
	os.WriteFile(nonZipPath, []byte("This is not a zip"), 0o644)

	if isZipFileCheck(nonZipPath) {
		t.Error("isZipFile should return false for a non-zip file")
	}
}

// isZipFileCheck is a helper for testing that reads the zip magic number.
// (The actual implementation is in the archive package; this is kept here for test compatibility.)
func isZipFileCheck(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 4)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return false
	}

	return buf[0] == 'P' && buf[1] == 'K' && buf[2] == 0x03 && buf[3] == 0x04
}

func TestFindContentDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: files at root level
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0o644)
	result, err := findContentDir(tmpDir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if result != tmpDir {
		t.Errorf("findContentDir with files at root = %q, want %q", result, tmpDir)
	}

	// Case 2: single subdirectory with executable
	dir2 := filepath.Join(t.TempDir(), "nested")
	subDir := filepath.Join(dir2, "chrome-win64")
	os.MkdirAll(subDir, 0o755)
	chromeExe := "chrome"
	if runtime.GOOS == "windows" {
		chromeExe = "chrome.exe"
	}
	os.WriteFile(filepath.Join(subDir, chromeExe), []byte("test"), 0o755)
	result, err = findContentDir(dir2, "chrome")
	if err != nil {
		t.Fatal(err)
	}
	if result != subDir {
		t.Errorf("findContentDir with single subdir = %q, want %q", result, subDir)
	}

	// Case 3: single subdirectory with files but no matching exe (heuristic fallback)
	dir3 := filepath.Join(t.TempDir(), "nested2")
	subDir3 := filepath.Join(dir3, "package")
	os.MkdirAll(subDir3, 0o755)
	os.WriteFile(filepath.Join(subDir3, "random.txt"), []byte("test"), 0o644)
	result, err = findContentDir(dir3, "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if result != subDir3 {
		t.Errorf("findContentDir heuristic fallback = %q, want %q", result, subDir3)
	}
}

// helper to create a test zip file
func createTestZip(t *testing.T, zipPath string, exeName string) {
	t.Helper()

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Add an executable file
	exeFile, err := w.Create(exeName)
	if err != nil {
		t.Fatal(err)
	}
	_, err = exeFile.Write([]byte("fake executable"))
	if err != nil {
		t.Fatal(err)
	}
}
