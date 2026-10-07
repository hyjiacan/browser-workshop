//go:build !windows

package driver

import (
	"os/exec"
	"testing"
)

func TestSetDetachedUnix(t *testing.T) {
	cmd := exec.Command("true")
	setDetached(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr = nil")
	}
	if !cmd.SysProcAttr.Setsid {
		t.Error("Setsid = false, 期望为 true")
	}
}
