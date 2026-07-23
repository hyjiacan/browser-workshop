package fingerprint

import (
	"fmt"
	"strings"
)

// ChromeArgs builds Chrome/Chromium command-line arguments from the fingerprint config.
// Returns nil if the config is empty.
func (c *Config) ChromeArgs() []string {
	if c.IsEmpty() {
		return nil
	}

	var args []string

	// User-Agent (HTTP header only, does NOT affect navigator.userAgent JS property)
	if c.UserAgent != "" {
		args = append(args, "--user-agent="+c.UserAgent)
	}

	// Language
	if c.Language != "" {
		args = append(args, "--lang="+c.Language)
	}

	// Window size
	if c.WindowWidth > 0 && c.WindowHeight > 0 {
		args = append(args, fmt.Sprintf("--window-size=%d,%d", c.WindowWidth, c.WindowHeight))
	}

	// Device pixel ratio
	if c.DevicePixelRatio > 0 {
		args = append(args, fmt.Sprintf("--force-device-scale-factor=%.1f", c.DevicePixelRatio))
	}

	// WebRTC
	switch c.WebRTC {
	case "disabled":
		// Disable all non-proxied UDP and hide local IPs
		args = append(args,
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
			"--enforce-webrtc-local-ip-allowed-check",
		)
	case "proxied":
		// Only use proxy interfaces for WebRTC
		args = append(args,
			"--force-webrtc-ip-handling-policy=default_public_interface_only",
		)
	}

	// WebGL
	if c.DisableWebGL {
		args = append(args, "--disable-webgl")
	}

	// Canvas read blocking
	if c.DisableCanvasRead {
		args = append(args, "--disable-reading-from-canvas")
	}

	// Fake media devices
	if c.FakeMediaDevices {
		args = append(args,
			"--use-fake-device-for-media-stream",
			"--use-fake-ui-for-media-stream",
		)
	}

	return args
}

// ChromeArgsSummary returns a human-readable summary of what fingerprint args will be applied to Chrome.
func (c *Config) ChromeArgsSummary() string {
	if c.IsEmpty() {
		return ""
	}

	var parts []string
	if c.UserAgent != "" {
		parts = append(parts, "User-Agent")
	}
	if c.Language != "" {
		parts = append(parts, "语言: "+c.Language)
	}
	if c.WindowWidth > 0 {
		parts = append(parts, fmt.Sprintf("窗口: %dx%d", c.WindowWidth, c.WindowHeight))
	}
	if c.DevicePixelRatio > 0 {
		parts = append(parts, fmt.Sprintf("DPR: %.1f", c.DevicePixelRatio))
	}
	if c.WebRTC != "" {
		parts = append(parts, "WebRTC: "+c.WebRTC)
	}
	if c.DisableWebGL {
		parts = append(parts, "WebGL 已禁用")
	}
	if c.DisableCanvasRead {
		parts = append(parts, "Canvas 读取已禁用")
	}
	if c.FakeMediaDevices {
		parts = append(parts, "虚拟媒体设备")
	}

	return strings.Join(parts, ", ")
}