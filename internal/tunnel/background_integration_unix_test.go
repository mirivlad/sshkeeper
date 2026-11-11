//go:build !windows

package tunnel

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
	gossh "golang.org/x/crypto/ssh"
)

func TestPassphraseBackgroundTunnelForwardsTraffic(t *testing.T) {
	for _, tool := range []string{"ssh", "ssh-agent", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	if err := Init(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}

	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "private-key-pass", "-f", keyFile).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, output)
	}
	pubBytes, err := os.ReadFile(keyFile + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	pub, _, _, _, err := gossh.ParseAuthorizedKey(pubBytes)
	if err != nil {
		t.Fatal(err)
	}
	_, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(hostPrivate)
	if err != nil {
		t.Fatal(err)
	}
	sshServerConfig := &gossh.ServerConfig{
		PublicKeyCallback: func(_ gossh.ConnMetadata, presented gossh.PublicKey) (*gossh.Permissions, error) {
			if !bytes.Equal(presented.Marshal(), pub.Marshal()) {
				return nil, fmt.Errorf("unknown SSH identity")
			}
			return nil, nil
		},
	}
	sshServerConfig.AddHostKey(signer)

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.WriteString(conn, "portainer-ready")
			}()
		}
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				remote, channels, reqs, err := gossh.NewServerConn(raw, sshServerConfig)
				if err != nil {
					_ = raw.Close()
					return
				}
				defer remote.Close()
				go gossh.DiscardRequests(reqs)
				for incoming := range channels {
					if incoming.ChannelType() != "direct-tcpip" {
						_ = incoming.Reject(gossh.UnknownChannelType, "only direct TCP forwarding is supported")
						continue
					}
					var request struct {
						Host string
						Port uint32
						OriginHost string
						OriginPort uint32
					}
					if err := gossh.Unmarshal(incoming.ExtraData(), &request); err != nil {
						_ = incoming.Reject(gossh.ConnectionFailed, "invalid payload")
						continue
					}
					dst, err := net.DialTimeout("tcp", net.JoinHostPort(request.Host, strconv.Itoa(int(request.Port))), 2*time.Second)
					if err != nil {
						_ = incoming.Reject(gossh.ConnectionFailed, "destination unavailable")
						continue
					}
					channel, pending, err := incoming.Accept()
					if err != nil {
						_ = dst.Close()
						continue
					}
					go gossh.DiscardRequests(pending)
					go func() {
						_, _ = io.Copy(channel, dst)
						_ = channel.Close()
						_ = dst.Close()
					}()
					go func() { _, _ = io.Copy(dst, channel) }()
				}
			}()
		}
	}()

	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localPort := local.Addr().(*net.TCPAddr).Port
	_ = local.Close()
	state, err := StartResolvedWithVault(
		&config.Config{SSH: config.SSHConfig{Binary: "ssh"}},
		&model.Server{
			ID: 1, Alias: "encrypted", Host: "127.0.0.1",
			Port: listener.Addr().(*net.TCPAddr).Port, User: "test",
			AuthMethod: model.AuthKeyPassphrase, IdentityFile: keyFile,
		},
		[]*model.Forward{{
			ID: 1, Type: model.ForwardLocal, Enabled: true,
			LocalAddr: "127.0.0.1", LocalPort: localPort,
			RemoteAddr: "127.0.0.1", RemotePort: backend.Addr().(*net.TCPAddr).Port,
		}},
		true, nil,
		func(alias, kind string) (string, error) {
			if alias != "encrypted" || kind != "key_passphrase" {
				return "", fmt.Errorf("unexpected vault request: %s/%s", alias, kind)
			}
			return "private-key-pass", nil
		},
	)
	if err != nil {
		t.Fatalf("start background tunnel: %v", err)
	}
	defer func() { _ = Stop(state.ID) }()
	if state.AgentPID <= 0 || state.AgentDir == "" {
		t.Fatalf("missing isolated agent metadata: %+v", state)
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))
	deadline := time.Now().Add(10 * time.Second)
	var received string
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			body, readErr := io.ReadAll(conn)
			_ = conn.Close()
			if readErr == nil {
				received = string(body)
				if received == "portainer-ready" {
					break
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if received != "portainer-ready" {
		t.Fatalf("encrypted-key background SSH tunnel did not forward traffic; received %q", received)
	}
	if err := Stop(state.ID); err != nil {
		t.Fatalf("stop tunnel: %v", err)
	}
	if _, err := os.Stat(state.AgentDir); !os.IsNotExist(err) {
		t.Fatalf("stopped tunnel left agent directory behind: %v", err)
	}
}
