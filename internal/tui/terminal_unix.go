//go:build !windows

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// enterRawModeUnix 进入 Unix 终端原始模式
func enterRawModeUnix() (func(), error) {
	fd := int(os.Stdin.Fd())

	// 获取当前终端属性
	var oldState syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&oldState)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}

	// 创建 raw mode 属性副本
	newState := oldState

	// 禁用规范模式（行编辑）
	newState.Iflag &^= syscall.ICANON
	// 禁用回显
	newState.Lflag &^= syscall.ECHO
	// 禁用信号字符（ISIG: INTR, QUIT, SUSP）
	newState.Lflag &^= syscall.ISIG
	// 禁用扩展处理（不将 \n 转换为 \r\n）
	newState.Iflag &^= syscall.ICRNL
	newState.Iflag &^= syscall.INLCR
	newState.Iflag &^= syscall.IXON
	// 禁用输出处理
	newState.Oflag &^= syscall.OPOST
	// 设置最小读取字符数为 1（不等待整行）
	newState.Cc[syscall.VMIN] = 1
	// 设置读取超时为 0（不超时，立即返回）
	newState.Cc[syscall.VTIME] = 0

	// 应用新终端属性
	_, _, errno = syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&newState)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}

	// 返回恢复函数
	return func() {
		syscall.Syscall6(syscall.SYS_IOCTL,
			uintptr(fd), uintptr(syscall.TCSETS),
			uintptr(unsafe.Pointer(&oldState)), 0, 0, 0)
	}, nil
}

// getTerminalSizeUnix 获取 Unix 终端大小
func getTerminalSizeUnix() (int, int) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}

	fd := int(os.Stdout.Fd())
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

// exitRawUnix 临时退出 raw mode 恢复终端
func exitRawUnix() {
	fd := int(os.Stdin.Fd())

	var currentState syscall.Termios
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&currentState)), 0, 0, 0)

	// 恢复回显和行编辑
	currentState.Lflag |= syscall.ECHO | syscall.ICANON
	currentState.Iflag |= syscall.ICRNL
	currentState.Oflag |= syscall.OPOST

	syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&currentState)), 0, 0, 0)
}

// reenterRawUnix 重新进入 raw mode
func reenterRawUnix() {
	fd := int(os.Stdin.Fd())

	var currentState syscall.Termios
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&currentState)), 0, 0, 0)

	currentState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	currentState.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IXON
	currentState.Oflag &^= syscall.OPOST
	currentState.Cc[syscall.VMIN] = 1
	currentState.Cc[syscall.VTIME] = 0

	syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd), uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&currentState)), 0, 0, 0)
}
