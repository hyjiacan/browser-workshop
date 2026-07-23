//	bws - Browser Manager
//
// Usage:
//
//	bws <command> [options]
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/cli"
	"github.com/bws/bws/internal/config"
	"github.com/bws/bws/internal/download"
	"github.com/bws/bws/internal/fingerprint"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/launch"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/plugin"
	"github.com/bws/bws/internal/repo"
	bmserve "github.com/bws/bws/internal/serve"
	"github.com/bws/bws/internal/shortcut"
	"github.com/bws/bws/internal/source"
	"github.com/bws/bws/internal/system"
)

const version = "0.1.0"

func main() {
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

	// Initialize paths
	p := paths.New(dataDir)
	if err := p.EnsureAll(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化目录失败: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger (dual output: file at DEBUG level, console at config level)
	consoleLevel := bmlog.ParseLevel(cfg.LogLevel)
	logger, err := bmlog.NewDualLogger(p.LogFile, bmlog.LevelDebug, consoleLevel, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 初始化日志系统失败: %v\n", err)
		logger = bmlog.Default()
	}
	defer logger.Close()

	// Replace default logger with our configured logger
	// so package-level log functions use the same output
	bmlog.SetDefault(logger)

	fmt.Printf("bws starting (version %s)\n", version)
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

	// 数据源：离线源（bws serve）优先，然后是内置在线源
	var sources []source.Source

	// 1. 离线源（如果配置了且启用）
	if cfg.IsServeSourceEnabled() && cfg.RemoteSource != "" {
		sources = append(sources, source.NewHTTPSourceWithProxy(cfg.RemoteSource, proxyURL))
	}

	// 2. Chrome 在线源（根据开关）
	if cfg.IsOmahaSourceEnabled() {
		sources = append(sources, source.NewChromeOmahaSourceWithProxy(proxyURL))
		sources = append(sources, source.NewChromeSourceWithProxy(proxyURL))
	}

	// 3. Firefox 在线源（根据开关）
	if cfg.IsFirefoxFTPEnabled() {
		sources = append(sources, source.NewFirefoxSourceWithProxy(proxyURL))
	}

	sourceMgr := source.NewMultiSource(sources...)

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
	ctx.Config = &configAdapter{cfg: cfg, configPath: configPath, dataDir: dataDir}
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
	ctx.Serve = &serveAdapter{version: version, source: sourceMgr}
	ctx.Plugin = &pluginAdapter{mgr: pluginMgr}
	ctx.Logger = logger
	if repoImporter != nil {
		ctx.Repo = &repoAdapter{scanner: repoScanner, importer: repoImporter}
	}

	// Create app
	app := cli.NewApp("bws", version, ctx)
	cli.RegisterCommands(app)

	// Execute
	if err := app.Execute(os.Args[1:]); err != nil {
		logger.Error("命令执行失败: %v", err)
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// resolvePaths determines the config file path and data root directory.
// Portable mode: bm-data subdirectory next to the executable.
// Otherwise, use ~/.bm/
func resolvePaths() (configPath string, dataRoot string, isNew bool) {
	// Check for BM_HOME environment variable (highest priority)
	if home := os.Getenv("BM_HOME"); home != "" {
		configPath := filepath.Join(home, "config.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, home, false
		}
		return configPath, home, true
	}

	// Default: bm-data subdirectory next to the executable (portable mode)
	exeDir, err := paths.ExeDir()
	if err == nil {
		dataDir := filepath.Join(exeDir, "bws-data")
		configPath := filepath.Join(dataDir, "config.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, dataDir, false
		}
		// Config doesn't exist yet, but we still use bm-data as default
		return configPath, dataDir, true
	}

	// Fallback: ~/.bm
	home, err := os.UserHomeDir()
	if err != nil {
		wd, _ := os.Getwd()
		home = wd
	}
	dataRoot = filepath.Join(home, ".bm")
	configPath = filepath.Join(dataRoot, "config.json")

	// Check if config exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, dataRoot, false
	}

	return configPath, dataRoot, true
}

// firstTimeSetup runs the first-time setup wizard.
func firstTimeSetup(configPath string, cfg *config.Config) *config.Config {
	fmt.Println("========================================")
	fmt.Println("  bws - Browser Manager")
	fmt.Println("  First-time setup")
	fmt.Println("========================================")
	fmt.Println()

	// Ask about data directory
	defaultDataDir := filepath.Dir(configPath)
	fmt.Printf("Data directory [default: %s]: ", defaultDataDir)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		// If reading fails (e.g. EOF/non-interactive), use defaults
		fmt.Println("\nNon-interactive mode, using default values.")
		cfg.DataDir = defaultDataDir
		// Create config directory and save
		configDir := filepath.Dir(configPath)
		os.MkdirAll(configDir, 0o755)
		config.Save(cfg, configPath)
		return cfg
	}
	input = strings.TrimSpace(input)

	if input != "" {
		absPath, err := filepath.Abs(input)
		if err == nil {
			cfg.DataDir = absPath
		} else {
			cfg.DataDir = input
		}
	}

	// Ask about default browser
	fmt.Printf("Default browser [default: chrome]: ")
	input, err = reader.ReadString('\n')
	if err != nil {
		// Use default on read error
		input = ""
	}
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.DefaultBrowser = input
	}

	// Create config directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 创建配置目录失败: %v\n", err)
	}

	// Save config
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 保存配置失败: %v\n", err)
	}

	fmt.Println()
	fmt.Printf("配置已保存: %s\n", configPath)
	fmt.Println("初始化完成! 使用 'bws --help' 查看可用命令。")
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

