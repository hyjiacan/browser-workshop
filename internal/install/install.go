// Package install manages browser version installation, uninstallation,
// and tracking of installed versions.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

// Manager handles installation and uninstallation of browser versions.
type Manager struct {
	paths          *paths.Paths
	browsers       *browser.Registry
	systemDetector SystemDetector
}

// NewManager creates a new install manager.
func NewManager(p *paths.Paths, br *browser.Registry) *Manager {
	return &Manager{
		paths:    p,
		browsers: br,
	}
}

// InstallOptions configures an installation.
type InstallOptions struct {
	Browser string
	Version string
	Source  string // e.g. "local-repo", "remote", "import"
	// For local import: source directory path
	SourceDir string
	// For remote download: download URL (handled by download module)
	DownloadURL string
}

// ProgressCallback is called during installation to report progress.
type ProgressCallback func(progress float64, message string)

// InstallFromDir installs a browser version from a local directory (copy mode).
// The source directory should contain the browser executable.
func (m *Manager) InstallFromDir(opts InstallOptions, onProgress ProgressCallback) (*version.InstallRecord, error) {
	log.Info("正在安装 %s@%s，来源目录: %s", opts.Browser, opts.Version, opts.SourceDir)

	if opts.Browser == "" || opts.Version == "" {
		return nil, errors.New("browser and version are required")
	}
	if opts.SourceDir == "" {
		return nil, errors.New("source directory is required")
	}

	// Check if already installed
	if m.IsInstalled(opts.Browser, opts.Version) {
		log.Warn("%s@%s 已安装", opts.Browser, opts.Version)
		return nil, fmt.Errorf("%s@%s 已安装", opts.Browser, opts.Version)
	}

	// Verify source directory exists
	srcInfo, err := os.Stat(opts.SourceDir)
	if err != nil {
		log.Error("源目录不存在: %s", opts.SourceDir)
		return nil, fmt.Errorf("source directory not found: %w", err)
	}
	if !srcInfo.IsDir() {
		log.Error("源路径不是目录: %s", opts.SourceDir)
		return nil, errors.New("source path is not a directory")
	}

	// Verify browser descriptor exists
	desc := m.browsers.Get(opts.Browser)
	if desc == nil {
		log.Error("不支持的浏览器: %s", opts.Browser)
		return nil, fmt.Errorf("unsupported browser: %s", opts.Browser)
	}

	// Verify source has the executable
	execRelPath, err := m.browsers.FindExecutable(opts.Browser, opts.SourceDir, paths.Platform(), paths.Arch())
	if err != nil {
		log.Error("验证源文件失败 %s@%s: %v", opts.Browser, opts.Version, err)
		return nil, fmt.Errorf("validating source: %w", err)
	}

	log.Debug("源文件已验证，可执行文件位置: %s", execRelPath)

	if onProgress != nil {
		onProgress(0.1, "正在准备安装目录...")
	}

	// Prepare destination directory
	destDir := m.paths.VersionDir(opts.Browser, opts.Version)
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		log.Error("创建目标目录失败: %v", err)
		return nil, err
	}

	// Use a temp dir first, then move (atomic-like)
	tmpDir := destDir + ".tmp"
	if err := os.RemoveAll(tmpDir); err != nil {
		log.Error("清理临时目录失败: %v", err)
		return nil, err
	}

	if onProgress != nil {
		onProgress(0.2, "正在复制文件...")
	}

	log.Debug("正在复制文件 %s -> %s", opts.SourceDir, tmpDir)

	// Copy entire directory (streaming progress by file count)
	var copiedFiles int
	var copiedBytes int64
	err = copyDir(opts.SourceDir, tmpDir, func(fileName string, size int64) {
		copiedFiles++
		copiedBytes += size
		log.Debug("已复制: %s (%d 字节)", fileName, size)
		if onProgress != nil {
			// Estimate progress based on files copied + bytes
			// We don't precompute total size (too slow for large dirs),
			// so show incremental progress instead
			onProgress(0.2+0.6*0.5, fmt.Sprintf("正在复制... %d 个文件, %s", copiedFiles, formatBytes(copiedBytes)))
		}
	})
	if err != nil {
		log.Error("复制文件失败: %v", err)
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("copying files: %w", err)
	}

	log.Debug("复制完成: %d 个文件, %s", copiedFiles, formatBytes(copiedBytes))

	if onProgress != nil {
		onProgress(0.85, "正在验证安装...")
	}

	// Verify the copy has the executable
	_, err = m.browsers.FindExecutable(opts.Browser, tmpDir, paths.Platform(), paths.Arch())
	if err != nil {
		log.Error("验证安装失败: %v", err)
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("verifying installation: %w", err)
	}

	// Move temp to final location
	if err := os.Rename(tmpDir, destDir); err != nil {
		log.Error("完成安装失败: %v", err)
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("finalizing installation: %w", err)
	}

	log.Debug("安装已移动到最终位置: %s", destDir)

	if onProgress != nil {
		onProgress(0.95, "正在写入元数据...")
	}

	// Calculate final size (best effort, non-fatal on error)
	installSize, sizeErr := dirSize(destDir)
	if sizeErr != nil {
		installSize = 0
		log.Warn("计算安装大小失败: %v", sizeErr)
	}

	// Create install record
	record := &version.InstallRecord{
		Browser:        opts.Browser,
		Version:        opts.Version,
		InstalledAt:    time.Now(),
		Platform:       paths.Platform(),
		Arch:           paths.Arch(),
		InstallDir:     destDir,
		ExecutablePath: execRelPath,
		Size:           installSize,
		Source:         opts.Source,
	}

	// Write metadata file
	metaPath := m.paths.VersionMetaFile(opts.Browser, opts.Version)
	if err := writeMeta(metaPath, record); err != nil {
		log.Error("写入元数据失败: %v", err)
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	if onProgress != nil {
		onProgress(1.0, "安装完成")
	}

	log.Info("成功安装 %s@%s (大小: %s)", opts.Browser, opts.Version, formatBytes(installSize))

	return record, nil
}

