package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/syncer"
	"github.com/mirivlad/sshkeeper/internal/tui"
	"github.com/mirivlad/sshkeeper/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// syncTransport builds the storage from the [sync] settings.
func syncTransport(settings config.SyncConfig) (syncer.Transport, error) {
	switch settings.Mode {
	case config.SyncFolder:
		folder := strings.TrimSpace(settings.Folder)
		if folder == "" {
			return nil, fmt.Errorf("%s", tr("sync folder is not set", "папка синхронизации не задана"))
		}
		return &syncer.Folder{Dir: expandHome(folder)}, nil
	case config.SyncGit:
		if strings.TrimSpace(settings.GitURL) == "" {
			return nil, fmt.Errorf("%s", tr("git repository URL is not set", "адрес git-репозитория не задан"))
		}
		return &syncer.Git{URL: strings.TrimSpace(settings.GitURL), Branch: settings.GitBranch, Cache: filepath.Join(cfg.DataDir, "sync-git")}, nil
	default:
		return nil, fmt.Errorf("%s", tr("sync is off; choose a folder or a git repository first", "синхронизация выключена; сначала выберите папку или git-репозиторий"))
	}
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
		}
	}
	return path
}

// newSyncEngine returns the engine for the saved settings.
func newSyncEngine() (*syncer.Engine, error) {
	transport, err := syncTransport(cfg.Sync)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &syncer.Engine{DB: appDB, Vault: getOrCreateVault(), Transport: transport, Home: home}, nil
}

// verifyMasterPassword checks a password against the vault file without
// touching the unlocked vault in use.
func verifyMasterPassword(password string) error {
	check := vault.New(config.VaultPath(cfg.DataDir))
	if err := check.Unlock(password); err != nil {
		return fmt.Errorf("%s", tr("wrong master password", "неверный мастер-пароль"))
	}
	check.Lock()
	return nil
}

// syncErrorText turns engine errors into user-facing text.
func syncErrorText(err error) string {
	switch {
	case errors.Is(err, syncer.ErrNoKey):
		return tr("This device is not set up for sync yet. Create a sync space on the first device, then add others with a pairing code.",
			"Это устройство ещё не подключено к синхронизации. Создайте пространство синхронизации на первом устройстве, затем добавьте остальные по коду.")
	case errors.Is(err, syncer.ErrSpaceExists):
		return tr("The storage already has synced data. Add this device with a pairing code from a device that is already set up.",
			"В хранилище уже есть данные синхронизации. Добавьте это устройство по коду с уже подключённого устройства.")
	case errors.Is(err, syncer.ErrVaultLocked):
		return tr("Unlock the vault before syncing.", "Разблокируйте хранилище перед синхронизацией.")
	case errors.Is(err, syncer.ErrNoPairing):
		return tr("No pairing code is waiting. Choose \"Add device\" or \"Add later (24h)\" on a device that is already set up.",
			"Код подключения не найден. Выберите «Добавить устройство» или «Добавить позже (24 ч)» на уже подключённом устройстве.")
	case errors.Is(err, syncer.ErrPairingExpired):
		return tr("The pairing code has expired. Create a new one.", "Срок действия кода истёк. Создайте новый.")
	case errors.Is(err, syncer.ErrPairingInvalid):
		return tr("Wrong pairing/offline code or master password.", "Неверный обычный/офлайн-код или мастер-пароль.")
	case errors.Is(err, syncer.ErrWrongKey):
		return tr("This storage belongs to a different sync space.", "Это хранилище принадлежит другому пространству синхронизации.")
	}
	return err.Error()
}

func syncReportText(report syncer.Report) string {
	text := trf("Sync done: %d change(s) received, %d sent.", "Синхронизация завершена: получено изменений — %d, отправлено — %d.", report.Received, report.Sent)
	if len(report.Warnings) > 0 {
		text += " " + trf("%d warning(s).", "Предупреждений: %d.", len(report.Warnings))
	}
	return text
}

