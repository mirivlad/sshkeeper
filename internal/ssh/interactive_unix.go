//go:build !windows

package ssh

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixInteractiveProcess struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func startInteractivePlatform(binary string, args []string, width, height int) (interactivePlatformProcess, error) {
	cmd := exec.Command(binary, args...)
	cmd.Env = withTerminalEnv(os.Environ())
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		return nil, fmt.Errorf("start ssh with pty: %w", err)
	}
	return &unixInteractiveProcess{cmd: cmd, ptmx: ptmx}, nil
}

func (p *unixInteractiveProcess) Read(buf []byte) (int, error)  { return p.ptmx.Read(buf) }
func (p *unixInteractiveProcess) Write(buf []byte) (int, error) { return p.ptmx.Write(buf) }
func (p *unixInteractiveProcess) Resize(width, height int) error {
	return pty.Setsize(p.ptmx, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
}
func (p *unixInteractiveProcess) Wait() error { return p.cmd.Wait() }
func (p *unixInteractiveProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
func (p *unixInteractiveProcess) Close() error { return p.ptmx.Close() }

func withTerminalEnv(env []string) []string {
	result := make([]string, 0, len(env)+1)
	for _, value := range env {
		if len(value) >= 5 && value[:5] == "TERM=" {
			continue
		}
		result = append(result, value)
	}
	return append(result, "TERM=xterm-256color")
}
