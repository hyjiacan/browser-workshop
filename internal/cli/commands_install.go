package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runInstall(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "from-dir", Short: "d", Usage: "从本地目录安装（未指定版本时自动检测）", HasValue: true, Default: ""},
		{Name: "from-file", Short: "", Usage: "从本地压缩包安装（未指定版本时自动检测）", HasValue: true, Default: ""},
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
		{Name: "channel", Short: "c", Usage: "发布渠道", HasValue: true, Default: "stable"},
		{Name: "refresh-cache", Short: "", Usage: "强制从 serve 重新下载（忽略本地缓存）", HasValue: false, Default: "false"},
	}

	flagVals, positional, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}

	fromDir := flagVals["from-dir"]
	fromFile := flagVals["from-file"]
	force := flagVals["force"] == "true"
	channel := flagVals["channel"]
	refreshCache := flagVals["refresh-cache"] == "true"

	if len(positional) == 0 && fromDir == "" && fromFile == "" {
		return fmt.Errorf("请指定要安装的版本，例如 'bws i chrome@120'")
	}

	spec := browserVersionSpec{Browser: ctx.Cfg.Defaults.DefaultBrowser(), Version: "latest", IsAlias: true}
	if len(positional) > 0 {
		spec = resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())
	}

	// Check disk space before any install operation
	dataDir := "."
	if ctx.Cfg != nil {
		dataDir = ctx.Cfg.Data.GetDataDir()
	}
	if err := checkDiskSpace(ctx, dataDir); err != nil {
		return err
	}

	if ctx.Logger != nil {
		if fromDir != "" {
			ctx.Logger.Debug("[install] 从本地目录安装: dir=%s spec=%s@%s force=%v", fromDir, spec.Browser, spec.Version, force)
		} else if fromFile != "" {
			ctx.Logger.Debug("[install] 从本地文件安装: file=%s spec=%s@%s force=%v", fromFile, spec.Browser, spec.Version, force)
		} else {
		ctx.Logger.Debug("[install] 远程安装: spec=%s@%s channel=%s force=%v refresh-cache=%v", spec.Browser, spec.Version, channel, force, refreshCache)
	}
	}

	// 本地目录安装
	if fromDir != "" {
		// 如果未指定版本，自动检测
		if spec.Version == "latest" && spec.IsAlias {
			ctx.Printf("正在扫描 %s...\n", fromDir)
			lastMsg := ""
			summary, err := ctx.Install.ImportFromDir(fromDir, force, func(current int, total int, message string) {
				if message != lastMsg {
					fmt.Fprintf(ctx.Stdout, "  %s\n", message)
					lastMsg = message
				}
			})
			if err != nil {
				return err
			}
			if summary.Total == 0 {
				return fmt.Errorf("在 %s 中未找到可识别的浏览器版本", fromDir)
			}
			ctx.Printf("\n导入完成：\n")
			ctx.Printf("  总计:    %d\n", summary.Total)
			ctx.Printf("  成功:    %d\n", summary.Success)
			ctx.Printf("  跳过:    %d\n", summary.Skipped)
			ctx.Printf("  失败:    %d\n", summary.Failed)
			if summary.Failed > 0 {
				return fmt.Errorf("%d 个导入失败", summary.Failed)
			}
			return nil
		}

		// 指定了版本，从目录中查找并安装
		ctx.Printf("正在从 %s 安装 %s@%s...\n", fromDir, spec.Browser, spec.Version)
		if ctx.Logger != nil {
			ctx.Logger.Debug("[install] 开始从目录安装 %s@%s (来源: %s)", spec.Browser, spec.Version, fromDir)
		}
		installStart := time.Now()
		record, err := ctx.Install.InstallFromDir(spec.Browser, spec.Version, fromDir)
		if err != nil {
			return fmt.Errorf("安装失败: %w", err)
		}
		if ctx.Logger != nil {
			ctx.Logger.Debug("[install] 安装完成: 路径=%s 版本=%s 总耗时=%s",
				record.InstallDir, record.Version, formatDuration(time.Since(installStart)))
		}
		ctx.Printf("✓ %s@%s 安装成功\n", record.Browser, record.Version)
		return nil
	}

	// 本地文件安装
	if fromFile != "" {
		ctx.Printf("正在从 %s 安装 %s@%s...\n", fromFile, spec.Browser, spec.Version)
		if ctx.Logger != nil {
			ctx.Logger.Debug("[install] 开始从文件安装（解压） %s@%s (来源: %s)", spec.Browser, spec.Version, fromFile)
		}
		installStart := time.Now()
		record, err := ctx.Install.InstallFromFile(spec.Browser, spec.Version, fromFile)
		if err != nil {
			return fmt.Errorf("安装失败: %w", err)
		}
		if ctx.Logger != nil {
			ctx.Logger.Debug("[install] 安装完成: 路径=%s 版本=%s 总耗时=%s",
				record.InstallDir, record.Version, formatDuration(time.Since(installStart)))
		}
		ctx.Printf("✓ %s@%s 安装成功\n", record.Browser, record.Version)
		return nil
	}

	// 远程下载安装
	if ctx.Source == nil || ctx.Download == nil {
		return fmt.Errorf("当前构建不支持远程下载。请使用 --from-dir 或 --from-file 进行本地安装。")
	}

	// 解析版本（支持部分版本号）
	ctx.Printf("正在解析 %s@%s...\n", spec.Browser, spec.Version)
	versionInfo, err := ctx.Source.ResolveVersion(spec.Browser, spec.Version)
	if err != nil {
		// 如果版本是渠道名，如 latest、beta 等，尝试从指定渠道获取
		if spec.IsAlias {
			versions, listErr := ctx.Source.ListVersions(spec.Browser, channel, "")
			if listErr == nil && len(versions) > 0 {
				versionInfo = versions[0]
			} else {
				return fmt.Errorf("解析版本失败: %w", err)
			}
		} else {
			return fmt.Errorf("解析版本失败: %w", err)
		}
	}

	if versionInfo.DownloadURL == "" {
		return fmt.Errorf("%s@%s 没有可用的下载链接", spec.Browser, versionInfo.Version)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[install] 版本已解析: %s@%s channel=%s platform=%s arch=%s url=%s",
			spec.Browser, versionInfo.Version, versionInfo.Channel, versionInfo.Platform, versionInfo.Arch, versionInfo.DownloadURL)
	}

	// 检查是否已安装（使用解析后的完整版本号）
	if !force && ctx.Install.IsInstalled(spec.Browser, versionInfo.Version) {
		ctx.Printf("%s@%s 已安装\n", spec.Browser, versionInfo.Version)
		return nil
	}

	ctx.Printf("正在下载 %s@%s...\n", spec.Browser, versionInfo.Version)

	// Use permanent download cache directory instead of a temp directory.
	// Cached files are kept across installs and only re-downloaded when
	// the file is missing, corrupt, or --refresh-cache is specified.
	cacheDir := ctx.Paths.DownloadCacheDir()
	if cacheDir == "" {
		return fmt.Errorf("下载缓存目录未配置")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating download cache dir: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[install] 下载缓存目录: %s", cacheDir)
	}

	// Determine filename from URL
	fileName := getDownloadFilename(spec.Browser, versionInfo.Version, versionInfo.DownloadURL, string(versionInfo.Platform))
	downloadDest := filepath.Join(cacheDir, fileName)

	// Check if the file is already cached and valid.
	// A cached file is valid when:
	//   1. --refresh-cache is not set
	//   2. The file exists and is non-empty
	//   3. If the manifest provides a size, it matches the local file size
	cacheValid := false
	if !refreshCache {
		if info, err := os.Stat(downloadDest); err == nil && info.Size() > 0 {
			if versionInfo.Size > 0 && info.Size() != versionInfo.Size {
				if ctx.Logger != nil {
					ctx.Logger.Debug("[install] 缓存文件大小不匹配: 本地=%d 远程=%d，将重新下载", info.Size(), versionInfo.Size)
				}
			} else {
				cacheValid = true
				if ctx.Logger != nil {
					ctx.Logger.Debug("[install] 命中本地缓存: %s (大小: %s)", downloadDest, FormatSize(info.Size()))
				}
				ctx.Printf("✓ 命中本地缓存，跳过下载: %s\n", fileName)
			}
		}
	}

	var downloadedPath string
	if cacheValid {
		downloadedPath = downloadDest
	} else {
		if ctx.Logger != nil {
			ctx.Logger.Debug("[install] 下载目标: %s", downloadDest)
		}

		downloadedPath, err = ctx.Download.Download(versionInfo.DownloadURL, downloadDest, func(downloaded, total int64, percent float64) {
			if total > 0 {
				ctx.Printf("\r  下载进度: %.1f%%", percent)
			} else {
				ctx.Printf("\r  下载中...")
			}
		})
		ctx.Println() // newline after progress

		if err != nil {
			return fmt.Errorf("下载失败: %w", err)
		}
	}

	// 强制模式：下载成功后再卸载旧版本，确保下载失败时不会丢失已安装的版本
	if force && ctx.Install.IsInstalled(spec.Browser, versionInfo.Version) {
		if err := ctx.Install.Uninstall(spec.Browser, versionInfo.Version); err != nil {
			return fmt.Errorf("卸载现有版本失败: %w", err)
		}
		ctx.Printf("已移除现有版本 %s@%s\n", spec.Browser, versionInfo.Version)
	}

	ctx.Printf("正在安装 %s@%s...\n", spec.Browser, versionInfo.Version)

	if ctx.Logger != nil {
		ctx.Logger.Debug("[install] 开始解压并安装 %s@%s", spec.Browser, versionInfo.Version)
	}
	installStart := time.Now()

	// Install from the downloaded file
	record, err := ctx.Install.InstallFromFile(spec.Browser, versionInfo.Version, downloadedPath)
	if err != nil {
		return fmt.Errorf("安装失败: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[install] 注册元数据完成: %s@%s", record.Browser, record.Version)
		ctx.Logger.Debug("[install] 安装完成: 路径=%s 版本=%s 总耗时=%s",
			record.InstallDir, record.Version, formatDuration(time.Since(installStart)))
	}

	ctx.Printf("✓ %s@%s 安装成功\n", record.Browser, record.Version)
	return nil
}

