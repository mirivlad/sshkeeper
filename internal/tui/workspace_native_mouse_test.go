package tui

import (
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

func commandContainsMouseMode(cmd tea.Cmd, desired tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	// Recognize direct commands without executing long-running SSH watchers.
	return reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(desired).Pointer()
}

func batchedMouseMode(cmd tea.Cmd, desired tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	// In these code paths the outer Batch only constructs a command list;
	// children may contain blocking watch commands, so do not execute them.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return commandContainsMouseMode(cmd, desired)
	}
	for _, child := range batch {
		if commandContainsMouseMode(child, desired) {
			return true
		}
	}
	return false
}

func TestSSHWorkspaceDisablesMouseAndDashboardRestoresIt(t *testing.T) {
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("first", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.workspaceSessions = []*workspacepkg.Session{session}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.workspaceActive != 1 || !batchedMouseMode(cmd, tea.DisableMouse) {
		t.Fatal("Alt+1 did not enter SSH tab with native mouse selection enabled")
	}
	if !batchedMouseMode(m.Init(), tea.DisableMouse) {
		t.Fatal("restored SSH workspace did not disable mouse reporting")
	}

	// Stale mouse reports must not trigger SSHKeeper's own copy/selection logic.
	_, mouseCmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 3, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if mouseCmd != nil || m.workspaceSelecting {
		t.Fatal("SSH tab intercepted a native terminal mouse selection")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}, Alt: true})
	if m.workspaceActive != 0 || !batchedMouseMode(cmd, tea.EnableMouseCellMotion) {
		t.Fatal("Alt+0 did not restore dashboard mouse reporting")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.workspaceActive != 1 || !batchedMouseMode(cmd, tea.DisableMouse) {
		t.Fatal("returning to SSH tab did not restore native selection mode")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	if m.workspaceActive != 0 || len(m.workspaceSessions) != 0 || !batchedMouseMode(cmd, tea.EnableMouseCellMotion) {
		t.Fatal("Alt+W did not close SSH session and restore dashboard mouse mode")
	}
	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Alt+W did not terminate SSH session")
	}
}

func TestWorkspaceTabNavigationReservesOnlyAltBindings(t *testing.T) {
	proc := newClipboardWorkspaceProcess()
	session := workspacepkg.New("keys", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	// Ordinary Ctrl+C is a remote interrupt; it must not be treated as Copy.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assertWorkspaceInput(t, proc, "\x03")

	// Ctrl+PgUp is not a tab-switch shortcut; remote applications may use it.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlPgUp})
	if m.workspaceActive != 1 {
		t.Fatal("Ctrl+PgUp incorrectly switched SSHKeeper tabs")
	}
	select {
	case <-proc.writes:
	case <-time.After(2 * time.Second):
		t.Fatal("Ctrl+PgUp was not forwarded to the remote PTY")
	}
}
