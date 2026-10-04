//go:build !windows

package ssh

import (
	"strings"
	"testing"
	"time"
)

func TestWorkspacePTYStartsAndCapturesOutput(t *testing.T) {
	proc, err := startInteractivePlatform("/bin/sh", []string{"-c", "printf 'WORKSPACE_UNIX_OK\\n'"}, 80, 24)
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
	// A Unix PTY reports EOF/EIO after the child exits. Let the reader drain
	// the final frame before closing the descriptor, matching Session's lifecycle.
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		_ = proc.Close()
		select {
		case <-readDone:
		case <-time.After(3 * time.Second):
			t.Fatal("PTY reader did not finish")
		}
	}
	_ = proc.Close()
	if !strings.Contains(output.String(), "WORKSPACE_UNIX_OK") {
		t.Fatalf("workspace output = %q", output.String())
	}
}