// Uninstall removes an installed browser version.
func (m *Manager) Uninstall(browserName string, version string) error {
	log.Info("正在卸载 %s@%s", browserName, version)

	if !m.IsInstalled(browserName, version) {
		log.Warn("%s@%s 未安装", browserName, version)
		return fmt.Errorf("%s@%s 未安装", browserName, version)
	}

	dir := m.paths.VersionDir(browserName, version)

	// Remove the metadata file first so IsInstalled immediately returns false
	metaPath := m.paths.VersionMetaFile(browserName, version)
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		log.Error("删除元数据失败 %s@%s: %v", browserName, version, err)
		return fmt.Errorf("removing metadata: %w", err)
	}

	log.Debug("已删除元数据 %s@%s", browserName, version)

	// Remove the entire version directory
	if err := removeAll(dir); err != nil {
		log.Error("删除安装目录失败 %s@%s: %v", browserName, version, err)
		return fmt.Errorf("removing installation: %w", err)
	}

	log.Info("成功卸载 %s@%s", browserName, version)

	return nil
}

// removeAll is a more robust version of os.RemoveAll that works around
// Windows-specific issues where os.RemoveAll may return nil without
// actually deleting the directory.
func removeAll(path string) error {
	// First try os.RemoveAll
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}

	// Check if it was actually removed
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil
	}

	// If still there, try manual recursive removal
	return removeAllRecursive(path)
}

func removeAllRecursive(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return os.Remove(path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		if err := removeAllRecursive(childPath); err != nil {
			return err
		}
	}

	return os.Remove(path)
}

// ProfileDir returns the profile directory for the given browser version.
// If profileName is empty, returns the version-default profile.
func (m *Manager) ProfileDir(browser string, version string, profileName string) string {
	if profileName != "" {
		return filepath.Join(m.paths.RuntimeDir, browser, "profiles", profileName)
	}
	return m.paths.ProfileDir(browser, version)
}

