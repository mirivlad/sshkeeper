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
		imported, err := importServersFromSSHConfig(func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		})
		if err != nil {
			return err
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

		fmt.Print(formatServersExport(servers))
		return nil
	},
}

func importServersFromSSHConfig(report func(format string, args ...interface{})) (int, error) {
	servers, err := ssh.ImportFromSSHConfig()
	if err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}

	if len(servers) == 0 {
		if report != nil {
			report("No servers found in ~/.ssh/config")
		}
		return 0, nil
	}

	imported := 0
	for _, s := range servers {
		existing, _ := appDB.GetServer(s.Alias)
		if existing != nil {
			if report != nil {
				report("  skip (exists): %s", s.Alias)
			}
			continue
		}
		if err := appDB.CreateServer(s); err != nil {
			if report != nil {
				report("  error: %s: %v", s.Alias, err)
			}
			continue
		}
		if report != nil {
			report("  imported: %s (%s@%s:%d)", s.Alias, s.User, s.Host, s.Port)
		}
		imported++
	}

	return imported, nil
}

func formatServersExport(servers []*model.Server) string {
	var b strings.Builder
	for _, s := range servers {
		fmt.Fprintf(&b, "%s\t%s@%s:%d\t%s\n", s.Alias, s.User, s.Host, s.Port, s.AuthMethod)
	}
	return b.String()
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
