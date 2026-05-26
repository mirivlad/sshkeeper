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
		return appDB.DeleteServer(alias)
	}
	tui.TestConnection = func(server *model.Server) (bool, string) {
		return ssh.Test(cfg, server, vaultFunc)
	}
	tui.SaveServer = func(server *model.Server, password string) error {
		if password != "" {
			v := getOrCreateVault()
			vaultKey := fmt.Sprintf("server:%s:ssh_password", server.Alias)
			secretType := "ssh_password"
			if server.AuthMethod == model.AuthKeyPassphrase {
				vaultKey = fmt.Sprintf("server:%s:key_passphrase", server.Alias)
				secretType = "key_passphrase"
			}
			if err := v.Put(vaultKey, secretType, []byte(password)); err != nil {
				return fmt.Errorf("store secret: %w", err)
			}
			if err := v.Save(); err != nil {
				return fmt.Errorf("save vault: %w", err)
			}
		}

		existing, _ := appDB.GetServer(server.Alias)
		if existing != nil {
			server.ID = existing.ID
			return appDB.UpdateServer(server)
		}
		return appDB.CreateServer(server)
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

		// Normal quit (q or Esc)
		return nil
	}
}


