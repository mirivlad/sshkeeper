package cmd

import (
	"fmt"
	"strconv"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	tunnelpkg "github.com/mirivlad/sshkeeper/internal/tunnel"
	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel <alias>",
	Short: tr("Start SSH session with port forwards", "Запустить SSH-сеанс с перенаправлением портов"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}

		forwardsOnly, _ := cmd.Flags().GetBool("forward-only")
		background, _ := cmd.Flags().GetBool("background")

		// Load forwards
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("load forwards", "загрузить перенаправления"), err)
		}

		if background {
			if err := validateBackgroundTunnel(server, forwards); err != nil {
				return err
			}
			state, err := tunnelpkg.StartResolved(cfg, server, forwards, true, dbProfileResolver)
			if err != nil {
				return err
			}
			fmt.Printf(tr("✓ Tunnel started [%d] PID %d → %s\n", "✓ Туннель запущен [%d] PID %d → %s\n"), state.ID, state.PID, server.Alias)
			return nil
		}

		active := enabledForwardCount(forwards)
		if active == 0 && forwardsOnly {
			return fmt.Errorf("%s", trf("no enabled forwards configured for %s", "для %s нет включённых перенаправлений", alias))
		}

		if active > 0 {
			fmt.Printf(tr("Starting tunnel to %s with %d enabled forward(s)...\n", "Запуск туннеля к %s с %d включёнными перенаправлениями...\n"), alias, active)
		} else {
			fmt.Printf(tr("Starting session to %s...\n", "Запуск сеанса с %s...\n"), alias)
		}

		if forwardsOnly {
			fmt.Print(tr("Tunnel mode (ssh -N). Press Ctrl+C to exit.\n", "Режим туннеля (ssh -N). Нажмите Ctrl+C для выхода.\n"))
		}

		return ssh.ConnectWithForwardsResolved(cfg, server, forwards, forwardsOnly, dbProfileResolver, serverVaultFunc(server))
	},
}

var tunnelListCmd = &cobra.Command{
	Use:   "list",
	Short: tr("List tracked background tunnels", "Показать фоновые туннели"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		states := tunnelpkg.List()
		if len(states) == 0 {
			fmt.Println(tr("No tracked tunnels.", "Фоновых туннелей нет."))
			return nil
		}
		fmt.Printf("%-22s %-8s %-10s %s\n", "ID", "PID", tr("STATUS", "СТАТУС"), tr("SERVER", "СЕРВЕР"))
		for _, state := range states {
			status := "stopped"
			if tunnelpkg.IsRunning(state.ID) {
				status = "running"
			}
			fmt.Printf("%-22d %-8d %-10s %s\n", state.ID, state.PID, status, state.ServerAlias)
		}
		return nil
	},
}

var tunnelStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: tr("Stop a tracked background tunnel", "Остановить фоновый туннель"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("%s", trf("invalid tunnel ID: %s", "недопустимый ID туннеля: %s", args[0]))
		}
		if err := tunnelpkg.Stop(id); err != nil {
			return err
		}
		fmt.Printf(tr("✓ Tunnel %d stopped\n", "✓ Туннель %d остановлен\n"), id)
		return nil
	},
}

var tunnelStopAllCmd = &cobra.Command{
	Use:   "stop-all",
	Short: tr("Stop all tracked background tunnels", "Остановить все фоновые туннели"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tunnelpkg.StopAll(); err != nil {
			return err
		}
		fmt.Println(tr("✓ All tracked tunnels stopped", "✓ Все фоновые туннели остановлены"))
		return nil
	},
}

func enabledForwardCount(forwards []*model.Forward) int {
	count := 0
	for _, forward := range forwards {
		if forward != nil && forward.Enabled {
			count++
		}
	}
	return count
}

func validateBackgroundTunnel(server *model.Server, forwards []*model.Forward) error {
	if server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase {
		return fmt.Errorf("%s", trf("background tunnels support only key or agent auth; use foreground tunnel for %s auth", "фоновые туннели поддерживают только вход по ключу или через агент; для %s используйте обычный туннель", server.AuthMethod))
	}
	if enabledForwardCount(forwards) == 0 {
		return fmt.Errorf("%s", trf("no enabled forwards configured for %s", "для %s нет включённых перенаправлений", server.Alias))
	}
	return nil
}

func init() {
	tunnelCmd.Flags().Bool("forward-only", false, tr("Start tunnel only (ssh -N)", "Запустить только туннель (ssh -N)"))
	tunnelCmd.Flags().Bool("background", false, tr("Start tunnel in background (ssh -N)", "Запустить туннель в фоне (ssh -N)"))
	tunnelCmd.AddCommand(tunnelListCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelStopAllCmd)
}
