package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func runCacheClear(ctx *Context, args []string) error {
	cacheDir := ctx.Paths.DownloadCacheDir()
	if cacheDir == "" {
		ctx.Printf("下载缓存目录未配置\n")
		return nil
	}

	// Count files and total size before clearing
	var totalSize int64
	var fileCount int
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	if fileCount == 0 {
		ctx.Printf("下载缓存为空，无需清理。\n")
		return nil
	}

	ctx.Printf("即将清理下载缓存:\n")
	ctx.Printf("  文件数: %d\n", fileCount)
	ctx.Printf("  总大小: %s\n", FormatSize(totalSize))
	ctx.Printf("  目录:   %s\n", cacheDir)

	if !ctx.Confirm("确认清理？") {
		ctx.Printf("已取消\n")
		return nil
	}

	// Remove all contents but keep the directory itself
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.Printf("下载缓存目录不存在\n")
			return nil
		}
		return fmt.Errorf("读取缓存目录失败: %w", err)
	}

	var removedCount int
	var failedCount int
	for _, entry := range entries {
		path := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Debug("[cache] 删除失败: %s: %v", path, err)
			}
			failedCount++
		} else {
			removedCount++
		}
	}

	ctx.Printf("✓ 已清理 %d 个文件", removedCount)
	if failedCount > 0 {
		ctx.Printf("，%d 个失败", failedCount)
	}
	ctx.Println()

	if ctx.Logger != nil {
		ctx.Logger.Debug("[cache] 清理完成: 已删除=%d 失败=%d 释放空间=%s", removedCount, failedCount, FormatSize(totalSize))
	}
	return nil
}

func runCacheInfo(ctx *Context, args []string) error {
	cacheDir := ctx.Paths.DownloadCacheDir()
	if cacheDir == "" {
		ctx.Printf("下载缓存目录未配置\n")
		return nil
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[cache] 缓存目录: %s", cacheDir)
	}

	var totalSize int64
	var fileCount int
	var files []struct {
		name string
		size int64
	}

	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && path != cacheDir {
			fileCount++
			totalSize += info.Size()
			rel, _ := filepath.Rel(cacheDir, path)
			files = append(files, struct {
				name string
				size int64
			}{rel, info.Size()})
		}
		return nil
	})

	ctx.Printf("下载缓存状态:\n")
	ctx.Printf("  目录:   %s\n", cacheDir)
	ctx.Printf("  文件数: %d\n", fileCount)
	ctx.Printf("  总大小: %s\n", FormatSize(totalSize))

	if fileCount > 0 {
		ctx.Printf("\n缓存文件列表:\n")
		for _, f := range files {
			ctx.Printf("  %s  (%s)\n", f.name, FormatSize(f.size))
		}
	}

	return nil
}
