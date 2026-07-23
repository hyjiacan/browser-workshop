package tui

import (
	"fmt"
)

// PluginPanel 是插件管理面板
type PluginPanel struct {
	app     App
	listBox *ListBox
	plugins []PluginInfo

	// 确认状态
	confirming bool
	confirmIdx int
	status     string
}

// NewPluginPanel 创建插件管理面板
func NewPluginPanel(app App) *PluginPanel {
	p := &PluginPanel{
		app: app,
	}
	p.listBox = NewListBox(nil)
	p.listBox.SetFocus(true)
	return p
}

// refreshData 刷新插件数据
func (p *PluginPanel) refreshData() {
	p.plugins = p.app.ListPlugins()
	p.listBox.Items = p.buildMenuItems()
	if p.listBox.Cursor >= len(p.listBox.Items) {
		p.listBox.Cursor = 0
	}
	p.listBox.ScrollStart = 0
}

// buildMenuItems 构建插件列表菜单项
func (p *PluginPanel) buildMenuItems() []MenuItem {
	if len(p.plugins) == 0 {
		return nil
	}
	items := make([]MenuItem, len(p.plugins))
	for i, pl := range p.plugins {
		label := fmt.Sprintf("  %s", pl.Name)
		subLabel := ""
		if pl.Type != "" {
			subLabel += pl.Type
		}
		if pl.Version != "" {
			if subLabel != "" {
				subLabel += " "
			}
			subLabel += "v" + pl.Version
		}
		if pl.Source != "" {
			subLabel += fmt.Sprintf(" (%s)", pl.Source)
		}

		items[i] = MenuItem{
			Label:    label,
			SubLabel: subLabel,
			Fn:       nil, // 插件不支持直接启动
		}
	}
	return items
}

// Handle 处理插件管理面板事件
func (p *PluginPanel) Handle(e Event) Result {
	// 确认卸载状态
	if p.confirming {
		switch e.Key {
		case 'y', 'Y', KeyEnter:
			p.confirming = false
			return p.doUninstall(p.confirmIdx)
		case 'n', 'N', KeyEsc, KeyCtrlC:
			p.confirming = false
			p.status = "\u5df2\u53d6\u6d88" // 已取消
			return Result{Action: ActionRefresh}
		}
		return Result{Action: ActionNone}
	}

	switch e.Key {
	case KeyCtrlC:
		return Result{Action: ActionPop}
	case KeyEsc, 'q':
		return Result{Action: ActionPop}
	case 'd':
		// 卸载选中插件
		if p.listBox.Cursor >= 0 && p.listBox.Cursor < len(p.plugins) {
			p.confirming = true
			p.confirmIdx = p.listBox.Cursor
			pl := p.plugins[p.listBox.Cursor]
			p.status = fmt.Sprintf("\u786e\u8ba4\u5378\u8f7d\u63d2\u4ef6 %s? [y/N]", pl.Name) // 确认卸载插件 X? [y/N]
			return Result{Action: ActionRefresh}
		}
	}
	return p.listBox.Handle(e)
}

// doUninstall 执行卸载操作
func (p *PluginPanel) doUninstall(idx int) Result {
	if idx < 0 || idx >= len(p.plugins) {
		return Result{Action: ActionNone}
	}
	pl := p.plugins[idx]
	p.status = fmt.Sprintf("\u6b63\u5728\u5378\u8f7d\u63d2\u4ef6 %s ...", pl.Name) // 正在卸载插件 X ...
	err := p.app.UninstallPlugin(pl.Name)
	if err != nil {
		p.status = fmt.Sprintf("\u5378\u8f7d\u5931\u8d25: %v", err) // 卸载失败
		return Result{Action: ActionRefresh}
	}
	p.status = fmt.Sprintf("\u5df2\u5378\u8f7d\u63d2\u4ef6 %s", pl.Name) // 已卸载插件 X
	p.refreshData()
	return Result{Action: ActionRefresh}
}

// Draw 绘制插件管理面板
func (p *PluginPanel) Draw(r *Renderer) {
	w := r.Width()
	h := r.Height()

	// 标题
	r.PrintCenter(1, "\u63d2\u4ef6\u7ba1\u7406", StyleTitle) // 插件管理

	// 分隔线
	r.DrawHLine(3, '=', StyleDim)

	if len(p.plugins) == 0 {
		emptyMsg := "\u6ca1\u6709\u5df2\u5b89\u88c5\u7684\u63d2\u4ef6" // 没有已安装的插件
		r.PrintCenter(h/2-2, emptyMsg, StyleWarning)
	} else {
		// 数量显示
		countText := fmt.Sprintf("\u5171 %d \u4e2a\u63d2\u4ef6", len(p.plugins)) // 共 N 个插件
		r.PrintRight(2, countText, StyleDim)

		// 列表区域
		listX := 2
		listY := 5
		listWidth := w - 4
		listHeight := h - 9

		p.listBox.AdjustScroll(listHeight)
		p.listBox.Draw(r, listX, listY, listWidth, listHeight)
	}

	// 底部状态栏
	statusY := h - 4
	r.DrawHLine(statusY, '-', StyleDim)
	r.Print(2, statusY+1, "[d] \u5378\u8f7d  [q] \u8fd4\u56de", StyleKey) // [d] 卸载  [q] 返回

	// 操作反馈
	if p.status != "" {
		statusStyle := StyleSuccess
		if p.confirming {
			statusStyle = StyleWarning
		}
		r.Print(2, statusY+2, p.status, statusStyle)
	}
}

// OnEnter 进入插件管理面板
func (p *PluginPanel) OnEnter() {
	p.confirming = false
	p.status = ""
	p.refreshData()
}

// OnExit 离开插件管理面板
func (p *PluginPanel) OnExit() {
	p.confirming = false
}
