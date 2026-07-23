package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bws/bws/internal/disk"
	"github.com/bws/bws/internal/help"
)

// RegisterCommands registers all built-in CLI commands.
func RegisterCommands(app *App) {
	app.AddCommand(NewLsCommand())
	app.AddCommand(NewInfoCommand())
	app.AddCommand(NewRunCommand())
	app.AddCommand(NewInstallCommand())
	app.AddCommand(NewImportCommand())
	app.AddCommand(NewUninstallCommand())
	app.AddCommand(NewUseCommand())
	// list-remote 已合并到 ls --remote，不再单独注册
	app.AddCommand(NewDownloadCommand())
	app.AddCommand(NewAliasCommand())
	app.AddCommand(NewRepoCommand())
	app.AddCommand(NewConfigCommand())
	app.AddCommand(NewCacheCommand())
	app.AddCommand(NewProfileCommand())
	app.AddCommand(NewDoctorCommand())
	app.AddCommand(NewShortcutCommand())
	app.AddCommand(NewServeCommand())
	app.AddCommand(NewHelpCommand())
}

// --- ls command ---

func NewLsCommand() *Command {
	return &Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "列出已安装或远程可用的浏览器版本",
		Usage:       "bws list [浏览器[@版本前缀]] [选项]",
		Examples: []string{
			"ls",
			"ls chrome",
			"ls chrome@79",
			"ls chrome@79.0",
			"ls --all",
			"ls --no-system",
			"ls --remote chrome",
			"ls -R chrome@79",
			"ls -R chrome --channel beta",
		},
		Flags: []*Flag{
			{Name: "all", Short: "a", Usage: "显示所有浏览器", HasValue: false, Default: "false"},
			{Name: "json", Usage: "以 JSON 格式输出", HasValue: false, Default: "false"},
			{Name: "no-system", Usage: "隐藏系统安装的浏览器", HasValue: false, Default: "false"},
			{Name: "remote", Short: "R", Usage: "列出远程源中的可用版本", HasValue: false, Default: "false"},
			{Name: "channel", Short: "c", Usage: "远程模式：按渠道过滤（stable/beta/dev/canary）", HasValue: true, Default: "stable"},
			{Name: "limit", Short: "n", Usage: "远程模式：限制结果数量", HasValue: true, Default: "20"},
		},
		Run: runLs,
	}
}

func runLs(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "all", Short: "a", Usage: "显示所有", HasValue: false, Default: "false"},
		{Name: "json", Usage: "JSON 输出", HasValue: false, Default: "false"},
		{Name: "system", Short: "s", Usage: "包含系统安装的浏览器", HasValue: false, Default: "true"},
		{Name: "no-system", Usage: "隐藏系统安装的浏览器", HasValue: false, Default: "false"},
		{Name: "remote", Short: "R", Usage: "列出远程源中的可用版本", HasValue: false, Default: "false"},
		{Name: "channel", Short: "c", Usage: "远程模式：按渠道过滤", HasValue: true, Default: "stable"},
		{Name: "limit", Short: "n", Usage: "远程模式：限制结果数量", HasValue: true, Default: "20"},
	})
	if err != nil {
		return err
	}

	// 远程模式
	if flags["remote"] == "true" {
		// 过滤掉 --remote / -R 参数后传递给远程列表函数
		var remoteArgs []string
		for _, arg := range args {
			if arg == "--remote" || arg == "-R" {
				continue
			}
			remoteArgs = append(remoteArgs, arg)
		}
		return runRemoteQuery(ctx, remoteArgs)
	}

	// 解析参数，支持 chrome@79 这样的语法
	spec := browserVersionSpec{Browser: "", Version: ""}
	if len(positional) > 0 {
		spec = parseBrowserVersion(positional[0], "")
		spec = resolveBrowserSpec(ctx, spec)
	}

	includeSystem := flags["no-system"] != "true"

	var versions []InstalledVersion

	if includeSystem {
		if spec.Browser != "" {
			versions, err = ctx.Install.ListWithSystemByBrowser(spec.Browser)
		} else {
			versions, err = ctx.Install.ListWithSystem()
		}
	} else {
		if spec.Browser != "" {
			versions, err = ctx.Install.ListInstalledByBrowser(spec.Browser)
		} else {
			versions, err = ctx.Install.ListInstalled()
		}
	}
	if err != nil {
		return fmt.Errorf("获取版本列表失败: %w", err)
	}

	// 按版本前缀筛选
	if spec.Version != "" && !spec.IsAlias {
		var filtered []InstalledVersion
		for _, v := range versions {
			if matchesVersionPrefix(v.Version, spec.Version) {
				filtered = append(filtered, v)
			}
		}
		versions = filtered
	}

	if len(versions) == 0 {
		ctx.Println("暂无匹配的版本。")
		if spec.Version != "" {
			ctx.Printf("筛选条件: %s@%s\n", spec.Browser, spec.Version)
		}
		ctx.Println("使用 'bws i <浏览器@版本>' 安装一个版本。")
		return nil
	}

	// Group by browser
	byBrowser := make(map[string][]InstalledVersion)
	for _, v := range versions {
		byBrowser[v.Browser] = append(byBrowser[v.Browser], v)
	}

	// Sort by browser name
	browserNames := make([]string, 0, len(byBrowser))
	for name := range byBrowser {
		browserNames = append(browserNames, name)
	}
	sort.Strings(browserNames)

	for _, bName := range browserNames {
		vs := byBrowser[bName]
		// Sort versions descending
		sort.Slice(vs, func(i, j int) bool {
			return compareVersions(vs[i].Version, vs[j].Version) > 0
		})

		desc := getBrowserDisplayName(ctx, bName)
		sysCount := 0
		for _, v := range vs {
			if v.IsSystem {
				sysCount++
			}
		}
		localCount := len(vs) - sysCount
		if sysCount > 0 && localCount > 0 {
			ctx.Printf("%s（已安装 %d 个，系统 %d 个）\n", desc, localCount, sysCount)
		} else if sysCount > 0 {
			ctx.Printf("%s（系统 %d 个）\n", desc, sysCount)
		} else {
			ctx.Printf("%s（已安装 %d 个）\n", desc, localCount)
		}

		for _, v := range vs {
			channel := ""
			if v.Channel != "" {
				channel = fmt.Sprintf(" [%s]", v.Channel)
			}
			sysTag := ""
			if v.IsSystem {
				sysTag = " [系统]"
			}
			ctx.Printf("  %s%s%s\n", v.Version, channel, sysTag)
		}
		ctx.Println()
	}

	return nil
}

