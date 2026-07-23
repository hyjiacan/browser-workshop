package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Renderer 管理屏幕绘制
type Renderer struct {
	buf    bytes.Buffer
	width  int
	height int
}

// NewRenderer 创建新的渲染器
func NewRenderer() *Renderer {
	return &Renderer{
		width:  80,
		height: 24,
	}
}

// Width 返回终端宽度
func (r *Renderer) Width() int {
	return r.width
}

// Height 返回终端高度
func (r *Renderer) Height() int {
	return r.height
}

// Clear 清屏并将光标移到左上角
func (r *Renderer) Clear() {
	r.buf.WriteString("\x1b[2J\x1b[H")
}

// ClearLine 清除从光标到行尾的内容
func (r *Renderer) ClearLine() {
	r.buf.WriteString("\x1b[2K")
}

// ClearLineFrom 清除指定行从 x 位置到行尾的内容
func (r *Renderer) ClearLineFrom(x, y int) {
	r.buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", y+1, x+1))
	r.buf.WriteString("\x1b[0K")
}

// HideCursor 隐藏光标
func (r *Renderer) HideCursor() {
	r.buf.WriteString("\x1b[?25l")
}

// ShowCursor 显示光标
func (r *Renderer) ShowCursor() {
	r.buf.WriteString("\x1b[?25h")
}

// MoveCursor 移动光标到指定位置
func (r *Renderer) MoveCursor(x, y int) {
	r.buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", y+1, x+1))
}

// Print 在指定位置输出带样式的文本
func (r *Renderer) Print(x, y int, text string, s Style) {
	r.buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", y+1, x+1))
	if seq := s.Render(); seq != "" {
		r.buf.WriteString(seq)
	}
	r.buf.WriteString(text)
	if s != (Style{}) {
		r.buf.WriteString(Reset())
	}
}

// Printf 格式化输出带样式文本
func (r *Renderer) Printf(x, y int, format string, s Style, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	r.Print(x, y, text, s)
}

// Println 在指定行输出（左对齐）
func (r *Renderer) Println(y int, text string, s Style) {
	r.Print(0, y, text, s)
}

// Printlnf 在指定行格式化输出
func (r *Renderer) Printlnf(y int, format string, s Style, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	r.Println(y, text, s)
}

// PrintTruncated 在指定位置输出截断后的文本（超出宽度部分用 "..." 截断）
func (r *Renderer) PrintTruncated(x, y, maxWidth int, text string, s Style) {
	if len(text) > maxWidth {
		if maxWidth <= 3 {
			text = strings.Repeat(".", maxWidth)
		} else {
			text = text[:maxWidth-3] + "..."
		}
	}
	r.Print(x, y, text, s)
}

// PrintRight 在指定行的右侧输出文本
func (r *Renderer) PrintRight(y int, text string, s Style) {
	x := r.width - len(text) - 1
	if x < 0 {
		x = 0
	}
	r.Print(x, y, text, s)
}

// PrintCenter 在指定行居中输出文本
func (r *Renderer) PrintCenter(y int, text string, s Style) {
	x := (r.width - len(text)) / 2
	if x < 0 {
		x = 0
	}
	r.Print(x, y, text, s)
}

// DrawHLine 在指定行绘制水平分隔线
func (r *Renderer) DrawHLine(y int, ch rune, s Style) {
	text := strings.Repeat(string(ch), r.width)
	r.Println(y, text, s)
}

// FillArea 用指定字符填充矩形区域
func (r *Renderer) FillArea(x, y, w, h int, ch rune, s Style) {
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			r.Print(x+col, y+row, string(ch), s)
		}
	}
}

// Flush 将缓冲区内容写入输出
func (r *Renderer) Flush(w io.Writer) error {
	_, err := w.Write(r.buf.Bytes())
	r.buf.Reset()
	return err
}

// UpdateSize 更新终端大小
func (r *Renderer) UpdateSize() {
	r.width, r.height = getTerminalSize()
	if r.width < 20 {
		r.width = 20
	}
	if r.height < 5 {
		r.height = 5
	}
}

// BufferSize 返回当前缓冲区大小
func (r *Renderer) BufferSize() int {
	return r.buf.Len()
}
