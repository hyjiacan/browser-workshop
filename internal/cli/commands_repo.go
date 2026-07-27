package cli

import (
	"fmt"

	"github.com/bws/bws/internal/repo"
)

func runRepoPath(ctx *Context, args []string) error {
	path := ctx.Cfg.Repo.GetRepoPath()
	if path == "" {
	ctx.Println("未配置仓库路径。")
	ctx.Println("使用 'bws repo set <路径>' 配置。")
	} else {
		ctx.Printf("仓库路径: %s\n", path)
	}
	return nil
}

func runRepoSet(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws repo set <路径>")
	}
	path := args[0]
	if err := ctx.Cfg.Repo.SetRepoPath(path); err != nil {
		return fmt.Errorf("设置仓库路径失败: %w", err)
	}
	ctx.Printf("仓库路径已设置为: %s\n", path)
	return nil
}

func runRepoScan(ctx *Context, args []string) error {
	if err := checkFeature(ctx.Repo, "仓库"); err != nil {
		return err
	}

	path := ctx.Cfg.Repo.GetRepoPath()
	if path == "" {
		return fmt.Errorf("未配置仓库路径。请先使用 'bws repo set <路径>' 配置")
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[repo] 扫描仓库: path=%s", path)
	}

	ctx.Printf("正在扫描仓库: %s ...\n", path)
	results, err := ctx.Repo.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	if len(results) == 0 {
		ctx.Println("仓库中未找到任何条目。")
		return nil
	}

	ctx.Printf("已扫描 %d 个条目，路径: %s\n\n", len(results), path)

	// Count by status
	statusCounts := make(map[repo.MatchStatus]int)
	for _, r := range results {
		statusCounts[r.Status]++
	}

	ctx.Printf("统计:\n")
	for status, count := range statusCounts {
		ctx.Printf("  %-15s %d\n", status.String()+":", count)
	}
	ctx.Println()

	// Print recognized versions
	ctx.Println("已识别的版本:")
	for _, r := range results {
		if r.Status == repo.MatchOK || r.Status == repo.MatchPartial {
			detail := ""
			if r.Status == repo.MatchPartial && r.Detail != "" {
				detail = " (" + r.Detail + ")"
			}
			ctx.Printf("  %s@%s  [%s]%s\n", r.Browser, r.Version, r.Arch, detail)
		}
	}

	return nil
}

func runRepoImport(ctx *Context, args []string) error {
	if err := checkFeature(ctx.Repo, "仓库"); err != nil {
		return err
	}

	flags, _, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	force := flags["force"] == "true"

	path := ctx.Cfg.Repo.GetRepoPath()
	if path == "" {
		return fmt.Errorf("未配置仓库路径。请先使用 'bws repo set <路径>' 配置")
	}

	ctx.Printf("正在从仓库导入: %s\n", path)
	if force {
		ctx.Println("强制模式: 现有版本将被重新安装")
	}

	lastMsg := ""
	summary, err := ctx.Repo.Import(force, func(current int, total int, message string) {
		if message != lastMsg {
			ctx.Printf("  %s\n", message)
			lastMsg = message
		}
	})
	if err != nil {
		return fmt.Errorf("导入失败: %w", err)
	}

	ctx.Printf("\n导入完成：\n")
	ctx.Printf("  扫描总数:       %d\n", summary.Total)
	ctx.Printf("  成功导入:       %d\n", summary.Success)
	ctx.Printf("  失败:           %d\n", summary.Failed)
	ctx.Printf("  跳过:           %d\n", summary.Skipped)
	if summary.SkippedAlreadyInstalled > 0 {
		ctx.Printf("    （已安装）:   %d\n", summary.SkippedAlreadyInstalled)
	}
	if summary.SkippedIncompatible > 0 {
		ctx.Printf("    （架构不兼容）: %d\n", summary.SkippedIncompatible)
	}
	return nil
}
