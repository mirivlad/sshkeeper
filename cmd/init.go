package cmd

import (
	"fmt"
	"os"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: tr("Initialize sshkeeper", "Инициализировать sshkeeper"),
	Long:  tr("Create config, database, and vault directories.", "Создать каталоги конфигурации, базы данных и хранилища."),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("load config", "загрузить конфигурацию"), err)
		}

		dirs := []string{cfg.ConfigDir, cfg.DataDir}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("%s: %w", trf("create dir %s", "создать каталог %s", dir), err)
			}
		}

		// Open database (triggers migrations)
		database, err := db.Open(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("open database", "открыть базу данных"), err)
		}
		defer database.Close()

		// Create empty vault if not exists
		vaultPath := config.VaultPath(cfg.DataDir)
		if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
			f, err := os.OpenFile(vaultPath, os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("%s: %w", tr("create vault", "создать хранилище"), err)
			}
			f.Close()
		}

		fmt.Printf(tr("Created config: %s/config.toml\n", "Создан конфиг: %s/config.toml\n"), cfg.ConfigDir)
		fmt.Printf(tr("Created database: %s/sshkeeper.db\n", "Создана база данных: %s/sshkeeper.db\n"), cfg.DataDir)
		fmt.Printf(tr("Created vault: %s/vault.bin\n", "Создано хранилище: %s/vault.bin\n"), cfg.DataDir)
		fmt.Println()
		fmt.Println(tr("Next step: run 'sshkeeper' or any command that needs secrets to create the vault master password.", "Далее запустите 'sshkeeper' или любую команду, которой нужны секреты, чтобы создать мастер-пароль хранилища."))
		return nil
	},
}
