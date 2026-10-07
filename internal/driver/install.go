package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bws/bws/internal/archive"
	"github.com/bws/bws/internal/download"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/util"
)

// EnsureOptions configures an Ensure call.
type EnsureOptions struct {
	// ChromeVersion is the Chrome build the driver must match.
	ChromeVersion string

	// Platform and Arch default to the current platform/arch when empty.
	Platform string
	Arch     string

	// Force reinstalls even when a matching driver is already present.
	Force bool

	// OnProgress reports download progress (may be nil).
	OnProgress func(downloaded, total int64, percent float64)
}

// Ensure makes sure a driver matching the given Chrome version is installed
// and returns its record. When a matching driver is already present the
// existing record is returned without touching the network.
func (m *Manager) Ensure(ctx context.Context, opts EnsureOptions) (*Record, error) {
	platform := opts.Platform
	if platform == "" {
		platform = paths.Platform()
	}
	arch := opts.Arch
	if arch == "" {
		arch = paths.Arch()
	}

	info, err := m.resolveSources(ctx, opts.ChromeVersion, platform, arch)
	if err != nil {
		return nil, err
	}

	major := info.MajorVersion
	if major == "" {
		major = majorKey(info.Version)
		info.MajorVersion = major
	}

	if !opts.Force {
		if rec, err := m.readRecord(info.Name, major); err == nil {
			if rec.Version == info.Version && fileExists(rec.ExecutablePath()) {
				bmlog.Debug("[driver] 已安装匹配的 %s %s，跳过下载", info.Name, rec.Version)
				return rec, nil
			}
			bmlog.Debug("[driver] 已安装的 %s 版本为 %s，需要 %s，将重新安装",
				info.Name, rec.Version, info.Version)
		}
	}

	return m.install(ctx, info, opts.OnProgress)
}

// Resolve returns the driver build matching chromeVersion without installing.
func (m *Manager) Resolve(ctx context.Context, chromeVersion, platform, arch string) (*Info, error) {
	if platform == "" {
		platform = paths.Platform()
	}
	if arch == "" {
		arch = paths.Arch()
	}
	return m.resolveSources(ctx, chromeVersion, platform, arch)
}

// Install downloads and installs the given driver build.
func (m *Manager) Install(ctx context.Context, info *Info, onProgress func(downloaded, total int64, percent float64)) (*Record, error) {
	return m.install(ctx, info, onProgress)
}

// install performs the download + extract + register sequence.
func (m *Manager) install(ctx context.Context, info *Info, onProgress func(downloaded, total int64, percent float64)) (*Record, error) {
	major := info.MajorVersion
	if major == "" {
		major = majorKey(info.Version)
	}

	bmlog.Info("[driver] 正在安装 %s %s (%s/%s)", info.Name, info.Version, info.Platform, info.Arch)

	// Download into the shared download cache.
	cacheDir := filepath.Join(m.paths.DownloadCacheDir, "drivers")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建驱动下载缓存目录失败: %w", err)
	}
	archivePath := filepath.Join(cacheDir, info.Filename)

	// Only a build resolved through serve is downloaded from serve; upstream
	// Chrome for Testing must never receive the serve bearer token.
	authToken := ""
	if info.Source == serveSourceName {
		authToken = m.authToken
	}

	if !fileExists(archivePath) {
		// No checksum verification here: Chrome for Testing publishes download
		// URLs only (no sha256/hash), and the legacy storage bucket is the same.
		// Integrity therefore rests on HTTPS in transit plus the zip CRC that
		// archive.Extract validates below. Upstream checksums are verified when
		// a source provides them (see serve/sync.go for browser packages).
		dlMgr := download.NewManagerWithProxy(m.proxyURL)
		if _, err := dlMgr.Download(ctx, download.Options{
			URL:       info.DownloadURL,
			DestPath:  archivePath,
			Resume:    true,
			AuthToken: authToken,
			OnProgress: func(p download.Progress) {
				if onProgress != nil {
					onProgress(p.Downloaded, p.Total, p.Percent)
				}
			},
			ProgressInterval: 100 * time.Millisecond,
		}); err != nil {
			return nil, fmt.Errorf("下载驱动失败: %w", err)
		}
	} else {
		bmlog.Debug("[driver] 命中下载缓存: %s", archivePath)
	}

	// Extract into a temporary directory under the drivers root so the final
	// move is a same-volume rename.
	driverRoot := filepath.Join(m.paths.DriversDir, info.Name)
	tmpDir := filepath.Join(driverRoot, ".tmp-"+info.Version)
	if err := os.RemoveAll(tmpDir); err != nil {
		return nil, fmt.Errorf("清理临时目录失败: %w", err)
	}
	if err := archive.Extract(archivePath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("解压驱动失败: %w", err)
	}

	execRel, err := findDriverBinary(tmpDir, info.Name)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	destDir := m.paths.DriverDir(info.Name, major)
	if err := os.RemoveAll(destDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("清理旧版本目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		// Fall back to a copy when the rename crosses devices.
		if cerr := copyTree(tmpDir, destDir); cerr != nil {
			os.RemoveAll(tmpDir)
			return nil, fmt.Errorf("移动驱动到安装目录失败: %w", err)
		}
		os.RemoveAll(tmpDir)
	}

	rec := &Record{
		Name:         info.Name,
		Version:      info.Version,
		MajorVersion: major,
		Platform:     info.Platform,
		Arch:         info.Arch,
		Dir:          destDir,
		Executable:   execRel,
		Size:         dirSize(destDir),
		Source:       info.Source,
		InstalledAt:  time.Now(),
	}
	if err := m.writeRecord(rec); err != nil {
		return nil, fmt.Errorf("写入驱动元数据失败: %w", err)
	}

	bmlog.Info("[driver] %s %s 安装完成 (%s)", info.Name, info.Version, util.FormatSize(rec.Size))
	return rec, nil
}

