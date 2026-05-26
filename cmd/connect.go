package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

var connectCmd = &cobra.Command{
	Use:     "connect <alias>",
	Aliases: []string{"c"},
	Short:   "Connect to a server via SSH",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		v := getOrCreateVault()
		vaultFunc := func(serverAlias string, secretType string) (string, error) {
			if !v.IsUnlocked() {
				return "", fmt.Errorf("vault is locked. Run 'sshkeeper vault unlock' first")
			}
			key := fmt.Sprintf("server:%s:%s", serverAlias, secretType)
			data, err := v.Get(key)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}

		if err := ssh.Connect(cfg, &model.Server{
			Alias:        server.Alias,
			Host:         server.Host,
			Port:         server.Port,
			User:         server.User,
			AuthMethod:   server.AuthMethod,
			IdentityFile: server.IdentityFile,
			ProxyJump:    server.ProxyJump,
		}, vaultFunc); err != nil {
			return err
		}

		appDB.UpdateLastConnected(alias)
		return nil
	},
}

var testCmd = &cobra.Command{
	Use:   "test <alias>",
	Short: "Test SSH connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		v := getOrCreateVault()
		vaultFunc := func(serverAlias string, secretType string) (string, error) {
			if !v.IsUnlocked() {
				return "", fmt.Errorf("vault is locked. Run 'sshkeeper vault unlock' first")
			}
			key := fmt.Sprintf("server:%s:%s", serverAlias, secretType)
			data, err := v.Get(key)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}

		ok, testErr := ssh.Test(cfg, &model.Server{
			Alias:        server.Alias,
			Host:         server.Host,
			Port:         server.Port,
			User:         server.User,
			AuthMethod:   server.AuthMethod,
			IdentityFile: server.IdentityFile,
			ProxyJump:    server.ProxyJump,
		}, vaultFunc)

		if ok {
			fmt.Println("Connection OK.")
			appDB.UpdateTestResult(alias, model.TestOK, "")
		} else {
			fmt.Printf("Connection failed:\n%s\n", testErr)
			appDB.UpdateTestResult(alias, model.TestFailed, testErr)
		}

		return nil
	},
}
