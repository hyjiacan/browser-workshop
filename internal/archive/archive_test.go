package archive

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// --- Test helpers ---

// createZip creates a zip file at the given path with the provided files.
// files is a map of path -> content.
func createZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("creating zip dir: %v", err)
	}

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("writing zip entry %s: %v", name, err)
		}
	}
}

// exeName returns the platform-appropriate executable name.
func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// --- ExtractRecursive tests ---

func TestExtractRecursive_NestedZip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create innermost content: a "browser" directory with chrome.exe
	innerFiles := map[string]string{
		"chrome/" + exeName("chrome"): "fake-chrome-binary",
		"chrome/version.txt":          "100.0.0.0",
	}

	// Create middle zip (inner.zip containing chrome dir)
	innerZipPath := filepath.Join(tmpDir, "inner.zip")
	createZip(t, innerZipPath, innerFiles)

	// Read inner zip content to embed in outer zip
	innerZipData, err := os.ReadFile(innerZipPath)
	if err != nil {
		t.Fatalf("reading inner zip: %v", err)
	}

	// Create outer zip containing inner.zip
	outerFiles := map[string]string{
		"inner.zip": string(innerZipData),
		"README.txt": "This is a test archive",
	}
	outerZipPath := filepath.Join(tmpDir, "outer.zip")
	createZip(t, outerZipPath, outerFiles)

	// Extract recursively
	destDir := filepath.Join(tmpDir, "dest")
	resultDir, err := ExtractRecursive(outerZipPath, destDir)
	if err != nil {
		t.Fatalf("ExtractRecursive failed: %v", err)
	}

	if resultDir != destDir {
		t.Errorf("expected result dir %s, got %s", destDir, resultDir)
	}

	// Verify inner.zip was extracted
	chromeExe := filepath.Join(destDir, "inner", "chrome", exeName("chrome"))
	if _, err := os.Stat(chromeExe); os.IsNotExist(err) {
		t.Errorf("chrome.exe not found at %s", chromeExe)
	}

	// Verify inner.zip was removed
	if _, err := os.Stat(filepath.Join(destDir, "inner.zip")); !os.IsNotExist(err) {
		t.Error("inner.zip should have been removed after extraction")
	}

	// Verify README is still there
	if _, err := os.Stat(filepath.Join(destDir, "README.txt")); os.IsNotExist(err) {
		t.Error("README.txt should still exist")
	}
}

func TestExtractRecursive_DeeplyNested(t *testing.T) {
	tmpDir := t.TempDir()

	// Level 3 (innermost): actual content
	level3Files := map[string]string{
		"app/" + exeName("myapp"): "binary-content",
	}
	level3Path := filepath.Join(tmpDir, "level3.zip")
	createZip(t, level3Path, level3Files)
	level3Data, _ := os.ReadFile(level3Path)

	// Level 2: contains level3.zip
	level2Files := map[string]string{
		"level3.zip": string(level3Data),
	}
	level2Path := filepath.Join(tmpDir, "level2.zip")
	createZip(t, level2Path, level2Files)
	level2Data, _ := os.ReadFile(level2Path)

	// Level 1 (outermost): contains level2.zip
	level1Files := map[string]string{
		"level2.zip": string(level2Data),
	}
	level1Path := filepath.Join(tmpDir, "level1.zip")
	createZip(t, level1Path, level1Files)

	// Extract recursively
	destDir := filepath.Join(tmpDir, "dest")
	resultDir, err := ExtractRecursive(level1Path, destDir)
	if err != nil {
		t.Fatalf("ExtractRecursive failed: %v", err)
	}

	if resultDir != destDir {
		t.Errorf("expected result dir %s, got %s", destDir, resultDir)
	}

	// Verify the innermost content exists (3 levels deep)
	appExe := filepath.Join(destDir, "level2", "level3", "app", exeName("myapp"))
	if _, err := os.Stat(appExe); os.IsNotExist(err) {
		t.Errorf("deeply nested app binary not found at %s", appExe)
	}

	// Verify all intermediate zips were removed
	for _, name := range []string{"level2.zip", "level2/level3.zip"} {
		p := filepath.Join(destDir, name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", name)
		}
	}
}

