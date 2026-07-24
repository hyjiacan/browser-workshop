//go:build windows

package disk

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

// FreeSpace returns the available free space in bytes for the disk containing the given path.
func FreeSpace(path string) (uint64, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	ret, _, err := proc.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(absPath))),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx failed: %w", err)
	}

	return freeBytesAvailable, nil
}
