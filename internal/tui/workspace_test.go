package tui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

func TestDashboardTabFocusesProfileActions(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "web", Host: "web.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	m.width, m.height = 110, 32
	m.forwardIndex = map[int64][]*model.Forward{
		1: {{ID: 1, ServerID: 1, Type: model.ForwardLocal, LocalPort: 9443, RemoteAddr: "127.0.0.1", RemotePort: 9443, Enabled: true}},
	}
	m.runtime = runtimeStatus{tunnels: map[string]int{}, sessions: map[string]int{}}

	_, _ = m.updateList(tea.KeyMsg{Type: tea.KeyTab})
	if m.dashboardFocus != 1 {
		t.Fatalf("dashboardFocus = %d, want profile actions", m.dashboardFocus)
	}
	view := m.View()
	for _, want := range []string{"Actions", "Connect", "Tunnel + Connect", "Start background tunnel", "Port-forward rules"} {
		if !strings.Contains(view, want) {
			t.Fatalf("right panel missing %q:\n%s", want, view)
		}
	}

	_, _ = m.updateDetailActions(tea.KeyMsg{Type: tea.KeyTab})
	if m.dashboardFocus != 0 {
		t.Fatalf("Tab did not return focus to servers: %d", m.dashboardFocus)
	}
}

func TestProfileActionsSwitchTunnelStartToStop(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "web", Host: "web.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	m.forwardIndex = map[int64][]*model.Forward{
		1: {{ID: 1, ServerID: 1, Type: model.ForwardLocal, LocalPort: 9443, RemoteAddr: "127.0.0.1", RemotePort: 9443, Enabled: true}},
	}
	m.runtime = runtimeStatus{tunnels: map[string]int{"web": 1}, sessions: map[string]int{}}

	var ids []string
	for _, action := range m.profileActions(server) {
		ids = append(ids, action.id)
	}
	if got := strings.Join(ids, ","); !strings.Contains(got, "tunnel-stop") || strings.Contains(got, "tunnel-start") {
		t.Fatalf("running tunnel actions = %s", got)
	}
}

func TestDashboardMouseWheelMovesServerCursor(t *testing.T) {
	m := New([]*model.Server{
		{ID: 1, Alias: "a", Host: "a", Port: 22, User: "root", AuthMethod: model.AuthKey},
		{ID: 2, Alias: "b", Host: "b", Port: 22, User: "root", AuthMethod: model.AuthKey},
	})
	m.width, m.height = 100, 30
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d", m.cursor)
	}
	_, _ = m.updateDashboardMouse(tea.MouseMsg(tea.MouseEvent{
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
		X:      10, Y: 10,
	}))
	if m.cursor != 1 {
		t.Fatalf("mouse wheel cursor = %d, want 1", m.cursor)
	}
}

func TestDashboardMouseClickSelectsServerAndRightPanelFocus(t *testing.T) {
	m := New([]*model.Server{
		{ID: 1, Alias: "a", Host: "a", Port: 22, User: "root", AuthMethod: model.AuthKey},
		{ID: 2, Alias: "b", Host: "b", Port: 22, User: "root", AuthMethod: model.AuthKey},
	})
	m.width, m.height = 110, 30
	_, _ = m.updateDashboardMouse(tea.MouseMsg(tea.MouseEvent{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 10, Y: 5,
	}))
	if m.cursor != 1 || m.selectedServer().Alias != "b" {
		t.Fatalf("click selected cursor=%d server=%v", m.cursor, m.selectedServer())
	}
	_, _ = m.updateDashboardMouse(tea.MouseMsg(tea.MouseEvent{
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 90, Y: 8,
	}))
	if m.dashboardFocus != 1 {
		t.Fatalf("right-panel click focus = %d, want 1", m.dashboardFocus)
	}
}

type tuiWorkspaceFakeProcess struct {
	done    chan struct{}
	waitErr error
	once    sync.Once
}

func newTUIWorkspaceFakeProcess() *tuiWorkspaceFakeProcess {
	return &tuiWorkspaceFakeProcess{done: make(chan struct{})}
}

func (p *tuiWorkspaceFakeProcess) Read([]byte) (int, error) {
	<-p.done
	return 0, io.EOF
}
func (p *tuiWorkspaceFakeProcess) Write(b []byte) (int, error)    { return len(b), nil }
func (p *tuiWorkspaceFakeProcess) Resize(width, height int) error { return nil }
func (p *tuiWorkspaceFakeProcess) Wait() error {
	<-p.done
	return p.waitErr
}
func (p *tuiWorkspaceFakeProcess) Kill() error {
	p.once.Do(func() { close(p.done) })
	return nil
}
func (p *tuiWorkspaceFakeProcess) Close() error {
	p.once.Do(func() { close(p.done) })
	return nil
}

