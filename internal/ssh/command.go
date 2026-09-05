package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/model"
)

type VaultFunc func(serverAlias string, secretType string) (string, error)

const windowsOpenSSHInstallHint = "Install OpenSSH Client via Windows Optional Features or PowerShell:\nAdd-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0"

func EnsureSSHBinary(binary string) error {
	return validateSSHBinaryForOS(runtime.GOOS, binary, exec.LookPath)
}

func validateSSHBinaryForOS(goos string, binary string, lookPath func(string) (string, error)) error {
	if goos == "windows" {
		if _, err := lookPath("ssh.exe"); err != nil {
			return fmt.Errorf("ssh.exe not found in PATH. Windows build is experimental and requires OpenSSH Client.\n%s", windowsOpenSSHInstallHint)
		}
		return nil
	}

	if strings.TrimSpace(binary) == "" {
		binary = "ssh"
	}
	if _, err := lookPath(binary); err != nil {
		return fmt.Errorf("ssh binary not found (%s): %w", binary, err)
	}
	return nil
}

func runPrepared(cfg *config.Config, args []string, server *model.Server, getVault VaultFunc) error {
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

func insertBeforeTarget(args []string, values ...string) []string {
	if len(args) == 0 {
		return append([]string(nil), values...)
	}
	result := make([]string, 0, len(args)+len(values))
	result = append(result, args[:len(args)-1]...)
	result = append(result, values...)
	result = append(result, args[len(args)-1])
	return result
}

func ConnectResolved(cfg *config.Config, server *model.Server, resolve ProfileResolver, getVault VaultFunc) error {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return err
	}
	invocation, err := PrepareSSHInvocation(server, nil, false, resolve)
	if err != nil {
		return err
	}
	defer invocation.Cleanup()
	args := append([]string(nil), invocation.Args...)
	if strings.TrimSpace(server.StartupCommand) != "" {
		args = append(args, server.StartupCommand)
	}
	return runPrepared(cfg, args, server, getVault)
}

func Connect(cfg *config.Config, server *model.Server, getVault VaultFunc) error {
	return ConnectResolved(cfg, server, nil, getVault)
}

func RunCommandResolved(cfg *config.Config, server *model.Server, resolve ProfileResolver, getVault VaultFunc, command string) error {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return err
	}
	invocation, err := PrepareSSHInvocation(server, nil, false, resolve)
	if err != nil {
		return err
	}
	defer invocation.Cleanup()
	args := append(append([]string(nil), invocation.Args...), command)
	return runPrepared(cfg, args, server, getVault)
}

func RunCommand(cfg *config.Config, server *model.Server, getVault VaultFunc, command string) error {
	return RunCommandResolved(cfg, server, nil, getVault, command)
}

func RunCommandOutputResolved(cfg *config.Config, server *model.Server, resolve ProfileResolver, getVault VaultFunc, command string) (string, error) {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return "", err
	}
	invocation, err := PrepareSSHInvocation(server, nil, false, resolve)
	if err != nil {
		return "", err
	}
	defer invocation.Cleanup()
	args := insertBeforeTarget(invocation.Args, "-o", fmt.Sprintf("ConnectTimeout=%d", cfg.SSH.ConnectTimeoutSec))

	switch server.AuthMethod {
	case model.AuthPassword:
		args = insertBeforeTarget(args, "-o", "NumberOfPasswordPrompts=1")
		args = append(args, command)
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
		args = insertBeforeTarget(args, "-o", "NumberOfPasswordPrompts=1")
		args = append(args, command)
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
		args = insertBeforeTarget(args, "-o", "BatchMode=yes")
		args = append(args, command)
		cmd := exec.Command(cfg.SSH.Binary, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return string(output), err
		}
		return string(output), nil
	}
}

func RunCommandOutput(cfg *config.Config, server *model.Server, getVault VaultFunc, command string) (string, error) {
	return RunCommandOutputResolved(cfg, server, nil, getVault, command)
}