func runRemoteQuery(ctx *Context, args []string) error {
	if ctx.Source == nil {
		return fmt.Errorf("当前构建不支持远程源")
	}

	flagVals, positional, err := ParseFlags(args, []*Flag{
		{Name: "channel", Short: "c", Usage: "按渠道过滤", HasValue: true, Default: "stable"},
		{Name: "limit", Short: "n", Usage: "限制结果数量", HasValue: true, Default: "20"},
		{Name: "all", Short: "a", Usage: "显示所有版本（所有渠道）", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	// 解析参数，支持 chrome@79 这样的语法
	spec := browserVersionSpec{Browser: ctx.Config.DefaultBrowser(), Version: ""}
	if len(positional) > 0 {
		spec = parseBrowserVersion(positional[0], ctx.Config.DefaultBrowser())
		spec = resolveBrowserSpec(ctx, spec)
	}

	channel := flagVals["channel"]
	limit := 20
	if flagVals["limit"] != "" {
		fmt.Sscanf(flagVals["limit"], "%d", &limit)
	}
	showAll := flagVals["all"] == "true"

	channels := []string{channel}
	if showAll {
		channels = []string{"stable", "beta", "esr", "dev", "canary"}
	}

	// 检查浏览器是否支持
	if !ctx.Browsers.Has(spec.Browser) {
		return fmt.Errorf("不支持的浏览器: %s", spec.Browser)
	}

	// 确定当前浏览器相关的源
	var activeSources []string
	var hasServe bool
	if ctx.Config != nil {
		if ctx.Config.IsServeSourceEnabled() {
			remoteURL := ctx.Config.GetRemoteSource()
			if remoteURL != "" {
				activeSources = append(activeSources, "远程 HTTP 源")
				hasServe = true
			}
		}
		if ctx.Config.IsOmahaSourceEnabled() {
			// Omaha 源支持 chrome 和 chromium
			if spec.Browser == "chrome" || spec.Browser == "chromium" {
				activeSources = append(activeSources, "Chrome Omaha 协议", "Chrome Omaha Proxy")
			}
		}
		if ctx.Config.IsFirefoxFTPEnabled() {
			if spec.Browser == "firefox" {
				activeSources = append(activeSources, "Mozilla Product Details")
			}
		}
	} else {
		// 无配置时默认显示所有
		activeSources = append(activeSources, "远程 HTTP 源", "Chrome Omaha 协议", "Chrome Omaha Proxy")
		if spec.Browser == "firefox" {
			activeSources = append(activeSources, "Mozilla Product Details")
		}
	}

	if len(activeSources) == 0 {
		ctx.Printf("没有为 %s 配置可用的远程源。\n", spec.Browser)
		if ctx.Config != nil {
			if (spec.Browser == "chrome" || spec.Browser == "chromium") && !ctx.Config.IsOmahaSourceEnabled() {
				ctx.Println("提示: Omaha 源已禁用，使用 'bws cfg set source-omaha true' 启用。")
			}
			if spec.Browser == "firefox" && !ctx.Config.IsFirefoxFTPEnabled() {
				ctx.Println("提示: Firefox 源已禁用，使用 'bws cfg set source-firefox-ftp true' 启用。")
			}
			if !ctx.Config.IsServeSourceEnabled() {
				ctx.Println("提示: Serve 源已禁用，使用 'bws cfg set source-serve true' 启用。")
			}
		}
		return nil
	}

	// 打印交互过程
	ctx.Printf("正在从 %s 请求 %s 版本列表...\n", strings.Join(activeSources, "、"), spec.Browser)
	if hasServe && ctx.Config != nil {
		remoteURL := ctx.Config.GetRemoteSource()
		if remoteURL != "" {
			ctx.Printf("  远程源地址: %s\n", remoteURL)
		}
	}
	ctx.Printf("  查询渠道: %s\n", strings.Join(channels, ", "))
	if spec.Version != "" && !spec.IsAlias {
		ctx.Printf("  版本筛选: %s\n", spec.Version)
	}
	ctx.Println()

	// 获取本地已安装版本，用于标记
	installedMap := make(map[string]bool)
	if ctx.Install != nil {
		installed, _ := ctx.Install.ListInstalledByBrowser(spec.Browser)
		for _, v := range installed {
			installedMap[v.Version] = true
		}
	}

	rows := [][]string{}
	totalShown := 0
	installedCount := 0
	totalResults := 0
	channelResults := make(map[string]int) // channel -> count

	for _, ch := range channels {
		ctx.Printf("  正在查询 %s 渠道...", ch)
		versions, err := ctx.Source.ListVersions(spec.Browser, ch)
		if err != nil {
			ctx.Printf(" 失败 (%v)\n", err)
			continue
		}

		if len(versions) == 0 {
			ctx.Println(" 无结果")
			continue
		}

		// 按版本前缀筛选
		filtered := versions
		if spec.Version != "" && !spec.IsAlias {
			filtered = nil
			for _, v := range versions {
				if matchesVersionPrefix(v.Version, spec.Version) {
					filtered = append(filtered, v)
				}
			}
		}

		if len(filtered) == 0 {
			ctx.Println(" 无匹配版本")
			continue
		}

		channelResults[ch] = len(filtered)
		ctx.Printf(" 找到 %d 个版本\n", len(filtered))
		totalResults += len(filtered)

		// 限制每个渠道显示的版本数量
		showCount := len(filtered)
		if !showAll {
			showCount = min(limit, len(filtered))
		}

		for i := 0; i < showCount; i++ {
			v := filtered[i]
			status := ""
			if installedMap[v.Version] {
				status = "已安装"
				installedCount++
			}
			rows = append(rows, []string{v.Version, ch, v.Platform, v.Arch, status})
			totalShown++
		}
	}

	// 分隔线
	ctx.Println()
	ctx.Println("----------------------------------------")
	ctx.Println()

	// 结果标题
	title := ""
	if spec.Version != "" && !spec.IsAlias {
		title = fmt.Sprintf("%s 中匹配 %s 的可用版本", spec.Browser, spec.Version)
	} else {
		title = fmt.Sprintf("%s 的可用版本", spec.Browser)
	}

	if totalResults > 0 {
		title += fmt.Sprintf("（共 %d 个版本", totalResults)
		if totalShown < totalResults {
			title += fmt.Sprintf("，显示前 %d 个）", totalShown)
		} else {
			title += "）"
		}
	}
	ctx.Println(title + "：")
	ctx.Println()

	if len(rows) == 0 {
		ctx.Println("  未找到匹配的版本。")
		return nil
	}

	// 大量结果时按主版本分组展示，提高可读性
	if totalResults > 30 {
		groups := groupByMajorVersion(rows)
		if showAll {
			// --all 显示所有组
			printGroupedVersions(ctx, groups, len(groups))
		} else {
			// 默认显示前 limit 组
			printGroupedVersions(ctx, groups, limit)
			ctx.Printf("\n  共 %d 个版本，已折叠为 %d 个主版本组。\n", totalResults, len(groups))
			ctx.Println("  使用 --all 或 -a 查看所有版本详情。")
		}
	} else {
		// 少量结果直接显示完整表格
		PrintTable(ctx.Stdout, []string{"版本", "渠道", "平台", "架构", "状态"}, rows)
	}

	if installedCount > 0 {
		ctx.Printf("\n  已安装 %d 个版本。\n", installedCount)
	}

	ctx.Printf("\n  安装命令: bws i %s@<版本>\n", spec.Browser)
	return nil
}

// --- run command ---

func NewRunCommand() *Command {
	return &Command{
		Name:        "run",
		Aliases:     []string{"r", "open"},
		Description: "运行指定版本的浏览器",
		Usage:       "bws r <浏览器@版本> [选项] [-- <浏览器参数>]",
		Examples: []string{
			"r chrome@120",
			"r firefox -- https://example.com",
			"r chrome@latest --headless",
			"r chrome --native",
			"r chrome@system",
			"r chrome --proxy socks5://127.0.0.1:1080",
			"r chrome --no-proxy",
			"r chrome --fingerprint random",
			"r chrome --fingerprint standard",
		},
		Flags: []*Flag{
			{Name: "headless", Short: "H", Usage: "无头模式运行", HasValue: false, Default: "false"},
			{Name: "incognito", Short: "i", Usage: "隐身/隐私模式运行", HasValue: false, Default: "false"},
			{Name: "new-window", Short: "w", Usage: "在新窗口中打开", HasValue: false, Default: "false"},
			{Name: "profile", Short: "p", Usage: "使用指定的配置文件名称", HasValue: true, Default: ""},
			{Name: "native", Short: "n", Usage: "原生模式启动（无 bws 隔离）", HasValue: false, Default: "false"},
			{Name: "detached", Short: "d", Usage: "后台运行（不等待进程结束）", HasValue: false, Default: "false"},
			{Name: "dry-run", Short: "", Usage: "仅打印命令，不实际运行", HasValue: false, Default: "false"},
			{Name: "proxy", Short: "", Usage: "代理地址（如 socks5://127.0.0.1:1080），留空使用全局配置", HasValue: true, Default: ""},
			{Name: "no-proxy", Short: "", Usage: "禁用代理（覆盖全局配置）", HasValue: false, Default: "false"},
			{Name: "fingerprint", Short: "fp", Usage: "指纹隔离预设（standard/random/none），或 JSON 配置/@文件路径", HasValue: true, Default: ""},
		},
		Run: runRun,
	}
}

func runRun(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "headless", Short: "H", Usage: "无头模式", HasValue: false, Default: "false"},
		{Name: "incognito", Short: "i", Usage: "隐身模式", HasValue: false, Default: "false"},
		{Name: "new-window", Short: "w", Usage: "新窗口", HasValue: false, Default: "false"},
		{Name: "profile", Short: "p", Usage: "配置文件名称", HasValue: true, Default: ""},
		{Name: "native", Short: "n", Usage: "原生模式（无隔离）", HasValue: false, Default: "false"},
		{Name: "detached", Short: "d", Usage: "后台运行", HasValue: false, Default: "false"},
		{Name: "dry-run", Short: "", Usage: "试运行", HasValue: false, Default: "false"},
		{Name: "proxy", Short: "", Usage: "代理地址（如 socks5://127.0.0.1:1080），留空使用全局配置", HasValue: true, Default: ""},
		{Name: "no-proxy", Short: "", Usage: "禁用代理（覆盖全局配置）", HasValue: false, Default: "false"},
		{Name: "fingerprint", Short: "fp", Usage: "指纹隔离预设（standard/random/none），或 JSON 配置/@文件路径", HasValue: true, Default: ""},
	}

	// Split args at -- to separate bm args from browser args
	var bmArgs, browserArgs []string
	foundDashDash := false
	for _, arg := range args {
		if arg == "--" {
			foundDashDash = true
			continue
		}
		if foundDashDash {
			browserArgs = append(browserArgs, arg)
		} else {
			bmArgs = append(bmArgs, arg)
		}
	}

	flagVals, positional, err := ParseFlags(bmArgs, flags)
	if err != nil {
		return err
	}

	if len(positional) == 0 {
		return fmt.Errorf("请指定浏览器版本，例如 'bws r chrome@120'")
	}

	// Parse browser@version
	spec := parseBrowserVersion(positional[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)

	// Handle "system" alias → resolve to system browser version
	if spec.Version == "system" {
		sysVersions, err := ctx.Install.ListWithSystemByBrowser(spec.Browser)
		if err != nil {
			return err
		}
		found := false
		for _, v := range sysVersions {
			if v.IsSystem && v.Channel == "stable" {
				spec.Version = v.Version
				found = true
				break
			}
		}
		if !found {
			// Fallback to any system version
			for _, v := range sysVersions {
				if v.IsSystem {
					spec.Version = v.Version
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("未找到系统安装的 %s", spec.Browser)
		}
	}

	// URLs from remaining positional args (before --)
	urls := positional[1:]
	// Also add browser args as extra args
	extraArgs := browserArgs

	// Resolve proxy: --no-proxy takes precedence, then --proxy, then config
	proxyURL := ""
	if flagVals["no-proxy"] != "true" {
		if p := flagVals["proxy"]; p != "" {
			proxyURL = p
		} else if ctx.Config != nil {
			proxyURL = ctx.Config.GetProxy()
		}
	}

	opts := LaunchOptions{
		Browser:     spec.Browser,
		Version:     spec.Version,
		URLs:        urls,
		Headless:    flagVals["headless"] == "true",
		Incognito:   flagVals["incognito"] == "true",
		NewWindow:   flagVals["new-window"] == "true",
		ProfileName: flagVals["profile"],
		NativeMode:  flagVals["native"] == "true",
		ExtraArgs:   extraArgs,
		Detached:    flagVals["detached"] == "true",
		DryRun:      flagVals["dry-run"] == "true",
		Proxy:       proxyURL,
		Fingerprint: flagVals["fingerprint"],
	}

	if opts.DryRun {
		exe, args, err := ctx.Launch.PreviewCommand(opts)
		if err != nil {
			return err
		}
		ctx.Printf("将要执行: %s %s\n", exe, strings.Join(args, " "))
		return nil
	}

	return ctx.Launch.Run(opts)
}

// --- install command ---

func NewInstallCommand() *Command {
	return &Command{
		Name:        "install",
		Aliases:     []string{"i"},
		Description: "安装指定版本的浏览器",
		Usage:       "bws i <浏览器@版本> [选项]",
		Examples: []string{
			"i chrome@120",
			"i firefox@latest",
			"i -d /path/to/browsers",
			"i -f /path/to/installer.exe chrome@120",
			"i -d /path/to/browser-dir chrome@120",
		},
		Flags: []*Flag{
			{Name: "from-dir", Short: "d", Usage: "从本地目录安装", HasValue: true, Default: ""},
			{Name: "from-file", Short: "", Usage: "从本地压缩包安装", HasValue: true, Default: ""},
			{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
			{Name: "channel", Short: "c", Usage: "发布渠道（stable, beta, dev, canary）", HasValue: true, Default: "stable"},
		},
		Run: runInstall,
	}
}

func runInstall(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "from-dir", Short: "d", Usage: "从本地目录安装（未指定版本时自动检测）", HasValue: true, Default: ""},
		{Name: "from-file", Short: "", Usage: "从本地压缩包安装（未指定版本时自动检测）", HasValue: true, Default: ""},
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
		{Name: "channel", Short: "c", Usage: "发布渠道", HasValue: true, Default: "stable"},
	}

	flagVals, positional, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}

	fromDir := flagVals["from-dir"]
	fromFile := flagVals["from-file"]
	force := flagVals["force"] == "true"
	channel := flagVals["channel"]

	if len(positional) == 0 && fromDir == "" && fromFile == "" {
		return fmt.Errorf("请指定要安装的版本，例如 'bws i chrome@120'")
	}

	spec := browserVersionSpec{Browser: ctx.Config.DefaultBrowser(), Version: "latest", IsAlias: true}
	if len(positional) > 0 {
		spec = parseBrowserVersion(positional[0], ctx.Config.DefaultBrowser())
		spec = resolveBrowserSpec(ctx, spec)
	}

	// Check disk space before any install operation
	dataDir := "."
	if ctx.Config != nil {
		dataDir = ctx.Config.GetDataDir()
	}
	if err := checkDiskSpace(ctx, dataDir); err != nil {
		return err
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
		record, err := ctx.Install.InstallFromDir(spec.Browser, spec.Version, fromDir)
		if err != nil {
			return fmt.Errorf("安装失败: %w", err)
		}
		ctx.Printf("✓ %s@%s 安装成功\n", record.Browser, record.Version)
		return nil
	}

	// 本地文件安装
	if fromFile != "" {
		ctx.Printf("正在从 %s 安装 %s@%s...\n", fromFile, spec.Browser, spec.Version)
		record, err := ctx.Install.InstallFromFile(spec.Browser, spec.Version, fromFile)
		if err != nil {
			return fmt.Errorf("安装失败: %w", err)
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

	// 检查是否已安装（使用解析后的完整版本号）
	if !force && ctx.Install.IsInstalled(spec.Browser, versionInfo.Version) {
		ctx.Printf("%s@%s 已安装\n", spec.Browser, versionInfo.Version)
		return nil
	}

	// 强制模式：先卸载
	if force && ctx.Install.IsInstalled(spec.Browser, versionInfo.Version) {
		if err := ctx.Install.Uninstall(spec.Browser, versionInfo.Version); err != nil {
			return fmt.Errorf("卸载现有版本失败: %w", err)
		}
		ctx.Printf("已移除现有版本 %s@%s\n", spec.Browser, versionInfo.Version)
	}

	ctx.Printf("正在下载 %s@%s...\n", spec.Browser, versionInfo.Version)

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "bws-download-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Determine filename from URL
	fileName := fmt.Sprintf("%s-%s-package", spec.Browser, versionInfo.Version)
	if u, err := url.Parse(versionInfo.DownloadURL); err == nil {
		if base := filepath.Base(u.Path); base != "" && base != "/" && base != "\\" && base != "." {
			fileName = base
		}
	}
	downloadDest := filepath.Join(tempDir, fileName)

	var downloadedPath string
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

	ctx.Printf("正在安装 %s@%s...\n", spec.Browser, versionInfo.Version)

	// Install from the downloaded file
	record, err := ctx.Install.InstallFromFile(spec.Browser, versionInfo.Version, downloadedPath)
	if err != nil {
		return fmt.Errorf("安装失败: %w", err)
	}

	ctx.Printf("✓ %s@%s 安装成功\n", record.Browser, record.Version)
	return nil
}

// --- import command ---

func NewImportCommand() *Command {
	return &Command{
		Name:        "import",
		Aliases:     []string{"imp"},
		Description: "从目录导入浏览器版本（自动检测）",
		Usage:       "bws imp <目录> [选项]",
		Examples: []string{
			"imp /path/to/browsers",
			"imp /path/to/browsers -f",
		},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "已安装时强制重新安装", HasValue: false, Default: "false"},
		},
		Run: runImport,
	}
}

func runImport(ctx *Context, args []string) error {
	flagVals, positional, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	if len(positional) == 0 {
		return fmt.Errorf("请指定要导入的目录，例如 'bws imp /path/to/browsers'")
	}

	dir := positional[0]
	force := flagVals["force"] == "true"

	// Check that directory exists
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("目录不存在: %s", dir)
	}

	// Check disk space
	dataDir := "."
	if ctx.Config != nil {
		dataDir = ctx.Config.GetDataDir()
	}
	if err := checkDiskSpace(ctx, dataDir); err != nil {
		return err
	}

	ctx.Printf("正在扫描 %s...\n", dir)

	lastMsg := ""
	summary, err := ctx.Install.ImportFromDir(dir, force, func(current int, total int, message string) {
		// Stream output: print each new message on its own line
		if message != lastMsg {
			fmt.Fprintf(ctx.Stdout, "  %s\n", message)
			lastMsg = message
		}
	})
	if err != nil {
		return err
	}

	ctx.Printf("\n导入完成：\n")
	ctx.Printf("  总计:    %d\n", summary.Total)
	ctx.Printf("  成功:    %d\n", summary.Success)
	ctx.Printf("  跳过:    %d\n", summary.Skipped)
	if summary.SkippedAlreadyInstalled > 0 {
		ctx.Printf("    - 已安装: %d\n", summary.SkippedAlreadyInstalled)
	}
	if summary.SkippedIncompatible > 0 {
		ctx.Printf("    - 不兼容: %d\n", summary.SkippedIncompatible)
	}
	ctx.Printf("  失败:    %d\n", summary.Failed)
	if summary.FailedUnrecognized > 0 {
		ctx.Printf("    - 无法识别: %d\n", summary.FailedUnrecognized)
	}

	if len(summary.Errors) > 0 {
		ctx.Printf("\n失败列表：\n")
		for _, e := range summary.Errors {
			name := e.Path
			if e.Browser != "" && e.Version != "" {
				name = fmt.Sprintf("%s@%s", e.Browser, e.Version)
			}
			ctx.Printf("  ✗ %s: %s\n", name, e.Error)
		}
	}

	if summary.Failed > 0 {
		return fmt.Errorf("%d 个导入失败", summary.Failed)
	}
	return nil
}

// --- uninstall command ---

func NewUninstallCommand() *Command {
	return &Command{
		Name:        "uninstall",
		Aliases:     []string{"rm", "remove"},
		Description: "卸载指定版本的浏览器",
		Usage:       "bws rm <浏览器@版本>",
		Examples: []string{
			"rm chrome@120",
			"rm firefox@121",
		},
		Run: runUninstall,
	}
}

func runUninstall(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定要卸载的版本")
	}

	spec := parseBrowserVersion(args[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)

	if !ctx.Install.IsInstalled(spec.Browser, spec.Version) {
		ctx.Printf("%s@%s 未安装\n", spec.Browser, spec.Version)
		return nil
	}

	if err := ctx.Install.Uninstall(spec.Browser, spec.Version); err != nil {
		return fmt.Errorf("卸载失败: %w", err)
	}

	ctx.Printf("✓ %s@%s 已卸载\n", spec.Browser, spec.Version)
	return nil
}

// --- use command ---

func NewUseCommand() *Command {
	return &Command{
		Name:        "use",
		Aliases:     []string{"u"},
		Description: "设置默认浏览器版本",
		Usage:       "bws u <浏览器@版本>",
		Examples: []string{
			"u chrome@120",
			"u firefox@latest",
			"u chrome@system",
		},
		Run: runUse,
	}
}

func runUse(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定版本")
	}

	spec := parseBrowserVersion(args[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)

	// Handle "system" alias
	if spec.Version == "system" {
		sysVersions, err := ctx.Install.ListWithSystemByBrowser(spec.Browser)
		if err != nil {
			return err
		}
		found := false
		for _, v := range sysVersions {
			if v.IsSystem && v.Channel == "stable" {
				spec.Version = v.Version
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("未找到系统安装的 %s", spec.Browser)
		}
	}

	// Store as alias "default"
	target := fmt.Sprintf("%s@%s", spec.Browser, spec.Version)
	if err := ctx.Config.AddAlias("default", target); err != nil {
		return err
	}

	ctx.Printf("当前使用 %s\n", target)
	return nil
}

// --- alias command ---

func NewAliasCommand() *Command {
	return &Command{
		Name:        "alias",
		Description: "管理版本别名",
		Usage:       "bws alias <名称> <目标>",
		SubCommands: []*Command{
			{
				Name:        "list",
				Description: "列出所有别名",
				Run:         runAliasList,
			},
			{
				Name:        "add",
				Description: "添加别名",
				Run:         runAliasAdd,
			},
			{
				Name:        "remove",
				Description: "删除别名",
				Run:         runAliasRemove,
			},
		},
	}
}

func runAliasList(ctx *Context, args []string) error {
	aliases := ctx.Config.ListAliases()
	if len(aliases) == 0 {
		ctx.Println("暂无别名。")
		return nil
	}

	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ctx.Printf("  %s -> %s\n", name, aliases[name])
	}
	return nil
}

func runAliasAdd(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: bws alias add <名称> <目标>")
	}
	if err := ctx.Config.AddAlias(args[0], args[1]); err != nil {
		return err
	}
	ctx.Printf("别名已添加: %s -> %s\n", args[0], args[1])
	return nil
}

