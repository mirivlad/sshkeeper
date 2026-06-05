package tunnel

import (
	"os/exec"
	"testing"
	"time"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestInitClearsPreviousInMemoryStates(t *testing.T) {
	if err := Init(t.TempDir()); err != nil {
		t.Fatalf("init first dir: %v", err)
	}
	states[123] = &model.TunnelState{ID: 123, ServerAlias: "old", PID: 1}

	if err := Init(t.TempDir()); err != nil {
		t.Fatalf("init second dir: %v", err)
	}

	if got := Get(123); got != nil {
		t.Fatalf("expected init to clear stale state, got %#v", got)
	}
}

func TestIsRunningDetectsLiveProcess(t *testing.T) {
	if err := Init(t.TempDir()); err != nil {
		t.Fatalf("init: %v", err)
	}

	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	states[1] = &model.TunnelState{ID: 1, ServerAlias: "live", PID: cmd.Process.Pid, StartedAt: time.Now()}

	if !IsRunning(1) {
		t.Fatalf("expected pid %d to be detected as running", cmd.Process.Pid)
	}
}
