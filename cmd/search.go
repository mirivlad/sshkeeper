package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search servers by alias, host, name, group, notes, tags, route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		servers, err := appDB.SearchServers(query)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		if len(servers) == 0 {
			fmt.Println("No servers found.")
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

			fmt.Printf("[%s] %-20s %-30s  route: %s", statusChar, s.Alias, target, routeStr)

			if len(s.Tags) > 0 {
				fmt.Printf("  tags: %s", strings.Join(s.Tags, ", "))
			}
			if s.Notes != "" {
				fmt.Printf("  notes: %s", s.Notes)
			}
			fmt.Println()
		}

		return nil
	},
}
