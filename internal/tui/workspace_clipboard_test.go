package tui

import (
	"fmt"
	"io"
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

func TestWorkspaceMouseSelectionCopiesWithoutShortcut(t *testing.T) {
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
	old := workspaceClipboardWrite
	var copied string
	workspaceClipboardWrite = func(text string) error { copied = text; return nil }
	t.Cleanup(func() { workspaceClipboardWrite = old })

	_, cmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if cmd != nil {
		t.Fatal("mouse press should not copy")
	}
	_, cmd = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 4, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease,
	}))
	if cmd == nil {
		t.Fatal("releasing a mouse selection did not request clipboard copy")
	}
	result, ok := cmd().(workspaceCopiedMsg)
	if !ok || result.err != nil || copied != "alpha" {
		t.Fatalf("mouse release copied %q, message %#v", copied, result)
	}
	if got := session.SelectedText(); got != "alpha" {
		t.Fatalf("copy discarded highlight: %q", got)
	}
}

func TestWorkspaceAltCVKeysReachRemotePTY(t *testing.T) {
	proc := newClipboardWorkspaceProcess()
	session := workspacepkg.New("key-forward", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	for _, r := range []rune{'c', 'v'} {
		_, cmd := m.updateWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true})
		if cmd != nil {
			t.Fatalf("Alt+%c was intercepted by SSHKeeper", r)
		}
		assertWorkspaceInput(t, proc, string([]rune{'\x1b', r}))
	}
}

func TestWorkspacePasteFromBracketedInputAndPrimary(t *testing.T) {
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
		if !primary {
			t.Fatal("middle-click should read PRIMARY, not CLIPBOARD")
		}
		return "primary-text", nil
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
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("clipboard source selection = %v; want PRIMARY only", calls)
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
	m.width, m.height = 20, 7
	old := workspaceClipboardRead
	workspaceClipboardRead = func(bool) (string, error) { return "keep-in-original-tab", nil }
	t.Cleanup(func() { workspaceClipboardRead = old })

	_, cmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 2, Y: 3, Button: tea.MouseButtonMiddle, Action: tea.MouseActionPress,
	}))
	if cmd == nil {
		t.Fatal("missing PRIMARY paste command")
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

// A real Bubble Tea program must decode the bracketed-paste bytes emitted by
// the outer terminal, then forward them to the remote PTY. A synthesized
// KeyMsg alone wouldn't exercise the terminal input parser.
type workspacePasteInputModel struct {
	workspace *tuiModel
	seen      chan tea.KeyMsg
}

func (m workspacePasteInputModel) Init() tea.Cmd { return nil }

func (m workspacePasteInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		m.seen <- key
		_, _ = m.workspace.updateWorkspaceKey(key)
		return m, tea.Quit
	}
	return m, nil
}

func (m workspacePasteInputModel) View() string { return "" }

func TestWorkspaceBracketedPasteThroughBubbleTeaInput(t *testing.T) {
	proc := newClipboardWorkspaceProcess()
	session := workspacepkg.New("real-paste", proc, 40, 10)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 40, 12
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1

	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	seen := make(chan tea.KeyMsg, 1)
	program := tea.NewProgram(workspacePasteInputModel{workspace: m, seen: seen},
		tea.WithInput(reader), tea.WithOutput(io.Discard))
	finished := make(chan error, 1)
	go func() {
		_, err := program.Run()
		finished <- err
	}()
	const pasted = "hello\nsecond строка"
	_, err := io.WriteString(writer, "\x1b[200~"+pasted+"\x1b[201~")
	if err != nil {
		t.Fatalf("send terminal paste: %v", err)
	}
	select {
	case key := <-seen:
		if !key.Paste || string(key.Runes) != pasted {
			t.Fatalf("Bubble Tea parsed clipboard paste as %#v", key)
		}
	case <-time.After(3 * time.Second):
		program.Kill()
		t.Fatal("Bubble Tea did not decode bracketed paste")
	}
	assertWorkspaceInput(t, proc, pasted)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Bubble Tea program failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		program.Kill()
		t.Fatal("Bubble Tea program did not exit")
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
