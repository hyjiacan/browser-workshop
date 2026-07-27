package cli

import (
	"fmt"
	"strings"
)

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
		{Name: "plugin", Short: "", Usage: "激活的插件（逗号分隔多个）", HasValue: true, Default: ""},
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
	spec := resolveSpec(ctx, positional[0], ctx.Cfg.Defaults.DefaultBrowser())
	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] 解析规格: %s@%s (别名=%v)", spec.Browser, spec.Version, spec.IsAlias)
	}

	// Resolve alias / partial versions (e.g. "system", "latest", "126") uniformly.
	if resolvedVersion, err := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version); err == nil {
		spec.Version = resolvedVersion
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
		} else if ctx.Cfg != nil && ctx.Cfg.Proxy != nil {
			proxyURL = ctx.Cfg.Proxy.GetProxy()
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
		Plugins:     parsePluginList(flagVals["plugin"]),
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] headless=%v incognito=%v newWindow=%v native=%v detached=%v dryRun=%v proxy=%q plugins=%v",
			opts.Headless, opts.Incognito, opts.NewWindow, opts.NativeMode, opts.Detached, opts.DryRun, proxyURL, opts.Plugins)
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
