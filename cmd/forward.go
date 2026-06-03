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

		fwd := &model.Forward{
			ServerID:   server.ID,
			Type:       model.ForwardType(fwdType),
			LocalAddr:  localAddr,
			LocalPort:  localPort,
			RemoteAddr: remoteAddr,
			RemotePort: remotePort,
		}

		if err := appDB.AddForward(fwd.ServerID, fwd.Type, fwd.LocalAddr, fwd.LocalPort, fwd.RemoteAddr, fwd.RemotePort); err != nil {
			return fmt.Errorf("add forward: %w", err)
		}
		fmt.Printf("✓ Forward added [%d]\n", fwd.ID)
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
		// Verify server exists
		if _, err := appDB.GetServer(alias); err != nil {
			return fmt.Errorf("server not found: %s", alias)
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
	forwardAddCmd.Flags().String("local-addr", "0.0.0.0", "Listen address")
	forwardAddCmd.Flags().Int("local-port", 0, "Listen port (required)")
	forwardAddCmd.MarkFlagRequired("local-port")
	forwardAddCmd.Flags().String("remote-addr", "", "Target address")
	forwardAddCmd.Flags().Int("remote-port", 0, "Target port")

	forwardCmd.AddCommand(forwardListCmd)
	forwardCmd.AddCommand(forwardAddCmd)
	forwardCmd.AddCommand(forwardDeleteCmd)
}
