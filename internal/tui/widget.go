package tui

import (
	"fmt"
	"strings"
)

// MenuItem 表示菜单中的一个选项
type MenuItem struct {
	Label    string       // 主标签
	SubLabel string       // 辅助说明（灰色显示）
	Fn       func() error // 选中时执行的函数
}

// ListBox 列表选择框组件
type ListBox struct {
	Items       []MenuItem // 菜单项列表
	Cursor      int        // 当前光标位置（索引）
	ScrollStart int        // 滚动起始偏移
	focused     bool       // 是否聚焦
}

// NewListBox 创建新的列表选择框
func NewListBox(items []MenuItem) *ListBox {
	return &ListBox{
		Items:   items,
		Cursor:  0,
	}
}

// SetFocus 设置聚焦状态
func (lb *ListBox) SetFocus(focused bool) {
	lb.focused = focused
}

// Handle 处理列表框事件
func (lb *ListBox) Handle(e Event) Result {
	if len(lb.Items) == 0 {
		return Result{Action: ActionNone}
	}

	switch e.Key {
	case KeyUp, KeyCtrlK:
		if lb.Cursor > 0 {
			lb.Cursor--
			lb.clampScroll()
		}
	case KeyDown, KeyCtrlJ:
		if lb.Cursor < len(lb.Items)-1 {
			lb.Cursor++
			lb.clampScroll()
		}
	case KeyHome, KeyCtrlA:
		lb.Cursor = 0
		lb.ScrollStart = 0
	case KeyEnd, KeyCtrlE:
		lb.Cursor = len(lb.Items) - 1
		lb.clampScroll()
	case KeyPageUp:
		lb.Cursor -= 5
		if lb.Cursor < 0 {
			lb.Cursor = 0
		}
		lb.clampScroll()
	case KeyPageDown:
		lb.Cursor += 5
		if lb.Cursor >= len(lb.Items) {
			lb.Cursor = len(lb.Items) - 1
		}
		lb.clampScroll()
	case KeyEnter:
		item := lb.Selected()
		if item != nil && item.Fn != nil {
			return Result{Action: ActionExec, Fn: item.Fn}
		}
	case KeyRune:
		// 支持按首字母快速跳转
		r := e.Rune
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		for i := lb.Cursor + 1; i < len(lb.Items); i++ {
			if len(lb.Items[i].Label) > 0 {
				first := lb.Items[i].Label[0]
				if first >= 'A' && first <= 'Z' {
					first += 'a' - 'A'
				}
				if first == byte(r) {
					lb.Cursor = i
					lb.clampScroll()
					return Result{Action: ActionNone}
				}
			}
		}
		// 从头开始搜索
		for i := 0; i <= lb.Cursor; i++ {
			if len(lb.Items[i].Label) > 0 {
				first := lb.Items[i].Label[0]
				if first >= 'A' && first <= 'Z' {
					first += 'a' - 'A'
				}
				if first == byte(r) {
					lb.Cursor = i
					lb.clampScroll()
					return Result{Action: ActionNone}
				}
			}
		}
	}

	return Result{Action: ActionNone}
}

// Draw 在给定区域内渲染列表框
func (lb *ListBox) Draw(r *Renderer, x, y, width, height int) {
	if height <= 0 {
		return
	}

	// 确保光标在可见范围内
	lb.clampScroll()

	// 计算可见项数
	visibleCount := min(height, len(lb.Items)-lb.ScrollStart)
	if visibleCount <= 0 {
		return
	}

	for i := 0; i < visibleCount; i++ {
		idx := lb.ScrollStart + i
		if idx >= len(lb.Items) {
			break
		}
		item := lb.Items[idx]
		itemY := y + i

		// 选择样式
		var style Style
		cursorMark := "  "
		if idx == lb.Cursor && lb.focused {
			style = StyleCursor
			cursorMark = " >"
		} else if idx == lb.Cursor {
			cursorMark = " >"
			style = StyleNormal
		}

		// 渲染标签
		label := fmt.Sprintf("%s %s", cursorMark, item.Label)
		r.PrintTruncated(x, itemY, width, label, style)

		// 渲染子标签
		if item.SubLabel != "" {
			availableWidth := width - len(label) - 3
			if availableWidth > 0 {
				subLabel := strings.TrimSpace(item.SubLabel)
				subX := x + len(label) + 2
				r.PrintTruncated(subX, itemY, availableWidth, subLabel, StyleDim)
			}
		}
	}

	// 显示滚动指示器
	if lb.ScrollStart > 0 {
		r.Print(x+width-2, y, "^", StyleDim)
	}
	if lb.ScrollStart+visibleCount < len(lb.Items) {
		r.Print(x+width-2, y+height-1, "v", StyleDim)
	}
}

// Selected 返回当前选中的菜单项
func (lb *ListBox) Selected() *MenuItem {
	if lb.Cursor >= 0 && lb.Cursor < len(lb.Items) {
		return &lb.Items[lb.Cursor]
	}
	return nil
}

// clampScroll 确保光标在可视区域内
func (lb *ListBox) clampScroll() {
	// 如果有 visibleHeight 可以在外面设置，这里使用默认值
	// 调用者应在 Draw 时确保正确滚动
	if lb.Cursor < lb.ScrollStart {
		lb.ScrollStart = lb.Cursor
	}
	// 预留至少 1 行的余量（假设 visibleHeight >= 1）
	// 实际滚动调整会在 Draw 时根据 height 参数进行
}

// AdjustScroll 根据可视区域高度调整滚动偏移
func (lb *ListBox) AdjustScroll(visibleHeight int) {
	if visibleHeight <= 0 {
		return
	}
	if lb.Cursor >= lb.ScrollStart+visibleHeight {
		lb.ScrollStart = lb.Cursor - visibleHeight + 1
	}
	if lb.Cursor < lb.ScrollStart {
		lb.ScrollStart = lb.Cursor
	}
	if lb.ScrollStart < 0 {
		lb.ScrollStart = 0
	}
}

// helper
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