// ResetProfile deletes and recreates the profile directory for a browser version.
func (m *Manager) ResetProfile(browser string, version string, profileName string) error {
	profileDir := m.ProfileDir(browser, version, profileName)

	// Ensure the profile path is within the runtime directory for safety
	runtimeDir := m.paths.RuntimeDir
	absProfile, err := filepath.Abs(profileDir)
	if err != nil {
		return fmt.Errorf("resolving profile path: %w", err)
	}
	absRuntime, err := filepath.Abs(runtimeDir)
	if err != nil {
		return fmt.Errorf("resolving runtime path: %w", err)
	}
	relPath, err := filepath.Rel(absRuntime, absProfile)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == "." {
		return fmt.Errorf("profile path is outside of runtime directory: %s", profileDir)
	}

	// If directory exists, remove it
	if info, err := os.Stat(profileDir); err == nil && info.IsDir() {
		log.Info("正在重置配置: %s", profileDir)
		if err := removeAll(profileDir); err != nil {
			return fmt.Errorf("failed to remove profile directory: %w", err)
		}
	}

	// Recreate empty directory
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	log.Info("配置重置成功: %s", profileDir)
	return nil
}

// ListProfiles returns all profiles for a given browser, including named profiles
// and version-specific default profiles.
type ProfileInfo struct {
	Name    string
	Path    string
	Type    string // "named" or "version"
	Version string // for version-type profiles
}

// ListProfiles lists all profiles for the given browser.
// It scans both the named profiles directory and all installed version directories.
func (m *Manager) ListProfiles(browser string) ([]ProfileInfo, error) {
	var result []ProfileInfo

	// Scan named profiles directory
	profilesDir := filepath.Join(m.paths.RuntimeDir, browser, "profiles")
	if entries, err := os.ReadDir(profilesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				result = append(result, ProfileInfo{
					Name: entry.Name(),
					Path: filepath.Join(profilesDir, entry.Name()),
					Type: "named",
				})
			}
		}
	}

	// Scan installed versions for default profiles
	versions, err := m.ListInstalledByBrowser(browser)
	if err != nil {
		return nil, fmt.Errorf("listing installed versions: %w", err)
	}
	for _, v := range versions {
		profilePath := m.paths.ProfileDir(browser, v.Version)
		if info, err := os.Stat(profilePath); err == nil && info.IsDir() {
			result = append(result, ProfileInfo{
				Name:    v.Version + " (默认)",
				Path:    profilePath,
				Type:    "version",
				Version: v.Version,
			})
		}
	}

	return result, nil
}

// CleanOrphanedProfiles removes profile directories for versions that are no longer installed.
// Returns the list of removed profiles.
func (m *Manager) CleanOrphanedProfiles(browser string) ([]string, error) {
	var removed []string

	// Get all installed versions
	installed, err := m.ListInstalledByBrowser(browser)
	if err != nil {
		return nil, fmt.Errorf("listing installed versions: %w", err)
	}
	installedSet := make(map[string]bool)
	for _, v := range installed {
		installedSet[v.Version] = true
	}

	// Scan version directories in runtime
	runtimeBrowserDir := filepath.Join(m.paths.RuntimeDir, browser)
	entries, err := os.ReadDir(runtimeBrowserDir)
	if err != nil {
		if os.IsNotExist(err) {
			return removed, nil
		}
		return nil, fmt.Errorf("reading runtime browser directory: %w", err)
	}

	for _, entry := range entries {
		// Skip the profiles directory (named profiles)
		if entry.Name() == "profiles" {
			continue
		}
		if !entry.IsDir() {
			continue
		}
		// Check if this version is still installed
		version := entry.Name()
		if !installedSet[version] {
			profileDir := filepath.Join(runtimeBrowserDir, version, "profile")
			if info, statErr := os.Stat(profileDir); statErr == nil && info.IsDir() {
				removed = append(removed, profileDir)
			}
		}
	}

	return removed, nil
}