func runAliasRemove(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws alias remove <名称>")
	}
	if err := ctx.Config.RemoveAlias(args[0]); err != nil {
		return err
	}
	ctx.Printf("别名已删除: %s\n", args[0])
	return nil
}

// --- repo command ---

func NewRepoCommand() *Command {
	return &Command{
		Name:        "repo",
		Description: "管理本地二进制仓库",
		Usage:       "bws repo [命令]",
		SubCommands: []*Command{
			{
				Name:        "path",
				Description: "显示当前仓库路径",
				Run:         runRepoPath,
			},
			{
				Name:        "set",
				Description: "设置仓库路径",
				Usage:       "bws repo set <路径>",
				Run:         runRepoSet,
			},
			{
				Name:        "scan",
				Description: "扫描仓库中的浏览器版本",
				Run:         runRepoScan,
			},
			{
				Name:        "import",
				Description: "从仓库导入浏览器版本",
				Flags: []*Flag{
					{Name: "force", Short: "f", Usage: "强制重新安装现有版本", HasValue: false, Default: "false"},
				},
				Run: runRepoImport,
			},
		},
		Run: runRepoPath,
	}
}

func runRepoPath(ctx *Context, args []string) error {
	path := ctx.Config.GetRepoPath()
	if path == "" {
		ctx.Println("未配置仓库路径。")
		ctx.Println("使用 'bws repo set <路径>' 进行配置。")
		return nil
	}
	ctx.Printf("仓库路径: %s\n", path)
	return nil
}

