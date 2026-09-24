package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
	sessionpkg "github.com/mirivlad/sshkeeper/internal/session"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/mirivlad/sshkeeper/internal/tui"
	tunnelpkg "github.com/mirivlad/sshkeeper/internal/tunnel"
)

func runTUI() error {
	tui.GetLanguagePreference = func() string { return cfg.UI.Language }
	tui.SetLanguagePreference = func(value string) error {
		if err := cfg.SetLanguage(value); err != nil {
			return err
		}
		return i18n.SetPreference(value)
	}
	servers, err := appDB.ListServers()
	if err != nil {
		return fmt.Errorf("%s: %w", tr("load servers", "загрузить серверы"), err)
	}

	tui.ListServers = func() ([]*model.Server, error) {
		return appDB.ListServers()
	}
	tui.SearchServers = func(query string) ([]*model.Server, error) {
		return appDB.SearchServers(query)
	}
	tui.DeleteServer = func(alias string) error {
		server, err := appDB.GetServer(alias)
		if err != nil {
			return err
		}
		if err := appDB.DeleteServer(alias); err != nil {
			return err
		}
		v := getOrCreateVault()
		if v.IsUnlocked() {
			cleanupServerSecretsForServer(v, server)
			if err := v.Save(); err != nil {
				// The vault file is unchanged on save failure; restore the DB profile.
				_ = appDB.CreateServer(server)
				_ = appDB.SetServerTags(server.ID, server.Tags)
				return fmt.Errorf("%s: %w", tr("save vault after cleanup", "сохранить хранилище после очистки"), err)
			}
		}
		return nil
	}
	tui.TestConnection = func(server *model.Server) (bool, string) {
		return ssh.TestResolved(cfg, server, dbProfileResolver, serverVaultFunc(server))
	}
	tui.TestConnectionWithPassword = func(server *model.Server, password string) (bool, string) {
		base := serverVaultFunc(server)
		return ssh.TestResolved(cfg, server, dbProfileResolver, formTestVaultFunc(base, server, password))
	}
	tui.SaveServer = func(server *model.Server, password string, oldAlias string) error {
		lookupAlias := server.Alias
		if oldAlias != "" {
			lookupAlias = oldAlias
		}
		existing, _ := appDB.GetServer(lookupAlias)
		var original *model.Server
		if existing != nil {
			original = cloneServer(existing)
			server.ID = existing.ID
			if err := appDB.UpdateServerByAlias(existing.Alias, server); err != nil {
				return err
			}
		} else {
			if err := appDB.CreateServer(server); err != nil {
				return err
			}
		}
		if err := appDB.SetServerTags(server.ID, server.Tags); err != nil {
			if original != nil {
				_ = appDB.UpdateServerByAlias(server.Alias, original)
				_ = appDB.SetServerTags(original.ID, original.Tags)
			} else {
				_ = appDB.DeleteServer(server.Alias)
			}
			return err
		}

		v := getOrCreateVault()
		if !v.IsUnlocked() {
			return nil
		}
		if err := syncServerSecrets(v, oldAlias, server, password); err != nil {
			rollbackSavedServer(server, original)
			return fmt.Errorf("%s: %w", tr("sync vault secrets", "синхронизировать секреты хранилища"), err)
		}
		if err := v.Save(); err != nil {
			rollbackSavedServer(server, original)
			return fmt.Errorf("%s: %w", tr("save vault", "сохранить хранилище"), err)
		}
		return nil
	}

	tui.ListIdentityFiles = listIdentityFiles
	tui.GetGroups = func() ([]string, error) {
		return appDB.GetGroups()
	}
	tui.ListGroups = func() ([]*model.Group, error) {
		return appDB.ListGroups()
	}
	tui.CreateGroup = func(name string) error {
		return appDB.CreateGroup(name)
	}
	tui.ResolveRouteAlias = func(alias string) (int64, bool) {
		return appDB.ResolveAlias(alias)
	}
	tui.RenameGroup = func(oldName, newName string) error {
		return appDB.RenameGroup(oldName, newName)
	}
	tui.DeleteGroup = func(name string) error {
		return appDB.DeleteGroup(name)
	}
	tui.ListTags = func() ([]string, error) {
		return appDB.ListTags()
	}
	tui.RenameTag = func(oldName, newName string) error {
		return appDB.RenameTag(oldName, newName)
	}
	tui.DeleteTag = func(name string) error {
		return appDB.DeleteTag(name)
	}
	tui.SetServerTags = func(server *model.Server, tags []string) error {
		server.Tags = tags
		return appDB.SetServerTags(server.ID, tags)
	}
	tui.ListCommandTemplates = func() ([]*model.CommandTemplate, error) {
		return appDB.ListCommandTemplates()
	}
	tui.SaveCommandTemplate = func(oldName string, template *model.CommandTemplate) error {
		if oldName == "" {
			return appDB.CreateCommandTemplate(template)
		}
		return appDB.UpdateCommandTemplate(oldName, template)
	}
	tui.DeleteCommandTemplate = func(name string) error {
		return appDB.DeleteCommandTemplate(name)
	}
	tui.RunTemplateBackground = func(server *model.Server, command string) (string, error) {
		fresh, err := appDB.GetServer(server.Alias)
		if err != nil {
			return "", err
		}
		return ssh.RunCommandOutputResolved(cfg, fresh, dbProfileResolver, serverVaultFunc(fresh), command)
	}
	tui.ListForwards = func(serverID int64) ([]*model.Forward, error) {
		return appDB.GetForwards(serverID)
	}
	tui.SaveForward = func(fwd *model.Forward) error {
		_, err := appDB.AddForward(fwd)
		return err
	}
	tui.UpdateForward = func(fwd *model.Forward) error {
		return appDB.UpdateForward(fwd)
	}
	tui.DeleteForward = func(forwardID int64) error {
		return appDB.DeleteForward(forwardID)
	}
	tui.StartBackgroundTunnel = func(alias string) (*model.TunnelState, error) {
		server, err := appDB.GetServer(alias)
		if err != nil {
			return nil, fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}
		forwards, err := appDB.GetForwards(server.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tr("load forwards", "загрузить перенаправления"), err)
		}
		if err := validateBackgroundTunnel(server, forwards); err != nil {
			return nil, err
		}
		return tunnelpkg.StartResolved(cfg, server, forwards, true, dbProfileResolver)
	}
	tui.ImportServers = func() (int, error) {
		return importServersFromSSHConfig(nil)
	}
	tui.LockVault = func() error {
		v := getOrCreateVault()
		v.Lock()
		return nil
	}
	tui.VaultUnlocked = func() bool {
		return getOrCreateVault().IsUnlocked()
	}
	tui.UpdateTestResult = func(alias string, status model.TestStatus, testErr string) error {
		return appDB.UpdateTestResult(alias, status, testErr)
	}
	tui.HasSecret = func(alias string, secretType string) bool {
		v := getOrCreateVault()
		if !v.IsUnlocked() {
			return false
		}
		server, err := appDB.GetServer(alias)
		if err != nil {
			return false
		}
		return hasServerSecret(v, server, secretType)
	}

	// Run TUI in a loop — if user requests connect, handle it and restart TUI
	for {
		m := tui.New(servers)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("%s: %w", tr("TUI error", "ошибка интерфейса"), err)
		}

		// Check if TUI requested a connect action
		result := m.Result()
		if result != nil && result.Action == "session_open" && result.Server != nil {
			fresh, err := appDB.GetServer(result.Server.Alias)
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Server not found: %s", "Сервер не найден: %s", result.Server.Alias))
			} else {
				windowID, _, openErr := sessionpkg.Open(fresh.Alias)
				if openErr != nil {
					fmt.Fprintln(os.Stderr, trf("Open session: %v", "Открыть сеанс: %v", openErr))
				} else if attachErr := sessionpkg.Attach(windowID); attachErr != nil {
					fmt.Fprintln(os.Stderr, trf("Attach session: %v", "Подключиться к сеансу: %v", attachErr))
				}
			}
			servers, _ = appDB.ListServers()
			continue
		}
		if result != nil && result.Action == "session_attach" && result.SessionID != "" {
			if err := sessionpkg.Attach(result.SessionID); err != nil {
				fmt.Fprintln(os.Stderr, trf("Attach session: %v", "Подключиться к сеансу: %v", err))
			}
			servers, _ = appDB.ListServers()
			continue
		}
		if result != nil && result.Action == "connect" && result.Server != nil {
			// TUI has exited, terminal is restored by tea.WithAltScreen.
			// Now connect.
			server := result.Server

			// Re-fetch fresh server data from DB
			fresh, err := appDB.GetServer(server.Alias)
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Server not found: %s", "Сервер не найден: %s", server.Alias))
				servers, _ = appDB.ListServers()
				continue
			}

			fmt.Printf(tr("Connecting to %s@%s:%d...\n", "Подключение к %s@%s:%d...\n"), fresh.User, fresh.Host, fresh.Port)

			if err := ssh.ConnectResolved(cfg, fresh, dbProfileResolver, serverVaultFunc(fresh)); err != nil {
				fmt.Fprintln(os.Stderr, trf("Connection error: %v", "Ошибка подключения: %v", err))
			} else {
				fmt.Println(tr("Connection closed.", "Подключение закрыто."))
			}

			appDB.UpdateLastConnected(server.Alias)

			// Wait for user to press Enter before returning to TUI
			fmt.Println(tr("\n[Press Enter to return to sshkeeper]", "\n[Нажмите Enter, чтобы вернуться в sshkeeper]"))
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			// Reload servers for TUI
			servers, _ = appDB.ListServers()
			continue
		}
		if result != nil && result.Action == "run_template_foreground" && len(result.Servers) > 0 {
			for _, server := range result.Servers {
				fresh, err := appDB.GetServer(server.Alias)
				if err != nil {
					fmt.Fprintln(os.Stderr, trf("Server not found: %s", "Сервер не найден: %s", server.Alias))
					continue
				}
				fmt.Printf(tr("Running template %q on %s...\n", "Выполнение шаблона %q на %s...\n"), result.TemplateName, fresh.Alias)
				if err := ssh.RunCommandResolved(cfg, fresh, dbProfileResolver, serverVaultFunc(fresh), result.Command); err != nil {
					fmt.Fprintln(os.Stderr, trf("Command error on %s: %v", "Ошибка команды на %s: %v", fresh.Alias, err))
				}
				appDB.UpdateLastConnected(fresh.Alias)
			}

			fmt.Println(tr("\n[Press Enter to return to sshkeeper]", "\n[Нажмите Enter, чтобы вернуться в sshkeeper]"))
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		if result != nil && result.Action == "export" {
			servers, err := appDB.ListServers()
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Export error: %v", "Ошибка экспорта: %v", err))
			} else {
				fmt.Print(formatServersExport(servers))
			}

			fmt.Println(tr("\n[Press Enter to return to sshkeeper]", "\n[Нажмите Enter, чтобы вернуться в sshkeeper]"))
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		if result != nil && result.Action == "vault_change_pw" {
			if err := changeVaultPasswordInteractive(); err != nil {
				fmt.Fprintln(os.Stderr, trf("Vault password change error: %v", "Ошибка смены пароля хранилища: %v", err))
			}

			fmt.Println(tr("\n[Press Enter to return to sshkeeper]", "\n[Нажмите Enter, чтобы вернуться в sshkeeper]"))
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		if result != nil && (result.Action == "tunnel" || result.Action == "tunnel_n") && result.Server != nil {
			server := result.Server
			fresh, err := appDB.GetServer(server.Alias)
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Server not found: %s", "Сервер не найден: %s", server.Alias))
				waitForTUIReturn()
				servers, _ = appDB.ListServers()
				continue
			}

			forwards, err := appDB.GetForwards(fresh.ID)
			if err != nil {
				fmt.Fprintln(os.Stderr, trf("Load forwards: %v", "Загрузить перенаправления: %v", err))
				waitForTUIReturn()
				servers, _ = appDB.ListServers()
				continue
			}

			forwardOnly := result.Action == "tunnel_n"
			active := enabledForwardCount(forwards)
			if active == 0 {
				fmt.Fprintln(os.Stderr, trf("No enabled port forwards for %s. Add or enable a rule before starting a tunnel.", "Для %s нет включённых перенаправлений. Добавьте или включите правило перед запуском туннеля.", fresh.Alias))
				waitForTUIReturn()
				servers, _ = appDB.ListServers()
				continue
			}

			fmt.Printf(tr("Starting tunnel to %s with %d enabled forward(s)...\n", "Запуск туннеля к %s с %d включёнными перенаправлениями...\n"), fresh.Alias, active)
			if forwardOnly {
				fmt.Println(tr("Tunnel mode (ssh -N). Press Ctrl+C to stop and return to sshkeeper.", "Режим туннеля (ssh -N). Нажмите Ctrl+C, чтобы остановить его и вернуться в sshkeeper."))
			}

			if err := ssh.ConnectWithForwardsResolved(cfg, fresh, forwards, forwardOnly, dbProfileResolver, serverVaultFunc(fresh)); err != nil {
				fmt.Fprintln(os.Stderr, trf("Tunnel error: %v", "Ошибка туннеля: %v", err))
			} else {
				fmt.Println(tr("Tunnel closed.", "Туннель закрыт."))
			}
			appDB.UpdateLastConnected(fresh.Alias)

			waitForTUIReturn()

			servers, _ = appDB.ListServers()
			continue
		}

		// Normal quit (q or Esc)
		return nil
	}
}

func waitForTUIReturn() {
	fmt.Println(tr("\n[Press Enter to return to sshkeeper]", "\n[Нажмите Enter, чтобы вернуться в sshkeeper]"))
	buf := make([]byte, 1)
	_, _ = os.Stdin.Read(buf)
}
