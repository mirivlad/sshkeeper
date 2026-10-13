package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

// A process that records the exact bytes sent to the remote PTY.
type clipboardWorkspaceProcess struct {
	*tuiWorkspaceFakeProcess
	writes chan string
}

func newClipboardWorkspaceProcess() *clipboardWorkspaceProcess {
	return &clipboardWorkspaceProcess{
		tuiWorkspaceFakeProcess: newTUIWorkspaceFakeProcess(),
		writes:                  make(chan string, 10),
	}
}

func (p *clipboardWorkspaceProcess) Write(b []byte) (int, error) {
	p.writes <- string(b)
	return len(b), nil
}

func assertWorkspaceInput(t *testing.T, p *clipboardWorkspaceProcess, want string) {
	t.Helper()
	select {
	case got := <-p.writes:
		if !strings.Contains(got, want) {
			t.Fatalf("PTY received %q; expected %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("PTY never received %q", want)
	}
}

func TestWorkspaceAltCCopiesExistingHighlight(t *testing.T) {
	proc := newFeedableWorkspaceProcess()
	session := workspacepkg.New("copy", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	_, _ = fmt.Fprint(proc.writer, "alpha beta\r\n")
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(session.Render(), "alpha beta") {
		if time.Now().After(deadline) {
			t.Fatal("PTY text never appeared")
		}
		time.Sleep(5 * time.Millisecond)
	}
	session.StartSelection(0, 0)
	if got := session.FinishSelection(4, 0); got != "alpha" {
		t.Fatalf("highlighted text = %q", got)
	}
	old := workspaceClipboardWrite
	var copied string
	workspaceClipboardWrite = func(text string) error { copied = text; return nil }
	t.Cleanup(func() { workspaceClipboardWrite = old })

	_, cmd := m.updateWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: true})
	if cmd == nil {
		t.Fatal("Alt+C did not request clipboard copy")
	}
	if _, ok := cmd().(workspaceCopiedMsg); !ok || copied != "alpha" {
		t.Fatalf("Alt+C clipboard value = %q", copied)
	}
	if got := session.SelectedText(); got != "alpha" {
		t.Fatalf("copy discarded highlight: %q", got)
	}
}

func TestWorkspacePasteFromBracketedInputAndClipboard(t *testing.T) {
	proc := newClipboardWorkspaceProcess()
	session := workspacepkg.New("paste", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	// Ctrl+Shift+V is handled by the outer terminal on most platforms:
	// Bubble Tea receives a KeyMsg with Paste=true.
	_, cmd := m.updateWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bracketed"), Paste: true})
	if cmd != nil {
		t.Fatal("bracketed paste unexpectedly starts a clipboard subprocess")
	}
	assertWorkspaceInput(t, proc, "bracketed")

	old := workspaceClipboardRead
	var calls []bool
	workspaceClipboardRead = func(primary bool) (string, error) {
		calls = append(calls, primary)
		if primary {
			return "primary-text", nil
		}
		return "clipboard-text", nil
	}
	t.Cleanup(func() { workspaceClipboardRead = old })

	_, cmd = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 2, Y: 3, Button: tea.MouseButtonMiddle, Action: tea.MouseActionPress,
	}))
	if cmd == nil {
		t.Fatal("middle-click did not request PRIMARY paste")
	}
	m.Update(cmd())
	assertWorkspaceInput(t, proc, "primary-text")

	_, cmd = m.updateWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true})
	if cmd == nil {
		t.Fatal("Alt+V did not request clipboard paste")
	}
	m.Update(cmd())
	assertWorkspaceInput(t, proc, "clipboard-text")
	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("clipboard source selection = %v; want PRIMARY then CLIPBOARD", calls)
	}
}

func TestWorkspaceClipboardPasteNeverTargetsSwitchedTab(t *testing.T) {
	a := newClipboardWorkspaceProcess()
	b := newClipboardWorkspaceProcess()
	sa := workspacepkg.New("a", a, 20, 5)
	sb := workspacepkg.New("b", b, 20, 5)
	t.Cleanup(func() { _ = sa.Close(); _ = sb.Close() })
	m := New(nil)
	m.workspaceSessions = []*workspacepkg.Session{sa, sb}
	m.workspaceActive = 1
	old := workspaceClipboardRead
	workspaceClipboardRead = func(bool) (string, error) { return "keep-in-original-tab", nil }
	t.Cleanup(func() { workspaceClipboardRead = old })

	_, cmd := m.updateWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true})
	if cmd == nil {
		t.Fatal("missing paste command")
	}
	m.workspaceActive = 2
	m.Update(cmd())
	select {
	case got := <-b.writes:
		t.Fatalf("paste leaked to newly active session: %q", got)
	case got := <-a.writes:
		t.Fatalf("unexpected background paste to old session: %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWorkspacePasteErrorIsVisible(t *testing.T) {
	proc := newClipboardWorkspaceProcess()
	session := workspacepkg.New("paste", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1
	m.Update(workspacePastedMsg{id: session.ID(), err: fmt.Errorf("clipboard unavailable")})
	if !strings.Contains(m.workspaceCopyNotice, "clipboard unavailable") {
		t.Fatalf("clipboard failure is silent: %q", m.workspaceCopyNotice)
	}
}
