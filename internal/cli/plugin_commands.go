package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bws/bws/internal/plugin"
)

// NewPluginCommand creates the plugin command.
func NewPluginCommand() *Command {
	return &Command{
		Name:        "plugin",
		Aliases:     []string{"plugins", "pl"},
		Description: "插件管理",
		Usage:       "plugin <subcommand> [args]",
		SubCommands: []*Command{
			NewPluginListCommand(),
			NewPluginInstallCommand(),
			NewPluginUninstallCommand(),
			NewPluginSearchCommand(),
		},
	}
}

// NewPluginListCommand creates the plugin list subcommand.
func NewPluginListCommand() *Command {
	return &Command{
		Name:        "list",
		Aliases:     []string{"ls", "l"},
		Description: "列出已安装的插件",
		Run:         runPluginList,
	}
}

func runPluginList(ctx *Context, args []string) error {
	if ctx.Plugin == nil {
		return fmt.Errorf("plugin manager not available")
	}
	plugins := ctx.Plugin.List()
	if len(plugins) == 0 {
		ctx.Println("没有已安装的插件")
		return nil
	}

	w := tabwriter.NewWriter(ctx.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tTYPE\tSOURCE\tINSTALLED")
	for _, p := range plugins {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			p.Name, p.Version, p.Type, p.Source,
			p.InstalledAt.Format("2006-01-02"))
	}
	return w.Flush()
}

// NewPluginInstallCommand creates the plugin install subcommand.
func NewPluginInstallCommand() *Command {
	return &Command{
		Name:        "install",
		Aliases:     []string{"i", "add"},
		Description: "安装插件",
		Usage:       "plugin install <name|url|path>",
		Examples: []string{
			"plugin install fingerprint-enhanced",
			"plugin install https://example.com/plugin.lua",
			"plugin install ./my-plugin.lua",
		},
		Run: runPluginInstall,
	}
}

func runPluginInstall(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws plugin install <name|url|path>")
	}
	source := args[0]

	// Local file path
	if strings.HasSuffix(source, ".lua") && fileExists(source) {
		name := strings.TrimSuffix(filepath.Base(source), ".lua")
		dest := filepath.Join(ctx.Plugin.PluginsDir(), name+".lua")
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		if err := ctx.Plugin.Install(plugin.ManifestEntry{
			Name:        name,
			Version:     "local",
			Source:      source,
			Type:        "lua",
			InstalledAt: time.Now(),
			Path:        dest,
		}); err != nil {
			return err
		}
		ctx.Printf("插件 %q 已安装 (来源: %s)\n", name, source)
		return nil
	}

	// Registry install
	if ctx.Plugin == nil {
		return fmt.Errorf("plugin manager not available")
	}
	client := plugin.NewRegistryClient(filepath.Join(ctx.Plugin.PluginsDir(), ".cache"))
	entry, err := client.Get(source)
	if err != nil {
		return fmt.Errorf("查找插件失败: %w", err)
	}

	ver, ok := entry.Versions[entry.Latest]
	if !ok {
		return fmt.Errorf("插件 %q 没有可用版本", source)
	}

	data, err := client.Download(ver.URL)
	if err != nil {
		return fmt.Errorf("下载插件失败: %w", err)
	}

	dest := filepath.Join(ctx.Plugin.PluginsDir(), entry.Name+".lua")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}

	if err := ctx.Plugin.Install(plugin.ManifestEntry{
		Name:        entry.Name,
		Version:     entry.Latest,
		Source:      entry.Source,
		Type:        entry.Type,
		InstalledAt: time.Now(),
		Path:        dest,
	}); err != nil {
		return err
	}

	ctx.Printf("插件 %q (v%s) 已安装\n", entry.Name, entry.Latest)
	return nil
}

// NewPluginUninstallCommand creates the plugin uninstall subcommand.
func NewPluginUninstallCommand() *Command {
	return &Command{
		Name:        "uninstall",
		Aliases:     []string{"rm", "remove", "del"},
		Description: "卸载插件",
		Usage:       "plugin uninstall <name>",
		Run:         runPluginUninstall,
	}
}

func runPluginUninstall(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws plugin uninstall <name>")
	}
	name := args[0]
	if ctx.Plugin == nil {
		return fmt.Errorf("plugin manager not available")
	}
	if err := ctx.Plugin.Uninstall(name); err != nil {
		return err
	}
	ctx.Printf("插件 %q 已卸载\n", name)
	return nil
}

// NewPluginSearchCommand creates the plugin search subcommand.
func NewPluginSearchCommand() *Command {
	return &Command{
		Name:        "search",
		Aliases:     []string{"find", "s"},
		Description: "搜索插件",
		Usage:       "plugin search <query>",
		Run:         runPluginSearch,
	}
}

func runPluginSearch(ctx *Context, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	client := plugin.NewRegistryClient(filepath.Join(ctx.Plugin.PluginsDir(), ".cache"))
	results, err := client.Search(query)
	if err != nil {
		return fmt.Errorf("搜索失败: %w", err)
	}
	if len(results) == 0 {
		ctx.Println("未找到匹配的插件")
		return nil
	}

	w := tabwriter.NewWriter(ctx.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tAUTHOR\tLATEST")
	for _, r := range results {
		desc := r.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, desc, r.Author, r.Latest)
	}
	return w.Flush()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
