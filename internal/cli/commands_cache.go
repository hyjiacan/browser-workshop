package cli

func runCacheClear(ctx *Context, args []string) error {
	// Cache is stored in temp directories by default
	// For now, just note that we don't have persistent cache
	ctx.Printf("注意: 下载文件存储在临时目录中，会自动清理。\n")
	ctx.Printf("没有需要清除的持久化缓存。\n")
	return nil
}

func runCacheInfo(ctx *Context, args []string) error {
	if ctx.Logger != nil {
		ctx.Logger.Debug("[cache] 缓存信息: 类型=临时（自动清理）")
	}
	ctx.Printf("缓存状态:\n")
	ctx.Printf("  类型: 临时（自动清理）\n")
	ctx.Printf("  说明: 下载文件存储在临时目录中，安装后会自动清理。\n")
	return nil
}
