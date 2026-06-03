package cmd

import (
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

// --- Route commands ---

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage server routes (ProxyJump)",
}

var routeShowCmd = &cobra.Command{
	Use:   "show <alias>",
	Short: "Show route for a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}
		target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
		if len(server.Route.Hops) > 0 {
			fmt.Printf("Route: %s\n", server.Route.DisplaySummary(target))
			fmt.Printf("Mode: %s\n", server.Route.RouteMode())
			fmt.Printf("ProxyJump: %s\n", server.Route.ProxyJumpString())
			if server.Route.HasProfileLinks() {
				fmt.Println("Hops:")
				for _, h := range server.Route.Hops {
					if h.IsProfile {
						fmt.Printf("  - %s (profile)\n", h.Alias)
					} else {
						fmt.Printf("  - %s (raw)\n", h.Raw)
					}
				}
			}
		} else if server.ProxyJump != "" {
			fmt.Printf("ProxyJump: %s\n", server.ProxyJump)
		} else {
			fmt.Println("Direct connection (no route)")
		}
		return nil
	},
}

var routeSetCmd = &cobra.Command{
	Use:   "set <alias>",
	Short: "Set route for a server",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		mode, _ := cmd.Flags().GetString("mode")
		jumps, _ := cmd.Flags().GetString("jumps")

		if mode == "clear" || jumps == "" {
			server.Route = model.Route{}
			server.ProxyJump = ""
		} else {
			parts := strings.Split(jumps, ",")
			hops := make([]model.RouteHop, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if strings.Contains(p, "@") || strings.Contains(p, ":") {
					hops = append(hops, model.RouteHop{Raw: p, IsProfile: false})
				} else {
					hops = append(hops, model.RouteHop{Alias: p, IsProfile: true})
				}
			}
			server.Route = model.Route{Hops: hops}
			server.ProxyJump = server.Route.ProxyJumpString()
		}

		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("update route: %w", err)
		}

		target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
		if len(server.Route.Hops) > 0 {
			fmt.Printf("✓ Route set: %s\n", server.Route.DisplaySummary(target))
		} else {
			fmt.Println("✓ Route cleared (direct connection)")
		}
		return nil
	},
}

var routeClearCmd = &cobra.Command{
	Use:   "clear <alias>",
	Short: "Clear route for a server (set direct)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
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
	routeSetCmd.Flags().String("mode", "via", "Route mode: direct, via, chain, or clear")
	routeSetCmd.Flags().String("jumps", "", "Comma-separated jump hosts (aliases or raw addresses)")

	routeCmd.AddCommand(routeShowCmd)
	routeCmd.AddCommand(routeSetCmd)
	routeCmd.AddCommand(routeClearCmd)
}