// IsInstalled checks if a browser version is installed.
func (m *Manager) IsInstalled(browserName string, version string) bool {
	metaPath := m.paths.VersionMetaFile(browserName, version)
	_, err := os.Stat(metaPath)
	installed := err == nil
	log.Debug("已安装检查 %s@%s: %v", browserName, version, installed)
	return installed
}

// ListInstalled returns all installed versions across all browsers.
func (m *Manager) ListInstalled() (version.List, error) {
	log.Debug("正在列出所有已安装版本")

	var result version.List

	versionsDir := m.paths.VersionsDir
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Debug("版本目录不存在: %s", versionsDir)
			return result, nil
		}
		log.Error("读取版本目录失败: %v", err)
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		browserName := entry.Name()
		browserDir := filepath.Join(versionsDir, browserName)

		versionDirs, err := os.ReadDir(browserDir)
		if err != nil {
			log.Debug("读取浏览器目录失败 %s: %v", browserDir, err)
			continue
		}

		for _, vEntry := range versionDirs {
			if !vEntry.IsDir() {
				continue
			}
			record, err := m.readRecord(browserName, vEntry.Name())
			if err != nil {
				log.Debug("读取记录失败 %s@%s: %v", browserName, vEntry.Name(), err)
				continue
			}
			result = append(result, record.ToVersion())
		}
	}

	log.Debug("发现 %d 个已安装版本", len(result))

	return result, nil
}

// ListInstalledByBrowser returns installed versions for a specific browser.
func (m *Manager) ListInstalledByBrowser(browserName string) (version.List, error) {
	log.Debug("正在列出 %s 的已安装版本", browserName)
	all, err := m.ListInstalled()
	if err != nil {
		return nil, err
	}
	result := all.Filter(version.Filter{Browser: browserName})
	log.Debug("发现 %d 个 %s 的已安装版本", len(result), browserName)
	return result, nil
}

// GetRecord returns the install record for a specific version.
func (m *Manager) GetRecord(browserName string, ver string) (*version.InstallRecord, error) {
	log.Debug("正在获取安装记录 %s@%s", browserName, ver)
	if !m.IsInstalled(browserName, ver) {
		return nil, fmt.Errorf("%s@%s 未安装", browserName, ver)
	}
	return m.readRecord(browserName, ver)
}

// GetExecutablePath returns the full path to the executable for an installed version.
func (m *Manager) GetExecutablePath(browserName string, ver string) (string, error) {
	log.Debug("正在获取可执行文件路径 %s@%s", browserName, ver)
	record, err := m.GetRecord(browserName, ver)
	if err != nil {
		return "", err
	}
	return filepath.Join(record.InstallDir, record.ExecutablePath), nil
}

// ResolveInstalledVersion resolves a version spec to an actual installed version.
// It supports:
//   - Exact version: returns the version as-is if installed (local or system)
//   - Partial version (e.g. "76"): returns the latest installed version matching the prefix
//   - "latest": returns the latest installed version
//   - "system": returns the system browser version (if available)
func (m *Manager) ResolveInstalledVersion(browserName string, ver string) (string, error) {
	log.Debug("正在解析已安装版本 %s@%s", browserName, ver)

	if browserName == "" || ver == "" {
		return "", errors.New("browser and version are required")
	}

	// Handle "system" special version
	if ver == "system" {
		if m.systemDetector != nil {
			sb, found := m.GetSystemDefault(browserName)
			if found {
				log.Debug("已解析系统版本 %s: %s", browserName, sb.Version)
				return sb.Version, nil
			}
		}
		return "", fmt.Errorf("未找到 %s 的系统浏览器", browserName)
	}

	// Get all installed versions (local + system) for this browser
	allVersions, err := m.ListWithSystemByBrowser(browserName)
	if err != nil {
		return "", fmt.Errorf("listing installed versions: %w", err)
	}

	if len(allVersions) == 0 {
		return "", fmt.Errorf("%s 没有已安装的版本", browserName)
	}

	// Handle "latest" special version
	if ver == "latest" {
		latest, ok := allVersions.Latest()
		if !ok {
			return "", fmt.Errorf("%s 没有最新版本", browserName)
		}
		log.Debug("已解析最新版本 %s: %s", browserName, latest.Version)
		return latest.Version, nil
	}

	// Check for exact match first
	for _, v := range allVersions {
		if v.Version == ver {
			log.Debug("已解析精确版本 %s: %s (系统: %v)", browserName, ver, v.IsSystem)
			return ver, nil
		}
	}

	// Try prefix match (partial version like "76" or "76.0")
	var matches version.List
	prefix := ver + "."
	for _, v := range allVersions {
		if v.Version == ver || strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) > 0 {
		latest, _ := matches.Latest()
		log.Debug("已解析前缀版本 %s@%s: %s", ver, browserName, latest.Version)
		return latest.Version, nil
	}

	return "", fmt.Errorf("%s@%s 未安装", browserName, ver)
}

