package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: tr("Search servers by alias, host, name, group, notes, tags, route", "Поиск серверов по псевдониму, хосту, имени, группе, заметкам, тегам и маршруту"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		servers, err := appDB.SearchServers(query)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("search", "поиск"), err)
		}

		if len(servers) == 0 {
			fmt.Println(tr("No servers found.", "Серверы не найдены."))
			return nil
		}

		for _, s := range servers {
			statusChar := "?"
			if s.LastTestStatus == "ok" {
				statusChar = "✓"
			} else if s.LastTestStatus == "failed" {
				statusChar = "!"
			}
			target := fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)

			// Show route summary if available
			routeStr := "direct"
			if len(s.Route.Hops) > 0 {
				routeStr = s.Route.DisplaySummary(target)
			} else if s.ProxyJump != "" {
				routeStr = "via " + s.ProxyJump
			}

			fmt.Printf(tr("[%s] %-20s %-30s  route: %s", "[%s] %-20s %-30s  маршрут: %s"), statusChar, s.Alias, target, routeStr)

			if len(s.Tags) > 0 {
				fmt.Printf(tr("  tags: %s", "  теги: %s"), strings.Join(s.Tags, ", "))
			}
			if s.Notes != "" {
				fmt.Printf(tr("  notes: %s", "  заметки: %s"), s.Notes)
			}
			fmt.Println()
		}

		return nil
	},
}
