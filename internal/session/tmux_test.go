package session

import (
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestSanitizeWindowName(t *testing.T) {
	if got := sanitizeWindowName(" prod:db "); got != "prod-db" {
		t.Fatalf("sanitizeWindowName = %q, want prod-db", got)
	}
	if got := sanitizeWindowName("   "); got != "ssh" {
		t.Fatalf("empty name = %q, want ssh", got)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("prod'one")
	want := `'prod'"'"'one'`
	if got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}

func TestTmuxWindowLifecycle(t *testing.T) {
	if !Available() {
		t.Skip("tmux is not available")
	}
	t.Setenv("TMUX", "")
	oldWorkspace := dedicatedWorkspace
	dedicatedWorkspace = fmt.Sprintf("sshkeeper-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = exec.Command("tmux", "kill-session", "-t", dedicatedWorkspace).CombinedOutput()
		dedicatedWorkspace = oldWorkspace
	})

	windowID, inside, err := openWindow("smoke-server", "sleep 30")
	if err != nil {
		t.Fatalf("openWindow: %v", err)
	}
	if inside {
		t.Fatal("expected dedicated workspace outside tmux")
	}
	windows, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(windows) != 1 || windows[0].ID != windowID || windows[0].ServerAlias != "smoke-server" {
		t.Fatalf("unexpected windows: %#v", windows)
	}
	if err := Close(windowID); err != nil {
		t.Fatalf("Close: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		windows, err = List()
		if err != nil {
			t.Fatalf("List after close: %v", err)
		}
		if len(windows) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("window %s still listed after close: %#v", windowID, windows)
}
