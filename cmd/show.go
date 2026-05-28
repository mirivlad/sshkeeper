package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <alias>",
	Short: "Show server details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		fmt.Printf("Alias:        %s\n", server.Alias)
		fmt.Printf("Display Name: %s\n", server.DisplayName)
		fmt.Printf("Host:         %s\n", server.Host)
		fmt.Printf("Port:         %d\n", server.Port)
		fmt.Printf("User:         %s\n", server.User)
		fmt.Printf("Auth Method:  %s\n", server.AuthMethod)
		if server.IdentityFile != "" {
			fmt.Printf("Identity:     %s\n", server.IdentityFile)
		}
		if server.ProxyJump != "" {
			fmt.Printf("ProxyJump:    %s\n", server.ProxyJump)
		}
		if server.GroupName != "" {
			fmt.Printf("Group:        %s\n", server.GroupName)
		}
		if len(server.Tags) > 0 {
			fmt.Printf("Tags:         %s\n", strings.Join(server.Tags, ", "))
		}
		if server.StartupCommand != "" {
			fmt.Printf("Startup Cmd:  %s\n", server.StartupCommand)
		}
		if server.Notes != "" {
			fmt.Printf("Notes:        %s\n", server.Notes)
		}
		fmt.Printf("Test Status:  %s\n", server.LastTestStatus)
		if server.LastTestAt != nil {
			fmt.Printf("Last Test:    %s\n", server.LastTestAt.Format("2006-01-02 15:04:05"))
		}
		if server.LastTestError != "" {
			fmt.Printf("Last Error:   %s\n", server.LastTestError)
		}
		if server.LastConnectedAt != nil {
			fmt.Printf("Last Connect: %s\n", server.LastConnectedAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}
