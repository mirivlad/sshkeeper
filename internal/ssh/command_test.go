package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestKeyPassphraseTestUsesVaultSecret(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-ssh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'Enter passphrase for key: '\nIFS= read -r passphrase\nif [ \"$passphrase\" = \"key-secret\" ]; then\n  echo SSHKEEPER_OK\n  exit 0\nfi\necho denied\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}

	cfg := &config.Config{
		SSH: config.SSHConfig{
			Binary:            script,
			ConnectTimeoutSec: 2,
			TestCommand:       "echo SSHKEEPER_OK",
		},
	}
	server := &model.Server{
		Alias:        "prod",
		Host:         "example.org",
		Port:         22,
		User:         "root",
		AuthMethod:   model.AuthKeyPassphrase,
		IdentityFile: "/tmp/test-key",
	}

	ok, errText := Test(cfg, server, func(alias string, secretType string) (string, error) {
		if alias != "prod" || secretType != "key_passphrase" {
			return "", fmt.Errorf("unexpected secret lookup %s %s", alias, secretType)
		}
		return "key-secret", nil
	})

	if !ok {
		t.Fatalf("expected key passphrase test to pass, error: %s", errText)
	}
}

func TestKeyPassphraseTestReportsVaultError(t *testing.T) {
	cfg := &config.Config{
		SSH: config.SSHConfig{
			Binary:            "ssh",
			ConnectTimeoutSec: 1,
			TestCommand:       "echo SSHKEEPER_OK",
		},
	}
	server := &model.Server{
		Alias:      "prod",
		Host:       "example.org",
		Port:       22,
		User:       "root",
		AuthMethod: model.AuthKeyPassphrase,
	}

	ok, errText := Test(cfg, server, func(alias string, secretType string) (string, error) {
		return "", fmt.Errorf("missing secret")
	})

	if ok {
		t.Fatal("expected key passphrase test to fail when vault lookup fails")
	}
	if !strings.Contains(errText, "vault error: missing secret") {
		t.Fatalf("expected vault error, got %q", errText)
	}
}

func TestConnectRunsStartupCommand(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := filepath.Join(dir, "fake-ssh")
	if err := os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", argsFile)), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}

	cfg := &config.Config{SSH: config.SSHConfig{Binary: script}}
	server := &model.Server{
		Alias:          "prod",
		Host:           "example.org",
		Port:           22,
		User:           "root",
		AuthMethod:     model.AuthKey,
		StartupCommand: "tmux attach -t ops",
	}

	if err := Connect(cfg, server, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if !strings.Contains(string(data), "tmux attach -t ops") {
		t.Fatalf("expected startup command in ssh args, got:\n%s", data)
	}
}

func TestBuildSSHArgs_Simple(t *testing.T) {
	server := &model.Server{Host: "example.org", Port: 22, User: "root"}
	args := BuildSSHArgsSimple(server)
	expected := []string{"-p", "22", "-o", "StrictHostKeyChecking=accept-new", "root@example.org"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Fatalf("arg[%d]: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestBuildSSHArgs_CustomPort(t *testing.T) {
	server := &model.Server{Host: "example.org", Port: 2222, User: "deploy"}
	args := BuildSSHArgsSimple(server)
	if args[1] != "2222" {
		t.Fatalf("expected port 2222, got %s", args[1])
	}
	if args[len(args)-1] != "deploy@example.org" {
		t.Fatalf("expected target deploy@example.org, got %s", args[len(args)-1])
	}
}

func TestBuildSSHArgs_WithIdentityFile(t *testing.T) {
	server := &model.Server{Host: "example.org", Port: 22, User: "root", IdentityFile: "~/.ssh/id_ed25519"}
	args := BuildSSHArgsSimple(server)
	found := false
	for _, a := range args {
		if a == "-i" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -i flag in args: %v", args)
	}
}

func TestBuildSSHArgs_WithProxyJump(t *testing.T) {
	server := &model.Server{Host: "internal.example.org", Port: 22, User: "root", ProxyJump: "bastion.example.org"}
	args := BuildSSHArgsSimple(server)
	found := false
	for _, a := range args {
		if a == "-J" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -J flag in args: %v", args)
	}
}

func TestBuildSSHArgs_ForwardLocal(t *testing.T) {
	server := &model.Server{Host: "db.internal", Port: 22, User: "root"}
	forwards := []*model.Forward{{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}}
	args := BuildSSHArgs(server, forwards, false)
	found := false
	for _, a := range args {
		if a == "-L" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -L flag in args: %v", args)
	}
}

func TestBuildSSHArgs_ForwardRemote(t *testing.T) {
	server := &model.Server{Host: "web.internal", Port: 22, User: "root"}
	forwards := []*model.Forward{{Type: model.ForwardRemote, LocalAddr: "127.0.0.1", LocalPort: 8080, RemoteAddr: "0.0.0.0", RemotePort: 80}}
	args := BuildSSHArgs(server, forwards, false)
	found := false
	for _, a := range args {
		if a == "-R" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -R flag in args: %v", args)
	}
}

func TestBuildSSHArgs_ForwardDynamic(t *testing.T) {
	server := &model.Server{Host: "jump.example.org", Port: 22, User: "me"}
	forwards := []*model.Forward{{Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080}}
	args := BuildSSHArgs(server, forwards, false)
	found := false
	for _, a := range args {
		if a == "-D" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -D flag in args: %v", args)
	}
}

func TestBuildSSHArgs_ForwardOnly(t *testing.T) {
	server := &model.Server{Host: "db.internal", Port: 22, User: "root"}
	forwards := []*model.Forward{{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}}
	args := BuildSSHArgs(server, forwards, true)
	found := false
	for _, a := range args {
		if a == "-N" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -N flag in args: %v", args)
	}
}

func TestBuildSSHArgs_ExitOnForwardFailure(t *testing.T) {
	server := &model.Server{Host: "db.internal", Port: 22, User: "root"}
	forwards := []*model.Forward{{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}}
	args := BuildSSHArgs(server, forwards, false)
	found := false
	for _, a := range args {
		if a == "ExitOnForwardFailure=yes" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ExitOnForwardFailure=yes in args: %v", args)
	}
}

func TestBuildSSHArgs_MultipleForwards(t *testing.T) {
	server := &model.Server{Host: "multi.internal", Port: 22, User: "root"}
	forwards := []*model.Forward{
		{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432},
		{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 18080, RemoteAddr: "internal.web", RemotePort: 80},
		{Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080},
	}
	args := BuildSSHArgs(server, forwards, false)
	lCount, dCount := 0, 0
	for _, a := range args {
		switch a {
		case "-L":
			lCount++
		case "-D":
			dCount++
		}
	}
	if lCount != 2 {
		t.Fatalf("expected 2 -L flags, got %d: %v", lCount, args)
	}
	if dCount != 1 {
		t.Fatalf("expected 1 -D flag, got %d: %v", dCount, args)
	}
}

func TestBuildSSHArgs_RouteAndForwards(t *testing.T) {
	server := &model.Server{Host: "secure.internal", Port: 22, User: "root", Route: model.Route{Hops: []model.RouteHop{{Alias: "bastion", IsProfile: true}}}}
	forwards := []*model.Forward{{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}}
	args := BuildSSHArgs(server, forwards, false)
	hasJ, hasL := false, false
	for _, a := range args {
		if a == "-J" {
			hasJ = true
		}
		if a == "-L" {
			hasL = true
		}
	}
	if !hasJ {
		t.Fatalf("expected -J flag in args: %v", args)
	}
	if !hasL {
		t.Fatalf("expected -L flag in args: %v", args)
	}
}

func TestBuildForwardArgs_Local(t *testing.T) {
	fwd := &model.Forward{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 8080, RemoteAddr: "internal.web", RemotePort: 80}
	args := BuildForwardArgs([]*model.Forward{fwd}, true)
	expected := []string{"-L", "127.0.0.1:8080:internal.web:80", "-o", "ExitOnForwardFailure=yes"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func TestBuildForwardArgs_Remote(t *testing.T) {
	fwd := &model.Forward{Type: model.ForwardRemote, LocalAddr: "127.0.0.1", LocalPort: 2222, RemoteAddr: "0.0.0.0", RemotePort: 22}
	args := BuildForwardArgs([]*model.Forward{fwd}, true)
	expected := []string{"-R", "0.0.0.0:22:127.0.0.1:2222", "-o", "ExitOnForwardFailure=yes"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func TestBuildForwardArgs_Dynamic(t *testing.T) {
	fwd := &model.Forward{Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080}
	args := BuildForwardArgs([]*model.Forward{fwd}, true)
	expected := []string{"-D", "127.0.0.1:1080", "-o", "ExitOnForwardFailure=yes"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func TestBuildForwardArgs_Empty(t *testing.T) {
	args := BuildForwardArgs(nil, true)
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestForwardHumanExplanation(t *testing.T) {
	tests := []struct {
		fwd  *model.Forward
		want string
	}{
		{&model.Forward{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}, "Port 127.0.0.1:15432 on this machine will be forwarded through  to 127.0.0.1:5432."},
		{&model.Forward{Type: model.ForwardRemote, LocalAddr: "127.0.0.1", LocalPort: 8080, RemoteAddr: "0.0.0.0", RemotePort: 80}, "Port 0.0.0.0:80 on  will be forwarded to 127.0.0.1:8080 on this machine."},
		{&model.Forward{Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080}, "SOCKS proxy on 127.0.0.1:1080 will route traffic through ."},
	}
	for _, tt := range tests {
		got := tt.fwd.ForwardHumanExplanation("")
		if got != tt.want {
			t.Fatalf("ForwardHumanExplanation() = %q, want %q", got, tt.want)
		}
	}
}

func TestForwardListenTarget(t *testing.T) {
	tests := []struct {
		fwd    *model.Forward
		listen string
		target string
	}{
		{&model.Forward{Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432}, "127.0.0.1:15432", "127.0.0.1:5432"},
		{&model.Forward{Type: model.ForwardRemote, LocalAddr: "127.0.0.1", LocalPort: 8080, RemoteAddr: "0.0.0.0", RemotePort: 80}, "0.0.0.0:80", "127.0.0.1:8080"},
		{&model.Forward{Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080}, "127.0.0.1:1080", "SOCKS"},
	}
	for _, tt := range tests {
		if got := tt.fwd.ForwardListen(); got != tt.listen {
			t.Fatalf("ForwardListen() = %q, want %q", got, tt.listen)
		}
		if got := tt.fwd.ForwardTarget(); got != tt.target {
			t.Fatalf("ForwardTarget() = %q, want %q", got, tt.target)
		}
	}
}