func runUninstall(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定要卸载的版本")
	}

	spec := resolveFirstArg(ctx, args)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[uninstall] 解析规格: %s@%s", spec.Browser, spec.Version)
	}

	// Resolve partial version to full installed version
	resolvedVersion, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version)
	if err != nil {
		ctx.Printf("%s@%s 未安装\n", spec.Browser, spec.Version)
		return nil
	}

	// 获取安装大小（用于 verbose 日志）
	var freedSize int64
	if ctx.Logger != nil {
		if record, getErr := ctx.Install.GetRecord(spec.Browser, resolvedVersion); getErr == nil {
			freedSize = record.Size
		}
	}

	if err := ctx.Install.Uninstall(spec.Browser, resolvedVersion); err != nil {
		return fmt.Errorf("卸载失败: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[uninstall] 卸载完成: %s@%s 释放空间 %s",
			spec.Browser, resolvedVersion, FormatSize(freedSize))
	}

	ctx.Printf("✓ %s@%s 已卸载\n", spec.Browser, resolvedVersion)
	return nil
}

func runUse(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定版本")
	}

	spec := resolveFirstArg(ctx, args)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[use] 设置默认: %s@%s", spec.Browser, spec.Version)
	}

	// Resolve alias / partial versions to full installed version
	resolvedVersion, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version)
	if err != nil {
		ctx.Printf("无法解析 %s@%s（版本未安装）\n", spec.Browser, spec.Version)
		return nil
	}

	// Set default browser first, then the default version
	if err := ctx.Cfg.Defaults.SetDefaultBrowser(spec.Browser); err != nil {
		return fmt.Errorf("设置默认浏览器失败: %w", err)
	}
	if err := ctx.Cfg.Defaults.SetDefaultVersion(resolvedVersion); err != nil {
		return fmt.Errorf("设置默认版本失败: %w", err)
	}

	ctx.Printf("当前使用: %s@%s\n", spec.Browser, resolvedVersion)
	return nil
}
