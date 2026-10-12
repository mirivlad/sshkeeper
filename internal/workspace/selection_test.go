package workspace

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWorkspaceMouseSelectionUnicodeAndMultiline(t *testing.T) {
	proc := newFakeProcess()
	s := New("copy", proc, 20, 4)
	t.Cleanup(func() { _ = s.Close() })
	proc.feed("hello мир\r\nnext row\r\n")
	waitUntil(t, func() bool { return strings.Contains(ansi.Strip(s.Render()), "next row") })
	s.StartSelection(0, 0)
	s.MoveSelection(3, 1)
	if got := s.FinishSelection(3, 1); got != "hello мир\nnext" {
		t.Fatalf("selection=%q, want unicode multiline", got)
	}
	if !strings.Contains(s.Render(), "\x1b[7m") {
		t.Fatal("selection not highlighted in rendered terminal")
	}
	s.ClearSelection()
	if got := s.Render(); !strings.Contains(ansi.Strip(got), "next") {
		t.Fatal("terminal was unexpectedly changed after clearing selection")
	}
}
func TestWorkspaceClickWithoutDragDoesNotCopy(t *testing.T) {
	proc := newFakeProcess()
	s := New("click", proc, 20, 4)
	t.Cleanup(func() { _ = s.Close() })
	proc.feed("abcd\r\n")
	waitUntil(t, func() bool { return strings.Contains(ansi.Strip(s.Render()), "abcd") })
	s.StartSelection(1, 0)
	if got := s.FinishSelection(1, 0); got != "" {
		t.Fatalf("click selected: %q", got)
	}
	s.StartSelection(3, 0)
	if got := s.FinishSelection(1, 0); got != "bcd" {
		t.Fatalf("reverse drag selected %q", got)
	}
}
func TestWorkspaceSelectionFromScrollback(t *testing.T) {
	proc := newFakeProcess()
	s := New("history", proc, 12, 3)
	t.Cleanup(func() { _ = s.Close() })
	for i := 0; i < 9; i++ {
		proc.feed("history\r\n")
	}
	waitUntil(t, func() bool {
		s.screenMu.Lock()
		n := s.emu.ScrollbackLen()
		s.screenMu.Unlock()
		return n >= 5
	})
	if !s.Scroll(4) {
		t.Fatal("unable to scroll")
	}
	view := ansi.Strip(s.Render())
	s.StartSelection(0, 0)
	got := s.FinishSelection(6, 0)
	if len(got) == 0 || !strings.Contains(view, got) {
		t.Fatalf("selection %q not in viewport %q", got, view)
	}
}
