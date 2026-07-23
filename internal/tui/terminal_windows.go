package tui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCtrlHandler      = kernel32.NewProc("SetConsoleCtrlHandler")
	procGetStdHandle               = kernel32.NewProc("GetStdHandle")

	stdOutputHandle = uint32(0xFFFFFFF5) // STD_OUTPUT_HANDLE
	stdInputHandle  = uint32(0xFFFFFFF6) // STD_INPUT_HANDLE
)

type windowsCoord struct {
	X int16
	Y int16
}

type windowsSmallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type windowsConsoleScreenBufferInfo struct {
	Size              windowsCoord
	CursorPosition   windowsCoord
	Attributes        uint16
	Window            windowsSmallRect
	MaximumWindowSize windowsCoord
}

type windowsKeyRecord struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	Char            uint16
	ControlKeyState uint32
}

type windowsInputRecord struct {
	EventType uint16
	Padding   [2]byte
	Event     [16]byte
}

// Windows 控制台模式常量
const (
	enableVirtualTerminalProcessing uint32 = 0x0004
	enableEchoInput                uint32 = 0x0004
	enableLineInput                uint32 = 0x0002
	enableProcessedInput           uint32 = 0x0001
	enableMouseInput               uint32 = 0x0010
	enableWindowInput              uint32 = 0x0008
)

// enterRawModeWindows 进入 Windows 终端原始模式
func enterRawModeWindows() (func(), error) {
	// 获取 stdin 句柄
	stdinHandle, _, _ := procGetStdHandle.Call(uintptr(stdInputHandle))

	// 获取当前模式
	var oldMode uint32
	ret, _, err := procGetConsoleMode.Call(stdinHandle, uintptr(unsafe.Pointer(&oldMode)))
	if ret == 0 {
		return nil, err
	}

	// 设置新模式：禁用回显、行编辑、鼠标
	newMode := oldMode &^ (enableEchoInput | enableLineInput | enableProcessedInput | enableMouseInput)
	newMode |= enableWindowInput

	ret, _, err = procSetConsoleMode.Call(stdinHandle, uintptr(newMode))
	if ret == 0 {
		return nil, err
	}

	// 启用 stdout 的 ANSI 支持
	stdoutHandle, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))

	var stdoutMode uint32
	procGetConsoleMode.Call(stdoutHandle, uintptr(unsafe.Pointer(&stdoutMode)))
	procSetConsoleMode.Call(stdoutHandle, uintptr(stdoutMode|enableVirtualTerminalProcessing))

	// 返回恢复函数
	return func() {
		procSetConsoleMode.Call(stdinHandle, uintptr(oldMode))
		procSetConsoleMode.Call(stdoutHandle, uintptr(stdoutMode))
	}, nil
}

// getTerminalSizeWindows 获取 Windows 终端大小
func getTerminalSizeWindows() (int, int) {
	stdoutHandle, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))

	var info windowsConsoleScreenBufferInfo
	ret, _, _ := procGetConsoleScreenBufferInfo.Call(
		stdoutHandle,
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return 80, 24
	}

	width := int(info.Window.Right - info.Window.Left + 1)
	height := int(info.Window.Bottom - info.Window.Top + 1)
	return width, height
}

// exitRawWindows 临时退出 raw mode 恢复终端
func exitRawWindows() {
	stdinHandle, _, _ := procGetStdHandle.Call(uintptr(stdInputHandle))

	var oldMode uint32
	procGetConsoleMode.Call(stdinHandle, uintptr(unsafe.Pointer(&oldMode)))

	// 恢复到正常模式
	normalMode := oldMode | (enableEchoInput | enableLineInput | enableProcessedInput)
	procSetConsoleMode.Call(stdinHandle, uintptr(normalMode))
}

// reenterRawWindows 重新进入 raw mode
func reenterRawWindows() {
	stdinHandle, _, _ := procGetStdHandle.Call(uintptr(stdInputHandle))

	var currentMode uint32
	procGetConsoleMode.Call(stdinHandle, uintptr(unsafe.Pointer(&currentMode)))

	newMode := currentMode &^ (enableEchoInput | enableLineInput | enableProcessedInput | enableMouseInput)
	newMode |= enableWindowInput
	procSetConsoleMode.Call(stdinHandle, uintptr(newMode))
}