func runRepoSet(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws repo set <路径>")
	}
	path := args[0]
	if err := ctx.Config.SetRepoPath(path); err != nil {
		return fmt.Errorf("设置仓库路径失败: %w", err)
	}
	ctx.Printf("仓库路径已设置为: %s\n", path)
	return nil
}

func runRepoScan(ctx *Context, args []string) error {
	if ctx.Repo == nil {
		return fmt.Errorf("当前构建不支持仓库功能")
	}

	path := ctx.Config.GetRepoPath()
	if path == "" {
		return fmt.Errorf("未配置仓库路径。请先使用 'bws repo set <路径>' 配置")
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
	statusCounts := make(map[string]int)
	for _, r := range results {
		statusCounts[r.Status]++
	}

	ctx.Printf("统计:\n")
	for status, count := range statusCounts {
		ctx.Printf("  %-15s %d\n", status+":", count)
	}
	ctx.Println()

	// Print recognized versions
	ctx.Println("已识别的版本:")
	for _, r := range results {
		if r.Status == "ok" || r.Status == "partial" {
			detail := ""
			if r.Status == "partial" && r.Detail != "" {
				detail = " (" + r.Detail + ")"
			}
			ctx.Printf("  %s@%s  [%s]%s\n", r.Browser, r.Version, r.Arch, detail)
		}
	}

	return nil
}

func runRepoImport(ctx *Context, args []string) error {
	if ctx.Repo == nil {
		return fmt.Errorf("当前构建不支持仓库功能")
	}

	flags, _, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	force := flags["force"] == "true"

	path := ctx.Config.GetRepoPath()
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

// --- tui command ---

// --- info command ---

func NewInfoCommand() *Command {
	return &Command{
		Name:        "info",
		Aliases:     []string{"show"},
		Description: "显示浏览器版本的详细信息",
		Usage:       "bws show <浏览器@版本>",
		Examples: []string{
			"show chrome@120",
			"show chrome@latest",
		},
		Run: runInfo,
	}
}

func runInfo(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定版本，例如 'bws show chrome@120'")
	}

	spec := parseBrowserVersion(args[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)

	// Check if it's installed
	if ctx.Install.IsInstalled(spec.Browser, spec.Version) {
		record, err := ctx.Install.GetRecord(spec.Browser, spec.Version)
		if err != nil {
			return fmt.Errorf("获取记录失败: %w", err)
		}

		ctx.Printf("%s@%s\n", record.Browser, record.Version)
		ctx.Printf("  平台:         %s\n", record.Platform)
		ctx.Printf("  架构:         %s\n", record.Arch)
		ctx.Printf("  安装时间:     %s\n", record.InstalledAt)
		ctx.Printf("  来源:         %s\n", record.Source)
		ctx.Printf("  安装目录:     %s\n", record.InstallDir)
		ctx.Printf("  可执行文件:   %s\n", record.ExecutablePath)
		return nil
	}

	// Check if it's a system version
	if ctx.Install.IsSystemVersion(spec.Browser, spec.Version) {
		versions, err := ctx.Install.ListWithSystemByBrowser(spec.Browser)
		if err != nil {
			return err
		}
		for _, v := range versions {
			if v.Version == spec.Version {
				ctx.Printf("%s@%s\n", v.Browser, v.Version)
				ctx.Printf("  类型:     系统安装\n")
				ctx.Printf("  渠道:     %s\n", v.Channel)
				return nil
			}
		}
	}

	// Try to resolve from remote source
	if ctx.Source != nil {
		versionInfo, err := ctx.Source.ResolveVersion(spec.Browser, spec.Version)
		if err == nil && versionInfo.Version != "" {
			ctx.Printf("%s@%s（远程）\n", versionInfo.Browser, versionInfo.Version)
			ctx.Printf("  渠道:         %s\n", versionInfo.Channel)
			ctx.Printf("  平台:         %s\n", versionInfo.Platform)
			ctx.Printf("  架构:         %s\n", versionInfo.Arch)
			if versionInfo.DownloadURL != "" {
				ctx.Printf("  下载链接:     %s\n", versionInfo.DownloadURL)
			}
			ctx.Printf("\n  未安装。使用 'bws i %s@%s' 进行安装。\n", spec.Browser, versionInfo.Version)
			return nil
		}
	}

	return fmt.Errorf("%s@%s 未找到（未安装且远程源中也不可用）", spec.Browser, spec.Version)
}

// --- list-remote command ---

// --- Grouped version display ---

// majorVersionGroup holds versions grouped by their major version number.
type majorVersionGroup struct {
	Major    string
	Versions []string
	Channel  string
	Platform string
	Arch     string
	Installed bool
	Count    int
}

// groupByMajorVersion groups version table rows by their major version number.
// Input rows are expected to have format: [version, channel, platform, arch, status]
func groupByMajorVersion(rows [][]string) []majorVersionGroup {
	groups := make(map[string]*majorVersionGroup)
	var order []string

	for _, row := range rows {
		ver := row[0]
		major := extractMajorVersion(ver)
		if major == "" {
			major = ver
		}

		g, ok := groups[major]
		if !ok {
			g = &majorVersionGroup{
				Major:    major,
				Channel:  row[1],
				Platform: row[2],
				Arch:     row[3],
			}
			groups[major] = g
			order = append(order, major)
		}
		g.Versions = append(g.Versions, ver)
		g.Count++
		if row[4] == "已安装" {
			g.Installed = true
		}
	}

	// Sort result by version descending
	result := make([]majorVersionGroup, 0, len(order))
	for _, major := range order {
		result = append(result, *groups[major])
	}
	return result
}

// printGroupedVersions prints versions grouped by major version.
// Shows at most limit groups, each with its versions in a compact inline format.
func printGroupedVersions(ctx *Context, groups []majorVersionGroup, limit int) {
	showCount := min(limit, len(groups))

	for i := 0; i < showCount; i++ {
		g := groups[i]
		status := ""
		if g.Installed {
			status = " [已安装]"
		}
		// Print group header: "152.x (6)  amd64  windows"
		ctx.Printf("  %s.x (%d)  %s  %s%s\n", g.Major, g.Count, g.Platform, g.Arch, status)

		// Print versions in a compact inline format
		line := "    "
		for j, ver := range g.Versions {
			if j > 0 {
				line += "  "
			}
			line += ver
		}
		ctx.Println(line)
	}

	if len(groups) > showCount {
		ctx.Printf("  ... 还有 %d 个主版本组未显示\n", len(groups)-showCount)
	}
}

// extractMajorVersion extracts the major version number from a version string.
// e.g., "152.0.5" -> "152", "128.8.0esr" -> "128"
func extractMajorVersion(ver string) string {
	// Strip non-numeric suffixes like "esr", "a1", "b10"
	clean := ver
	clean = strings.Split(clean, "esr")[0]
	clean = strings.Split(clean, "a")[0]
	clean = strings.Split(clean, "b")[0]
	parts := strings.Split(clean, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- download command ---

func NewDownloadCommand() *Command {
	return &Command{
		Name:        "download",
		Aliases:     []string{"dl"},
		Description: "下载浏览器版本但不安装",
		Usage:       "bws dl <浏览器@版本> [选项]",
		Examples: []string{
			"dl chrome@120",
			"dl chrome@latest --output ~/downloads",
			"dl chrome@beta --channel beta",
		},
		Flags: []*Flag{
			{Name: "output", Short: "o", Usage: "输出目录", HasValue: true, Default: ""},
			{Name: "channel", Short: "c", Usage: "发布渠道", HasValue: true, Default: "stable"},
		},
		Run: runDownload,
	}
}

func runDownload(ctx *Context, args []string) error {
	if ctx.Source == nil || ctx.Download == nil {
		return fmt.Errorf("当前构建不支持下载功能")
	}

	if len(args) == 0 {
		return fmt.Errorf("请指定要下载的版本，例如 'bws dl chrome@120'")
	}

	spec := parseBrowserVersion(args[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)
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
		if ctx.Config != nil {
			checkPath = ctx.Config.GetDataDir()
		}
	}
	if err := checkDiskSpace(ctx, checkPath); err != nil {
		return err
	}

	// Resolve the version
	ctx.Printf("正在解析 %s@%s...\n", spec.Browser, spec.Version)
	versionInfo, err := ctx.Source.ResolveVersion(spec.Browser, spec.Version)
	if err != nil {
		// Try channel name
		versions, listErr := ctx.Source.ListVersions(spec.Browser, channel)
		if listErr == nil && len(versions) > 0 {
			versionInfo = versions[0]
		} else {
			return fmt.Errorf("解析版本失败: %w", err)
		}
	}

	if versionInfo.DownloadURL == "" {
		return fmt.Errorf("%s@%s 没有可用的下载链接", spec.Browser, versionInfo.Version)
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
	fileName := fmt.Sprintf("%s-%s-package", spec.Browser, versionInfo.Version)
	if u, err := url.Parse(versionInfo.DownloadURL); err == nil {
		if base := filepath.Base(u.Path); base != "" && base != "/" && base != "\\" && base != "." {
			fileName = base
		}
	}
	destPath := filepath.Join(outputDir, fileName)

	ctx.Printf("正在下载 %s@%s 到 %s...\n", spec.Browser, versionInfo.Version, outputDir)

	_, err = ctx.Download.Download(versionInfo.DownloadURL, destPath, func(downloaded, total int64, percent float64) {
		if total > 0 {
			ctx.Printf("\r  进度: %.1f%%", percent)
		} else {
			ctx.Printf("\r  下载中...")
		}
	})
	ctx.Println()

	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	ctx.Printf("✓ 已下载到 %s\n", destPath)
	return nil
}

// --- config command ---

func NewConfigCommand() *Command {
	return &Command{
		Name:        "config",
		Aliases:     []string{"cfg"},
		Description: "管理 bws 配置",
		Usage:       "bws cfg <命令> [选项]",
		SubCommands: []*Command{
			NewConfigShowCommand(),
			NewConfigGetCommand(),
			NewConfigSetCommand(),
			NewConfigPathCommand(),

		},
	}
}

func NewConfigShowCommand() *Command {
	return &Command{
		Name:        "show",
		Aliases:     []string{"list", "ls"},
		Description: "显示所有配置项",
		Run:         runConfigShow,
	}
}

func runConfigShow(ctx *Context, args []string) error {
	ctx.Printf("配置信息：\n\n")
	ctx.Printf("  配置文件:       %s\n", ctx.Config.ConfigPath())
	ctx.Printf("  数据目录:       %s\n", ctx.Config.GetDataDir())
	ctx.Printf("  默认浏览器:     %s\n", ctx.Config.DefaultBrowser())
	ctx.Printf("  默认渠道:       %s\n", ctx.Config.DefaultChannel())
	ctx.Printf("  日志级别:       %s\n", ctx.Config.GetLogLevel())
	ctx.Printf("  仓库路径:       %s\n", ctx.Config.GetRepoPath())
	ctx.Printf("\n  数据源开关:\n")
	ctx.Printf("    Serve 源:     %s\n", boolStr(ctx.Config.IsServeSourceEnabled()))
	ctx.Printf("    Omaha 源:     %s\n", boolStr(ctx.Config.IsOmahaSourceEnabled()))
	ctx.Printf("    Firefox FTP:  %s\n", boolStr(ctx.Config.IsFirefoxFTPEnabled()))
	ctx.Printf("\n  磁盘空间阈值:   %d GB (低于此值会提示)\n", ctx.Config.GetDiskSpaceThresholdGB())

	proxy := ctx.Config.GetProxy()
	if proxy == "" {
		ctx.Printf("  代理:           (未设置，直连)\n")
	} else {
		ctx.Printf("  代理:           %s\n", proxy)
	}

	aliases := ctx.Config.ListAliases()
	if len(aliases) > 0 {
		ctx.Printf("\n  别名:\n")
		for name, target := range aliases {
			ctx.Printf("    %s -> %s\n", name, target)
		}
	}
	return nil
}

func NewConfigGetCommand() *Command {
	return &Command{
		Name:        "get",
		Description: "获取配置项的值",
		Usage:       "bws cfg get <键名>",
		Examples: []string{
			"cfg get default-browser",
			"cfg get log-level",
			"cfg get data-dir",
		},
		Run: runConfigGet,
	}
}

func runConfigGet(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定配置键名。使用 'bws cfg show' 查看所有配置项。")
	}

	key := strings.ToLower(args[0])
	switch key {
	case "default-browser", "default", "browser":
		ctx.Println(ctx.Config.DefaultBrowser())
	case "default-channel", "channel":
		ctx.Println(ctx.Config.DefaultChannel())
	case "log-level", "log":
		ctx.Println(ctx.Config.GetLogLevel())
	case "data-dir", "datadir", "data":
		ctx.Println(ctx.Config.GetDataDir())
	case "repo-path", "repo":
		ctx.Println(ctx.Config.GetRepoPath())
	case "source", "remote-source", "remote":
		src := ctx.Config.GetRemoteSource()
		if src == "" {
			ctx.Println("（未设置）")
		} else {
			ctx.Println(src)
		}
	case "source-serve", "serve-source":
		ctx.Println(boolStr(ctx.Config.IsServeSourceEnabled()))
	case "source-omaha", "omaha-source":
		ctx.Println(boolStr(ctx.Config.IsOmahaSourceEnabled()))
	case "source-firefox-ftp", "firefox-ftp":
		ctx.Println(boolStr(ctx.Config.IsFirefoxFTPEnabled()))
	case "disk-threshold", "disk-space-threshold", "space-threshold":
		ctx.Printf("%d GB\n", ctx.Config.GetDiskSpaceThresholdGB())
	case "proxy":
		p := ctx.Config.GetProxy()
		if p == "" {
			ctx.Println("(未设置)")
		} else {
			ctx.Println(p)
		}
	case "path", "config-path", "config":
		ctx.Println(ctx.Config.ConfigPath())
	default:
		// Try alias
		if alias, ok := ctx.Config.GetAlias(args[0]); ok {
			ctx.Println(alias)
			return nil
		}
		return fmt.Errorf("未知的配置键: %s。使用 'bws cfg show' 查看可用配置项。", args[0])
	}
	return nil
}

func NewConfigSetCommand() *Command {
	return &Command{
		Name:        "set",
		Description: "设置配置项的值",
		Usage:       "bws cfg set <键名> <值>",
		Examples: []string{
			"cfg set default-browser firefox",
			"cfg set log-level debug",
			"cfg set data-dir /path/to/data",
			"cfg set repo-path /path/to/repo",
		},
		Run: runConfigSet,
	}
}

func runConfigSet(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: bws cfg set <键名> <值>。使用 'bws cfg show' 查看可用配置项。")
	}

	key := strings.ToLower(args[0])
	value := args[1]

	switch key {
	case "default-browser", "default", "browser":
		if !ctx.Browsers.Has(value) {
			return fmt.Errorf("未知的浏览器: %s", value)
		}
		if err := ctx.Config.SetDefaultBrowser(value); err != nil {
			return fmt.Errorf("设置默认浏览器失败: %w", err)
		}
		ctx.Printf("默认浏览器已设置为: %s\n", value)

	case "default-channel", "channel":
		validChannels := map[string]bool{"stable": true, "beta": true, "dev": true, "canary": true}
		if !validChannels[strings.ToLower(value)] {
			return fmt.Errorf("无效的渠道: %s（必须是 stable、beta、dev 或 canary）", value)
		}
		if err := ctx.Config.SetDefaultChannel(value); err != nil {
			return fmt.Errorf("设置默认渠道失败: %w", err)
		}
		ctx.Printf("默认渠道已设置为: %s\n", value)

	case "log-level", "log":
		level := strings.ToLower(value)
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
		if !validLevels[level] {
			return fmt.Errorf("无效的日志级别: %s（必须是 debug、info、warn 或 error）", value)
		}
		if err := ctx.Config.SetLogLevel(level); err != nil {
			return fmt.Errorf("设置日志级别失败: %w", err)
		}
		ctx.Printf("日志级别已设置为: %s\n", level)

	case "data-dir", "datadir", "data":
		if err := ctx.Config.SetDataDir(value); err != nil {
			return fmt.Errorf("设置数据目录失败: %w", err)
		}
		ctx.Printf("数据目录已设置为: %s\n", value)
		ctx.Printf("注意: 重启 bws 后生效。\n")

	case "repo-path", "repo":
		if err := ctx.Config.SetRepoPath(value); err != nil {
			return fmt.Errorf("设置仓库路径失败: %w", err)
		}
		ctx.Printf("仓库路径已设置为: %s\n", value)

	case "source", "remote-source", "remote":
		if err := ctx.Config.SetRemoteSource(value); err != nil {
			return fmt.Errorf("设置离线源失败: %w", err)
		}
		ctx.Printf("离线源已设置为: %s\n", value)
		ctx.Println("优先级: 离线源 → 在线源（离线源优先）")

	case "source-serve", "serve-source":
		enabled := parseBool(value)
		if err := ctx.Config.SetServeSourceEnabled(enabled); err != nil {
			return fmt.Errorf("设置 Serve 源开关失败: %w", err)
		}
		if enabled {
			ctx.Println("Serve 源已启用")
		} else {
			ctx.Println("Serve 源已禁用")
		}

	case "source-omaha", "omaha-source":
		enabled := parseBool(value)
		if err := ctx.Config.SetOmahaSourceEnabled(enabled); err != nil {
			return fmt.Errorf("设置 Omaha 源开关失败: %w", err)
		}
		if enabled {
			ctx.Println("Omaha 源已启用")
		} else {
			ctx.Println("Omaha 源已禁用")
		}

	case "source-firefox-ftp", "firefox-ftp":
		enabled := parseBool(value)
		if err := ctx.Config.SetFirefoxFTPEnabled(enabled); err != nil {
			return fmt.Errorf("设置 Firefox FTP 源开关失败: %w", err)
		}
		if enabled {
			ctx.Println("Firefox FTP 源已启用")
		} else {
			ctx.Println("Firefox FTP 源已禁用")
		}

	case "disk-threshold", "disk-space-threshold", "space-threshold":
		gb, err := strconv.Atoi(value)
		if err != nil || gb <= 0 {
			return fmt.Errorf("无效的阈值: %s（必须是正整数，单位 GB）", value)
		}
		if err := ctx.Config.SetDiskSpaceThresholdGB(gb); err != nil {
			return fmt.Errorf("设置磁盘空间阈值失败: %w", err)
		}
		ctx.Printf("磁盘空间阈值已设置为: %d GB\n", gb)

	case "proxy":
		// Allow "none", "direct", "" to clear proxy
		if value == "none" || value == "direct" || value == "" {
			if err := ctx.Config.SetProxy(""); err != nil {
				return fmt.Errorf("清除代理设置失败: %w", err)
			}
			ctx.Println("代理已清除（直连模式）")
		} else {
			// Validate proxy URL format
			if err := validateProxyURL(value); err != nil {
				return err
			}
			if err := ctx.Config.SetProxy(value); err != nil {
				return fmt.Errorf("设置代理失败: %w", err)
			}
			ctx.Printf("代理已设置为: %s\n", value)
		}

	default:
		// Treat as alias
		if err := ctx.Config.AddAlias(key, value); err != nil {
			return fmt.Errorf("设置别名失败: %w", err)
		}
		ctx.Printf("别名已设置: %s -> %s\n", key, value)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("config set: %s = %s", key, value)
	}

	return nil
}

func NewConfigPathCommand() *Command {
	return &Command{
		Name:        "path",
		Description: "显示配置文件路径",
		Run: func(ctx *Context, args []string) error {
			ctx.Println(ctx.Config.ConfigPath())
			return nil
		},
	}
}

// --- doctor command ---

func NewDoctorCommand() *Command {
	return &Command{
		Name:        "doctor",
		Aliases:     []string{"dt"},
		Description: "检查系统健康状态并诊断问题",
		Usage:       "bws dt",
		Run:         runDoctor,
	}
}

func runDoctor(ctx *Context, args []string) error {
	issues := 0
	okCount := 0

	check := func(name string, ok bool, detail string) {
		if ok {
			ctx.Printf("  ✓ %s: %s\n", name, detail)
			okCount++
		} else {
			ctx.Printf("  ✗ %s: %s\n", name, detail)
			issues++
		}
	}

	ctx.Printf("正在运行健康检查...\n\n")

	// Check paths
	pathsOk := true
	if err := ctx.Paths.EnsureAll(); err != nil {
		pathsOk = false
	}
	check("目录结构", pathsOk, "所有必需目录已存在")

	// Check config
	check("配置文件", true, "配置加载成功")

	// Check browsers
	browserCount := len(ctx.Browsers.List())
	check("浏览器描述符", browserCount > 0, fmt.Sprintf("支持 %d 种浏览器", browserCount))

	// Check installed versions
	installed, err := ctx.Install.ListInstalled()
	if err != nil {
		check("已安装版本", false, fmt.Sprintf("错误: %v", err))
	} else {
		check("已安装版本", true, fmt.Sprintf("已安装 %d 个版本", len(installed)))
	}

	// Check system browser detection
	if ctx.Install.HasSystem() {
		sysVersions, _ := ctx.Install.ListWithSystem()
		sysCount := 0
		for _, v := range sysVersions {
			if v.IsSystem {
				sysCount++
			}
		}
		check("系统浏览器", true, fmt.Sprintf("检测到 %d 个系统浏览器", sysCount))
	} else {
		check("系统浏览器", true, "未检测到系统浏览器（可选）")
	}

	// Check remote source
	if ctx.Source != nil {
		check("远程源", true, "远程版本信息可用")
	} else {
		check("远程源", false, "当前构建不可用")
	}

	// Check download
	if ctx.Download != nil {
		check("下载支持", true, "下载管理器可用")
	} else {
		check("下载支持", false, "当前构建不可用")
	}

	ctx.Printf("\n%d 项检查通过，发现 %d 个问题\n", okCount, issues)

	if issues > 0 {
		return fmt.Errorf("发现 %d 个问题", issues)
	}
	return nil
}

// --- cache command ---

func NewCacheCommand() *Command {
	return &Command{
		Name:        "cache",
		Aliases:     []string{"cc"},
		Description: "管理下载缓存",
		Usage:       "bws cc <命令>",
		SubCommands: []*Command{
			NewCacheClearCommand(),
			NewCacheInfoCommand(),
		},
	}
}

func NewCacheClearCommand() *Command {
	return &Command{
		Name:        "clear",
		Description: "清除所有缓存的下载文件",
		Run:         runCacheClear,
	}
}

func runCacheClear(ctx *Context, args []string) error {
	// Cache is stored in temp directories by default
	// For now, just note that we don't have persistent cache
	ctx.Printf("注意: 下载文件存储在临时目录中，会自动清理。\n")
	ctx.Printf("没有需要清除的持久化缓存。\n")
	return nil
}

func NewCacheInfoCommand() *Command {
	return &Command{
		Name:        "info",
		Description: "显示缓存信息",
		Run:         runCacheInfo,
	}
}

func runCacheInfo(ctx *Context, args []string) error {
	ctx.Printf("缓存状态:\n")
	ctx.Printf("  类型: 临时（自动清理）\n")
	ctx.Printf("  说明: 下载文件存储在临时目录中，安装后会自动清理。\n")
	return nil
}

// --- profile command ---

func NewProfileCommand() *Command {
	return &Command{
		Name:        "profile",
		Aliases:     []string{"pf"},
		Description: "管理浏览器 profile（数据目录）",
		Usage:       "bws pf <command> [options]",
		SubCommands: []*Command{
			NewProfileListCommand(),
			NewProfilePathCommand(),
			NewProfileResetCommand(),
			NewProfileCleanCommand(),
		},
	}
}

func NewProfileListCommand() *Command {
	return &Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "列出指定浏览器的所有 profile",
		Usage:       "bws pf list [browser] [options]",
		Examples: []string{
			"pf list",
			"pf list chrome",
			"pf list --browser firefox",
		},
		Flags: []*Flag{
			{Name: "browser", Short: "b", Usage: "指定浏览器", HasValue: true, Default: ""},
		},
		Run: runProfileList,
	}
}

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
			browser = ctx.Config.DefaultBrowser()
		}
	}

	profiles, err := ctx.Profile.ListProfiles(browser)
	if err != nil {
		return fmt.Errorf("获取 profile 列表失败: %w", err)
	}

	ctx.Printf("浏览器 %s 的 profile 列表:\n\n", browser)

	if len(profiles) == 0 {
		ctx.Println("  暂无 profile。")
		ctx.Println()
		ctx.Printf("  使用 'bws r %s --profile <名称>' 创建命名 profile。\n", browser)
		return nil
	}

	// 分类显示
	var namedProfiles []ProfileInfo
	var versionProfiles []ProfileInfo
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

