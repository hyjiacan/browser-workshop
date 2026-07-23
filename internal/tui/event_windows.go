package tui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// Windows 事件类型
	evtKey          = 0x0001
	evtMouse        = 0x0002
	evtWindowBuffer = 0x0004

	// Windows 虚拟键码
	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0D
	vkShift  = 0x10
	vkControl = 0x11
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkPrior  = 0x21 // PageUp
	vkNext   = 0x22 // PageDown
	vkEnd    = 0x23
	vkHome   = 0x24
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkDelete = 0x2E
	vkF1     = 0x70
)

// Ctrl 键状态标志
const (
	leftCtrlPressed  = 0x0002
	rightCtrlPressed = 0x0008
)

// readEventWindows 从 Windows 控制台读取事件
func readEventWindows() Event {
	var stdinHandle uintptr
	stdinHandle, _, _ = procGetStdHandle.Call(uintptr(stdInputHandle))

	var record windowsInputRecord
	var numRead uint32 = 0

	for {
		ret, _, _ := procReadConsoleInput.Call(
			stdinHandle,
			uintptr(unsafe.Pointer(&record)),
			uintptr(1),
			uintptr(unsafe.Pointer(&numRead)),
		)
		if ret == 0 || numRead == 0 {
			return Event{Type: EventError, Err: windows.GetLastError()}
		}

		switch record.EventType {
		case evtKey:
			return handleKeyEventWindows(record)
		case evtWindowBuffer:
			return Event{
				Type:   EventResize,
				Width:  80, // 会在主循环中更新
				Height: 24,
			}
		// 忽略鼠标事件
		}
	}
}

// procReadConsoleInput 读取控制台输入记录
var procReadConsoleInput = kernel32.NewProc("ReadConsoleInputW")

// handleKeyEventWindows 处理 Windows 按键事件
func handleKeyEventWindows(record windowsInputRecord) Event {
	// 解析 KEY_EVENT_RECORD
	keyDown := *(*int32)(unsafe.Pointer(&record.Event[0]))
	if keyDown == 0 {
		// KeyUp 事件，忽略
		return Event{Type: EventKey, Key: KeyUnknown}
	}

	// virtualKeyCode 在偏移 8 (int32 + uint16 + uint16)
	virtualKeyCode := *(*uint16)(unsafe.Pointer(&record.Event[8]))
	// char 在偏移 10
	char := *(*uint16)(unsafe.Pointer(&record.Event[10]))
	// controlKeyState 在偏移 12
	controlKeyState := *(*uint32)(unsafe.Pointer(&record.Event[12]))

	isCtrl := (controlKeyState & (leftCtrlPressed | rightCtrlPressed)) != 0

	switch virtualKeyCode {
	case vkUp:
		return Event{Type: EventKey, Key: KeyUp}
	case vkDown:
		return Event{Type: EventKey, Key: KeyDown}
	case vkLeft:
		return Event{Type: EventKey, Key: KeyLeft}
	case vkRight:
		return Event{Type: EventKey, Key: KeyRight}
	case vkReturn:
		return Event{Type: EventKey, Key: KeyEnter}
	case vkEscape:
		return Event{Type: EventKey, Key: KeyEsc}
	case vkTab:
		return Event{Type: EventKey, Key: KeyTab}
	case vkBack:
		return Event{Type: EventKey, Key: KeyBackspace}
	case vkDelete:
		return Event{Type: EventKey, Key: KeyDelete}
	case vkHome:
		return Event{Type: EventKey, Key: KeyHome}
	case vkEnd:
		return Event{Type: EventKey, Key: KeyEnd}
	case vkPrior:
		return Event{Type: EventKey, Key: KeyPageUp}
	case vkNext:
		return Event{Type: EventKey, Key: KeyPageDown}
	case vkSpace:
		return Event{Type: EventKey, Key: KeySpace}
	default:
		// Ctrl 组合键
		if isCtrl {
			switch virtualKeyCode {
			case 0x43: // Ctrl+C
				return Event{Type: EventKey, Key: KeyCtrlC}
			case 0x4C: // Ctrl+L
				return Event{Type: EventKey, Key: KeyCtrlL}
			case 0x41: // Ctrl+A
				return Event{Type: EventKey, Key: KeyCtrlA}
			case 0x45: // Ctrl+E
				return Event{Type: EventKey, Key: KeyCtrlE}
			case 0x55: // Ctrl+U
				return Event{Type: EventKey, Key: KeyCtrlU}
			case 0x4B: // Ctrl+K
				return Event{Type: EventKey, Key: KeyCtrlK}
			}
		}

		// 普通字符
		if char != 0 {
			return Event{Type: EventKey, Key: KeyRune, Rune: rune(char)}
		}

		return Event{Type: EventKey, Key: KeyUnknown}
	}
}
