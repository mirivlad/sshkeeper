package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func withNow(t *testing.T, value time.Time) {
	t.Helper()
	previous := now
	now = func() time.Time { return value }
	t.Cleanup(func() { now = previous })
}

func sortTestServers(base time.Time) []*model.Server {
	hourAgo := base.Add(-time.Hour)
	weekAgo := base.Add(-8 * 24 * time.Hour)
	return []*model.Server{
		{Alias: "alpha", Host: "a", Port: 22, User: "u", GroupName: "Zeta", LastConnectedAt: &weekAgo},
		{Alias: "bravo", Host: "b", Port: 22, User: "u"},
		{Alias: "charlie", Host: "c", Port: 22, User: "u", GroupName: "Alpha", LastConnectedAt: &hourAgo},
	}
}

func rowAliases(m *tuiModel) string {
	return strings.Join(visibleAliases(m), ",")
}

func TestSortKeyCyclesNameRecentGroupAndPersists(t *testing.T) {
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	withNow(t, base)
	previousGet, previousSet := GetSortPreference, SetSortPreference
	t.Cleanup(func() { GetSortPreference, SetSortPreference = previousGet, previousSet })
	saved := ""
	GetSortPreference = func() string { return sortByName }
	SetSortPreference = func(value string) error { saved = value; return nil }

	m := New(sortTestServers(base))
	if got := rowAliases(m); got != "alpha,bravo,charlie" {
		t.Fatalf("name order = %q", got)
	}
	m = typeKeys(m, "s")
	if got := rowAliases(m); got != "charlie,alpha,bravo" || saved != sortByRecent {
		t.Fatalf("recent order = %q saved=%q; never-connected goes last", got, saved)
	}
	m = typeKeys(m, "s")
	if got := rowAliases(m); got != "charlie,alpha,bravo" || saved != sortByGroup {
		t.Fatalf("group order = %q saved=%q; ungrouped goes last", got, saved)
	}
	m = typeKeys(m, "s")
	if m.sortMode != sortByName || saved != sortByName {
		t.Fatalf("sort should wrap to name, got %q", m.sortMode)
	}
}

func TestSortKeepsCursorOnSameServer(t *testing.T) {
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	m := New(sortTestServers(base))
	m.moveCursor(1) // bravo
	m = typeKeys(m, "s")
	if selected := m.selectedServer(); selected == nil || selected.Alias != "bravo" {
		t.Fatalf("cursor should follow bravo across sort, got %#v", selected)
	}
}

func TestSortPreferenceIsReadOnStartup(t *testing.T) {
	previousGet := GetSortPreference
	t.Cleanup(func() { GetSortPreference = previousGet })
	GetSortPreference = func() string { return sortByRecent }
	base := time.Now()
	m := New(sortTestServers(base))
	if m.sortMode != sortByRecent || rowAliases(m) != "charlie,alpha,bravo" {
		t.Fatalf("startup sort = %q order=%q", m.sortMode, rowAliases(m))
	}
}

func TestRelativeAgeIsCompact(t *testing.T) {
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	withNow(t, base)
	cases := map[time.Duration]string{
		10 * time.Second:     "now",
		5 * time.Minute:      "5m",
		3 * time.Hour:        "3h",
		2 * 24 * time.Hour:   "2d",
		21 * 24 * time.Hour:  "3w",
		800 * 24 * time.Hour: "2y",
	}
	for age, want := range cases {
		at := base.Add(-age)
		if got := relativeAge(&at); got != want {
			t.Errorf("relativeAge(-%v) = %q, want %q", age, got, want)
		}
	}
	if got := relativeAge(nil); got != "—" {
		t.Errorf("never = %q", got)
	}
}

func TestStateRestoresCursorAndNotice(t *testing.T) {
	base := time.Now()
	m := New(sortTestServers(base))
	m.moveCursor(2)
	state := m.State()
	state.Notice = "Back from charlie"

	restored := New(sortTestServers(base))
	restored.Restore(state)
	if selected := restored.selectedServer(); selected == nil || selected.Alias != "charlie" {
		t.Fatalf("restored cursor = %#v", selected)
	}
	restored.width, restored.height = 120, 30
	if !strings.Contains(restored.View(), "Back from charlie") {
		t.Fatal("restored notice should be visible")
	}
	updated, _ := restored.Update(tea.KeyMsg{Type: tea.KeyDown})
	if strings.Contains(updated.(*tuiModel).View(), "Back from charlie") {
		t.Fatal("notice should clear on the next key")
	}
}
