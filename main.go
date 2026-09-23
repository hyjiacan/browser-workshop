//	bws - Browser Manager
//
// Usage:
//
//	bws <command> [options]
package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/cli"
	"github.com/bws/bws/internal/config"
	"github.com/bws/bws/internal/download"
	"github.com/bws/bws/internal/fingerprint"
	"github.com/bws/bws/internal/i18n"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/launch"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/plugin"
	"github.com/bws/bws/internal/repo"
	bmserve "github.com/bws/bws/internal/serve"
	"github.com/bws/bws/internal/shortcut"
	"github.com/bws/bws/internal/source"
	"github.com/bws/bws/internal/system"
	bwversion "github.com/bws/bws/internal/version"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
)

const version = "1.0.0-beta"

func main() {
	bwversion.ClientVersion = version
	// Parse global flags before command processing
	verbose := parseGlobalVerbose()

	// Determine config path (portable mode: config in exe directory)
	configPath, dataRoot, isNewConfig := resolvePaths()

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// If config file doesn't exist yet, run first-time setup (skip for serve command)
	if isNewConfig && !(len(os.Args) > 1 && os.Args[1] == "serve") {
		cfg = firstTimeSetup(configPath, cfg)
	}

	// Determine data directory
	dataDir := dataRoot
	if cfg.DataDir != "" {
		dataDir = cfg.DataDir
	}

	// Initialize i18n (must be before any CLI output, needs dataDir for external override path)
	i18n.Init(cfg.GetLanguage(), filepath.Join(dataDir, "i18n"))

	// Initialize paths
	p := paths.New(dataDir)
	if err := p.EnsureAll(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化目录失败: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger (dual output: file and console with separate levels)
	fileLevel := bmlog.ParseLevel(cfg.Log.FileLevel)
	consoleLevel := bmlog.ParseLevel(cfg.Log.ConsoleLevel)
	if verbose {
		// -V 模式下，控制台日志级别与文件日志级别一致，输出所有日志
		consoleLevel = fileLevel
	}
	maxSizeBytes := int64(cfg.Log.MaxSizeMB) * 1024 * 1024
	logger, err := bmlog.NewDualLogger(p.LogFile, fileLevel, consoleLevel, true,
		bmlog.WithMaxSize(maxSizeBytes),
		bmlog.WithMaxBackups(cfg.Log.MaxBackups),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 初始化日志系统失败: %v\n", err)
		logger = bmlog.Default()
	}
	defer logger.Close()

	// Replace default logger with our configured logger
	// so package-level log functions use the same output
	bmlog.SetDefault(logger)

	fmt.Printf("bws starting (version %s)\n", version)
	if verbose {
		fmt.Fprintln(os.Stderr, "[verbose] 调试输出已开启")
	}
	fmt.Println("------------------------------")

	// Create managers
	inst := install.NewManager(p, browser.DefaultRegistry)
	launcher := launch.NewManager(p, browser.DefaultRegistry, inst)
	proxyURL := cfg.GetProxy()
	downloadMgr := download.NewManagerWithProxy(proxyURL)
	pluginMgr, err := plugin.NewManager(p.PluginsDir)
	if err != nil {
		logger.Error("初始化插件管理器失败: %v", err)
		pluginMgr = nil
	}

	// System browser detection
	sysDetector := system.NewDetector(browser.DefaultRegistry)
	inst.AttachSystem(sysDetector)

	// 数据源配置
	// - serve 源启用时：客户端仅通过 HTTPSource 访问 serve（manifest），
	//   在线源仅用于 serve 内部（sync、online-fallback），客户端不直接访问
	// - serve 源未启用时：客户端直接访问在线源（Firefox FTP）
	var clientSources []source.Source
	var onlineSources []source.Source // serve 专用：仅在线源，不含 HTTPSource（避免自引用死锁）

	serveEnabled := cfg.IsServeSourceEnabled() && cfg.RemoteSource != ""

	// 1. HTTPSource（serve 离线源）——仅客户端使用
	if serveEnabled {
		clientSources = append(clientSources, source.NewHTTPSourceWithProxy(cfg.RemoteSource, proxyURL))
		logger.Info("serve 源已启用，客户端将通过 serve 查询版本（地址: %s）", cfg.RemoteSource)
	}

	// 2. Firefox FTP 在线源
	var firefoxSource *source.FirefoxSource
	if cfg.IsFirefoxFTPEnabled() {
		firefoxSource = source.NewFirefoxSourceWithProxy(proxyURL)
		onlineSources = append(onlineSources, firefoxSource) // serve 始终需要在线源
		if !serveEnabled {
			clientSources = append(clientSources, firefoxSource) // 无 serve 时客户端直接访问
		}
	}

	sourceMgr := source.NewMultiSource(clientSources...)
	onlineSourceMgr := source.NewMultiSource(onlineSources...)
	onlineSourceMgr.SetQuiet(true) // serve 端静默 [source] 层日志，由 [online] 层统一记录

	// Repository scanner and importer
	var repoScanner *repo.Scanner
	var repoImporter *repo.Importer
	if cfg.RepoPath != "" {
		repoScanner, err = repo.NewScanner(cfg.RepoPath, browser.DefaultRegistry)
		if err != nil {
			logger.Warn("创建仓库扫描器失败: %v", err)
		} else {
			repoImporter = repo.NewImporter(repoScanner, inst)
		}
	}

	// Create CLI context
	ctx := cli.DefaultContext()
	ctx.Paths = &pathsAdapter{p: p}
	ca := &configAdapter{cfg: cfg, configPath: configPath, dataDir: dataDir}
	ctx.Cfg = &cli.Settings{
		Aliases:  ca,
		Repo:     ca,
		Defaults: ca,
		Data:     ca,
		Proxy:    ca,
		Source:   ca,
		Disk:     ca,
		Log:      ca,
		Language: ca,
	}
	ctx.Browsers = &browserAdapter{reg: browser.DefaultRegistry}

	// Create a shared scanner for import functionality (always available)
	sharedScanner, err := repo.NewScanner("", browser.DefaultRegistry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 创建扫描器失败: %v\n", err)
	}
	ctx.Install = &installAdapter{mgr: inst, scanner: sharedScanner}
	ctx.Profile = &profileAdapter{mgr: inst}

	pluginExec := &pluginExecutor{mgr: pluginMgr}
	ctx.Launch = &launchAdapter{mgr: launcher, pluginExec: pluginExec}
	ctx.Download = &downloadAdapter{mgr: downloadMgr, paths: p}
	ctx.Source = &sourceAdapter{src: sourceMgr, cfg: cfg}
	ctx.Shortcut = &shortcutAdapter{}
	ctx.Serve = &serveAdapter{version: version, source: onlineSourceMgr, firefoxSrc: firefoxSource, verbose: verbose}
	ctx.Plugin = &pluginAdapter{mgr: pluginMgr}
	ctx.Logger = logger
	if repoImporter != nil {
		ctx.Repo = &repoAdapter{scanner: repoScanner, importer: repoImporter}
	}

	// Create app
	app := cli.NewApp("bws", version, ctx)
	cli.RegisterCommands(app)

	if err := app.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// resolvePaths determines the config file path and data root directory.
// The config file (bws-client.ini) is always located in the executable's directory.
// The data root directory defaults to bws-data/ next to the executable,
// but can be overridden via BM_HOME environment variable.
func resolvePaths() (configPath string, dataRoot string, isNew bool) {
	exeDir, err := paths.ExeDir()
	if err != nil {
		wd, _ := os.Getwd()
		exeDir = wd
	}

	// Config file is always in the executable directory
	configPath = filepath.Join(exeDir, "bws-client.ini")

	// Data directory: BM_HOME env var, or bws-data/ next to exe
	if home := os.Getenv("BM_HOME"); home != "" {
		dataRoot = home
	} else {
		dataRoot = filepath.Join(exeDir, "bws-data")
	}

	// Check if config exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, dataRoot, false
	}

	return configPath, dataRoot, true
}

// parseGlobalVerbose checks os.Args for --verbose / -V and removes all of them.
// Returns true if verbose mode is enabled.
func parseGlobalVerbose() bool {
	verbose := false
	keep := make([]string, 0, len(os.Args))
	for _, arg := range os.Args {
		if arg == "--verbose" || arg == "-V" {
			verbose = true
			continue
		}
		keep = append(keep, arg)
	}
	os.Args = keep
	return verbose
}

// firstTimeSetup runs the first-time setup wizard.
// Every input/selection step carries a short explanation so users can
// understand each option before deciding. Non-interactive environments
// (pipes/EOF) fall back to defaults.
func firstTimeSetup(configPath string, cfg *config.Config) *config.Config {
	fmt.Println("========================================")
	fmt.Println("  bws - Browser Manager")
	fmt.Println("  首次初始化")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("这是 bws 首次运行，将引导你完成基本配置。")
	fmt.Println("每一步都有说明，直接回车即可使用默认值 (推荐)。")
	fmt.Println()

	defaultDataDir := filepath.Join(filepath.Dir(configPath), "bws-data")

	// 非交互环境 (管道/脚本/EOF)：跳过向导，使用默认配置
	if !term.IsTerminal(os.Stdin.Fd()) {
		fmt.Println("检测到非交互环境，使用默认配置。")
		cfg.DataDir = defaultDataDir
		return saveInitialConfig(configPath, cfg)
	}

	// 构建"默认浏览器"选项：以内置浏览器为主，若当前默认值不在其中则补充
	knownBrowser := map[string]bool{"chrome": true, "chromium": true, "firefox": true}
	defaultBrowser := cfg.DefaultBrowser
	browserOpts := []huh.Option[string]{
		huh.NewOption("Chrome", "chrome"),
		huh.NewOption("Chromium", "chromium"),
		huh.NewOption("Firefox", "firefox"),
	}
	if !knownBrowser[defaultBrowser] {
		browserOpts = append([]huh.Option[string]{huh.NewOption(defaultBrowser, defaultBrowser)}, browserOpts...)
	}

	// 步骤一：数据目录 / 默认浏览器 / 是否配置离线源
	var serveEnabled bool
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("数据目录").
			Description("浏览器版本、下载缓存、Profile 与日志都会存放在这里。\n留空则使用默认值 (推荐)。").
			Prompt("> ").
			Placeholder(defaultDataDir).
			Value(&cfg.DataDir),
		huh.NewSelect[string]().
			Title("默认浏览器").
			Description("执行 `bws r` 且不带版本时，将使用该浏览器。\n用 ↑↓ 选择，回车确认。可通过 `bws config set default-browser` 修改。").
			Options(browserOpts...).
			Value(&defaultBrowser),
		huh.NewConfirm().
			Title("是否配置离线源 (serve)？").
			Description("配置离线服务器后，可无需连接公网即可获取浏览器版本，\n适合内网分发。首次使用建议配置。").
			Affirmative("配置").
			Negative("跳过"),
	)).WithTheme(huh.ThemeCharm()).Run()
	if err != nil {
		// 用户取消 (q/Ctrl+C)：用默认配置继续，避免下次运行重复触发向导
		fmt.Println("已取消初始化，使用默认配置。")
		cfg.DataDir = defaultDataDir
		return saveInitialConfig(configPath, cfg)
	}

	cfg.DefaultBrowser = defaultBrowser
	if strings.TrimSpace(cfg.DataDir) == "" {
		cfg.DataDir = defaultDataDir
	} else if abs, aerr := filepath.Abs(cfg.DataDir); aerr == nil {
		cfg.DataDir = abs
	}

	// 步骤二：若选择配置离线源，进一步填写地址
	if serveEnabled {
		var serveURL string
		err = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("离线源 (serve) 地址").
				Description("填写 serve 服务器地址，形如 http://192.168.1.100:8080。\n即网页端顶部展示的连接地址，可复制粘贴。留空则跳过。").
				Prompt("> ").
				Placeholder("http://<服务器IP>:<端口>").
				Value(&serveURL),
		)).WithTheme(huh.ThemeCharm()).Run()
		if err == nil && strings.TrimSpace(serveURL) != "" {
			cfg.RemoteSource = strings.TrimSpace(serveURL)
			cfg.EnableServeSource = true
			fmt.Printf("  • 已配置离线源: %s\n", cfg.RemoteSource)
		}
		// 跳过或未填地址时不改动 EnableServeSource，保留配置默认值，
		// 避免后续 `bws config set source` 因开关被关而无法启用离线源
	}

	return saveInitialConfig(configPath, cfg)
}

