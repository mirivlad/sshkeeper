package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

var sshConfigCmd = &cobra.Command{
	Use:   "ssh-config",
	Short: "OpenSSH config management",
}

var sshConfigGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate OpenSSH config from server profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		if err := ssh.WriteConfig(servers); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		home, _ := os.UserHomeDir()
		fmt.Printf("Config written to: %s/.ssh/config.d/sshkeeper.conf\n", home)
		return nil
	},
}

var sshConfigInstallIncludeCmd = &cobra.Command{
	Use:   "install-include",
	Short: "Add Include directive to ~/.ssh/config",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ssh.InstallInclude(); err != nil {
			return fmt.Errorf("install include: %w", err)
		}
		fmt.Println("Include directive added to ~/.ssh/config")
		return nil
	},
}

func init() {
	sshConfigCmd.AddCommand(sshConfigGenerateCmd)
	sshConfigCmd.AddCommand(sshConfigInstallIncludeCmd)
}
