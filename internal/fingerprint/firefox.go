package fingerprint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FirefoxPrefs generates Firefox user.js content from the fingerprint config.
// Returns the prefs content as a string, or empty if config is empty.
func (c *Config) FirefoxPrefs() string {
	if c.IsEmpty() {
		return ""
	}

	var b strings.Builder
	b.WriteString("// Browser fingerprint isolation settings written by bws\n")

	// --- Core: resistFingerprinting ---
	// For "standard" and "random" presets, enable RFP which gives comprehensive protection.
	// For custom configs, only enable what's specified.
	if c.Preset == "standard" || c.Preset == "random" {
		b.WriteString(firefoxRFP)
	}

	// --- User-Agent ---
	if c.UserAgent != "" {
		b.WriteString(fmt.Sprintf("user_pref(\"general.useragent.override\", %s);\n", jsString(c.UserAgent)))
	}

	// --- Language ---
	if c.Language != "" {
		b.WriteString(fmt.Sprintf("user_pref(\"intl.accept_languages\", %s);\n", jsString(c.Language)))
		// Also attempt to spoof English
		if !strings.HasPrefix(c.Language, "en") {
			b.WriteString("user_pref(\"privacy.spoof_english\", 2);\n")
		}
	}

	// --- WebRTC ---
	switch c.WebRTC {
	case "disabled":
		b.WriteString(firefoxWebRTCDisabled)
	case "proxied":
		b.WriteString(firefoxWebRTCProxied)
	}

	// --- WebGL ---
	if c.DisableWebGL {
		b.WriteString("user_pref(\"webgl.disabled\", true);\n")
	}

	// --- Media devices ---
	if c.FakeMediaDevices {
		// Firefox doesn't have direct fake media device flags like Chrome.
		// Instead, disable media device enumeration.
		b.WriteString("user_pref(\"media.navigator.enabled\", false);\n")
		b.WriteString("user_pref(\"media.webspeech.enabled\", false);\n")
	}

	// --- Additional privacy prefs (always applied for standard/random) ---
	if c.Preset == "standard" || c.Preset == "random" {
		b.WriteString(firefoxPrivacyExtras)
	}

	return b.String()
}

// jsString escapes a Go string for safe insertion as a JavaScript string literal.
func jsString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return fmt.Sprintf("\"%s\"", s)
}

// WriteFirefoxUserJS writes the fingerprint prefs to user.js in the profile directory.
// Existing content is preserved (the user.js format allows multiple writes).
// This merges with any existing user.js content (e.g. from proxy settings).
func (c *Config) WriteFirefoxUserJS(profileDir string) error {
	if c.IsEmpty() {
		return nil
	}

	prefs := c.FirefoxPrefs()
	if prefs == "" {
		return nil
	}

	prefsPath := filepath.Join(profileDir, "user.js")

	// Read existing content
	var existing string
	if data, err := os.ReadFile(prefsPath); err == nil {
		existing = string(data)
	}

	// Only append if not already present (avoid duplicates)
	if strings.Contains(existing, "fingerprint isolation settings written by bws") {
		return nil
	}

	// Append new prefs
	newContent := existing + "\n" + prefs
	return os.WriteFile(prefsPath, []byte(newContent), 0o644)
}

// Firefox prefs constants

const firefoxRFP = `user_pref("privacy.resistFingerprinting", true);
user_pref("privacy.resistFingerprinting.letterboxing", true);
`

const firefoxWebRTCDisabled = `user_pref("media.peerconnection.enabled", false);
user_pref("media.peerconnection.ice.default_address_only", true);
user_pref("media.peerconnection.ice.no_host", true);
`

const firefoxWebRTCProxied = `user_pref("media.peerconnection.ice.default_address_only", true);
user_pref("media.peerconnection.ice.proxy_only_if_behind_proxy", true);
`

const firefoxPrivacyExtras = `user_pref("geo.enabled", false);
user_pref("device.sensors.enabled", false);
user_pref("dom.battery.enabled", false);
user_pref("dom.event.clipboardevents.enabled", false);
user_pref("browser.send_pings", false);
user_pref("beacon.enabled", false);
user_pref("network.http.referer.XOriginPolicy", 1);
// WebAudio enabled: disabled audio context is a distinct fingerprint signal.
// We intentionally keep it enabled to avoid standing out.
user_pref("dom.webaudio.enabled", true);
`
