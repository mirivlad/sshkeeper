package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
)

type VaultFunc func(serverAlias string, secretType string) (string, error)

func Connect(cfg *config.Config, server *model.Server, getVault VaultFunc) error {
	args := BuildSSHArgs(server)

	switch server.AuthMethod {
	case model.AuthPassword:
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return fmt.Errorf("get password from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, password)

	case model.AuthKeyPassphrase:
		// For key+passphrase, let ssh-agent handle it or prompt normally
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
	args := BuildSSHArgs(server)
	args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", cfg.SSH.ConnectTimeoutSec))

	switch server.AuthMethod {
	case model.AuthPassword:
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

// testWithPassword tests SSH connection with password auth via PTY-wrapper.
// It connects, sends the password, runs the test command, and checks the output.
func testWithPassword(cfg *config.Config, args []string, password string) (bool, string) {
	args = append(args, cfg.SSH.TestCommand)

	ok, output := connectWithPasswordAndRead(cfg.SSH.Binary, args, password, cfg.SSH.ConnectTimeoutSec)
	if !ok {
		return false, output
	}

	result := strings.TrimSpace(output)
	if result == "SSHKEEPER_OK" {
		return true, ""
	}
	// The output might have the test command echo before the result
	if strings.Contains(result, "SSHKEEPER_OK") {
		return true, ""
	}
	return false, result
}

// BuildSSHArgs builds the SSH command arguments for a server profile.
func BuildSSHArgs(server *model.Server) []string {
	var args []string

	args = append(args, "-p", fmt.Sprintf("%d", server.Port))

	if server.IdentityFile != "" {
		args = append(args, "-i", server.IdentityFile)
	}

	if server.ProxyJump != "" {
		args = append(args, "-J", server.ProxyJump)
	}

	args = append(args, "-o", "StrictHostKeyChecking=accept-new")

	target := fmt.Sprintf("%s@%s", server.User, server.Host)
	args = append(args, target)

	return args
}
