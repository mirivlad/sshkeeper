package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Config:  %s/config.toml\n", cfg.ConfigDir)
		fmt.Printf("DB:      %s/sshkeeper.db\n", cfg.DataDir)
		fmt.Printf("Vault:   %s/vault.bin\n", cfg.DataDir)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
}
