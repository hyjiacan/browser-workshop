package tui

import (
	"fmt"
)

// ConfigPanel 是配置管理面板
type ConfigPanel struct {
	app     App
	listBox *ListBox
	items   []ConfigItem

	// 编辑状态
	editing      bool
	editIdx      int
	editOptions  []string // 枚举选项
	editCursor   int      // 当前枚举选项光标
	status       string
}

// NewConfigPanel 创建配置管理面板
func NewConfigPanel(app App) *ConfigPanel {
	p := &ConfigPanel{
		app: app,
	}
	p.listBox = NewListBox(nil)
	p.listBox.SetFocus(true)
	return p
}

// refreshData 刷新配置数据
func (p *ConfigPanel) refreshData() {
	p.items = p.app.GetConfigItems()
	p.listBox.Items = p.buildMenuItems()
	if p.listBox.Cursor >= len(p.listBox.Items) {
		p.listBox.Cursor = 0
	}
	p.listBox.ScrollStart = 0
}

// buildMenuItems 构建配置列表菜单项
func (p *ConfigPanel) buildMenuItems() []MenuItem {
	if len(p.items) == 0 {
		return nil
	}
	items := make([]MenuItem, len(p.items))
	for i, item := range p.items {
		label := fmt.Sprintf("  %s", item.Label)
		subLabel := item.Value
		if subLabel == "" {
			subLabel = "(空)"
		}

		idx := i
		items[i] = MenuItem{
			Label:    label,
			SubLabel: subLabel,
			Fn: func() error {
				return p.cmdEdit(idx)
			},
		}
	}
	return items
}

// Handle 处理配置管理面板事件
func (p *ConfigPanel) Handle(e Event) Result {
	// 编辑枚举状态
	if p.editing {
		switch e.Key {
		case KeyLeft:
			if p.editCursor > 0 {
				p.editCursor--
			}
			return Result{Action: ActionRefresh}
		case KeyRight:
			if p.editOptions != nil && p.editCursor < len(p.editOptions)-1 {
				p.editCursor++
			}
			return Result{Action: ActionRefresh}
		case KeyEnter:
			// 确认选择
			p.editing = false
			if p.editOptions != nil && p.editIdx < len(p.items) {
				newValue := p.editOptions[p.editCursor]
				p.status = fmt.Sprintf("正在保存 %s = %s ...", p.items[p.editIdx].Label, newValue)
				err := p.app.SetConfig(p.items[p.editIdx].Key, newValue)
				if err != nil {
					p.status = fmt.Sprintf("保存失败: %v", err)
				} else {
					p.status = fmt.Sprintf("已保存 %s = %s", p.items[p.editIdx].Label, newValue)
					p.refreshData()
				}
			}
			return Result{Action: ActionRefresh}
		case KeyEsc, KeyCtrlC, 'q':
			p.editing = false
			p.status = "已取消"
			return Result{Action: ActionRefresh}
		}
		// 对于枚举类型，支持字母快速选择
		if e.Key == KeyRune {
			r := e.Rune
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			if p.editOptions != nil {
				for j, opt := range p.editOptions {
					if len(opt) > 0 {
						first := opt[0]
						if first >= 'A' && first <= 'Z' {
							first += 'a' - 'A'
						}
						if first == byte(r) {
							p.editCursor = j
							return Result{Action: ActionRefresh}
						}
					}
				}
			}
		}
		return Result{Action: ActionNone}
	}

	switch e.Key {
	case KeyCtrlC:
		return Result{Action: ActionPop}
	case KeyEsc, 'q':
		return Result{Action: ActionPop}
	}
	return p.listBox.Handle(e)
}