// List returns all installed driver records.
func (m *Manager) List() ([]Record, error) {
	return m.ListByName(NameChromedriver)
}

// ListByName returns installed records for a specific driver name.
func (m *Manager) ListByName(name string) ([]Record, error) {
	root := filepath.Join(m.paths.DriversDir, name)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var result []Record
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		rec, err := m.readRecord(name, entry.Name())
		if err != nil {
			bmlog.Debug("[driver] 跳过无法读取的记录 %s/%s: %v", name, entry.Name(), err)
			continue
		}
		if !fileExists(rec.ExecutablePath()) {
			bmlog.Debug("[driver] 记录 %s/%s 的可执行文件缺失，跳过", name, entry.Name())
			continue
		}
		result = append(result, *rec)
	}
	return result, nil
}

// IsInstalled reports whether the driver major is installed with the exact
// version.
func (m *Manager) IsInstalled(major, driverVersion string) bool {
	rec, err := m.readRecord(NameChromedriver, major)
	if err != nil {
		return false
	}
	if driverVersion != "" && rec.Version != driverVersion {
		return false
	}
	return fileExists(rec.ExecutablePath())
}

// BinaryPath returns the absolute path of the installed driver binary for the
// given major version.
func (m *Manager) BinaryPath(major string) (string, error) {
	rec, err := m.readRecord(NameChromedriver, major)
	if err != nil {
		return "", fmt.Errorf("未安装 %s %s: %w", NameChromedriver, major, err)
	}
	path := rec.ExecutablePath()
	if !fileExists(path) {
		return "", fmt.Errorf("%s %s 的可执行文件缺失: %s", NameChromedriver, major, path)
	}
	return path, nil
}

// Uninstall removes an installed driver major version.
func (m *Manager) Uninstall(major string) error {
	dir := m.paths.DriverDir(NameChromedriver, major)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("%s %s 未安装", NameChromedriver, major)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("移除驱动目录失败: %w", err)
	}
	bmlog.Info("[driver] 已卸载 %s %s", NameChromedriver, major)
	return nil
}

// findDriverBinary locates the driver executable inside an extracted archive
// and returns its path relative to root.
func findDriverBinary(root, name string) (string, error) {
	target := strings.ToLower(name)
	var found string
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		base := strings.ToLower(fi.Name())
		base = strings.TrimSuffix(base, ".exe")
		if base == target {
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			found = rel
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return "", fmt.Errorf("扫描驱动文件失败: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("解压结果中未找到 %s 可执行文件", name)
	}
	return found, nil
}

// copyTree recursively copies src into dst, preserving file modes.
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
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
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if _, err := util.CopyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// dirSize returns the total size of a directory (0 on error).
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		size += fi.Size()
		return nil
	})
	return size
}

// fileExists reports whether path exists and is not a directory.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
