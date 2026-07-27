package cli

import "fmt"

func runServe(ctx *Context, args []string) error {
	flags, _, err := ParseFlags(args, []*Flag{
		{Name: "dir", Short: "d", Usage: "基础目录（包含 packages/ 和 bin/ 子目录，默认: 程序所在目录）", HasValue: true, Default: ""},
	})
	if err != nil {
		return err
	}

	dir := flags["dir"]

	// If dir is specified, ensure the config exists
	if dir != "" {
		configPath, created, err := ctx.Serve.EnsureDefaultConfig(dir)
		if err != nil {
			return fmt.Errorf("准备服务目录失败: %w", err)
		}
		if created {
			if ctx.Logger != nil {
				ctx.Logger.Debug("[serve] 创建默认配置: %s", configPath)
			}
		}
	}

	ctx.Printf("启动 bws 服务端...\n")

	if err := ctx.Serve.StartFromConfig(ctx.Cfg.Data.GetDataDir()); err != nil {
		return fmt.Errorf("启动服务端失败: %w", err)
	}

	return nil
}
