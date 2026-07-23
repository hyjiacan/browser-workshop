//go:build !windows

package tui

import (
	"syscall"
)

// sigWinchSignal 是 Unix 平台的 SIGWINCH 信号
var sigWinchSignal = syscall.SIGWINCH

// isSigWinchSupported 在 Unix 平台检查是否支持 SIGWINCH
func isSigWinchSupported() bool {
	return syscall.SIGWINCH != 0
}
