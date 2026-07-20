//go:build !windows
// +build !windows

package system

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
)

// readFileVersion reads the version by running the browser with --version.
// On non-Windows platforms, this is reliable and fast.
func readFileVersion(path string) string {
	// Try --version flag
	cmd := exec.Command(path, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		// Try -v flag (Firefox uses -v or --version)
		cmd = exec.Command(path, "-v")
		out.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return ""
		}
	}

	output := strings.TrimSpace(out.String())
	return extractVersion(output)
}

// extractVersion extracts a version number from a version output string.
// Examples:
//   "Google Chrome 120.0.6099.109 " -> "120.0.6099.109"
//   "Mozilla Firefox 121.0" -> "121.0"
//   "Chromium 121.0.6156.0" -> "121.0.6156.0"
func extractVersion(output string) string {
	// Match version-like patterns: digits.digits[.digits[.digits]]
	re := regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:\.\d+)?`)
	match := re.FindString(output)
	return match
}
