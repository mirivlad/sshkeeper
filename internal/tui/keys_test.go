package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestLetterKeysMirrorCtrlShortcuts(t *testing.T) {
	servers := func() []*model.Server {
		return []*model.Server{
			{ID: 1, Alias: "one", Host: "one.example.org", Port: 22, User: "root", AuthMethod: model.AuthKey},
			{ID: 2, Alias: "two", Host: "two.example.org", Port: 22, User: "root", AuthMethod: model.AuthKey},
		}
	}
	cases := []struct {
		key  string
		want screen
	}{
		{"a", screenForm},
		{"e", screenForm},
		{"d", screenConfirm},
		{"x", screenActionMenu},
		{"f", screenForwardList},
		{"m", screenManageMenu},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			m := typeKeys(New(servers()), tc.key)
			if m.screen != tc.want {
				t.Fatalf("%q opened screen %v, want %v", tc.key, m.screen, tc.want)
			}
		})
	}
}

func TestSpaceSelectsAndJKMove(t *testing.T) {
	m := New([]*model.Server{
		{Alias: "one", Host: "a", Port: 22, User: "u"},
		{Alias: "two", Host: "b", Port: 22, User: "u"},
		{Alias: "three", Host: "c", Port: 22, User: "u"},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m = updated.(*tuiModel)
	if !m.selected["one"] || m.selectedServer().Alias != "three" {
		t.Fatalf("space should select and advance: selected=%v cursor=%s", m.selected, m.selectedServer().Alias)
	}
	m = typeKeys(m, "k")
	if m.selectedServer().Alias != "one" {
		t.Fatalf("k should move up, got %s", m.selectedServer().Alias)
	}
	m = typeKeys(m, "G")
	if m.selectedServer().Alias != "two" {
		t.Fatalf("G should jump to last, got %s", m.selectedServer().Alias)
	}
	m = typeKeys(m, "g")
	if m.selectedServer().Alias != "one" {
		t.Fatalf("g should jump to first, got %s", m.selectedServer().Alias)
	}
}

func TestQQuitsFromList(t *testing.T) {
	m := New(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should quit from the list")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should return tea.Quit")
	}
}

func TestLettersStayTextInsideFilter(t *testing.T) {
	m := typeKeys(New([]*model.Server{{Alias: "one", Host: "a", Port: 22, User: "u"}}), "/aeq")
	if m.screen != screenSearch || m.searchInput.Value() != "aeq" {
		t.Fatalf("letters must be filter text: screen=%v value=%q", m.screen, m.searchInput.Value())
	}
}
