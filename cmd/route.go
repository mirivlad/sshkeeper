package cmd

import (
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: tr("Manage server routes (bastions / ProxyJump)", "Управление маршрутами серверов (бастионы / ProxyJump)"),
}

var routeShowCmd = &cobra.Command{
	Use:   "show <alias>",
	Short: tr("Show route for a server", "Показать маршрут сервера"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", args[0]))
		}
		target := server.Host
		if server.User != "" {
			target = server.User + "@" + server.Host
		}
		target = fmt.Sprintf("%s:%d", target, server.Port)
		if len(server.Route.Hops) == 0 {
			fmt.Println(tr("Direct connection (no route)", "Прямое подключение (без маршрута)"))
			return nil
		}
		fmt.Printf(tr("Route: %s\n", "Маршрут: %s\n"), server.Route.DisplaySummary(target))
		fmt.Printf(tr("Mode: %s\n", "Режим: %s\n"), server.Route.RouteMode())
		fmt.Printf(tr("Spec: %s\n", "Описание: %s\n"), model.FormatRouteSpec(server.Route))
		fmt.Println(tr("Hops:", "Узлы:"))
		for index, hop := range server.Route.Hops {
			if hop.Profile() {
				fmt.Printf(tr("  %d. %s (sshkeeper profile #%d)\n", "  %d. %s (профиль sshkeeper #%d)\n"), index+1, hop.Alias, hop.ServerID)
			} else {
				fmt.Printf(tr("  %d. %s (raw OpenSSH target)\n", "  %d. %s (адрес OpenSSH)\n"), index+1, hop.Raw)
			}
		}
		return nil
	},
}

var routeSetCmd = &cobra.Command{
	Use:   "set <alias>",
	Short: tr("Set route for a server", "Задать маршрут сервера"),
	Long: tr(`Set an ordered route. Known aliases are resolved to stable sshkeeper profile IDs.
Use profile:<alias> to require a profile reference and raw:<target> to force a literal OpenSSH target.`, `Задать упорядоченный маршрут. Известные псевдонимы преобразуются в постоянные ID профилей sshkeeper.
Используйте profile:<alias> для ссылки на профиль или raw:<target> для буквального адреса OpenSSH.`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", args[0]))
		}
		mode, _ := cmd.Flags().GetString("mode")
		jumps, _ := cmd.Flags().GetString("jumps")
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "clear" || mode == "direct" {
			server.Route = model.Route{}
			server.ProxyJump = ""
		} else {
			if strings.TrimSpace(jumps) == "" {
				return fmt.Errorf("%s", tr("--jumps is required unless --mode=direct/clear", "Требуется --jumps, кроме режимов --mode=direct/clear"))
			}
			route, err := parseRouteSpec(jumps)
			if err != nil {
				return err
			}
			server.Route = route
			server.ProxyJump = route.ProxyJumpString()
		}
		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("%s: %w", tr("update route", "обновить маршрут"), err)
		}
		if len(server.Route.Hops) == 0 {
			fmt.Println(tr("✓ Route cleared (direct connection)", "✓ Маршрут очищен (прямое подключение)"))
		} else {
			fmt.Printf(tr("✓ Route set: %s\n", "✓ Маршрут задан: %s\n"), model.FormatRouteSpec(server.Route))
		}
		return nil
	},
}

var routeClearCmd = &cobra.Command{
	Use:   "clear <alias>",
	Short: tr("Clear route for a server (set direct)", "Очистить маршрут сервера (прямое подключение)"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := appDB.GetServer(args[0])
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", args[0]))
		}
		server.Route = model.Route{}
		server.ProxyJump = ""
		if err := appDB.UpdateServer(server); err != nil {
			return fmt.Errorf("%s: %w", tr("clear route", "очистить маршрут"), err)
		}
		fmt.Println(tr("✓ Route cleared (direct connection)", "✓ Маршрут очищен (прямое подключение)"))
		return nil
	},
}

func init() {
	routeSetCmd.Flags().String("mode", "via", tr("Route mode: via, chain, direct, or clear", "Режим маршрута: via, chain, direct или clear"))
	routeSetCmd.Flags().String("jumps", "", tr("Comma-separated hops; use profile:<alias> or raw:<target> for explicit type", "Узлы через запятую; profile:<alias> или raw:<target> задают тип"))
	routeCmd.AddCommand(routeShowCmd)
	routeCmd.AddCommand(routeSetCmd)
	routeCmd.AddCommand(routeClearCmd)
}
