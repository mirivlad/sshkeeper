package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <alias>",
	Short: tr("Show server details", "Показать сведения о сервере"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}

		fmt.Printf(tr("Alias:        %s\n", "Псевдоним:    %s\n"), server.Alias)
		fmt.Printf(tr("Display Name: %s\n", "Имя:          %s\n"), server.DisplayName)
		fmt.Printf(tr("Host:         %s\n", "Хост:         %s\n"), server.Host)
		fmt.Printf(tr("Port:         %d\n", "Порт:         %d\n"), server.Port)
		fmt.Printf(tr("User:         %s\n", "Пользователь: %s\n"), server.User)
		fmt.Printf(tr("Auth Method:  %s\n", "Способ входа: %s\n"), server.AuthMethod)
		if server.IdentityFile != "" {
			fmt.Printf(tr("Identity:     %s\n", "Ключ:         %s\n"), server.IdentityFile)
		}
		if server.ProxyJump != "" {
			fmt.Printf(tr("ProxyJump:    %s\n", "ProxyJump:    %s\n"), server.ProxyJump)
		}
		if server.GroupName != "" {
			fmt.Printf(tr("Group:        %s\n", "Группа:       %s\n"), server.GroupName)
		}
		if len(server.Tags) > 0 {
			fmt.Printf(tr("Tags:         %s\n", "Теги:         %s\n"), strings.Join(server.Tags, ", "))
		}
		if server.StartupCommand != "" {
			fmt.Printf(tr("Startup Cmd:  %s\n", "Команда входа: %s\n"), server.StartupCommand)
		}
		if server.Notes != "" {
			fmt.Printf(tr("Notes:        %s\n", "Заметки:      %s\n"), server.Notes)
		}
		fmt.Printf(tr("Test Status:  %s\n", "Статус теста: %s\n"), server.LastTestStatus)
		if server.LastTestAt != nil {
			fmt.Printf(tr("Last Test:    %s\n", "Послед. тест: %s\n"), server.LastTestAt.Format("2006-01-02 15:04:05"))
		}
		if server.LastTestError != "" {
			fmt.Printf(tr("Last Error:   %s\n", "Послед. ошибка: %s\n"), server.LastTestError)
		}
		if server.LastConnectedAt != nil {
			fmt.Printf(tr("Last Connect: %s\n", "Послед. вход: %s\n"), server.LastConnectedAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}
