package tui

import (
	"fmt"
)

// VersionPanel 是版本管理面板
type VersionPanel struct {
	app     App
	listBox *ListBox
	versions []BrowserInfo

	// 浏览器类型 Tab
	browserTabs []string // e.g. ["chrome", "firefox", "chromium"]
	activeTab   int

	// 确认状态
	confirming  bool
	confirmIdx  int

	status      string
}

// NewVersionPanel 创建版本管理面板
func NewVersionPanel(app App) *VersionPanel {
	p := &VersionPanel{
		app:         app,
		browserTabs: []string{"chrome", "firefox", "chromium"},
		activeTab:   0,
	}
	p.listBox = NewListBox(nil)
	p.listBox.SetFocus(true)
	return p
}

// refreshData 刷新当前 Tab 对应的浏览器版本列表
func (p *VersionPanel) refreshData() {
	browser := p.browserTabs[p.activeTab]
	p.versions = p.app.ListInstalledByBrowser(browser)
	p.listBox.Items = p.buildMenuItems()
	if p.listBox.Cursor >= len(p.listBox.Items) {
		p.listBox.Cursor = 0
	}
	p.listBox.ScrollStart = 0
}

// buildMenuItems 构建版本列表菜单项
func (p *VersionPanel) buildMenuItems() []MenuItem {
	if len(p.versions) == 0 {
		return nil
	}
	items := make([]MenuItem, len(p.versions))
	for i, v := range p.versions {
		label := fmt.Sprintf("  %s", v.Version)
		subLabel := ""
		if v.Size > 0 {
			subLabel = FormatSize(v.Size)
		}
		if v.IsSystem {
			label = "S " + v.Version
			subLabel += " [系统]"
		}
		if v.Source != "" && !v.IsSystem {
			subLabel += fmt.Sprintf(" [%s]", v.Source)
		}

		idx := i
		items[i] = MenuItem{
			Label:    label,
			SubLabel: subLabel,
			Fn: func() error {
				return p.cmdLaunch(idx)
			},
		}
	}
	return items
}

// Handle 处理版本管理面板事件
func (p *VersionPanel) Handle(e Event) Result {
	// 确认卸载状态
	if p.confirming {
		switch e.Key {
		case 'y', 'Y', KeyEnter:
			p.confirming = false
			return p.doUninstall(p.confirmIdx)
		case 'n', 'N', KeyEsc, KeyCtrlC:
			p.confirming = false
			p.status = "已取消"
			return Result{Action: ActionRefresh}
		}
		return Result{Action: ActionNone}
	}

	switch e.Key {
	case KeyCtrlC:
		return Result{Action: ActionPop}
	case KeyEsc, 'q':
		return Result{Action: ActionPop}
	case KeyTab, KeyRight:
		// 下一个 Tab
		if p.activeTab < len(p.browserTabs)-1 {
			p.activeTab++
		} else {
			p.activeTab = 0
		}
		p.status = ""
		p.refreshData()
		return Result{Action: ActionRefresh}
	case KeyLeft:
		// 上一个 Tab
		if p.activeTab > 0 {
			p.activeTab--
		} else {
			p.activeTab = len(p.browserTabs) - 1
		}
		p.status = ""
		p.refreshData()
		return Result{Action: ActionRefresh}
	case 'd':
		// 卸载选中版本
		if p.listBox.Cursor >= 0 && p.listBox.Cursor < len(p.versions) {
			v := p.versions[p.listBox.Cursor]
			if v.IsSystem {
				p.status = "系统浏览器无法卸载"
				return Result{Action: ActionRefresh}
			}
			p.confirming = true
			p.confirmIdx = p.listBox.Cursor
			p.status = fmt.Sprintf("确认卸载 %s@%s? [y/N]", v.Browser, v.Version)
			return Result{Action: ActionRefresh}
		}
	}
	return p.listBox.Handle(e)
}

