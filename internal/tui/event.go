package tui

// Key 表示按键
type Key int

const (
	KeyUnknown Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyEsc
	KeyTab
	KeyBackspace
	KeySpace
	KeyCtrlC
	KeyCtrlL
	KeyCtrlA
	KeyCtrlE
	KeyCtrlU
	KeyCtrlK
	KeyCtrlJ // Ctrl+J (在 raw mode 中等同于向下)
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyInsert
	KeyRune // 普通字符
)

// EventType 事件类型
type EventType int

const (
	EventKey    EventType = iota // 按键事件
	EventResize                  // 窗口大小变化事件
	EventError                   // 错误事件
)

// Event 表示终端事件
type Event struct {
	Type   EventType
	Key    Key
	Rune   rune
	Width  int // 窗口宽度（resize 事件）
	Height int // 窗口高度（resize 事件）
	Err    error
}

// ReadEvent 阻塞读取下一个终端事件
func ReadEvent() Event {
	if isWindows() {
		return readEventWindows()
	}
	return readEventUnix()
}
