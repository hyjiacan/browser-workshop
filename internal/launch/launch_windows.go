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
	// On Windows, CREATE_NEW_PROCESS_GROUP flag
	cmd.SysProcAttr.CreationFlags = 0x00000200 // CREATE_NEW_PROCESS_GROUP
}
