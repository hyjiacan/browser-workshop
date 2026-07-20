// Package disk provides disk space utilities.
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

	// On Windows, use GetDiskFreeSpaceEx via kernel32
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

// FreeSpaceGB returns the available free space in gigabytes.
func FreeSpaceGB(path string) (float64, error) {
	bytes, err := FreeSpace(path)
	if err != nil {
		return 0, err
	}
	return float64(bytes) / (1024 * 1024 * 1024), nil
}

// FormatSize formats a byte count as a human-readable string.
func FormatSize(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
