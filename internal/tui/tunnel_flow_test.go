package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func tunnelTestModel(auth model.AuthMethod) *tuiModel {
	server := &model.Server{ID: 1, Alias: "demo", Host: "example.org", User: "demo", AuthMethod: auth}
	m := New([]*model.Server{server})
	m.width, m.height = 80, 24
	return m
}

func selectTunnelAction(menu *actionMenuModel, action string) {
	for i, raw := range menu.list.Items() {
		if raw.(actionMenuItem).action == action {
			menu.list.Select(i)
			return
		}
	}
}

func TestTunnelModesCheckEnabledRulesBeforeLeavingTUI(t *testing.T) {
	previous := ListForwards
	t.Cleanup(func() { ListForwards = previous })
	ListForwards = func(int64) ([]*model.Forward, error) {
		return []*model.Forward{{Enabled: false}}, nil
	}
	for _, action := range []string{"tunnel", "tunnel_n", "tunnel_bg"} {
		t.Run(action, func(t *testing.T) {
			m := tunnelTestModel(model.AuthAgent)
			updated, load := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
			m = updated.(*tuiModel)
			if m.screen != screenActionMenu || load == nil {
				t.Fatal("server actions did not load their forward availability")
			}
			updated, _ = m.Update(load())
			m = updated.(*tuiModel)
			selectTunnelAction(m.actionMenu, action)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(*tuiModel)
			if cmd != nil || m.screen != screenActionMenu || m.Result() != nil {
				t.Fatalf("unavailable action left TUI: screen=%v result=%#v", m.screen, m.Result())
			}
			if !strings.Contains(m.View(), "No enabled port forwards") {
				t.Fatalf("missing actionable explanation:\n%s", m.View())
			}
		})
	}
}

func TestBackgroundTunnelModeExplainsAuthRestriction(t *testing.T) {
	previous := ListForwards
	t.Cleanup(func() { ListForwards = previous })
	ListForwards = func(int64) ([]*model.Forward, error) {
		return []*model.Forward{{Enabled: true}}, nil
	}
	m := tunnelTestModel(model.AuthPassword)
	updated, load := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(*tuiModel)
	updated, _ = m.Update(load())
	m = updated.(*tuiModel)
	selectTunnelAction(m.actionMenu, "tunnel_bg")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd != nil || m.Result() != nil || !strings.Contains(m.View(), "key or agent authentication") {
		t.Fatalf("background auth limitation was not shown in TUI:\n%s", m.View())
	}
}

func TestForwardListStartsBackgroundTunnelAndKeepsFeedback(t *testing.T) {
	previous := StartBackgroundTunnel
	t.Cleanup(func() { StartBackgroundTunnel = previous })
	calls := 0
	StartBackgroundTunnel = func(alias string) (*model.TunnelState, error) {
		calls++
		if alias != "demo" {
			t.Fatalf("started tunnel for %q, want demo", alias)
		}
		return &model.TunnelState{PID: 1234}, nil
	}
	m := tunnelTestModel(model.AuthAgent)
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(1, "demo", 80, 24)
	m.forwardScreen.list = []*model.Forward{{Enabled: true}}
	updated, start := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = updated.(*tuiModel)
	if start == nil || !m.tunnelStarting || !strings.Contains(m.View(), "Starting background tunnel") {
		t.Fatalf("start has no immediate feedback:\n%s", m.View())
	}
	_, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if duplicate != nil {
		t.Fatal("a repeated key queued another background start")
	}
	updated, _ = m.Update(start())
	m = updated.(*tuiModel)
	if calls != 1 || m.tunnelStarting || m.screen != screenForwardList {
		t.Fatalf("start result lost its parent: calls=%d screen=%v", calls, m.screen)
	}
	first, second := m.View(), m.View()
	if !strings.Contains(first, "PID 1234") || !strings.Contains(second, "PID 1234") {
		t.Fatalf("launch acknowledgement did not persist:\n%s", second)
	}
}

func TestForwardListTunnelFailureStaysVisible(t *testing.T) {
	previous := StartBackgroundTunnel
	t.Cleanup(func() { StartBackgroundTunnel = previous })
	StartBackgroundTunnel = func(string) (*model.TunnelState, error) {
		return nil, errors.New("connection refused")
	}
	m := tunnelTestModel(model.AuthAgent)
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(1, "demo", 80, 24)
	m.forwardScreen.list = []*model.Forward{{Enabled: true}}
	updated, start := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = updated.(*tuiModel)
	updated, _ = m.Update(start())
	m = updated.(*tuiModel)
	if m.screen != screenForwardList || !strings.Contains(m.View(), "connection refused") || !strings.Contains(m.View(), "connection refused") {
		t.Fatalf("start error was lost:\n%s", m.View())
	}
}

func TestForwardListTunnelModesReturnToForwardList(t *testing.T) {
	m := tunnelTestModel(model.AuthAgent)
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(1, "demo", 80, 24)
	m.forwardScreen.list = []*model.Forward{{Enabled: true}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(*tuiModel)
	if cmd != nil || m.screen != screenActionMenu || !strings.Contains(m.View(), "demo") || !strings.Contains(m.View(), "1 enabled forwards") {
		t.Fatalf("mode menu lost server context:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if m.screen != screenForwardList {
		t.Fatalf("Esc returned to %v, want forward list", m.screen)
	}
}

func TestActionMenuStartsBackgroundTunnelWithoutQuitting(t *testing.T) {
	previousForwards, previousStart := ListForwards, StartBackgroundTunnel
	t.Cleanup(func() { ListForwards, StartBackgroundTunnel = previousForwards, previousStart })
	ListForwards = func(int64) ([]*model.Forward, error) {
		return []*model.Forward{{Enabled: true}}, nil
	}
	StartBackgroundTunnel = func(alias string) (*model.TunnelState, error) {
		if alias != "demo" {
			t.Fatalf("started tunnel for %q, want demo", alias)
		}
		return nil, errors.New("connection refused")
	}
	m := tunnelTestModel(model.AuthAgent)
	updated, load := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(*tuiModel)
	updated, _ = m.Update(load())
	m = updated.(*tuiModel)
	selectTunnelAction(m.actionMenu, "tunnel_bg")
	updated, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if start == nil || m.screen != screenList || m.Result() != nil || !strings.Contains(m.View(), "Starting background tunnel") {
		t.Fatalf("background action quit or lost pending feedback:\n%s", m.View())
	}
	updated, _ = m.Update(start())
	m = updated.(*tuiModel)
	if m.screen != screenList || !strings.Contains(m.View(), "connection refused") || !strings.Contains(m.View(), "connection refused") {
		t.Fatalf("background failure did not persist on dashboard:\n%s", m.View())
	}
}

func TestTunnelActionMenuFitsSupportedSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		menu := newActionMenuModel(size.width, size.height)
		menu.setServer(&model.Server{ID: 1, Alias: "production-数据库", AuthMethod: model.AuthPassword}, 1, false)
		selectTunnelAction(menu, "tunnel_bg")
		view := menu.View()
		assertUnifiedScreen(t, view, size.width, size.height)
		if !strings.Contains(view, "unavailable") || !strings.Contains(view, "key or agent") {
			t.Fatalf("missing disabled-mode explanation at %dx%d:\n%s", size.width, size.height, view)
		}
	}
}
