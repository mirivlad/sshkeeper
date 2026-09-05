package cmd

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
		original := cloneServer(server)

		if cmd.Flags().Changed("host") {
			server.Host = parsedHost
		}
		if cmd.Flags().Changed("port") {
			if parsedPort < 1 || parsedPort > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
			server.Port = parsedPort
		}
		if cmd.Flags().Changed("user") {
			server.User = parsedUser
		}
		authChanged := cmd.Flags().Changed("auth")
		if authChanged {
			server.AuthMethod = model.AuthMethod(parsedAuth)
		}
		if cmd.Flags().Changed("identity-file") {
			server.IdentityFile = parsedIdentity
		}
		if cmd.Flags().Changed("group") {
			server.GroupName = parsedGroup
		}
		if cmd.Flags().Changed("display-name") {
			server.DisplayName = parsedDisplayName
		}
		if cmd.Flags().Changed("notes") {
			server.Notes = parsedNotes
		}
		if cmd.Flags().Changed("startup-command") {
			server.StartupCommand = parsedStartup
		}
		routeChanged := cmd.Flags().Changed("route") || cmd.Flags().Changed("proxy-jump")
		if routeChanged {
			if cmd.Flags().Changed("route") && cmd.Flags().Changed("proxy-jump") {
				return fmt.Errorf("use either --route or --proxy-jump, not both")
			}
			spec := parsedRoute
			if cmd.Flags().Changed("proxy-jump") {
				spec = parsedProxyJump
			}
			route, err := parseRouteSpec(spec)
			if err != nil {
				return fmt.Errorf("route: %w", err)
			}
			server.Route = route
			server.ProxyJump = route.ProxyJumpString()
		}
		tagsChanged := cmd.Flags().Changed("tags")
		if tagsChanged {
			if strings.TrimSpace(parsedTags) == "" {
				server.Tags = nil
			} else {
				server.Tags = strings.Split(parsedTags, ",")
			}
		}
		if err := model.ValidateServerBasics(server); err != nil {
			return err
		}

		var secret []byte
		v := getOrCreateVault()
		if authChanged {
			if err := unlockVaultForCommand(v); err != nil {
				return err
			}
			switch server.AuthMethod {
			case model.AuthPassword, model.AuthKeyPassphrase:
				label := "password"
				if server.AuthMethod == model.AuthKeyPassphrase {
					label = "key passphrase"
				}
				fmt.Printf("Enter new %s (stored in vault, input hidden): ", label)
				secret, err = term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("read %s: %w", label, err)
				}
				if len(secret) == 0 {
					return fmt.Errorf("%s cannot be empty", label)
				}
				defer func() {
					for i := range secret {
						secret[i] = 0
					}
				}()
			}
		}

		if err := appDB.UpdateServerByAlias(alias, server); err != nil {
			return fmt.Errorf("update server: %w", err)
		}
		rollback := func() {
			_ = appDB.UpdateServerByAlias(server.Alias, original)
			_ = appDB.SetServerTags(original.ID, original.Tags)
		}
		if tagsChanged {
			if err := appDB.SetServerTags(server.ID, server.Tags); err != nil {
				rollback()
				return fmt.Errorf("set tags: %w", err)
			}
		}
		if authChanged {
			if err := syncServerSecrets(v, alias, server, string(secret)); err != nil {
				rollback()
				return fmt.Errorf("sync vault secrets: %w", err)
			}
			if err := v.Save(); err != nil {
				rollback()
				return fmt.Errorf("save vault: %w", err)
			}
		}
		fmt.Println("Saved.")
		return nil
	},
}

func cloneServer(server *model.Server) *model.Server {
	copyServer := *server
	copyServer.Route.Hops = append([]model.RouteHop(nil), server.Route.Hops...)
	copyServer.Tags = append([]string(nil), server.Tags...)
	return &copyServer
}

var (
	parsedHost        string
	parsedPort        int
	parsedUser        string
	parsedAuth        string
	parsedIdentity    string
	parsedRoute       string
	parsedProxyJump   string
	parsedGroup       string
	parsedDisplayName string
	parsedNotes       string
	parsedStartup     string
	parsedTags        string
)

func init() {
	editCmd.Flags().StringVar(&parsedHost, "host", "", "Server hostname or IP")
	editCmd.Flags().IntVar(&parsedPort, "port", 0, "SSH port")
	editCmd.Flags().StringVar(&parsedUser, "user", "", "SSH username; empty lets OpenSSH choose")
	editCmd.Flags().StringVar(&parsedAuth, "auth", "", "Auth method")
	editCmd.Flags().StringVar(&parsedIdentity, "identity-file", "", "Path to SSH private key; empty clears it")
	editCmd.Flags().StringVar(&parsedRoute, "route", "", "Route hops: profile:<alias>, raw:<target>, comma-separated; empty means direct")
	editCmd.Flags().StringVar(&parsedProxyJump, "proxy-jump", "", "Compatibility alias for --route")
	editCmd.Flags().StringVar(&parsedGroup, "group", "", "Server group; empty clears it")
	editCmd.Flags().StringVar(&parsedDisplayName, "display-name", "", "Display name; empty clears it")
	editCmd.Flags().StringVar(&parsedNotes, "notes", "", "Notes; empty clears them")
	editCmd.Flags().StringVar(&parsedStartup, "startup-command", "", "Startup command; empty clears it")
	editCmd.Flags().StringVar(&parsedTags, "tags", "", "Comma-separated tags; empty clears all")
}
