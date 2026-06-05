package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

var (
	mu      sync.Mutex
	states  = map[int64]*model.TunnelState{}
	dataDir string
)

// Init initializes the tunnel state manager with the data directory.
func Init(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	dataDir = dir
	states = map[int64]*model.TunnelState{}
	return loadStates()
}

// StateFilePath returns the path to the tunnel state file.
func StateFilePath() string {
	return filepath.Join(dataDir, "tunnels.json")
}

func loadStates() error {
	path := StateFilePath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read tunnel states: %w", err)
	}
	var list []*model.TunnelState
	if err := json.Unmarshal(b, &list); err != nil {
		return fmt.Errorf("unmarshal tunnel states: %w", err)
	}
	for _, s := range list {
		states[s.ID] = s
	}
	return nil
}

func saveStates() error {
	path := StateFilePath()
	list := make([]*model.TunnelState, 0, len(states))
	for _, s := range states {
		list = append(list, s)
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tunnel states: %w", err)
	}
	os.MkdirAll(dataDir, 0700)
	return os.WriteFile(path, b, 0600)
}

// List returns all tunnel states.
func List() []*model.TunnelState {
	mu.Lock()
	defer mu.Unlock()
	result := make([]*model.TunnelState, 0, len(states))
	for _, s := range states {
		result = append(result, s)
	}
	return result
}

// Get returns a tunnel state by ID.
func Get(id int64) *model.TunnelState {
	mu.Lock()
	defer mu.Unlock()
	return states[id]
}

// Start starts a tunnel for the given server with its forwards.
func Start(cfg *config.Config, server *model.Server, forwards []*model.Forward, forwardOnly bool) (*model.TunnelState, error) {
	mu.Lock()
	defer mu.Unlock()

	// Filter enabled forwards
	var active []*model.Forward
	for _, f := range forwards {
		if f.Enabled {
			active = append(active, f)
		}
	}

	sshArgs := ssh.BuildSSHArgs(server, active, forwardOnly)
	args := make([]string, len(sshArgs))
	copy(args, sshArgs)

	cmd := exec.Command(cfg.SSH.Binary, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tunnel: %w", err)
	}

	forwardIDs := make([]int64, len(active))
	for i, f := range active {
		forwardIDs[i] = f.ID
	}

	id := time.Now().UnixNano()
	state := &model.TunnelState{
		ID:          id,
		ServerID:    server.ID,
		ServerAlias: server.Alias,
		Name:        fmt.Sprintf("Tunnel to %s", server.Alias),
		PID:         cmd.Process.Pid,
		ForwardIDs:  forwardIDs,
		StartedAt:   time.Now(),
	}

	states[id] = state
	if err := saveStates(); err != nil {
		delete(states, id)
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("save tunnel state: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		delete(states, id)
		return nil, fmt.Errorf("release tunnel process: %w", err)
	}

	return state, nil
}

// Stop stops a tunnel by ID.
func Stop(id int64) error {
	mu.Lock()
	defer mu.Unlock()

	state, ok := states[id]
	if !ok {
		return fmt.Errorf("tunnel %d not found", id)
	}

	if state.PID > 0 {
		proc, err := os.FindProcess(state.PID)
		if err == nil {
			proc.Kill()
		}
	}

	delete(states, id)
	return saveStates()
}

// StopAll stops all running tunnels.
func StopAll() error {
	mu.Lock()
	defer mu.Unlock()

	for id, state := range states {
		if state.PID > 0 {
			proc, _ := os.FindProcess(state.PID)
			if proc != nil {
				proc.Kill()
			}
		}
		delete(states, id)
	}
	return saveStates()
}

// IsRunning checks if a tunnel process is still running.
func IsRunning(id int64) bool {
	mu.Lock()
	defer mu.Unlock()

	state, ok := states[id]
	if !ok {
		return false
	}
	if state.PID <= 0 {
		return false
	}
	proc, err := os.FindProcess(state.PID)
	if err != nil {
		return false
	}
	// Signal 0 just checks if process exists
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
