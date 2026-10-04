package ssh

import (
	"bytes"
	"io"
	"testing"
)

type promptFakePlatform struct {
	writes bytes.Buffer
}

func (p *promptFakePlatform) Read([]byte) (int, error)       { return 0, io.EOF }
func (p *promptFakePlatform) Write(b []byte) (int, error)    { return p.writes.Write(b) }
func (p *promptFakePlatform) Resize(width, height int) error { return nil }
func (p *promptFakePlatform) Wait() error                    { return nil }
func (p *promptFakePlatform) Kill() error                    { return nil }
func (p *promptFakePlatform) Close() error                   { return nil }

func TestInteractiveProcessSendsStoredSecretOnlyAtPasswordPrompt(t *testing.T) {
	platform := &promptFakePlatform{}
	p := &InteractiveProcess{platform: platform, secret: []byte("s3cret")}

	p.maybeSendSecret([]byte("Connecting...\r\n"))
	if platform.writes.Len() != 0 {
		t.Fatalf("secret sent before prompt: %q", platform.writes.String())
	}
	p.maybeSendSecret([]byte("user@host's password: "))
	if got := platform.writes.String(); got != "s3cret\r" {
		t.Fatalf("prompt write = %q", got)
	}
	p.maybeSendSecret([]byte("user@host's password: "))
	if got := platform.writes.String(); got != "s3cret\r" {
		t.Fatalf("secret sent twice: %q", got)
	}
	if len(p.secret) != 0 {
		t.Fatal("secret bytes were not cleared after use")
	}
}
