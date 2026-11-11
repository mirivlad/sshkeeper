//go:build !windows

package tunnel

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh/agent"
)

func TestPassphraseAgentLoadsEncryptedEd25519Key(t *testing.T) {
	if _, err := exec.LookPath("ssh-agent"); err != nil {
		t.Skip("ssh-agent is not installed")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen is not installed")
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "test-passphrase", "-f", keyPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate encrypted test key: %v: %s", err, output)
	}
	if _, err := startPassphraseAgent(keyPath, "wrong-passphrase"); err == nil {
		t.Fatal("wrong passphrase unexpectedly accepted")
	}

	managed, err := startPassphraseAgent(keyPath, "test-passphrase")
	if err != nil {
		t.Fatalf("start isolated agent: %v", err)
	}
	t.Cleanup(func() { stopPassphraseAgent(managed.pid, managed.dir) })
	if managed.pid <= 0 {
		t.Fatalf("invalid ssh-agent PID %d", managed.pid)
	}
	conn, err := net.Dial("unix", managed.socket)
	if err != nil {
		t.Fatalf("connect to agent: %v", err)
	}
	defer conn.Close()
	keys, err := agent.NewClient(conn).List()
	if err != nil {
		t.Fatalf("list loaded SSH keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("agent has %d identities; want 1", len(keys))
	}
	stopPassphraseAgent(managed.pid, managed.dir)
	if _, err := os.Stat(managed.dir); !os.IsNotExist(err) {
		t.Fatalf("agent private directory not removed: %v", err)
	}
}
