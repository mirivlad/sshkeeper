package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func groupTestModel() *tuiModel {
	m := New([]*model.Server{
		{Alias: "web", Host: "a", Port: 22, User: "u", GroupName: "Production"},
		{Alias: "db", Host: "b", Port: 22, User: "u", GroupName: "Production"},
		{Alias: "api", Host: "c", Port: 22, User: "u", GroupName: "Staging"},
		{Alias: "home", Host: "d", Port: 22, User: "u"},
	})
	m.sortMode = sortByGroup
	m.rebuildServerRows("")
	m.width, m.height = 120, 30
	return m
}

func rowShape(m *tuiModel) string {
	parts := []string{}
	for _, row := range m.rows {
		if row.isHeader() {
			parts = append(parts, "["+row.group+"]")
		} else {
			parts = append(parts, row.server.Alias)
		}
	}
	return strings.Join(parts, " ")
}

func TestGroupOrderShowsHeadersWithUngroupedLast(t *testing.T) {
	m := groupTestModel()
	if got := rowShape(m); got != "[Production] db web [Staging] api [] home" {
		t.Fatalf("rows = %q", got)
	}
	if selected := m.selectedServer(); selected == nil || selected.Alias != "db" {
		t.Fatalf("cursor should start on the first profile, not a header: %#v", selected)
	}
	view := m.View()
	for _, want := range []string{"▾ Production  2", "▾ Staging  1", "▾ No group  1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing header %q:\n%s", want, view)
		}
	}
}

func TestLeftFoldsGroupAndEnterOnHeaderUnfolds(t *testing.T) {
	m := groupTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(*tuiModel)
	if got := rowShape(m); got != "[Production] [Staging] api [] home" {
		t.Fatalf("after fold rows = %q", got)
	}
	if header, ok := m.selectedHeader(); !ok || header.group != "Production" {
		t.Fatal("cursor should move to the folded header")
	}
	if !strings.Contains(m.View(), "▸ Production  2") {
		t.Fatal("folded header should show the collapsed glyph")
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd != nil {
		t.Fatal("Enter on a header must not connect")
	}
	if got := rowShape(m); got != "[Production] db web [Staging] api [] home" {
		t.Fatalf("after unfold rows = %q", got)
	}
}

func TestHeaderRowIgnoresServerActions(t *testing.T) {
	m := groupTestModel()
	m.cursor = 0
	m = typeKeys(m, "e")
	if m.screen != screenList {
		t.Fatalf("edit on a header should do nothing, screen=%v", m.screen)
	}
}

func TestFilterFlattensGroups(t *testing.T) {
	m := typeKeys(typeKeys(groupTestModel(), "/"), "api")
	if got := rowShape(m); got != "api" {
		t.Fatalf("filtered rows should be flat, got %q", got)
	}
}

func TestCollapsedGroupsSurviveRestart(t *testing.T) {
	m := groupTestModel()
	m.setGroupCollapsed("Staging", true)
	state := m.State()

	restored := groupTestModel()
	restored.Restore(state)
	if got := rowShape(restored); got != "[Production] db web [Staging] [] home" {
		t.Fatalf("restored rows = %q", got)
	}
}
