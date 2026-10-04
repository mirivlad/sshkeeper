//go:build windows

package ssh

import "fmt"

type windowsInteractiveProcess struct {
	pty *conPTY
}

func startInteractivePlatform(binary string, args []string, width, height int) (interactivePlatformProcess, error) {
	p, err := startConPTY(binary, args, width, height)
	if err != nil {
		return nil, fmt.Errorf("start ssh with pseudo console: %w", err)
	}
	return &windowsInteractiveProcess{pty: p}, nil
}

func (p *windowsInteractiveProcess) Read(buf []byte) (int, error)  { return p.pty.Read(buf) }
func (p *windowsInteractiveProcess) Write(buf []byte) (int, error) { return p.pty.Write(buf) }
func (p *windowsInteractiveProcess) Resize(width, height int) error {
	return p.pty.resize(width, height)
}
func (p *windowsInteractiveProcess) Wait() error { return p.pty.wait() }
func (p *windowsInteractiveProcess) Kill() error {
	p.pty.kill()
	return nil
}
func (p *windowsInteractiveProcess) Close() error {
	p.pty.close()
	return nil
}
