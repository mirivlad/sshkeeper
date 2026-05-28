package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
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
	startup      string
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
		return addInteractive()
	},
}

func addInteractive() error {
	server, err := promptServerForAdd(os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	return saveServerWithOptionalSecret(server)
}

func addNonInteractive(alias string) error {
	server := &model.Server{
		Alias:          alias,
		DisplayName:    addFlags.displayName,
		Host:           addFlags.host,
		Port:           addFlags.port,
		User:           addFlags.user,
		AuthMethod:     model.AuthMethod(addFlags.authMethod),
		IdentityFile:   addFlags.identityFile,
		ProxyJump:      addFlags.proxyJump,
		GroupName:      addFlags.groupName,
		Notes:          addFlags.notes,
		StartupCommand: addFlags.startup,
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

	return saveServerWithOptionalSecret(server)
}

func saveServerWithOptionalSecret(server *model.Server) error {
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
		if err := unlockVaultForCommand(v); err != nil {
			return err
		}

		vaultKey := fmt.Sprintf("server:%s:ssh_password", server.Alias)
		vaultType := "ssh_password"
		if server.AuthMethod == model.AuthKeyPassphrase {
			vaultKey = fmt.Sprintf("server:%s:key_passphrase", server.Alias)
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
		server.Tags = strings.Split(addFlags.tags, ",")
	}
	if len(server.Tags) > 0 {
		if err := appDB.SetServerTags(server.ID, server.Tags); err != nil {
			return fmt.Errorf("set tags: %w", err)
		}
	}

	fmt.Println("Saved.")
	return nil
}

func promptServerForAdd(in io.Reader, out io.Writer) (*model.Server, error) {
	reader := bufio.NewReader(in)

	alias, err := promptRequired(reader, out, "Alias")
	if err != nil {
		return nil, err
	}
	displayName, err := promptOptional(reader, out, "Display name", alias)
	if err != nil {
		return nil, err
	}
	host, err := promptRequired(reader, out, "Host")
	if err != nil {
		return nil, err
	}
	portText, err := promptOptional(reader, out, "Port", "22")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return nil, fmt.Errorf("invalid port: %s", portText)
	}
	user, err := promptOptional(reader, out, "User", "root")
	if err != nil {
		return nil, err
	}
	authText, err := promptOptional(reader, out, "Auth method (password/key/key_passphrase/agent)", string(model.AuthKey))
	if err != nil {
		return nil, err
	}
	authMethod := model.AuthMethod(authText)
	if !isSupportedAuthMethod(authMethod) {
		return nil, fmt.Errorf("unsupported auth method: %s", authText)
	}
	identityFile, err := promptOptional(reader, out, "Identity file", "")
	if err != nil {
		return nil, err
	}
	proxyJump, err := promptOptional(reader, out, "ProxyJump", "")
	if err != nil {
		return nil, err
	}
	groupName, err := promptOptional(reader, out, "Group", "")
	if err != nil {
		return nil, err
	}
	notes, err := promptOptional(reader, out, "Notes", "")
	if err != nil {
		return nil, err
	}
	startupCommand, err := promptOptional(reader, out, "Startup command", "")
	if err != nil {
		return nil, err
	}
	tagsText, err := promptOptional(reader, out, "Tags (comma-separated)", "")
	if err != nil {
		return nil, err
	}

	return &model.Server{
		Alias:          alias,
		DisplayName:    displayName,
		Host:           host,
		Port:           port,
		User:           user,
		AuthMethod:     authMethod,
		IdentityFile:   identityFile,
		ProxyJump:      proxyJump,
		GroupName:      groupName,
		Notes:          notes,
		StartupCommand: startupCommand,
		Tags:           strings.Split(tagsText, ","),
	}, nil
}

func promptRequired(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		value, err := promptOptional(reader, out, label, "")
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		fmt.Fprintf(out, "%s is required.\n", label)
	}
}

func promptOptional(reader *bufio.Reader, out io.Writer, label string, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Fprintf(out, "%s: ", label)
	} else {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func isSupportedAuthMethod(method model.AuthMethod) bool {
	switch method {
	case model.AuthPassword, model.AuthKey, model.AuthKeyPassphrase, model.AuthAgent:
		return true
	default:
		return false
	}
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
	addCmd.Flags().StringVar(&addFlags.startup, "startup-command", "", "Command to run after connecting")
	addCmd.Flags().StringVar(&addFlags.tags, "tags", "", "Comma-separated tags")
}
