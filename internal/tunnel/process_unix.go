//go:build !windows

package tunnel

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// prepareBackgroundCommand puts the tunnel in its own session so closing the
// terminal that launched sshkeeper does not send it SIGHUP.
func prepareBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func processRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