func TestExtractRecursive_NoNesting(t *testing.T) {
	tmpDir := t.TempDir()

	// A simple zip with no nested archives
	files := map[string]string{
		"hello.txt": "hello world",
		"data/info.txt": "info here",
	}
	zipPath := filepath.Join(tmpDir, "simple.zip")
	createZip(t, zipPath, files)

	destDir := filepath.Join(tmpDir, "dest")
	resultDir, err := ExtractRecursive(zipPath, destDir)
	if err != nil {
		t.Fatalf("ExtractRecursive failed: %v", err)
	}

	if resultDir != destDir {
		t.Errorf("expected result dir %s, got %s", destDir, resultDir)
	}

	// Verify content
	if _, err := os.Stat(filepath.Join(destDir, "hello.txt")); os.IsNotExist(err) {
		t.Error("hello.txt should exist")
	}
}

func TestExtractRecursive_FindBrowserExeAfterExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a chrome-like nested structure: outer.zip -> inner.zip -> chrome/chrome.exe
	innerFiles := map[string]string{
		"chrome/" + exeName("chrome"): "fake-chrome",
		"chrome/version.txt":          "120.0.0.0",
	}
	innerZipPath := filepath.Join(tmpDir, "chrome-pkg.zip")
	createZip(t, innerZipPath, innerFiles)
	innerZipData, _ := os.ReadFile(innerZipPath)

	// Outer zip (simulating an installer that contains the actual package)
	outerFiles := map[string]string{
		"chrome-pkg.zip": string(innerZipData),
		"installer.ini":  "installer config",
	}
	outerZipPath := filepath.Join(tmpDir, "installer.zip")
	createZip(t, outerZipPath, outerFiles)

	// Extract recursively
	destDir := filepath.Join(tmpDir, "dest")
	resultDir, err := ExtractRecursive(outerZipPath, destDir)
	if err != nil {
		t.Fatalf("ExtractRecursive failed: %v", err)
	}

	// Verify FindBrowserExe can find chrome.exe after recursive extraction
	exeCandidates := []string{exeName("chrome")}
	browserDir, err := FindBrowserExe(resultDir, "chrome", runtime.GOOS, runtime.GOARCH, exeCandidates)
	if err != nil {
		t.Fatalf("FindBrowserExe failed after recursive extraction: %v", err)
	}

	// The browser dir should contain chrome.exe
	chromePath := filepath.Join(browserDir, exeName("chrome"))
	if _, err := os.Stat(chromePath); os.IsNotExist(err) {
		t.Errorf("chrome.exe not found in browser dir %s", browserDir)
	}
}

