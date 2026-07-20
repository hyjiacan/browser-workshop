//go:build !windows

package launch

import (
	"os/exec"
	"syscall"
)

// setDetached configures the command to run detached from the parent process.
func setDetached(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// On Unix, setsid
	cmd.SysProcAttr.Setsid = true
}
