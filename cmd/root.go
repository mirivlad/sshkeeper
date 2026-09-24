package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	tunnelpkg "github.com/mirivlad/sshkeeper/internal/tunnel"
	"github.com/mirivlad/sshkeeper/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	cfg   *config.Config
	appDB *db.DB
)

var rootCmd = &cobra.Command{
	Use:     "sshkeeper",
	Version: Version,
	Short:   tr("sshkeeper — SSH connection manager", "sshkeeper — менеджер SSH-подключений"),
	Long: tr(`sshkeeper is a console SSH connection manager.
Linux and macOS are primary release targets; Windows is experimental.
It manages server profiles, secrets, and provides a convenient way
to launch SSH sessions using the system OpenSSH client.`, `sshkeeper — консольный менеджер SSH-подключений.
Основные целевые платформы — Linux и macOS; поддержка Windows экспериментальная.
Он управляет профилями серверов и секретами и запускает SSH-сеансы
через системный клиент OpenSSH.`),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func Execute() {
	if language, err := config.ReadLanguage(); err == nil {
		_ = i18n.SetPreference(language)
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	localizeCLIHelp()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("sshkeeper {{.Version}}\n")
	cobra.OnInitialize(initApp)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(vaultCmd)
	rootCmd.AddCommand(sshConfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(groupCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(runTemplateCmd)
	rootCmd.AddCommand(routeCmd)
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(sessionConnectCmd)
}

func initApp() {
	if commandSkipsAppInitialization(os.Args[1:]) {
		return
	}

	var err error

	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, trf("Error loading config: %v", "Ошибка загрузки конфигурации: %v", err))
		os.Exit(1)
	}
	if err := i18n.SetPreference(cfg.UI.Language); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	localizeCLIHelp()

	appDB, err = db.Open(cfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, trf("Error opening database: %v", "Ошибка открытия базы данных: %v", err))
		os.Exit(1)
	}

	// Initialize tunnel state manager
	if err := tunnelpkg.Init(cfg.DataDir); err != nil {
		fmt.Fprintln(os.Stderr, trf("Error initializing tunnel manager: %v", "Ошибка инициализации менеджера туннелей: %v", err))
		os.Exit(1)
	}

	// Handle vault: create on first run, unlock on subsequent runs
	vaultPath := config.VaultPath(cfg.DataDir)
	v := vault.New(vaultPath)

	if !vault.Exists(vaultPath) {
		// First run — create vault
		fmt.Println(tr("Welcome to sshkeeper!", "Добро пожаловать в sshkeeper!"))
		fmt.Println(tr("No vault found. Let's create one.", "Хранилище не найдено. Создадим его."))
		fmt.Println()

		for {
			fmt.Print(tr("Create master password: ", "Создайте мастер-пароль: "))
			pw1, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Error reading password: %v", "Ошибка чтения пароля: %v", err))
				os.Exit(1)
			}

			if len(pw1) == 0 {
				fmt.Println(tr("Password cannot be empty. Try again.", "Пароль не может быть пустым. Повторите попытку."))
				continue
			}

			fmt.Print(tr("Repeat master password: ", "Повторите мастер-пароль: "))
			pw2, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Error reading password: %v", "Ошибка чтения пароля: %v", err))
				os.Exit(1)
			}

			if string(pw1) != string(pw2) {
				fmt.Println(tr("Passwords do not match. Try again.", "Пароли не совпадают. Повторите попытку."))
				continue
			}

			if err := vault.Create(vaultPath, string(pw1)); err != nil {
				fmt.Fprintln(os.Stderr, trf("Error creating vault: %v", "Ошибка создания хранилища: %v", err))
				os.Exit(1)
			}

			// Unlock immediately after creation
			if err := v.Unlock(string(pw1)); err != nil {
				fmt.Fprintln(os.Stderr, trf("Error unlocking vault: %v", "Ошибка разблокировки хранилища: %v", err))
				os.Exit(1)
			}

			vaultInstance = v
			fmt.Println()
			fmt.Println(tr("Vault created and unlocked for this command. You're ready to go!", "Хранилище создано и разблокировано для этой команды. Всё готово!"))
			fmt.Println()
			break
		}
	} else {
		// Vault exists — unlock only for commands that may need secrets.
		if !commandRequiresStartupVaultUnlock(os.Args[1:]) {
			vaultInstance = v
			return
		}

		for attempts := 0; attempts < 3; attempts++ {
			fmt.Print(tr("Master password: ", "Мастер-пароль: "))
			pw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Error reading password: %v", "Ошибка чтения пароля: %v", err))
				os.Exit(1)
			}

			if err := v.Unlock(string(pw)); err != nil {
				remaining := 2 - attempts
				if remaining > 0 {
					fmt.Println(trf("Invalid password. %d attempts remaining.", "Неверный пароль. Осталось попыток: %d.", remaining))
					continue
				}
				fmt.Fprintln(os.Stderr, tr("Too many failed attempts. Start the command again to retry.", "Слишком много неудачных попыток. Запустите команду снова."))
				os.Exit(1)
			}

			vaultInstance = v
			fmt.Println(tr("Vault unlocked.", "Хранилище разблокировано."))
			fmt.Println()
			return
		}
	}
}

func commandSkipsAppInitialization(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "--version" {
			return true
		}
	}
	return args[0] == "version"
}

func commandRequiresStartupVaultUnlock(args []string) bool {
	if len(args) == 0 {
		return true
	}

	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return false
		}
	}

	switch args[0] {
	case "connect", "c", "run", "run-template", "test", "edit", "delete", "tunnel":
		if args[0] == "tunnel" && tunnelCommandSkipsStartupVaultUnlock(args[1:]) {
			return false
		}
		return true
	default:
		return false
	}
}

func tunnelCommandSkipsStartupVaultUnlock(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "list", "stop", "stop-all":
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--background" {
			return true
		}
	}
	return false
}