// FindMatchingVersions returns all installed versions matching the given version query,
// sorted descending (newest first). The first element is the one that would be selected.
func (m *Manager) FindMatchingVersions(browserName string, ver string) (version.List, error) {
	log.Debug("正在查找匹配版本 %s@%s", browserName, ver)

	if browserName == "" || ver == "" {
		return nil, errors.New("browser and version are required")
	}

	// Handle "system" special version
	if ver == "system" {
		if m.systemDetector != nil {
			sb, found := m.GetSystemDefault(browserName)
			if found {
				log.Debug("发现系统版本 %s: %s", browserName, sb.Version)
				return version.List{systemBrowserToVersion(sb)}, nil
			}
		}
		return nil, fmt.Errorf("未找到 %s 的系统浏览器", browserName)
	}

	// Get all installed versions (local + system) for this browser
	allVersions, err := m.ListWithSystemByBrowser(browserName)
	if err != nil {
		return nil, fmt.Errorf("listing installed versions: %w", err)
	}

	if len(allVersions) == 0 {
		return nil, fmt.Errorf("%s 没有已安装的版本", browserName)
	}

	// Handle "latest" special version: return all versions sorted descending
	if ver == "latest" {
		sorted := allVersions.Sort(true) // descending
		log.Debug("发现 %d 个 %s 的版本 (latest)", len(sorted), browserName)
		return sorted, nil
	}

	// Check for exact match first
	for _, v := range allVersions {
		if v.Version == ver {
			log.Debug("发现精确版本匹配 %s: %s", browserName, ver)
			return version.List{v}, nil
		}
	}

	// Try prefix match (partial version like "76" or "76.0")
	var matches version.List
	prefix := ver + "."
	for _, v := range allVersions {
		if v.Version == ver || strings.HasPrefix(v.Version, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) > 0 {
		sorted := matches.Sort(true) // descending
		log.Debug("发现 %d 个前缀匹配 %s@%s", len(sorted), browserName, ver)
		return sorted, nil
	}

	return nil, fmt.Errorf("%s@%s 未安装", browserName, ver)
}

// readRecord reads the install record from .bws.json.
func (m *Manager) readRecord(browserName string, ver string) (*version.InstallRecord, error) {
	metaPath := m.paths.VersionMetaFile(browserName, ver)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var record version.InstallRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	// Ensure InstallDir is absolute and correct
	if record.InstallDir == "" {
		record.InstallDir = m.paths.VersionDir(browserName, ver)
	}

	return &record, nil
}

// writeMeta writes the install record to .bws.json.
func writeMeta(path string, record *version.InstallRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// dirSize calculates the total size of a directory.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes formats a byte count into a human-readable string.
func formatBytes(bytes int64) string {
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

// copyDir copies a directory recursively.
// The callback is called for each file with the file name and size.
func copyDir(src string, dst string, onFile func(fileName string, size int64)) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath, onFile); err != nil {
				return err
			}
		} else {
			size, err := copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
			if onFile != nil {
				onFile(entry.Name(), size)
			}
		}
	}

	return nil
}

// copyFile copies a single file, preserving permissions.
func copyFile(src string, dst string) (int64, error) {
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
	defer dstFile.Close()

	n, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return n, err
	}

	return n, nil
}
