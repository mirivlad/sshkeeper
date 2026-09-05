package cmd

import (
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import servers from ~/.ssh/config",
	RunE: func(cmd *cobra.Command, args []string) error {
		imported, err := importServersFromSSHConfig(func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		})
		if err != nil {
			return err
		}
		fmt.Printf("\nImported %d servers.\n", imported)
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export servers to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := appDB.ListServers()
		if err != nil {
			return fmt.Errorf("list servers: %w", err)
		}

		fmt.Print(formatServersExport(servers))
		return nil
	},
}

func importServersFromSSHConfig(report func(format string, args ...interface{})) (int, error) {
	servers, err := ssh.ImportFromSSHConfig()
	if err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}
	if len(servers) == 0 {
		if report != nil {
			report("No servers found in ~/.ssh/config")
		}
		return 0, nil
	}

	// First pass creates every profile without routes. That makes ProxyJump
	// aliases resolvable to stable sshkeeper IDs in the second pass, regardless
	// of declaration order in ~/.ssh/config.
	type pendingRoute struct {
		server *model.Server
		spec   string
	}
	pending := make([]pendingRoute, 0, len(servers))
	imported := 0
	for _, server := range servers {
		if existing, _ := appDB.GetServer(server.Alias); existing != nil {
			if report != nil {
				report("  skip (exists): %s", server.Alias)
			}
			continue
		}
		spec := strings.TrimSpace(server.ProxyJump)
		server.ProxyJump = ""
		server.Route = model.Route{}
		if err := appDB.CreateServer(server); err != nil {
			if report != nil {
				report("  error: %s: %v", server.Alias, err)
			}
			continue
		}
		pending = append(pending, pendingRoute{server: server, spec: spec})
		imported++
	}

	for _, item := range pending {
		if item.spec != "" {
			route, err := parseRouteSpec(item.spec)
			if err != nil {
				if report != nil {
					report("  warning: %s imported direct; route %q could not be parsed: %v", item.server.Alias, item.spec, err)
				}
				continue
			}
			item.server.Route = route
			item.server.ProxyJump = route.ProxyJumpString()
			if err := appDB.UpdateServer(item.server); err != nil {
				item.server.Route = model.Route{}
				item.server.ProxyJump = ""
				if report != nil {
					report("  warning: %s imported direct; route could not be saved: %v", item.server.Alias, err)
				}
				continue
			}
		}
		if report != nil {
			routeSuffix := ""
			if len(item.server.Route.Hops) > 0 {
				routeSuffix = " via " + model.FormatRouteSpec(item.server.Route)
			}
			report("  imported: %s (%s@%s:%d)%s", item.server.Alias, item.server.User, item.server.Host, item.server.Port, routeSuffix)
		}
	}
	return imported, nil
}

func formatServersExport(servers []*model.Server) string {
	var b strings.Builder
	for _, s := range servers {
		fmt.Fprintf(&b, "%s\t%s@%s:%d\t%s\n", s.Alias, s.User, s.Host, s.Port, s.AuthMethod)
	}
	return b.String()
}

var runCmd = &cobra.Command{
	Use:   "run <alias> <command>",
	Short: "Run a command on a server",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		command := strings.Join(args[1:], " ")

		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		return runCommandOnServer(server, command)
	},
}

func runCommandOnServer(server *model.Server, command string) error {
	return ssh.RunCommandResolved(cfg, server, dbProfileResolver, serverVaultFunc(server), command)
}
