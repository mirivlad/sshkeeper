package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search servers",
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
			fmt.Printf("[%s] %-20s %s\n", statusChar, s.Alias, target)
		}

		return nil
	},
}
