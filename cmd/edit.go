package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/model"
)

var editCmd = &cobra.Command{
	Use:   "edit <alias>",
	Short: "Edit a server profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		if parsedHost != "" {
			server.Host = parsedHost
		}
		if parsedPort != 0 {
			server.Port = parsedPort
		}
		if parsedUser != "" {
			server.User = parsedUser
		}
		if parsedAuth != "" {
			server.AuthMethod = model.AuthMethod(parsedAuth)
		}
		if parsedIdentity != "" {
			server.IdentityFile = parsedIdentity
		}
		if parsedProxyJump != "" {
			server.ProxyJump = parsedProxyJump
		}
		if parsedGroup != "" {
			server.GroupName = parsedGroup
		}
		if parsedDisplayName != "" {
			server.DisplayName = parsedDisplayName
		}
		if parsedNotes != "" {
			server.Notes = parsedNotes
		}

		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("update server: %w", err)
		}

		fmt.Println("Saved.")
		return nil
	},
}

var (
	parsedHost      string
	parsedPort      int
	parsedUser      string
	parsedAuth      string
	parsedIdentity  string
	parsedProxyJump string
	parsedGroup     string
	parsedDisplayName string
	parsedNotes     string
)

func init() {
	editCmd.Flags().StringVar(&parsedHost, "host", "", "Server hostname or IP")
	editCmd.Flags().IntVar(&parsedPort, "port", 0, "SSH port")
	editCmd.Flags().StringVar(&parsedUser, "user", "", "SSH username")
	editCmd.Flags().StringVar(&parsedAuth, "auth", "", "Auth method")
	editCmd.Flags().StringVar(&parsedIdentity, "identity-file", "", "Path to SSH private key")
	editCmd.Flags().StringVar(&parsedProxyJump, "proxy-jump", "", "ProxyJump host")
	editCmd.Flags().StringVar(&parsedGroup, "group", "", "Server group")
	editCmd.Flags().StringVar(&parsedDisplayName, "display-name", "", "Display name")
	editCmd.Flags().StringVar(&parsedNotes, "notes", "", "Notes")
}
