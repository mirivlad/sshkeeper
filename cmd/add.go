package cmd

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/model"
	"golang.org/x/term"
)

var addFlags struct {
	host         string
	port         int
	user         string
	authMethod   string
	identityFile string
	proxyJump    string
	groupName    string
	displayName  string
	notes        string
	tags         string
}

var addCmd = &cobra.Command{
	Use:   "add [alias]",
	Short: "Add a new server",
	Long:  "Add a new server profile. If alias is provided with --host, non-interactive mode is used.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && addFlags.host != "" {
			return addNonInteractive(args[0])
		}
		return fmt.Errorf("interactive add not yet implemented, use: sshkeeper add <alias> --host <host> --user <user> --auth <method>")
	},
}

func addNonInteractive(alias string) error {
	server := &model.Server{
		Alias:        alias,
		DisplayName:  addFlags.displayName,
		Host:         addFlags.host,
		Port:         addFlags.port,
		User:         addFlags.user,
		AuthMethod:   model.AuthMethod(addFlags.authMethod),
		IdentityFile: addFlags.identityFile,
		ProxyJump:    addFlags.proxyJump,
		GroupName:    addFlags.groupName,
		Notes:        addFlags.notes,
	}

	if server.Port == 0 {
		server.Port = 22
	}
	if server.AuthMethod == "" {
		server.AuthMethod = model.AuthKey
	}
	if server.DisplayName == "" {
		server.DisplayName = alias
	}

	// Handle password/passphrase auth — request interactively, never via argv
	if server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase {
		secretType := "password"
		if server.AuthMethod == model.AuthKeyPassphrase {
			secretType = "passphrase"
		}

		fmt.Printf("Enter %s (will be stored in vault, input hidden): ", secretType)
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read %s: %w", secretType, err)
		}
		if len(password) == 0 {
			return fmt.Errorf("%s cannot be empty", secretType)
		}

		v := getOrCreateVault()
		if !v.IsUnlocked() {
			return fmt.Errorf("vault is locked. Run 'sshkeeper vault unlock' first")
		}

		vaultKey := fmt.Sprintf("server:%s:ssh_password", alias)
		vaultType := "ssh_password"
		if server.AuthMethod == model.AuthKeyPassphrase {
			vaultKey = fmt.Sprintf("server:%s:key_passphrase", alias)
			vaultType = "key_passphrase"
		}

		if err := v.Put(vaultKey, vaultType, password); err != nil {
			return fmt.Errorf("store %s in vault: %w", secretType, err)
		}
		if err := v.Save(); err != nil {
			return fmt.Errorf("save vault: %w", err)
		}
	}

	if err := appDB.CreateServer(server); err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if addFlags.tags != "" {
		tagList := strings.Split(addFlags.tags, ",")
		for _, t := range tagList {
			t = strings.TrimSpace(t)
			if t != "" {
				if err := appDB.AddTagToServer(server.ID, t); err != nil {
					return fmt.Errorf("add tag %s: %w", t, err)
				}
			}
		}
	}

	fmt.Println("Saved.")
	return nil
}

func init() {
	addCmd.Flags().StringVar(&addFlags.host, "host", "", "Server hostname or IP")
	addCmd.Flags().IntVar(&addFlags.port, "port", 22, "SSH port")
	addCmd.Flags().StringVar(&addFlags.user, "user", "", "SSH username")
	addCmd.Flags().StringVar(&addFlags.authMethod, "auth", "key", "Auth method: password, key, key_passphrase, agent")
	addCmd.Flags().StringVar(&addFlags.identityFile, "identity-file", "", "Path to SSH private key")
	addCmd.Flags().StringVar(&addFlags.proxyJump, "proxy-jump", "", "ProxyJump host")
	addCmd.Flags().StringVar(&addFlags.groupName, "group", "", "Server group")
	addCmd.Flags().StringVar(&addFlags.displayName, "display-name", "", "Display name")
	addCmd.Flags().StringVar(&addFlags.notes, "notes", "", "Notes")
	addCmd.Flags().StringVar(&addFlags.tags, "tags", "", "Comma-separated tags")
}
