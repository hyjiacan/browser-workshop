// Package archive provides unified archive extraction.
// It prefers 7-Zip for multi-format support, with native zip as fallback.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// supportedFormats lists all archive format extensions we can handle.
// Order matters for compound extensions like .tar.gz.
var supportedFormats = []string{
	".zip",
	".7z",
	".rar",
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".gz",
	".bz2",
	".xz",
	".zst",
	".exe", // self-extracting archives
	".msi", // Windows installers (extractable via 7z)
	".dmg", // macOS disk images (extractable via 7z)
	".pkg", // macOS packages (extractable via 7z)
	".deb", // Debian packages (extractable via 7z)
	".rpm", // RPM packages (extractable via 7z)
	".apk", // Android APKs (zip-based)
	".jar", // Java archives (zip-based)
	".war", // Web archives (zip-based)
	".cab", // Windows cabinet files
	".iso", // ISO images (extractable via 7z)
	".wim", // Windows imaging format
}

// nestedArchiveFormats lists formats that are definitely pure archives and safe
// to extract during recursive nested extraction. This excludes executable and
// installer formats (.exe, .msi, .dmg, etc.) which might be real programs
// rather than self-extracting archives, preventing over-extraction of actual
// browser binaries like chrome.exe.
var nestedArchiveFormats = []string{
	".zip",
	".7z",
	".rar",
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".gz",
	".bz2",
	".xz",
	".zst",
	".jar",
	".war",
	".cab",
}

// SupportedFormats returns the list of supported archive format extensions.
func SupportedFormats() []string {
	result := make([]string, len(supportedFormats))
	copy(result, supportedFormats)
	return result
}

// IsSupportedFormat checks if a file has a supported archive extension.
func IsSupportedFormat(path string) bool {
	return hasSupportedFormat(path, supportedFormats)
}

// isNestedArchiveFormat checks if a file has an extension that is definitely
// a pure archive format (safe for recursive nested extraction).
func isNestedArchiveFormat(path string) bool {
	return hasSupportedFormat(path, nestedArchiveFormats)
}

