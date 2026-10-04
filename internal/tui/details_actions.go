package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

type profileAction struct {
	id    string
	label string
	hint  string
}

func (m *tuiModel) profileActions(server *model.Server) []profileAction {
	if server == nil {
		return nil
	}
	actions := []profileAction{{
		id:    "connect",
		label: i18n.T("Connect", "Подключиться"),
		hint:  i18n.T("open SSH workspace tab", "открыть SSH-вкладку"),
	}}
	forwards := m.forwardIndex[server.ID]
	enabled := enabledForwardCount(forwards)
	running := m.runtime.tunnels[server.Alias] > 0
	if enabled > 0 || running {
		actions = append(actions, profileAction{
			id:    "tunnel-connect",
			label: i18n.T("Tunnel + Connect", "Туннель + подключение"),
			hint:  i18n.T("keep tunnel in background and open SSH tab", "оставить туннель в фоне и открыть SSH-вкладку"),
		})
		toggle := profileAction{
			id:    "tunnel-start",
			label: i18n.T("Start background tunnel", "Запустить фоновый туннель"),
			hint:  i18n.T("run enabled forwards without leaving sshkeeper", "запустить включённые пробросы, не покидая sshkeeper"),
		}
		if running {
			toggle.id = "tunnel-stop"
			toggle.label = i18n.T("Stop background tunnel", "Остановить фоновый туннель")
			toggle.hint = i18n.T("SSH workspace tabs stay open", "SSH-вкладки останутся открыты")
		}
		actions = append(actions, toggle)
	}
	actions = append(actions, profileAction{
		id:    "forwards",
		label: i18n.T("Port-forward rules", "Правила проброса"),
		hint:  i18n.T("add, edit, enable or disable forwards", "добавить, изменить, включить или выключить пробросы"),
	})
	return actions
}

// detailActionIndexAtLine maps an inner right-panel row to a profile action.
// It derives the hit row from the same rendered detail lines used by the
// dashboard so mouse hit-testing follows layout changes automatically.
func (m *tuiModel) detailActionIndexAtLine(server *model.Server, width, line int) int {
	if server == nil || line < 0 {
		return -1
	}
	actions := m.profileActions(server)
	if len(actions) == 0 {
		return -1
	}
	lines := m.serverDetailLines(server, width)
	if line >= len(lines) {
		return -1
	}
	text := lines[line]
	for index, action := range actions {
		if strings.Contains(text, action.label) {
			return index
		}
	}
	return -1
}

func (m *tuiModel) clampDetailAction(server *model.Server) {
	actions := m.profileActions(server)
	if len(actions) == 0 {
		m.detailAction = 0
		return
	}
	if m.detailAction < 0 {
		m.detailAction = len(actions) - 1
	}
	if m.detailAction >= len(actions) {
		m.detailAction = 0
	}
}

func (m *tuiModel) detailActionLines(server *model.Server, width int) []string {
	actions := m.profileActions(server)
	if len(actions) == 0 {
		return nil
	}
	m.clampDetailAction(server)
	lines := []string{"", dashboardSection(i18n.T("Actions", "Действия"))}
	for index, action := range actions {
		marker := "  "
		label := normalStyle.Render(action.label)
		if m.dashboardFocus == 1 && index == m.detailAction {
			marker = cursorStyle.Render(glyphs.cursor) + " "
			label = selectedRowStyle.Render(action.label)
		}
		line := marker + label
		if width >= 46 && action.hint != "" {
			line += mutedStyle.Render("  " + action.hint)
		}
		lines = append(lines, fitLine(line, max(1, width)))
	}
	return lines
}

func (m *tuiModel) updateDetailActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	server := m.selectedServer()
	if server == nil {
		m.dashboardFocus = 0
		m.detailAction = 0
		return m, nil
	}
	m.clampDetailAction(server)
	actions := m.profileActions(server)
	switch msg.Type {
	case tea.KeyEsc, tea.KeyTab, tea.KeyShiftTab:
		m.dashboardFocus = 0
		return m, nil
	case tea.KeyUp:
		m.detailAction--
		m.clampDetailAction(server)
		return m, nil
	case tea.KeyDown:
		m.detailAction++
		m.clampDetailAction(server)
		return m, nil
	case tea.KeyEnter:
		if len(actions) == 0 {
			return m, nil
		}
		return m.runDetailAction(server, actions[m.detailAction])
	case tea.KeyRunes:
		switch msg.String() {
		case "k", "K":
			m.detailAction--
			m.clampDetailAction(server)
			return m, nil
		case "j", "J":
			m.detailAction++
			m.clampDetailAction(server)
			return m, nil
		}
	}
	return m, nil
}

func (m *tuiModel) runDetailAction(server *model.Server, action profileAction) (tea.Model, tea.Cmd) {
	switch action.id {
	case "connect":
		return m, func() tea.Msg { return connectRequestMsg{server: server} }
	case "tunnel-connect":
		if m.runtime.tunnels[server.Alias] > 0 {
			return m.openWorkspaceSession(server.Alias)
		}
		m.pendingTunnelConnect = server.Alias
		return m.beginBackgroundTunnel(server, screenList)
	case "tunnel-start":
		return m.beginBackgroundTunnel(server, screenList)
	case "tunnel-stop":
		alias := server.Alias
		if StopBackgroundTunnels == nil {
			return m, func() tea.Msg {
				return backgroundTunnelStoppedMsg{alias: alias, err: fmt.Errorf("%s", i18n.T("tunnel stop is unavailable", "остановка туннеля недоступна"))}
			}
		}
		m.success = i18n.Tf("Stopping tunnel for %s...", "Остановка туннеля для %s...", alias)
		return m, func() tea.Msg {
			return backgroundTunnelStoppedMsg{alias: alias, err: StopBackgroundTunnels(alias)}
		}
	case "forwards":
		m.forwardScreen = newForwardScreenModel(server.ID, server.Alias, m.width, m.height)
		m.screen = screenForwardList
		m.dashboardFocus = 0
		return m, m.forwardScreen.loadForwards()
	}
	return m, nil
}
