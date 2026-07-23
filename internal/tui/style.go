package tui

import (
	"bytes"
	"fmt"
	"strings"
)

// Color 定义 ANSI 颜色码
type Color int

const (
	ColorDefault Color = iota
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
	ColorGray // 亮黑/灰色，通过 bold+black 模拟
)

// Style 定义文本样式
type Style struct {
	Foreground Color
	Background Color
	Bold       bool
	Dim        bool
	Reverse    bool
	Underline  bool
}

// Render 输出 ANSI 转义序列
func (s Style) Render() string {
	if s.Reverse {
		return "\x1b[7m"
	}

	var codes []string
	if s.Bold {
		codes = append(codes, "1")
	}
	if s.Dim {
		codes = append(codes, "2")
	}
	if s.Underline {
		codes = append(codes, "4")
	}
	if s.Foreground != ColorDefault {
		codes = append(codes, fmt.Sprintf("%d", 30+int(s.Foreground)))
	}
	if s.Background != ColorDefault {
		codes = append(codes, fmt.Sprintf("%d", 40+int(s.Background)))
	}

	if len(codes) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("\x1b[")
	buf.WriteString(strings.Join(codes, ";"))
	buf.WriteString("m")
	return buf.String()
}

// Reset 返回重置所有样式的 ANSI 转义序列
func Reset() string {
	return "\x1b[0m"
}

// 预定义样式
var (
	StyleTitle   = Style{Foreground: ColorCyan, Bold: true}
	StyleCursor  = Style{Reverse: true}
	StyleNormal  = Style{}
	StyleDim     = Style{Foreground: ColorGray}
	StyleSuccess = Style{Foreground: ColorGreen}
	StyleWarning = Style{Foreground: ColorYellow}
	StyleError   = Style{Foreground: ColorRed}
	StyleHeader  = Style{Foreground: ColorBlue, Bold: true}
	StyleKey     = Style{Foreground: ColorYellow}
	StyleMuted   = Style{Foreground: ColorGray, Dim: true}
	StyleBold    = Style{Bold: true}
)
