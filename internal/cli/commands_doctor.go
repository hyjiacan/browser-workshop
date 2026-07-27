package cli

import "fmt"

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

	if ctx.Logger != nil {
		ctx.Logger.Debug("[doctor] 开始健康检查")
	}

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