// saveInitialConfig creates the config directory (if needed) and persists cfg.
func saveInitialConfig(configPath string, cfg *config.Config) *config.Config {
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 创建配置目录失败: %v\n", err)
	}
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 保存配置失败: %v\n", err)
	}

	fmt.Println()
	fmt.Println("配置已保存:", configPath)
	fmt.Println("初始化完成! 使用 'bws --help' 查看可用命令。")
	fmt.Println()
	fmt.Println("后续如需修改配置，可使用以下命令：")
	fmt.Println("  bws config show                       查看当前配置")
	fmt.Println("  bws config set source <地址>          设置/修改离线源")
	fmt.Println("  bws config set default-browser <名称> 修改默认浏览器")
	fmt.Println("  bws config set data-dir <路径>        修改数据目录")
	fmt.Println()

	return cfg
}

// buildRemoteSources creates HTTPSource instances from the given URLs.
// --- Adapters to wire internal packages to CLI interfaces ---

type pathsAdapter struct {
	p *paths.Paths
}

func (a *pathsAdapter) VersionDir(browser string, version string) string {
	return a.p.VersionDir(browser, version)
}

func (a *pathsAdapter) DownloadCacheDir() string {
	return a.p.DownloadCacheDir
}

func (a *pathsAdapter) EnsureAll() error {
	return a.p.EnsureAll()
}

