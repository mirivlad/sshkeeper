package cmd

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var editCmd = &cobra.Command{
	Use:   "edit <alias>",
	Short: tr("Edit a server profile", "Изменить профиль сервера"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}
		original := cloneServer(server)

		if cmd.Flags().Changed("host") {
			server.Host = parsedHost
		}
		if cmd.Flags().Changed("port") {
			if parsedPort < 1 || parsedPort > 65535 {
				return fmt.Errorf("%s", tr("port must be between 1 and 65535", "порт должен быть от 1 до 65535"))
			}
			server.Port = parsedPort
		}
		if cmd.Flags().Changed("user") {
			server.User = parsedUser
		}
		authChanged := cmd.Flags().Changed("auth")
		if authChanged {
			server.AuthMethod = model.AuthMethod(parsedAuth)
		}
		if cmd.Flags().Changed("identity-file") {
			server.IdentityFile = parsedIdentity
		}
		if cmd.Flags().Changed("group") {
			server.GroupName = parsedGroup
		}
		if cmd.Flags().Changed("display-name") {
			server.DisplayName = parsedDisplayName
		}
		if cmd.Flags().Changed("notes") {
			server.Notes = parsedNotes
		}
		if cmd.Flags().Changed("startup-command") {
			server.StartupCommand = parsedStartup
		}
		routeChanged := cmd.Flags().Changed("route") || cmd.Flags().Changed("proxy-jump")
		if routeChanged {
			if cmd.Flags().Changed("route") && cmd.Flags().Changed("proxy-jump") {
				return fmt.Errorf("%s", tr("use either --route or --proxy-jump, not both", "используйте либо --route, либо --proxy-jump"))
			}
			spec := parsedRoute
			if cmd.Flags().Changed("proxy-jump") {
				spec = parsedProxyJump
			}
			route, err := parseRouteSpec(spec)
			if err != nil {
				return fmt.Errorf("%s: %w", tr("route", "маршрут"), err)
			}
			server.Route = route
			server.ProxyJump = route.ProxyJumpString()
		}
		tagsChanged := cmd.Flags().Changed("tags")
		if tagsChanged {
			if strings.TrimSpace(parsedTags) == "" {
				server.Tags = nil
			} else {
				server.Tags = strings.Split(parsedTags, ",")
			}
		}
		if err := model.ValidateServerBasics(server); err != nil {
			return err
		}

		var secret []byte
		v := getOrCreateVault()
		if authChanged {
			if err := unlockVaultForCommand(v); err != nil {
				return err
			}
			switch server.AuthMethod {
			case model.AuthPassword, model.AuthKeyPassphrase:
				label := tr("password", "пароль")
				if server.AuthMethod == model.AuthKeyPassphrase {
					label = tr("key passphrase", "парольную фразу ключа")
				}
				fmt.Print(trf("Enter new %s (stored in vault, input hidden): ", "Введите новый %s (сохранится в хранилище, ввод скрыт): ", label))
				secret, err = term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("%s: %w", trf("read %s", "прочитать %s", label), err)
				}
				if len(secret) == 0 {
					return fmt.Errorf("%s", trf("%s cannot be empty", "%s не может быть пустым", label))
				}
				defer func() {
					for i := range secret {
						secret[i] = 0
					}
				}()
			}
		}

		if err := appDB.UpdateServerByAlias(alias, server); err != nil {
			return fmt.Errorf("%s: %w", tr("update server", "обновить сервер"), err)
		}
		rollback := func() {
			_ = appDB.UpdateServerByAlias(server.Alias, original)
			_ = appDB.SetServerTags(original.ID, original.Tags)
		}
		if tagsChanged {
			if err := appDB.SetServerTags(server.ID, server.Tags); err != nil {
				rollback()
				return fmt.Errorf("%s: %w", tr("set tags", "назначить теги"), err)
			}
		}
		if authChanged {
			if err := syncServerSecrets(v, alias, server, string(secret)); err != nil {
				rollback()
				return fmt.Errorf("%s: %w", tr("sync vault secrets", "синхронизировать секреты хранилища"), err)
			}
			if err := v.Save(); err != nil {
				rollback()
				return fmt.Errorf("%s: %w", tr("save vault", "сохранить хранилище"), err)
			}
		}
		fmt.Println(tr("Saved.", "Сохранено."))
		return nil
	},
}

func cloneServer(server *model.Server) *model.Server {
	copyServer := *server
	copyServer.Route.Hops = append([]model.RouteHop(nil), server.Route.Hops...)
	copyServer.Tags = append([]string(nil), server.Tags...)
	return &copyServer
}

var (
	parsedHost        string
	parsedPort        int
	parsedUser        string
	parsedAuth        string
	parsedIdentity    string
	parsedRoute       string
	parsedProxyJump   string
	parsedGroup       string
	parsedDisplayName string
	parsedNotes       string
	parsedStartup     string
	parsedTags        string
)

func init() {
	editCmd.Flags().StringVar(&parsedHost, "host", "", tr("Server hostname or IP", "Имя хоста или IP-адрес сервера"))
	editCmd.Flags().IntVar(&parsedPort, "port", 0, tr("SSH port", "Порт SSH"))
	editCmd.Flags().StringVar(&parsedUser, "user", "", tr("SSH username; empty lets OpenSSH choose", "Имя пользователя SSH; пустое — выбор OpenSSH"))
	editCmd.Flags().StringVar(&parsedAuth, "auth", "", tr("Auth method", "Способ входа"))
	editCmd.Flags().StringVar(&parsedIdentity, "identity-file", "", tr("Path to SSH private key; empty clears it", "Путь к закрытому ключу SSH; пустое — удалить"))
	editCmd.Flags().StringVar(&parsedRoute, "route", "", tr("Route hops: profile:<alias>, raw:<target>, comma-separated; empty means direct", "Узлы через запятую: profile:<alias>, raw:<target>; пустое — прямое подключение"))
	editCmd.Flags().StringVar(&parsedProxyJump, "proxy-jump", "", tr("Compatibility alias for --route", "Совместимый псевдоним для --route"))
	editCmd.Flags().StringVar(&parsedGroup, "group", "", tr("Server group; empty clears it", "Группа сервера; пустое — удалить"))
	editCmd.Flags().StringVar(&parsedDisplayName, "display-name", "", tr("Display name; empty clears it", "Отображаемое имя; пустое — удалить"))
	editCmd.Flags().StringVar(&parsedNotes, "notes", "", tr("Notes; empty clears them", "Заметки; пустое — удалить"))
	editCmd.Flags().StringVar(&parsedStartup, "startup-command", "", tr("Startup command; empty clears it", "Команда при подключении; пустое — удалить"))
	editCmd.Flags().StringVar(&parsedTags, "tags", "", tr("Comma-separated tags; empty clears all", "Теги через запятую; пустое — удалить все"))
}
