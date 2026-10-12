package workspace

import (
	"fmt"
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

func TestWordSelectionUnicodeAndPunctuation(t *testing.T) {
	proc := newFakeProcess()
	s := New("word", proc, 40, 5)
	t.Cleanup(func() { _ = s.Close() })
	proc.feed("hello мир_42, world\r\n")
	waitUntil(t, func() bool { return strings.Contains(ansi.Strip(s.Render()), "мир_42") })
	s.StartSelectionWithClicks(8, 0, 2)
	if got := s.FinishSelection(8, 0); got != "мир_42" {
		t.Fatalf("word = %q", got)
	}
	s.StartSelectionWithClicks(12, 0, 2)
	if got := s.FinishSelection(12, 0); got != "," {
		t.Fatalf("punctuation = %q", got)
	}
	s.StartSelectionWithClicks(3, 0, 2)
	s.MoveSelection(17, 0)
	if got := s.FinishSelection(17, 0); got != "hello мир_42, world" {
		t.Fatalf("word drag = %q", got)
	}
	s.StartSelectionWithClicks(15, 0, 2)
	if got := s.FinishSelection(1, 0); got != "hello мир_42, world" {
		t.Fatalf("reverse word drag = %q", got)
	}
}

func TestTripleClickSelectsLineAndDragWholeLines(t *testing.T) {
	proc := newFakeProcess()
	s := New("line", proc, 20, 5)
	t.Cleanup(func() { _ = s.Close() })
	proc.feed("short\r\nsecond line\r\nthird\r\n")
	waitUntil(t, func() bool { return strings.Contains(ansi.Strip(s.Render()), "third") })
	s.StartSelectionWithClicks(2, 1, 3)
	if got := s.FinishSelection(2, 1); got != "second line" {
		t.Fatalf("triple click=%q", got)
	}
	s.StartSelectionWithClicks(4, 1, 3)
	if got := s.FinishSelection(3, 2); got != "second line\nthird" {
		t.Fatalf("line drag=%q", got)
	}
	s.StartSelectionWithClicks(3, 2, 3)
	if got := s.FinishSelection(0, 0); got != "short\nsecond line\nthird" {
		t.Fatalf("reverse line drag=%q", got)
	}
}

func TestSelectionRemainsAnchoredWhileScrolling(t *testing.T) {
	proc := newFakeProcess()
	s := New("scroll-selection", proc, 16, 4)
	t.Cleanup(func() { _ = s.Close() })
	for i := 0; i < 14; i++ {
		proc.feed(fmt.Sprintf("line%02d\r\n", i))
	}
	waitUntil(t, func() bool {
		s.screenMu.Lock()
		n := s.emu.ScrollbackLen()
		s.screenMu.Unlock()
		return n >= 10
	})
	s.StartSelection(0, 2)
	// Move the viewport up through scrollback while preserving the anchor at the
	// originally clicked absolute row.
	if !s.Scroll(5) {
		t.Fatal("unable to scroll while selecting")
	}
	s.MoveSelection(5, 0)
	got := s.FinishSelection(5, 0)
	if !strings.Contains(got, "line") || strings.Count(got, "\n") < 3 {
		t.Fatalf("selection not spanning history: %q", got)
	}
	if !s.Scroll(2) {
		t.Fatal("unable to scroll after mouse release")
	}
	if again := s.selectedText(); again != got {
		t.Fatalf("wheel invalidated selection: old %q now %q", got, again)
	}
}

func (s *Session) selectedText() string {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	return s.selectedTextLocked()
}
