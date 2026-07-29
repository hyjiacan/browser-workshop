package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOnlineFallback_ConfigRoundTrip verifies the online-fallback option
// survives save/load and can be set/got via the config key API.
func TestOnlineFallback_ConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Default should be false.
	cfg, err := LoadServeConfig(dir)
	if err != nil {
		t.Fatalf("LoadServeConfig: %v", err)
	}
	if cfg.OnlineFallback {
		t.Fatalf("default OnlineFallback = true, want false")
	}

	// Set it to true via the key API.
	cfg, err = SetConfigKey(dir, "online-fallback", "true")
	if err != nil {
		t.Fatalf("SetConfigKey: %v", err)
	}
	if !cfg.OnlineFallback {
		t.Fatalf("after set, OnlineFallback = false, want true")
	}

	// Reload from disk.
	cfg, err = LoadServeConfig(dir)
	if err != nil {
		t.Fatalf("LoadServeConfig reload: %v", err)
	}
	if !cfg.OnlineFallback {
		t.Fatalf("reloaded OnlineFallback = false, want true")
	}

	// GetConfigKey should report true.
	got, err := GetConfigKey(dir, "online-fallback")
	if err != nil {
		t.Fatalf("GetConfigKey: %v", err)
	}
	if got != "true" {
		t.Errorf("GetConfigKey online-fallback = %q, want %q", got, "true")
	}

	// The alternate (no-dash) key should also work.
	got2, err := GetConfigKey(dir, "onlinefallback")
	if err != nil {
		t.Errorf("GetConfigKey onlinefallback: %v", err)
	}
	if got2 != "true" {
		t.Errorf("GetConfigKey onlinefallback = %q, want %q", got2, "true")
	}

	// Setting false via alternate key.
	_, err = SetConfigKey(dir, "onlinefallback", "false")
	if err != nil {
		t.Fatalf("SetConfigKey onlinefallback false: %v", err)
	}
	cfg, _ = LoadServeConfig(dir)
	if cfg.OnlineFallback {
		t.Errorf("after setting false, OnlineFallback = true, want false")
	}

	// Ensure the INI file actually contains the online-fallback line.
	data, err := os.ReadFile(filepath.Join(dir, "bws-serve.ini"))
	if err != nil {
		t.Fatalf("read ini: %v", err)
	}
	if !strings.Contains(string(data), "online-fallback = false") {
		t.Errorf("INI file missing 'online-fallback = false' line:\n%s", string(data))
	}
}