type configAdapter struct {
	cfg        *config.Config
	configPath string
	dataDir    string
}

func (a *configAdapter) DefaultBrowser() string {
	return a.cfg.DefaultBrowser
}

func (a *configAdapter) SetDefaultBrowser(browser string) error {
	a.cfg.DefaultBrowser = browser
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) DefaultVersion() string {
	return a.cfg.DefaultVersion
}

func (a *configAdapter) SetDefaultVersion(version string) error {
	a.cfg.DefaultVersion = version
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) DefaultChannel() string {
	return a.cfg.DefaultChannel
}

func (a *configAdapter) SetDefaultChannel(channel string) error {
	a.cfg.DefaultChannel = channel
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetLogLevel() string {
	return a.cfg.Log.ConsoleLevel
}

func (a *configAdapter) SetLogLevel(level string) error {
	a.cfg.Log.ConsoleLevel = level
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetDataDir() string {
	if a.cfg.DataDir != "" {
		return a.cfg.DataDir
	}
	return a.dataDir
}

func (a *configAdapter) SetDataDir(path string) error {
	a.cfg.DataDir = path
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) ConfigPath() string {
	return a.configPath
}

func (a *configAdapter) GetAlias(name string) (string, bool) {
	return a.cfg.GetAlias(name)
}

func (a *configAdapter) AddAlias(name, target string) error {
	a.cfg.AddAlias(name, target)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) RemoveAlias(name string) error {
	a.cfg.RemoveAlias(name)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) ListAliases() map[string]string {
	return a.cfg.ListAliases()
}

func (a *configAdapter) GetRepoPath() string {
	return a.cfg.RepoPath
}

func (a *configAdapter) SetRepoPath(path string) error {
	a.cfg.SetRepoPath(path)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetRemoteSource() string {
	return a.cfg.GetRemoteSource()
}

func (a *configAdapter) SetRemoteSource(url string) error {
	a.cfg.SetRemoteSource(url)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) ClearRemoteSource() error {
	a.cfg.ClearRemoteSource()
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) IsServeSourceEnabled() bool { return a.cfg.IsServeSourceEnabled() }
func (a *configAdapter) SetServeSourceEnabled(v bool) error {
	a.cfg.SetServeSourceEnabled(v)
	return config.Save(a.cfg, a.configPath)
}
func (a *configAdapter) IsFirefoxFTPEnabled() bool { return a.cfg.IsFirefoxFTPEnabled() }
func (a *configAdapter) SetFirefoxFTPEnabled(v bool) error {
	a.cfg.SetFirefoxFTPEnabled(v)
	return config.Save(a.cfg, a.configPath)
}
func (a *configAdapter) GetDiskSpaceThresholdGB() int { return a.cfg.GetDiskSpaceThresholdGB() }
func (a *configAdapter) SetDiskSpaceThresholdGB(v int) error {
	a.cfg.SetDiskSpaceThresholdGB(v)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetProxy() string { return a.cfg.GetProxy() }
func (a *configAdapter) SetProxy(proxy string) error {
	a.cfg.SetProxy(proxy)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetLanguage() string { return a.cfg.GetLanguage() }
func (a *configAdapter) SetLanguage(lang string) error {
	a.cfg.SetLanguage(lang)
	return config.Save(a.cfg, a.configPath)
}

type pluginAdapter struct {
	mgr *plugin.Manager
}

func (a *pluginAdapter) List() []plugin.ManifestEntry { return a.mgr.List() }
func (a *pluginAdapter) GetManifestEntry(name string) (*plugin.ManifestEntry, error) {
	return a.mgr.GetManifestEntry(name)
}
func (a *pluginAdapter) Install(entry plugin.ManifestEntry) error { return a.mgr.Install(entry) }
func (a *pluginAdapter) Uninstall(name string) error              { return a.mgr.Uninstall(name) }
func (a *pluginAdapter) PluginsDir() string                       { return a.mgr.PluginsDir() }

type pluginExecutor struct {
	mgr *plugin.Manager
}

func (e *pluginExecutor) RunPreRunPlugins(opts *launch.Options) error {
	if len(opts.Plugins) == 0 {
		return nil
	}
	for _, name := range opts.Plugins {
		// Look up plugin type from manifest
		entry, err := e.mgr.GetManifestEntry(name)
		if err != nil {
			// Fallback: try as .lua plugin
			pluginPath := filepath.Join(e.mgr.PluginsDir(), name+".lua")
			if _, statErr := os.Stat(pluginPath); statErr == nil {
				entry = &plugin.ManifestEntry{Name: name, Type: "lua", Path: pluginPath}
			} else {
				bmlog.Warn("插件 %q 未找到 (不在清单中，也没有 .lua 文件)，已跳过", name)
				continue
			}
		}

		// Build shared context
		ctx := &plugin.ScriptContext{
			Browser:    opts.Browser,
			Version:    opts.Version,
			Profile:    opts.ProfileName,
			ProfileDir: opts.ProfileDir,
			AddArg: func(arg string) {
				opts.ExtraArgs = append(opts.ExtraArgs, arg)
			},
			SetEnv: func(k, v string) {
				if opts.Env == nil {
					opts.Env = make(map[string]string)
				}
				opts.Env[k] = v
			},
		}

		switch entry.Type {
		case "lua":
			if err := e.runLuaPlugin(entry.Path, ctx); err != nil {
				bmlog.Warn("Lua 插件 %q 执行失败: %v，已跳过", name, err)
				continue
			}
		case "binary":
			resp, err := plugin.RunIPCPlugin(entry.Path, ctx)
			if err != nil {
				bmlog.Warn("IPC 插件 %q 执行失败: %v，已跳过", name, err)
				continue
			}
			opts.ExtraArgs = append(opts.ExtraArgs, resp.ExtraArgs...)
			for k, v := range resp.Env {
				if opts.Env == nil {
					opts.Env = make(map[string]string)
				}
				opts.Env[k] = v
			}
		default:
			bmlog.Warn("插件 %q: 未知类型 %q，已跳过", name, entry.Type)
			continue
		}
	}
	return nil
}

func (e *pluginExecutor) runLuaPlugin(pluginPath string, ctx *plugin.ScriptContext) error {
	rt := plugin.NewLuaRuntime()
	defer rt.Close()
	return rt.RunScript(pluginPath, ctx)
}

type browserAdapter struct {
	reg *browser.Registry
}

func (a *browserAdapter) Get(name string) cli.BrowserDescriptor {
	desc := a.reg.Get(name)
	if desc == nil {
		return cli.BrowserDescriptor{}
	}
	return cli.BrowserDescriptor{
		Name:        desc.Name,
		DisplayName: desc.DisplayName,
	}
}

func (a *browserAdapter) List() []cli.BrowserDescriptor {
	descs := a.reg.List()
	result := make([]cli.BrowserDescriptor, len(descs))
	for i, d := range descs {
		result[i] = cli.BrowserDescriptor{
			Name:        d.Name,
			DisplayName: d.DisplayName,
		}
	}
	return result
}

func (a *browserAdapter) Has(name string) bool {
	return a.reg.Has(name)
}

func (a *browserAdapter) ResolveName(name string) (string, bool) {
	return a.reg.ResolveName(name)
}

type installAdapter struct {
	mgr     *install.Manager
	scanner *repo.Scanner
}

func (a *installAdapter) IsInstalled(browser, version string) bool {
	return a.mgr.IsInstalled(browser, version)
}

func (a *installAdapter) ListInstalled() (bwversion.List, error) {
	return a.mgr.ListInstalled()
}

func (a *installAdapter) ListInstalledByBrowser(browser string) (bwversion.List, error) {
	return a.mgr.ListInstalledByBrowser(browser)
}

func (a *installAdapter) GetRecord(browser, version string) (*bwversion.InstallRecord, error) {
	return a.mgr.GetRecord(browser, version)
}

func (a *installAdapter) Uninstall(browser, version string) error {
	return a.mgr.Uninstall(browser, version)
}

func (a *installAdapter) InstallFromDir(browser, version, sourceDir string) (*bwversion.InstallRecord, error) {
	return a.mgr.InstallFromDir(install.InstallOptions{
		Browser:   browser,
		Version:   version,
		Source:    "cli",
		SourceDir: sourceDir,
	}, nil)
}

func (a *installAdapter) InstallFromFile(browser, version, filePath string) (*bwversion.InstallRecord, error) {
	return a.mgr.InstallFromFile(browser, version, filePath)
}

func (a *installAdapter) HasSystem() bool {
	return a.mgr.HasSystem()
}

func (a *installAdapter) ListWithSystem() (bwversion.List, error) {
	return a.mgr.ListWithSystem()
}

func (a *installAdapter) ListWithSystemByBrowser(browser string) (bwversion.List, error) {
	return a.mgr.ListWithSystemByBrowser(browser)
}

func (a *installAdapter) IsSystemVersion(browser, version string) bool {
	return a.mgr.IsSystemVersion(browser, version)
}

func (a *installAdapter) ResolveInstalledVersion(browser, version string) (string, error) {
	return a.mgr.ResolveInstalledVersion(browser, version)
}

func (a *installAdapter) ImportFromDir(dir string, force bool, onProgress func(current int, total int, message string)) (*cli.ImportSummary, error) {
	if a.scanner == nil {
		return nil, fmt.Errorf("scanner not available")
	}

	bmlog.Info("开始从目录导入: %s", dir)
	bmlog.Debug("强制模式: %v", force)

	// Scan the directory
	if onProgress != nil {
		onProgress(0, 0, fmt.Sprintf("正在扫描 %s...", dir))
	}
	bmlog.Debug("正在扫描目录中的浏览器版本...")
	matches, err := a.scanner.ScanRepository(dir, "", "")
	if err != nil {
		bmlog.Error("扫描目录失败: %v", err)
		return nil, fmt.Errorf("扫描目录: %w", err)
	}
	bmlog.Info("发现 %d 个条目待处理", len(matches))

	// Log match details at debug level
	for _, m := range matches {
		if m.Status == repo.MatchUnrecognized {
			bmlog.Warn("无法识别: %s", filepath.Base(m.Path))
		} else {
			bmlog.Debug("已匹配: %s -> %s@%s (arch=%s, platform=%s, status=%s, pattern=%s, detail=%s)",
				filepath.Base(m.Path), m.Browser, m.Version, m.Arch, m.Platform, m.Status, m.Pattern, m.Detail)
		}
	}

	summary := &cli.ImportSummary{Total: len(matches)}

	for i, m := range matches {
		idx := i + 1

		// Skip unrecognized
		if m.Status == repo.MatchUnrecognized {
			summary.FailedUnrecognized++
			summary.Failed++
			summary.Errors = append(summary.Errors, cli.ImportError{
				Path:  m.Path,
				Error: "无法识别",
			})
			bmlog.Warn("[%d/%d] 跳过 (无法识别): %s", idx, len(matches), filepath.Base(m.Path))
			if onProgress != nil {
				onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] 跳过 (无法识别): %s", idx, len(matches), filepath.Base(m.Path)))
			}
			continue
		}

		statusText := "导入中"
		if m.IsFile {
			statusText = "解压并导入中"
		}
		bmlog.Info("[%d/%d] %s %s@%s", idx, len(matches), statusText, m.Browser, m.Version)
		if onProgress != nil {
			onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] %s %s@%s", idx, len(matches), statusText, m.Browser, m.Version))
		}

		// Check if already installed
		if !force && a.mgr.IsInstalled(m.Browser, m.Version) {
			summary.SkippedAlreadyInstalled++
			summary.Skipped++
			bmlog.Info("[%d/%d] 跳过 (已安装): %s@%s", idx, len(matches), m.Browser, m.Version)
			if onProgress != nil {
				onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] 跳过 (已安装): %s@%s", idx, len(matches), m.Browser, m.Version))
			}
			continue
		}

		// Force uninstall if needed
		if force && a.mgr.IsInstalled(m.Browser, m.Version) {
			bmlog.Debug("[%d/%d] 卸载现有版本: %s@%s", idx, len(matches), m.Browser, m.Version)
			if err := a.mgr.Uninstall(m.Browser, m.Version); err != nil {
				summary.Failed++
				summary.Errors = append(summary.Errors, cli.ImportError{
					Path:    m.Path,
					Browser: m.Browser,
					Version: m.Version,
					Error:   fmt.Sprintf("卸载失败: %v", err),
				})
				bmlog.Error("[%d/%d] 失败 (卸载): %s@%s - %v", idx, len(matches), m.Browser, m.Version, err)
				if onProgress != nil {
					onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] 失败 (卸载): %s@%s - %v", idx, len(matches), m.Browser, m.Version, err))
				}
				continue
			}
		}

		// Install
		var installErr error
		if m.IsFile {
			bmlog.Debug("从文件安装: %s", m.Path)
			_, installErr = a.mgr.InstallFromFile(m.Browser, m.Version, m.Path)
		} else {
			bmlog.Debug("从目录安装: %s", m.Path)
			_, installErr = a.mgr.InstallFromDir(install.InstallOptions{
				Browser:   m.Browser,
				Version:   m.Version,
				Source:    "import",
				SourceDir: m.Path,
			}, nil)
		}

		if installErr != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, cli.ImportError{
				Path:    m.Path,
				Browser: m.Browser,
				Version: m.Version,
				Error:   installErr.Error(),
			})
			bmlog.Error("[%d/%d] 失败: %s@%s - %v", idx, len(matches), m.Browser, m.Version, installErr)
			if onProgress != nil {
				onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] 失败: %s@%s - %v", idx, len(matches), m.Browser, m.Version, installErr))
			}
		} else {
			summary.Success++
			bmlog.Info("[%d/%d] 已导入: %s@%s", idx, len(matches), m.Browser, m.Version)
			if onProgress != nil {
				onProgress(idx, len(matches), fmt.Sprintf("[%d/%d] 已导入: %s@%s", idx, len(matches), m.Browser, m.Version))
			}
		}
	}

	bmlog.Info("导入完成: 共 %d 个, 成功 %d, 失败 %d, 跳过 %d",
		summary.Total, summary.Success, summary.Failed, summary.Skipped)

	return summary, nil
}