func printSyncReport(report syncer.Report) {
	fmt.Println(syncReportText(report))
	for _, warning := range report.Warnings {
		fmt.Println("  ! " + warning)
	}
}

func formatPairingCode(code string) string {
	if len(code) == 6 {
		return code[:3] + " " + code[3:]
	}
	return code
}

func readSecretLine(prompt string) (string, error) {
	fmt.Print(prompt)
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return string(value), err
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: tr("Sync profiles, secrets, and keys between devices", "Синхронизация профилей, секретов и ключей между устройствами"),
	Long: tr(`Keep profiles, forwards, templates, vault secrets, and the private keys that
profiles reference in step across devices through an end-to-end encrypted
bundle in a folder or a git repository.

  sshkeeper sync setup folder ~/Sync/sshkeeper   (or: setup git <url>)
  sshkeeper sync init                            first device: create the space
  sshkeeper sync add-device                      show a six-digit code
  sshkeeper sync add-device --offline            24-hour code for dual-boot/offline device
  sshkeeper sync join                            new device: enter a code
  sshkeeper sync                                 sync now`,
		`Синхронизирует профили, пробросы, шаблоны, секреты хранилища и закрытые
ключи, на которые ссылаются профили, через пакет со сквозным шифрованием в
папке или git-репозитории.

  sshkeeper sync setup folder ~/Sync/sshkeeper   (или: setup git <url>)
  sshkeeper sync init                            первое устройство: создать пространство
  sshkeeper sync add-device                      показать шестизначный код
  sshkeeper sync add-device --offline            код на 24 часа для dual-boot/выключенного устройства
  sshkeeper sync join                            новое устройство: ввести код
  sshkeeper sync                                 синхронизировать сейчас`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		report, err := engine.Sync()
		if err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		printSyncReport(report)
		return nil
	},
}

var syncSetupBranch string
var syncSetupNoAuto bool
var syncAddDeviceOffline bool

var syncSetupCmd = &cobra.Command{
	Use:   "setup folder <path> | git <url>",
	Short: tr("Choose where the encrypted bundle is stored", "Выбрать, где хранится зашифрованный пакет"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		settings := cfg.Sync
		settings.Auto = !syncSetupNoAuto
		switch args[0] {
		case config.SyncFolder:
			settings.Mode, settings.Folder = config.SyncFolder, args[1]
		case config.SyncGit:
			settings.Mode, settings.GitURL = config.SyncGit, args[1]
			if syncSetupBranch != "" {
				settings.GitBranch = syncSetupBranch
			}
		default:
			return fmt.Errorf("%s", tr("use \"folder\" or \"git\"", "укажите \"folder\" или \"git\""))
		}
		if err := cfg.SetSync(settings); err != nil {
			return err
		}
		fmt.Println(tr("Sync storage saved. Next: \"sshkeeper sync init\" on the first device or \"sshkeeper sync join\" on another.",
			"Хранилище синхронизации сохранено. Далее: «sshkeeper sync init» на первом устройстве или «sshkeeper sync join» на остальных."))
		return nil
	},
}

var syncInitCmd = &cobra.Command{
	Use:   "init",
	Short: tr("Create a sync space from this device", "Создать пространство синхронизации с этого устройства"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		recovery, report, err := engine.Create()
		if err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		printSyncReport(report)
		fmt.Println()
		fmt.Println(tr("Recovery key — keep it offline; it restores sync if every device is lost:",
			"Ключ восстановления — храните его офлайн; он вернёт синхронизацию, если все устройства потеряны:"))
		fmt.Println("  " + recovery)
		return nil
	},
}

