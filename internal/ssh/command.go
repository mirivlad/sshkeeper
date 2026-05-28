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
	if strings.TrimSpace(server.StartupCommand) != "" {
		args = append(args, server.StartupCommand)
	}

	switch server.AuthMethod {
	case model.AuthPassword:
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return fmt.Errorf("get password from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, password)

	case model.AuthKeyPassphrase:
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			return fmt.Errorf("get key passphrase from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, passphrase)

	default:
		// key and agent auth use direct OpenSSH execution.
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

func RunCommand(cfg *config.Config, server *model.Server, getVault VaultFunc, command string) error {
	args := BuildSSHArgs(server)
	args = append(args, command)

	switch server.AuthMethod {
	case model.AuthPassword:
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return fmt.Errorf("get password from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, password)
	case model.AuthKeyPassphrase:
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			return fmt.Errorf("get key passphrase from vault: %w", err)
		}
		return ConnectWithPassword(cfg.SSH.Binary, args, passphrase)
	default:
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

func RunCommandOutput(cfg *config.Config, server *model.Server, getVault VaultFunc, command string) (string, error) {
	args := BuildSSHArgs(server)
	args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", cfg.SSH.ConnectTimeoutSec))

	switch server.AuthMethod {
	case model.AuthPassword:
		args = append(args, "-o", "NumberOfPasswordPrompts=1", command)
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return "", fmt.Errorf("get password from vault: %w", err)
		}
		ok, output := connectWithPasswordAndRead(cfg.SSH.Binary, args, password, cfg.SSH.ConnectTimeoutSec)
		if !ok {
			return output, fmt.Errorf("ssh command failed")
		}
		return output, nil
	case model.AuthKeyPassphrase:
		args = append(args, "-o", "NumberOfPasswordPrompts=1", command)
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			return "", fmt.Errorf("get key passphrase from vault: %w", err)
		}
		ok, output := connectWithPasswordAndRead(cfg.SSH.Binary, args, passphrase, cfg.SSH.ConnectTimeoutSec)
		if !ok {
			return output, fmt.Errorf("ssh command failed")
		}
		return output, nil
	default:
		args = append(args, "-o", "BatchMode=yes", command)
		cmd := exec.Command(cfg.SSH.Binary, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return string(output), err
		}
		return string(output), nil
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

	case model.AuthKeyPassphrase:
		args = append(args, "-o", "NumberOfPasswordPrompts=1")
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			return false, fmt.Sprintf("vault error: %v", err)
		}
		return testWithPassword(cfg, args, passphrase)

	default:
		// key and agent auth should not prompt during tests.
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
