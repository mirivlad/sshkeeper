package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: tr("List all servers", "Показать все серверы"),
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("list servers", "получить список серверов"), err)
		}

		if len(servers) == 0 {
			fmt.Println(tr("No servers. Use 'sshkeeper add' to add one.", "Серверов нет. Добавьте сервер командой 'sshkeeper add'."))
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
		fmt.Println(headerStyle.Render(fmt.Sprintf("%-20s %-25s %-8s %-12s %s", tr("ALIAS", "ПСЕВДОНИМ"), tr("TARGET", "АДРЕС"), tr("AUTH", "ВХОД"), tr("STATUS", "СТАТУС"), tr("LAST TEST", "ПОСЛ. ПРОВЕРКА"))))
		fmt.Println("─────────────────────────────────────────────────────────────────────────")

		for _, s := range servers {
			statusChar := "?"
			if s.LastTestStatus == model.TestOK {
				statusChar = "✓"
			} else if s.LastTestStatus == model.TestFailed {
				statusChar = "!"
			}

			target := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
			lastTest := "never"
			if s.LastTestAt != nil {
				lastTest = s.LastTestAt.Format("2006-01-02 15:04")
			}

			fmt.Printf("%-20s %-25s %-8s [%s]       %s\n", s.Alias, target, s.AuthMethod, statusChar, lastTest)
		}

		return nil
	},
}
