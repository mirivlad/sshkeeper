package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func esc(t *testing.T, m *tuiModel) *tuiModel {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	return updated.(*tuiModel)
}

// Esc on a picker opened from the server form closes only the picker.
func TestEscClosesFormPickersNotTheForm(t *testing.T) {
	pickers := map[string]func(fm *formModel) *bool{
		"auth":     func(fm *formModel) *bool { return &fm.showAuthList },
		"group":    func(fm *formModel) *bool { return &fm.showGroupList },
		"identity": func(fm *formModel) *bool { return &fm.showIdentityList },
		"tags":     func(fm *formModel) *bool { return &fm.showTagList },
		"startup":  func(fm *formModel) *bool { return &fm.showStartupList },
		"route":    func(fm *formModel) *bool { return &fm.showRouteList },
	}
	for name, flag := range pickers {
		t.Run(name, func(t *testing.T) {
			m := New(nil)
			m.width, m.height = 100, 30
			m = typeKeys(m, "a")
			m.form.inputs[0].SetValue("typed")
			*flag(m.form) = true

			m = esc(t, m)
			if m.screen != screenForm || m.form == nil {
				t.Fatalf("Esc on the %s picker left the form: screen=%v", name, m.screen)
			}
			if *flag(m.form) {
				t.Fatalf("Esc did not close the %s picker", name)
			}
			if m.form.inputs[0].Value() != "typed" {
				t.Fatal("closing a picker must keep the entered values")
			}
		})
	}
}

func TestEscClearsPickerFilterBeforeClosing(t *testing.T) {
	m := New(nil)
	m.width, m.height = 100, 30
	m = typeKeys(m, "a")
	m.form.identityList = newStringList([]string{"~/.ssh/id_ed25519", "~/.ssh/work"}, "keys", 40, 10)
	m.form.identityList.SetFilterText("work")
	m.form.showIdentityList = true
	if m.form.identityList.FilterState() == list.Unfiltered {
		t.Fatal("test setup: filter should be applied")
	}

	m = esc(t, m)
	if !m.form.showIdentityList || m.form.identityList.FilterState() != list.Unfiltered {
		t.Fatalf("first Esc should clear the filter and keep the picker: open=%v state=%v", m.form.showIdentityList, m.form.identityList.FilterState())
	}
	m = esc(t, m)
	if m.form.showIdentityList || m.screen != screenForm {
		t.Fatal("second Esc should close the picker and stay in the form")
	}
}

// Esc on a screen opened from Manage returns to Manage with that entry
// highlighted; the same screen opened by a shortcut returns to the list.
func TestEscReturnsManagerScreensToManageMenu(t *testing.T) {
	for _, action := range []string{"groups", "tags", "templates", "sessions", "tunnels", "settings"} {
		t.Run(action, func(t *testing.T) {
			m := New([]*model.Server{{Alias: "a", Host: "a", Port: 22, User: "u"}})
			m.width, m.height = 100, 30
			m.sessionsAvailable = true
			m.openManageMenu(action)
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(*tuiModel)
			if m.screen == screenManageMenu || m.screen == screenList {
				t.Fatalf("Enter on %s did not open it, screen=%v", action, m.screen)
			}

			m = esc(t, m)
			if m.screen != screenManageMenu {
				t.Fatalf("Esc from %s went to screen %v, want Manage", action, m.screen)
			}
			item, ok := m.manageMenu.list.SelectedItem().(actionMenuItem)
			if !ok || item.action != action {
				t.Fatalf("Manage should highlight %s, got %#v", action, item)
			}
			m = esc(t, m)
			if m.screen != screenList {
				t.Fatalf("Esc from Manage should reach the list, got %v", m.screen)
			}
		})
	}
}

func TestEscFromShortcutManagerReturnsToList(t *testing.T) {
	m := New([]*model.Server{{Alias: "a", Host: "a", Port: 22, User: "u"}})
	// Enter Tags through Manage first, then through Ctrl+G: the shortcut
	// must not inherit the earlier parent.
	m.openManageMenu("tags")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = esc(t, updated.(*tuiModel))
	m = esc(t, m)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = esc(t, updated.(*tuiModel))
	if m.screen != screenList {
		t.Fatalf("Esc from Tags opened with Ctrl+G should reach the list, got %v", m.screen)
	}
}
