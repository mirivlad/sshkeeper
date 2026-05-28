package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/mirivlad/sshkeeper/internal/tui"
)

func runTUI() error {
	servers, err := appDB.ListServers()
	if err != nil {
		return fmt.Errorf("load servers: %w", err)
	}

	vaultFunc := func(sa string, st string) (string, error) {
		v := getOrCreateVault()
		if !v.IsUnlocked() {
			return "", fmt.Errorf("vault is locked")
		}
		key := fmt.Sprintf("server:%s:%s", sa, st)
		data, err := v.Get(key)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	tui.ListServers = func() ([]*model.Server, error) {
		return appDB.ListServers()
	}
	tui.SearchServers = func(query string) ([]*model.Server, error) {
		return appDB.SearchServers(query)
	}
	tui.DeleteServer = func(alias string) error {
		if err := appDB.DeleteServer(alias); err != nil {
			return err
		}
		v := getOrCreateVault()
		if v.IsUnlocked() {
			cleanupServerSecrets(v, alias)
			if err := v.Save(); err != nil {
				return fmt.Errorf("save vault after cleanup: %w", err)
			}
		}
		return nil
	}
	tui.TestConnection = func(server *model.Server) (bool, string) {
		return ssh.Test(cfg, server, vaultFunc)
	}
	tui.TestConnectionWithPassword = func(server *model.Server, password string) (bool, string) {
		return ssh.Test(cfg, server, formTestVaultFunc(vaultFunc, server, password))
	}
	tui.SaveServer = func(server *model.Server, password string, oldAlias string) error {
		v := getOrCreateVault()
		if v.IsUnlocked() {
			if err := syncServerSecrets(v, oldAlias, server, password); err != nil {
				return fmt.Errorf("sync vault secrets: %w", err)
			}
			if err := v.Save(); err != nil {
				return fmt.Errorf("save vault: %w", err)
			}
		}

		lookupAlias := server.Alias
		if oldAlias != "" {
			lookupAlias = oldAlias
		}
		existing, _ := appDB.GetServer(lookupAlias)
		if existing != nil {
			server.ID = existing.ID
			if err := appDB.UpdateServerByAlias(existing.Alias, server); err != nil {
				return err
			}
			return appDB.SetServerTags(existing.ID, server.Tags)
		}
		if err := appDB.CreateServer(server); err != nil {
			return err
		}
		return appDB.SetServerTags(server.ID, server.Tags)
	}

	tui.GetGroups = func() ([]string, error) {
		return appDB.GetGroups()
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
		return ssh.RunCommandOutput(cfg, fresh, vaultFunc, command)
	}
	tui.UpdateTestResult = func(alias string, status model.TestStatus, testErr string) error {
		return appDB.UpdateTestResult(alias, status, testErr)
	}
	tui.HasSecret = func(alias string, secretType string) bool {
		v := getOrCreateVault()
		if !v.IsUnlocked() {
			return false
		}
		return v.HasSecret(serverSecretID(alias, secretType))
	}

	// Run TUI in a loop — if user requests connect, handle it and restart TUI
	for {
		m := tui.New(servers)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		// Check if TUI requested a connect action
		result := m.Result()
		if result != nil && result.Action == "connect" && result.Server != nil {
			// TUI has exited, terminal is restored by tea.WithAltScreen.
			// Now connect.
			server := result.Server

			// Re-fetch fresh server data from DB
			fresh, err := appDB.GetServer(server.Alias)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Server not found: %s\n", server.Alias)
				servers, _ = appDB.ListServers()
				continue
			}

			fmt.Printf("Connecting to %s@%s:%d...\n", fresh.User, fresh.Host, fresh.Port)

			if err := ssh.Connect(cfg, fresh, vaultFunc); err != nil {
				fmt.Fprintf(os.Stderr, "Connection error: %v\n", err)
			} else {
				fmt.Println("Connection closed.")
			}

			appDB.UpdateLastConnected(server.Alias)

			// Wait for user to press Enter before returning to TUI
			fmt.Println("\n[Press Enter to return to sshkeeper]")
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
					fmt.Fprintf(os.Stderr, "Server not found: %s\n", server.Alias)
					continue
				}
				fmt.Printf("Running template %q on %s...\n", result.TemplateName, fresh.Alias)
				if err := ssh.RunCommand(cfg, fresh, vaultFunc, result.Command); err != nil {
					fmt.Fprintf(os.Stderr, "Command error on %s: %v\n", fresh.Alias, err)
				}
				appDB.UpdateLastConnected(fresh.Alias)
			}

			fmt.Println("\n[Press Enter to return to sshkeeper]")
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		// Normal quit (q or Esc)
		return nil
	}
}
