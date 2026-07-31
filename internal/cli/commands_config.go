package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func runConfigShow(ctx *Context, args []string) error {
	if ctx.Logger != nil {
		ctx.Logger.Debug("[config] 显示配置: path=%s", ctx.Cfg.Data.ConfigPath())
	}
	ctx.Printf("配置信息：\n\n")
	ctx.Printf("  config-path:         %s\n", ctx.Cfg.Data.ConfigPath())
	ctx.Printf("  data-dir:            %s\n", ctx.Cfg.Data.GetDataDir())
	ctx.Printf("  default-browser:     %s\n", ctx.Cfg.Defaults.DefaultBrowser())
	ctx.Printf("  default-channel:     %s\n", ctx.Cfg.Defaults.DefaultChannel())
	ctx.Printf("  log-level:           %s\n", ctx.Cfg.Log.GetLogLevel())
	ctx.Printf("  repo-path:           %s\n", ctx.Cfg.Repo.GetRepoPath())
	ctx.Printf("\n  数据源开关:\n")
	ctx.Printf("    source-serve:      %s\n", boolStr(ctx.Cfg.Source.IsServeSourceEnabled()))
	ctx.Printf("    source-firefox-ftp:%s\n", boolStr(ctx.Cfg.Source.IsFirefoxFTPEnabled()))
	ctx.Printf("\n  disk-threshold:      %d GB (低于此值会提示)\n", ctx.Cfg.Disk.GetDiskSpaceThresholdGB())

	proxy := ctx.Cfg.Proxy.GetProxy()
	if proxy == "" {
		ctx.Printf("  proxy:               (未设置，直连)\n")
	} else {
		ctx.Printf("  proxy:               %s\n", proxy)
	}

	lang := ctx.Cfg.Language.GetLanguage()
	if lang == "" {
		ctx.Printf("  language:            (自动检测)\n")
	} else {
		ctx.Printf("  language:            %s\n", lang)
	}

	aliases := ctx.Cfg.Aliases.ListAliases()
	if len(aliases) > 0 {
		ctx.Printf("\n  别名:\n")
		for name, target := range aliases {
			ctx.Printf("    %s -> %s\n", name, target)
		}
	}

	ctx.Println()
	ctx.Printf("提示: 您也可以直接编辑配置文件进行高级设置:\n  %s\n", ctx.Cfg.Data.ConfigPath())
	return nil
}

func runConfigGet(ctx *Context, args []string) error {
	if len(args) == 0 {
		printConfigKeyList(ctx.Stdout, readableConfigKeys(), false)
		ctx.Printf("提示: 您也可以直接编辑配置文件进行高级设置:\n  %s\n", ctx.Cfg.Data.ConfigPath())
		return nil
	}

	key := strings.ToLower(args[0])
	switch key {
	case "default-browser", "default", "browser":
		ctx.Println(ctx.Cfg.Defaults.DefaultBrowser())
	case "default-channel", "channel":
		ctx.Println(ctx.Cfg.Defaults.DefaultChannel())
	case "log-level", "log":
		ctx.Println(ctx.Cfg.Log.GetLogLevel())
	case "data-dir", "datadir", "data":
		ctx.Println(ctx.Cfg.Data.GetDataDir())
	case "repo-path", "repo":
		ctx.Println(ctx.Cfg.Repo.GetRepoPath())
	case "source", "remote-source", "remote":
		src := ctx.Cfg.Source.GetRemoteSource()
		if src == "" {
			ctx.Println("（未设置）")
		} else {
			ctx.Println(src)
		}
	case "source-serve", "serve-source":
		ctx.Println(boolStr(ctx.Cfg.Source.IsServeSourceEnabled()))
	case "source-firefox-ftp", "firefox-ftp":
		ctx.Println(boolStr(ctx.Cfg.Source.IsFirefoxFTPEnabled()))
	case "disk-threshold", "disk-space-threshold", "space-threshold":
		ctx.Printf("%d GB\n", ctx.Cfg.Disk.GetDiskSpaceThresholdGB())
	case "proxy":
		p := ctx.Cfg.Proxy.GetProxy()
		if p == "" {
			ctx.Println("(未设置)")
		} else {
			ctx.Println(p)
		}
	case "language", "lang":
		lang := ctx.Cfg.Language.GetLanguage()
		if lang == "" {
			ctx.Println("(自动检测)")
		} else {
			ctx.Println(lang)
		}
	case "path", "config-path", "config":
		ctx.Println(ctx.Cfg.Data.ConfigPath())
	default:
		// Try alias
		if alias, ok := ctx.Cfg.Aliases.GetAlias(args[0]); ok {
			ctx.Println(alias)
			return nil
		}
		return fmt.Errorf("未知的配置键: %s。使用 'bws cfg get' 查看可用配置项。", args[0])
	}
	return nil
}

