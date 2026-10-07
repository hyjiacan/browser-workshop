package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bws/bws/internal/driver"
	"github.com/bws/bws/internal/version"
)

// NewDriverCommand creates the `bws driver` command group.
//
// Automation drivers are version-locked to the browser: a chromedriver build
// only drives the matching Chrome major. bws therefore resolves, downloads and
// runs the driver on demand instead of shipping a fixed version.
func NewDriverCommand() *Command {
	return &Command{
		Name:        "driver",
		Aliases:     []string{"drv"},
		Description: "管理自动化驱动（chromedriver）",
		Usage:       "bws driver <命令> [选项]",
		Examples: []string{
			"driver ls",
			"driver install chrome@120",
			"driver install 120 --dry-run",
			"driver start 120",
			"driver uninstall 120",
		},
		SubCommands: []*Command{
			NewDriverLsCommand(),
			NewDriverInstallCommand(),
			NewDriverStartCommand(),
			NewDriverUninstallCommand(),
		},
		Run: runDriverLs,
	}
}

// --- driver ls ---

func NewDriverLsCommand() *Command {
	return &Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "列出已安装的驱动",
		Usage:       "bws driver ls",
		Run:         runDriverLs,
	}
}

func runDriverLs(ctx *Context, args []string) error {
	if err := checkFeature(ctx.Driver, "驱动管理"); err != nil {
		return err
	}

	records, err := ctx.Driver.List()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		ctx.Println("未安装任何驱动。使用 'bws driver install chrome@<版本>' 安装。")
		return nil
	}

	rows := make([][]string, 0, len(records))
	for _, r := range records {
		installedAt := "-"
		if !r.InstalledAt.IsZero() {
			installedAt = r.InstalledAt.Format("2006-01-02 15:04")
		}
		rows = append(rows, []string{
			r.Name,
			r.MajorVersion,
			r.Version,
			r.Platform + "/" + r.Arch,
			FormatSize(r.Size),
			installedAt,
		})
	}
	PrintTable(ctx.Stdout, []string{"驱动", "主版本", "版本", "平台", "大小", "安装时间"}, rows)
	return nil
}

// --- driver install ---

func NewDriverInstallCommand() *Command {
	return &Command{
		Name:        "install",
		Aliases:     []string{"i"},
		Description: "下载并安装匹配指定 Chrome 版本的驱动",
		Usage:       "bws driver install <chrome@版本> [选项]",
		Examples: []string{
			"driver install chrome@120",
			"driver install 120",
			"driver install chrome@120 --force",
			"driver install chrome@120 --dry-run",
		},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
			{Name: "dry-run", Short: "", Usage: "仅解析并显示下载信息，不实际安装", HasValue: false, Default: "false"},
		},
		Run: runDriverInstall,
	}
}

func runDriverInstall(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "force", Short: "f", Usage: "强制重新安装", HasValue: false, Default: "false"},
		{Name: "dry-run", Short: "", Usage: "仅解析并显示下载信息，不实际安装", HasValue: false, Default: "false"},
	}
	flagVals, positional, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}
	if err := checkFeature(ctx.Driver, "驱动管理"); err != nil {
		return err
	}
	if len(positional) == 0 {
		return fmt.Errorf("请指定 Chrome 版本，例如 'bws driver install chrome@120'")
	}

	chromeVersion, err := resolveDriverChromeVersion(ctx, positional[0])
	if err != nil {
		return err
	}

	if flagVals["dry-run"] == "true" {
		info, err := ctx.Driver.Resolve(chromeVersion)
		if err != nil {
			return err
		}
		ctx.Printf("将安装:   %s %s (%s/%s)\n", info.Name, info.Version, info.Platform, info.Arch)
		ctx.Printf("数据源:   %s\n", info.Source)
		ctx.Printf("下载地址: %s\n", info.DownloadURL)
		return nil
	}

	ctx.Printf("正在解析 Chrome %s 对应的 chromedriver...\n", chromeVersion)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[driver] 解析驱动: chrome=%s force=%v", chromeVersion, flagVals["force"] == "true")
	}

	rec, err := ctx.Driver.Ensure(chromeVersion, flagVals["force"] == "true", driverProgressPrinter(ctx))
	if err != nil {
		return err
	}

	ctx.Printf("\n✓ %s %s 已就绪\n", rec.Name, rec.Version)
	ctx.Printf("  目录: %s\n", rec.Dir)
	return nil
}

