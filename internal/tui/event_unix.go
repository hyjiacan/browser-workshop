//go:build !windows

package tui

import (
	"os"
	"syscall"
)

// ANSI 转义序列状态机状态
const (
	stateNormal = iota
	stateEscape  // 收到 ESC (\x1b)
	stateCSI     // 收到 ESC [
)

// ANSI 转义序列中的特殊字节
const (
	esc     = 0x1b
	bracket = '['
)

// readEventUnix 从 Unix 终端读取事件（阻塞）
func readEventUnix() Event {
	buf := make([]byte, 64)
	state := stateNormal
	csiBuf := make([]byte, 0, 16)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return Event{Type: EventError, Err: err}
		}

		for i := 0; i < n; i++ {
			b := buf[i]

			switch state {
			case stateNormal:
				if b == esc {
					state = stateEscape
					csiBuf = csiBuf[:0]
					continue
				}
				// 处理普通字节
				return handleNormalByte(b)

			case stateEscape:
				if b == bracket {
					state = stateCSI
					continue
				}
				if b == 'O' {
					// ESC O 前缀（如 ESC O P = F1）
					state = stateCSI
					csiBuf = append(csiBuf, b)
					continue
				}
				// 单独的 ESC
				return Event{Type: EventKey, Key: KeyEsc}

			case stateCSI:
				csiBuf = append(csiBuf, b)
				if isCSITerminator(b) {
					ev := parseCSISequence(csiBuf)
					if ev.Key != KeyUnknown {
						return ev
					}
					// 解析失败，重置状态
					state = stateNormal
					csiBuf = csiBuf[:0]
				}
			}
		}
	}
}

// isCSITerminator 判断是否为 CSI 序列终止符
func isCSITerminator(b byte) bool {
	// CSI 序列以 0x40-0x7E 范围的字节终止
	return b >= 0x40 && b <= 0x7E
}

// handleNormalByte 处理非转义序列的普通字节
func handleNormalByte(b byte) Event {
	// Ctrl 组合键（ASCII 0x00-0x1F，其中 0x1B ESC 已在上面处理）
	if b < 0x20 {
		return handleControlByte(b)
	}

	// 普通可打印字符
	return Event{Type: EventKey, Key: KeyRune, Rune: rune(b)}
}

// handleControlByte 处理控制字符
func handleControlByte(b byte) Event {
	switch b {
	case 0x0A: // Ctrl+J (通常等于 Enter 在 raw mode 中)
		return Event{Type: EventKey, Key: KeyEnter}
	case 0x0D: // 回车 (Ctrl+M)
		return Event{Type: EventKey, Key: KeyEnter}
	case 0x7F: // DEL / Backspace
		return Event{Type: EventKey, Key: KeyBackspace}
	case 0x08: // Backspace (Ctrl+H)
		return Event{Type: EventKey, Key: KeyBackspace}
	case 0x09: // Tab (Ctrl+I)
		return Event{Type: EventKey, Key: KeyTab}
	case 0x00: // Ctrl+Space / NUL
		return Event{Type: EventKey, Key: KeySpace}
	default:
		// Ctrl+A ~ Ctrl+Z (除已处理的)
		if b >= 0x01 && b <= 0x1A {
			return Event{Type: EventKey, Key: ctrlFromByte(b)}
		}
		return Event{Type: EventKey, Key: KeyUnknown}
	}
}

// ctrlFromByte 将 Ctrl 组合键的字节映射为 Key
func ctrlFromByte(b byte) Key {
	// Ctrl+字母：Ctrl+A=0x01, Ctrl+B=0x02, ..., Ctrl+Z=0x1A
	switch b {
	case 0x03: // Ctrl+C
		return KeyCtrlC
	case 0x0C: // Ctrl+L
		return KeyCtrlL
	case 0x01: // Ctrl+A
		return KeyCtrlA
	case 0x05: // Ctrl+E
		return KeyCtrlE
	case 0x15: // Ctrl+U
		return KeyCtrlU
	case 0x0B: // Ctrl+K
		return KeyCtrlK
	default:
		return KeyUnknown
	}
}

// parseCSISequence 解析 ANSI CSI 转义序列
func parseCSISequence(buf []byte) Event {
	if len(buf) == 0 {
		return Event{Type: EventKey, Key: KeyUnknown}
	}

	last := buf[len(buf)-1]

	// ESC O 前缀序列
	if buf[0] == 'O' {
		switch last {
		case 'P': // F1
			return Event{Type: EventKey, Key: KeyUnknown}
		case 'Q': // F2
			return Event{Type: EventKey, Key: KeyUnknown}
		case 'R': // F3
			return Event{Type: EventKey, Key: KeyUnknown}
		case 'S': // F4
			return Event{Type: EventKey, Key: KeyUnknown}
		case 'H': // Home
			return Event{Type: EventKey, Key: KeyHome}
		case 'F': // End
			return Event{Type: EventKey, Key: KeyEnd}
		}
		return Event{Type: EventKey, Key: KeyUnknown}
	}

	// 标准方向键和功能键
	switch last {
	case 'A': // Up
		return Event{Type: EventKey, Key: KeyUp}
	case 'B': // Down
		return Event{Type: EventKey, Key: KeyDown}
	case 'C': // Right
		return Event{Type: EventKey, Key: KeyRight}
	case 'D': // Left
		return Event{Type: EventKey, Key: KeyLeft}
	case 'H': // Home
		return Event{Type: EventKey, Key: KeyHome}
	case 'F': // End
		return Event{Type: EventKey, Key: KeyEnd}
	case '~':
		// 扩展键：ESC [ <n> ~
		// 例如: 5~ = PageUp, 6~ = PageDown, 3~ = Delete, 1~ = Home, 4~ = End
		return parseExtendedKey(buf)
	}

	return Event{Type: EventKey, Key: KeyUnknown}
}

// parseExtendedKey 解析 ESC [ <n> ~ 格式的扩展键
func parseExtendedKey(buf []byte) Event {
	// 解析数字参数
	num := 0
	for i := 0; i < len(buf)-1; i++ {
		if buf[i] >= '0' && buf[i] <= '9' {
			num = num*10 + int(buf[i]-'0')
		}
	}

	switch num {
	case 1:
		return Event{Type: EventKey, Key: KeyHome}
	case 2:
		return Event{Type: EventKey, Key: KeyInsert}
	case 3:
		return Event{Type: EventKey, Key: KeyDelete}
	case 4:
		return Event{Type: EventKey, Key: KeyEnd}
	case 5:
		return Event{Type: EventKey, Key: KeyPageUp}
	case 6:
		return Event{Type: EventKey, Key: KeyPageDown}
	default:
		return Event{Type: EventKey, Key: KeyUnknown}
	}
}

// isSigWinchSupported 检查是否支持 SIGWINCH（Unix 窗口大小变化信号）
func isSigWinchSupported() bool {
	return syscall.SIGWINCH != 0
}
