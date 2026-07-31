package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bws/bws/internal/source"
	"github.com/bws/bws/internal/version"
)

func runLs(ctx *Context, args []string) error {
	flags, positional, err := ParseFlags(args, []*Flag{
		{Name: "all", Short: "a", Usage: "显示所有", HasValue: false, Default: "false"},
		{Name: "json", Usage: "JSON 输出", HasValue: false, Default: "false"},
		{Name: "system", Short: "s", Usage: "包含系统安装的浏览器", HasValue: false, Default: "true"},
		{Name: "no-system", Usage: "隐藏系统安装的浏览器", HasValue: false, Default: "false"},
		{Name: "remote", Short: "R", Usage: "列出远程源中的可用版本", HasValue: false, Default: "false"},
		{Name: "refresh", Usage: "强制刷新远程源缓存（远程模式）", HasValue: false, Default: "false"},
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
		spec = resolveSpec(ctx, positional[0], "")
	}

	includeSystem := flags["no-system"] != "true"
	if ctx.Logger != nil {
		ctx.Logger.Debug("[list] 本地模式: browser=%s version=%s includeSystem=%v",
			spec.Browser, spec.Version, includeSystem)
	}

	var versions []version.Version

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

	// 统计本地版本和系统集成版本数量（用于 verbose 日志）
	if ctx.Logger != nil {
		localCount := 0
		sysCount := 0
		for _, v := range versions {
			if v.IsSystem {
				sysCount++
			} else {
				localCount++
			}
		}
		ctx.Logger.Debug("[list] 找到本地版本 %d 个，系统集成版本 %d 个", localCount, sysCount)
	}

	// 按版本前缀筛选
	if spec.Version != "" && !spec.IsAlias {
		var filtered []version.Version
		for _, v := range versions {
			if matchesVersionPrefix(v.Version, spec.Version) {
				filtered = append(filtered, v)
			}
		}
		versions = filtered
		if ctx.Logger != nil {
			ctx.Logger.Debug("[list] 按版本前缀 %q 筛选后剩余 %d 个版本", spec.Version, len(versions))
		}
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
	byBrowser := make(map[string][]version.Version)
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
			return version.Compare(vs[i].Version, vs[j].Version) > 0
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
	if err := checkFeature(ctx.Source, "远程源"); err != nil {
		return err
	}

	// 接受所有 runLs 中的 flag，local-only 的在此模式下忽略
	flagVals, positional, err := ParseFlags(args, []*Flag{
		{Name: "channel", Short: "c", Usage: "按渠道过滤", HasValue: true, Default: "stable"},
		{Name: "limit", Short: "n", Usage: "限制结果数量", HasValue: true, Default: "20"},
		{Name: "all", Short: "a", Usage: "显示所有版本（所有渠道）", HasValue: false, Default: "false"},
		{Name: "refresh", Usage: "强制刷新远程源缓存", HasValue: false, Default: "false"},
		// 以下为 local-only flag，remote 模式下接受但忽略
		{Name: "json", Usage: "JSON 输出（远程模式暂不支持）", HasValue: false, Default: "false"},
		{Name: "system", Short: "s", Usage: "（local-only）", HasValue: false, Default: "true"},
		{Name: "no-system", Usage: "（local-only）", HasValue: false, Default: "false"},
	})
	if err != nil {
		return err
	}

	// 如果指定了 --refresh，对所有支持缓存刷新的源设置强制刷新
	if flagVals["refresh"] == "true" {
		if r, ok := ctx.Source.(interface{ ForceRefresh() }); ok {
			r.ForceRefresh()
		}
	}

	// 解析参数，支持 chrome@79 这样的语法
	spec := browserVersionSpec{Browser: ctx.Cfg.Defaults.DefaultBrowser(), Version: ""}
	if len(positional) > 0 {
		spec = resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())
	}

	channel := flagVals["channel"]
	// 当版本是别名且为有效渠道名时，将其作为默认渠道
	if spec.IsAlias {
		if c := source.ParseChannel(spec.Version); c != source.ChannelUnknown {
			channel = string(c)
		}
	}

	limit := 20
	if flagVals["limit"] != "" {
		if n, err := strconv.Atoi(flagVals["limit"]); err == nil && n > 0 {
			limit = n
		} else {
			return fmt.Errorf("无效的 limit 值: %s（必须为正整数）", flagVals["limit"])
		}
	}
	showAll := flagVals["all"] == "true"

	channels := []string{channel}
	if showAll {
		channels = []string{"stable", "beta", "esr", "dev", "canary"}
	}
	// 当用户指定了具体版本前缀（非别名）时，自动搜索所有渠道，
	// 避免遗漏 ESR 等非默认渠道中的匹配版本。
	if spec.Version != "" && !spec.IsAlias && !showAll {
		channels = []string{"stable", "beta", "esr", "dev", "canary"}
	}

	// 检查浏览器是否支持
	if !ctx.Browsers.Has(spec.Browser) {
		return fmt.Errorf("不支持的浏览器: %s", spec.Browser)
	}

	// 确定当前浏览器相关的源
	// 当 serve 源启用时，客户端仅通过 HTTPSource 访问 serve，不直接查询在线源。
	var activeSources []string
	var hasServe bool
	if ctx.Cfg != nil {
		if ctx.Cfg.Source.IsServeSourceEnabled() {
			remoteURL := ctx.Cfg.Source.GetRemoteSource()
			if remoteURL != "" {
				activeSources = append(activeSources, "远程 HTTP 源")
				hasServe = true
			}
		}
		// 仅在未启用 serve 源时才直接查询 Firefox FTP
		if !hasServe && ctx.Cfg.Source.IsFirefoxFTPEnabled() {
			if spec.Browser == "firefox" {
				activeSources = append(activeSources, "Mozilla FTP 目录")
			}
		}
	} else {
		// 无配置时默认显示所有
		activeSources = append(activeSources, "远程 HTTP 源")
		if spec.Browser == "firefox" {
			activeSources = append(activeSources, "Mozilla FTP 目录")
		}
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[list] 远程模式: browser=%s version=%s channels=%v activeSources=%v",
			spec.Browser, spec.Version, channels, activeSources)
	}

	if len(activeSources) == 0 {
		ctx.Printf("没有为 %s 配置可用的远程源。\n", spec.Browser)
		if ctx.Cfg != nil {
			if spec.Browser == "firefox" && !ctx.Cfg.Source.IsFirefoxFTPEnabled() {
				ctx.Println("提示: Firefox 源已禁用，使用 'bws cfg set source-firefox-ftp true' 启用。")
			}
			if !ctx.Cfg.Source.IsServeSourceEnabled() {
				ctx.Println("提示: Serve 源已禁用，使用 'bws cfg set source-serve true' 启用。")
			}
		}
		return nil
	}

	// 打印交互过程
	ctx.Printf("正在从 %s 请求 %s 版本列表...\n", strings.Join(activeSources, "、"), spec.Browser)
	if hasServe && ctx.Cfg != nil {
		remoteURL := ctx.Cfg.Source.GetRemoteSource()
		if remoteURL != "" {
			ctx.Printf("  远程源地址: %s\n", remoteURL)
		}
	}
	ctx.Printf("  查询渠道: %s\n", strings.Join(channels, ", "))
	if spec.Version != "" && !spec.IsAlias {
		ctx.Printf("  版本筛选: %s\n", spec.Version)
	}
	ctx.Println()

	// 获取本地已安装版本（含系统安装），用于标记
	installedMap := make(map[string]bool)
	if ctx.Install != nil {
		installed, _ := ctx.Install.ListWithSystemByBrowser(spec.Browser)
		for _, v := range installed {
			installedMap[v.Version] = true
		}
	}

	// 版本前缀（非别名时传递给源进行服务端过滤）
	versionPrefix := ""
	if spec.Version != "" && !spec.IsAlias {
		versionPrefix = spec.Version
	}

	// 并行查询所有渠道
	type channelResult struct {
		channel string
		versions []source.VersionInfo
		err      error
	}

	results := make([]channelResult, len(channels))
	var wg sync.WaitGroup

	for i, ch := range channels {
		wg.Add(1)
		go func(idx int, ch string) {
			defer wg.Done()
			if ctx.Logger != nil {
				ctx.Logger.Debug("[list] 向源查询: source=%s browser=%s channel=%s versionPrefix=%s platform=%s arch=%s",
					ctx.Source.Describe(), spec.Browser, ch, versionPrefix, source.CurrentPlatform(), source.CurrentArch())
			}
			versions, err := ctx.Source.ListVersions(spec.Browser, ch, versionPrefix)
			results[idx] = channelResult{channel: ch, versions: versions, err: err}
		}(i, ch)
	}

	wg.Wait()

	// 按渠道顺序处理结果
	rows := [][]string{}
	totalShown := 0
	installedCount := 0
	totalResults := 0
	channelResultCount := make(map[string]int) // channel -> count

	for _, r := range results {
		ch := r.channel
		if r.err != nil {
			ctx.Printf("  %s 渠道: 查询失败 (%v)\n", ch, r.err)
			if ctx.Logger != nil {
				ctx.Logger.Debug("[list] 源调用失败: channel=%s error=%v", ch, r.err)
			}
			continue
		}

		if len(r.versions) == 0 {
			ctx.Printf("  %s 渠道: 无结果\n", ch)
			if ctx.Logger != nil {
				ctx.Logger.Debug("[list] 源调用成功: channel=%s 返回 0 个版本", ch)
			}
			continue
		}

		if ctx.Logger != nil {
			ctx.Logger.Debug("[list] 源调用成功: channel=%s 返回 %d 个版本", ch, len(r.versions))
			for _, v := range r.versions {
				ctx.Logger.Debug("[list]   源返回: %s | url=%s | size=%d | platform=%s | arch=%s",
					v.Version, v.DownloadURL, v.Size, v.Platform, v.Arch)
			}
		}

		// 按版本前缀筛选（双重保障：源可能未执行过滤）
		filtered := r.versions
		if versionPrefix != "" {
			filtered = nil
			for _, v := range r.versions {
				if matchesVersionPrefix(v.Version, versionPrefix) {
					filtered = append(filtered, v)
				}
			}
		}

		if len(filtered) == 0 {
			ctx.Printf("  %s 渠道: 无匹配版本\n", ch)
			if ctx.Logger != nil {
				ctx.Logger.Debug("[list] 版本前缀筛选后无匹配: channel=%s 原始 %d 个", ch, len(r.versions))
			}
			continue
		}

		if ctx.Logger != nil {
			ctx.Logger.Debug("[list] 版本前缀筛选: 从 %d 个匹配到 %d 个", len(r.versions), len(filtered))
		}

		channelResultCount[ch] = len(filtered)
		ctx.Printf("  %s 渠道: 找到 %d 个版本\n", ch, len(filtered))
		totalResults += len(filtered)

		// 限制每个渠道显示的版本数量
		showCount := len(filtered)
		if !showAll {
			showCount = min(limit, len(filtered))
		}

		for i := 0; i < showCount; i++ {
			v := filtered[i]
			status := "—"
			if installedMap[v.Version] {
				status = "已安装"
				installedCount++
			}
			if ctx.Logger != nil {
				ctx.Logger.Debug("[list] 命中版本: %s | 下载地址: %s | 大小: %d | 平台: %s | 架构: %s | 状态: %s",
					v.Version, v.DownloadURL, v.Size, v.Platform, v.Arch, status)
			}
			rows = append(rows, []string{v.Version, ch, string(v.Platform), string(v.Arch), status})
			totalShown++
		}
	}

	// 分隔线
	ctx.Println()
	ctx.Println("----------------------------------------")
	ctx.Println()

	if ctx.Logger != nil {
		ctx.Logger.Debug("[list] 远程查询完成: 共 %d 个匹配版本，%d 个渠道有结果", totalResults, len(channelResultCount))
	}

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

// --- Grouped version display ---

// majorVersionGroup holds versions grouped by their major version number.
type majorVersionGroup struct {
	Major     string
	Versions  []string
	Channel   string
	Platform  string
	Arch      string
	Installed bool
	Count     int
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
