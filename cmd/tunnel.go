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
	Short: "Start SSH session with port forwards",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		forwardsOnly, _ := cmd.Flags().GetBool("forward-only")
		background, _ := cmd.Flags().GetBool("background")

		// Load forwards
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("load forwards: %w", err)
		}

		if background {
			if err := validateBackgroundTunnel(server, forwards); err != nil {
				return err
			}
			state, err := tunnelpkg.Start(cfg, server, forwards, true)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Tunnel started [%d] PID %d → %s\n", state.ID, state.PID, server.Alias)
			return nil
		}

		if len(forwards) == 0 && forwardsOnly {
			return fmt.Errorf("no forwards configured for %s", alias)
		}

		v := getOrCreateVault()
		vaultFunc := func(serverAlias string, secretType string) (string, error) {
			if !v.IsUnlocked() {
				return "", fmt.Errorf("%s", vaultLockedProcessMessage())
			}
			key := fmt.Sprintf("server:%s:%s", serverAlias, secretType)
			data, err := v.Get(key)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}

		if len(forwards) > 0 {
			fmt.Printf("Starting tunnel to %s with %d forward(s)...\n", alias, len(forwards))
		} else {
			fmt.Printf("Starting session to %s...\n", alias)
		}

		sshArgs := ssh.BuildSSHArgs(server, forwards, forwardsOnly)
		if forwardsOnly {
			fmt.Printf("Tunnel mode (ssh -N). Press Ctrl+C to exit.\n")
		}

		return ssh.ConnectWithArgs(cfg, sshArgs, vaultFunc, server)
	},
}

var tunnelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked background tunnels",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		states := tunnelpkg.List()
		if len(states) == 0 {
			fmt.Println("No tracked tunnels.")
			return nil
		}
		fmt.Printf("%-22s %-8s %-10s %s\n", "ID", "PID", "STATUS", "SERVER")
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
	Short: "Stop a tracked background tunnel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid tunnel ID: %s", args[0])
		}
		if err := tunnelpkg.Stop(id); err != nil {
			return err
		}
		fmt.Printf("✓ Tunnel %d stopped\n", id)
		return nil
	},
}

var tunnelStopAllCmd = &cobra.Command{
	Use:   "stop-all",
	Short: "Stop all tracked background tunnels",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tunnelpkg.StopAll(); err != nil {
			return err
		}
		fmt.Println("✓ All tracked tunnels stopped")
		return nil
	},
}

func validateBackgroundTunnel(server *model.Server, forwards []*model.Forward) error {
	if server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase {
		return fmt.Errorf("background tunnels support only key or agent auth; use foreground tunnel for %s auth", server.AuthMethod)
	}
	for _, f := range forwards {
		if f.Enabled {
			return nil
		}
	}
	return fmt.Errorf("no enabled forwards configured for %s", server.Alias)
}

func init() {
	tunnelCmd.Flags().Bool("forward-only", false, "Start tunnel only (ssh -N)")
	tunnelCmd.Flags().Bool("background", false, "Start tunnel in background (ssh -N)")
	tunnelCmd.AddCommand(tunnelListCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelStopAllCmd)
}
