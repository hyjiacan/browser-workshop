package cli

import (
	"fmt"
	"os"

	"github.com/bws/bws/internal/install"
)

func runProfileList(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "browser", Short: "b", Usage: "指定浏览器", HasValue: true, Default: ""},
	})
	if err != nil {
		return err
	}

	browser := flags["browser"]
	if browser == "" {
		if len(positional) > 0 {
			browser = positional[0]
		} else {
			browser = ctx.Cfg.Defaults.DefaultBrowser()
		}
	}

	profiles, err := ctx.Profile.ListProfiles(browser)
	if err != nil {
		return fmt.Errorf("获取 profile 列表失败: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[profile] 列出 %s 的 profile: %d 个", browser, len(profiles))
	}
	ctx.Printf("浏览器 %s 的 profile 列表:\n\n", browser)

	if len(profiles) == 0 {
		ctx.Println("  暂无 profile。")
		ctx.Println()
		ctx.Printf("  使用 'bws r %s --profile <名称>' 创建命名 profile。\n", browser)
		return nil
	}

	// 分类显示
	var namedProfiles []install.ProfileInfo
	var versionProfiles []install.ProfileInfo
	for _, p := range profiles {
		if p.Type == "named" {
			namedProfiles = append(namedProfiles, p)
		} else {
			versionProfiles = append(versionProfiles, p)
		}
	}

	if len(namedProfiles) > 0 {
		ctx.Println("  命名 profile:")
		for _, p := range namedProfiles {
			ctx.Printf("    %-20s %s\n", p.Name, p.Path)
		}
		ctx.Println()
	}

	if len(versionProfiles) > 0 {
		ctx.Println("  版本默认 profile:")
		for _, p := range versionProfiles {
			ctx.Printf("    %-20s %s\n", p.Version, p.Path)
		}
		ctx.Println()
	}

	ctx.Printf("  共 %d 个 profile\n", len(profiles))
	return nil
}

func runProfilePath(ctx *Context, args []string) error {
	browser := ctx.Cfg.Defaults.DefaultBrowser()
	profileName := ""

	if len(args) > 0 {
		spec := resolveSpec(ctx, args[0], ctx.Cfg.Defaults.DefaultBrowser())
		browser = spec.Browser
	}
	if len(args) > 1 {
		profileName = args[1]
	}

	profilePath := ctx.Profile.ProfileDir(browser, "", profileName)
	ctx.Printf("profile 路径: %s\n", profilePath)
	return nil
}

func runProfileReset(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "跳过确认直接重置", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	if len(positional) == 0 {
		return fmt.Errorf("请指定浏览器版本，例如 'bws pf reset chrome@120'")
	}

	spec := resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())

	profileName := ""
	if len(positional) > 1 {
		profileName = positional[1]
	}

	// 解析版本别名（仅对默认 profile 有意义）。优先用 ResolveInstalledVersion 统一解析。
	if spec.IsAlias && spec.Version != "system" && profileName == "" {
		if v, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version); err == nil {
			spec.Version = v
		}
		// 如果无法解析，继续使用别名作为版本名（profile 目录可能已存在）
	}

	force := flags["force"] == "true"
	profileDir := ctx.Profile.ProfileDir(spec.Browser, spec.Version, profileName)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[profile] 重置: browser=%s version=%s profile=%s dir=%s force=%v",
			spec.Browser, spec.Version, profileName, profileDir, force)
	}

	// 显示将要重置的信息
	if profileName != "" {
		ctx.Printf("将重置 profile:\n")
		ctx.Printf("  浏览器:  %s\n", spec.Browser)
		ctx.Printf("  Profile: %s\n", profileName)
		ctx.Printf("  路径:    %s\n", profileDir)
	} else {
		ctx.Printf("将重置默认 profile:\n")
		ctx.Printf("  浏览器:  %s\n", spec.Browser)
		ctx.Printf("  版本:    %s\n", spec.Version)
		ctx.Printf("  路径:    %s\n", profileDir)
	}
	ctx.Println()

	// 确认提示
	if !force {
		if !ctx.Confirm("确定要重置此 profile 吗？此操作不可恢复。") {
			ctx.Println("已取消。")
			return nil
		}
	}

	ctx.Println("正在重置 profile...")
	if err := ctx.Profile.ResetProfile(spec.Browser, spec.Version, profileName); err != nil {
		return fmt.Errorf("重置 profile 失败: %w", err)
	}

	ctx.Printf("✓ Profile 已重置: %s\n", profileDir)
	return nil
}

func runProfileClean(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "直接执行清理", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	browser := ctx.Cfg.Defaults.DefaultBrowser()
	if len(positional) > 0 {
		browser = positional[0]
	}

	orphaned, err := ctx.Profile.CleanOrphanedProfiles(browser)
	if err != nil {
		return fmt.Errorf("获取孤立 profile 失败: %w", err)
	}

	if len(orphaned) == 0 {
		ctx.Printf("%s 没有发现孤立 profile。\n", browser)
		return nil
	}

	ctx.Printf("%s 有 %d 个孤立 profile:\n", browser, len(orphaned))
	for _, p := range orphaned {
		ctx.Printf("  %s\n", p)
	}

	force := flags["force"] == "true"
	if !force {
		if !ctx.Confirm("确认清理这些 profile？") {
			ctx.Println("已取消。")
			return nil
		}
	}

	// Delete orphaned profile directories
	deleted := 0
	for _, p := range orphaned {
		if err := os.RemoveAll(p); err != nil {
			ctx.Printf("警告: 删除 %s 失败: %v\n", p, err)
			continue
		}
		deleted++
	}
	ctx.Printf("清理完成，已删除 %d/%d 个孤立 profile。\n", deleted, len(orphaned))
	return nil
}
