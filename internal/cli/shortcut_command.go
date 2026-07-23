package cli

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NewShortcutCommand creates the shortcut command.
func NewShortcutCommand() *Command {
	return &Command{
		Name:        "shortcut",
		Aliases:     []string{"sc"},
		Description: "管理桌面快捷方式",
		Usage:       "bws shortcut <子命令> [浏览器[@版本]] [选项]",
		Examples: []string{
			"shortcut create chrome@120",
			"shortcut create firefox --profile dev",
			"shortcut create --all",
			"shortcut remove chrome@120",
			"shortcut remove --all",
			"shortcut list",
		},
		SubCommands: []*Command{
			{
				Name:        "create",
				Aliases:     []string{"c", "add"},
				Description: "为已安装的浏览器创建桌面快捷方式",
				Usage:       "bws shortcut create <浏览器[@版本]> [选项]",
				Examples: []string{
					"shortcut create chrome@120",
					"shortcut create firefox@latest --profile dev",
					"shortcut create --all",
				},
				Flags: []*Flag{
					{Name: "profile", Short: "p", Usage: "指定配置名称", HasValue: true},
					{Name: "native", Short: "n", Usage: "原生模式（不使用 profile）"},
					{Name: "all", Short: "a", Usage: "为所有已安装版本创建"},
					{Name: "name", Usage: "自定义快捷方式名称", HasValue: true},
				},
				Run: runShortcutCreate,
			},
			{
				Name:        "remove",
				Aliases:     []string{"rm", "del"},
				Description: "移除桌面快捷方式",
				Usage:       "bws shortcut remove <浏览器[@版本]> [选项]",
				Examples: []string{
					"shortcut remove chrome@120",
					"shortcut remove --all",
				},
				Flags: []*Flag{
					{Name: "all", Short: "a", Usage: "移除所有快捷方式"},
				},
				Run: runShortcutRemove,
			},
			{
				Name:        "list",
				Aliases:     []string{"ls"},
				Description: "列出已创建的桌面快捷方式",
				Usage:       "bws shortcut list",
				Run:         runShortcutList,
			},
		},
	}
}

func runShortcutCreate(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "profile", Short: "p", Usage: "指定配置名称", HasValue: true},
		{Name: "native", Short: "n", Usage: "原生模式"},
		{Name: "all", Short: "a", Usage: "为所有已安装版本创建"},
		{Name: "name", Usage: "自定义快捷方式名称", HasValue: true},
	})
	if err != nil {
		return err
	}

	all := flags["all"] == "true"
	if !all && len(positional) == 0 {
		return fmt.Errorf("请指定浏览器版本，或使用 --all 为所有已安装版本创建")
	}

	profileName := flags["profile"]
	native := flags["native"] == "true"
	customName := flags["name"]

	// Resolve version if needed
	resolveVersion := func(browser, ver string) (string, error) {
		if isVersionAlias(ver) {
			resolved, err := ctx.Source.ResolveVersion(browser, ver)
			if err != nil {
				return "", fmt.Errorf("解析版本失败: %w", err)
			}
			return resolved.Version, nil
		}
		return ver, nil
	}

	if all {
		// Create shortcuts for all installed versions
		installed, err := ctx.Install.ListInstalled()
		if err != nil {
			return fmt.Errorf("获取已安装列表失败: %w", err)
		}
		if len(installed) == 0 {
			fmt.Println("没有已安装的浏览器")
			return nil
		}

		var created int
		for _, rec := range installed {
			ver, err := resolveVersion(rec.Browser, rec.Version)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "  跳过 %s@%s: %v\n", rec.Browser, rec.Version, err)
				continue
			}

			name := customName
			if name == "" {
				name = fmt.Sprintf("%s %s", rec.Browser, ver)
			}

			if err := createOneShortcut(ctx, rec.Browser, ver, profileName, native, name); err != nil {
				fmt.Fprintf(ctx.Stderr, "  创建 %s 快捷方式失败: %v\n", name, err)
				continue
			}
			fmt.Printf("  已创建: %s\n", name)
			created++
		}
		fmt.Printf("\n共创建 %d 个快捷方式\n", created)
		return nil
	}

	// Single shortcut
	defaultBrowser := ""
	if ctx.Config != nil {
		defaultBrowser = ctx.Config.DefaultBrowser()
	}
	spec := parseBrowserVersion(positional[0], defaultBrowser)

	ver, err := resolveVersion(spec.Browser, spec.Version)
	if err != nil {
		return err
	}

	name := customName
	if name == "" {
		name = fmt.Sprintf("%s %s", spec.Browser, ver)
	}

	return createOneShortcut(ctx, spec.Browser, ver, profileName, native, name)
}

func createOneShortcut(ctx *Context, browser, version, profileName string, native bool, name string) error {
	// Get the executable path and args via launch preview
	opts := LaunchOptions{
		Browser:     browser,
		Version:     version,
		ProfileName: profileName,
		NativeMode:  native,
		Proxy:       ctx.Config.GetProxy(),
	}

	exePath, args, err := ctx.Launch.PreviewCommand(opts)
	if err != nil {
		return fmt.Errorf("获取启动命令失败: %w", err)
	}

	// Resolve to absolute path
	if !filepath.IsAbs(exePath) {
		abs, err := filepath.Abs(exePath)
		if err == nil {
			exePath = abs
		}
	}

	scOpts := ShortcutOptions{
		Name:       name,
		Target:     exePath,
		Args:       args,
		WorkingDir: filepath.Dir(exePath),
	}

	if err := ctx.Shortcut.Create(scOpts); err != nil {
		return err
	}

	fmt.Printf("已创建快捷方式: %s -> %s\n", name, exePath)
	if len(args) > 0 {
		fmt.Printf("  参数: %s\n", strings.Join(args, " "))
	}
	return nil
}

func runShortcutRemove(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "all", Short: "a", Usage: "移除所有快捷方式"},
	})
	if err != nil {
		return err
	}

	all := flags["all"] == "true"
	if !all && len(positional) == 0 {
		return fmt.Errorf("请指定快捷方式名称，或使用 --all 移除所有")
	}

	if all {
		names, err := ctx.Shortcut.List("")
		if err != nil {
			return fmt.Errorf("获取快捷方式列表失败: %w", err)
		}
		if len(names) == 0 {
			fmt.Println("没有可移除的快捷方式")
			return nil
		}

		if !ctx.Confirm(fmt.Sprintf("确定要移除所有 %d 个快捷方式吗", len(names))) {
			fmt.Println("已取消")
			return nil
		}

		var removed int
		for _, name := range names {
			if err := ctx.Shortcut.Remove(name, ""); err != nil {
				fmt.Fprintf(ctx.Stderr, "  移除 %s 失败: %v\n", name, err)
				continue
			}
			fmt.Printf("  已移除: %s\n", name)
			removed++
		}
		fmt.Printf("\n共移除 %d 个快捷方式\n", removed)
		return nil
	}

	name := positional[0]
	if err := ctx.Shortcut.Remove(name, ""); err != nil {
		return err
	}
	fmt.Printf("已移除快捷方式: %s\n", name)
	return nil
}

func runShortcutList(ctx *Context, args []string) error {
	_, _, err := ParseFlags(args, nil)
	if err != nil {
		return err
	}

	names, err := ctx.Shortcut.List("")
	if err != nil {
		return fmt.Errorf("获取快捷方式列表失败: %w", err)
	}

	if len(names) == 0 {
		fmt.Println("没有已创建的快捷方式")
		return nil
	}

	fmt.Printf("已创建的快捷方式 (%d 个):\n", len(names))
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
	return nil
}
