//go:build !windows

package disk

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// FreeSpace returns the available free space in bytes for the disk containing the given path.
func FreeSpace(path string) (uint64, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(absPath, &stat); err != nil {
		return 0, fmt.Errorf("statfs failed: %w", err)
	}

	// Available blocks * block size
	return stat.Bavail * uint64(stat.Bsize), nil
}
