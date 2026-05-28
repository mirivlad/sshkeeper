package cmd

import (
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/spf13/cobra"
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

		return runCommandOnServer(server, command)
	},
}

func runCommandOnServer(server *model.Server, command string) error {
	return ssh.RunCommand(cfg, server, commandVaultFunc, command)
}

func commandVaultFunc(serverAlias string, secretType string) (string, error) {
	v := getOrCreateVault()
	if !v.IsUnlocked() {
		return "", fmt.Errorf("%s", vaultLockedProcessMessage())
	}
	vaultKey := fmt.Sprintf("server:%s:%s", serverAlias, secretType)
	data, err := v.Get(vaultKey)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