var syncAddDeviceCmd = &cobra.Command{
	Use:   "add-device",
	Short: tr("Create a code to add another device", "Создать код для добавления другого устройства"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		if syncAddDeviceOffline {
			code, expires, err := engine.PairOffline()
			if err != nil {
				return fmt.Errorf("%s", syncErrorText(err))
			}
			fmt.Println(tr("Save this one-time code. If the storage is a folder synced by another app, let that app finish syncing it before rebooting. Then run \"sshkeeper sync join\" on the other device:", "Сохраните этот одноразовый код. Если хранилище — папка, которую синхронизирует другая программа, дождитесь окончания её синхронизации перед перезагрузкой. Затем выполните «sshkeeper sync join» на другом устройстве:"))
			fmt.Println()
			fmt.Println("    " + code)
			fmt.Println()
			fmt.Println(trf("No master password from this device is needed. Valid until %s.", "Мастер-пароль этого устройства не нужен. Код действует до %s.", expires.Format("2006-01-02 15:04")))
			return nil
		}
		password, err := readSecretLine(tr("Confirm master password: ", "Подтвердите мастер-пароль: "))
		if err != nil {
			return err
		}
		if err := verifyMasterPassword(password); err != nil {
			return err
		}
		code, expires, err := engine.Pair(password)
		if err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		fmt.Println(tr("On the new device run \"sshkeeper sync join\" and enter:", "На новом устройстве выполните «sshkeeper sync join» и введите:"))
		fmt.Println()
		fmt.Println("    " + formatPairingCode(code))
		fmt.Println()
		fmt.Println(trf("with the master password of this device. Valid until %s.", "и мастер-пароль этого устройства. Код действует до %s.", expires.Format("15:04")))
		return nil
	},
}

var syncJoinCmd = &cobra.Command{
	Use:   "join [code | offline-code | recovery-key]",
	Short: tr("Join a sync space with a pairing, offline, or recovery code", "Подключиться к синхронизации по обычному, офлайн-коду или ключу восстановления"),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		secret := strings.Join(args, " ")
		if secret == "" {
			if secret, err = readSecretLine(tr("Pairing, offline, or recovery code: ", "Код сопряжения, офлайн-код или ключ восстановления: ")); err != nil {
				return err
			}
		}
		password := ""
		if !syncer.LooksLikeRecoveryKey(secret) && !syncer.LooksLikeOfflinePairingCode(secret) {
			if password, err = readSecretLine(tr("Master password of the device showing the code: ", "Мастер-пароль устройства, показавшего код: ")); err != nil {
				return err
			}
		}
		report, err := engine.Join(secret, password)
		if err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		printSyncReport(report)
		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: tr("Show sync settings and the last result", "Показать настройки и последний результат синхронизации"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(trf("Storage: %s", "Хранилище: %s", syncStorageText(cfg.Sync)))
		engine, err := newSyncEngine()
		if err != nil {
			return nil
		}
		status := engine.Status()
		if !status.LastSync.IsZero() {
			fmt.Println(trf("Last sync: %s (%d items)", "Последняя синхронизация: %s (объектов: %d)", status.LastSync.Format(time.DateTime), status.Records))
		}
		if status.LastError != "" {
			fmt.Println(trf("Last error: %s", "Последняя ошибка: %s", status.LastError))
		}
		return nil
	},
}

func syncStorageText(settings config.SyncConfig) string {
	switch settings.Mode {
	case config.SyncFolder:
		return tr("folder ", "папка ") + settings.Folder
	case config.SyncGit:
		branch := settings.GitBranch
		if branch == "" {
			branch = syncer.DefaultGitBranch
		}
		return "git " + settings.GitURL + " (" + branch + ")"
	}
	return tr("off", "выключено")
}

var syncRecoveryKeyCmd = &cobra.Command{
	Use:   "recovery-key",
	Short: tr("Print the recovery key of this sync space", "Показать ключ восстановления пространства синхронизации"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		key, err := engine.RecoveryKey()
		if err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		fmt.Println(key)
		return nil
	},
}

var syncLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: tr("Stop syncing this device; local data stays", "Отключить устройство от синхронизации; данные остаются"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		engine, err := newSyncEngine()
		if err != nil {
			return err
		}
		if err := engine.Leave(); err != nil {
			return fmt.Errorf("%s", syncErrorText(err))
		}
		settings := cfg.Sync
		settings.Mode = config.SyncOff
		if err := cfg.SetSync(settings); err != nil {
			return err
		}
		fmt.Println(tr("This device no longer syncs. Its data is unchanged.", "Устройство больше не синхронизируется. Его данные не изменены."))
		return nil
	},
}

