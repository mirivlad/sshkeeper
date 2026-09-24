package cmd

import (
	"fmt"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var sessionConnectCmd = &cobra.Command{
	Use:    "__session-connect <alias>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}
		if server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase {
			if err := unlockVaultForSession(); err != nil {
				return err
			}
		}
		if err := ssh.ConnectResolved(cfg, server, dbProfileResolver, serverVaultFunc(server)); err != nil {
			return err
		}
		_ = appDB.UpdateLastConnected(alias)
		return nil
	},
}

func unlockVaultForSession() error {
	v := getOrCreateVault()
	if v.IsUnlocked() {
		return nil
	}
	for attempts := 0; attempts < 3; attempts++ {
		fmt.Print(tr("Master password: ", "Мастер-пароль: "))
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("read vault password", "прочитать пароль хранилища"), err)
		}
		if err := v.Unlock(string(password)); err == nil {
			return nil
		}
		remaining := 2 - attempts
		if remaining > 0 {
			fmt.Println(trf("Invalid password. %d attempts remaining.", "Неверный пароль. Осталось попыток: %d.", remaining))
		}
	}
	return fmt.Errorf("%s", tr("too many failed vault unlock attempts", "слишком много неудачных попыток разблокировать хранилище"))
}
