package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
)

type VaultFunc func(serverAlias string, secretType string) (string, error)

func Connect(cfg *config.Config, server *model.Server, getVault VaultFunc) error {
	args := buildArgs(server)

	switch server.AuthMethod {
	case model.AuthPassword:
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return fmt.Errorf("get password from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, password)

	case model.AuthKeyPassphrase:
		// For key+passphrase, we need to handle the passphrase
		// For now, let ssh-agent handle it or prompt normally
		// TODO: use ssh-agent or similar
		fallthrough

	default:
		// key, agent, key+passphrase - direct execution
		cmd := exec.Command(cfg.SSH.Binary, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start ssh: %w", err)
		}

		return cmd.Wait()
	}
}

func Test(cfg *config.Config, server *model.Server, getVault VaultFunc) (bool, string) {
	args := buildArgs(server)
	args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", cfg.SSH.ConnectTimeoutSec))

	switch server.AuthMethod {
	case model.AuthPassword:
		// For password auth, we can't use BatchMode
		// Use a short timeout and try to connect
		args = append(args, "-o", "NumberOfPasswordPrompts=1")
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return false, fmt.Sprintf("vault error: %v", err)
		}
		return testWithPassword(cfg, args, password)

	default:
		// key, agent, key+passphrase
		args = append(args, "-o", "BatchMode=yes")
		args = append(args, cfg.SSH.TestCommand)

		cmd := exec.Command(cfg.SSH.Binary, args...)
		cmd.Stdin = nil

		output, err := cmd.CombinedOutput()
		if err != nil {
			return false, strings.TrimSpace(string(output))
		}

		result := strings.TrimSpace(string(output))
		if result == "SSHKEEPER_OK" {
			return true, ""
		}
		return false, result
	}
}

func testWithPassword(cfg *config.Config, args []string, password string) (bool, string) {
	// For password test, we use PTY approach with a short timeout
	// This is a simplified version - in production, use ConnectWithPassword
	// with a test command
	args = append(args, cfg.SSH.TestCommand)

	cmd := exec.Command(cfg.SSH.Binary, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Use a timeout
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return false, err.Error()
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return false, err.Error()
		}
		return true, ""
	case <-time.After(time.Duration(cfg.SSH.ConnectTimeoutSec) * time.Second):
		cmd.Process.Kill()
		return false, "connection timeout"
	}
}

func buildArgs(server *model.Server) []string {
	var args []string

	args = append(args, "-p", fmt.Sprintf("%d", server.Port))

	if server.IdentityFile != "" {
		args = append(args, "-i", server.IdentityFile)
	}

	if server.ProxyJump != "" {
		args = append(args, "-J", server.ProxyJump)
	}

	// Disable strict host key checking for first connection
	// In production, this should be configurable
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")

	target := fmt.Sprintf("%s@%s", server.User, server.Host)
	args = append(args, target)

	return args
}