// --- driver start ---

func NewDriverStartCommand() *Command {
	return &Command{
		Name:        "start",
		Description: "启动已安装的驱动并监听端口",
		Usage:       "bws driver start <chrome@版本|主版本> [选项]",
		Examples: []string{
			"driver start 120",
			"driver start chrome@120 --port 9515",
			"driver start 120 --detach",
		},
		Flags: []*Flag{
			{Name: "port", Short: "p", Usage: "监听端口（0 表示自动分配）", HasValue: true, Default: "0"},
			{Name: "detach", Short: "d", Usage: "后台运行（不等待进程结束）", HasValue: false, Default: "false"},
			{Name: "allowed-ips", Short: "", Usage: "允许连接的 IP（默认 127.0.0.1）", HasValue: true, Default: ""},
			{Name: "log", Short: "", Usage: "驱动日志文件路径", HasValue: true, Default: ""},
		},
		Run: runDriverStart,
	}
}

func runDriverStart(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "port", Short: "p", Usage: "监听端口（0 表示自动分配）", HasValue: true, Default: "0"},
		{Name: "detach", Short: "d", Usage: "后台运行（不等待进程结束）", HasValue: false, Default: "false"},
		{Name: "allowed-ips", Short: "", Usage: "允许连接的 IP（默认 127.0.0.1）", HasValue: true, Default: ""},
		{Name: "log", Short: "", Usage: "驱动日志文件路径", HasValue: true, Default: ""},
	}
	flagVals, positional, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}
	if err := checkFeature(ctx.Driver, "驱动管理"); err != nil {
		return err
	}
	if len(positional) == 0 {
		return fmt.Errorf("请指定驱动版本，例如 'bws driver start 120'")
	}

	major, err := driverMajorFromArg(ctx, positional[0])
	if err != nil {
		return err
	}

	port, err := parseDriverPort(flagVals["port"])
	if err != nil {
		return err
	}

	detach := flagVals["detach"] == "true"
	proc, err := ctx.Driver.Start(driver.StartOptions{
		Major:      major,
		Port:       port,
		AllowedIPs: flagVals["allowed-ips"],
		LogFile:    flagVals["log"],
		Detach:     detach,
	})
	if err != nil {
		return err
	}

	ctx.Printf("WebDriver: %s\n", proc.URL)
	ctx.Printf("PID:       %d\n", proc.Pid)

	if detach {
		return nil
	}
	ctx.Println("按 Ctrl+C 结束驱动进程。")
	return proc.Wait()
}

// --- driver uninstall ---

func NewDriverUninstallCommand() *Command {
	return &Command{
		Name:        "uninstall",
		Aliases:     []string{"rm", "remove"},
		Description: "卸载指定主版本的驱动",
		Usage:       "bws driver uninstall <chrome@版本|主版本> [选项]",
		Examples: []string{
			"driver uninstall 120",
			"driver uninstall chrome@120 --force",
		},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "跳过确认直接卸载", HasValue: false, Default: "false"},
		},
		Run: runDriverUninstall,
	}
}

func runDriverUninstall(ctx *Context, args []string) error {
	flags := []*Flag{
		{Name: "force", Short: "f", Usage: "跳过确认直接卸载", HasValue: false, Default: "false"},
	}
	flagVals, positional, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}
	if err := checkFeature(ctx.Driver, "驱动管理"); err != nil {
		return err
	}
	if len(positional) == 0 {
		return fmt.Errorf("请指定要卸载的驱动版本，例如 'bws driver uninstall 120'")
	}

	major, err := driverMajorFromArg(ctx, positional[0])
	if err != nil {
		return err
	}

	if flagVals["force"] != "true" && !ctx.Confirm(fmt.Sprintf("确认卸载 chromedriver %s？", major)) {
		ctx.Println("已取消。")
		return nil
	}

	if err := ctx.Driver.Uninstall(major); err != nil {
		return err
	}
	ctx.Printf("✓ 已卸载 chromedriver %s\n", major)
	return nil
}

// --- helpers ---

