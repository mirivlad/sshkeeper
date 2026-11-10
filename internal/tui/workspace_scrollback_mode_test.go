package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

// Exercise the full TUI mouse event path, not the internal scroll helper.
// A nested SSH shell generates ordinary VT output in the same workspace.
func TestSSHScrollbackWheelViaTUI(t *testing.T) {
	proc := newFeedableWorkspaceProcess()
	session := workspacepkg.New("tomas -> vpn", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	for i := 0; i < 36; i++ {
		_, _ = fmt.Fprintf(proc.writer, "remote%02d\r\n", i)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(session.Render(), "remote35") {
		if time.Now().After(deadline) {
			t.Fatal("nested SSH output did not reach the workspace")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _ = m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 5, Y: 3, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress,
	}))
	if got := session.ScrollOffset(); got != 3 {
		t.Fatalf("wheel did not scroll the internal history, offset=%d", got)
	}
	_, _ = m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 5, Y: 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	}))
	if got := session.ScrollOffset(); got != 0 {
		t.Fatalf("wheel down failed to return to live output, offset=%d", got)
	}
}

// In scrollback mode the user can highlight old lines and copy explicitly,
// without destroying clipboard contents on every drag or double click.
func TestSSHScrollbackDragAndAltCopyViaTUI(t *testing.T) {
	proc := newFeedableWorkspaceProcess()
	session := workspacepkg.New("copy", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	for i := 0; i < 22; i++ {
		_, _ = fmt.Fprintf(proc.writer, "entry%02d\r\n", i)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(session.Render(), "entry21") {
		if time.Now().After(deadline) {
			t.Fatal("log output did not reach the workspace")
		}
		time.Sleep(5 * time.Millisecond)
	}

	oldPrimary := workspacePrimaryWrite
	oldClipboard := workspaceClipboardWrite
	var primary, clipboard string
	workspacePrimaryWrite = func(text string) { primary = text }
	workspaceClipboardWrite = func(text string) error { clipboard = text; return nil }
	t.Cleanup(func() {
		workspacePrimaryWrite = oldPrimary
		workspaceClipboardWrite = oldClipboard
	})

	_, _ = m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 1, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	_, scrollCmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion,
	}))
	if scrollCmd == nil || session.ScrollOffset() == 0 || !m.workspaceSelecting {
		t.Fatal("dragging to top edge did not start SSH history scrolling")
	}
	_, primaryCmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 0, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease,
	}))
	if primaryCmd == nil {
		t.Fatal("drag selection did not produce PRIMARY update")
	}
	_ = primaryCmd()
	if primary == "" || clipboard != "" {
		t.Fatalf("selection wrote unexpected clipboard values: PRIMARY=%q CLIPBOARD=%q", primary, clipboard)
	}

	_, copyCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: true})
	if copyCmd == nil {
		t.Fatal("Alt+C in scrollback mode did not request copy")
	}
	msg, ok := copyCmd().(workspaceCopiedMsg)
	if !ok || msg.err != nil || clipboard != primary {
		t.Fatalf("Alt+C failed to copy selection: result=%#v PRIMARY=%q CLIPBOARD=%q", msg, primary, clipboard)
	}
}