type launchAdapter struct {
	mgr        *launch.Manager
	pluginExec *pluginExecutor
}

// profileAdapter adapts install.Manager to cli.ProfileProvider.
type profileAdapter struct {
	mgr *install.Manager
}

func (a *profileAdapter) ProfileDir(browser string, version string, profileName string) string {
	return a.mgr.ProfileDir(browser, version, profileName)
}

func (a *profileAdapter) ResetProfile(browser string, version string, profileName string) error {
	return a.mgr.ResetProfile(browser, version, profileName)
}

func (a *profileAdapter) ListProfiles(browser string) ([]install.ProfileInfo, error) {
	return a.mgr.ListProfiles(browser)
}

func (a *profileAdapter) CleanOrphanedProfiles(browser string) ([]string, error) {
	return a.mgr.CleanOrphanedProfiles(browser)
}

func (a *launchAdapter) Run(opts cli.LaunchOptions) error {
	launchOpts := launch.Options{
		Browser:     opts.Browser,
		Version:     opts.Version,
		URLs:        opts.URLs,
		Headless:    opts.Headless,
		Incognito:   opts.Incognito,
		NewWindow:   opts.NewWindow,
		ProfileName: opts.ProfileName,
		NativeMode:  opts.NativeMode,
		ExtraArgs:   opts.ExtraArgs,
		Detached:    opts.Detached,
		Proxy:       opts.Proxy,
	}

	// Parse fingerprint config
	if opts.Fingerprint != "" {
		fp, err := fingerprint.FromString(opts.Fingerprint)
		if err != nil {
			return fmt.Errorf("指纹配置解析失败: %w", err)
		}
		launchOpts.Fingerprint = fp
	}

	// Run pre-run plugins
	if a.pluginExec != nil {
		if err := a.pluginExec.RunPreRunPlugins(&launchOpts); err != nil {
			return err
		}
	}

	proc, err := a.mgr.Launch(launchOpts)
	if err != nil {
		return err
	}

	bmlog.Debug("[run] 进程已启动：PID=%d", proc.Pid)

	// If not detached, wait for the process
	if !opts.Detached {
		waitErr := proc.Wait()
		if waitErr != nil {
			// Try to extract exit code
			if exitErr, ok := waitErr.(interface{ ExitCode() int }); ok {
				bmlog.Debug("[run] 进程已退出：退出码=%d", exitErr.ExitCode())
			} else {
				bmlog.Debug("[run] 进程已退出：%v", waitErr)
			}
		} else {
			bmlog.Debug("[run] 进程已退出：退出码=0")
		}
		return waitErr
	}

	fmt.Printf("已启动 %s@%s (PID: %d)\n", opts.Browser, opts.Version, proc.Pid)
	return nil
}

