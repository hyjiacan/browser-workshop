//go:build windows

package instance

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Windows access rights and process status constants. They are declared
// locally because the standard library does not export all of them.
const (
	processQueryInformation        = 0x0400
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// ProcessAlive reports whether a process with the given PID exists and is
// still running.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		h, err = syscall.OpenProcess(processQueryInformation, false, uint32(pid))
		if err != nil {
			return false
		}
	}
	defer syscall.CloseHandle(h)

	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// killProcess terminates the process tree. Chrome spawns child processes, so
// taskkill /T is used to avoid leaving orphans behind.
func killProcess(pid int) error {
	if err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run(); err == nil {
		return nil
	}

	// Fall back to a direct kill when taskkill is unavailable.
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Kill(); err != nil {
		if !ProcessAlive(pid) {
			return nil
		}
		return err
	}
	return nil
}
