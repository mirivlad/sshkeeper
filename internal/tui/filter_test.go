package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func filterTestModel() *tuiModel {
	m := New([]*model.Server{
		{ID: 1, Alias: "bastion", DisplayName: "Bastion Host", Host: "bastion.example.com", Port: 22, User: "jump", AuthMethod: model.AuthKey},
		{ID: 2, Alias: "db", DisplayName: "DB Master", Host: "db01.internal", Port: 22, User: "postgres", AuthMethod: model.AuthKey, GroupName: "Production"},
		{ID: 3, Alias: "prod", DisplayName: "Production web", Host: "web01.example.com", Port: 22, User: "ops", AuthMethod: model.AuthAgent, GroupName: "Production"},
	})
	m.width, m.height = 120, 30
	return m
}

func typeKeys(m *tuiModel, keys string) *tuiModel {
	for _, r := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(*tuiModel)
	}
	return m
}

func visibleAliases(m *tuiModel) []string {
	aliases := []string{}
	for _, row := range m.rows {
		if row.server != nil {
			aliases = append(aliases, row.server.Alias)
		}
	}
	return aliases
}

func TestSlashStartsLiveFilterThatNarrowsAsYouType(t *testing.T) {
	m := typeKeys(filterTestModel(), "/")
	if m.screen != screenSearch {
		t.Fatalf("slash should start the live filter, screen=%v", m.screen)
	}
	m = typeKeys(m, "prod")
	if got := strings.Join(visibleAliases(m), ","); got != "prod,db" {
		t.Fatalf("visible after 'prod' = %q; want label match first, then group match", got)
	}
	if selected := m.selectedServer(); selected == nil || selected.Alias != "prod" {
		t.Fatalf("cursor should sit on the best match, got %#v", selected)
	}
	view := m.View()
	if !strings.Contains(view, "/ prod") || !strings.Contains(view, "2 of 3") {
		t.Fatalf("filter prompt and match count missing:\n%s", view)
	}
}

func TestLiveFilterEnterConnectsToBestMatch(t *testing.T) {
	m := typeKeys(typeKeys(filterTestModel(), "/"), "bst")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in the filter should connect")
	}
	msg, ok := cmd().(connectRequestMsg)
	if !ok || msg.server.Alias != "bastion" {
		t.Fatalf("expected fuzzy match to connect to bastion, got %#v", msg)
	}
}

func TestLiveFilterEscClearsAndTabKeeps(t *testing.T) {
	m := typeKeys(typeKeys(filterTestModel(), "/"), "db")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*tuiModel)
	if m.screen != screenList || len(visibleAliases(m)) != 1 {
		t.Fatalf("Tab should keep the filter and return to the list: screen=%v rows=%v", m.screen, visibleAliases(m))
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if m.filterQuery() != "" || len(visibleAliases(m)) != 3 {
		t.Fatalf("Esc on the list should clear a kept filter: %v", visibleAliases(m))
	}
	if selected := m.selectedServer(); selected == nil || selected.Alias != "db" {
		t.Fatalf("clearing the filter should keep the cursor on db, got %#v", selected)
	}
}

func TestLiveFilterMatchesForwardPorts(t *testing.T) {
	m := filterTestModel()
	m.setForwardIndex([]*model.Forward{{ServerID: 2, Name: "pg", LocalPort: 15432, RemotePort: 5432}})
	m = typeKeys(typeKeys(m, "/"), "15432")
	if got := strings.Join(visibleAliases(m), ","); got != "db" {
		t.Fatalf("forward port should match its server, got %q", got)
	}
}

func TestLiveFilterShowsEmptyState(t *testing.T) {
	m := typeKeys(typeKeys(filterTestModel(), "/"), "zzz")
	if m.selectedServer() != nil {
		t.Fatal("no server should be selected when nothing matches")
	}
	if !strings.Contains(m.View(), "Nothing matches") {
		t.Fatal("expected empty filter hint")
	}
}

func TestMatchServerHighlightsLabelRunes(t *testing.T) {
	server := &model.Server{Alias: "web", DisplayName: "Продакшн веб"}
	match, ok := matchServer(server, "веб", "")
	if !ok {
		t.Fatal("expected Cyrillic substring to match")
	}
	if got := len(match.label); got != 3 || match.label[0] != 9 {
		t.Fatalf("highlight positions = %v", match.label)
	}
	if _, ok := matchServer(server, "веб missing", ""); ok {
		t.Fatal("every token must match")
	}
}

func TestLiveFilterAcceptsCoalescedSlashInput(t *testing.T) {
	m := filterTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/mail")})
	m = updated.(*tuiModel)
	if m.screen != screenSearch || m.searchInput.Value() != "mail" {
		t.Fatalf("coalesced input should start the filter with the rest: screen=%v value=%q", m.screen, m.searchInput.Value())
	}
}
