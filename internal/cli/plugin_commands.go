package cli

import (
	"fmt"
	"os"
	"os/exec"
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
			NewPluginUpdateCommand(),
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
		Usage:       "plugin install <name|git-url|http-url|path>",
		Examples: []string{
			"plugin install fingerprint-enhanced       # 从注册表安装",
			"plugin install https://gitee.com/user/bws-plugin-foo  # 从 Git 仓库安装",
			"plugin install https://example.com/plugin.lua  # 从 HTTP URL 安装",
			"plugin install ./my-plugin.lua                # 从本地文件安装",
		},
		Run: runPluginInstall,
	}
}

func runPluginInstall(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws plugin install <name|git-url|http-url|path>")
	}
	source := args[0]

	if ctx.Plugin == nil {
		return fmt.Errorf("plugin manager not available")
	}

	// 1. Local file path
	if fileExists(source) {
		return installFromLocalFile(ctx, source)
	}

	// 2. Git repository URL (https://... or git@...)
	if isGitURL(source) {
		return installFromGit(ctx, source)
	}

	// 3. Direct HTTP/HTTPS URL to a plugin file
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return installFromHTTP(ctx, source)
	}

	// 4. Registry name
	return installFromRegistry(ctx, source)
}

// installFromLocalFile installs a plugin from a local file path.
func installFromLocalFile(ctx *Context, source string) error {
	name := filepath.Base(source)
	ext := filepath.Ext(name)
	pluginType := "lua"
	if ext != ".lua" {
		pluginType = "binary"
		// For binary plugins, keep the original filename (including extension)
	} else {
		name = strings.TrimSuffix(name, ".lua")
	}

	dest := filepath.Join(ctx.Plugin.PluginsDir(), filepath.Base(source))
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if pluginType == "binary" {
		perm = 0o755
	}
	if err := os.WriteFile(dest, data, perm); err != nil {
		return err
	}
	if err := ctx.Plugin.Install(plugin.ManifestEntry{
		Name:        name,
		Version:     "local",
		Source:      source,
		Type:        pluginType,
		InstalledAt: time.Now(),
		Path:        dest,
	}); err != nil {
		return err
	}
	ctx.Printf("插件 %q 已安装 (类型: %s, 来源: %s)\n", name, pluginType, source)
	return nil
}

// installFromGit installs a plugin from a Git repository.
// Clones the repo to a temp dir, finds the plugin file, and copies it.
func installFromGit(ctx *Context, repoURL string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("安装 Git 插件需要系统已安装 git，未找到 git 命令")
	}

	// Create temp dir for cloning
	tmpDir, err := os.MkdirTemp("", "bws-plugin-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx.Printf("正在克隆仓库 %s ...\n", repoURL)
	// Shallow clone for speed
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("克隆仓库失败: %w", err)
	}

	// Find plugin file in cloned repo: prefer .lua, then any executable file
	var pluginPath string
	for _, name := range []string{"plugin.lua", "main.lua", "index.lua"} {
		p := filepath.Join(tmpDir, name)
		if fileExists(p) {
			pluginPath = p
			break
		}
	}

	// Fallback: find any .lua file in repo root
	if pluginPath == "" {
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			return fmt.Errorf("读取仓库目录失败: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".lua") {
				pluginPath = filepath.Join(tmpDir, e.Name())
				break
			}
		}
	}

	if pluginPath == "" {
		return fmt.Errorf("在仓库中未找到插件文件（期望 .lua 文件）")
	}

	// Derive plugin name from repo URL
	repoName := filepath.Base(repoURL)
	repoName = strings.TrimSuffix(repoName, ".git")
	repoName = strings.TrimPrefix(repoName, "bws-plugin-")

	ext := filepath.Ext(pluginPath)
	perm := os.FileMode(0o644)
	pluginType := "lua"
	if ext != ".lua" {
		pluginType = "binary"
		perm = 0o755
	}

	dest := filepath.Join(ctx.Plugin.PluginsDir(), repoName+ext)
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("读取插件文件失败: %w", err)
	}
	if err := os.WriteFile(dest, data, perm); err != nil {
		return err
	}
	if err := ctx.Plugin.Install(plugin.ManifestEntry{
		Name:        repoName,
		Version:     "git",
		Source:      repoURL,
		Type:        pluginType,
		InstalledAt: time.Now(),
		Path:        dest,
	}); err != nil {
		return err
	}
	ctx.Printf("插件 %q 已安装 (类型: %s, 来源: %s)\n", repoName, pluginType, repoURL)
	return nil
}

