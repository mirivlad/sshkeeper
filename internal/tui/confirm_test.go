package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestServerDeleteRequiresExplicitConfirmation(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "prod", Host: "prod.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	deleted := 0
	oldDelete, oldList := DeleteServer, ListServers
	t.Cleanup(func() { DeleteServer, ListServers = oldDelete, oldList })
	DeleteServer = func(alias string) error {
		deleted++
		return nil
	}
	ListServers = func() ([]*model.Server, error) { return nil, nil }

	updated, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(*tuiModel)
	if cmd != nil || deleted != 0 {
		t.Fatalf("delete ran before confirmation: cmd=%v deleted=%d", cmd != nil, deleted)
	}
	if m.screen != screenConfirm || m.confirm == nil {
		t.Fatalf("expected confirmation screen, got screen=%v confirm=%v", m.screen, m.confirm)
	}
	if m.confirm.focus != confirmCancel {
		t.Fatalf("default focus = %v, want Cancel", m.confirm.focus)
	}
	view := m.View()
	for _, want := range []string{"prod", "saved port forwards", "vault secrets", "> [ Cancel ]", "[ Delete ]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}

	updated, cmd = m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd != nil || deleted != 0 || m.screen != screenList {
		t.Fatalf("Enter on default Cancel must cancel: screen=%v cmd=%v deleted=%d", m.screen, cmd != nil, deleted)
	}
}

func TestServerDeleteRunsOnceAndReturnsToList(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "prod", Host: "prod.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	deleted := 0
	oldDelete, oldList := DeleteServer, ListServers
	t.Cleanup(func() { DeleteServer, ListServers = oldDelete, oldList })
	DeleteServer = func(alias string) error {
		deleted++
		return nil
	}
	ListServers = func() ([]*model.Server, error) { return nil, nil }

	updated, _ := m.updateList(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(*tuiModel)
	updated, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*tuiModel)
	updated, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd == nil || m.confirm == nil || !m.confirm.pending {
		t.Fatal("expected pending destructive command")
	}
	_, duplicate := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("repeated Enter must not start a duplicate delete")
	}
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(*tuiModel)
	if deleted != 1 {
		t.Fatalf("delete calls = %d, want 1", deleted)
	}
	if m.screen != screenList || m.confirm != nil || len(m.servers) != 0 {
		t.Fatalf("unexpected completion state: screen=%v confirm=%v servers=%d", m.screen, m.confirm, len(m.servers))
	}
}

func TestServerDeletePartialSuccessRemovesLocalRow(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "prod", Host: "prod.example", Port: 22}
	m := New([]*model.Server{server})
	oldDelete, oldList := DeleteServer, ListServers
	t.Cleanup(func() { DeleteServer, ListServers = oldDelete, oldList })
	DeleteServer = func(string) error { return nil }
	ListServers = func() ([]*model.Server, error) { return nil, errors.New("reload unavailable") }

	m.confirmServerDelete(server)
	m.confirm.focus = confirmAccept
	updated, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	updated, _ = m.Update(cmd())
	m = updated.(*tuiModel)
	if len(m.servers) != 0 || len(m.list.Items()) != 0 {
		t.Fatalf("deleted server remained visible: servers=%d items=%d", len(m.servers), len(m.list.Items()))
	}
	if m.err != nil || !strings.Contains(m.success, "refresh failed") {
		t.Fatalf("partial success was misreported: err=%v success=%q", m.err, m.success)
	}
}

