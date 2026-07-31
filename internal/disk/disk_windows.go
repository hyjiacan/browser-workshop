//go:build windows

package disk

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FreeSpace returns the available free space in bytes for the disk containing the given path.
// 使用 golang.org/x/sys/windows 替代已废弃的 syscall 包。
// windows.GetDiskFreeSpaceEx 内部使用延迟加载机制，DLL 仅在首次调用时加载一次并缓存，
// 无需手动使用 sync.Once 管理。
func FreeSpace(path string) (uint64, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// 将路径转换为 Windows API 所需的 UTF-16 宽字符指针
	pathPtr, err := windows.UTF16PtrFromString(absPath)
	if err != nil {
		return 0, fmt.Errorf("转换路径为 UTF-16 失败: %w", err)
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailable,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx 调用失败: %w", err)
	}

	return freeBytesAvailable, nil
}
