//go:build !windows

package driver

import (
	"os/exec"
	"syscall"
)

// setDetached configures the command to run in its own session so it survives
// the parent bws process.
func setDetached(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