func NewProfilePathCommand() *Command {
	return &Command{
		Name:        "path",
		Description: "显示 profile 目录路径",
		Usage:       "bws pf path [browser] [profileName]",
		Examples: []string{
			"pf path",
			"pf path chrome",
			"pf path chrome my-profile",
		},
		Run: runProfilePath,
	}
}

func runProfilePath(ctx *Context, args []string) error {
	browser := ctx.Config.DefaultBrowser()
	profileName := ""

	if len(args) > 0 {
		browser = args[0]
	}
	if len(args) > 1 {
		profileName = args[1]
	}

	// 对于默认 profile，需要一个版本号
	version := ""
	if profileName == "" {
		// 尝试获取已安装的最新版本
		versions, err := ctx.Install.ListInstalledByBrowser(browser)
		if err == nil && len(versions) > 0 {
			version = versions[0].Version
		} else {
			// 如果没有已安装版本，使用 "latest" 作为占位符
			version = "latest"
		}
	}

	profileDir := ctx.Profile.ProfileDir(browser, version, profileName)
	ctx.Println(profileDir)
	return nil
}

func NewProfileResetCommand() *Command {
	return &Command{
		Name:        "reset",
		Description: "重置（清除）指定的 profile 数据",
		Usage:       "bws pf reset <browser@version> [profileName] [options]",
		Examples: []string{
			"pf reset chrome@120",
			"pf reset chrome@latest my-profile",
			"pf reset chrome@120 --force",
		},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "跳过确认直接重置", HasValue: false, Default: "false"},
		},
		Run: runProfileReset,
	}
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

	spec := parseBrowserVersion(positional[0], ctx.Config.DefaultBrowser())
	spec = resolveBrowserSpec(ctx, spec)

	profileName := ""
	if len(positional) > 1 {
		profileName = positional[1]
	}

	// 解析版本别名（仅对默认 profile 有意义）
	if spec.IsAlias && spec.Version != "system" && profileName == "" {
		resolved, err := ctx.Install.ListInstalledByBrowser(spec.Browser)
		if err == nil && len(resolved) > 0 {
			// 使用最新版本
			spec.Version = resolved[0].Version
		}
		// 如果无法解析，继续使用别名作为版本名（profile 目录可能已存在）
	}

	force := flags["force"] == "true"
	profileDir := ctx.Profile.ProfileDir(spec.Browser, spec.Version, profileName)

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
		ctx.Printf("确定要重置此 profile 吗？此操作不可恢复。(y/N): ")
		reader := bufioNewReader(ctx.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
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

func NewProfileCleanCommand() *Command {
	return &Command{
		Name:        "clean",
		Description: "清理已卸载版本的孤立 profile",
		Usage:       "bws pf clean [browser] [options]",
		Examples: []string{
			"pf clean",
			"pf clean chrome",
			"pf clean --force",
		},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "直接执行清理，不提示确认", HasValue: false, Default: "false"},
		},
		Run: runProfileClean,
	}
}

