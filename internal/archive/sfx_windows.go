//go:build windows
// +build windows

package archive

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// extractExeViaInstaller runs a Windows .exe installer with the -ExtractDir
// flag to extract its contents without performing an actual installation.
//
// This is primarily used for Firefox's NSIS installer, which supports
// -ExtractDir=<path> to extract browser files to a specified directory.
// The flag has been available since Firefox 4.0.
//
// The installer window is hidden using CREATE_NO_WINDOW to avoid
// distracting the user with a flashing console window.
func extractExeViaInstaller(srcPath, destDir string) error {
	// Ensure destDir exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	cmd := exec.Command(srcPath, "-ExtractDir="+destDir)
	// CREATE_NO_WINDOW (0x08000000) prevents a console window from flashing
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}

	// Run the installer — ignore exit code, verify by checking extracted files
	_ = cmd.Run()

	// Verify that files were actually extracted
	entries, err := os.ReadDir(destDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("安装器提取后目标目录为空")
	}

	return nil
}
