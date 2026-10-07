//go:build !windows

package instance

import (
	"errors"
	"syscall"
	"time"
)

// ProcessAlive reports whether a process with the given PID exists.
// Signal 0 performs error checking without actually sending a signal.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but belongs to another user.
	return errors.Is(err, syscall.EPERM)
}

// killProcess sends SIGTERM, waits briefly, then escalates to SIGKILL.
func killProcess(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !ProcessAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
