package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/model"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("No servers. Use 'sshkeeper add' to add one.")
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
		fmt.Println(headerStyle.Render(fmt.Sprintf("%-20s %-25s %-8s %-12s %s", "ALIAS", "TARGET", "AUTH", "STATUS", "LAST TEST")))
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