func runProfileClean(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "force", Short: "f", Usage: "直接执行清理", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	browser := ctx.Config.DefaultBrowser()
	if len(positional) > 0 {
		browser = positional[0]
	}

	force := flags["force"] == "true"

	// 查找孤立的 profile
	orphaned, err := ctx.Profile.CleanOrphanedProfiles(browser)
	if err != nil {
		return fmt.Errorf("扫描孤立 profile 失败: %w", err)
	}

	ctx.Printf("浏览器 %s 的孤立 profile 扫描结果:\n\n", browser)

	if len(orphaned) == 0 {
		ctx.Println("  没有发现孤立的 profile。")
		return nil
	}

	ctx.Printf("  发现 %d 个孤立 profile:\n\n", len(orphaned))
	for _, p := range orphaned {
		ctx.Printf("    - %s\n", p)
	}
	ctx.Println()

	// 确认
	if !force {
		ctx.Printf("确定要删除这些 profile 吗？此操作不可恢复。(y/N): ")
		reader := bufioNewReader(ctx.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			ctx.Println("已取消。")
			return nil
		}
	}

	// 执行清理
	cleaned := 0
	for _, p := range orphaned {
		ctx.Printf("  清理: %s\n", p)
		if err := os.RemoveAll(p); err != nil {
			ctx.Printf("    警告: 删除失败: %v\n", err)
			continue
		}
		cleaned++
	}

	ctx.Printf("\n✓ 已清理 %d 个孤立 profile\n", cleaned)
	return nil
}

