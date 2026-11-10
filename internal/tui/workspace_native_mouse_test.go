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
	return reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(desired).Pointer()
}

func batchedMouseMode(cmd tea.Cmd, desired tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	// Never run child commands: a workspace watcher may block on SSH.
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

func TestSSHWorkspaceRestoresScrollbackMouseAndNativeToggle(t *testing.T) {
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("first", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 80, 26
	m.workspaceSessions = []*workspacepkg.Session{session}

	// Default mouse reporting remains active when opening an SSH tab.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.workspaceActive != 1 || m.workspaceNativeMouse || batchedMouseMode(cmd, tea.DisableMouse) {
		t.Fatal("Alt+1 unexpectedly disabled the mouse in scrollback mode")
	}
	if batchedMouseMode(m.Init(), tea.DisableMouse) {
		t.Fatal("restored SSH workspace incorrectly disabled scrollback mouse")
	}

	_, _ = m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 3, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if !m.workspaceSelecting {
		t.Fatal("SSH scrollback mode ignored mouse selection")
	}

	// Native mode releases mouse reporting to the outer terminal.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true})
	if !m.workspaceNativeMouse || !batchedMouseMode(cmd, tea.DisableMouse) || m.workspaceSelecting {
		t.Fatal("Alt+M did not enter native selection and cancel pending selection")
	}
	if !batchedMouseMode(m.Init(), tea.DisableMouse) {
		t.Fatal("restored native mode did not disable mouse reporting")
	}
	_, mouseCmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: 3, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if mouseCmd != nil || m.workspaceSelecting {
		t.Fatal("native mode intercepted a terminal mouse selection")
	}

	// Moving to the dashboard and back preserves native mode, with mouse
	// reporting enabled only on the dashboard.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}, Alt: true})
	if m.workspaceActive != 0 || !batchedMouseMode(cmd, tea.EnableMouseCellMotion) {
		t.Fatal("Alt+0 did not restore dashboard mouse reporting")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.workspaceActive != 1 || !batchedMouseMode(cmd, tea.DisableMouse) {
		t.Fatal("returning to native SSH tab did not disable reporting")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true})
	if m.workspaceNativeMouse || !batchedMouseMode(cmd, tea.EnableMouseCellMotion) {
		t.Fatal("Alt+M did not restore internal scrollback mouse")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	if m.workspaceActive != 0 || len(m.workspaceSessions) != 0 || batchedMouseMode(cmd, tea.DisableMouse) {
		t.Fatal("Alt+W did not close SSH session and leave mouse reporting enabled")
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

	// Ordinary Ctrl+C remains a remote interrupt.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assertWorkspaceInput(t, proc, "\x03")

	// Ctrl+PgUp belongs to the remote program, not tab switching.
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
