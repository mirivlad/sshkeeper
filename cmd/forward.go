package cmd

import (
	"fmt"
	"strconv"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

// --- Forward commands ---

var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: tr("Manage port forwards", "Управление перенаправлением портов"),
}

var forwardListCmd = &cobra.Command{
	Use:   "list <alias>",
	Short: tr("List port forwards for a server", "Показать перенаправления портов сервера"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("list forwards", "получить список перенаправлений"), err)
		}
		if len(forwards) == 0 {
			fmt.Println(tr("No port forwards configured.", "Перенаправления портов не настроены."))
			return nil
		}
		fmt.Printf(tr("Port forwards for %s:\n", "Перенаправления портов для %s:\n"), alias)
		for _, f := range forwards {
			switch f.Type {
			case model.ForwardLocal:
				fmt.Printf("  [%d] -L %s:%d:%s:%d\n", f.ID, f.LocalAddr, f.LocalPort, f.RemoteAddr, f.RemotePort)
			case model.ForwardRemote:
				fmt.Printf("  [%d] -R %s:%d:%s:%d\n", f.ID, f.RemoteAddr, f.RemotePort, f.LocalAddr, f.LocalPort)
			case model.ForwardDynamic:
				fmt.Printf("  [%d] -D %s:%d\n", f.ID, f.LocalAddr, f.LocalPort)
			}
		}
		return nil
	},
}

var forwardAddCmd = &cobra.Command{
	Use:   "add <alias>",
	Short: tr("Add a port forward", "Добавить перенаправление порта"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}

		fwdType, _ := cmd.Flags().GetString("type")
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		localAddr, _ := cmd.Flags().GetString("local-addr")
		localPort, _ := cmd.Flags().GetInt("local-port")
		remoteAddr, _ := cmd.Flags().GetString("remote-addr")
		remotePort, _ := cmd.Flags().GetInt("remote-port")

		// Validate type
		if fwdType != "local" && fwdType != "remote" && fwdType != "dynamic" {
			return fmt.Errorf("%s", trf("invalid forward type %q: must be local, remote, or dynamic", "недопустимый тип перенаправления %q: используйте local, remote или dynamic", fwdType))
		}

		// Validate ports
		if localPort < 1 || localPort > 65535 {
			return fmt.Errorf("%s", trf("invalid local port %d: must be 1-65535", "недопустимый локальный порт %d: требуется 1–65535", localPort))
		}

		// Validate fields based on type
		switch fwdType {
		case "local":
			if localAddr == "" || localAddr == "0.0.0.0" {
				localAddr = "0.0.0.0"
			}
			if remoteAddr == "" {
				return fmt.Errorf("%s", tr("remote-addr is required for local forward", "для локального перенаправления требуется remote-addr"))
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("%s", trf("invalid remote port %d: must be 1-65535", "недопустимый удалённый порт %d: требуется 1–65535", remotePort))
			}
		case "remote":
			if remoteAddr == "" {
				return fmt.Errorf("%s", tr("remote-addr is required for remote forward", "для удалённого перенаправления требуется remote-addr"))
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("%s", trf("invalid remote port %d: must be 1-65535", "недопустимый удалённый порт %d: требуется 1–65535", remotePort))
			}
			if localAddr == "" {
				localAddr = "0.0.0.0"
			}
		case "dynamic":
			if localAddr == "" || localAddr == "0.0.0.0" {
				localAddr = "0.0.0.0"
			}
			// dynamic doesn't use target fields — clear them
			remoteAddr = ""
			remotePort = 0
		}

		fwd := &model.Forward{
			ServerID:    server.ID,
			Name:        name,
			Description: description,
			Type:        model.ForwardType(fwdType),
			LocalAddr:   localAddr,
			LocalPort:   localPort,
			RemoteAddr:  remoteAddr,
			RemotePort:  remotePort,
		}

		fwd.Enabled = true
		fwdID, err := appDB.AddForward(fwd)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("add forward", "добавить перенаправление"), err)
		}
		fmt.Printf(tr("✓ Forward added [%d]\n", "✓ Перенаправление добавлено [%d]\n"), fwdID)
		return nil
	},
}

var forwardEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: tr("Edit a port forward", "Изменить перенаправление порта"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("%s", trf("invalid forward ID: %s", "недопустимый ID перенаправления: %s", args[0]))
		}

		fwd, err := appDB.GetForward(id)
		if err != nil {
			return fmt.Errorf("%s", trf("forward not found: %d", "перенаправление не найдено: %d", id))
		}

		enabled, _ := cmd.Flags().GetBool("enabled")
		if cmd.Flags().Changed("enabled") {
			fwd.Enabled = enabled
		}
		if err := appDB.UpdateForward(fwd); err != nil {
			return fmt.Errorf("%s: %w", tr("update forward", "обновить перенаправление"), err)
		}

		fmt.Printf(tr("✓ Forward %d updated\n", "✓ Перенаправление %d обновлено\n"), id)
		return nil
	},
}

var forwardDeleteCmd = &cobra.Command{
	Use:   "delete <alias> <id>",
	Short: tr("Delete a port forward", "Удалить перенаправление порта"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("%s", trf("invalid forward ID: %s", "недопустимый ID перенаправления: %s", args[1]))
		}
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}
		// Verify forward belongs to this server
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("load forwards", "загрузить перенаправления"), err)
		}
		found := false
		for _, f := range forwards {
			if f.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s", trf("forward %d does not belong to server %s", "перенаправление %d не принадлежит серверу %s", id, alias))
		}
		if err := appDB.DeleteForward(id); err != nil {
			return fmt.Errorf("%s: %w", tr("delete forward", "удалить перенаправление"), err)
		}
		fmt.Println(tr("✓ Forward deleted", "✓ Перенаправление удалено"))
		return nil
	},
}

func init() {
	forwardAddCmd.Flags().String("type", "local", tr("Forward type: local, remote, dynamic", "Тип перенаправления: local, remote, dynamic"))
	forwardAddCmd.Flags().String("name", "", tr("Forward name", "Имя перенаправления"))
	forwardAddCmd.Flags().String("description", "", tr("Forward description", "Описание перенаправления"))
	forwardAddCmd.Flags().String("local-addr", "127.0.0.1", tr("Listen address", "Адрес прослушивания"))
	forwardAddCmd.Flags().Int("local-port", 0, tr("Listen port", "Порт прослушивания"))
	forwardAddCmd.MarkFlagRequired("local-port")
	forwardAddCmd.Flags().String("remote-addr", "", tr("Target address", "Адрес назначения"))
	forwardAddCmd.Flags().Int("remote-port", 0, tr("Target port", "Порт назначения"))
	forwardEditCmd.Flags().Bool("enabled", true, tr("Enable/disable forward", "Включить или выключить перенаправление"))

	forwardCmd.AddCommand(forwardListCmd)
	forwardCmd.AddCommand(forwardAddCmd)
	forwardCmd.AddCommand(forwardDeleteCmd)
	forwardCmd.AddCommand(forwardEditCmd)
}
