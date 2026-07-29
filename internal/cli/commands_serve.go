package cli

import "fmt"

func runServe(ctx *Context, args []string) error {
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
	if err := ctx.Serve.StartFromConfig(); err != nil {
		return fmt.Errorf("启动服务端失败: %w", err)
	}

	return nil
}
