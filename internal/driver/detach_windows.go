//go:build windows

package driver

import (
	"os/exec"
	"syscall"
)

// setDetached configures the command to run in its own process group so it
// survives the parent bws process.
func setDetached(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr.CreationFlags = 0x00000200
}
