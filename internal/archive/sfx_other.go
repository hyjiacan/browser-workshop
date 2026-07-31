//go:build !windows
// +build !windows

package archive

import "fmt"

// extractExeViaInstaller is a no-op on non-Windows platforms.
// Windows .exe installers cannot be executed on Unix/macOS.
func extractExeViaInstaller(srcPath, destDir string) error {
	return fmt.Errorf("不支持在非 Windows 系统上运行 .exe 安装器")
}
