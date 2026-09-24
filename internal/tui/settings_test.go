package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
)

func TestSettingsLanguageSelectionAndLayout(t *testing.T) {
	previous := i18n.Preference()
	t.Cleanup(func() { _ = i18n.SetPreference(previous) })
	for _, language := range []string{i18n.English, i18n.Russian} {
		if err := i18n.SetPreference(language); err != nil {
			t.Fatal(err)
		}
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
			model := newSettingsModel(size.width, size.height, i18n.Auto)
			view := model.View()
			assertUnifiedScreen(t, view, size.width, size.height)
			if !strings.Contains(view, "Русский") || !strings.Contains(view, "English") {
				t.Fatalf("language choices missing at %dx%d:\n%s", size.width, size.height, view)
			}
		}
	}
	model := newSettingsModel(80, 24, i18n.Auto)
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected, changed, back := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != i18n.Russian || !changed || back {
		t.Fatalf("unexpected language selection: %q %v %v", selected, changed, back)
	}
	model.applyPreference(selected)
	if _, changed, back := model.Update(tea.KeyMsg{Type: tea.KeyEsc}); changed || !back {
		t.Fatal("Esc did not return from settings")
	}
}
