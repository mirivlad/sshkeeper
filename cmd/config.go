package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: tr("Configuration management", "Управление конфигурацией"),
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: tr("Show config file paths", "Показать пути файлов конфигурации"),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf(tr("Config:  %s/config.toml\n", "Конфиг: %s/config.toml\n"), cfg.ConfigDir)
		fmt.Printf(tr("DB:      %s/sshkeeper.db\n", "БД:      %s/sshkeeper.db\n"), cfg.DataDir)
		fmt.Printf(tr("Vault:   %s/vault.bin\n", "Хранилище: %s/vault.bin\n"), cfg.DataDir)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
}
