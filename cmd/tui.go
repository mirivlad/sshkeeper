package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/mirivlad/sshkeeper/internal/tui"
	tunnelpkg "github.com/mirivlad/sshkeeper/internal/tunnel"
)

func runTUI() error {
	servers, err := appDB.ListServers()
	if err != nil {
		return fmt.Errorf("load servers: %w", err)
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
				return fmt.Errorf("save vault after cleanup: %w", err)
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
			return fmt.Errorf("sync vault secrets: %w", err)
		}
		if err := v.Save(); err != nil {
			rollbackSavedServer(server, original)
			return fmt.Errorf("save vault: %w", err)
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

			if err := ssh.ConnectResolved(cfg, fresh, dbProfileResolver, serverVaultFunc(fresh)); err != nil {
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
				if err := ssh.RunCommandResolved(cfg, fresh, dbProfileResolver, serverVaultFunc(fresh), result.Command); err != nil {
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

		if result != nil && result.Action == "export" {
			servers, err := appDB.ListServers()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Export error: %v\n", err)
			} else {
				fmt.Print(formatServersExport(servers))
			}

			fmt.Println("\n[Press Enter to return to sshkeeper]")
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		if result != nil && result.Action == "vault_change_pw" {
			if err := changeVaultPasswordInteractive(); err != nil {
				fmt.Fprintf(os.Stderr, "Vault password change error: %v\n", err)
			}

			fmt.Println("\n[Press Enter to return to sshkeeper]")
			buf := make([]byte, 1)
			os.Stdin.Read(buf)

			servers, _ = appDB.ListServers()
			continue
		}

		if result != nil && (result.Action == "tunnel" || result.Action == "tunnel_n" || result.Action == "tunnel_bg") && result.Server != nil {
			server := result.Server
			fresh, err := appDB.GetServer(server.Alias)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Server not found: %s\n", server.Alias)
				servers, _ = appDB.ListServers()
				continue
			}

			forwards, err := appDB.GetForwards(fresh.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Load forwards: %v\n", err)
				servers, _ = appDB.ListServers()
				continue
			}

			forwardOnly := result.Action == "tunnel_n" || result.Action == "tunnel_bg"
			background := result.Action == "tunnel_bg"

			if background {
				// Start detached tunnel process
				if err := validateBackgroundTunnel(fresh, forwards); err != nil {
					fmt.Fprintf(os.Stderr, "Start tunnel: %v\n", err)
					servers, _ = appDB.ListServers()
					continue
				}
				state, err := tunnelpkg.StartResolved(cfg, fresh, forwards, forwardOnly, dbProfileResolver)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Start tunnel: %v\n", err)
				} else {
					fmt.Printf("✓ Tunnel started [%d] PID %d → %s\n", state.ID, state.PID, fresh.Alias)
				}
				servers, _ = appDB.ListServers()
				continue
			}

			if len(forwards) > 0 {
				fmt.Printf("Starting tunnel to %s with %d forward(s)...\n", fresh.Alias, len(forwards))
			} else {
				fmt.Printf("Starting session to %s...\n", fresh.Alias)
			}

			if err := ssh.ConnectWithForwardsResolved(cfg, fresh, forwards, forwardOnly, dbProfileResolver, serverVaultFunc(fresh)); err != nil {
				fmt.Fprintf(os.Stderr, "Tunnel error: %v\n", err)
			} else {
				fmt.Println("Tunnel closed.")
			}
			appDB.UpdateLastConnected(fresh.Alias)

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
