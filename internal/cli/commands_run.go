package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bws/bws/internal/automation"
	"github.com/bws/bws/internal/driver"
	"github.com/bws/bws/internal/instance"
)

// runFlags returns the flags accepted by the `run` command. It is shared with
// NewRunCommand so help text and parsing can never drift apart.
func runFlags() []*Flag {
	return []*Flag{
		{Name: "headless", Short: "H", Usage: "无头模式", HasValue: false, Default: "false"},
		{Name: "incognito", Short: "i", Usage: "隐身模式", HasValue: false, Default: "false"},
		{Name: "new-window", Short: "w", Usage: "新窗口", HasValue: false, Default: "false"},
		{Name: "profile", Short: "p", Usage: "配置文件名称", HasValue: true, Default: ""},
		{Name: "native", Short: "n", Usage: "原生模式（无隔离）", HasValue: false, Default: "false"},
		{Name: "detached", Short: "d", Usage: "后台运行", HasValue: false, Default: "false"},
		{Name: "daemon", Usage: "后台运行并注册为可管理实例（等价 detached + 注册表）", HasValue: false, Default: "false"},
		{Name: "dry-run", Short: "", Usage: "试运行", HasValue: false, Default: "false"},
		{Name: "proxy", Short: "", Usage: "代理地址（如 socks5://127.0.0.1:1080），留空使用全局配置", HasValue: true, Default: ""},
		{Name: "no-proxy", Short: "", Usage: "禁用代理（覆盖全局配置）", HasValue: false, Default: "false"},
		{Name: "fingerprint", Short: "fp", Usage: "指纹隔离预设（standard/random/none），或 JSON 配置/@文件路径", HasValue: true, Default: ""},
		{Name: "plugin", Short: "", Usage: "激活的插件（逗号分隔多个）", HasValue: true, Default: ""},
		{Name: "refresh", Short: "", Usage: "强制刷新远程源缓存（自动安装时生效）", HasValue: false, Default: "false"},
		{Name: "automation", Usage: "自动化模式：注入 CDP 参数、管理匹配版本的驱动并输出端点", HasValue: false, Default: "false"},
		{Name: "cdp", Usage: "启用 CDP 端点（--automation 的子集）", HasValue: false, Default: "false"},
		{Name: "webdriver", Usage: "启用 WebDriver 端点（--automation 的子集）", HasValue: false, Default: "false"},
		{Name: "driver-port", Usage: "chromedriver 监听端口（0 表示自动分配）", HasValue: true, Default: "0"},
		{Name: "driver-no-download", Usage: "自动化模式下不自动下载驱动（缺失时仅告警）", HasValue: false, Default: "false"},
		{Name: "endpoint-timeout", Usage: "端点发现超时（秒）", HasValue: true, Default: "10"},
		{Name: "json", Usage: "以 JSON 格式输出（需配合自动化/守护模式）", HasValue: false, Default: "false"},
	}
}

// runData is the stable JSON payload of the `run` command in automation mode.
// Pointer fields render as `null` when the endpoint is unavailable, so scripts
// can rely on the keys always being present.
type runData struct {
	Instance  string  `json:"instance"`
	Browser   string  `json:"browser"`
	Version   string  `json:"version"`
	PID       int     `json:"pid"`
	Binary    string  `json:"binary"`
	Profile   *string `json:"profile"`
	CDP       *string `json:"cdp"`
	WebDriver *string `json:"webdriver"`
	Daemon    bool    `json:"daemon"`
}