// bufioNewReader creates a new bufio.Reader from an io.Reader.
func bufioNewReader(r io.Reader) *bufioReader {
	return &bufioReader{r: bufio.NewReader(r)}
}

type bufioReader struct {
	r *bufio.Reader
}

func (b *bufioReader) ReadString(delim byte) (string, error) {
	return b.r.ReadString(delim)
}

// --- serve command ---

func NewServeCommand() *Command {
	return &Command{
		Name:        "serve",
		Aliases:     []string{"sv", "server"},
		Description: "启动 HTTP 服务以提供浏览器版本下载",
		Usage:       "bws sv [选项]",
		Examples: []string{
			"sv",
			"sv -d /path/to/data",
		},
		Flags: []*Flag{
			{Name: "dir", Short: "d", Usage: "基础目录（包含 packages/ 和 bin/ 子目录，默认: 程序所在目录）", HasValue: true, Default: ""},
		},
		Run: runServe,
	}
}

func runServe(ctx *Context, args []string) error {
	flags, _, err := ParseFlags(args, []*Flag{
		{Name: "dir", Short: "d", Usage: "基础目录", HasValue: true, Default: ""},
	})
	if err != nil {
		return err
	}

	baseDir := flags["dir"]

	if ctx.Serve == nil {
		return fmt.Errorf("当前构建不支持 serve 功能")
	}

	// 确保配置文件存在
	configPath, isNew, err := ctx.Serve.EnsureDefaultConfig(baseDir)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}

	if isNew {
		ctx.Printf("配置文件已创建: %s\n", configPath)
		ctx.Printf("请编辑配置文件，然后重新运行 bws sv\n\n")
		ctx.Printf("配置项说明:\n")
		ctx.Printf("  host          = 监听地址，默认 0.0.0.0\n")
		ctx.Printf("  port          = 监听端口，默认 8080\n")
		ctx.Printf("  packages-dir  = 浏览器安装包存放目录\n")
		ctx.Printf("  bin-dir       = 客户端二进制存放目录\n")
		ctx.Printf("  sync          = 是否启用自动同步 (true/false)\n")
		ctx.Printf("  sync-interval = 同步间隔，如 24h、30d\n")
		ctx.Printf("  sync-browsers = 同步浏览器，逗号分隔，留空表示全部\n")
		ctx.Printf("  sync-channels = 同步渠道，逗号分隔，默认 stable\n")
		return nil
	}

	// 检查磁盘空间
	checkPath := baseDir
	if checkPath == "" {
		checkPath = "."
	}
	if err := checkDiskSpace(ctx, checkPath); err != nil {
		return err
	}

	return ctx.Serve.StartFromConfig(baseDir)
}

