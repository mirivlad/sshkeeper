//go:build windows

package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeSSH writes a batch script that stands in for ssh.exe.
func writeFakeSSH(t *testing.T, lines ...string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-ssh.cmd")
	body := "@echo off\r\n" + strings.Join(lines, "\r\n") + "\r\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	return script
}

func TestConPTYTypesPasswordAtPrompt(t *testing.T) {
	script := writeFakeSSH(t,
		`set /p secret=user@host's password: `,
		`if "%secret%"=="s3cret" (echo SSHKEEPER_OK) else (echo WRONG PASSWORD)`,
	)

	ok, output := connectWithPasswordAndRead("cmd.exe", []string{"/d", "/c", script}, "s3cret", 15)
	if !ok {
		t.Fatalf("expected success, got %q", output)
	}
	if !strings.Contains(output, "SSHKEEPER_OK") {
		t.Fatalf("expected the password to be accepted, got %q", output)
	}
	if strings.Contains(output, "\x1b") {
		t.Fatalf("expected plain text output, got %q", output)
	}
}

func TestConPTYTimesOutWithoutHanging(t *testing.T) {
	script := writeFakeSSH(t, `ping -n 30 127.0.0.1 >nul`)

	done := make(chan struct{})
	var ok bool
	var output string
	go func() {
		ok, output = connectWithPasswordAndRead("cmd.exe", []string{"/d", "/c", script}, "unused", 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("connectWithPasswordAndRead did not return after its timeout")
	}
	if ok || output != "connection timeout" {
		t.Fatalf("expected a timeout, got ok=%v output=%q", ok, output)
	}
}