func (a *configAdapter) DefaultChannel() string {
	return a.cfg.DefaultChannel
}

func (a *configAdapter) SetDefaultChannel(channel string) error {
	a.cfg.DefaultChannel = channel
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetLogLevel() string {
	return a.cfg.LogLevel
}

func (a *configAdapter) SetLogLevel(level string) error {
	a.cfg.LogLevel = level
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
	v, ok := a.cfg.Aliases[name]
	return v, ok
}

func (a *configAdapter) AddAlias(name, target string) error {
	if a.cfg.Aliases == nil {
		a.cfg.Aliases = make(map[string]string)
	}
	a.cfg.Aliases[name] = target
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) RemoveAlias(name string) error {
	delete(a.cfg.Aliases, name)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) ListAliases() map[string]string {
	return a.cfg.Aliases
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

func (a *configAdapter) IsServeSourceEnabled() bool     { return a.cfg.IsServeSourceEnabled() }
func (a *configAdapter) SetServeSourceEnabled(v bool) error {
	a.cfg.SetServeSourceEnabled(v)
	return config.Save(a.cfg, a.configPath)
}
func (a *configAdapter) IsOmahaSourceEnabled() bool      { return a.cfg.IsOmahaSourceEnabled() }
func (a *configAdapter) SetOmahaSourceEnabled(v bool) error {
	a.cfg.SetOmahaSourceEnabled(v)
	return config.Save(a.cfg, a.configPath)
}
func (a *configAdapter) IsFirefoxFTPEnabled() bool       { return a.cfg.IsFirefoxFTPEnabled() }
func (a *configAdapter) SetFirefoxFTPEnabled(v bool) error {
	a.cfg.SetFirefoxFTPEnabled(v)
	return config.Save(a.cfg, a.configPath)
}
func (a *configAdapter) GetDiskSpaceThresholdGB() int    { return a.cfg.GetDiskSpaceThresholdGB() }
func (a *configAdapter) SetDiskSpaceThresholdGB(v int) error {
	a.cfg.SetDiskSpaceThresholdGB(v)
	return config.Save(a.cfg, a.configPath)
}

func (a *configAdapter) GetProxy() string { return a.cfg.GetProxy() }
func (a *configAdapter) SetProxy(proxy string) error {
	a.cfg.SetProxy(proxy)
	return config.Save(a.cfg, a.configPath)
}

type pluginAdapter struct {
	mgr *plugin.Manager
}

func (a *pluginAdapter) List() []plugin.ManifestEntry               { return a.mgr.List() }
func (a *pluginAdapter) Install(entry plugin.ManifestEntry) error  { return a.mgr.Install(entry) }
func (a *pluginAdapter) Uninstall(name string) error               { return a.mgr.Uninstall(name) }
func (a *pluginAdapter) PluginsDir() string                        { return a.mgr.PluginsDir() }

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

func (a *installAdapter) ListInstalled() ([]cli.InstalledVersion, error) {
	list, err := a.mgr.ListInstalled()
	if err != nil {
		return nil, err
	}
	result := make([]cli.InstalledVersion, len(list))
	for i, v := range list {
		result[i] = cli.InstalledVersion{
			Browser: v.Browser,
			Version: v.Version,
			Channel: v.Channel,
			Size:    0,
		}
	}
	// Get sizes from records
	for i := range result {
		rec, err := a.mgr.GetRecord(result[i].Browser, result[i].Version)
		if err == nil {
			result[i].Size = rec.Size
		}
	}
	return result, nil
}

func (a *installAdapter) ListInstalledByBrowser(browser string) ([]cli.InstalledVersion, error) {
	list, err := a.mgr.ListInstalledByBrowser(browser)
	if err != nil {
		return nil, err
	}
	result := make([]cli.InstalledVersion, len(list))
	for i, v := range list {
		result[i] = cli.InstalledVersion{
			Browser: v.Browser,
			Version: v.Version,
			Channel: v.Channel,
		}
		rec, err := a.mgr.GetRecord(v.Browser, v.Version)
		if err == nil {
			result[i].Size = rec.Size
		}
	}
	return result, nil
}

func (a *installAdapter) GetRecord(browser, version string) (*cli.InstallRecord, error) {
	rec, err := a.mgr.GetRecord(browser, version)
	if err != nil {
		return nil, err
	}
	return &cli.InstallRecord{
		Browser:        rec.Browser,
		Version:        rec.Version,
		InstalledAt:    rec.InstalledAt.String(),
		Platform:       rec.Platform,
		Arch:           rec.Arch,
		InstallDir:     rec.InstallDir,
		ExecutablePath: rec.ExecutablePath,
		Size:           rec.Size,
		Source:         rec.Source,
	}, nil
}

func (a *installAdapter) Uninstall(browser, version string) error {
	return a.mgr.Uninstall(browser, version)
}

func (a *installAdapter) InstallFromDir(browser, version, sourceDir string) (*cli.InstallRecord, error) {
	rec, err := a.mgr.InstallFromDir(install.InstallOptions{
		Browser:   browser,
		Version:   version,
		Source:    "cli",
		SourceDir: sourceDir,
	}, nil)
	if err != nil {
		return nil, err
	}
	return &cli.InstallRecord{
		Browser:        rec.Browser,
		Version:        rec.Version,
		InstalledAt:    rec.InstalledAt.String(),
		Platform:       rec.Platform,
		Arch:           rec.Arch,
		InstallDir:     rec.InstallDir,
		ExecutablePath: rec.ExecutablePath,
		Size:           rec.Size,
		Source:         rec.Source,
	}, nil
}

func (a *installAdapter) InstallFromFile(browser, version, filePath string) (*cli.InstallRecord, error) {
	rec, err := a.mgr.InstallFromFile(browser, version, filePath)
	if err != nil {
		return nil, err
	}
	return &cli.InstallRecord{
		Browser:        rec.Browser,
		Version:        rec.Version,
		InstalledAt:    rec.InstalledAt.String(),
		Platform:       rec.Platform,
		Arch:           rec.Arch,
		InstallDir:     rec.InstallDir,
		ExecutablePath: rec.ExecutablePath,
		Size:           rec.Size,
		Source:         rec.Source,
	}, nil
}

func (a *installAdapter) HasSystem() bool {
	return a.mgr.HasSystem()
}

func (a *installAdapter) ListWithSystem() ([]cli.InstalledVersion, error) {
	list, err := a.mgr.ListWithSystem()
	if err != nil {
		return nil, err
	}
	result := make([]cli.InstalledVersion, len(list))
	for i, v := range list {
		result[i] = cli.InstalledVersion{
			Browser:  v.Browser,
			Version:  v.Version,
			Channel:  v.Channel,
			IsSystem: v.IsSystem,
			Source:   v.Source,
		}
		if !v.IsSystem {
			rec, err := a.mgr.GetRecord(v.Browser, v.Version)
			if err == nil {
				result[i].Size = rec.Size
			}
		}
	}
	return result, nil
}

func (a *installAdapter) ListWithSystemByBrowser(browser string) ([]cli.InstalledVersion, error) {
	list, err := a.mgr.ListWithSystemByBrowser(browser)
	if err != nil {
		return nil, err
	}
	result := make([]cli.InstalledVersion, len(list))
	for i, v := range list {
		result[i] = cli.InstalledVersion{
			Browser:  v.Browser,
			Version:  v.Version,
			Channel:  v.Channel,
			IsSystem: v.IsSystem,
			Source:   v.Source,
		}
		if !v.IsSystem {
			rec, err := a.mgr.GetRecord(v.Browser, v.Version)
			if err == nil {
				result[i].Size = rec.Size
			}
		}
	}
	return result, nil
}

func (a *installAdapter) IsSystemVersion(browser, version string) bool {
	return a.mgr.IsSystemVersion(browser, version)
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

func (a *profileAdapter) ListProfiles(browser string) ([]cli.ProfileInfo, error) {
	list, err := a.mgr.ListProfiles(browser)
	if err != nil {
		return nil, err
	}
	result := make([]cli.ProfileInfo, len(list))
	for i, p := range list {
		result[i] = cli.ProfileInfo{
			Name:    p.Name,
			Path:    p.Path,
			Type:    p.Type,
			Version: p.Version,
		}
	}
	return result, nil
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

	// If not detached, wait for the process
	if !opts.Detached {
		return proc.Wait()
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

func (a *repoAdapter) Scan() ([]cli.RepoScanResult, error) {
	matches, err := a.scanner.Scan()
	if err != nil {
		return nil, err
	}

	result := make([]cli.RepoScanResult, len(matches))
	for i, m := range matches {
		result[i] = cli.RepoScanResult{
			Path:    m.Path,
			Browser: m.Browser,
			Version: m.Version,
			Arch:    m.Arch,
			Status:  m.Status.String(),
			Detail:  m.Detail,
		}
	}
	return result, nil
}

func (a *repoAdapter) Import(force bool, onProgress func(int, int, string)) (*cli.RepoImportSummary, error) {
	var cb repo.ProgressCallback
	if onProgress != nil {
		cb = func(p repo.ImportProgress) {
			onProgress(p.Current, p.Total, p.Message)
		}
	}
	summary, err := a.importer.ImportAll(repo.ImportOptions{
		Force: force,
	}, cb)
	if err != nil {
		return nil, err
	}

	return &cli.RepoImportSummary{
		Total:                  summary.Total,
		Success:                summary.Success,
		Failed:                 summary.Failed,
		Skipped:                summary.Skipped,
		SkippedIncompatible:    summary.SkippedIncompatible,
		SkippedAlreadyInstalled: summary.SkippedAlreadyInstalled,
	}, nil
}

// serveAdapter adapts serve.Server to cli.ServeProvider.
type serveAdapter struct {
	version string
	source  source.Source // the multi-source for syncing
}

func (a *serveAdapter) StartFromConfig(baseDir string) error {
	// Load serve config from bws-serve.ini
	cfg, err := bmserve.LoadServeConfig(baseDir)
	if err != nil {
		return fmt.Errorf("加载 serve 配置失败: %w", err)
	}

	addr := cfg.Addr()

	// Resolve effective directories: config value > CLI -d default
	exeDir, _ := paths.ExeDir()
	if exeDir == "" {
		exeDir, _ = os.Getwd()
	}

	packagesDir := cfg.PackagesDir
	if packagesDir == "" {
		packagesDir = filepath.Join(exeDir, "packages")
	}

	binDir := cfg.BinDir
	if binDir == "" {
		binDir = filepath.Join(exeDir, "bin")
	}

	var syncSource bmserve.SyncSource

	if cfg.SyncEnabled && a.source != nil {
		// Parse interval
		interval, err := cfg.SyncDuration()
		if err != nil {
			return fmt.Errorf("解析同步间隔失败: %w", err)
		}

		// Create sync source adapter
		syncSource = &serveSyncSource{
			src: a.source,
		}

		srv := bmserve.NewServerWithOptions(bmserve.ServerOptions{
			Addr:         addr,
			Version:      a.version,
			PackagesDir:  packagesDir,
			BinDir:       binDir,
			SyncSource: syncSource,
			SyncConfig: bmserve.SyncConfig{
				Enabled:  true,
				Interval: interval,
				Browsers: cfg.SyncBrowsersList(),
				Channels: cfg.SyncChannelsList(),
			},
		})
		return srv.Start()
	}

	srv := bmserve.NewServerWithOptions(bmserve.ServerOptions{
		Addr:        addr,
		Version:     a.version,
		PackagesDir: packagesDir,
		BinDir:      binDir,
	})
	return srv.Start()
}

func (a *serveAdapter) ConfigPath() string {
	return bmserve.ConfigPath("")
}

func (a *serveAdapter) EnsureDefaultConfig(baseDir string) (string, bool, error) {
	return bmserve.EnsureDefaultConfig(baseDir)
}

// serveSyncSource adapts source.Source to serve.SyncSource.
type serveSyncSource struct {
	src source.Source
}

func (s *serveSyncSource) ListVersions(browser string, channel string, platform string, arch string) ([]bmserve.SyncVersionInfo, error) {
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

	versions, err := s.src.List(context.TODO(), filter)
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
			Size:        v.Size,
		})
	}
	return result, nil
}

func (s *serveSyncSource) Download(url string, destDir string, onProgress func(downloaded, total int64)) (string, error) {
	return bmserve.DefaultDownload(url, destDir, onProgress)
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

func (a *shortcutAdapter) Create(opts cli.ShortcutOptions) error {
	a.ensureManager()
	return a.mgr.Create(shortcut.Options{
		Name:       opts.Name,
		Target:     opts.Target,
		Args:       opts.Args,
		WorkingDir: opts.WorkingDir,
		IconPath:   opts.IconPath,
		DesktopDir: opts.DesktopDir,
	})
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

func (a *sourceAdapter) ResolveVersion(browser string, version string) (cli.SourceVersionInfo, error) {
	v, err := a.src.Resolve(context.TODO(), browser, version, source.CurrentPlatform(), source.CurrentArch())
	if err != nil {
		return cli.SourceVersionInfo{}, err
	}
	return cli.SourceVersionInfo{
		Browser:     v.Browser,
		Version:     v.Version,
		Channel:     string(v.Channel),
		Platform:    string(v.Platform),
		Arch:        string(v.Arch),
		DownloadURL: v.DownloadURL,
		Size:        v.Size,
	}, nil
}

func (a *sourceAdapter) ListVersions(browser string, channel string) ([]cli.SourceVersionInfo, error) {
	filter := &source.Filter{
		Browser:  browser,
		Platform: source.CurrentPlatform(),
		Arch:     source.CurrentArch(),
	}
	if channel != "" {
		filter.Channel = source.Channel(channel)
	}
	versions, err := a.src.List(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	result := make([]cli.SourceVersionInfo, len(versions))
	for i, v := range versions {
		result[i] = cli.SourceVersionInfo{
			Browser:     v.Browser,
			Version:     v.Version,
			Channel:     string(v.Channel),
			Platform:    string(v.Platform),
			Arch:        string(v.Arch),
			DownloadURL: v.DownloadURL,
			Size:        v.Size,
		}
	}
	return result, nil
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
	case "chrome-omaha":
		return "Chrome Omaha 协议"
	case "chrome-omahaproxy":
		return "Chrome Omaha Proxy"
	case "firefox-mozilla":
		return "Mozilla Product Details"
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
