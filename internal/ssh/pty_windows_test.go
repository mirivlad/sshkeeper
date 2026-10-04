//go:build windows

package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
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

func TestConPTYEnvironmentDefinesTerminalType(t *testing.T) {
	block := conPTYEnvironment()
	decoded := string(utf16.Decode(block))
	if !strings.Contains(strings.ToUpper(decoded), "TERM=XTERM-256COLOR\x00") {
		t.Fatalf("TERM missing from ConPTY environment: %q", decoded)
	}
}

func TestWorkspaceConPTYStartsAndCapturesOutput(t *testing.T) {
	proc, err := startInteractivePlatform("cmd.exe", []string{"/d", "/c", "echo WORKSPACE_OK"}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, readErr := proc.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if readErr != nil {
				return
			}
		}
	}()
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if err := proc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-readDone:
	case <-time.After(10 * time.Second):
		t.Fatal("ConPTY reader did not finish")
	}
	if !strings.Contains(output.String(), "WORKSPACE_OK") {
		t.Fatalf("workspace output = %q", output.String())
	}
}
