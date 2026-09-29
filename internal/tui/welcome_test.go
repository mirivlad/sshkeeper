package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestEmptyDashboardShowsWelcomeWithFirstSteps(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		m := New(nil)
		m.width, m.height = size.width, size.height
		view := m.View()
		assertViewFits(t, view, size.width, size.height)
		for _, want := range []string{"Welcome", "sshkeeper", "import hosts from ~/.ssh/config", "add a server by hand", "i import"} {
			if !strings.Contains(view, want) {
				t.Fatalf("welcome at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestImportKeyWorksOnlyWhenEmpty(t *testing.T) {
	previousImport, previousList := ImportServers, ListServers
	t.Cleanup(func() { ImportServers, ListServers = previousImport, previousList })
	imported := 0
	ImportServers = func() (int, error) { imported++; return 1, nil }
	ListServers = func() ([]*model.Server, error) {
		return []*model.Server{{Alias: "alpha", Host: "a", Port: 22, User: "u"}}, nil
	}

	m := New(nil)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(*tuiModel)
	if cmd == nil {
		t.Fatal("i should import on the welcome screen")
	}
	updated, _ = m.Update(cmd())
	m = updated.(*tuiModel)
	if imported != 1 || len(m.servers) != 1 || !strings.Contains(m.success, "Imported 1") {
		t.Fatalf("import result not applied: imported=%d servers=%d success=%q", imported, len(m.servers), m.success)
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}); cmd != nil {
		t.Fatal("i must not import once profiles exist")
	}
}
