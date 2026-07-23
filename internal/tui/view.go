package tui

// Action 定义视图操作的返回动作
type Action int

const (
	ActionNone    Action = iota // 无操作
	ActionQuit                  // 退出 TUI
	ActionPush                  // 压入新视图到栈顶
	ActionPop                   // 弹出栈顶视图
	ActionExec                  // 执行外部函数（需临时退出 raw mode）
	ActionRefresh               // 刷新屏幕
)

// Result 包含动作和关联数据
type Result struct {
	Action Action
	View   View
	Fn     func() error
}

// View 是 TUI 页面接口
type View interface {
	// Handle 处理终端事件，返回操作结果
	Handle(e Event) Result
	// Draw 绘制页面内容
	Draw(r *Renderer)
	// OnEnter 进入页面时调用
	OnEnter()
	// OnExit 离开页面时调用
	OnExit()
}

// Navigator 管理页面栈，实现页面导航
type Navigator struct {
	stack []View
}

// NewNavigator 创建新的导航器，以 root 为根页面
func NewNavigator(root View) *Navigator {
	nav := &Navigator{
		stack: make([]View, 0, 8),
	}
	nav.Push(root)
	return nav
}

// Push 压入新视图到栈顶
func (n *Navigator) Push(view View) {
	// 调用当前栈顶视图的退出回调
	if len(n.stack) > 0 {
		n.stack[len(n.stack)-1].OnExit()
	}
	n.stack = append(n.stack, view)
	// 调用新视图的进入回调
	view.OnEnter()
}

// Pop 弹出栈顶视图
func (n *Navigator) Pop() {
	if len(n.stack) <= 1 {
		// 根视图不可弹出
		return
	}
	n.stack[len(n.stack)-1].OnExit()
	n.stack = n.stack[:len(n.stack)-1]
	n.stack[len(n.stack)-1].OnEnter()
}

// Current 返回当前栈顶视图
func (n *Navigator) Current() View {
	if len(n.stack) == 0 {
		return nil
	}
	return n.stack[len(n.stack)-1]
}

// Handle 将事件分发给栈顶视图，根据返回的 Result 处理导航动作
func (n *Navigator) Handle(e Event) Result {
	current := n.Current()
	if current == nil {
		return Result{Action: ActionQuit}
	}

	result := current.Handle(e)

	switch result.Action {
	case ActionPush:
		if result.View != nil {
			n.Push(result.View)
			return Result{Action: ActionNone}
		}
	case ActionPop:
		if len(n.stack) > 1 {
			n.Pop()
			return Result{Action: ActionNone}
		}
		// 栈底视图 pop 等同于退出
		return Result{Action: ActionQuit}
	case ActionQuit:
		return result
	}

	return result
}

// Draw 从底到顶绘制所有页面（通常只绘制栈顶）
func (n *Navigator) Draw(r *Renderer) {
	if len(n.stack) == 0 {
		return
	}
	// 只绘制栈顶视图
	current := n.Current()
	if current != nil {
		current.Draw(r)
	}
}

// Depth 返回页面栈深度
func (n *Navigator) Depth() int {
	return len(n.stack)
}