func (a *launchAdapter) PreviewCommand(opts cli.LaunchOptions) (string, []string, error) {
	launchOpts := launch.Options{
		Browser:     opts.Browser,
		Version:     opts.Version,
		URLs:        opts.URLs,
		Headless:    opts.Headless,
		Incognito:   opts.Incognito,
		NewWindow:   opts.NewWindow,
		ProfileName: opts.ProfileName,
		NativeMode:  opts.NativeMode,
		ExtraArgs:   opts.ExtraArgs,
		Proxy:       opts.Proxy,
	}

	// Parse fingerprint config
	if opts.Fingerprint != "" {
		fp, err := fingerprint.FromString(opts.Fingerprint)
		if err != nil {
			return "", nil, fmt.Errorf("指纹配置解析失败: %w", err)
		}
		launchOpts.Fingerprint = fp
	}

	// Run pre-run plugins
	if a.pluginExec != nil {
		if err := a.pluginExec.RunPreRunPlugins(&launchOpts); err != nil {
			return "", nil, err
		}
	}

	return a.mgr.BuildCommandPreview(launchOpts)
}

// repoAdapter adapts repo.Scanner and repo.Importer to cli.RepoProvider.
type repoAdapter struct {
	scanner  *repo.Scanner
	importer *repo.Importer
}

