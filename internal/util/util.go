// Package util provides shared utility functions used across multiple
// internal packages (repo, install, serve, log). Extracting these here
// eliminates previously duplicated code.
package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bws/bws/internal/archive"
	"github.com/bws/bws/internal/paths"
)

// --- Browser executable helpers ---

// BrowserExecutableCandidates returns common executable file names for a
// given browser. These are used to locate the browser executable inside
// extracted archives.
func BrowserExecutableCandidates(browserName string) []string {
	lower := strings.ToLower(browserName)

	// Platform-specific extensions
	exeExt := ""
	if paths.Platform() == "windows" {
		exeExt = ".exe"
	}

	switch lower {
	case "chrome", "google chrome", "google-chrome":
		return []string{
			"chrome" + exeExt,
			"chrome.exe", // always include .exe variant for archives from other platforms
			"Google Chrome" + exeExt,
		}
	case "firefox", "mozilla firefox", "mozilla-firefox":
		return []string{
			"firefox" + exeExt,
			"firefox.exe",
		}
	case "chromium":
		return []string{
			"chromium" + exeExt,
			"chromium.exe",
			"chrome" + exeExt,
		}
	case "edge", "microsoft edge", "microsoft-edge", "msedge":
		return []string{
			"msedge" + exeExt,
			"msedge.exe",
			"edge" + exeExt,
		}
	case "brave":
		return []string{
			"brave" + exeExt,
			"brave.exe",
			"brave-browser" + exeExt,
		}
	case "opera":
		return []string{
			"opera" + exeExt,
			"opera.exe",
		}
	case "safari":
		return []string{
			"Safari" + exeExt,
		}
	default:
		// Generic fallback: browser name as executable
		return []string{
			lower + exeExt,
			lower + ".exe",
		}
	}
}

// FindContentDir finds the actual content directory within an extracted
// archive. It first tries to locate the browser executable (by common names),
// then falls back to the single-subdirectory heuristic.
func FindContentDir(root string, browserName string) (string, error) {
	// Get common executable names for this browser
	exeCandidates := BrowserExecutableCandidates(browserName)

	// Use the archive package's FindContentDir which tries executable search first
	return archive.FindContentDir(root, browserName, paths.Platform(), paths.Arch(), exeCandidates)
}

// --- Installer extension helpers ---

// InstallerExtensions lists known installer/archive extensions that should be
// stripped. Order matters: compound extensions like .tar.gz must come before
// .gz.
var InstallerExtensions = []string{
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".exe",
	".msi",
	".zip",
	".7z",
	".rar",
	".dmg",
	".pkg",
	".deb",
	".rpm",
	".apk",
	".gz",
	".bz2",
	".xz",
}

// StripExtension removes known installer/archive extensions from a filename.
// If no known extension is found, it removes the last extension using
// filepath.Ext.
func StripExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range InstallerExtensions {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	// Fallback: remove last extension
	ext := filepath.Ext(name)
	if ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

// --- Size formatting ---

// FormatSize formats a byte count into a human-readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// --- File copy ---

// CopyFile copies a single file from src to dst, preserving permissions.
// It returns the number of bytes copied.
func CopyFile(src string, dst string) (int64, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return 0, err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(dstFile, srcFile)
	if err != nil {
		dstFile.Close()
		return n, err
	}
	if err := dstFile.Sync(); err != nil {
		dstFile.Close()
		return n, err
	}
	if err := dstFile.Close(); err != nil {
		return n, err
	}
	return n, nil
}
