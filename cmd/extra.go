package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import servers from ~/.ssh/config",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := ssh.ImportFromSSHConfig()
		if err != nil {
			return fmt.Errorf("import: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("No servers found in ~/.ssh/config")
			return nil
		}

		imported := 0
		for _, s := range servers {
			existing, _ := appDB.GetServer(s.Alias)
			if existing != nil {
				fmt.Printf("  skip (exists): %s\n", s.Alias)
				continue
			}
			if err := appDB.CreateServer(s); err != nil {
				fmt.Printf("  error: %s: %v\n", s.Alias, err)
				continue
			}
			fmt.Printf("  imported: %s (%s@%s:%d)\n", s.Alias, s.User, s.Host, s.Port)
			imported++
		}

		fmt.Printf("\nImported %d servers.\n", imported)
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export servers to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		for _, s := range servers {
			fmt.Printf("%s\t%s@%s:%d\t%s\n", s.Alias, s.User, s.Host, s.Port, s.AuthMethod)
		}
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run <alias> <command>",
	Short: "Run a command on a server",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		command := strings.Join(args[1:], " ")

		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		// Build ssh args with the command
		sshArgs := buildSSHArgs(server)
		sshArgs = append(sshArgs, command)

		sshCmd := exec.Command(cfg.SSH.Binary, sshArgs...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr

		if err := sshCmd.Start(); err != nil {
			return fmt.Errorf("start ssh: %w", err)
		}

		return sshCmd.Wait()
	},
}

func buildSSHArgs(server *model.Server) []string {
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
