//go:build !windows

package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// A private OpenSSH agent keeps an unlocked key available to the detached
// tunnel after sshkeeper exits. Neither the passphrase nor the key is written
// to a temporary file; the agent socket lives in a 0700 directory.
func supportsPassphraseBackground() bool { return true }

var agentPIDPattern = regexp.MustCompile(`SSH_AGENT_PID=([0-9]+)`)

func startPassphraseAgent(identityFile, passphrase string) (*passphraseAgent, error) {
	if strings.TrimSpace(identityFile) == "" {
		return nil, fmt.Errorf("key+passphrase tunnel requires an identity file")
	}
	if strings.HasPrefix(identityFile, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve SSH key path: %w", err)
		}
		identityFile = filepath.Join(home, identityFile[2:])
	}
	identity, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, fmt.Errorf("read SSH identity: %w", err)
	}
	defer clear(identity)
	passphraseBytes := []byte(passphrase)
	privateKey, err := ssh.ParseRawPrivateKeyWithPassphrase(identity, passphraseBytes)
	clear(passphraseBytes)
	if err != nil {
		return nil, fmt.Errorf("unlock SSH identity (check its passphrase): %w", err)
	}

	dir, err := os.MkdirTemp("", "sk-agent-")
	if err != nil {
		return nil, fmt.Errorf("create SSH agent directory: %w", err)
	}
	socket := filepath.Join(dir, "agent.sock")
	output, err := exec.Command("ssh-agent", "-s", "-a", socket).Output()
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("start isolated ssh-agent (install OpenSSH client): %w", err)
	}
	pidMatch := agentPIDPattern.FindSubmatch(output)
	if len(pidMatch) != 2 {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("ssh-agent did not return a process ID")
	}
	pid, err := strconv.Atoi(string(pidMatch[1]))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("invalid ssh-agent process ID: %w", err)
	}
	result := &passphraseAgent{pid: pid, dir: dir, socket: socket}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		stopPassphraseAgent(pid, dir)
		return nil, fmt.Errorf("connect to isolated ssh-agent: %w", err)
	}
	defer conn.Close()
	if err := agent.NewClient(conn).Add(agent.AddedKey{PrivateKey: privateKey, Comment: "sshkeeper background tunnel"}); err != nil {
		stopPassphraseAgent(pid, dir)
		return nil, fmt.Errorf("load SSH identity into isolated agent: %w", err)
	}
	return result, nil
}

func stopPassphraseAgent(pid int, dir string) {
	// Tunnel state is persisted on disk. Never trust its directory field
	// blindly: RemoveAll on a forged or corrupt value would be destructive.
	clean := filepath.Clean(dir)
	if !filepath.IsAbs(dir) || filepath.Dir(clean) != filepath.Clean(os.TempDir()) ||
		!strings.HasPrefix(filepath.Base(clean), "sk-agent-") {
		return
	}
	// Do not signal a potentially reused PID when the private agent socket
	// is already dead. In that case only remove its stale directory.
	conn, err := net.Dial("unix", filepath.Join(clean, "agent.sock"))
	if err == nil {
		_ = conn.Close()
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	_ = os.RemoveAll(clean)
}
