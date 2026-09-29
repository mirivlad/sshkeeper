package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestSuccessNoticeFadesButNewerNoticeSurvives(t *testing.T) {
	m := New([]*model.Server{{Alias: "a", Host: "a", Port: 22, User: "u"}})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*tuiModel)
	if m.success == "" || cmd == nil {
		t.Fatal("sorting should show a notice and schedule its expiry")
	}
	first := cmd()

	// A newer notice replaces the first before the first one expires.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*tuiModel)
	newer := m.success
	updated, _ = m.Update(first)
	m = updated.(*tuiModel)
	if m.success != newer {
		t.Fatalf("stale expiry cleared the newer notice: %q", m.success)
	}
}

func TestExpiryClearsCurrentNotice(t *testing.T) {
	m := New(nil)
	m.success = "done"
	expire := m.expireNoticeCmd()
	updated, _ := m.Update(expire())
	if updated.(*tuiModel).success != "" {
		t.Fatal("the current notice should fade")
	}
}
