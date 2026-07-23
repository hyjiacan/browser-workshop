package fingerprint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFromString_Presets(t *testing.T) {
	tests := []struct {
		input    string
		wantPreset string
		wantErr  bool
	}{
		{"", "none", false},
		{"none", "none", false},
		{"standard", "standard", false},
		{"random", "random", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		cfg, err := FromString(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("FromString(%q) expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("FromString(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if cfg.Preset != tt.wantPreset {
			t.Errorf("FromString(%q).Preset = %q, want %q", tt.input, cfg.Preset, tt.wantPreset)
		}
	}
}

func TestFromString_JSON(t *testing.T) {
	json := `{"preset":"custom","userAgent":"test","language":"en-US","windowWidth":1280,"windowHeight":720,"devicePixelRatio":1.0,"webrtc":"disabled","disableWebGL":true,"fakeMediaDevices":true}`
	cfg, err := FromString(json)
	if err != nil {
		t.Fatalf("FromString(JSON) unexpected error: %v", err)
	}
	if cfg.Preset != "custom" {
		t.Errorf("preset = %q, want custom", cfg.Preset)
	}
	if cfg.UserAgent != "test" {
		t.Errorf("userAgent = %q, want test", cfg.UserAgent)
	}
	if cfg.WindowWidth != 1280 {
		t.Errorf("windowWidth = %d, want 1280", cfg.WindowWidth)
	}
}

func TestFromString_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fp.json")
	content := `{"preset":"custom","language":"ja-JP","webrtc":"proxied"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := FromString("@" + path)
	if err != nil {
		t.Fatalf("FromString(@file) unexpected error: %v", err)
	}
	if cfg.Language != "ja-JP" {
		t.Errorf("language = %q, want ja-JP", cfg.Language)
	}
	if cfg.WebRTC != "proxied" {
		t.Errorf("webrtc = %q, want proxied", cfg.WebRTC)
	}
}

func TestIsEmpty(t *testing.T) {
	var nilCfg *Config
	if !nilCfg.IsEmpty() {
		t.Error("nil config should be empty")
	}
	if !(&Config{Preset: "none"}).IsEmpty() {
		t.Error("none preset should be empty")
	}
	if (&Config{Preset: "standard"}).IsEmpty() {
		t.Error("standard preset should not be empty")
	}
}

func TestChromeArgs(t *testing.T) {
	cfg := &Config{
		Preset:           "custom",
		UserAgent:        "Mozilla/5.0 Test",
		Language:         "zh-CN",
		WindowWidth:      1280,
		WindowHeight:     720,
		DevicePixelRatio: 1.0,
		WebRTC:           "disabled",
		DisableWebGL:     true,
		DisableCanvasRead: true,
		FakeMediaDevices: true,
	}

	args := cfg.ChromeArgs()
	argStr := strings.Join(args, " ")
	checks := []string{
		"--user-agent=Mozilla/5.0 Test",
		"--lang=zh-CN",
		"--window-size=1280,720",
		"--force-device-scale-factor=1.0",
		"force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"enforce-webrtc-local-ip-allowed-check",
		"--disable-webgl",
		"--disable-reading-from-canvas",
		"--use-fake-device-for-media-stream",
		"--use-fake-ui-for-media-stream",
	}
	for _, check := range checks {
		if !strings.Contains(argStr, check) {
			t.Errorf("ChromeArgs missing %q in %q", check, argStr)
		}
	}
}

func TestChromeArgsEmpty(t *testing.T) {
	args := (&Config{Preset: "none"}).ChromeArgs()
	if len(args) != 0 {
		t.Errorf("empty config should produce no args, got %d: %v", len(args), args)
	}
}

func TestFirefoxPrefs(t *testing.T) {
	cfg := StandardPreset()
	prefs := cfg.FirefoxPrefs()
	checks := []string{
		"privacy.resistFingerprinting",
		"privacy.resistFingerprinting.letterboxing",
		"media.peerconnection.enabled",
		"geo.enabled",
	}
	for _, check := range checks {
		if !strings.Contains(prefs, check) {
			t.Errorf("FirefoxPrefs missing %q", check)
		}
	}
}

func TestFirefoxPrefsEmpty(t *testing.T) {
	prefs := (&Config{Preset: "none"}).FirefoxPrefs()
	if prefs != "" {
		t.Errorf("empty config should produce no prefs, got %d chars", len(prefs))
	}
}

func TestWriteFirefoxUserJS(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Preset: "standard",
		WebRTC: "disabled",
	}
	if err := cfg.WriteFirefoxUserJS(dir); err != nil {
		t.Fatalf("WriteFirefoxUserJS: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "user.js"))
	if err != nil {
		t.Fatalf("reading user.js: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "privacy.resistFingerprinting") {
		t.Error("user.js missing resistFingerprinting")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		cfg     Config
		wantErr bool
	}{
		{Config{Preset: "standard"}, false},
		{Config{Preset: "bad"}, true},
		{Config{DevicePixelRatio: 0.3}, true},
		{Config{DevicePixelRatio: 5.0}, true},
		{Config{DevicePixelRatio: 2.0}, false},
		{Config{WindowWidth: 100}, true},
		{Config{WindowWidth: 1920, WindowHeight: 1080}, false},
		{Config{WebRTC: "invalid"}, true},
		{Config{WebRTC: "disabled"}, false},
		{Config{WebRTC: ""}, false},
	}
	for _, tt := range tests {
		err := tt.cfg.Validate()
		if tt.wantErr && err == nil {
			t.Errorf("Validate(%+v) expected error, got nil", tt.cfg)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("Validate(%+v) unexpected error: %v", tt.cfg, err)
		}
	}
}

func TestRandomPreset(t *testing.T) {
	// Run multiple times to ensure consistency
	for i := 0; i < 10; i++ {
		cfg := RandomPreset()
		if cfg.Preset != "random" {
			t.Errorf("RandomPreset.Preset = %q", cfg.Preset)
		}
		if cfg.UserAgent == "" {
			t.Error("RandomPreset.UserAgent is empty")
		}
		if cfg.Language == "" {
			t.Error("RandomPreset.Language is empty")
		}
		if cfg.WindowWidth == 0 || cfg.WindowHeight == 0 {
			t.Error("RandomPreset window size is zero")
		}
		if cfg.DevicePixelRatio <= 0 {
			t.Error("RandomPreset.DevicePixelRatio is zero")
		}
	}
}

func TestChromeArgsSummary(t *testing.T) {
	cfg := StandardPreset()
	summary := cfg.ChromeArgsSummary()
	if summary == "" {
		t.Error("StandardPreset should have a summary")
	}
	if !strings.Contains(summary, "WebRTC") {
		t.Error("summary should mention WebRTC")
	}
}