func (a *repoAdapter) Scan() ([]repo.MatchResult, error) {
	return a.scanner.Scan()
}

func (a *repoAdapter) Import(force bool, onProgress func(int, int, string)) (*repo.ImportSummary, error) {
	var cb repo.ProgressCallback
	if onProgress != nil {
		cb = func(p repo.ImportProgress) {
			onProgress(p.Current, p.Total, p.Message)
		}
	}
	return a.importer.ImportAll(repo.ImportOptions{
		Force: force,
	}, cb)
}

// serveAdapter adapts serve.Server to cli.ServeProvider.
type serveAdapter struct {
	version    string
	source     source.Source        // the multi-source for syncing
	firefoxSrc *source.FirefoxSource // direct reference for GetChecksum (nil if Firefox source not enabled)
	verbose    bool
}

func (a *serveAdapter) StartFromConfig(baseDir string) error {
	// dataDir is the bws-data directory (shared with client config, logs, etc.)
	dataDir := paths.Default().Root

	// exeDir is the executable directory (used for config file location and
	// resolving relative packages/bin paths).
	// If --dir flag is provided, use it as the base directory for packages/bin instead.
	exeDir, err := paths.ExeDir()
	if err != nil {
		wd, _ := os.Getwd()
		exeDir = wd
	}
	packagesBaseDir := exeDir
	if baseDir != "" {
		abs, err := filepath.Abs(baseDir)
		if err == nil {
			packagesBaseDir = abs
		} else {
			packagesBaseDir = baseDir
		}
	}

	// Load serve config from bws-serve.ini (always in the executable directory)
	cfg, err := bmserve.LoadServeConfig("")
	if err != nil {
		return fmt.Errorf("加载 serve 配置失败: %w", err)
	}

	addr := cfg.Addr()

	// Resolve packages directory:
	// If config has an absolute path, use it directly.
	// If relative, resolve relative to the base directory.
	// If empty, default to {packagesBaseDir}/packages.
	packagesDir := resolveServeDir(cfg.PackagesDir, packagesBaseDir, "packages")

	// Resolve bin directory (same logic)
	binDir := resolveServeDir(cfg.BinDir, packagesBaseDir, "bin")

	// Create serve logger: dual output (file + console)
	// File log: at {dataDir}/logs/bws.log (与客户端模式共用同一日志文件)
	logFile := filepath.Join(dataDir, "logs", "bws.log")
	consoleLevel := bmlog.ParseLevel(cfg.LogLevel)
	fileLevel := bmlog.ParseLevel(cfg.FileLogLevel)
	if a.verbose {
		// -V 模式下，控制台日志级别与文件日志级别一致，输出所有日志
		consoleLevel = fileLevel
	}
	maxSizeBytes := int64(cfg.LogMaxSizeMB) * 1024 * 1024
	serveLogger, err := bmlog.NewDualLogger(logFile, fileLevel, consoleLevel, true,
		bmlog.WithMaxSize(maxSizeBytes),
		bmlog.WithMaxBackups(cfg.LogMaxBackups),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 初始化 serve 日志失败: %v\n", err)
		serveLogger = nil // fall back to default logger
	}
	if serveLogger != nil {
		defer serveLogger.Close()
	}

	// The online source adapter is reused for both auto-sync (when enabled)
	// and on-demand online fallback. serveSyncSource implements SyncSource.
	var onlineSource bmserve.SyncSource
	if a.source != nil {
		onlineSource = &serveSyncSource{src: a.source, firefoxSrc: a.firefoxSrc}
	}

	// Sync uses the user-configured browsers/channels.
	syncBrowsers := cfg.SyncBrowsersList()
	syncChannels := cfg.SyncChannelsList()

	// Online fallback always queries all channels so that ESR, beta, etc.
	// versions are discoverable even when sync-channels only lists "stable".
	onlineBrowsers := syncBrowsers

	if cfg.SyncEnabled && onlineSource != nil {
		// Parse interval
		interval, err := cfg.SyncDuration()
		if err != nil {
			return fmt.Errorf("解析同步间隔失败: %w", err)
		}

		srv := bmserve.NewServerWithOptions(bmserve.ServerOptions{
			Addr:        addr,
			Version:     a.version,
			PackagesDir: packagesDir,
			BinDir:      binDir,
			SyncSource:  onlineSource,
			SyncConfig: bmserve.SyncConfig{
				Enabled:  true,
				Interval: interval,
				Browsers: syncBrowsers,
				Channels: syncChannels,
			},
			OnlineSource:   onlineSource,
			OnlineFallback: cfg.OnlineFallback,
			OnlineBrowsers: onlineBrowsers,
			ScanWorkers:    cfg.ScanWorkers,
			ConfigPath:     bmserve.ConfigPath(""),
			AuthToken:      cfg.AuthToken,
			Logger:         serveLogger,
		})
		return srv.Start()
	}

	srv := bmserve.NewServerWithOptions(bmserve.ServerOptions{
		Addr:           addr,
		Version:        a.version,
		PackagesDir:    packagesDir,
		BinDir:         binDir,
		OnlineSource:   onlineSource,
		OnlineFallback: cfg.OnlineFallback,
		OnlineBrowsers: onlineBrowsers,
		ScanWorkers:    cfg.ScanWorkers,
		ConfigPath:     bmserve.ConfigPath(""),
		Logger:         serveLogger,
	})
	return srv.Start()
}