func init() {
	syncSetupCmd.Flags().StringVar(&syncSetupBranch, "branch", "", tr("git branch for the bundle (default \"sshkeeper\")", "ветка git для пакета (по умолчанию «sshkeeper»)"))
	syncSetupCmd.Flags().BoolVar(&syncSetupNoAuto, "no-auto", false, tr("do not sync automatically from the TUI", "не синхронизировать автоматически из TUI"))
	syncAddDeviceCmd.Flags().BoolVar(&syncAddDeviceOffline, "offline", false, tr("create a one-time 24-hour code for dual-boot or an offline device", "создать одноразовый код на 24 часа для dual-boot или выключенного устройства"))
	syncCmd.AddCommand(syncSetupCmd, syncInitCmd, syncAddDeviceCmd, syncJoinCmd, syncStatusCmd, syncRecoveryKeyCmd, syncLeaveCmd)
	// Sync errors are explained in words and printed once by Execute; the
	// usage block would bury them.
	for _, command := range append(syncCmd.Commands(), syncCmd) {
		command.SilenceUsage = true
		command.SilenceErrors = true
	}
	rootCmd.AddCommand(syncCmd)
}

// localizeSyncHelp refreshes the sync command descriptions after the saved
// language is loaded, like localizeCLIHelp does for the other commands.
func localizeSyncHelp() {
	short := map[*cobra.Command][2]string{
		syncCmd:            {"Sync profiles, secrets, and keys between devices", "Синхронизация профилей, секретов и ключей между устройствами"},
		syncSetupCmd:       {"Choose where the encrypted bundle is stored", "Выбрать, где хранится зашифрованный пакет"},
		syncInitCmd:        {"Create a sync space from this device", "Создать пространство синхронизации с этого устройства"},
		syncAddDeviceCmd:   {"Create a code to add another device", "Создать код для добавления другого устройства"},
		syncJoinCmd:        {"Join with a pairing, offline, or recovery code", "Подключиться по обычному, офлайн-коду или ключу восстановления"},
		syncStatusCmd:      {"Show sync settings and the last result", "Показать настройки и последний результат синхронизации"},
		syncRecoveryKeyCmd: {"Print the recovery key of this sync space", "Показать ключ восстановления пространства синхронизации"},
		syncLeaveCmd:       {"Stop syncing this device; local data stays", "Отключить устройство от синхронизации; данные остаются"},
	}
	for command, pair := range short {
		command.Short = tr(pair[0], pair[1])
	}
	syncCmd.Long = tr(`Keep profiles, forwards, templates, vault secrets, and the private keys that
profiles reference in step across devices through an end-to-end encrypted
bundle in a folder or a git repository.

  sshkeeper sync setup folder ~/Sync/sshkeeper   (or: setup git <url>)
  sshkeeper sync init                            first device: create the space
  sshkeeper sync add-device                      show a six-digit code
  sshkeeper sync add-device --offline            24-hour code for dual-boot/offline device
  sshkeeper sync join                            new device: enter a code
  sshkeeper sync                                 sync now`,
		`Синхронизирует профили, пробросы, шаблоны, секреты хранилища и закрытые
ключи, на которые ссылаются профили, через пакет со сквозным шифрованием в
папке или git-репозитории.

  sshkeeper sync setup folder ~/Sync/sshkeeper   (или: setup git <url>)
  sshkeeper sync init                            первое устройство: создать пространство
  sshkeeper sync add-device                      показать шестизначный код
  sshkeeper sync add-device --offline            код на 24 часа для dual-boot/выключенного устройства
  sshkeeper sync join                            новое устройство: ввести код
  sshkeeper sync                                 синхронизировать сейчас`)
	if flag := syncSetupCmd.Flags().Lookup("branch"); flag != nil {
		flag.Usage = tr("git branch for the bundle (default \"sshkeeper\")", "ветка git для пакета (по умолчанию «sshkeeper»)")
	}
	if flag := syncSetupCmd.Flags().Lookup("no-auto"); flag != nil {
		flag.Usage = tr("do not sync automatically from the TUI", "не синхронизировать автоматически из TUI")
	}
	if flag := syncAddDeviceCmd.Flags().Lookup("offline"); flag != nil {
		flag.Usage = tr("create a one-time 24-hour code for dual-boot or an offline device", "создать одноразовый код на 24 часа для dual-boot или выключенного устройства")
	}
}

