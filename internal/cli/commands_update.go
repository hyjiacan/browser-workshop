package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bws/bws/internal/source"
	bwversion "github.com/bws/bws/internal/version"
	"github.com/bws/bws/internal/util"
)

// NewUpdateCommand returns the "update" command which downloads and replaces
// the bws binary from a configured serve source.
func NewUpdateCommand() *Command {
	return &Command{
		Name:        "update",
		Aliases:     []string{"upgrade", "up"},
		Description: "从配置的离线源更新 bws 到最新版本",
		Usage:       "bws update",
		Examples:    []string{"update"},
		Run:         runUpdate,
	}
}

// binListResponse mirrors the serve /api/v1/bin JSON listing.
type binListResponse struct {
	Status string    `json:"status"`
	Data   []binItem `json:"data"`
}

type binItem struct {
	Filename string `json:"filename"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Size     int64  `json:"size"`
}

func runUpdate(ctx *Context, args []string) error {
	remoteSource := strings.TrimRight(ctx.Cfg.Source.GetRemoteSource(), "/")
	if remoteSource == "" {
		return fmt.Errorf("请先配置离线源: bws config set source <url>")
	}

	// Fetch bin listing from serve
	ctx.Printf("正在检查更新 (来源: %s)...\n", remoteSource)
	bins, err := fetchBinList(remoteSource)
	if err != nil {
		return fmt.Errorf("获取可用版本失败: %w", err)
	}
	if len(bins) == 0 {
		return fmt.Errorf("服务端 bin 目录中没有可用的版本文件")
	}

	// Find latest matching platform+arch
	curPlatform := strings.ToLower(string(source.CurrentPlatform()))
	curArch := strings.ToLower(string(source.CurrentArch()))

	var best *binItem
	var bestVersion string
	for i := range bins {
		b := &bins[i]
		if b.Platform != curPlatform || b.Arch != curArch {
			continue
		}
		if bestVersion == "" || bwversion.Compare(b.Version, bestVersion) > 0 {
			best = b
			bestVersion = b.Version
		}
	}
	if best == nil {
		return fmt.Errorf("未找到匹配当前平台 (%s/%s) 的二进制文件", curPlatform, curArch)
	}

	// Compare versions
	currentVersion := bwversion.ClientVersion
	cmp := bwversion.Compare(best.Version, currentVersion)
	if cmp <= 0 {
		ctx.Printf("当前已是最新版本 (%s)\n", currentVersion)
		return nil
	}

	ctx.Printf("发现新版本: %s → %s (%s)\n", currentVersion, best.Version, util.FormatSize(best.Size))

	// Download
	downloadURL := fmt.Sprintf("%s/api/v1/bin/%s", remoteSource, best.Filename)
	ctx.Printf("正在下载 %s ...\n", downloadURL)

	tmpFile, err := downloadToTemp(downloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)
	defer tmpFile.Close()

	// Replace current binary
	if err := replaceBinary(tmpFile.Name()); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}

	ctx.Printf("更新成功! 下次启动将使用新版本 %s\n", best.Version)
	return nil
}

// fetchBinList requests the bin directory listing from a serve server.
func fetchBinList(baseURL string) ([]binItem, error) {
	resp, err := http.Get(baseURL + "/api/v1/bin")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务端返回状态码 %d", resp.StatusCode)
	}

	var result binListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Status != "ok" {
		return nil, fmt.Errorf("服务端返回异常状态: %s", result.Status)
	}
	return result.Data, nil
}

// downloadToTemp downloads a URL to a temporary file and returns the open file.
func downloadToTemp(url string) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "bws-update-*")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return tmpFile, nil
}

// replaceBinary replaces the currently running executable with newPath.
func replaceBinary(newPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	baseDir := filepath.Dir(exePath)
	baseName := filepath.Base(exePath)

	// Determine backup path: bws_old.exe (Windows) or bws.old (Unix)
	var oldBackup string
	if runtime.GOOS == "windows" {
		ext := filepath.Ext(baseName)
		oldBackup = filepath.Join(baseDir, strings.TrimSuffix(baseName, ext)+"_old"+ext)
	} else {
		oldBackup = filepath.Join(baseDir, baseName+".old")
	}

	// Clean up stale backup from a previous update
	_ = os.Remove(oldBackup)

	// Rename the current binary out of the way.  On both Windows and Linux
	// the kernel allows renaming (or unlinking) a running binary because
	// it holds a reference to the underlying inode.
	if err := os.Rename(exePath, oldBackup); err != nil {
		return fmt.Errorf("重命名旧版本失败: %w", err)
	}

	// Write the new binary to the original location.
	if err := copyFile(newPath, exePath); err != nil {
		// Attempt rollback
		_ = os.Rename(oldBackup, exePath)
		return fmt.Errorf("复制新版本失败: %w", err)
	}

	// Success – remove the old backup
	_ = os.Remove(oldBackup)
	return nil
}

// copyFile copies src to dst, preserving the source file's permission bits.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get permissions from source
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
