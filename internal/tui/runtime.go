package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
	sessionpkg "github.com/mirivlad/sshkeeper/internal/session"
	"github.com/mirivlad/sshkeeper/internal/tunnel"
)

// runtimeRefreshInterval is how often the dashboard re-reads running tunnels
// and tmux sessions for the per-server indicators.
const runtimeRefreshInterval = 10 * time.Second

// maxParallelTests bounds concurrent connection tests started by "test all".
const maxParallelTests = 6

// runtimeStatus counts live tunnel processes and tmux session windows per
// profile alias.
type runtimeStatus struct {
	tunnels  map[string]int
	sessions map[string]int
}

type runtimeLoadedMsg struct {
	status runtimeStatus
}

type runtimeTickMsg struct{}

// serverTestedMsg is the result of one connection test started from the list.
// It carries the alias so the result lands on the tested profile even if the
// cursor has moved.
type serverTestedMsg struct {
	alias string
	ok    bool
	err   string
}

// loadRuntimeStatus reads running tunnels and, when tmux is available, the
// sshkeeper session windows. Tests replace it.
var loadRuntimeStatus = func(sessions bool) runtimeStatus {
	status := runtimeStatus{tunnels: map[string]int{}, sessions: map[string]int{}}
	_ = tunnel.Reload()
	for _, state := range tunnel.List() {
		if state != nil && tunnel.IsRunning(state.ID) {
			status.tunnels[state.ServerAlias]++
		}
	}
	if sessions {
		if windows, err := sessionpkg.List(); err == nil {
			for _, window := range windows {
				status.sessions[window.ServerAlias]++
			}
		}
	}
	return status
}

func (m *tuiModel) loadRuntimeCmd() tea.Cmd {
	sessions := m.sessionsAvailable
	return func() tea.Msg {
		return runtimeLoadedMsg{status: loadRuntimeStatus(sessions)}
	}
}

func runtimeTickCmd() tea.Cmd {
	return tea.Tick(runtimeRefreshInterval, func(time.Time) tea.Msg { return runtimeTickMsg{} })
}

// testServersCmd tests each profile, at most maxParallelTests at a time, and
// marks them as in progress until their serverTestedMsg arrives.
func (m *tuiModel) testServersCmd(servers []*model.Server) tea.Cmd {
	if TestConnection == nil || len(servers) == 0 {
		return nil
	}
	if m.testing == nil {
		m.testing = map[string]bool{}
	}
	slots := make(chan struct{}, maxParallelTests)
	cmds := make([]tea.Cmd, 0, len(servers))
	for _, server := range servers {
		if m.testing[server.Alias] {
			continue
		}
		m.testing[server.Alias] = true
		m.testTotal++
		server := server
		cmds = append(cmds, func() tea.Msg {
			slots <- struct{}{}
			defer func() { <-slots }()
			ok, testErr := TestConnection(server)
			return serverTestedMsg{alias: server.Alias, ok: ok, err: testErr}
		})
	}
	return tea.Batch(cmds...)
}

// applyServerTest records one test result in the database and in memory, and
// summarizes the batch once the last test finishes.
func (m *tuiModel) applyServerTest(msg serverTestedMsg) {
	delete(m.testing, msg.alias)
	status := model.TestFailed
	if msg.ok {
		status = model.TestOK
		m.testPassed++
	}
	if UpdateTestResult != nil {
		_ = UpdateTestResult(msg.alias, status, msg.err)
	}
	if server := m.serverByAlias(msg.alias); server != nil {
		tested := now()
		server.LastTestStatus = status
		server.LastTestError = msg.err
		server.LastTestAt = &tested
	}
	if len(m.testing) > 0 {
		return
	}
	total, passed := m.testTotal, m.testPassed
	m.testTotal, m.testPassed = 0, 0
	switch {
	case total == 1 && passed == 1:
		m.err = nil
		m.success = i18n.Tf("%s: connection OK.", "%s: соединение установлено.", msg.alias)
	case total == 1:
		m.success = ""
		m.err = errorNotice(i18n.Tf("%s: connection failed: %s", "%s: ошибка соединения: %s", msg.alias, msg.err))
	case passed == total:
		m.err = nil
		m.success = i18n.Tf("Tested %d servers: all OK.", "Проверено серверов: %d, все доступны.", total)
	default:
		m.err, m.success = nil, ""
		m.warning = i18n.Tf("Tested %d servers: %d OK, %d failed.", "Проверено серверов: %d; доступны: %d, с ошибкой: %d.", total, passed, total-passed)
	}
}

// testTargets is what "test all" checks: the marked profiles when there are
// any, otherwise every visible profile.
func (m *tuiModel) testTargets() []*model.Server {
	if selected := m.selectedServers(); len(selected) > 0 {
		return selected
	}
	servers := make([]*model.Server, 0, len(m.rows))
	for _, row := range m.rows {
		if row.server != nil {
			servers = append(servers, row.server)
		}
	}
	return servers
}
