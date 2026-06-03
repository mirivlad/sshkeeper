package cmd

import (
	"fmt"

	"github.com/mirivlad/sshkeeper/internal/ssh"
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

		// Load forwards
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return fmt.Errorf("load forwards: %w", err)
		}

		if len(forwards) == 0 && forwardsOnly {
			return fmt.Errorf("no forwards configured for %s", alias)
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

func init() {
	tunnelCmd.Flags().Bool("forward-only", false, "Start tunnel only (ssh -N)")
}