func TestResolved(cfg *config.Config, server *model.Server, resolve ProfileResolver, getVault VaultFunc) (bool, string) {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return false, err.Error()
	}
	invocation, err := PrepareSSHInvocation(server, nil, false, resolve)
	if err != nil {
		return false, err.Error()
	}
	defer invocation.Cleanup()
	args := insertBeforeTarget(invocation.Args, "-o", fmt.Sprintf("ConnectTimeout=%d", cfg.SSH.ConnectTimeoutSec))

	switch server.AuthMethod {
	case model.AuthPassword:
		args = insertBeforeTarget(args, "-o", "NumberOfPasswordPrompts=1")
		password, err := getVault(server.Alias, "ssh_password")
		if err != nil {
			return false, fmt.Sprintf("vault error: %v", err)
		}
		return testWithPassword(cfg, append(args, cfg.SSH.TestCommand), password)
	case model.AuthKeyPassphrase:
		args = insertBeforeTarget(args, "-o", "NumberOfPasswordPrompts=1")
		passphrase, err := getVault(server.Alias, "key_passphrase")
		if err != nil {
			return false, fmt.Sprintf("vault error: %v", err)
		}
		return testWithPassword(cfg, append(args, cfg.SSH.TestCommand), passphrase)
	default:
		args = insertBeforeTarget(args, "-o", "BatchMode=yes")
		args = append(args, cfg.SSH.TestCommand)
		cmd := exec.Command(cfg.SSH.Binary, args...)
		cmd.Stdin = nil
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false, strings.TrimSpace(string(output))
		}
		result := strings.TrimSpace(string(output))
		if result == "SSHKEEPER_OK" || strings.Contains(result, "SSHKEEPER_OK") {
			return true, ""
		}
		return false, result
	}
}

func Test(cfg *config.Config, server *model.Server, getVault VaultFunc) (bool, string) {
	return TestResolved(cfg, server, nil, getVault)
}

func testWithPassword(cfg *config.Config, args []string, password string) (bool, string) {
	ok, output := connectWithPasswordAndRead(cfg.SSH.Binary, args, password, cfg.SSH.ConnectTimeoutSec)
	if !ok {
		return false, output
	}
	result := strings.TrimSpace(output)
	if result == "SSHKEEPER_OK" || strings.Contains(result, "SSHKEEPER_OK") {
		return true, ""
	}
	return false, result
}

func ConnectWithForwardsResolved(cfg *config.Config, server *model.Server, forwards []*model.Forward, forwardOnly bool, resolve ProfileResolver, getVault VaultFunc) error {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return err
	}
	invocation, err := PrepareSSHInvocation(server, forwards, forwardOnly, resolve)
	if err != nil {
		return err
	}
	defer invocation.Cleanup()
	return runPrepared(cfg, invocation.Args, server, getVault)
}

func ConnectWithArgs(cfg *config.Config, args []string, vaultFunc VaultFunc, server *model.Server) error {
	if err := EnsureSSHBinary(cfg.SSH.Binary); err != nil {
		return err
	}
	return runPrepared(cfg, args, server, vaultFunc)
}

func BuildForwardArgs(forwards []*model.Forward, exitOnForwardFailure bool) []string {
	var args []string
	for _, f := range forwards {
		switch f.Type {
		case model.ForwardLocal:
			listen := fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort)
			target := fmt.Sprintf("%s:%d", f.RemoteAddr, f.RemotePort)
			args = append(args, "-L", listen+":"+target)
		case model.ForwardRemote:
			listen := fmt.Sprintf("%s:%d", f.RemoteAddr, f.RemotePort)
			target := fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort)
			args = append(args, "-R", listen+":"+target)
		case model.ForwardDynamic:
			args = append(args, "-D", fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort))
		}
	}
	if exitOnForwardFailure && len(forwards) > 0 {
		args = append(args, "-o", "ExitOnForwardFailure=yes")
	}
	return args
}

// BuildSSHArgs builds the SSH command arguments for a server profile.
func BuildSSHArgs(server *model.Server, forwards []*model.Forward, forwardOnly bool) []string {
	var args []string

	port := server.Port
	if port == 0 {
		port = 22
	}
	args = append(args, "-p", fmt.Sprintf("%d", port))

	if server.IdentityFile != "" {
		args = append(args, "-i", server.IdentityFile)
	}

	// Use Route if available, fall back to raw ProxyJump for backward compatibility
	routeArgs := BuildRouteArgs(server.Route)
	if len(routeArgs) > 0 {
		args = append(args, routeArgs...)
	} else if server.ProxyJump != "" {
		args = append(args, "-J", server.ProxyJump)
	}

	// Port forwarding
	if len(forwards) > 0 {
		args = append(args, BuildForwardArgs(forwards, true)...)
	}

	args = append(args, "-o", "StrictHostKeyChecking=accept-new")

	if forwardOnly {
		args = append(args, "-N")
	}

	target := server.Host
	if strings.TrimSpace(server.User) != "" {
		target = fmt.Sprintf("%s@%s", server.User, server.Host)
	}
	args = append(args, target)

	return args
}

// BuildSSHArgsSimple builds SSH args without forwards (backward compatible).
func BuildSSHArgsSimple(server *model.Server) []string {
	return BuildSSHArgs(server, nil, false)
}
