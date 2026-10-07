//go:build windows

package driver

import (
	"os/exec"
	"testing"
)

func TestSetDetachedWindows(t *testing.T) {
	cmd := exec.Command("cmd")
	setDetached(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr = nil")
	}
	const createNewProcessGroup = 0x00000200
	if cmd.SysProcAttr.CreationFlags&createNewProcessGroup == 0 {
		t.Errorf("CreationFlags = %#x, 期望设置 CREATE_NEW_PROCESS_GROUP", cmd.SysProcAttr.CreationFlags)
	}
}
