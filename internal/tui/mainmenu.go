package tui

import (
	"fmt"
)

// MainMenu 是 TUI 的主菜单视图，显示已安装浏览器列表
type MainMenu struct {
	app     App
	listBox *ListBox
	browsers []BrowserInfo
	defaultBrowser string
	status  string // 操作反馈信息
}

// NewMainMenu 创建主菜单视图
func NewMainMenu(app App) *MainMenu {
	menu := &MainMenu{
		app: app,
	}
	menu.listBox = NewListBox(nil)
	menu.listBox.SetFocus(true)
	return menu
}

// buildMenuItems 从已安装浏览器列表构建菜单项
func (m *MainMenu) buildMenuItems() []MenuItem {
	if len(m.browsers) == 0 {
		return nil
	}
	items := make([]MenuItem, len(m.browsers))
	for i, b := range m.browsers {
		prefix := "  "
		if b.IsSystem {
			prefix = "S "
		}
		if b.Browser == m.defaultBrowser {
			prefix = "\u2605 " // ★
		}

		label := fmt.Sprintf("%s%s %s", prefix, b.Browser, b.Version)
		subLabel := ""
		if b.Size > 0 {
			subLabel = FormatSize(b.Size)
		}
		if b.IsSystem {
			subLabel += " [系统]"
		}
		if b.Source != "" && !b.IsSystem {
			subLabel += fmt.Sprintf(" [%s]", b.Source)
		}

		idx := i
		items[i] = MenuItem{
			Label:    label,
			SubLabel: subLabel,
			Fn: func() error {
				return m.cmdLaunch(idx)
			},
		}
	}
	return items
}

// Handle 处理主菜单事件
func (m *MainMenu) Handle(e Event) Result {
	switch e.Key {
	case KeyCtrlC:
		return Result{Action: ActionQuit}
	case 'q':
		return Result{Action: ActionQuit}
	case 'v':
		// 进入版本管理面板
		return Result{Action: ActionPush, View: NewVersionPanel(m.app)}
	case 'c':
		// 进入配置管理面板
		return Result{Action: ActionPush, View: NewConfigPanel(m.app)}
	case 'p':
		// 进入插件管理面板
		return Result{Action: ActionPush, View: NewPluginPanel(m.app)}
	}
	return m.listBox.Handle(e)
}

// cmdLaunch 启动选中的浏览器
func (m *MainMenu) cmdLaunch(idx int) error {
	if idx < 0 || idx >= len(m.browsers) {
		return nil
	}
	b := m.browsers[idx]
	m.status = fmt.Sprintf("正在启动 %s@%s ...", b.Browser, b.Version)
	err := m.app.LaunchBrowser(b.Browser, b.Version)
	if err != nil {
		m.status = fmt.Sprintf("启动失败: %v", err)
		return err
	}
	m.status = fmt.Sprintf("已启动 %s@%s", b.Browser, b.Version)
	return nil
}

// Draw 绘制主菜单
func (m *MainMenu) Draw(r *Renderer) {
	w := r.Width()
	h := r.Height()

	// 标题
	title := "bws - Browser Manager"
	r.PrintCenter(1, title, StyleTitle)

	// 版本
	versionText := fmt.Sprintf("v%s", Version)
	r.PrintRight(1, versionText, StyleDim)

	// 副标题
	subtitle := "\u5df2\u5b89\u88c5\u7684\u6d4f\u89c8\u5668" // 已安装的浏览器
	r.Print(2, 2, subtitle, StyleDim)

	// 分隔线
	r.DrawHLine(3, '=', StyleDim)

	if len(m.browsers) == 0 {
		// 空状态
		emptyMsg := "\u5c1a\u672a\u5b89\u88c5\u6d4f\u89c8\u5668\uff0c\u8bf7\u4f7f\u7528 'bws install' \u5b89\u88c5" // 尚未安装浏览器，请使用 'bws install' 安装
		r.PrintCenter(h/2-2, emptyMsg, StyleWarning)
		r.PrintCenter(h/2, "\u6309 [i] \u5b89\u88c5\u6d4f\u89c8\u5668", StyleDim) // 按 [i] 安装浏览器
	} else {
		// 已安装数量
		countText := fmt.Sprintf("\u5171 %d \u4e2a\u6d4f\u89c8\u5668", len(m.browsers)) // 共 N 个浏览器
		r.Print(2, w-len(countText)-2, countText, StyleDim)

		// 列表区域
		listX := 2
		listY := 5
		listWidth := w - 4
		listHeight := h - 9 // 标题(5行) + 快捷键提示(2行) + 状态栏(2行)

		m.listBox.AdjustScroll(listHeight)
		m.listBox.Draw(r, listX, listY, listWidth, listHeight)
	}

	// 快捷键提示区域
	helpY := h - 5
	r.DrawHLine(helpY, '-', StyleDim)
	helpY++
	if len(m.browsers) > 0 {
		r.Print(2, helpY, "[Enter] \u542f\u52a8  [v] \u7248\u672c\u7ba1\u7406  [c] \u914d\u7f6e  [p] \u63d2\u4ef6", StyleKey) // [Enter] 启动  [v] 版本管理  [c] 配置  [p] 插件
	} else {
		r.Print(2, helpY, "[v] \u7248\u672c\u7ba1\u7406  [c] \u914d\u7f6e  [p] \u63d2\u4ef6", StyleKey) // [v] 版本管理  [c] 配置  [p] 插件
	}

	// 操作反馈
	if m.status != "" {
		helpY++
		// 截断过长的状态信息
		maxStatusW := w - 4
		statusText := m.status
		if len([]rune(statusText)) > maxStatusW {
			statusText = string([]rune(statusText)[:maxStatusW-3]) + "..."
		}
		r.Print(2, helpY, statusText, StyleSuccess)
	}
}

// OnEnter 进入主菜单，刷新数据
func (m *MainMenu) OnEnter() {
	m.refreshData()
	m.status = ""
}

// OnExit 离开主菜单
func (m *MainMenu) OnExit() {
	// 无需清理
}

// refreshData 刷新已安装浏览器数据
func (m *MainMenu) refreshData() {
	m.browsers = m.app.ListInstalled()
	m.defaultBrowser = m.app.GetDefaultBrowser()
	m.listBox.Items = m.buildMenuItems()
	if m.listBox.Cursor >= len(m.listBox.Items) {
		m.listBox.Cursor = 0
	}
	m.listBox.ScrollStart = 0
}
