package tui

import (
	"fmt"
	"os"
	"os/signal"
)

// Version 是 TUI 显示的版本号，由 main 包设置
var Version = "unknown"

// BrowserInfo 浏览器信息
type BrowserInfo struct {
	Browser  string
	Version  string
	Size     int64
	IsSystem bool
	Source   string
}

// ConfigItem 配置项
type ConfigItem struct {
	Key         string   // 配置键
	Label       string   // 显示名称
	Value       string   // 当前值
	Type        string   // "string", "enum", "int", "bool"
	EnumOptions []string // 枚举类型的可选值
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name    string
	Type    string // "lua" or "binary"
	Version string
	Source  string
}

// App 定义 TUI 面板需要的所有操作接口
type App interface {
	ListInstalled() []BrowserInfo
	ListInstalledByBrowser(browser string) []BrowserInfo
	GetDefaultBrowser() string
	LaunchBrowser(browser, version string) error
	UninstallBrowser(browser, version string) error
	GetConfigItems() []ConfigItem
	SetConfig(key, value string) error
	ListPlugins() []PluginInfo
	UninstallPlugin(name string) error
}

// Run 启动 TUI 主循环
func Run(app App) error {
	cleanup, err := enterRawMode()
	if err != nil {
		return fmt.Errorf("进入终端原始模式失败: %w", err)
	}
	defer cleanup()

	// 设置 SIGWINCH 处理（Unix）
	setupResizeHandler()

	renderer := NewRenderer()
	renderer.UpdateSize()

	t := &TUI{
		renderer: renderer,
	}

	// 设置版本号
	if Version != "" {
		// already set via package-level var
	}

	// 创建主菜单视图
	root := NewMainMenu(app)
	t.nav = NewNavigator(root)

	// 主循环
	for {
		// 更新终端大小（处理窗口大小变化）
		renderer.UpdateSize()

		// 绘制
		renderer.Clear()
		renderer.HideCursor()
		t.nav.Draw(renderer)
		t.drawStatusBar(renderer)
		renderer.ShowCursor()
		renderer.Flush(os.Stdout)

		// 读取事件
		ev := ReadEvent()

		// 处理窗口大小变化
		if ev.Type == EventResize {
			renderer.UpdateSize()
			continue
		}

		// 处理错误事件
		if ev.Type == EventError {
			return fmt.Errorf("终端事件错误: %w", ev.Err)
		}

		// 处理按键事件
		result := t.nav.Handle(ev)

		switch result.Action {
		case ActionQuit:
			// 清屏后退出
			renderer.Clear()
			renderer.Flush(os.Stdout)
			return nil

		case ActionExec:
			if result.Fn != nil {
				// 临时退出 raw mode 执行外部函数
				cleanup()
				err := result.Fn()
				// 重新进入 raw mode
				enterRawMode()
				if IsQuitError(err) {
					// 清屏后退出
					renderer.Clear()
					renderer.Flush(os.Stdout)
					return nil
				}
				if err != nil {
					// 显示错误后继续
					continue
				}
			}

		case ActionRefresh:
			continue
		}
	}
}

// TUI 是终端用户界面主结构
type TUI struct {
	renderer *Renderer
	nav      *Navigator
}

// drawStatusBar 绘制底部状态栏
func (t *TUI) drawStatusBar(r *Renderer) {
	y := r.Height() - 2

	// 状态栏分隔线
	r.DrawHLine(y, '-', StyleDim)

	// 状态栏内容
	statusY := r.Height() - 1
	r.Print(0, statusY, " ESC:退出  \u2191\u2193:\u79fb\u52a8  Enter:\u9009\u62e9", StyleDim)
	versionText := fmt.Sprintf("bws %s", Version)
	r.PrintRight(statusY, versionText, StyleDim)
}

// setupResizeHandler 设置窗口大小变化信号处理器
func setupResizeHandler() {
	if !isWindows() {
		setupSigWinchHandler()
	}
}

// setupSigWinchHandler 在 Unix 平台设置 SIGWINCH 信号处理
func setupSigWinchHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sigWinchSignal)
	go func() {
		for range sigCh {
			// SIGWINCH 信号被触发，主循环会在下一次迭代中更新大小
		}
	}()
}

// FormatSize 格式化文件大小（tui 包内部使用，避免循环导入 cli 包）
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// isQuitCmd 标记退出命令（供主循环识别）
var isQuitCmd = fmt.Errorf("QUIT")

// MakeQuitFunc 返回一个执行后会触发退出的函数
func MakeQuitFunc() func() error {
	return func() error {
		return isQuitCmd
	}
}

// IsQuitError 判断错误是否为退出信号
func IsQuitError(err error) bool {
	return err == isQuitCmd
}
