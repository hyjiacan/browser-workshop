package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runDownload(ctx *Context, args []string) error {
	if ctx.Source == nil || ctx.Download == nil {
		return fmt.Errorf("当前构建不支持下载功能")
	}

	if len(args) == 0 {
		return fmt.Errorf("请指定要下载的版本，例如 'bws dl chrome@120'")
	}

	spec := resolveFirstArg(ctx, args)
	channel := "stable"
	outputDir := ""

	if len(args) > 1 {
		flagVals, _, err := ParseFlags(args[1:], []*Flag{
			{Name: "output", Short: "o", Usage: "输出目录", HasValue: true, Default: ""},
			{Name: "channel", Short: "c", Usage: "发布渠道", HasValue: true, Default: "stable"},
		})
		if err != nil {
			return err
		}
		channel = flagVals["channel"]
		outputDir = flagVals["output"]
	}

	// Check disk space
	checkPath := outputDir
	if checkPath == "" {
		checkPath = "."
		if ctx.Cfg != nil {
			checkPath = ctx.Cfg.Data.GetDataDir()
		}
	}
	if err := checkDiskSpace(ctx, checkPath); err != nil {
		return err
	}

	// Resolve the version
	ctx.Printf("正在解析 %s@%s...\n", spec.Browser, spec.Version)
	versionInfo, err := ctx.Source.ResolveVersion(spec.Browser, spec.Version)
	if err != nil {
		// 仅对别名（latest/beta/esr 等）回退到渠道列表
		if spec.IsAlias {
			versions, listErr := ctx.Source.ListVersions(spec.Browser, channel)
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
		ctx.Logger.Debug("[download] 版本已解析: %s@%s channel=%s url=%s",
			spec.Browser, versionInfo.Version, versionInfo.Channel, versionInfo.DownloadURL)
	}

	// Determine output path
	if outputDir == "" {
		// Use current directory
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取工作目录失败: %w", err)
		}
		outputDir = wd
	}

	// Create output directory if needed
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// Determine filename from URL
	fileName := getDownloadFilename(spec.Browser, versionInfo.Version, versionInfo.DownloadURL, string(versionInfo.Platform))
	destPath := filepath.Join(outputDir, fileName)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[download] 目标文件: %s", destPath)
	}

	// 检查本地缓存（文件已存在且非空则视为命中缓存）
	if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
		if ctx.Logger != nil {
			ctx.Logger.Debug("[download] 命中本地缓存，跳过下载: %s (大小: %s)", destPath, FormatSize(info.Size()))
		}
		ctx.Printf("✓ 下载完成: %s\n", destPath)
		return nil
	}

	if ctx.Logger != nil {
		if versionInfo.Size > 0 {
			ctx.Logger.Debug("[download] 开始下载: %s (预期大小: %s)", versionInfo.DownloadURL, FormatSize(versionInfo.Size))
		} else {
			ctx.Logger.Debug("[download] 开始下载: %s (文件大小未知)", versionInfo.DownloadURL)
		}
	}

	ctx.Printf("正在下载 %s@%s 到 %s...\n", spec.Browser, versionInfo.Version, outputDir)

	startTime := time.Now()
	_, err = ctx.Download.Download(versionInfo.DownloadURL, destPath, func(downloaded, total int64, percent float64) {
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

	if ctx.Logger != nil {
		elapsed := time.Since(startTime)
		var speedStr string
		if elapsed.Seconds() > 0 {
			if info, statErr := os.Stat(destPath); statErr == nil {
				speed := float64(info.Size()) / elapsed.Seconds()
				speedStr = FormatSpeed(speed)
				ctx.Logger.Debug("[download] 下载完成: 大小=%s 耗时=%s 速度=%s",
					FormatSize(info.Size()), formatDuration(elapsed), speedStr)
			} else {
				ctx.Logger.Debug("[download] 下载完成: 耗时=%s", formatDuration(elapsed))
			}
		} else {
			ctx.Logger.Debug("[download] 下载完成: 耗时极短")
		}
	}

	ctx.Printf("✓ 下载完成: %s\n", destPath)
	return nil
}