func TestExtractRecursive_Nested7z(t *testing.T) {
	if !Has7z() {
		t.Skip("7z not available, skipping 7z test")
	}

	tmpDir := t.TempDir()

	// Step 1: Create inner content files
	innerContentDir := filepath.Join(tmpDir, "inner_content")
	appDir := filepath.Join(innerContentDir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testApp := filepath.Join(appDir, exeName("testapp"))
	if err := os.WriteFile(testApp, []byte("test-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Step 2: Create inner.7z using 7z command
	inner7zPath := filepath.Join(tmpDir, "inner.7z")
	if err := create7z(inner7zPath, innerContentDir); err != nil {
		t.Fatalf("creating inner.7z: %v", err)
	}

	// Step 3: Create outer content directory containing inner.7z
	outerContentDir := filepath.Join(tmpDir, "outer_content")
	if err := os.MkdirAll(outerContentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Copy inner.7z into outer content dir
	inner7zData, err := os.ReadFile(inner7zPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outerContentDir, "inner.7z"), inner7zData, 0o644); err != nil {
		t.Fatal(err)
	}
	// Also add a readme
	if err := os.WriteFile(filepath.Join(outerContentDir, "README.txt"), []byte("test readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 4: Create outer.7z
	outer7zPath := filepath.Join(tmpDir, "outer.7z")
	if err := create7z(outer7zPath, outerContentDir); err != nil {
		t.Fatalf("creating outer.7z: %v", err)
	}

	// Step 5: Extract recursively
	destDir := filepath.Join(tmpDir, "dest")
	resultDir, err := ExtractRecursive(outer7zPath, destDir)
	if err != nil {
		t.Fatalf("ExtractRecursive failed: %v", err)
	}

	if resultDir != destDir {
		t.Errorf("expected result dir %s, got %s", destDir, resultDir)
	}

	// Verify the innermost content exists
	appExe := filepath.Join(destDir, "inner", "app", exeName("testapp"))
	if _, err := os.Stat(appExe); os.IsNotExist(err) {
		t.Errorf("testapp not found at %s", appExe)
	}

	// Verify inner.7z was removed
	if _, err := os.Stat(filepath.Join(destDir, "inner.7z")); !os.IsNotExist(err) {
		t.Error("inner.7z should have been removed after extraction")
	}
}

// create7z creates a .7z archive from a source directory using the 7z command.
func create7z(archivePath, sourceDir string) error {
	exePath := find7z()
	if exePath == "" {
		return fmt.Errorf("7z executable not found")
	}
	// 7z a <archive> <files> -y
	cmd := exec.Command(exePath, "a", archivePath, sourceDir+string(os.PathSeparator)+"*", "-y")
	cmd.Dir = filepath.Dir(sourceDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func TestDeriveExtractDir(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"C:/temp/inner.zip", "C:/temp/inner"},
		{"/tmp/archive.tar.gz", "/tmp/archive"},
		{"/tmp/package.7z", "/tmp/package"},
		{"data/file.rar", "data/file"},
		{"./test.tar.bz2", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := deriveExtractDir(tt.path)
			// Normalize for comparison
			expected := filepath.FromSlash(tt.expected)
			result = filepath.Clean(result)
			if result != expected {
				t.Errorf("deriveExtractDir(%q) = %q, want %q", tt.path, result, expected)
			}
		})
	}
}

// --- Existing function tests ---

func TestIsSupportedFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.zip", true},
		{"test.7z", true},
		{"test.tar.gz", true},
		{"test.rar", true},
		{"test.exe", true},
		{"test.msi", true},
		{"test.txt", false},
		{"test.doc", false},
		{"test.ZIP", true}, // case insensitive
		{"TEST.7Z", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsSupportedFormat(tt.path)
			if result != tt.expected {
				t.Errorf("IsSupportedFormat(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractZipNative(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"hello.txt":          "hello world",
		"subdir/world.txt":   "nested content",
		"subdir/deep/f.txt":  "deep content",
	}
	zipPath := filepath.Join(tmpDir, "test.zip")
	createZip(t, zipPath, files)

	destDir := filepath.Join(tmpDir, "extract")
	err := extractZipNative(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractZipNative failed: %v", err)
	}

	for name, content := range files {
		p := filepath.Join(destDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("file %s not found: %v", name, err)
			continue
		}
		if string(data) != content {
			t.Errorf("file %s content = %q, want %q", name, string(data), content)
		}
	}
}

func TestFindBrowserExe(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory structure: root/chrome-win/chrome.exe
	chromeDir := filepath.Join(tmpDir, "chrome-win")
	if err := os.MkdirAll(chromeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	chromeExe := filepath.Join(chromeDir, exeName("chrome"))
	if err := os.WriteFile(chromeExe, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Find the browser
	result, err := FindBrowserExe(tmpDir, "chrome", runtime.GOOS, runtime.GOARCH, []string{exeName("chrome")})
	if err != nil {
		t.Fatalf("FindBrowserExe failed: %v", err)
	}

	if result != chromeDir {
		t.Errorf("FindBrowserExe = %q, want %q", result, chromeDir)
	}
}

func TestFindContentDir_Heuristic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create: root/single-dir/content.txt
	innerDir := filepath.Join(tmpDir, "single-dir")
	if err := os.MkdirAll(innerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerDir, "content.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := FindContentDir(tmpDir, "unknown", runtime.GOOS, runtime.GOARCH, nil)
	if err != nil {
		t.Fatalf("FindContentDir failed: %v", err)
	}

	if result != innerDir {
		t.Errorf("FindContentDir = %q, want %q", result, innerDir)
	}
}

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()
	if len(formats) == 0 {
		t.Error("SupportedFormats returned empty list")
	}

	// Check that common formats are present
	hasZip := false
	has7z := false
	hasTarGz := false
	for _, f := range formats {
		switch f {
		case ".zip":
			hasZip = true
		case ".7z":
			has7z = true
		case ".tar.gz":
			hasTarGz = true
		}
	}
	if !hasZip {
		t.Error(".zip not found in supported formats")
	}
	if !has7z {
		t.Error(".7z not found in supported formats")
	}
	if !hasTarGz {
		t.Error(".tar.gz not found in supported formats")
	}
}