func TestTagAndTemplateDeletePartialSuccessRemovesLocalRow(t *testing.T) {
	t.Run("tag", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTags
		m.setTags([]string{"prod"})
		oldDelete, oldList := DeleteTag, ListTags
		t.Cleanup(func() { DeleteTag, ListTags = oldDelete, oldList })
		DeleteTag = func(string) error { return nil }
		ListTags = func() ([]string, error) { return nil, errors.New("reload unavailable") }

		updated, _ := m.updateTags(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = updated.(*tuiModel)
		m.confirm.focus = confirmAccept
		updated, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(*tuiModel)
		updated, _ = m.Update(cmd())
		m = updated.(*tuiModel)
		if len(m.tags) != 0 || !strings.Contains(m.success, "refresh failed") {
			t.Fatalf("tag partial success: tags=%v success=%q err=%v", m.tags, m.success, m.err)
		}
	})

	t.Run("template", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTemplates
		m.setTemplates([]*model.CommandTemplate{{Name: "uptime", Command: "uptime"}})
		oldDelete, oldList := DeleteCommandTemplate, ListCommandTemplates
		t.Cleanup(func() { DeleteCommandTemplate, ListCommandTemplates = oldDelete, oldList })
		DeleteCommandTemplate = func(string) error { return nil }
		ListCommandTemplates = func() ([]*model.CommandTemplate, error) { return nil, errors.New("reload unavailable") }

		updated, _ := m.updateTemplates(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = updated.(*tuiModel)
		m.confirm.focus = confirmAccept
		updated, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(*tuiModel)
		updated, _ = m.Update(cmd())
		m = updated.(*tuiModel)
		if len(m.templates) != 0 || !strings.Contains(m.success, "refresh failed") {
			t.Fatalf("template partial success: templates=%v success=%q err=%v", m.templates, m.success, m.err)
		}
	})
}

func TestForwardDeleteReturnsToForwardListAndRetainsError(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "prod"}
	fwd := &model.Forward{ID: 7, ServerID: 1, Name: "postgres", Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "db", RemotePort: 5432, Enabled: true}
	m := New([]*model.Server{server})
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(server.ID, server.Alias, 80, 24)
	m.forwardScreen.list = []*model.Forward{fwd}
	m.forwardScreen.selected = 0

	oldDelete := DeleteForward
	t.Cleanup(func() { DeleteForward = oldDelete })
	DeleteForward = func(id int64) error { return errors.New("database is read-only") }

	updated, _ := m.updateForwardList(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(*tuiModel)
	if m.confirm == nil || m.confirm.parent != screenForwardList {
		t.Fatalf("expected forward-list parent, confirm=%#v", m.confirm)
	}
	updated, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*tuiModel)
	updated, cmd := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(*tuiModel)
	if m.screen != screenForwardList || m.confirm != nil {
		t.Fatalf("expected return to forwards, screen=%v confirm=%v", m.screen, m.confirm)
	}
	if m.forwardScreen == nil || m.forwardScreen.err == nil || !strings.Contains(m.forwardScreen.View(), "database is read-only") {
		t.Fatalf("forward error not retained: model=%#v view=%q", m.forwardScreen, m.forwardScreen.View())
	}
}

func TestOtherDestructiveActionsOpenConfirmation(t *testing.T) {
	t.Run("tag", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTags
		m.setTags([]string{"prod"})
		updated, cmd := m.updateTags(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = updated.(*tuiModel)
		if cmd != nil || m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenTags {
			t.Fatalf("tag deletion did not open confirmation: screen=%v confirm=%#v cmd=%v", m.screen, m.confirm, cmd != nil)
		}
	})

	t.Run("template", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTemplates
		m.setTemplates([]*model.CommandTemplate{{Name: "uptime", Command: "uptime"}})
		updated, cmd := m.updateTemplates(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = updated.(*tuiModel)
		if cmd != nil || m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenTemplates {
			t.Fatalf("template deletion did not open confirmation: screen=%v confirm=%#v cmd=%v", m.screen, m.confirm, cmd != nil)
		}
	})

	t.Run("tunnel", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTunnelManager
		m.tunnelScreen = newTunnelScreenModel(80, 24)
		m.tunnelScreen.tunnels = []*model.TunnelState{{ID: 11, Name: "prod tunnel", ServerAlias: "prod"}}
		m.tunnelScreen.rebuildList()
		updated, cmd := m.updateTunnelManager(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = updated.(*tuiModel)
		if cmd != nil || m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenTunnelManager {
			t.Fatalf("tunnel stop did not open confirmation: screen=%v confirm=%#v cmd=%v", m.screen, m.confirm, cmd != nil)
		}
	})
}
