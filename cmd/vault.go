package cmd

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var vaultInstance *vault.Vault

func getOrCreateVault() *vault.Vault {
	if vaultInstance == nil {
		vaultInstance = vault.New(config.VaultPath(cfg.DataDir))
	}
	return vaultInstance
}

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: tr("Vault management commands", "Команды управления хранилищем"),
}

var vaultUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: tr("Verify the vault master password", "Проверить мастер-пароль хранилища"),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()

		if v.IsUnlocked() {
			fmt.Println(tr("Vault is already unlocked.", "Хранилище уже разблокировано."))
			return nil
		}

		vaultPath := config.VaultPath(cfg.DataDir)

		// Check if vault exists and has content
		info, err := os.Stat(vaultPath)
		if os.IsNotExist(err) || info.Size() == 0 {
			// New vault - create with master password
			fmt.Print(tr("Create master password: ", "Создайте мастер-пароль: "))
			pw1, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
			}

			if len(pw1) == 0 {
				return fmt.Errorf("%s", tr("password cannot be empty", "пароль не может быть пустым"))
			}

			fmt.Print(tr("Repeat master password: ", "Повторите мастер-пароль: "))
			pw2, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
			}

			if string(pw1) != string(pw2) {
				return fmt.Errorf("%s", tr("passwords do not match", "пароли не совпадают"))
			}

			if err := vault.Create(vaultPath, string(pw1)); err != nil {
				return fmt.Errorf(tr("create vault: %w", "создать хранилище: %w"), err)
			}

			if err := v.Unlock(string(pw1)); err != nil {
				return fmt.Errorf(tr("unlock vault: %w", "разблокировать хранилище: %w"), err)
			}

			fmt.Println(tr("Vault created. Commands will ask for the master password when they need secrets.", "Хранилище создано. Команды запросят мастер-пароль, когда им понадобятся секреты."))
			return nil
		}

		fmt.Print(tr("Master password: ", "Мастер-пароль: "))
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
		}

		if err := v.Unlock(string(pw)); err != nil {
			return fmt.Errorf(tr("unlock vault: %w", "разблокировать хранилище: %w"), err)
		}

		fmt.Println(tr("Master password accepted. Vault unlock is process-local; commands will ask again when they need secrets.", "Мастер-пароль принят. Разблокировка действует только в текущем процессе; команды повторно запросят пароль, когда понадобятся секреты."))
		return nil
	},
}

var vaultLockCmd = &cobra.Command{
	Use:   "lock",
	Short: tr("Lock the vault", "Заблокировать хранилище"),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()
		v.Lock()
		fmt.Println(tr("Vault locked.", "Хранилище заблокировано."))
		return nil
	},
}

var vaultStatusCmd = &cobra.Command{
	Use:   "status",
	Short: tr("Show vault status", "Показать состояние хранилища"),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()
		fmt.Println(formatVaultStatus(v.IsUnlocked(), vault.Exists(config.VaultPath(cfg.DataDir))))
		return nil
	},
}

var vaultChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: tr("Change master password", "Сменить мастер-пароль"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return changeVaultPasswordInteractive()
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: tr("List stored secret metadata", "Показать метаданные сохранённых секретов"),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()
		if err := unlockVaultForCommand(v); err != nil {
			return err
		}
		output, err := formatVaultSecretsList(v)
		if err != nil {
			return err
		}
		fmt.Print(output)
		return nil
	},
}

var vaultDeleteCmd = &cobra.Command{
	Use:   "delete <alias> [type]",
	Short: tr("Delete stored secrets for a server", "Удалить секреты сервера"),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		secretType := ""
		if len(args) == 2 {
			secretType = args[1]
		}

		v := getOrCreateVault()
		if err := unlockVaultForCommand(v); err != nil {
			return err
		}
		if err := deleteVaultSecrets(v, alias, secretType); err != nil {
			return err
		}
		if err := v.Save(); err != nil {
			return fmt.Errorf(tr("save vault: %w", "сохранить хранилище: %w"), err)
		}
		if secretType == "" {
			fmt.Printf(tr("Deleted secrets for %s.\n", "Секреты для %s удалены.\n"), alias)
		} else {
			fmt.Printf(tr("Deleted %s for %s.\n", "Секрет %s для %s удалён.\n"), secretType, alias)
		}
		return nil
	},
}

func unlockVaultForCommand(v *vault.Vault) error {
	if v.IsUnlocked() {
		return nil
	}

	fmt.Print(tr("Master password: ", "Мастер-пароль: "))
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
	}

	if err := v.Unlock(string(pw)); err != nil {
		return fmt.Errorf(tr("unlock vault: %w", "разблокировать хранилище: %w"), err)
	}
	return nil
}

func changeVaultPasswordInteractive() error {
	v := getOrCreateVault()

	if err := unlockVaultForCommand(v); err != nil {
		return err
	}

	fmt.Print(tr("New master password: ", "Новый мастер-пароль: "))
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
	}

	if len(pw1) == 0 {
		return fmt.Errorf("%s", tr("password cannot be empty", "пароль не может быть пустым"))
	}

	fmt.Print(tr("Repeat new master password: ", "Повторите новый мастер-пароль: "))
	pw2, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf(tr("read password: %w", "прочитать пароль: %w"), err)
	}

	if string(pw1) != string(pw2) {
		return fmt.Errorf("%s", tr("passwords do not match", "пароли не совпадают"))
	}

	if err := v.ChangePassword(string(pw1)); err != nil {
		return fmt.Errorf(tr("change password: %w", "сменить пароль: %w"), err)
	}

	fmt.Println(tr("Master password changed.", "Мастер-пароль изменён."))
	return nil
}

func vaultLockedProcessMessage() string {
	return tr("vault is locked in this process; enter the master password when this command prompts for it", "хранилище заблокировано в этом процессе; введите мастер-пароль по запросу команды")
}

func formatVaultStatus(unlocked bool, exists bool) string {
	if !exists {
		return tr("Vault: not found", "Хранилище: не найдено")
	}
	if unlocked {
		return tr("Vault: unlocked in current process", "Хранилище: разблокировано в текущем процессе")
	}
	return tr("Vault: locked (vault commands unlock per command)", "Хранилище: заблокировано (разблокировка отдельно для каждой команды)")
}

func formatVaultSecretsList(v *vault.Vault) (string, error) {
	metas, err := v.ListSecrets()
	if err != nil {
		return "", err
	}
	if len(metas) == 0 {
		return tr("No secrets stored.\n", "Секретов нет.\n"), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-24s %-18s\n", tr("ALIAS", "ПСЕВДОНИМ"), tr("TYPE", "ТИП"))
	for _, meta := range metas {
		alias := meta.Alias
		if alias == "" && meta.ServerID > 0 {
			alias = fmt.Sprintf("#%d", meta.ServerID)
			if appDB != nil {
				if server, err := appDB.GetServerByID(meta.ServerID); err == nil && server != nil {
					alias = server.Alias
				}
			}
		}
		fmt.Fprintf(&b, "%-24s %-18s\n", alias, meta.Type)
	}
	return b.String(), nil
}

func init() {
	vaultCmd.AddCommand(vaultUnlockCmd)
	vaultCmd.AddCommand(vaultLockCmd)
	vaultCmd.AddCommand(vaultStatusCmd)
	vaultCmd.AddCommand(vaultChangePasswordCmd)
	vaultCmd.AddCommand(vaultListCmd)
	vaultCmd.AddCommand(vaultDeleteCmd)
}
