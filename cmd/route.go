package cmd

import (
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage server routes (bastions / ProxyJump)",
}

var routeShowCmd = &cobra.Command{
	Use:   "show <alias>",
	Short: "Show route for a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("server not found: %s", args[0])
		}
		target := server.Host
		if server.User != "" {
			target = server.User + "@" + server.Host
		}
		target = fmt.Sprintf("%s:%d", target, server.Port)
		if len(server.Route.Hops) == 0 {
			fmt.Println("Direct connection (no route)")
			return nil
		}
		fmt.Printf("Route: %s\n", server.Route.DisplaySummary(target))
		fmt.Printf("Mode: %s\n", server.Route.RouteMode())
		fmt.Printf("Spec: %s\n", model.FormatRouteSpec(server.Route))
		fmt.Println("Hops:")
		for index, hop := range server.Route.Hops {
			if hop.Profile() {
				fmt.Printf("  %d. %s (sshkeeper profile #%d)\n", index+1, hop.Alias, hop.ServerID)
			} else {
				fmt.Printf("  %d. %s (raw OpenSSH target)\n", index+1, hop.Raw)
			}
		}
		return nil
	},
}

var routeSetCmd = &cobra.Command{
	Use:   "set <alias>",
	Short: "Set route for a server",
	Long: `Set an ordered route. Known aliases are resolved to stable sshkeeper profile IDs.
Use profile:<alias> to require a profile reference and raw:<target> to force a literal OpenSSH target.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("server not found: %s", args[0])
		}
		mode, _ := cmd.Flags().GetString("mode")
		jumps, _ := cmd.Flags().GetString("jumps")
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "clear" || mode == "direct" {
			server.Route = model.Route{}
			server.ProxyJump = ""
		} else {
			if strings.TrimSpace(jumps) == "" {
				return fmt.Errorf("--jumps is required unless --mode=direct/clear")
			}
			route, err := parseRouteSpec(jumps)
			if err != nil {
				return err
			}
			server.Route = route
			server.ProxyJump = route.ProxyJumpString()
		}
		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("update route: %w", err)
		}
		if len(server.Route.Hops) == 0 {
			fmt.Println("✓ Route cleared (direct connection)")
		} else {
			fmt.Printf("✓ Route set: %s\n", model.FormatRouteSpec(server.Route))
		}
		return nil
	},
}

var routeClearCmd = &cobra.Command{
	Use:   "clear <alias>",
	Short: "Clear route for a server (set direct)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("server not found: %s", args[0])
		}
		server.Route = model.Route{}
		server.ProxyJump = ""
		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("clear route: %w", err)
		}
		fmt.Println("✓ Route cleared (direct connection)")
		return nil
	},
}

func init() {
	routeSetCmd.Flags().String("mode", "via", "Route mode: via, chain, direct, or clear")
	routeSetCmd.Flags().String("jumps", "", "Comma-separated hops; use profile:<alias> or raw:<target> for explicit type")
	routeCmd.AddCommand(routeShowCmd)
	routeCmd.AddCommand(routeSetCmd)
	routeCmd.AddCommand(routeClearCmd)
}