func runConfigSet(ctx *Context, args []string) error {
	if len(args) == 0 {
		printConfigKeyList(ctx.Stdout, writableConfigKeys(), true)
		ctx.Printf("提示: 您也可以直接编辑配置文件进行高级设置:\n  %s\n", ctx.Cfg.Data.ConfigPath())
		return nil
	}
	if len(args) < 2 {
		printConfigKeyList(ctx.Stdout, writableConfigKeys(), true)
		return fmt.Errorf("请提供配置值")
	}

	key := strings.ToLower(args[0])
	value := args[1]

	switch key {
	case "default-browser", "default", "browser":
		if !ctx.Browsers.Has(value) {
			return fmt.Errorf("未知的浏览器: %s", value)
		}
		if err := ctx.Cfg.Defaults.SetDefaultBrowser(value); err != nil {
			return fmt.Errorf("设置默认浏览器失败: %w", err)
		}
		ctx.Printf("默认浏览器已设置为: %s\n", value)

	case "default-channel", "channel":
		validChannels := map[string]bool{"stable": true, "beta": true, "dev": true, "canary": true, "esr": true}
		if !validChannels[strings.ToLower(value)] {
			return fmt.Errorf("无效的渠道: %s（必须是 stable、beta、dev、canary 或 esr）", value)
		}
		if err := ctx.Cfg.Defaults.SetDefaultChannel(value); err != nil {
			return fmt.Errorf("设置默认渠道失败: %w", err)
		}
		ctx.Printf("默认渠道已设置为: %s\n", value)

	case "log-level", "log":
		level := strings.ToLower(value)
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
		if !validLevels[level] {
			return fmt.Errorf("无效的日志级别: %s（必须是 debug、info、warn 或 error）", value)
		}
		if err := ctx.Cfg.Log.SetLogLevel(level); err != nil {
			return fmt.Errorf("设置日志级别失败: %w", err)
		}
		ctx.Printf("日志级别已设置为: %s\n", level)

	case "data-dir", "datadir", "data":
		if err := ctx.Cfg.Data.SetDataDir(value); err != nil {
			return fmt.Errorf("设置数据目录失败: %w", err)
		}
		ctx.Printf("数据目录已设置为: %s\n", value)
		ctx.Printf("注意: 重启 bws 后生效。\n")

	case "repo-path", "repo":
		if err := ctx.Cfg.Repo.SetRepoPath(value); err != nil {
			return fmt.Errorf("设置仓库路径失败: %w", err)
		}
		ctx.Printf("仓库路径已设置为: %s\n", value)

	case "source", "remote-source", "remote":
		if err := ctx.Cfg.Source.SetRemoteSource(value); err != nil {
			return fmt.Errorf("设置离线源失败: %w", err)
		}
		ctx.Printf("离线源已设置为: %s\n", value)
		ctx.Println("优先级: 离线源 → 在线源（离线源优先）")

	case "source-serve", "serve-source":
		enabled := parseBool(value)
		if err := ctx.Cfg.Source.SetServeSourceEnabled(enabled); err != nil {
			return fmt.Errorf("设置 Serve 源开关失败: %w", err)
		}
		if enabled {
			ctx.Println("Serve 源已启用")
		} else {
			ctx.Println("Serve 源已禁用")
		}

	case "source-firefox-ftp", "firefox-ftp":
		enabled := parseBool(value)
		if err := ctx.Cfg.Source.SetFirefoxFTPEnabled(enabled); err != nil {
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
			return fmt.Errorf("无效的磁盘阈值: %s（必须为正整数）", value)
		}
		if err := ctx.Cfg.Disk.SetDiskSpaceThresholdGB(gb); err != nil {
			return fmt.Errorf("设置磁盘阈值失败: %w", err)
		}
		ctx.Printf("磁盘空间阈值已设置为 %d GB\n", gb)

	case "proxy":
		if strings.ToLower(value) == "none" || value == "" {
			if err := ctx.Cfg.Proxy.SetProxy(""); err != nil {
				return fmt.Errorf("清除代理设置失败: %w", err)
			}
			ctx.Println("代理已清除（直连模式）。")
		} else {
			if err := validateProxyURL(value); err != nil {
				return err
			}
			if err := ctx.Cfg.Proxy.SetProxy(value); err != nil {
				return fmt.Errorf("设置代理失败: %w", err)
			}
			ctx.Printf("代理已设置为: %s\n", value)
		}

	case "language", "lang":
		value = strings.ToLower(value)
		if value != "zh" && value != "en" {
			return fmt.Errorf("不支持的语言: %s（支持 zh、en）", value)
		}
		if err := ctx.Cfg.Language.SetLanguage(value); err != nil {
			return fmt.Errorf("设置语言失败: %w", err)
		}
		ctx.Printf("界面语言已设置为: %s\n", value)

	default:
		return fmt.Errorf("未知的配置项: %s。使用 'bws cfg set' 查看可设置项。", args[0])
	}
	return nil
}