// cmdLaunch 启动选中版本
func (p *VersionPanel) cmdLaunch(idx int) error {
	if idx < 0 || idx >= len(p.versions) {
		return nil
	}
	v := p.versions[idx]
	p.status = fmt.Sprintf("正在启动 %s@%s ...", v.Browser, v.Version)
	err := p.app.LaunchBrowser(v.Browser, v.Version)
	if err != nil {
		p.status = fmt.Sprintf("启动失败: %v", err)
		return err
	}
	p.status = fmt.Sprintf("已启动 %s@%s", v.Browser, v.Version)
	return nil
}

// doUninstall 执行卸载操作
func (p *VersionPanel) doUninstall(idx int) Result {
	if idx < 0 || idx >= len(p.versions) {
		return Result{Action: ActionNone}
	}
	v := p.versions[idx]
	p.status = fmt.Sprintf("正在卸载 %s@%s ...", v.Browser, v.Version)
	err := p.app.UninstallBrowser(v.Browser, v.Version)
	if err != nil {
		p.status = fmt.Sprintf("卸载失败: %v", err)
		return Result{Action: ActionRefresh}
	}
	p.status = fmt.Sprintf("已卸载 %s@%s", v.Browser, v.Version)
	p.refreshData()
	return Result{Action: ActionRefresh}
}

// Draw 绘制版本管理面板
func (p *VersionPanel) Draw(r *Renderer) {
	w := r.Width()
	h := r.Height()

	// 标题
	r.PrintCenter(1, "\u7248\u672c\u7ba1\u7406", StyleTitle) // 版本管理

	// 浏览器类型 Tab 栏
	tabY := 3
	tabX := 2
	for i, tab := range p.browserTabs {
		if i == p.activeTab {
			r.Printf(tabX, tabY, "[%s]", StyleBold, tab)
		} else {
			r.Printf(tabX, tabY, " %s ", StyleDim, tab)
		}
		tabX += len(tab) + 4
	}

	// 左右箭头提示
	r.Print(tabX, tabY, "\u2190 \u2192 \u5207\u6362", StyleDim) // ← → 切换

	// 分隔线
	r.DrawHLine(4, '-', StyleDim)

	// 版本列表
	listX := 2
	listY := 6
	listWidth := w - 4
	listHeight := h - 10 // 标题(6行) + 底部提示(2行) + 状态栏(2行)

	if len(p.versions) == 0 {
		emptyMsg := fmt.Sprintf("\u6ca1\u6709\u5df2\u5b89\u88c5\u7684 %s \u7248\u672c", p.browserTabs[p.activeTab]) // 没有已安装的 X 版本
		r.PrintCenter(h/2-2, emptyMsg, StyleWarning)
	} else {
		// 数量显示
		countText := fmt.Sprintf("\u5171 %d \u4e2a\u7248\u672c", len(p.versions)) // 共 N 个版本
		r.PrintRight(3, countText, StyleDim)

		p.listBox.AdjustScroll(listHeight)
		p.listBox.Draw(r, listX, listY, listWidth, listHeight)
	}

	// 底部状态栏
	statusY := h - 4
	r.DrawHLine(statusY, '-', StyleDim)
	r.Print(2, statusY+1, "[Enter] \u542f\u52a8  [d] \u5378\u8f7d  [Tab] \u5207\u6362\u6d4f\u89c8\u5668  [q] \u8fd4\u56de", StyleKey)
	// [Enter] 启动  [d] 卸载  [Tab] 切换浏览器  [q] 返回

	// 操作反馈
	if p.status != "" {
		statusStyle := StyleSuccess
		if p.confirming {
			statusStyle = StyleWarning
		}
		r.Print(2, statusY+2, p.status, statusStyle)
	}
}

// OnEnter 进入版本管理面板
func (p *VersionPanel) OnEnter() {
	p.confirming = false
	p.status = ""
	p.refreshData()
}

// OnExit 离开版本管理面板
func (p *VersionPanel) OnExit() {
	p.confirming = false
}