func TestDashboardQuitWithLiveWorkspaceAsksConfirmation(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "web", Host: "web.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("web", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })
	m.workspaceSessions = []*workspacepkg.Session{session}

	_, _ = m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.screen != screenConfirm || m.confirm == nil {
		t.Fatalf("q with live workspace did not ask for confirmation: screen=%v confirm=%v", m.screen, m.confirm)
	}
}

func TestDashboardMouseClickRunsRightPanelAction(t *testing.T) {
	server := &model.Server{ID: 1, Alias: "web", Host: "web.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	m := New([]*model.Server{server})
	m.width, m.height = 120, 34
	m.forwardIndex = map[int64][]*model.Forward{}
	m.runtime = runtimeStatus{tunnels: map[string]int{}, sessions: map[string]int{}}

	contentWidth := max(1, m.width-1)
	leftWidth := contentWidth * 62 / 100
	rightWidth := contentWidth - leftWidth - 1
	innerWidth := max(1, rightWidth-2)

	targetLine := -1
	actions := m.profileActions(server)
	targetAction := -1
	for index, action := range actions {
		if action.id == "forwards" {
			targetAction = index
			break
		}
	}
	if targetAction < 0 {
		t.Fatal("forwards action missing")
	}
	for line := 0; line < 64; line++ {
		if m.detailActionIndexAtLine(server, innerWidth, line) == targetAction {
			targetLine = line
			break
		}
	}
	if targetLine < 0 {
		t.Fatal("could not locate forwards action row")
	}

	eventY := m.dashboardBodyTop(contentWidth) + 1 + targetLine
	_, _ = m.updateDashboardMouse(tea.MouseMsg(tea.MouseEvent{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      leftWidth + 3,
		Y:      eventY,
	}))
	if m.screen != screenForwardList || m.forwardScreen == nil {
		t.Fatalf("right-panel action click did not open forwards: screen=%v", m.screen)
	}
}

func TestDashboardMouseClickMatchesGroupedVisualRow(t *testing.T) {
	m := New([]*model.Server{
		{ID: 1, Alias: "lesbest_new", Host: "a", Port: 22, User: "root", GroupName: "Clients", AuthMethod: model.AuthKey},
		{ID: 2, Alias: "test.lesbest", Host: "b", Port: 22, User: "root", GroupName: "Clients", AuthMethod: model.AuthKey},
		{ID: 3, Alias: "KP_Backup1", Host: "c", Port: 22, User: "root", GroupName: "KP", AuthMethod: model.AuthKey},
		{ID: 4, Alias: "kp_gate", Host: "d", Port: 22, User: "root", GroupName: "KP", AuthMethod: model.AuthKey},
		{ID: 5, Alias: "KP_Mail", Host: "e", Port: 22, User: "root", GroupName: "KP", AuthMethod: model.AuthKey},
	})
	m.width, m.height = 140, 34
	m.sortMode = sortByGroup
	m.rebuildServerRows("")

	target := -1
	for index, row := range m.rows {
		if row.server != nil && row.server.Alias == "kp_gate" {
			target = index
			break
		}
	}
	if target < 0 {
		t.Fatal("kp_gate row missing")
	}
	contentWidth := max(1, m.width-1)
	visualY := m.dashboardBodyTop(contentWidth) + 2 + target
	_, _ = m.updateDashboardMouse(tea.MouseMsg(tea.MouseEvent{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      10,
		Y:      visualY,
	}))
	if got := m.selectedServer(); got == nil || got.Alias != "kp_gate" {
		t.Fatalf("click on kp_gate visual row selected %#v", got)
	}
}

func TestWorkspaceAltArrowsCycleTabs(t *testing.T) {
	m := New(nil)
	m.workspaceSessions = []*workspacepkg.Session{nil, nil}
	m.workspaceActive = 0

	if !m.workspaceShortcut(tea.KeyMsg{Type: tea.KeyRight, Alt: true}) || m.workspaceActive != 1 {
		t.Fatalf("Alt+Right active=%d, want 1", m.workspaceActive)
	}
	if !m.workspaceShortcut(tea.KeyMsg{Type: tea.KeyRight, Alt: true}) || m.workspaceActive != 2 {
		t.Fatalf("second Alt+Right active=%d, want 2", m.workspaceActive)
	}
	if !m.workspaceShortcut(tea.KeyMsg{Type: tea.KeyLeft, Alt: true}) || m.workspaceActive != 1 {
		t.Fatalf("Alt+Left active=%d, want 1", m.workspaceActive)
	}
}

func TestWorkspaceSessionUpdateCmdWaitsForRealSessionEvent(t *testing.T) {
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("idle", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })

	cmd := workspaceSessionUpdateCmd(session)
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()

	select {
	case msg := <-result:
		t.Fatalf("idle workspace produced a synthetic refresh: %#v", msg)
	case <-time.After(120 * time.Millisecond):
	}

	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-result:
		update, ok := msg.(workspaceSessionUpdateMsg)
		if !ok || update.id != session.ID() || !update.done {
			t.Fatalf("exit update = %#v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("workspace watcher did not wake when session exited")
	}
}

func TestThreeIdleWorkspaceWatchersStaySilent(t *testing.T) {
	result := make(chan tea.Msg, 3)
	var sessions []*workspacepkg.Session
	for _, alias := range []string{"one", "two", "three"} {
		proc := newTUIWorkspaceFakeProcess()
		session := workspacepkg.New(alias, proc, 80, 24)
		sessions = append(sessions, session)
		cmd := workspaceSessionUpdateCmd(session)
		go func() { result <- cmd() }()
	}
	t.Cleanup(func() {
		for _, session := range sessions {
			_ = session.Close()
		}
	})

	select {
	case msg := <-result:
		t.Fatalf("idle workspace watcher produced a refresh: %#v", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWorkspaceExitZeroAutoClosesTabAndPreservesFocus(t *testing.T) {
	m := New(nil)
	a := newTUIWorkspaceFakeProcess()
	b := newTUIWorkspaceFakeProcess()
	sa := workspacepkg.New("a", a, 80, 24)
	sb := workspacepkg.New("b", b, 80, 24)
	t.Cleanup(func() { _ = sa.Close(); _ = sb.Close() })
	m.workspaceSessions = []*workspacepkg.Session{sa, sb}
	m.workspaceActive = 2
	a.once.Do(func() { close(a.done) })
	select {
	case <-sa.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("session did not exit")
	}
	m.Update(workspaceSessionUpdateMsg{id: sa.ID(), done: true})
	if len(m.workspaceSessions) != 1 || m.workspaceSessions[0] != sb || m.workspaceActive != 1 {
		t.Fatalf("successful background tab exit left tabs=%d active=%d", len(m.workspaceSessions), m.workspaceActive)
	}
	b.once.Do(func() { close(b.done) })
	<-sb.Done()
	m.Update(workspaceSessionUpdateMsg{id: sb.ID(), done: true})
	if len(m.workspaceSessions) != 0 || m.workspaceActive != 0 {
		t.Fatalf("last successful exit left active=%d tabs=%d", m.workspaceActive, len(m.workspaceSessions))
	}
}
func TestWorkspaceExitErrorKeepsOutputForInspection(t *testing.T) {
	m := New(nil)
	proc := newTUIWorkspaceFakeProcess()
	proc.waitErr = fmt.Errorf("connection lost")
	session := workspacepkg.New("bad", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1
	proc.once.Do(func() { close(proc.done) })
	<-session.Done()
	m.Update(workspaceSessionUpdateMsg{id: session.ID(), done: true})
	if len(m.workspaceSessions) != 1 || m.workspaceActive != 1 {
		t.Fatal("failed session was closed instead of keeping output")
	}
}

// Synthetic SGR mouse events exercise the actual Bubble Tea workspace routing,
// not just the low-level terminal selection model.
func TestWorkspaceMultiClickAndScrollTimers(t *testing.T) {
	// Windows timers can report identical timestamps for consecutive clicks.
	originalNow := workspaceMouseNow
	instant := time.Now()
	workspaceMouseNow = func() time.Time { return instant }
	t.Cleanup(func() { workspaceMouseNow = originalNow })
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("test", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1
	press := func(x, y int) {
		_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}))
	}
	release := func(x, y int) {
		_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease}))
	}
	press(3, 2)
	release(3, 2)
	if m.workspaceClickCount != 1 {
		t.Fatalf("one click count %d", m.workspaceClickCount)
	}
	press(3, 2)
	release(3, 2)
	if m.workspaceClickCount != 2 {
		t.Fatalf("double click count %d", m.workspaceClickCount)
	}
	press(3, 2)
	release(3, 2)
	if m.workspaceClickCount != 3 {
		t.Fatalf("triple click count %d", m.workspaceClickCount)
	}
	press(3, 2)
	if m.workspaceClickCount != 1 {
		t.Fatalf("fourth click should reset, got %d", m.workspaceClickCount)
	}
	// A drag to the upper tab-strip must not activate a tab.
	_, cmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}))
	if cmd == nil || m.workspaceScrollDir != 1 || m.workspaceActive != 1 {
		t.Fatalf("drag out of viewport failed, dir=%d", m.workspaceScrollDir)
	}
	oldEpoch := m.workspaceScrollEpoch
	release(0, 0)
	if m.workspaceScrollDir != 0 || m.workspaceScrollEpoch == oldEpoch {
		t.Fatal("release left autoscroll enabled")
	}
	_, next := m.handleWorkspaceAutoScroll(workspaceAutoScrollMsg{id: session.ID(), epoch: oldEpoch})
	if next != nil {
		t.Fatal("stale autoscroll timer restarted after release")
	}
}
func TestWorkspaceMouseWheelDoesNotClearSelection(t *testing.T) {
	proc := newTUIWorkspaceFakeProcess()
	session := workspacepkg.New("wheel", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1
	m.workspaceSelecting = true
	_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{X: 1, Y: 3, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress}))
	if !m.workspaceSelecting {
		t.Fatal("wheel cancelled in-progress selection")
	}
	m.workspaceSelecting = false
	_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{X: 1, Y: 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}))
	if m.workspaceScrollDir != 0 {
		t.Fatal("wheel triggered auto scroll")
	}
}

