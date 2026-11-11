package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

type passphraseAgent struct {
	pid    int
	dir    string
	socket string
}

// SupportsPassphraseBackground reports whether a platform can unlock an
// encrypted key for a detached SSH process without exposing its passphrase.
func SupportsPassphraseBackground() bool { return supportsPassphraseBackground() }

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

// Reload re-reads the state file so tunnels started by another sshkeeper
// process become visible. On a read error the previous states are kept.
func Reload() error {
	mu.Lock()
	defer mu.Unlock()

	if dataDir == "" {
		return nil
	}
	previous := states
	states = map[int64]*model.TunnelState{}
	if err := loadStates(); err != nil {
		states = previous
		return err
	}
	return nil
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
	dirty := false
	for _, state := range list {
		if state == nil || state.PID <= 0 || !processRunning(state.PID) {
			if state != nil {
				if state.ConfigPath != "" {
					_ = os.Remove(state.ConfigPath)
				}
				stopPassphraseAgent(state.AgentPID, state.AgentDir)
			}
			dirty = true
			continue
		}
		states[state.ID] = state
	}
	if dirty {
		return saveStates()
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

// Start starts a tunnel without profile resolution (legacy/direct routes).
func Start(cfg *config.Config, server *model.Server, forwards []*model.Forward, forwardOnly bool) (*model.TunnelState, error) {
	return StartResolved(cfg, server, forwards, forwardOnly, nil)
}

// StartResolved starts a background tunnel and retains any generated OpenSSH
// config until the tunnel is stopped.
func StartResolved(cfg *config.Config, server *model.Server, forwards []*model.Forward, forwardOnly bool, resolve ssh.ProfileResolver) (*model.TunnelState, error) {
	return StartResolvedWithVault(cfg, server, forwards, forwardOnly, resolve, nil)
}

// StartResolvedWithVault unlocks a passphrase-protected SSH identity into a
// private, per-tunnel agent. The agent survives sshkeeper, but is terminated
// with its tunnel. Password-authenticated tunnels still require a foreground
// PTY and are never silently detached.
func StartResolvedWithVault(cfg *config.Config, server *model.Server, forwards []*model.Forward, forwardOnly bool, resolve ssh.ProfileResolver, getVault ssh.VaultFunc) (*model.TunnelState, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := ssh.EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return nil, err
	}
	if server.AuthMethod == model.AuthPassword {
		return nil, fmt.Errorf("password-authenticated background tunnels require a foreground PTY")
	}
	if server.AuthMethod == model.AuthKeyPassphrase && !supportsPassphraseBackground() {
		return nil, fmt.Errorf("background key+passphrase tunnels are unsupported on this platform; use a foreground tunnel or an SSH agent")
	}

	// Filter enabled forwards
	var active []*model.Forward
	for _, f := range forwards {
		if f != nil && f.Enabled {
			active = append(active, f)
		}
	}

	if len(active) == 0 {
		return nil, fmt.Errorf("no enabled forwards for %s", server.Alias)
	}

	invocation, err := ssh.PrepareSSHInvocation(server, active, forwardOnly, resolve)
	if err != nil {
		return nil, err
	}
	args := append([]string(nil), invocation.Args...)

	var unlocked *passphraseAgent
	if server.AuthMethod == model.AuthKeyPassphrase {
		if getVault == nil {
			invocation.Cleanup()
			return nil, fmt.Errorf("vault access is required for background key+passphrase tunnels")
		}
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			invocation.Cleanup()
			return nil, fmt.Errorf("get key passphrase from vault: %w", err)
		}
		unlocked, err = startPassphraseAgent(server.IdentityFile, passphrase)
		if err != nil {
			invocation.Cleanup()
			return nil, err
		}
		// Never ask for a key passphrase in the detached process. OpenSSH
		// authenticates using the identity already loaded into the agent.
		args = append([]string{"-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes"}, args...)
	}

	cleanupAgent := func() {
		if unlocked != nil {
			stopPassphraseAgent(unlocked.pid, unlocked.dir)
		}
	}

	cmd := exec.Command(cfg.SSH.Binary, args...)
	cmd.Env = os.Environ()
	if unlocked != nil {
		filtered := make([]string, 0, len(cmd.Env)+1)
		for _, value := range cmd.Env {
			if !strings.HasPrefix(value, "SSH_AUTH_SOCK=") {
				filtered = append(filtered, value)
			}
		}
		cmd.Env = append(filtered, "SSH_AUTH_SOCK="+unlocked.socket)
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	prepareBackgroundCommand(cmd)

	if err := cmd.Start(); err != nil {
		cleanupAgent()
		invocation.Cleanup()
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
		ConfigPath:  invocation.ConfigPath,
		StartedAt:   time.Now(),
	}

	if unlocked != nil {
		state.AgentPID = unlocked.pid
		state.AgentDir = unlocked.dir
	}

	states[id] = state
	if err := saveStates(); err != nil {
		delete(states, id)
		_ = cmd.Process.Kill()
		cleanupAgent()
		invocation.Cleanup()
		return nil, fmt.Errorf("save tunnel state: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		delete(states, id)
		_ = cmd.Process.Kill()
		cleanupAgent()
		invocation.Cleanup()
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

	if state.ConfigPath != "" {
		_ = os.Remove(state.ConfigPath)
	}
	stopPassphraseAgent(state.AgentPID, state.AgentDir)
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
		if state.ConfigPath != "" {
			_ = os.Remove(state.ConfigPath)
		}
		stopPassphraseAgent(state.AgentPID, state.AgentDir)
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
	return processRunning(state.PID)
}
