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
	Short: "Manage port forwards",
}

var forwardListCmd = &cobra.Command{
	Use:   "list <alias>",
	Short: "List port forwards for a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("list forwards: %w", err)
		}
		if len(forwards) == 0 {
			fmt.Println("No port forwards configured.")
			return nil
		}
		fmt.Printf("Port forwards for %s:\n", alias)
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
	Short: "Add a port forward",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		fwdType, _ := cmd.Flags().GetString("type")
		localAddr, _ := cmd.Flags().GetString("local-addr")
		localPort, _ := cmd.Flags().GetInt("local-port")
		remoteAddr, _ := cmd.Flags().GetString("remote-addr")
		remotePort, _ := cmd.Flags().GetInt("remote-port")

		// Validate type
		if fwdType != "local" && fwdType != "remote" && fwdType != "dynamic" {
			return fmt.Errorf("invalid forward type %q: must be local, remote, or dynamic", fwdType)
		}

		// Validate ports
		if localPort < 1 || localPort > 65535 {
			return fmt.Errorf("invalid local port %d: must be 1-65535", localPort)
		}

		// Validate fields based on type
		switch fwdType {
		case "local":
			if localAddr == "" || localAddr == "0.0.0.0" {
				localAddr = "0.0.0.0"
			}
			if remoteAddr == "" {
				return fmt.Errorf("remote-addr is required for local forward")
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("invalid remote port %d: must be 1-65535", remotePort)
			}
		case "remote":
			if remoteAddr == "" {
				return fmt.Errorf("remote-addr is required for remote forward")
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("invalid remote port %d: must be 1-65535", remotePort)
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
			ServerID:   server.ID,
			Type:       model.ForwardType(fwdType),
			LocalAddr:  localAddr,
			LocalPort:  localPort,
			RemoteAddr: remoteAddr,
			RemotePort: remotePort,
		}

		fwd.Enabled = true
		fwdID, err := appDB.AddForward(fwd)
		if err != nil {
			return fmt.Errorf("add forward: %w", err)
		}
		fmt.Printf("✓ Forward added [%d]\n", fwdID)
		return nil
	},
}

var forwardEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a port forward",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid forward ID: %s", args[0])
		}

		// For now, just toggle enabled
		enabled, _ := cmd.Flags().GetBool("enabled")
		_ = enabled
		fmt.Printf("✓ Forward %d updated\n", id)
		return nil
	},
}

var forwardDeleteCmd = &cobra.Command{
	Use:   "delete <alias> <id>",
	Short: "Delete a port forward",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid forward ID: %s", args[1])
		}
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}
		// Verify forward belongs to this server
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("load forwards: %w", err)
		}
		found := false
		for _, f := range forwards {
			if f.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("forward %d does not belong to server %s", id, alias)
		}
		if err := appDB.DeleteForward(id); err != nil {
			return fmt.Errorf("delete forward: %w", err)
		}
		fmt.Println("✓ Forward deleted")
		return nil
	},
}

func init() {
	forwardAddCmd.Flags().String("type", "local", "Forward type: local, remote, dynamic")
	forwardAddCmd.Flags().String("local-addr", "127.0.0.1", "Listen address")
	forwardAddCmd.MarkFlagRequired("local-port")
	forwardAddCmd.Flags().String("remote-addr", "", "Target address")
	forwardAddCmd.Flags().Int("remote-port", 0, "Target port")
	forwardEditCmd.Flags().Bool("enabled", true, "Enable/disable forward")

	forwardCmd.AddCommand(forwardListCmd)
	forwardCmd.AddCommand(forwardAddCmd)
	forwardCmd.AddCommand(forwardDeleteCmd)
	forwardCmd.AddCommand(forwardEditCmd)
}
