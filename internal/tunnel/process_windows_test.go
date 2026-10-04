//go:build windows

package tunnel

import (
	"os/exec"
	"testing"
)

func TestBackgroundCommandUsesDetachedWindowsProcessGroup(t *testing.T) {
	cmd := exec.Command("ssh.exe")
	prepareBackgroundCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}
	flags := cmd.SysProcAttr.CreationFlags
	if flags&createNewProcessGroup == 0 {
		t.Fatalf("CreationFlags %#x missing CREATE_NEW_PROCESS_GROUP", flags)
	}
	if flags&createNoWindow == 0 {
		t.Fatalf("CreationFlags %#x missing CREATE_NO_WINDOW", flags)
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is false")
	}
}
