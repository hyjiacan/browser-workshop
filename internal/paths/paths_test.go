package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".bm")
	p := New(root)

	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if p.Config != filepath.Join(root, "config.json") {
		t.Errorf("Config path incorrect")
	}
	if p.VersionsDir != filepath.Join(root, "versions") {
		t.Errorf("VersionsDir incorrect")
	}
	if p.CacheDir != filepath.Join(root, "cache") {
		t.Errorf("CacheDir incorrect")
	}
	if p.ManifestCacheDir != filepath.Join(root, "cache", "manifests") {
		t.Errorf("ManifestCacheDir incorrect")
	}
	if p.DownloadCacheDir != filepath.Join(root, "cache", "downloads") {
		t.Errorf("DownloadCacheDir incorrect")
	}
	if p.RuntimeDir != filepath.Join(root, "runtime") {
		t.Errorf("RuntimeDir incorrect")
	}
}

func TestEnsureAll(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".bm")
	p := New(root)

	if err := p.EnsureAll(); err != nil {
		t.Fatalf("EnsureAll() error = %v", err)
	}

	dirs := []string{
		p.Root,
		p.VersionsDir,
		p.CacheDir,
		p.ManifestCacheDir,
		p.DownloadCacheDir,
		p.RuntimeDir,
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("os.Stat(%q) error = %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestVersionDir(t *testing.T) {
	root := t.TempDir()
	p := New(root)

	got := p.VersionDir("chrome", "120.0.6099.109")
	want := filepath.Join(root, "versions", "chrome", "120.0.6099.109")
	if got != want {
		t.Errorf("VersionDir() = %q, want %q", got, want)
	}
}

func TestVersionMetaFile(t *testing.T) {
	root := t.TempDir()
	p := New(root)

	got := p.VersionMetaFile("firefox", "121.0")
	want := filepath.Join(root, "versions", "firefox", "121.0", ".bws.json")
	if got != want {
		t.Errorf("VersionMetaFile() = %q, want %q", got, want)
	}
}

func TestDownloadDir(t *testing.T) {
	root := t.TempDir()
	p := New(root)

	got := p.DownloadDir("chrome", "120.0.6099.109")
	want := filepath.Join(root, "cache", "downloads", "chrome", "120.0.6099.109")
	if got != want {
		t.Errorf("DownloadDir() = %q, want %q", got, want)
	}
}

func TestProfileDir(t *testing.T) {
	root := t.TempDir()
	p := New(root)

	got := p.ProfileDir("chrome", "120.0.6099.109")
	want := filepath.Join(root, "runtime", "chrome", "120.0.6099.109", "profile")
	if got != want {
		t.Errorf("ProfileDir() = %q, want %q", got, want)
	}
}

func TestManifestFile(t *testing.T) {
	root := t.TempDir()
	p := New(root)

	got := p.ManifestFile("firefox")
	want := filepath.Join(root, "cache", "manifests", "firefox.json")
	if got != want {
		t.Errorf("ManifestFile() = %q, want %q", got, want)
	}
}

func TestPlatform(t *testing.T) {
	got := Platform()
	switch runtime.GOOS {
	case "windows":
		if got != "windows" {
			t.Errorf("Platform() = %q, want 'windows'", got)
		}
	case "darwin":
		if got != "darwin" {
			t.Errorf("Platform() = %q, want 'darwin'", got)
		}
	case "linux":
		if got != "linux" {
			t.Errorf("Platform() = %q, want 'linux'", got)
		}
	}
}

func TestArch(t *testing.T) {
	got := Arch()
	switch runtime.GOARCH {
	case "amd64":
		if got != "amd64" {
			t.Errorf("Arch() = %q, want 'amd64'", got)
		}
	case "386":
		if got != "386" {
			t.Errorf("Arch() = %q, want '386'", got)
		}
	case "arm64":
		if got != "arm64" {
			t.Errorf("Arch() = %q, want 'arm64'", got)
		}
	}
}

func TestArchCompatible(t *testing.T) {
	current := Arch()

	// Empty arch is always compatible
	if !ArchCompatible("") {
		t.Error("ArchCompatible(\"\") should return true")
	}

	// Same arch is always compatible
	if !ArchCompatible(current) {
		t.Errorf("ArchCompatible(%q) should return true on %s system", current, current)
	}

	// amd64 system can run 386
	if current == "amd64" {
		if !ArchCompatible("386") {
			t.Error("ArchCompatible(\"386\") should return true on amd64 system")
		}
		if ArchCompatible("arm64") {
			t.Error("ArchCompatible(\"arm64\") should return false on amd64 system")
		}
	}

	// 386 system can only run 386
	if current == "386" {
		if ArchCompatible("amd64") {
			t.Error("ArchCompatible(\"amd64\") should return false on 386 system")
		}
		if ArchCompatible("arm64") {
			t.Error("ArchCompatible(\"arm64\") should return false on 386 system")
		}
	}

	// arm64 system can only run arm64 (no Rosetta check)
	if current == "arm64" {
		if ArchCompatible("amd64") {
			t.Error("ArchCompatible(\"amd64\") should return false on arm64 system")
		}
		if ArchCompatible("386") {
			t.Error("ArchCompatible(\"386\") should return false on arm64 system")
		}
	}
}

func TestDefault(t *testing.T) {
	// Reset singleton for test
	once = sync.Once{}
	instance = nil

	p := Default()
	if p == nil {
		t.Fatal("Default() returned nil")
	}

	// Second call should return the same instance
	p2 := Default()
	if p != p2 {
		t.Error("Default() returned different instances")
	}

	// Root should be a valid directory (either exe dir or home dir with .bm)
	if p.Root == "" {
		t.Error("Default().Root is empty")
	}

	// Verify that all expected sub-paths are set correctly relative to Root
	if p.Config != filepath.Join(p.Root, "config.json") {
		t.Errorf("Default().Config path incorrect")
	}
	if p.VersionsDir != filepath.Join(p.Root, "versions") {
		t.Errorf("Default().VersionsDir path incorrect")
	}
	if p.CacheDir != filepath.Join(p.Root, "cache") {
		t.Errorf("Default().CacheDir path incorrect")
	}
}

func TestEnsureAll_Idempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".bm")
	p := New(root)

	// First call creates everything
	if err := p.EnsureAll(); err != nil {
		t.Fatalf("first EnsureAll() error = %v", err)
	}

	// Second call should not fail (idempotent)
	if err := p.EnsureAll(); err != nil {
		t.Fatalf("second EnsureAll() error = %v", err)
	}
}
