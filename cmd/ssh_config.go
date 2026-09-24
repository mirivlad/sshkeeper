package cmd

import (
	"fmt"
	"os"

	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/spf13/cobra"
)

var sshConfigCmd = &cobra.Command{
	Use:   "ssh-config",
	Short: tr("OpenSSH config management", "Управление конфигурацией OpenSSH"),
}

var sshConfigGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: tr("Generate OpenSSH config from server profiles", "Создать конфигурацию OpenSSH из профилей серверов"),
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("list servers", "получить список серверов"), err)
		}

		if err := ssh.WriteConfig(servers); err != nil {
			return fmt.Errorf("%s: %w", tr("write config", "записать конфигурацию"), err)
		}

		home, _ := os.UserHomeDir()
		fmt.Printf(tr("Config written to: %s/.ssh/config.d/sshkeeper.conf\n", "Конфигурация записана в: %s/.ssh/config.d/sshkeeper.conf\n"), home)
		return nil
	},
}

var sshConfigInstallIncludeCmd = &cobra.Command{
	Use:   "install-include",
	Short: tr("Add Include directive to ~/.ssh/config", "Добавить директиву Include в ~/.ssh/config"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ssh.InstallInclude(); err != nil {
			return fmt.Errorf("%s: %w", tr("install include", "добавить Include"), err)
		}
		fmt.Println(tr("Include directive added to ~/.ssh/config", "Директива Include добавлена в ~/.ssh/config"))
		return nil
	},
}

func init() {
	sshConfigCmd.AddCommand(sshConfigGenerateCmd)
	sshConfigCmd.AddCommand(sshConfigInstallIncludeCmd)
}
