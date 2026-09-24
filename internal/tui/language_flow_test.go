package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestSettingsSwitchesVisibleLanguageAndReturnsToManage(t *testing.T) {
	previousLanguage := i18n.Preference()
	previousGet, previousSet := GetLanguagePreference, SetLanguagePreference
	t.Cleanup(func() {
		_ = i18n.SetPreference(previousLanguage)
		GetLanguagePreference, SetLanguagePreference = previousGet, previousSet
	})
	_ = i18n.SetPreference(i18n.English)
	preference := i18n.Auto
	GetLanguagePreference = func() string { return preference }
	SetLanguagePreference = func(value string) error { preference = value; return nil }
	m := New([]*model.Server{{ID: 1, Alias: "mail", Host: "example.org"}})
	m.width, m.height = 80, 24
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(*tuiModel)
	if m.screen != screenManageMenu {
		t.Fatal("Manage did not open")
	}
	for index, raw := range m.manageMenu.list.Items() {
		if raw.(actionMenuItem).action == "settings" {
			m.manageMenu.list.Select(index)
			break
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if m.screen != screenSettings || !strings.Contains(m.View(), "English") {
		t.Fatalf("settings did not open:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*tuiModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if preference != i18n.Russian || i18n.Effective() != i18n.Russian || !strings.Contains(m.View(), "Настройки") {
		t.Fatalf("Russian language was not applied immediately:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if m.screen != screenManageMenu || !strings.Contains(m.View(), "Работающие туннели") {
		t.Fatalf("Manage did not rebuild in Russian:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if m.screen != screenList || !strings.Contains(m.View(), "Ctrl+W") || !strings.Contains(m.View(), "правила проброса") {
		t.Fatalf("dashboard lost discoverable forward entry:\n%s", m.View())
	}
	m.width, m.height = 60, 16
	assertUnifiedScreen(t, m.View(), 60, 16)
	if !strings.Contains(m.View(), "Ctrl+W") {
		t.Fatalf("narrow dashboard hides forward entry:\n%s", m.View())
	}
}

func TestSettingsSaveFailureKeepsCurrentLanguage(t *testing.T) {
	previousLanguage := i18n.Preference()
	previousSet := SetLanguagePreference
	t.Cleanup(func() {
		_ = i18n.SetPreference(previousLanguage)
		SetLanguagePreference = previousSet
	})
	_ = i18n.SetPreference(i18n.English)
	SetLanguagePreference = func(string) error { return errors.New("disk full") }
	m := New(nil)
	m.width, m.height = 80, 24
	m.screen = screenSettings
	m.settingsScreen = newSettingsModel(80, 24, i18n.English)
	m.settingsScreen.selected = 1 // Russian
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if i18n.Effective() != i18n.English || m.settingsScreen.preference != i18n.English || !strings.Contains(m.View(), "disk full") {
		t.Fatalf("failed save changed language or hid error:\n%s", m.View())
	}
}

func TestNarrowActionMenuExplainsRuleAndRoute(t *testing.T) {
	previous := i18n.Preference()
	t.Cleanup(func() { _ = i18n.SetPreference(previous) })
	_ = i18n.SetPreference(i18n.Russian)
	menu := newActionMenuModel(60, 16)
	for _, action := range []string{"forwards", "route"} {
		for index, raw := range menu.list.Items() {
			if raw.(actionMenuItem).action == action {
				menu.list.Select(index)
				break
			}
		}
		view := menu.View()
		assertUnifiedScreen(t, view, 60, 16)
		if !strings.Contains(view, "Выбранное действие") {
			t.Fatalf("selected action has no explanation at 60x16:\n%s", view)
		}
	}
}
