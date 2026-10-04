package ssh

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// interactivePlatformProcess is the OS-specific PTY/ConPTY process behind an
// embedded workspace session.
type interactivePlatformProcess interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(width, height int) error
	Wait() error
	Kill() error
	Close() error
}

// InteractiveProcess exposes an SSH process through a PTY/ConPTY without
// taking over sshkeeper's own stdin/stdout. This lets the TUI keep multiple
// live SSH sessions and switch between them.
//
// For password and key-passphrase profiles the secret is sent automatically
// to the first matching OpenSSH prompt, just like the traditional foreground
// connection path.
type InteractiveProcess struct {
	platform interactivePlatformProcess
	cleanup  func()

	mu          sync.Mutex
	secret      []byte
	secretSent  bool
	accumulated strings.Builder
	closeOnce   sync.Once
}

func StartInteractiveResolved(cfg *config.Config, server *model.Server, resolve ProfileResolver, getVault VaultFunc, width, height int) (*InteractiveProcess, error) {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return nil, err
	}
	invocation, err := PrepareSSHInvocation(server, nil, false, resolve)
	if err != nil {
		return nil, err
	}
	args := append([]string(nil), invocation.Args...)
	if strings.TrimSpace(server.StartupCommand) != "" {
		args = append(args, server.StartupCommand)
	}

	var secret string
	switch server.AuthMethod {
	case model.AuthPassword:
		secret, err = getVault(server.Alias, "ssh_password")
		if err != nil {
			invocation.Cleanup()
			return nil, fmt.Errorf("get password from vault: %w", err)
		}
	case model.AuthKeyPassphrase:
		secret, err = getVault(server.Alias, "key_passphrase")
		if err != nil {
			invocation.Cleanup()
			return nil, fmt.Errorf("get key passphrase from vault: %w", err)
		}
	}

	platform, err := startInteractivePlatform(cfg.SSH.Binary, args, width, height)
	if err != nil {
		invocation.Cleanup()
		return nil, err
	}
	return &InteractiveProcess{
		platform: platform,
		cleanup:  invocation.Cleanup,
		secret:   []byte(secret),
	}, nil
}

func (p *InteractiveProcess) Read(buf []byte) (int, error) {
	n, err := p.platform.Read(buf)
	if n > 0 {
		p.maybeSendSecret(buf[:n])
	}
	return n, err
}

func (p *InteractiveProcess) maybeSendSecret(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.secretSent || len(p.secret) == 0 {
		return
	}
	p.accumulated.Write(data)
	text := ansi.Strip(p.accumulated.String())
	if passwordPromptRe.MatchString(text) {
		secret := append([]byte(nil), p.secret...)
		p.secretSent = true
		for i := range p.secret {
			p.secret[i] = 0
		}
		p.secret = nil
		p.accumulated.Reset()
		secret = append(secret, '\r')
		_, _ = p.platform.Write(secret)
		for i := range secret {
			secret[i] = 0
		}
		return
	}
	if p.accumulated.Len() > 8192 {
		value := p.accumulated.String()
		p.accumulated.Reset()
		if len(value) > 2048 {
			value = value[len(value)-2048:]
		}
		p.accumulated.WriteString(value)
	}
}

func (p *InteractiveProcess) Write(data []byte) (int, error) {
	return p.platform.Write(data)
}

func (p *InteractiveProcess) Resize(width, height int) error {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return p.platform.Resize(width, height)
}

func (p *InteractiveProcess) Wait() error {
	err := p.platform.Wait()
	if p.cleanup != nil {
		p.cleanup()
	}
	return err
}

func (p *InteractiveProcess) Kill() error {
	return p.platform.Kill()
}

func (p *InteractiveProcess) Close() error {
	var err error
	p.closeOnce.Do(func() {
		p.mu.Lock()
		for i := range p.secret {
			p.secret[i] = 0
		}
		p.secret = nil
		p.mu.Unlock()
		err = p.platform.Close()
		if p.cleanup != nil {
			p.cleanup()
		}
	})
	return err
}
