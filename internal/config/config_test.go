package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.DefaultBrowser != "chrome" {
		t.Errorf("DefaultBrowser = %q, want 'chrome'", cfg.DefaultBrowser)
	}
	if cfg.DefaultChannel != "stable" {
		t.Errorf("DefaultChannel = %q, want 'stable'", cfg.DefaultChannel)
	}
	if cfg.Download.MaxConcurrency != 3 {
		t.Errorf("Download.MaxConcurrency = %d, want 3", cfg.Download.MaxConcurrency)
	}
	if cfg.Download.RetryCount != 3 {
		t.Errorf("Download.RetryCount = %d, want 3", cfg.Download.RetryCount)
	}
	if cfg.Download.RetryDelay != 2*time.Second {
		t.Errorf("Download.RetryDelay = %v, want 2s", cfg.Download.RetryDelay)
	}
	if cfg.Download.Timeout != 30*time.Minute {
		t.Errorf("Download.Timeout = %v, want 30m", cfg.Download.Timeout)
	}
	if cfg.Cache.ManifestTTL != 24*time.Hour {
		t.Errorf("Cache.ManifestTTL = %v, want 24h", cfg.Cache.ManifestTTL)
	}
	if cfg.Cache.DownloadTTL != 168*time.Hour {
		t.Errorf("Cache.DownloadTTL = %v, want 168h", cfg.Cache.DownloadTTL)
	}
	if len(cfg.Aliases) == 0 {
		t.Error("Aliases is empty, expected defaults")
	}
	// Default repo path should be empty
	if cfg.RepoPath != "" {
		t.Errorf("RepoPath = %q, want empty string (default)", cfg.RepoPath)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (non-existent file returns defaults)", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	// Should have defaults
	if cfg.DefaultBrowser != "chrome" {
		t.Errorf("DefaultBrowser = %q, want 'chrome'", cfg.DefaultBrowser)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Default()
	original.DefaultBrowser = "firefox"
	original.DefaultChannel = "esr"
	original.Download.MaxConcurrency = 5
	original.RepoPath = "/tmp/browsers"

	if err := Save(original, path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Load it back
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.DefaultBrowser != "firefox" {
		t.Errorf("loaded DefaultBrowser = %q, want 'firefox'", loaded.DefaultBrowser)
	}
	if loaded.DefaultChannel != "esr" {
		t.Errorf("loaded DefaultChannel = %q, want 'esr'", loaded.DefaultChannel)
	}
	if loaded.Download.MaxConcurrency != 5 {
		t.Errorf("loaded MaxConcurrency = %d, want 5", loaded.Download.MaxConcurrency)
	}
	if loaded.RepoPath != "/tmp/browsers" {
		t.Errorf("loaded RepoPath = %q, want '/tmp/browsers'", loaded.RepoPath)
	}
	// Durations should survive round-trip
	if loaded.Download.RetryDelay != 2*time.Second {
		t.Errorf("loaded RetryDelay = %v, want 2s", loaded.Download.RetryDelay)
	}
	if loaded.Cache.ManifestTTL != 24*time.Hour {
		t.Errorf("loaded ManifestTTL = %v, want 24h", loaded.Cache.ManifestTTL)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not valid json{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() with invalid JSON returned nil error, expected error")
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.json")

	// Only set a few fields, rest should get defaults
	partial := `{
		"defaultBrowser": "chromium",
		"download": {
			"maxConcurrency": 10
		}
	}`
	if err := os.WriteFile(path, []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Explicitly set value
	if cfg.DefaultBrowser != "chromium" {
		t.Errorf("DefaultBrowser = %q, want 'chromium'", cfg.DefaultBrowser)
	}
	if cfg.Download.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d, want 10", cfg.Download.MaxConcurrency)
	}
	// Default values should be filled in
	if cfg.DefaultChannel != "stable" {
		t.Errorf("DefaultChannel = %q, want 'stable' (default)", cfg.DefaultChannel)
	}
	if cfg.Download.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3 (default)", cfg.Download.RetryCount)
	}
}

func TestRepoPath(t *testing.T) {
	cfg := Default()

	// Default should be empty
	if cfg.GetRepoPath() != "" {
		t.Errorf("GetRepoPath() = %q, want empty string", cfg.GetRepoPath())
	}

	// Set and get
	cfg.SetRepoPath("/path/to/repo")
	if cfg.GetRepoPath() != "/path/to/repo" {
		t.Errorf("GetRepoPath() = %q, want '/path/to/repo'", cfg.GetRepoPath())
	}

	// Overwrite
	cfg.SetRepoPath("/new/path")
	if cfg.GetRepoPath() != "/new/path" {
		t.Errorf("GetRepoPath() after overwrite = %q, want '/new/path'", cfg.GetRepoPath())
	}
}

func TestGetSources(t *testing.T) {
	cfg := Default()

	sources := cfg.GetSources("firefox")
	if len(sources) == 0 {
		t.Fatal("GetSources('firefox') returned empty list")
	}

	// Verify sorting by priority
	for i := 0; i < len(sources)-1; i++ {
		if sources[i].Priority > sources[i+1].Priority {
			t.Errorf("sources not sorted by priority: [%d] > [%d]", sources[i].Priority, sources[i+1].Priority)
		}
	}

	// Disabled sources should be excluded
	cfg.Sources["firefox"][0].Enabled = false
	sources = cfg.GetSources("firefox")
	if len(sources) != 0 {
		t.Errorf("GetSources() with all disabled returned %d sources, want 0", len(sources))
	}

	// Unknown browser
	sources = cfg.GetSources("unknown-browser")
	if sources != nil {
		t.Errorf("GetSources('unknown-browser') = %v, want nil", sources)
	}
}

func TestDurationParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "durations.json")

	custom := `{
		"download": {
			"retryDelay": "5s",
			"timeout": "1h"
		},
		"cache": {
			"manifestTTL": "12h",
			"downloadTTL": "48h"
		}
	}`
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Download.RetryDelay != 5*time.Second {
		t.Errorf("RetryDelay = %v, want 5s", cfg.Download.RetryDelay)
	}
	if cfg.Download.Timeout != 1*time.Hour {
		t.Errorf("Timeout = %v, want 1h", cfg.Download.Timeout)
	}
	if cfg.Cache.ManifestTTL != 12*time.Hour {
		t.Errorf("ManifestTTL = %v, want 12h", cfg.Cache.ManifestTTL)
	}
	if cfg.Cache.DownloadTTL != 48*time.Hour {
		t.Errorf("DownloadTTL = %v, want 48h", cfg.Cache.DownloadTTL)
	}
}

func TestInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-dur.json")

	bad := `{"download": {"retryDelay": "not-a-duration"}}`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() with invalid duration should return error")
	}
}
