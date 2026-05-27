package ssh

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestConnectWithPasswordDoesNotConsumeInputAfterReturn(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-ssh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'password: '\nsleep 0.1\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("open stdin pty: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	stdoutSink, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout sink: %v", err)
	}
	defer stdoutSink.Close()
	os.Stdin = slave
	os.Stdout = stdoutSink
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	if err := ConnectWithPassword(script, nil, "secret"); err != nil {
		t.Fatalf("connect with password: %v", err)
	}

	if _, err := master.Write([]byte("\n")); err != nil {
		t.Fatalf("write next enter: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	readDone := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 1)
		n, _ := slave.Read(buf)
		readDone <- buf[:n]
	}()

	select {
	case got := <-readDone:
		if len(got) != 1 || got[0] != '\n' {
			t.Fatalf("expected to read newline, got %q", got)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected next Enter to remain readable after ConnectWithPassword returned")
	}
}
