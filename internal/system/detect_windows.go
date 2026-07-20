//go:build windows
// +build windows

package system

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

// readFileVersion reads the version by running the browser with --version.
// On Windows, we also try directory-based detection first (faster).
func readFileVersion(path string) string {
	// First try: detect from directory structure (faster, no process spawn)
	if v := detectVersionFromPath(path); v != "" {
		return v
	}

	// Second try: run browser with --version
	cmd := exec.Command(path, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// Use CREATE_NO_WINDOW to avoid flashing a console window
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	err := cmd.Run()
	if err != nil {
		return ""
	}

	output := strings.TrimSpace(out.String())
	return extractVersion(output)
}

// extractVersion extracts a version number from a version output string.
func extractVersion(output string) string {
	re := regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:\.\d+)?`)
	match := re.FindString(output)
	return match
}
