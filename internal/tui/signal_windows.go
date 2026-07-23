package tui

import (
	"os"
)

// sigWinchSignal 在 Windows 上为空（Windows 不支持 SIGWINCH）
var sigWinchSignal os.Signal = nil

// isSigWinchSupported 在 Windows 上始终返回 false
func isSigWinchSupported() bool {
	return false
}

// readEventUnix 在 Windows 上不可用（存根）
func readEventUnix() Event {
	return Event{Type: EventError, Err: os.ErrClosed}
}
