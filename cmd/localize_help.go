package cmd

import (
	"strings"

	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var originalUsageTemplate = rootCmd.UsageTemplate()

type commandDescription struct{ short, long string }

var completionDescriptions map[*cobra.Command]commandDescription

// localizeCLIHelp refreshes help after the saved language has been loaded.
// Command and flag names remain stable; only descriptions change.
func localizeCLIHelp() {
	usageTemplate := originalUsageTemplate
	usageTemplate = strings.NewReplacer(
		"Usage:", i18n.T("Usage:", "Использование:"),
		"Aliases:", i18n.T("Aliases:", "Псевдонимы:"),
		"Examples:", i18n.T("Examples:", "Примеры:"),
		"Available Commands:", i18n.T("Available Commands:", "Доступные команды:"),
		"Additional Commands:", i18n.T("Additional Commands:", "Дополнительные команды:"),
		"Global Flags:", i18n.T("Global Flags:", "Общие флаги:"),
		"Flags:", i18n.T("Flags:", "Флаги:"),
		"Additional help topics:", i18n.T("Additional help topics:", "Дополнительные разделы справки:"),
		"Use \"{{.CommandPath}} [command] --help\" for more information about a command.", i18n.T("Use \"{{.CommandPath}} [command] --help\" for more information about a command.", "Введите \"{{.CommandPath}} [command] --help\", чтобы узнать больше о команде."),
	).Replace(usageTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.InitDefaultVersionFlag()
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	if completionDescriptions == nil {
		completionDescriptions = make(map[*cobra.Command]commandDescription)
		for _, command := range rootCmd.Commands() {
			if command.Name() == "completion" {
				completionDescriptions[command] = commandDescription{command.Short, command.Long}
				for _, child := range command.Commands() {
					completionDescriptions[child] = commandDescription{child.Short, child.Long}
				}
			}
		}
	}
	if flag := rootCmd.Flags().Lookup("version"); flag != nil {
		flag.Usage = i18n.T("version for sshkeeper", "версия sshkeeper")
	}
	short := map[*cobra.Command][2]string{
		rootCmd:                    {"sshkeeper — SSH connection manager", "sshkeeper — менеджер SSH-подключений"},
		versionCmd:                 {"Print sshkeeper version", "Показать версию sshkeeper"},
		initCmd:                    {"Initialize sshkeeper", "Инициализировать sshkeeper"},
		addCmd:                     {"Add a new server", "Добавить сервер"},
		listCmd:                    {"List all servers", "Показать все серверы"},
		showCmd:                    {"Show server details", "Показать сведения о сервере"},
		editCmd:                    {"Edit a server profile", "Изменить профиль сервера"},
		deleteCmd:                  {"Delete a server profile", "Удалить профиль сервера"},
		connectCmd:                 {"Connect to a server via SSH", "Подключиться к серверу по SSH"},
		testCmd:                    {"Test SSH connection", "Проверить SSH-подключение"},
		searchCmd:                  {"Search servers by alias, host, name, group, notes, tags, route", "Поиск серверов по псевдониму, хосту, имени, группе, заметкам, тегам и маршруту"},
		vaultCmd:                   {"Vault management commands", "Управление хранилищем"},
		vaultUnlockCmd:             {"Verify the vault master password", "Проверить мастер-пароль хранилища"},
		vaultLockCmd:               {"Lock the vault", "Заблокировать хранилище"},
		vaultStatusCmd:             {"Show vault status", "Показать состояние хранилища"},
		vaultChangePasswordCmd:     {"Change master password", "Изменить мастер-пароль"},
		vaultListCmd:               {"List stored secret metadata", "Показать сведения о сохранённых секретах"},
		vaultDeleteCmd:             {"Delete stored secrets for a server", "Удалить секреты сервера"},
		sshConfigCmd:               {"OpenSSH config management", "Управление конфигурацией OpenSSH"},
		sshConfigGenerateCmd:       {"Generate OpenSSH config from server profiles", "Создать конфигурацию OpenSSH из профилей серверов"},
		sshConfigInstallIncludeCmd: {"Add Include directive to ~/.ssh/config", "Добавить директиву Include в ~/.ssh/config"},
		configCmd:                  {"Configuration management", "Управление конфигурацией"},
		configPathCmd:              {"Show config file paths", "Показать пути файлов конфигурации"},
		importCmd:                  {"Import servers from ~/.ssh/config", "Импортировать серверы из ~/.ssh/config"},
		exportCmd:                  {"Export servers to stdout", "Вывести серверы в stdout"},
		runCmd:                     {"Run a command on a server", "Выполнить команду на сервере"},
		groupCmd:                   {"Group management", "Управление группами"},
		groupListCmd:               {"List server groups", "Показать группы серверов"},
		groupRenameCmd:             {"Rename a group (updates all servers in the group)", "Переименовать группу (обновить все серверы группы)"},
		groupDeleteCmd:             {"Delete a group (removes group from all servers)", "Удалить группу у всех серверов"},
		templateCmd:                {"Global command template management", "Управление шаблонами команд"},
		templateListCmd:            {"List global command templates", "Показать шаблоны команд"},
		templateAddCmd:             {"Add a global command template", "Добавить шаблон команды"},
		templateEditCmd:            {"Edit a global command template", "Изменить шаблон команды"},
		templateDeleteCmd:          {"Delete a global command template", "Удалить шаблон команды"},
		runTemplateCmd:             {"Run a global command template on a server", "Выполнить шаблон команды на сервере"},
		routeCmd:                   {"Manage server routes (bastions / ProxyJump)", "Управление маршрутами серверов (бастионы / ProxyJump)"},
		routeShowCmd:               {"Show route for a server", "Показать маршрут сервера"},
		routeSetCmd:                {"Set route for a server", "Задать маршрут сервера"},
		routeClearCmd:              {"Clear route for a server (set direct)", "Очистить маршрут сервера (прямое подключение)"},
		forwardCmd:                 {"Manage port forwards", "Управление пробросами портов"},
		forwardListCmd:             {"List port forwards for a server", "Показать пробросы портов сервера"},
		forwardAddCmd:              {"Add a port forward", "Добавить проброс порта"},
		forwardEditCmd:             {"Edit a port forward", "Изменить проброс порта"},
		forwardDeleteCmd:           {"Delete a port forward", "Удалить проброс порта"},
		tunnelCmd:                  {"Start SSH session with port forwards", "Запустить SSH-сеанс с пробросами портов"},
		tunnelListCmd:              {"List tracked background tunnels", "Показать отслеживаемые фоновые туннели"},
		tunnelStopCmd:              {"Stop a tracked background tunnel", "Остановить фоновый туннель"},
		tunnelStopAllCmd:           {"Stop all tracked background tunnels", "Остановить все фоновые туннели"},
	}
	for command, pair := range short {
		command.Short = i18n.T(pair[0], pair[1])
	}
	for _, command := range rootCmd.Commands() {
		if command.Name() == "help" {
			command.Short = i18n.T("Help about any command", "Справка по любой команде")
			command.Long = i18n.T("Help provides help for any command in the application.\nSimply type sshkeeper help [path to command] for full details.", "Справка доступна для любой команды.\nВведите sshkeeper help [путь к команде], чтобы узнать подробности.")
		}
		if command.Name() == "completion" {
			original := completionDescriptions[command]
			command.Short = i18n.T(original.short, "Создать скрипт автодополнения для выбранной оболочки")
			command.Long = i18n.T(original.long, "Создать скрипт автодополнения sshkeeper для выбранной оболочки. Подробности — в справке вложенной команды.")
			for _, child := range command.Commands() {
				original := completionDescriptions[child]
				child.Short = i18n.T(original.short, "Создать скрипт автодополнения для "+child.Name())
				child.Long = i18n.T(original.long, child.Short)
			}
		}
	}
	rootCmd.Long = i18n.T(`sshkeeper is a console SSH connection manager.
Linux and macOS are primary release targets; Windows is experimental.
It manages server profiles, secrets, and provides a convenient way
to launch SSH sessions using the system OpenSSH client.`, `sshkeeper — консольный менеджер SSH-подключений.
Основные целевые платформы — Linux и macOS; поддержка Windows экспериментальная.
Он управляет профилями серверов и секретами и запускает SSH-сеансы
через системный клиент OpenSSH.`)
	initCmd.Long = i18n.T("Create config, database, and vault directories.", "Создать каталоги конфигурации, базы данных и хранилища.")
	addCmd.Long = i18n.T("Add a new server profile. If alias is provided with --host, non-interactive mode is used.", "Добавить профиль сервера. Если указаны псевдоним и --host, используется неинтерактивный режим.")
	routeSetCmd.Long = i18n.T(`Set an ordered route. Known aliases are resolved to stable sshkeeper profile IDs.
Use profile:<alias> to require a profile reference and raw:<target> to force a literal OpenSSH target.`, `Задать упорядоченный маршрут. Известные псевдонимы преобразуются в постоянные ID профилей sshkeeper.
Используйте profile:<alias> для ссылки на профиль или raw:<target> для буквального адреса OpenSSH.`)

	usage := map[string]string{
		"Server hostname or IP": "Имя хоста или IP-адрес сервера",
		"SSH port":              "Порт SSH",
		"SSH username":          "Имя пользователя SSH",
		"Auth method: password, key, key_passphrase, agent":          "Способ входа: password, key, key_passphrase, agent",
		"Path to SSH private key":                                    "Путь к закрытому ключу SSH",
		"Route hops: profile:<alias>, raw:<target>, comma-separated": "Узлы маршрута через запятую: profile:<alias>, raw:<target>",
		"Compatibility alias for --route":                            "Совместимый псевдоним для --route",
		"Server group":                                               "Группа сервера",
		"Display name":                                               "Отображаемое имя",
		"Notes":                                                      "Заметки",
		"Command to run after connecting":                            "Команда после подключения",
		"Comma-separated tags":                                       "Теги через запятую",
		"SSH username; empty lets OpenSSH choose":                    "Имя пользователя SSH; пустое — выбор OpenSSH",
		"Auth method":                                                "Способ входа",
		"Path to SSH private key; empty clears it":                   "Путь к закрытому ключу SSH; пустое — удалить",
		"Route hops: profile:<alias>, raw:<target>, comma-separated; empty means direct": "Узлы через запятую: profile:<alias>, raw:<target>; пустое — прямое подключение",
		"Server group; empty clears it":                                               "Группа сервера; пустое — удалить",
		"Display name; empty clears it":                                               "Отображаемое имя; пустое — удалить",
		"Notes; empty clears them":                                                    "Заметки; пустое — удалить",
		"Startup command; empty clears it":                                            "Команда при подключении; пустое — удалить",
		"Comma-separated tags; empty clears all":                                      "Теги через запятую; пустое — удалить все",
		"Delete without confirmation":                                                 "Удалить без подтверждения",
		"Route mode: via, chain, direct, or clear":                                    "Режим маршрута: via, chain, direct или clear",
		"Comma-separated hops; use profile:<alias> or raw:<target> for explicit type": "Узлы через запятую; profile:<alias> или raw:<target> задают тип",
		"Forward type: local, remote, dynamic":                                        "Тип проброса: local, remote, dynamic",
		"Forward name":                                                                "Имя проброса",
		"Forward description":                                                         "Описание проброса",
		"Listen address":                                                              "Адрес прослушивания",
		"Listen port":                                                                 "Порт прослушивания",
		"Target address":                                                              "Целевой адрес",
		"Target port":                                                                 "Целевой порт",
		"Enable/disable forward":                                                      "Включить/выключить проброс",
		"Start tunnel only (ssh -N)":                                                  "Запустить только туннель (ssh -N)",
		"Start tunnel in background (ssh -N)":                                         "Запустить туннель в фоне (ssh -N)",
	}
	for _, command := range append([]*cobra.Command{rootCmd}, rootCmd.Commands()...) {
		localizeCommandFlags(command, usage)
		for _, child := range command.Commands() {
			localizeCommandFlags(child, usage)
		}
	}
}

func localizeCommandFlags(command *cobra.Command, usage map[string]string) {
	command.InitDefaultHelpFlag()
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "help" {
			flag.Usage = i18n.T("help for "+command.Name(), "справка по "+command.Name())
			return
		}
		for english, russian := range usage {
			if flag.Usage == english || flag.Usage == russian {
				flag.Usage = i18n.T(english, russian)
				return
			}
		}
	})
}
