package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestVaultStatusTracksSuccessfulLock(t *testing.T) {
	oldUnlocked, oldLock := VaultUnlocked, LockVault
	t.Cleanup(func() { VaultUnlocked, LockVault = oldUnlocked, oldLock })
	VaultUnlocked = func() bool { return true }
	LockVault = func() error { return nil }

	m := New(nil)
	if !strings.Contains(m.View(), "Vault unlocked") {
		t.Fatalf("initial status is not unlocked:\n%s", m.View())
	}
	m.actionMenu = newActionMenuModel(80, 24)
	m.screen = screenActionMenu
	for i := range m.actionMenu.list.Items() {
		m.actionMenu.list.Select(i)
		item, ok := m.actionMenu.list.SelectedItem().(actionMenuItem)
		if ok && item.action == "vault_lock" {
			break
		}
	}
	updated, _ := m.updateActionMenu(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	view := m.View()
	if !strings.Contains(view, "Vault locked") || strings.Contains(view, "Vault unlocked") {
		t.Fatalf("status after lock is false:\n%s", view)
	}
}

func TestNotificationSurvivesRepeatedView(t *testing.T) {
	m := New(nil)
	m.err = errors.New("reload failed")
	first := m.View()
	second := m.View()
	if !strings.Contains(first, "reload failed") || !strings.Contains(second, "reload failed") {
		t.Fatalf("notification was consumed by View: first=%q second=%q", first, second)
	}
}

func TestFullHelpReturnsToOriginatingScreen(t *testing.T) {
	m := New(nil)
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(1, "prod", 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(*tuiModel)
	if m.screen != screenFullHelp || m.fullHelp == nil {
		t.Fatalf("F1 did not open full help from forward list: screen=%v", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if m.screen != screenForwardList || m.fullHelp != nil {
		t.Fatalf("help did not return to forward list: screen=%v help=%v", m.screen, m.fullHelp)
	}
}

func TestContextHelpReturnsToOriginatingManager(t *testing.T) {
	m := New(nil)
	m.screen = screenForwardList
	m.forwardScreen = newForwardScreenModel(1, "prod", 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(*tuiModel)
	if m.screen != screenHelp || m.helpScreen == nil {
		t.Fatalf("? did not open help from manager: screen=%v", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(*tuiModel)
	if m.screen != screenForwardList || m.helpScreen != nil {
		t.Fatalf("q did not return to manager: screen=%v help=%v", m.screen, m.helpScreen)
	}
}

func TestHelpShortcutDoesNotStealQuestionMarkFromSearch(t *testing.T) {
	m := New(nil)
	m.screen = screenSearch
	m.searchInput.Focus()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(*tuiModel)
	if m.screen != screenSearch || m.searchInput.Value() != "?" {
		t.Fatalf("search input lost printable rune: screen=%v value=%q", m.screen, m.searchInput.Value())
	}
}

func TestResizePropagatesToActiveChildren(t *testing.T) {
	m := New(nil)
	m.form = newFormModel(80, 24)
	m.forwardScreen = newForwardScreenModel(1, "prod", 80, 24)
	m.forwardForm = newForwardFormModel(1, 80, 24)
	m.templateForm = newTemplateFormModel(nil, 80, 24)
	m.tunnelScreen = newTunnelScreenModel(80, 24)
	m.helpScreen = newHelpScreenModel(80, 24)
	m.fullHelp = newFullHelpModel(80, 24)
	m.actionMenu = newActionMenuModel(80, 24)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	m = updated.(*tuiModel)
	if m.form.width != 60 || m.form.height != 16 ||
		m.forwardScreen.width != 60 || m.forwardScreen.height != 16 ||
		m.forwardForm.width != 60 || m.forwardForm.height != 16 ||
		m.templateForm.width != 60 || m.templateForm.height != 16 ||
		m.tunnelScreen.width != 60 || m.tunnelScreen.height != 16 ||
		m.helpScreen.width != 60 || m.fullHelp.width != 60 || m.fullHelp.height != 16 {
		t.Fatalf("resize did not reach every child: %#v", m)
	}
}
