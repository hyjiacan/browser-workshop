package tui

import (
	"os"
	"runtime"
)

// IsInteractive 检测 stdin 是否连接到交互式终端
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// SupportsColor 检测终端是否支持 ANSI 颜色
func SupportsColor() bool {
	// Windows 10+ 和现代 Unix 终端都支持 ANSI 颜色
	return true
}

// enterRawMode 进入终端原始模式，返回恢复函数
func enterRawMode() (func(), error) {
	if isWindows() {
		return enterRawModeWindows()
	}
	return enterRawModeUnix()
}

// exitRaw 临时退出 raw mode
func exitRaw() {
	if isWindows() {
		exitRawWindows()
	} else {
		exitRawUnix()
	}
}

// reenterRaw 重新进入 raw mode（exitRaw 的配对操作）
func reenterRaw() {
	if isWindows() {
		reenterRawWindows()
	} else {
		reenterRawUnix()
	}
}

// isWindows 检测操作系统是否为 Windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// getTerminalSize 获取终端大小（列数, 行数）
func getTerminalSize() (int, int) {
	if isWindows() {
		return getTerminalSizeWindows()
	}
	return getTerminalSizeUnix()
}

// terminalState 保存终端状态，用于临时退出和恢复
type terminalState struct {
	cleanup func()
}

// saveState 保存当前终端状态
func saveState() (*terminalState, error) {
	cleanup, err := enterRawMode()
	if err != nil {
		return nil, err
	}
	return &terminalState{cleanup: cleanup}, nil
}

// restore 恢复终端状态
func (t *terminalState) restore() {
	if t.cleanup != nil {
		t.cleanup()
	}
}