func runRun(ctx *Context, args []string) error {
	// Split args at -- to separate bws args from browser args
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

	flagVals, positional, err := ParseFlags(bmArgs, runFlags())
	if err != nil {
		return err
	}

	jsonMode := flagVals["json"] == "true"
	out := NewOutput(ctx, "run", jsonMode)

	if len(positional) == 0 {
		return out.Error(ErrCodeInvalidArgument, "请指定浏览器版本，例如 'bws r chrome@120'")
	}

	// Parse browser@version
	spec := resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())
	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] 解析规格: %s@%s (别名=%v)", spec.Browser, spec.Version, spec.IsAlias)
	}

	// --- Version Resolution (Chain of Responsibility + Selection Strategy) ---
	// Detects TTY automatically: arrow-key selection in terminal, auto-pick in pipe.
	resolvedVersion, err := RunResolutionChain(
		ctx, spec.Browser, spec.Version,
		flagVals["refresh"] == "true",
		DetectSelector(),
	)
	if err != nil {
		return out.Error(ErrCodeNotFound, err.Error())
	}
	spec.Version = resolvedVersion

	// URLs from remaining positional args (before --)
	urls := positional[1:]
	extraArgs := browserArgs

	// Resolve proxy: --no-proxy takes precedence, then --proxy, then config
	proxyURL := ""
	if flagVals["no-proxy"] != "true" {
		if p := flagVals["proxy"]; p != "" {
			proxyURL = p
		} else if ctx.Cfg != nil && ctx.Cfg.Proxy != nil {
			proxyURL = ctx.Cfg.Proxy.GetProxy()
		}
	}

	// Automation switches. --automation is the superset; --cdp / --webdriver
	// enable a single capability. Combining them is a no-op, never an error.
	automationMode := flagVals["automation"] == "true"
	daemon := flagVals["daemon"] == "true"
	wantCDP := (automationMode || flagVals["cdp"] == "true") && automation.SupportsCDP(spec.Browser)
	wantDriver := (automationMode || flagVals["webdriver"] == "true") && automation.SupportsWebDriver(spec.Browser)
	managed := automationMode || flagVals["cdp"] == "true" || flagVals["webdriver"] == "true" || daemon

	if jsonMode && !managed {
		return out.Error(ErrCodeInvalidArgument, "--json 需要与 --automation/--cdp/--webdriver/--daemon 一起使用")
	}

	endpointTimeout, err := parseEndpointTimeout(flagVals["endpoint-timeout"])
	if err != nil {
		return out.Error(ErrCodeInvalidArgument, err.Error())
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
		Detached:    flagVals["detached"] == "true" || managed,
		DryRun:      flagVals["dry-run"] == "true",
		Proxy:       proxyURL,
		Fingerprint: flagVals["fingerprint"],
		Plugins:     parsePluginList(flagVals["plugin"]),
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] headless=%v incognito=%v newWindow=%v native=%v detached=%v dryRun=%v proxy=%q plugins=%v",
			opts.Headless, opts.Incognito, opts.NewWindow, opts.NativeMode, opts.Detached, opts.DryRun, proxyURL, opts.Plugins)
	}

	// Inject automation-friendly CDP flags (user-supplied flags always win).
	if wantCDP {
		opts.ExtraArgs = append(opts.ExtraArgs, automation.BrowserArgs(opts.ExtraArgs, 0)...)
	}
	// A user-supplied --remote-debugging-port is the only discovery channel
	// available in native mode, so it must be carried into endpoint discovery.
	cdpPort := automation.CDPPortFromArgs(opts.ExtraArgs)

	if opts.DryRun {
		exe, cmdArgs, derr := ctx.Launch.PreviewCommand(opts)
		if derr != nil {
			return derr
		}
		ctx.Printf("将要执行: %s %s\n", exe, strings.Join(cmdArgs, " "))
		return nil
	}

	// Verbose: 预览可执行文件路径和完整命令行
	if ctx.Logger != nil {
		if exe, cmdArgs, perr := ctx.Launch.PreviewCommand(opts); perr == nil {
			ctx.Logger.Debug("[run] 可执行文件路径: %s", exe)
			ctx.Logger.Debug("[run] 完整启动命令: %s %s", exe, strings.Join(cmdArgs, " "))
		}
	}

	// Non-automation launches keep the original blocking behaviour.
	if !managed {
		startTime := time.Now()
		err = ctx.Launch.Run(opts)
		if ctx.Logger != nil {
			if opts.Detached {
				ctx.Logger.Debug("[run] 进程已启动（detached 模式）")
			} else if err == nil {
				ctx.Logger.Debug("[run] 进程已退出：退出码=0 运行时长=%s", formatDuration(time.Since(startTime)))
			} else {
				ctx.Logger.Debug("[run] 进程异常：%v 运行时长=%s", err, formatDuration(time.Since(startTime)))
			}
		}
		return err
	}

	return runAutomation(ctx, out, opts, spec, automationOptions{
		WantCDP:          wantCDP,
		WantDriver:       wantDriver,
		Daemon:           daemon,
		DriverPortRaw:    flagVals["driver-port"],
		NoDriverDownload: flagVals["driver-no-download"] == "true",
		EndpointTimeout:  endpointTimeout,
		CDPPort:          cdpPort,
	})
}

// automationOptions bundles the automation-specific inputs for runAutomation.
type automationOptions struct {
	WantCDP          bool
	WantDriver       bool
	Daemon           bool
	DriverPortRaw    string
	NoDriverDownload bool
	EndpointTimeout  time.Duration

	// CDPPort is the fixed port the user pinned via --remote-debugging-port
	// (0 when unspecified, i.e. the OS assigns one).
	CDPPort int
}

