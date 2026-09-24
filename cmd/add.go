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
	route        string
	groupName    string
	displayName  string
	notes        string
	startup      string
	tags         string
}

var addCmd = &cobra.Command{
	Use:   "add [alias]",
	Short: tr("Add a new server", "Добавить сервер"),
	Long:  tr("Add a new server profile. If alias is provided with --host, non-interactive mode is used.", "Добавить профиль сервера. Если указаны псевдоним и --host, используется неинтерактивный режим."),
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
	routeSpec := strings.TrimSpace(addFlags.route)
	if routeSpec == "" {
		routeSpec = strings.TrimSpace(addFlags.proxyJump)
	}
	route, err := parseRouteSpec(routeSpec)
	if err != nil {
		return fmt.Errorf("%s: %w", tr("route", "маршрут"), err)
	}
	server := &model.Server{
		Alias:          alias,
		DisplayName:    addFlags.displayName,
		Host:           addFlags.host,
		Port:           addFlags.port,
		User:           addFlags.user,
		AuthMethod:     model.AuthMethod(addFlags.authMethod),
		IdentityFile:   addFlags.identityFile,
		Route:          route,
		ProxyJump:      route.ProxyJumpString(),
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
	if len(server.Route.Hops) == 0 && strings.TrimSpace(server.ProxyJump) != "" {
		route, err := parseRouteSpec(server.ProxyJump)
		if err != nil {
			return fmt.Errorf("%s: %w", tr("route", "маршрут"), err)
		}
		server.Route = route
	}
	server.ProxyJump = server.Route.ProxyJumpString()
	if err := model.ValidateServerBasics(server); err != nil {
		return err
	}

	if addFlags.tags != "" {
		server.Tags = strings.Split(addFlags.tags, ",")
	}

	var secret []byte
	var v = getOrCreateVault()
	needsSecret := server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase
	if needsSecret {
		secretType := "password"
		if server.AuthMethod == model.AuthKeyPassphrase {
			secretType = "passphrase"
		}
		fmt.Print(trf("Enter %s (will be stored in vault, input hidden): ", "Введите %s (сохранится в хранилище, ввод скрыт): ", secretType))
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("%s: %w", trf("read %s", "прочитать %s", secretType), err)
		}
		if len(password) == 0 {
			return fmt.Errorf("%s", trf("%s cannot be empty", "%s не может быть пустым", secretType))
		}
		secret = password
		defer func() {
			for i := range secret {
				secret[i] = 0
			}
		}()
		if err := unlockVaultForCommand(v); err != nil {
			return err
		}
	}

	if err := appDB.CreateServer(server); err != nil {
		return fmt.Errorf("%s: %w", tr("create server", "создать сервер"), err)
	}
	rollbackDB := func() { _ = appDB.DeleteServer(server.Alias) }

	if len(server.Tags) > 0 {
		if err := appDB.SetServerTags(server.ID, server.Tags); err != nil {
			rollbackDB()
			return fmt.Errorf("%s: %w", tr("set tags", "назначить теги"), err)
		}
	}

	if needsSecret {
		if err := syncServerSecrets(v, "", server, string(secret)); err != nil {
			rollbackDB()
			return fmt.Errorf("%s: %w", tr("store secret in vault", "сохранить секрет в хранилище"), err)
		}
		if err := v.Save(); err != nil {
			cleanupServerSecretsForServer(v, server)
			rollbackDB()
			return fmt.Errorf("%s: %w", tr("save vault", "сохранить хранилище"), err)
		}
	}

	fmt.Println(tr("Saved.", "Сохранено."))
	return nil
}

func promptServerForAdd(in io.Reader, out io.Writer) (*model.Server, error) {
	reader := bufio.NewReader(in)

	alias, err := promptRequired(reader, out, tr("Alias", "Псевдоним"))
	if err != nil {
		return nil, err
	}
	displayName, err := promptOptional(reader, out, tr("Display name", "Отображаемое имя"), alias)
	if err != nil {
		return nil, err
	}
	host, err := promptRequired(reader, out, tr("Host", "Хост"))
	if err != nil {
		return nil, err
	}
	portText, err := promptOptional(reader, out, tr("Port", "Порт"), "22")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return nil, fmt.Errorf("%s", trf("invalid port: %s", "недопустимый порт: %s", portText))
	}
	user, err := promptOptional(reader, out, tr("User", "Пользователь"), "root")
	if err != nil {
		return nil, err
	}
	authText, err := promptOptional(reader, out, tr("Auth method (password/key/key_passphrase/agent)", "Способ входа (password/key/key_passphrase/agent)"), string(model.AuthKey))
	if err != nil {
		return nil, err
	}
	authMethod := model.AuthMethod(authText)
	if !isSupportedAuthMethod(authMethod) {
		return nil, fmt.Errorf("%s", trf("unsupported auth method: %s", "неподдерживаемый способ входа: %s", authText))
	}
	identityFile, err := promptOptional(reader, out, tr("Identity file", "Файл ключа"), "")
	if err != nil {
		return nil, err
	}
	proxyJump, err := promptOptional(reader, out, tr("Route / ProxyJump (profile:<alias> or raw:<target>)", "Маршрут / ProxyJump (profile:<alias> или raw:<target>)"), "")
	if err != nil {
		return nil, err
	}
	groupName, err := promptOptional(reader, out, tr("Group", "Группа"), "")
	if err != nil {
		return nil, err
	}
	notes, err := promptOptional(reader, out, tr("Notes", "Заметки"), "")
	if err != nil {
		return nil, err
	}
	startupCommand, err := promptOptional(reader, out, tr("Startup command", "Команда при подключении"), "")
	if err != nil {
		return nil, err
	}
	tagsText, err := promptOptional(reader, out, tr("Tags (comma-separated)", "Теги (через запятую)"), "")
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
		fmt.Fprintln(out, trf("%s is required.", "%s — обязательное поле.", label))
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
	addCmd.Flags().StringVar(&addFlags.host, "host", "", tr("Server hostname or IP", "Имя хоста или IP-адрес сервера"))
	addCmd.Flags().IntVar(&addFlags.port, "port", 22, tr("SSH port", "Порт SSH"))
	addCmd.Flags().StringVar(&addFlags.user, "user", "", tr("SSH username", "Имя пользователя SSH"))
	addCmd.Flags().StringVar(&addFlags.authMethod, "auth", "key", tr("Auth method: password, key, key_passphrase, agent", "Способ входа: password, key, key_passphrase, agent"))
	addCmd.Flags().StringVar(&addFlags.identityFile, "identity-file", "", tr("Path to SSH private key", "Путь к закрытому ключу SSH"))
	addCmd.Flags().StringVar(&addFlags.route, "route", "", tr("Route hops: profile:<alias>, raw:<target>, comma-separated", "Узлы маршрута через запятую: profile:<alias>, raw:<target>"))
	addCmd.Flags().StringVar(&addFlags.proxyJump, "proxy-jump", "", tr("Compatibility alias for --route", "Совместимый псевдоним для --route"))
	addCmd.Flags().StringVar(&addFlags.groupName, "group", "", tr("Server group", "Группа сервера"))
	addCmd.Flags().StringVar(&addFlags.displayName, "display-name", "", tr("Display name", "Отображаемое имя"))
	addCmd.Flags().StringVar(&addFlags.notes, "notes", "", tr("Notes", "Заметки"))
	addCmd.Flags().StringVar(&addFlags.startup, "startup-command", "", tr("Command to run after connecting", "Команда после подключения"))
	addCmd.Flags().StringVar(&addFlags.tags, "tags", "", tr("Comma-separated tags", "Теги через запятую"))
}