// resolveDriverChromeVersion turns a user-supplied spec into a concrete Chrome
// version to match a driver against. A locally installed build wins, so that
// the driver matches the exact patch release the user actually runs.
func resolveDriverChromeVersion(ctx *Context, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("版本不能为空")
	}

	spec := resolveSpec(ctx, arg, "chrome")
	switch spec.Browser {
	case "chrome", "chromium":
	default:
		return "", fmt.Errorf("驱动管理目前仅支持 Chrome/Chromium，收到: %s", spec.Browser)
	}

	// Prefer the exact installed version when the spec matches a local build.
	if ctx.Install != nil {
		if v, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version); err == nil && v != "" {
			return v, nil
		}
	}

	if spec.IsAlias {
		return "", fmt.Errorf("无法确定 Chrome 版本 %q，请显式指定，例如 'chrome@120'", arg)
	}
	if !looksLikeVersion(spec.Version) {
		return "", fmt.Errorf("无法解析 Chrome 版本: %q", arg)
	}
	return spec.Version, nil
}

// driverMajorFromArg derives the driver install directory key (major version)
// from a version spec.
func driverMajorFromArg(ctx *Context, arg string) (string, error) {
	chromeVersion, err := resolveDriverChromeVersion(ctx, arg)
	if err != nil {
		return "", err
	}
	major := version.Major(chromeVersion)
	if major == 0 {
		return "", fmt.Errorf("无法解析主版本号: %q", arg)
	}
	return strconv.Itoa(major), nil
}

// driverProgressPrinter renders download progress on a single line.
func driverProgressPrinter(ctx *Context) func(downloaded, total int64, percent float64) {
	last := -1
	done := false
	return func(downloaded, total int64, percent float64) {
		p := int(percent)
		if p == last {
			return
		}
		last = p
		if total > 0 {
			fmt.Fprintf(ctx.Stdout, "\r  下载中: %3d%% (%s / %s)", p, FormatSize(downloaded), FormatSize(total))
		} else {
			fmt.Fprintf(ctx.Stdout, "\r  下载中: %s", FormatSize(downloaded))
		}
		if p >= 100 && !done {
			done = true
			fmt.Fprintln(ctx.Stdout)
		}
	}
}

// parseDriverPort parses the --driver-port value.
func parseDriverPort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 0 || port > 65535 {
		return 0, fmt.Errorf("无效的驱动端口: %q", raw)
	}
	return port, nil
}

// startAutomationDriver ensures and starts a driver matching the resolved
// browser version. Only Chromium-family browsers are supported, since
// chromedriver speaks exclusively to Chrome/Chromium.
func startAutomationDriver(ctx *Context, spec browserVersionSpec, port int, detach, noDownload bool) (*driver.Process, error) {
	if ctx.Driver == nil {
		return nil, fmt.Errorf("当前构建不支持驱动管理")
	}
	switch spec.Browser {
	case "chrome", "chromium":
	default:
		return nil, fmt.Errorf("暂不支持为 %s 自动管理驱动（仅支持 Chrome/Chromium）", spec.Browser)
	}

	major := version.Major(spec.Version)
	if major == 0 {
		return nil, fmt.Errorf("无法解析浏览器版本: %q", spec.Version)
	}
	majorKey := strconv.Itoa(major)

	if noDownload {
		if findInstalledDriver(ctx, majorKey) == nil {
			return nil, fmt.Errorf("未安装 chromedriver %s，且已禁用自动下载", majorKey)
		}
	} else {
		rec, err := ctx.Driver.Ensure(spec.Version, false, driverProgressPrinter(ctx))
		if err != nil {
			return nil, err
		}
		if ctx.Logger != nil {
			ctx.Logger.Debug("[run] chromedriver 已就绪: %s (目录 %s)", rec.Version, rec.Dir)
		}
	}

	return ctx.Driver.Start(driver.StartOptions{
		Major:  majorKey,
		Port:   port,
		Detach: detach,
	})
}

// findInstalledDriver returns the installed driver record for a major version.
func findInstalledDriver(ctx *Context, major string) *driver.Record {
	records, err := ctx.Driver.List()
	if err != nil {
		return nil
	}
	for i := range records {
		if records[i].MajorVersion == major {
			return &records[i]
		}
	}
	return nil
}