func hasSupportedFormat(path string, formats []string) bool {
	lower := strings.ToLower(path)
	for _, ext := range formats {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// detectFormat returns the archive format extension from a file path.
// It checks compound extensions first (e.g., .tar.gz before .gz).
func detectFormat(path string) string {
	lower := strings.ToLower(path)
	for _, ext := range supportedFormats {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	// Fallback: last extension
	ext := filepath.Ext(lower)
	if ext != "" {
		return ext
	}
	return ""
}

// Extract extracts an archive file to the destination directory.
// It auto-detects the best extraction method based on file format and available tools.
// For .zip files, native Go extraction is used first.
// For other formats, 7-Zip is used if available.
func Extract(srcPath, destDir string) error {
	// Ensure source file exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("source file not found: %s", srcPath)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	format := detectFormat(srcPath)
	has7z := Has7z()

	// Strategy based on format
	switch {
	case format == ".zip":
		// Try native zip first
		err := extractZipNative(srcPath, destDir)
		if err == nil {
			return nil
		}
		// Fall back to 7z if available
		if has7z {
			return extractWith7z(srcPath, destDir)
		}
		return fmt.Errorf("extracting zip: %w", err)

	case format == ".exe":
		// For .exe files, try native zip first (self-extracting archives are often zips)
		if isZipFile(srcPath) {
			err := extractZipNative(srcPath, destDir)
			if err == nil {
				return nil
			}
		}
		// Then try 7z
		if has7z {
			return extractWith7z(srcPath, destDir)
		}
		return fmt.Errorf("cannot extract .exe file: 7-Zip not available. "+
			"Install 7-Zip from https://www.7-zip.org/ to extract %s and other formats", format)

	case format == ".jar", format == ".war", format == ".apk":
		// These are zip-based, try native first
		err := extractZipNative(srcPath, destDir)
		if err == nil {
			return nil
		}
		if has7z {
			return extractWith7z(srcPath, destDir)
		}
		return fmt.Errorf("extracting %s: %w", format, err)

	default:
		// For all other formats, require 7z
		if has7z {
			return extractWith7z(srcPath, destDir)
		}
		return fmt.Errorf("cannot extract %s archive: 7-Zip is not available. "+
			"Install 7-Zip from https://www.7-zip.org/ to enable multi-format support", format)
	}
}

// ExtractRecursive extracts an archive and recursively extracts any nested archives.
// It scans the extracted directory for archive files and extracts them in-place,
// continuing until no more archives are found.
// Returns the path to destDir (the root directory containing the final extracted content).
func ExtractRecursive(srcPath, destDir string) (string, error) {
	// Step 1: Extract the initial archive
	if err := Extract(srcPath, destDir); err != nil {
		return "", fmt.Errorf("initial extraction: %w", err)
	}

	// Step 2: Recursively extract any nested archives
	if err := extractNested(destDir); err != nil {
		return "", fmt.Errorf("extracting nested archives: %w", err)
	}

	return destDir, nil
}

// maxExtractPasses is the maximum number of extraction passes for nested archives.
// This prevents infinite loops in edge cases.
const maxExtractPasses = 10

// extractNested walks the directory tree and extracts any archive files found.
// It repeats the process until no more archives are found.
func extractNested(rootDir string) error {
	for passes := 0; passes < maxExtractPasses; passes++ {
		found, err := extractNestedOnce(rootDir)
		if err != nil {
			return err
		}
		if !found {
			// No more archives found, we're done
			return nil
		}
		// We found and extracted at least one archive; loop again to check for more
	}
	return fmt.Errorf("exceeded maximum nested extraction passes (%d)", maxExtractPasses)
}

// extractNestedOnce performs one pass of the directory tree, extracting any
// archive files found. Returns true if at least one archive was extracted.
func extractNestedOnce(rootDir string) (bool, error) {
	var archives []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip files we've already processed (marked with sentinel suffixes)
		if strings.HasSuffix(path, ".bm_done") || strings.HasSuffix(path, ".extracted") {
			return nil
		}
		if isNestedArchiveFormat(path) {
			archives = append(archives, path)
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("scanning for nested archives: %w", err)
	}

	if len(archives) == 0 {
		return false, nil
	}

	// Extract each archive found
	extractedAny := false
	for _, archivePath := range archives {
		// Create extraction directory name based on the archive file name
		extractDir := deriveExtractDir(archivePath)

		// Ensure the extraction directory doesn't already exist
		if _, err := os.Stat(extractDir); err == nil {
			// Directory already exists; append a suffix to avoid collision
			extractDir = extractDir + "_extracted"
		}

		if err := Extract(archivePath, extractDir); err != nil {
			// If extraction fails, skip this archive but continue with others
			// This could happen if the file has an archive extension but isn't actually an archive
			// (e.g. a regular .exe file that isn't a self-extracting archive)
			continue
		}

		// Remove the archive file after successful extraction
		if err := os.Remove(archivePath); err != nil {
			// If we can't delete the archive, rename it to avoid infinite re-extraction
			renamedPath := archivePath + ".extracted"
			if renameErr := os.Rename(archivePath, renamedPath); renameErr != nil {
				// If even rename fails, at least mark it by touching a sentinel file
				// to prevent infinite loop in extractNested
				sentinelPath := archivePath + ".bm_done"
				os.WriteFile(sentinelPath, []byte("extracted"), 0o644)
			}
		}
		extractedAny = true
	}

	return extractedAny, nil
}

// deriveExtractDir returns the directory path where an archive should be extracted.
// It strips the archive extension from the file name and uses that as the directory name.
func deriveExtractDir(archivePath string) string {
	dir := filepath.Dir(archivePath)
	base := filepath.Base(archivePath)
	lowerBase := strings.ToLower(base)

	// Check compound extensions first (e.g., .tar.gz)
	for _, ext := range supportedFormats {
		if strings.HasSuffix(lowerBase, ext) {
			name := base[:len(base)-len(ext)]
			if name == "" {
				name = base + "_contents"
			}
			return filepath.Join(dir, name)
		}
	}

	// Fallback: strip last extension
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	if name == "" {
		name = base + "_contents"
	}
	return filepath.Join(dir, name)
}

// --- 7-Zip integration ---

var cached7zPath string
var cached7zChecked bool

// Has7z checks if the 7z executable is available on the system.
func Has7z() bool {
	path := find7z()
	return path != ""
}

// find7z locates the 7z executable, checking PATH and common install locations.
// Results are cached after the first call.
func find7z() string {
	if cached7zChecked {
		return cached7zPath
	}
	cached7zChecked = true

	// Check PATH first
	if path, err := exec.LookPath("7z"); err == nil {
		cached7zPath = path
		return path
	}
	// Also try 7za (standalone version)
	if path, err := exec.LookPath("7za"); err == nil {
		cached7zPath = path
		return path
	}

	// Check common install locations by platform
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files\7-Zip\7z.exe`,
			`C:\Program Files (x86)\7-Zip\7z.exe`,
			filepath.Join(os.Getenv("ProgramFiles"), "7-Zip", "7z.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "7-Zip", "7z.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "7-Zip", "7z.exe"),
		}
	case "darwin":
		candidates = []string{
			"/opt/homebrew/bin/7z",
			"/usr/local/bin/7z",
			"/Applications/Keka.app/Contents/MacOS/Keka",
		}
	case "linux":
		candidates = []string{
			"/usr/bin/7z",
			"/usr/bin/7za",
			"/usr/local/bin/7z",
			"/snap/bin/7z",
		}
	default:
		candidates = []string{
			"/usr/bin/7z",
			"/usr/local/bin/7z",
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			cached7zPath = candidate
			return candidate
		}
	}

	return ""
}

// extractWith7z extracts an archive using the 7z command-line tool.
func extractWith7z(srcPath, destDir string) error {
	exePath := find7z()
	if exePath == "" {
		return fmt.Errorf("7z executable not found")
	}

	// 7z x <src> -o<dest> -y
	// x = extract with full paths
	// -o = output directory
	// -y = assume yes to all queries
	cmd := exec.Command(exePath, "x", srcPath, "-o"+destDir, "-y")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("7z extraction failed: %w", err)
	}

	return nil
}

// --- Native zip extraction ---

// extractZipNative extracts a zip archive using Go's archive/zip package.
func extractZipNative(srcPath, destDir string) error {
	r, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Sanitize path to prevent zip slip
		fpath := filepath.Join(destDir, f.Name)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(fpath), cleanDest) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// isZipFile checks if a file is actually a zip archive by reading its header.
func isZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 4)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return false
	}

	// ZIP magic number: PK\x03\x04
	return buf[0] == 'P' && buf[1] == 'K' && buf[2] == 0x03 && buf[3] == 0x04
}

// --- Browser executable detection ---

// FindBrowserExe searches for a browser executable in the extracted directory.
// It looks for known executable names up to maxDepth levels deep.
// Returns the directory containing the executable, or an error if not found.
func FindBrowserExe(rootDir, browserName, platform, arch string, executableNames []string) (string, error) {
	if len(executableNames) == 0 {
		// Fallback: use browser name as executable
		exeName := browserName
		if platform == "windows" {
			exeName += ".exe"
		}
		executableNames = []string{exeName}
	}

	// Search up to 3 levels deep
	const maxDepth = 3

	var result string
	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		if depth > maxDepth {
			return false
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}

		// Check for executable files in current directory
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			for _, exeName := range executableNames {
				if strings.EqualFold(entry.Name(), exeName) {
					result = dir
					return true
				}
			}
		}

		// Recurse into subdirectories
		for _, entry := range entries {
			if entry.IsDir() {
				if walk(filepath.Join(dir, entry.Name()), depth+1) {
					return true
				}
			}
		}

		return false
	}

	if walk(rootDir, 0) {
		return result, nil
	}

	return "", fmt.Errorf("browser executable not found in %s (searched %d levels deep for: %s)",
		rootDir, maxDepth, strings.Join(executableNames, ", "))
}

// FindContentDir finds the actual content directory within an extracted archive.
// It first tries to find a browser executable. If not found, it falls back to
// the single-subdirectory heuristic.
func FindContentDir(root string, browserName string, platform string, arch string, exeCandidates []string) (string, error) {
	// Try to find browser executable first (most reliable)
	if len(exeCandidates) > 0 {
		if dir, err := FindBrowserExe(root, browserName, platform, arch, exeCandidates); err == nil {
			return dir, nil
		}
	}

	// Fallback: single subdirectory heuristic
	return findContentDirHeuristic(root)
}

// findContentDirHeuristic finds the content directory using the single-subdir heuristic.
// If the archive contains a single top-level directory, it recurses into it.
func findContentDirHeuristic(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}

	var dirs []os.DirEntry
	var files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	// If there are files at root level, the root itself is the content dir
	if len(files) > 0 {
		return root, nil
	}

	// If there's exactly one directory, look inside it
	if len(dirs) == 1 {
		return findContentDirHeuristic(filepath.Join(root, dirs[0].Name()))
	}

	// Multiple directories or empty - return root
	return root, nil
}
