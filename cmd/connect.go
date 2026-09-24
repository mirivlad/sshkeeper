package cmd

import (
	"fmt"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:     "connect <alias>",
	Aliases: []string{"c"},
	Short:   tr("Connect to a server via SSH", "Подключиться к серверу по SSH"),
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf(tr("server not found: %s", "сервер не найден: %s"), alias)
		}
		if err := ssh.ConnectResolved(cfg, server, dbProfileResolver, serverVaultFunc(server)); err != nil {
			return err
		}
		appDB.UpdateLastConnected(alias)
		return nil
	},
}

var testCmd = &cobra.Command{
	Use:   "test <alias>",
	Short: tr("Test SSH connection", "Проверить SSH-подключение"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf(tr("server not found: %s", "сервер не найден: %s"), alias)
		}
		ok, testErr := ssh.TestResolved(cfg, server, dbProfileResolver, serverVaultFunc(server))
		if ok {
			fmt.Println(tr("Connection OK.", "Подключение успешно."))
			appDB.UpdateTestResult(alias, model.TestOK, "")
		} else {
			fmt.Printf(tr("Connection failed:\n%s\n", "Ошибка подключения:\n%s\n"), testErr)
			appDB.UpdateTestResult(alias, model.TestFailed, testErr)
		}
		return nil
	},
}
