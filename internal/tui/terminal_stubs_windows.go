package tui

// enterRawModeUnix 存根（Windows 平台不使用）
func enterRawModeUnix() (func(), error) {
	return func() {}, nil
}

// exitRawUnix 存根（Windows 平台不使用）
func exitRawUnix() {}

// reenterRawUnix 存根（Windows 平台不使用）
func reenterRawUnix() {}

// getTerminalSizeUnix 存根（Windows 平台不使用）
func getTerminalSizeUnix() (int, int) {
	return 80, 24
}