// wireTUISync connects the TUI's sync screen and automatic sync to the
// engine. Errors reach the TUI already explained in words.
func wireTUISync() {
	explain := func(err error) error {
		if err == nil {
			return nil
		}
		return errors.New(syncErrorText(err))
	}
	result := func(report syncer.Report) tui.SyncResult {
		return tui.SyncResult{Received: report.Received, Sent: report.Sent, Warnings: report.Warnings}
	}
	tui.GetSyncInfo = func() tui.SyncInfo {
		info := tui.SyncInfo{Settings: tui.SyncSettings{
			Mode: cfg.Sync.Mode, Folder: cfg.Sync.Folder, GitURL: cfg.Sync.GitURL, GitBranch: cfg.Sync.GitBranch, Auto: cfg.Sync.Auto,
		}}
		home, _ := os.UserHomeDir()
		engine := &syncer.Engine{DB: appDB, Vault: getOrCreateVault(), Home: home}
		status := engine.Status()
		info.Joined, info.LastSync, info.LastError, info.Records = status.Joined, status.LastSync, status.LastError, status.Records
		return info
	}
	tui.SaveSyncSettings = func(settings tui.SyncSettings) error {
		return cfg.SetSync(config.SyncConfig{Mode: settings.Mode, Folder: settings.Folder, GitURL: settings.GitURL, GitBranch: settings.GitBranch, Auto: settings.Auto})
	}
	tui.RunSync = func() (tui.SyncResult, error) {
		engine, err := newSyncEngine()
		if err != nil {
			return tui.SyncResult{}, err
		}
		report, err := engine.Sync()
		return result(report), explain(err)
	}
	tui.CreateSyncSpace = func() (tui.SyncResult, error) {
		engine, err := newSyncEngine()
		if err != nil {
			return tui.SyncResult{}, err
		}
		recovery, report, err := engine.Create()
		out := result(report)
		out.RecoveryKey = recovery
		return out, explain(err)
	}
	tui.PairSyncDevice = func(password string) (string, time.Time, error) {
		if err := verifyMasterPassword(password); err != nil {
			return "", time.Time{}, err
		}
		engine, err := newSyncEngine()
		if err != nil {
			return "", time.Time{}, err
		}
		code, expires, err := engine.Pair(password)
		return code, expires, explain(err)
	}
	tui.PairSyncDeviceOffline = func() (string, time.Time, error) {
		engine, err := newSyncEngine()
		if err != nil {
			return "", time.Time{}, err
		}
		code, expires, err := engine.PairOffline()
		return code, expires, explain(err)
	}
	tui.JoinSyncSpace = func(secret, password string) (tui.SyncResult, error) {
		engine, err := newSyncEngine()
		if err != nil {
			return tui.SyncResult{}, err
		}
		report, err := engine.Join(secret, password)
		return result(report), explain(err)
	}
	tui.SyncRecoveryKey = func() (string, error) {
		engine, err := newSyncEngine()
		if err != nil {
			return "", err
		}
		key, err := engine.RecoveryKey()
		return key, explain(err)
	}
	tui.LeaveSync = func() error {
		home, _ := os.UserHomeDir()
		engine := &syncer.Engine{DB: appDB, Vault: getOrCreateVault(), Home: home}
		if err := engine.Leave(); err != nil {
			return explain(err)
		}
		settings := cfg.Sync
		settings.Mode = config.SyncOff
		return cfg.SetSync(settings)
	}
}
