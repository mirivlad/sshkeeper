package tui

import (
	"io"
	"strings"
	"sync"
	"testing"

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
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 10, Y: 4,
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
	done chan struct{}
	once sync.Once
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
	return nil
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