// installFromHTTP installs a plugin from a direct HTTP/HTTPS URL.
func installFromHTTP(ctx *Context, url string) error {
	client := plugin.NewRegistryClient(filepath.Join(ctx.Plugin.PluginsDir(), ".cache"))
	data, err := client.Download(url)
	if err != nil {
		return fmt.Errorf("下载插件失败: %w", err)
	}

	// Determine type from URL path
	name := filepath.Base(url)
	ext := filepath.Ext(name)
	pluginType := "lua"
	perm := os.FileMode(0o644)
	if ext != ".lua" {
		pluginType = "binary"
		perm = 0o755
	} else {
		name = strings.TrimSuffix(name, ".lua")
	}

	dest := filepath.Join(ctx.Plugin.PluginsDir(), filepath.Base(url))
	if err := os.WriteFile(dest, data, perm); err != nil {
		return err
	}
	if err := ctx.Plugin.Install(plugin.ManifestEntry{
		Name:        name,
		Version:     "remote",
		Source:      url,
		Type:        pluginType,
		InstalledAt: time.Now(),
		Path:        dest,
	}); err != nil {
		return err
	}
	ctx.Printf("插件 %q 已安装 (类型: %s, 来源: %s)\n", name, pluginType, url)
	return nil
}

// installFromRegistry installs a plugin from the remote registry.
func installFromRegistry(ctx *Context, name string) error {
	client := plugin.NewRegistryClient(filepath.Join(ctx.Plugin.PluginsDir(), ".cache"))
	entry, err := client.Get(name)
	if err != nil {
		return fmt.Errorf("查找插件失败: %w", err)
	}

	ver, ok := entry.Versions[entry.Latest]
	if !ok {
		return fmt.Errorf("插件 %q 没有可用版本", name)
	}

	// Download with SHA256 hash verification
	data, err := client.DownloadWithHash(ver.URL, ver.Hash)
	if err != nil {
		return fmt.Errorf("下载插件失败: %w", err)
	}

	// Determine destination file name and permissions based on plugin type
	ext := ".lua"
	perm := os.FileMode(0o644)
	if entry.Type == "binary" {
		ext = "" // binary plugins keep no extension by default
		perm = 0o755
	}
	dest := filepath.Join(ctx.Plugin.PluginsDir(), entry.Name+ext)
	if err := os.WriteFile(dest, data, perm); err != nil {
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

// isGitURL checks if the source looks like a Git repository URL.
func isGitURL(source string) bool {
	return strings.HasPrefix(source, "https://gitee.com/") ||
		strings.HasPrefix(source, "https://github.com/") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "git://")
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

// NewPluginUpdateCommand creates the plugin update subcommand.
func NewPluginUpdateCommand() *Command {
	return &Command{
		Name:        "update",
		Aliases:     []string{"up", "u"},
		Description: "更新插件到最新版本",
		Usage:       "plugin update <name>",
		Run:         runPluginUpdate,
	}
}

func runPluginUpdate(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: bws plugin update <name>")
	}
	name := args[0]
	if ctx.Plugin == nil {
		return fmt.Errorf("plugin manager not available")
	}

	// Get current installation info
	current, err := ctx.Plugin.GetManifestEntry(name)
	if err != nil {
		return fmt.Errorf("插件 %q 未安装", name)
	}

	// Only registry-sourced plugins can be updated
	if current.Source == "local" || current.Source == "git" || current.Source == "remote" {
		ctx.Printf("插件 %q 是通过 %s 安装的，请重新安装以更新\n", name, current.Source)
		return nil
	}

	client := plugin.NewRegistryClient(filepath.Join(ctx.Plugin.PluginsDir(), ".cache"))
	entry, err := client.Get(name)
	if err != nil {
		return fmt.Errorf("查找插件失败: %w", err)
	}

	if entry.Latest == current.Version {
		ctx.Printf("插件 %q 已是最新版本 (v%s)\n", name, current.Version)
		return nil
	}

	ver, ok := entry.Versions[entry.Latest]
	if !ok {
		return fmt.Errorf("插件 %q 没有可用版本", name)
	}

	ctx.Printf("正在更新插件 %q: v%s -> v%s ...\n", name, current.Version, entry.Latest)
	data, err := client.DownloadWithHash(ver.URL, ver.Hash)
	if err != nil {
		return fmt.Errorf("下载插件失败: %w", err)
	}

	ext := ".lua"
	perm := os.FileMode(0o644)
	if entry.Type == "binary" {
		ext = ""
		perm = 0o755
	}
	dest := filepath.Join(ctx.Plugin.PluginsDir(), entry.Name+ext)
	if err := os.WriteFile(dest, data, perm); err != nil {
		return err
	}

	// Uninstall old version and reinstall with new version
	if err := ctx.Plugin.Uninstall(name); err != nil {
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

	ctx.Printf("插件 %q 已更新到 v%s\n", name, entry.Latest)
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
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
