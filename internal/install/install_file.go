package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bws/bws/internal/archive"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

// InstallFromFile installs a browser version from an archive file.
// It extracts the archive and then installs from the extracted directory.
func (m *Manager) InstallFromFile(browserName, version, filePath string) (*version.InstallRecord, error) {
	if browserName == "" || version == "" {
		return nil, fmt.Errorf("browser and version are required")
	}
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Check if it's a supported archive format
	if !archive.IsSupportedFormat(filePath) {
		return nil, fmt.Errorf("unsupported file format: %s", filepath.Ext(filePath))
	}

	// Create temp directory for extraction
	tmpDir, err := os.MkdirTemp("", "bws-install-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract the archive (with recursive extraction for nested archives)
	if _, err := archive.ExtractRecursive(filePath, tmpDir); err != nil {
		return nil, fmt.Errorf("extracting archive: %w", err)
	}

	// Find the content directory containing the browser
	contentDir, err := findContentDir(tmpDir, browserName)
	if err != nil {
		return nil, fmt.Errorf("finding browser in extracted archive: %w", err)
	}

	// Install from the content directory
	return m.InstallFromDir(InstallOptions{
		Browser:   browserName,
		Version:   version,
		Source:    "file",
		SourceDir: contentDir,
	}, nil)
}

// findContentDir finds the browser executable directory within an extracted archive.
func findContentDir(root string, browserName string) (string, error) {
	exeCandidates := browserExecutableCandidates(browserName)
	return archive.FindContentDir(root, browserName, paths.Platform(), paths.Arch(), exeCandidates)
}

// browserExecutableCandidates returns common executable names for a browser.
func browserExecutableCandidates(browserName string) []string {
	lower := strings.ToLower(browserName)
	exeExt := ""
	if paths.Platform() == "windows" {
		exeExt = ".exe"
	}

	switch lower {
	case "chrome", "google chrome", "google-chrome":
		return []string{"chrome" + exeExt, "chrome.exe", "Google Chrome" + exeExt}
	case "firefox", "mozilla firefox":
		return []string{"firefox" + exeExt, "firefox.exe"}
	case "chromium":
		return []string{"chromium" + exeExt, "chromium.exe", "chrome" + exeExt}
	case "edge", "microsoft edge", "msedge":
		return []string{"msedge" + exeExt, "msedge.exe", "edge" + exeExt}
	case "brave":
		return []string{"brave" + exeExt, "brave.exe", "brave-browser" + exeExt}
	case "opera":
		return []string{"opera" + exeExt, "opera.exe"}
	case "safari":
		return []string{"Safari" + exeExt}
	default:
		return []string{lower + exeExt, lower + ".exe"}
	}
}
