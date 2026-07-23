// Package fingerprint provides browser fingerprint isolation configuration.
// It supports presets ("standard", "random", "none") and custom JSON configs,
// generating browser-specific arguments for Chrome and Firefox.
package fingerprint

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
)

// Config holds fingerprint isolation settings.
type Config struct {
	Preset string `json:"preset"` // "standard", "random", "custom", "none"

	// User-Agent
	UserAgent string `json:"userAgent,omitempty"` // HTTP User-Agent header

	// Language
	Language string `json:"language,omitempty"` // e.g. "zh-CN", "en-US"

	// Window
	WindowWidth  int `json:"windowWidth,omitempty"`  // e.g. 1280
	WindowHeight int `json:"windowHeight,omitempty"` // e.g. 720

	// Device pixel ratio
	DevicePixelRatio float64 `json:"devicePixelRatio,omitempty"` // e.g. 1.0, 2.0

	// WebRTC
	WebRTC string `json:"webrtc,omitempty"` // "disabled", "proxied", "default"

	// WebGL
	DisableWebGL bool `json:"disableWebGL,omitempty"` // Disable WebGL entirely

	// Canvas
	DisableCanvasRead bool `json:"disableCanvasRead,omitempty"` // Block canvas readback

	// Media devices
	FakeMediaDevices bool `json:"fakeMediaDevices,omitempty"` // Use fake camera/mic
}

// Validate checks the config for validity.
func (c *Config) Validate() error {
	if c.Preset != "" {
		switch c.Preset {
		case "standard", "random", "none", "custom":
		default:
			return fmt.Errorf("unknown preset %q (valid: standard, random, none, custom)", c.Preset)
		}
	}
	if c.DevicePixelRatio > 0 && (c.DevicePixelRatio < 0.5 || c.DevicePixelRatio > 4.0) {
		return fmt.Errorf("devicePixelRatio %.1f out of range (0.5-4.0)", c.DevicePixelRatio)
	}
	if c.WindowWidth > 0 && c.WindowWidth < 320 {
		return fmt.Errorf("windowWidth %d too small (min 320)", c.WindowWidth)
	}
	if c.WindowHeight > 0 && c.WindowHeight < 240 {
		return fmt.Errorf("windowHeight %d too small (min 240)", c.WindowHeight)
	}
	switch c.WebRTC {
	case "", "disabled", "proxied", "default":
	default:
		return fmt.Errorf("unknown webrtc value %q (valid: disabled, proxied, default)", c.WebRTC)
	}
	return nil
}

// IsEmpty returns true if no fingerprint settings are configured.
func (c *Config) IsEmpty() bool {
	if c == nil {
		return true
	}
	return c.Preset == "" || c.Preset == "none"
}

// FromString parses a fingerprint config from a string.
// Supports: "standard", "random", "none", JSON string, or "@filepath".
func FromString(s string) (*Config, error) {
	if s == "" || s == "none" {
		return &Config{Preset: "none"}, nil
	}

	// Presets
	if s == "standard" {
		return StandardPreset(), nil
	}
	if s == "random" {
		return RandomPreset(), nil
	}

	// File reference
	if strings.HasPrefix(s, "@") {
		path := s[1:]
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading fingerprint config file %q: %w", path, err)
		}
		return parseJSON(data)
	}

	// JSON string
	return parseJSON([]byte(s))
}

func parseJSON(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing fingerprint config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// StandardPreset returns a config with basic privacy protection.
// - Chrome: disables WebRTC, uses fake media devices, masks language
// - Firefox: uses resistFingerprinting (handled via user.js separately)
func StandardPreset() *Config {
	return &Config{
		Preset:           "standard",
		WebRTC:           "disabled",
		FakeMediaDevices: true,
	}
}

// commonResolutions is a list of realistic screen resolutions.
var commonResolutions = []struct{ W, H int }{
	{1920, 1080},
	{1366, 768},
	{2560, 1440},
	{1440, 900},
	{1536, 864},
	{1280, 720},
	{1600, 900},
	{1680, 1050},
}

// commonUserAgents are realistic Chrome user agents.
var commonUserAgents = []struct {
	UA, Platform string
}{
	{
		UA:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Platform: "Win32",
	},
	{
		UA:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Platform: "MacIntel",
	},
	{
		UA:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Platform: "Linux x86_64",
	},
}

// commonLanguages are realistic browser language preferences.
var commonLanguages = []string{
	"zh-CN", "en-US", "en-GB", "ja-JP", "ko-KR", "de-DE", "fr-FR",
}

// RandomPreset returns a config with randomly generated but self-consistent fingerprint values.
// The generated values are internally consistent (e.g. Windows UA with Windows resolution).
func RandomPreset() *Config {
	rng := rand.New(rand.NewSource(rand.Int63()))

	// Pick a platform and its UA
	ua := commonUserAgents[rng.Intn(len(commonUserAgents))]

	// Pick a resolution
	res := commonResolutions[rng.Intn(len(commonResolutions))]

	// Pick a language
	lang := commonLanguages[rng.Intn(len(commonLanguages))]

	// Pixel ratio (1.0 for most non-Retina, 2.0 for Mac)
	dpr := 1.0
	if strings.Contains(ua.Platform, "Mac") {
		dpr = 2.0
	}

	// WebRTC policy
	webrtcOptions := []string{"disabled", "proxied"}
	webrtc := webrtcOptions[rng.Intn(len(webrtcOptions))]

	return &Config{
		Preset:           "random",
		UserAgent:        ua.UA,
		Language:         lang,
		WindowWidth:      res.W,
		WindowHeight:     res.H,
		DevicePixelRatio: dpr,
		WebRTC:           webrtc,
		FakeMediaDevices: true,
		DisableWebGL:     rng.Float64() < 0.5,
	}
}