package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/db"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sshkeeper",
	Long:  "Create config, database, and vault directories.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		dirs := []string{cfg.ConfigDir, cfg.DataDir}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("create dir %s: %w", dir, err)
			}
		}

		// Open database (triggers migrations)
		database, err := db.Open(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		// Create empty vault if not exists
		vaultPath := config.VaultPath(cfg.DataDir)
		if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
			f, err := os.OpenFile(vaultPath, os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("create vault: %w", err)
			}
			f.Close()
		}

		fmt.Printf("Created config: %s/config.toml\n", cfg.ConfigDir)
		fmt.Printf("Created database: %s/sshkeeper.db\n", cfg.DataDir)
		fmt.Printf("Created vault: %s/vault.bin\n", cfg.DataDir)
		fmt.Println()
		fmt.Println("Next step: run 'sshkeeper vault unlock' to set master password.")
		return nil
	},
}