// --- helpers ---

type browserVersionSpec struct {
	Browser string
	Version string
	IsAlias bool
}

func parseBrowserVersion(input string, defaultBrowser string) browserVersionSpec {
	input = strings.TrimSpace(input)

	if input == "" {
		return browserVersionSpec{Browser: defaultBrowser, Version: "latest", IsAlias: true}
	}

	if idx := strings.Index(input, "@"); idx > 0 {
		browser := strings.TrimSpace(input[:idx])
		ver := strings.TrimSpace(input[idx+1:])
		isAlias := isVersionAlias(ver)
		return browserVersionSpec{Browser: browser, Version: ver, IsAlias: isAlias}
	}

	// Check if it looks like a version
	if looksLikeVersion(input) {
		return browserVersionSpec{Browser: defaultBrowser, Version: input, IsAlias: false}
	}

	// Check if it's a version alias (e.g. "latest", "beta")
	if isVersionAlias(input) {
		return browserVersionSpec{Browser: defaultBrowser, Version: input, IsAlias: true}
	}

	// Treat as browser name
	return browserVersionSpec{Browser: input, Version: "latest", IsAlias: true}
}

func isVersionAlias(v string) bool {
	v = strings.ToLower(v)
	aliases := map[string]bool{
		"latest": true, "stable": true, "beta": true,
		"dev": true, "canary": true, "esr": true,
		"release": true, "nightly": true, "system": true,
	}
	return aliases[v]
}

// --- help command ---

func NewHelpCommand() *Command {
	return &Command{
		Name:        "help",
		Aliases:     []string{"h"},
		Description: "显示详细帮助信息",
		Usage:       "bws help [topic]",
		Run:         runHelp,
	}
}

func runHelp(ctx *Context, args []string) error {
	if len(args) == 0 {
		// Show main help
		content, err := help.Get("main")
		if err != nil {
			return err
		}
		fmt.Println(content)
		return nil
	}

	topic := args[0]
	content, err := help.Get(topic)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, err.Error())
		fmt.Println()
		fmt.Println("可用帮助主题：")
		fmt.Println()
		topics := help.Topics()
		for _, t := range topics {
			fmt.Printf("  %-10s  %s\n", t.Name, t.Description)
		}
		fmt.Println()
		fmt.Println("使用: bws help <topic> 查看详细帮助。")
		return nil
	}

	fmt.Println(content)
	return nil
}

// resolveBrowserSpec resolves browser name aliases in the spec.
// Returns the updated spec with the canonical browser name.
func resolveBrowserSpec(ctx *Context, spec browserVersionSpec) browserVersionSpec {
	if canonical, ok := ctx.Browsers.ResolveName(spec.Browser); ok {
		spec.Browser = canonical
	}
	return spec
}

func looksLikeVersion(s string) bool {
	if len(s) == 0 {
		return false
	}
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if len(s) == 0 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// matchesVersionPrefix checks if a full version string matches a version prefix.
// e.g. "79" matches "79.0.3945.130", "79.0" matches "79.0.3945.130",
// "79.0.3945.130" matches exactly.
func matchesVersionPrefix(fullVersion string, prefix string) bool {
	if prefix == "" {
		return true
	}
	// Exact match
	if fullVersion == prefix {
		return true
	}
	// Prefix match with dot separator
	prefixDot := prefix + "."
	return strings.HasPrefix(fullVersion, prefixDot)
}

func getBrowserDisplayName(ctx *Context, name string) string {
	desc := ctx.Browsers.Get(name)
	if desc.Name != "" {
		return desc.DisplayName
	}
	return name
}

// Very simple version comparison for sorting
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal, bVal := 0, 0
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aVal)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bVal)
		}
		if aVal < bVal {
			return -1
		}
		if aVal > bVal {
			return 1
		}
	}
	return 0
}

// boolStr converts a bool to "是" or "否" for display.
func boolStr(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// parseBool parses a string as a boolean value.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}

// validateProxyURL validates a proxy URL.
// Supported schemes: http, https, socks5, socks5h.
func validateProxyURL(proxyURL string) error {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("无效的代理地址: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
		// valid
	default:
		return fmt.Errorf("不支持的代理协议: %s（支持 http, https, socks5, socks5h）", scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("代理地址缺少主机和端口: %s", proxyURL)
	}
	return nil
}

// checkDiskSpace checks if there's enough free disk space at the given path.
// If space is below the configured threshold, it warns the user and asks for confirmation.
// Returns an error if the check fails or the user declines to continue.
func checkDiskSpace(ctx *Context, path string) error {
	freeBytes, err := disk.FreeSpace(path)
	if err != nil {
		// Don't block on space check failure, just log a warning
		if ctx.Logger != nil {
			ctx.Logger.Warn("磁盘空间检查失败: %v", err)
		}
		return nil
	}

	thresholdGB := 5
	if ctx.Config != nil {
		thresholdGB = ctx.Config.GetDiskSpaceThresholdGB()
	}
	thresholdBytes := uint64(thresholdGB) * 1024 * 1024 * 1024

	if freeBytes < thresholdBytes {
		freeGB := float64(freeBytes) / (1024 * 1024 * 1024)
		fmt.Fprintf(ctx.Stderr, "\n⚠ 警告: %s 所在磁盘剩余空间不足 (%.1f GB，阈值 %d GB)。\n", path, freeGB, thresholdGB)
		if !ctx.Confirm("是否继续执行？") {
			return fmt.Errorf("用户取消操作")
		}
	}
	return nil
}