// feedableWorkspaceProcess lets the UI tests exercise scrollback and clipboard
// end-to-end without opening a real SSH connection.
type feedableWorkspaceProcess struct {
	*tuiWorkspaceFakeProcess
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newFeedableWorkspaceProcess() *feedableWorkspaceProcess {
	r, w := io.Pipe()
	return &feedableWorkspaceProcess{tuiWorkspaceFakeProcess: newTUIWorkspaceFakeProcess(), reader: r, writer: w}
}
func (p *feedableWorkspaceProcess) Read(b []byte) (int, error) { return p.reader.Read(b) }
func (p *feedableWorkspaceProcess) Kill() error {
	_ = p.writer.Close()
	_ = p.reader.Close()
	return p.tuiWorkspaceFakeProcess.Kill()
}
func (p *feedableWorkspaceProcess) Close() error {
	_ = p.writer.Close()
	_ = p.reader.Close()
	return p.tuiWorkspaceFakeProcess.Close()
}

func TestWorkspaceDragAutoScrollCopiesBeyondVisibleScreen(t *testing.T) {
	proc := newFeedableWorkspaceProcess()
	session := workspacepkg.New("scroll", proc, 20, 5)
	t.Cleanup(func() { _ = session.Close() })
	m := New(nil)
	m.width, m.height = 20, 7
	m.workspaceSessions = []*workspacepkg.Session{session}
	m.workspaceActive = 1
	for i := 0; i < 18; i++ {
		_, _ = fmt.Fprintf(proc.writer, "row%02d\r\n", i)
	}
	deadline := time.Now().Add(2 * time.Second)
	for session.ScrollOffset() != 0 || !strings.Contains(session.Render(), "row17") {
		if time.Now().After(deadline) {
			t.Fatal("terminal output was not rendered")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 1, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}))
	_, firstCmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}))
	if firstCmd == nil {
		t.Fatal("no auto-scroll timer scheduled at viewport edge")
	}
	firstOffset := session.ScrollOffset()
	if firstOffset == 0 {
		t.Fatal("no initial scroll at top edge")
	}
	token := m.workspaceScrollEpoch
	for i := 0; i < 8; i++ {
		_, next := m.handleWorkspaceAutoScroll(workspaceAutoScrollMsg{id: session.ID(), epoch: token})
		if next == nil {
			t.Fatal("auto-scroll unexpectedly stopped")
		}
	}
	if got := session.ScrollOffset(); got <= firstOffset {
		t.Fatalf("timer failed to move viewport: first=%d now=%d", firstOffset, got)
	}
	_, primaryCmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: 0, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease}))
	if primaryCmd == nil {
		t.Fatal("no PRIMARY update after drag beyond viewport")
	}
	copied := ""
	original := workspaceClipboardWrite
	workspaceClipboardWrite = func(value string) error { copied = value; return nil }
	defer func() { workspaceClipboardWrite = original }()
	_ = primaryCmd()
	if copied != "" {
		t.Fatalf("drag unexpectedly overwrote CLIPBOARD: %q", copied)
	}
	_, copyCmd := m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 1, Y: m.height - 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}))
	if copyCmd == nil {
		t.Fatal("no command from explicit Copy button after drag")
	}
	if _, ok := copyCmd().(workspaceCopiedMsg); !ok {
		t.Fatal("explicit Copy button did not return copied status")
	}
	if !strings.Contains(copied, "row") || strings.Count(copied, "\n") < 5 {
		t.Fatalf("drag did not include scrolled text: %q", copied)
	}
	previousOffset := session.ScrollOffset()
	_, _ = m.updateWorkspaceMouse(tea.MouseMsg(tea.MouseEvent{
		X: 5, Y: 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}))
	if session.ScrollOffset() >= previousOffset {
		t.Fatal("wheel did not scroll the viewport after mouse selection")
	}
}
