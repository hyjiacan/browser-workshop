package cli

import (
	"fmt"
)

func runServe(ctx *Context, args []string) error {
	// Parse flags
	flags := []*Flag{
		{Name: "dir", Short: "d", Usage: "基础目录（包含 packages/ 和 bin/ 子目录，默认: 程序所在目录）", HasValue: true, Default: ""},
	}
	flagVals, _, err := ParseFlags(args, flags)
	if err != nil {
		return err
	}

	baseDir := flagVals["dir"]

	// Ensure the config file exists (create a default one if missing).
	configPath, created, err := ctx.Serve.EnsureDefaultConfig("")
	if err != nil {
		return fmt.Errorf("准备服务配置失败: %w", err)
	}
	if created {
		if ctx.Logger != nil {
			ctx.Logger.Debug("[serve] 创建默认配置: %s", configPath)
		}
	}

	// Startup logging is handled by the serve package itself.
	if err := ctx.Serve.StartFromConfig(baseDir); err != nil {
		return fmt.Errorf("启动服务端失败: %w", err)
	}

	return nil
}
