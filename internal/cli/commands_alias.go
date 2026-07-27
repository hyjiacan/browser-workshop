package cli

import (
	"fmt"
	"sort"
	"strings"
)

func runAliasList(ctx *Context, args []string) error {
	aliases := ctx.Cfg.Aliases.ListAliases()

	if len(aliases) == 0 {
		ctx.Println("没有配置别名。")
		ctx.Println("使用 'bws alias add <名称> <浏览器@版本>' 添加别名。")
		return nil
	}

	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := [][]string{}
	for _, name := range names {
		target := aliases[name]
		// target 格式为 "browser@version"
		parts := strings.SplitN(target, "@", 2)
		browser := parts[0]
		ver := ""
		if len(parts) == 2 {
			ver = parts[1]
		}
		rows = append(rows, []string{name, browser, ver})
	}
	PrintTable(ctx.Stdout, []string{"别名", "浏览器", "版本"}, rows)
	return nil
}

func runAliasAdd(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: bws alias add <名称> <浏览器@版本>")
	}

	name := args[0]
	target := args[1]

	if err := ctx.Cfg.Aliases.AddAlias(name, target); err != nil {
		return fmt.Errorf("添加别名失败: %w", err)
	}

	ctx.Printf("别名已添加: %s -> %s\n", name, target)
	return nil
}

func runAliasRemove(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws alias rm <名称>")
	}

	name := args[0]
	if err := ctx.Cfg.Aliases.RemoveAlias(name); err != nil {
		return fmt.Errorf("删除别名失败: %w", err)
	}

	ctx.Printf("别名已删除: %s\n", name)
	return nil
}