// runAutomation launches the browser in the background, wires up the CDP and
// WebDriver endpoints, optionally registers the instance, and emits the launch
// contract. Endpoint failures never block the launch: the affected field is
// simply left null and a warning is written to stderr.
func runAutomation(
	ctx *Context,
	out *Output,
	opts LaunchOptions,
	spec browserVersionSpec,
	ao automationOptions,
) error {
	if ctx.Launch == nil {
		return out.Error(ErrCodeUnsupported, "当前构建不支持启动功能")
	}

	// --- WebDriver: start a driver matching the exact browser build ---
	var driverProc *driver.Process
	if ao.WantDriver {
		driverPort, perr := parseDriverPort(ao.DriverPortRaw)
		if perr != nil {
			return out.Error(ErrCodeInvalidArgument, perr.Error())
		}
		proc, derr := startAutomationDriver(ctx, spec, driverPort, true, ao.NoDriverDownload)
		if derr != nil {
			out.Meta("⚠ 驱动启动失败: %v", derr)
		} else {
			driverProc = proc
		}
	} else if !automation.SupportsWebDriver(spec.Browser) {
		out.Meta("⚠ %s 暂不支持托管的 WebDriver 驱动，webdriver 端点不可用。", spec.Browser)
	}

	// --- Browser: non-blocking start so endpoints can be discovered ---
	result, err := ctx.Launch.Start(opts)
	if err != nil {
		if driverProc != nil {
			_ = driverProc.Kill()
		}
		return out.Error(ErrCodeInternal, err.Error())
	}

	// --- CDP endpoint discovery ---
	var cdpEndpoint *string
	if ao.WantCDP {
		// Native mode (and system browsers, which imply it) never receives
		// --user-data-dir, so DevToolsActivePort is never written. Without an
		// explicit port there is no discovery channel at all: fail fast with an
		// actionable hint instead of burning the whole endpoint timeout.
		if result.ProfileDir == "" && ao.CDPPort == 0 {
			out.Meta("⚠ 原生模式无法从 profile 发现 CDP 端点；如需 CDP，请显式指定端口，例如 '-- --remote-debugging-port=9222'。")
		} else if endpoint, derr := automation.DiscoverCDP(context.Background(), result.ProfileDir, ao.CDPPort, ao.EndpointTimeout); derr != nil {
			out.Meta("⚠ CDP 端点发现失败: %v", derr)
		} else {
			cdpEndpoint = &endpoint
		}
	}

	// --- WebDriver endpoint ---
	var webdriverURL *string
	if driverProc != nil {
		url := driverProc.URL
		webdriverURL = &url
	}

	// --- Instance registration (daemon only) ---
	name := instance.Name(spec.Browser, spec.Version, opts.ProfileName)
	inst := instance.Instance{
		Name:       name,
		Browser:    spec.Browser,
		Version:    spec.Version,
		Profile:    opts.ProfileName,
		PID:        result.PID,
		Binary:     result.Binary,
		ProfileDir: result.ProfileDir,
		CDP:        cdpEndpoint,
		WebDriver:  webdriverURL,
		StartedAt:  time.Now(),
		Daemon:     ao.Daemon,
	}
	if driverProc != nil {
		inst.DriverPID = driverProc.Pid
	}

	if ao.Daemon {
		if ctx.Instance == nil {
			if result.Proc != nil {
				_ = result.Proc.Kill()
			}
			if driverProc != nil {
				_ = driverProc.Kill()
			}
			return out.Error(ErrCodeUnsupported, "当前构建不支持实例管理")
		}
		if err := ctx.Instance.Add(inst); err != nil {
			if result.Proc != nil {
				_ = result.Proc.Kill()
			}
			if driverProc != nil {
				_ = driverProc.Kill()
			}
			return out.Error(ErrCodeConflict, err.Error())
		}
	}

	// --- Emit the launch contract ---
	var profilePtr *string
	if result.ProfileDir != "" {
		p := result.ProfileDir
		profilePtr = &p
	}

	data := runData{
		Instance:  name,
		Browser:   spec.Browser,
		Version:   spec.Version,
		PID:       result.PID,
		Binary:    result.Binary,
		Profile:   profilePtr,
		CDP:       cdpEndpoint,
		WebDriver: webdriverURL,
		Daemon:    ao.Daemon,
	}

	return out.Emit(data, func() {
		out.Object([]Field{
			{Key: "Instance", Value: name},
			{Key: "Browser", Value: spec.Browser},
			{Key: "Version", Value: spec.Version},
			{Key: "Binary", Value: result.Binary},
			{Key: "Profile", Value: dashIfEmpty(result.ProfileDir)},
			{Key: "PID", Value: strconv.Itoa(result.PID)},
			{Key: "CDP", Value: dashIfNil(cdpEndpoint)},
			{Key: "WebDriver", Value: dashIfNil(webdriverURL)},
			{Key: "Daemon", Value: strconv.FormatBool(ao.Daemon)},
		})
		if ao.Daemon {
			out.Meta("实例已在后台运行。使用 'bws ps' 查看，'bws stop %s' 停止。", name)
		}
	})
}

// parseEndpointTimeout parses the --endpoint-timeout value (seconds).
func parseEndpointTimeout(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return automation.DefaultEndpointTimeout, nil
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("无效的 --endpoint-timeout: %q", raw)
	}
	return time.Duration(secs) * time.Second, nil
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func dashIfNil(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}
