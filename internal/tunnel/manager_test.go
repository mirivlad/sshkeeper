package tunnel

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

func TestReloadPicksUpStatesWrittenByAnotherProcess(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tunnels.json"), []byte(`[{"id": 7, "server_alias": "db", "pid": 1}]`), 0600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := Get(7); got == nil || got.ServerAlias != "db" {
		t.Fatalf("reload did not pick up state: %#v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "tunnels.json"), []byte(`{broken`), 0600); err != nil {
		t.Fatalf("write broken state: %v", err)
	}
	if err := Reload(); err == nil {
		t.Fatal("expected error for broken state file")
	}
	if Get(7) == nil {
		t.Fatal("a failed reload must keep the previous states")
	}
}

func TestInitPrunesStoppedTunnelStates(t *testing.T) {
	dir := t.TempDir()
	staleConfig := filepath.Join(dir, "stale-ssh.conf")
	if err := os.WriteFile(staleConfig, []byte("temporary"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal([]*model.TunnelState{{
		ID: 42, ServerAlias: "old", PID: 0, ConfigPath: staleConfig, StartedAt: time.Now().Add(-time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tunnels.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := List(); len(got) != 0 {
		t.Fatalf("stale tunnel survived reload: %#v", got)
	}
	if _, err := os.Stat(staleConfig); !os.IsNotExist(err) {
		t.Fatalf("stale temporary config still exists: %v", err)
	}
	stored, err := os.ReadFile(StateFilePath())
	if err != nil {
		t.Fatal(err)
	}
	var states []*model.TunnelState
	if err := json.Unmarshal(stored, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Fatalf("state file still contains stale tunnels: %#v", states)
	}
}