// cmdEdit 编辑配置项
func (p *ConfigPanel) cmdEdit(idx int) error {
	if idx < 0 || idx >= len(p.items) {
		return nil
	}
	item := p.items[idx]

	// 对于枚举类型，进入枚举选择模式
	if item.Type == "enum" && len(item.EnumOptions) > 0 {
		p.editing = true
		p.editIdx = idx
		p.editOptions = item.EnumOptions
		// 找到当前值对应的光标位置
		p.editCursor = 0
		for j, opt := range item.EnumOptions {
			if opt == item.Value {
				p.editCursor = j
				break
			}
		}
		p.status = fmt.Sprintf("选择 %s: \u2190 \u2192 \u5207\u6362, Enter \u786e\u8ba4", item.Label) // 选择 X: ← → 切换, Enter 确认
		return nil
	}

	// 对于布尔类型，切换值
	if item.Type == "bool" {
		newValue := "true"
		if item.Value == "true" {
			newValue = "false"
		}
		p.status = fmt.Sprintf("正在保存 %s = %s ...", item.Label, newValue)
		err := p.app.SetConfig(item.Key, newValue)
		if err != nil {
			p.status = fmt.Sprintf("保存失败: %v", err)
		} else {
			p.status = fmt.Sprintf("已保存 %s = %s", item.Label, newValue)
			p.refreshData()
		}
		return nil
	}

	// 字符串/整数类型暂不支持在 TUI 内编辑（需要文本输入框）
	p.status = fmt.Sprintf("%s (\u7c7b\u578b: %s) \u8bf7\u4f7f\u7528 CLI \u4fee\u6539", item.Label, item.Type)
	// X (类型: T) 请使用 CLI 修改
	return nil
}

// Draw 绘制配置管理面板
func (p *ConfigPanel) Draw(r *Renderer) {
	w := r.Width()
	h := r.Height()

	// 标题
	r.PrintCenter(1, "\u914d\u7f6e\u7ba1\u7406", StyleTitle) // 配置管理

	// 分隔线
	r.DrawHLine(3, '=', StyleDim)

	if len(p.items) == 0 {
		emptyMsg := "\u65e0\u914d\u7f6e\u9879" // 无配置项
		r.PrintCenter(h/2-2, emptyMsg, StyleWarning)
	} else {
		// 数量显示
		countText := fmt.Sprintf("\u5171 %d \u9879", len(p.items)) // 共 N 项
		r.PrintRight(2, countText, StyleDim)

		// 列表区域
		listX := 2
		listY := 5
		listWidth := w - 4
		listHeight := h - 10

		p.listBox.AdjustScroll(listHeight)
		p.listBox.Draw(r, listX, listY, listWidth, listHeight)
	}

	// 枚举选择下拉提示
	if p.editing && p.editOptions != nil {
		optY := h - 8
		r.DrawHLine(optY, '-', StyleDim)
		r.Print(2, optY+1, "\u53ef\u9009\u503c:", StyleBold) // 可选值:
		optX := 10
		for j, opt := range p.editOptions {
			if j == p.editCursor {
				r.Printf(optX, optY+1, "[%s]", StyleBold, opt)
			} else {
				r.Printf(optX, optY+1, " %s ", StyleDim, opt)
			}
			optX += len(opt) + 4
		}
	}

	// 底部状态栏
	statusY := h - 4
	r.DrawHLine(statusY, '-', StyleDim)

	var helpText string
	if p.editing {
		helpText = "[\u2190\u2192] \u5207\u6362  [Enter] \u786e\u8ba4  [Esc] \u53d6\u6d88" // [←→] 切换  [Enter] 确认  [Esc] 取消
	} else {
		helpText = "[Enter] \u4fee\u6539  [\u2191\u2193] \u79fb\u52a8  [q] \u8fd4\u56de" // [Enter] 修改  [↑↓] 移动  [q] 返回
	}
	r.Print(2, statusY+1, helpText, StyleKey)

	// 操作反馈
	if p.status != "" {
		statusStyle := StyleSuccess
		if p.editing {
			statusStyle = StyleWarning
		}
		r.Print(2, statusY+2, p.status, statusStyle)
	}
}

// OnEnter 进入配置管理面板
func (p *ConfigPanel) OnEnter() {
	p.editing = false
	p.status = ""
	p.refreshData()
}

// OnExit 离开配置管理面板
func (p *ConfigPanel) OnExit() {
	p.editing = false
}
