package cli

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
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
	app.AddCommand(NewPluginCommand())
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
			{Name: "plugin", Short: "", Usage: "激活的插件（逗号分隔多个）", HasValue: true, Default: ""},
		},
		Run: runRun,
	}
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
			"i chrome@120 --refresh-cache",
		},
		Flags: []*Flag{
			{Name: "from-dir", Short: "d", Usage: "从本地目录安装", HasValue: true, Default: ""},
			{Name: "from-file", Short: "", Usage: "从本地压缩包安装", HasValue: true, Default: ""},
			{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
			{Name: "channel", Short: "c", Usage: "发布渠道（stable, beta, dev, canary）", HasValue: true, Default: "stable"},
			{Name: "refresh-cache", Short: "", Usage: "强制从 serve 重新下载（忽略本地缓存）", HasValue: false, Default: "false"},
		},
		Run: runInstall,
	}
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

// configKeyInfo describes a configuration key for display.
type configKeyInfo struct {
	Keys        []string
	Description string
	Example     string
}

// readableConfigKeys returns all readable config keys with descriptions.
func readableConfigKeys() []configKeyInfo {
	return []configKeyInfo{
		{[]string{"default-browser", "default", "browser"}, "默认浏览器", "chrome"},
		{[]string{"default-channel", "channel"}, "默认渠道", "stable"},
		{[]string{"log-level", "log"}, "日志级别", "debug / info / warn / error"},
		{[]string{"data-dir", "datadir", "data"}, "数据目录", "/path/to/data"},
		{[]string{"repo-path", "repo"}, "仓库路径", "/path/to/repo"},
		{[]string{"source", "remote-source", "remote"}, "离线源 URL", "http://192.168.1.1:8080"},
		{[]string{"source-serve", "serve-source"}, "Serve 源开关", "true / false"},
		{[]string{"source-firefox-ftp", "firefox-ftp"}, "Firefox FTP 源开关", "true / false"},
		{[]string{"disk-threshold", "disk-space-threshold", "space-threshold"}, "磁盘空间阈值 (GB)", "5"},
		{[]string{"proxy"}, "代理服务器", "http://proxy:8080 或 none"},
		{[]string{"language", "lang"}, "界面语言", "zh / en"},
		{[]string{"path", "config-path", "config"}, "配置文件路径", "(只读)"},
	}
}

// writableConfigKeys returns all writable config keys with descriptions.
func writableConfigKeys() []configKeyInfo {
	return []configKeyInfo{
		{[]string{"default-browser", "default", "browser"}, "默认浏览器", "chrome"},
		{[]string{"default-channel", "channel"}, "默认渠道", "stable / beta / dev / canary"},
		{[]string{"log-level", "log"}, "日志级别", "debug / info / warn / error"},
		{[]string{"data-dir", "datadir", "data"}, "数据目录", "/path/to/data"},
		{[]string{"repo-path", "repo"}, "仓库路径", "/path/to/repo"},
		{[]string{"source", "remote-source", "remote"}, "离线源 URL", "http://192.168.1.1:8080"},
		{[]string{"source-serve", "serve-source"}, "Serve 源开关", "true / false"},
		{[]string{"source-firefox-ftp", "firefox-ftp"}, "Firefox FTP 源开关", "true / false"},
		{[]string{"disk-threshold", "disk-space-threshold", "space-threshold"}, "磁盘空间阈值 (GB)", "5"},
		{[]string{"proxy"}, "代理服务器", "http://proxy:8080 或 none 清除"},
		{[]string{"language", "lang"}, "界面语言", "zh / en"},
	}
}

func printConfigKeyList(w io.Writer, keys []configKeyInfo, forSet bool) {
	fmt.Fprintln(w, "可用配置项:")
	fmt.Fprintln(w)
	maxLen := 0
	for _, k := range keys {
		name := k.Keys[0]
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	for _, k := range keys {
		primary := k.Keys[0]
		aliases := ""
		if len(k.Keys) > 1 {
			aliases = fmt.Sprintf(" (别名: %s)", strings.Join(k.Keys[1:], ", "))
		}
		fmt.Fprintf(w, "  %-*s  %s%s\n", maxLen, primary, k.Description, aliases)
		if forSet && k.Example != "" && k.Example != "(只读)" {
			fmt.Fprintf(w, "  %s  示例: %s\n", strings.Repeat(" ", maxLen), k.Example)
		}
	}
	fmt.Fprintln(w)
	if forSet {
		fmt.Fprintln(w, "用法: bws cfg set <键名> <值>")
	} else {
		fmt.Fprintln(w, "用法: bws cfg get <键名>")
	}
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

func NewConfigPathCommand() *Command {
	return &Command{
		Name:        "path",
		Description: "显示配置文件路径",
		Run: func(ctx *Context, args []string) error {
			ctx.Println(ctx.Cfg.Data.ConfigPath())
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

func NewCacheInfoCommand() *Command {
	return &Command{
		Name:        "info",
		Description: "显示缓存信息",
		Run:         runCacheInfo,
	}
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

// resolveSpec parses input and resolves browser name aliases.
// This is the canonical entry point shared by all commands.
func resolveSpec(ctx *Context, input string, defaultBrowser string) browserVersionSpec {
	spec := parseBrowserVersion(input, defaultBrowser)
	return resolveBrowserSpec(ctx, spec)
}

// resolveFirstArg resolves the first positional argument as a browser@version spec.
// Uses the configured default browser when no browser name is given.
func resolveFirstArg(ctx *Context, args []string) browserVersionSpec {
	if len(args) > 0 {
		return resolveSpec(ctx, args[0], ctx.Cfg.Defaults.DefaultBrowser())
	}
	return browserVersionSpec{Browser: ctx.Cfg.Defaults.DefaultBrowser(), Version: "latest", IsAlias: true}
}

// checkFeature returns an error if the given provider is nil, indicating the build doesn't support this feature.
func checkFeature(provider interface{}, name string) error {
	if provider == nil {
		return fmt.Errorf("当前构建不支持%s功能", name)
	}
	return nil
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

// parsePluginList parses a comma-separated plugin list into a slice.
func parsePluginList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
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
	if ctx.Cfg != nil {
		thresholdGB = ctx.Cfg.Disk.GetDiskSpaceThresholdGB()
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

// getDownloadFilename generates a proper filename for a download.
// It first tries to extract the filename from the URL path.
// If the URL path doesn't contain a meaningful filename (e.g., query-based URLs),
// it generates a filename based on browser, version, and platform.
func getDownloadFilename(browser, version, downloadURL, platform string) string {
	// Try to extract filename from URL path first
	if u, err := url.Parse(downloadURL); err == nil {
		if base := filepath.Base(u.Path); base != "" && base != "/" && base != "\\" && base != "." {
			return base
		}
	}

	// Fallback: generate filename with proper extension based on platform
	base := fmt.Sprintf("%s-%s", browser, version)

	switch strings.ToLower(browser) {
	case "firefox":
		switch strings.ToLower(platform) {
		case "windows":
			return base + ".exe"
		case "darwin", "macos", "mac":
			return base + ".dmg"
		case "linux":
			return base + ".tar.bz2"
		}
	case "chrome", "chromium":
		switch strings.ToLower(platform) {
		case "windows":
			return base + ".exe"
		case "darwin", "macos", "mac":
			return base + ".dmg"
		case "linux":
			return base + ".zip"
		}
	}

	// Generic fallback with .zip extension (most common archive format)
	return base + ".zip"
}