func (a *serveAdapter) ConfigPath() string {
	return bmserve.ConfigPath("")
}

func (a *serveAdapter) EnsureDefaultConfig(baseDir string) (string, bool, error) {
	return bmserve.EnsureDefaultConfig(baseDir)
}

// resolveServeDir resolves a directory path from serve config.
// If the path is absolute, it's used as-is.
// If relative, it's resolved relative to baseDir.
// If empty, it defaults to {baseDir}/{defaultName}.
func resolveServeDir(path string, baseDir string, defaultName string) string {
	if path == "" {
		return filepath.Join(baseDir, defaultName)
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

// serveSyncSource adapts source.Source to serve.SyncSource.
type serveSyncSource struct {
	src        source.Source
	firefoxSrc *source.FirefoxSource // direct reference for GetChecksum (nil if Firefox source not enabled)
}

func (s *serveSyncSource) ListVersions(ctx context.Context, browser string, channel string, platform string, arch string) ([]bmserve.SyncVersionInfo, error) {
	filter := &source.Filter{
		Browser: browser,
	}
	if channel != "" {
		filter.Channel = source.Channel(channel)
	}
	if platform != "" {
		filter.Platform = source.Platform(platform)
	}
	if arch != "" {
		filter.Arch = source.Arch(arch)
	}

	versions, err := s.src.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []bmserve.SyncVersionInfo
	for _, v := range versions {
		result = append(result, bmserve.SyncVersionInfo{
			Browser:     browser,
			Version:     v.Version,
			Channel:     string(v.Channel),
			Platform:    string(v.Platform),
			Arch:        string(v.Arch),
			DownloadURL: v.DownloadURL,
			Filename:    buildServeDownloadFilename(v.Version, string(v.Platform), string(v.Arch), v.DownloadURL),
			Size:        v.Size,
			Checksum:    v.Checksum,
		})
	}

	// Firefox 的 DownloadURL 已由 List() 中的 buildDownloadURL 模式构造完成，
	// 足以用于清单展示和在线回退下载。
	// 校验和在实际下载时由 doOnlineDownload 按需解析，
	// 避免对 2000+ 个版本逐个发起 HTTP 请求导致超时。

	return result, nil
}

func (s *serveSyncSource) Download(url string, destDir string, onProgress func(downloaded, total int64)) (string, error) {
	return bmserve.DefaultDownload(url, destDir, onProgress)
}

func (s *serveSyncSource) StreamDownload(ctx context.Context, url string) (io.ReadCloser, int64, error) {
	return bmserve.StreamDownload(ctx, url)
}

// buildServeDownloadFilename generates a filename that includes platform and
// architecture information, so that the serve scanner can correctly extract
// platform/arch metadata when scanning downloaded files.
//
// If the original URL filename already contains platform/arch keywords
// recognized by the scanner, it is returned unchanged.
// Otherwise, a suffix like "_win64", "_linux64", "_mac64" etc. is inserted
// before the file extension.
//
// Examples:
//   - "firefox-68.9.0esr.tar.bz2" + linux/amd64  → "firefox-68.9.0esr_linux64.tar.bz2"
//   - "chrome_120.0.6099.109_win64.zip"           → unchanged (already has win64)
func buildServeDownloadFilename(version, platform, arch, downloadURL string) string {
	// Extract the URL-decoded base filename from the download URL.
	base := filepath.Base(downloadURL)
	if decoded, err := urlDecode(base); err == nil && decoded != "" {
		base = decoded
	}

	// If the filename already contains platform keywords recognized by the
	// scanner, return it unchanged.
	if scannerPlatformKeywordInFilename(base) {
		return base
	}

	// Insert platform_arch before the extension.
	suffix := platformArchAbbreviation(platform, arch)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	return fmt.Sprintf("%s_%s%s", nameWithoutExt, suffix, ext)
}

// scannerPlatformKeywordInFilename checks whether the filename contains any
// platform keyword that the scanner's detectPlatform function can recognize.
// These are the keywords defined in internal/repo/scanner.go platformKeywords.
func scannerPlatformKeywordInFilename(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "windows") ||
		strings.Contains(lower, "win64") ||
		strings.Contains(lower, "win32") ||
		strings.Contains(lower, "linux64") ||
		strings.Contains(lower, "linux") ||
		strings.Contains(lower, "macos") ||
		strings.Contains(lower, "mac64") ||
		strings.Contains(lower, "macarm64")
}

// platformArchAbbreviation returns a scanner-compatible abbreviation for the
// given platform and architecture.
func platformArchAbbreviation(platform, arch string) string {
	switch {
	case platform == "windows" && (arch == "amd64" || arch == "64"):
		return "win64"
	case platform == "windows" && (arch == "386" || arch == "32"):
		return "win32"
	case platform == "windows" && arch == "arm64":
		return "winarm64"
	case platform == "darwin" && (arch == "amd64" || arch == "64"):
		return "mac64"
	case platform == "darwin" && arch == "arm64":
		return "macarm64"
	case platform == "linux" && (arch == "amd64" || arch == "64"):
		return "linux64"
	case platform == "linux" && (arch == "386" || arch == "32"):
		return "linux32"
	case platform == "linux" && arch == "arm64":
		return "linuxarm64"
	default:
		return fmt.Sprintf("%s_%s", platform, arch)
	}
}

// urlDecode URL-decodes a string, wrapping url.PathUnescape.
func urlDecode(s string) (string, error) {
	// Use url.QueryUnescape for broader decoding (handles both + and %20).
	return url.QueryUnescape(s)
}

func (s *serveSyncSource) GetChecksum(ctx context.Context, browser, version, platform, arch string) (string, error) {
	if s.firefoxSrc != nil {
		return s.firefoxSrc.GetChecksum(ctx, version, source.Platform(platform), source.Arch(arch))
	}
	// 非 Firefox 源不提供校验和（chrome/chromium 的下载源已内嵌校验）
	return "", nil
}

// shortcutAdapter adapts shortcut.Manager to cli.ShortcutProvider.
type shortcutAdapter struct {
	mgr *shortcut.Manager
}

func (a *shortcutAdapter) ensureManager() {
	if a.mgr == nil {
		a.mgr = shortcut.NewManager()
	}
}

func (a *shortcutAdapter) Create(opts shortcut.Options) error {
	a.ensureManager()
	return a.mgr.Create(opts)
}

func (a *shortcutAdapter) Remove(name string, desktopDir string) error {
	a.ensureManager()
	return a.mgr.Remove(name, desktopDir)
}

func (a *shortcutAdapter) List(desktopDir string) ([]string, error) {
	a.ensureManager()
	return a.mgr.List(desktopDir)
}

// downloadAdapter adapts download.Manager to cli.DownloadProvider.
type downloadAdapter struct {
	mgr   *download.Manager
	paths *paths.Paths
}

func (a *downloadAdapter) Download(url string, destPath string, onProgress func(downloaded, total int64, percent float64)) (string, error) {
	result, err := a.mgr.Download(context.TODO(), download.Options{
		URL:      url,
		DestPath: destPath,
		Resume:   true,
		OnProgress: func(p download.Progress) {
			onProgress(p.Downloaded, p.Total, p.Percent)
		},
		ProgressInterval: 100 * time.Millisecond,
	})
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

// sourceAdapter adapts source.Source to cli.SourceProvider.
type sourceAdapter struct {
	src source.Source
	cfg *config.Config
}

func (a *sourceAdapter) ResolveVersion(browser string, version string) (source.VersionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return a.src.Resolve(ctx, browser, version, source.CurrentPlatform(), source.CurrentArch())
}

func (a *sourceAdapter) ListVersions(browser string, channel string, versionPrefix string) ([]source.VersionInfo, error) {
	filter := &source.Filter{
		Browser:  browser,
		Platform: source.CurrentPlatform(),
		Arch:     source.CurrentArch(),
	}
	if channel != "" {
		filter.Channel = source.Channel(channel)
	}
	if versionPrefix != "" {
		filter.VersionPrefix = versionPrefix
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return a.src.List(ctx, filter)
}

// ForceRefresh 对所有支持缓存刷新的底层源设置强制刷新标志。
func (a *sourceAdapter) ForceRefresh() {
	if ms, ok := a.src.(*source.MultiSource); ok {
		ms.ForceRefresh()
	}
}

func (a *sourceAdapter) Describe() string {
	name := a.src.Name()
	if name == "" {
		return "未知源"
	}

	// Single source
	if desc := describeSourceName(name); desc != "" {
		return desc
	}

	// Multi source: parse "multi(source1,source2,...)"
	if strings.HasPrefix(name, "multi(") && strings.HasSuffix(name, ")") {
		inner := name[6 : len(name)-1]
		parts := strings.Split(inner, ",")
		var descs []string
		seen := make(map[string]bool)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			if d := describeSourceName(p); d != "" {
				descs = append(descs, d)
			}
		}
		if len(descs) > 0 {
			return strings.Join(descs, "、")
		}
	}

	return name
}

// describeSourceName returns a human-readable description for a source name.
func describeSourceName(name string) string {
	switch name {
	case "firefox-ftp":
		return "Mozilla FTP 目录"
	case "http":
		return "远程 HTTP 源"
	default:
		// HTTP source name format: "http:<url>" — extract just the URL part for display
		if strings.HasPrefix(name, "http:") && len(name) > 5 {
			return "远程 HTTP 源"
		}
		if strings.HasPrefix(name, "http") {
			return "远程 HTTP 源"
		}
		return ""
	}
}
