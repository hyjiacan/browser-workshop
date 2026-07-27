package cli

import (
	"fmt"
)

func runInfo(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定版本，例如 'bws show chrome@120'")
	}

	spec := resolveFirstArg(ctx, args)
	if ctx.Logger != nil {
		ctx.Logger.Debug("[info] 查询: %s@%s", spec.Browser, spec.Version)
	}

	// Resolve partial / alias versions against installed + system versions.
	resolvedVersion, localErr := ctx.Install.ResolveInstalledVersion(spec.Browser, spec.Version)
	if localErr == nil {
		// Local version found — show installed or system info
		isSystem := ctx.Install.IsSystemVersion(spec.Browser, resolvedVersion)
		if record, err := ctx.Install.GetRecord(spec.Browser, resolvedVersion); err == nil && !isSystem {
			ctx.Printf("%s@%s\n", record.Browser, record.Version)
			ctx.Printf("  平台:         %s\n", record.Platform)
			ctx.Printf("  架构:         %s\n", record.Arch)
			ctx.Printf("  安装时间:     %s\n", record.InstalledAt.Format("2006-01-02 15:04:05"))
			ctx.Printf("  来源:         %s\n", record.Source)
			ctx.Printf("  安装目录:     %s\n", record.InstallDir)
			ctx.Printf("  可执行文件:   %s\n", record.ExecutablePath)
		} else {
			// System version
			ctx.Printf("%s@%s\n", spec.Browser, resolvedVersion)
			ctx.Printf("  类型:     系统安装\n")
			if versions, err := ctx.Install.ListWithSystemByBrowser(spec.Browser); err == nil {
				for _, v := range versions {
					if v.Version == resolvedVersion {
						ctx.Printf("  渠道:     %s\n", v.Channel)
						break
					}
				}
			}
		}

		// Check remote for update
		if ctx.Source != nil {
			if versionInfo, err := ctx.Source.ResolveVersion(spec.Browser, spec.Version); err == nil && versionInfo.Version != "" && versionInfo.Version != resolvedVersion {
				ctx.Printf("\n%s@%s（远程更新可用）\n", versionInfo.Browser, versionInfo.Version)
				ctx.Printf("  渠道:         %s\n", versionInfo.Channel)
				ctx.Printf("  平台:         %s\n", versionInfo.Platform)
				ctx.Printf("  架构:         %s\n", versionInfo.Arch)
				if versionInfo.DownloadURL != "" {
					ctx.Printf("  下载链接:     %s\n", versionInfo.DownloadURL)
				}
				ctx.Printf("\n  使用 'bws i %s@%s' 升级到此版本。\n", spec.Browser, versionInfo.Version)
			}
		}
		return nil
	}

	// Not installed locally — try remote source
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
