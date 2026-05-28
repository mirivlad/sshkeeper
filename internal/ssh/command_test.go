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
	if err := os.WriteFile(script, []byte(`#!/bin/sh
printf 'Enter passphrase for key: '
IFS= read -r passphrase
if [ "$passphrase" = "key-secret" ]; then
  echo SSHKEEPER_OK
  exit 0
fi
echo denied
exit 1
`), 0o700); err != nil {
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
	if err := os.WriteFile(script, []byte(fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
`, argsFile)), 0o700); err != nil {
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
