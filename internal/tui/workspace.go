package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

const workspaceRefreshInterval = 50 * time.Millisecond

type workspaceOpenedMsg struct {
	session *workspacepkg.Session
	err     error
}

type workspaceTickMsg struct{}

func workspaceTickCmd() tea.Cmd {
	return tea.Tick(workspaceRefreshInterval, func(time.Time) tea.Msg { return workspaceTickMsg{} })
}

func (m *tuiModel) workspaceTerminalSize() (int, int) {
	width := max(20, m.width)
	// One row for tabs and one for sshkeeper's workspace help/status line.
	height := max(5, m.height-2)
	return width, height
}

func (m *tuiModel) activeWorkspaceSession() *workspacepkg.Session {
	index := m.workspaceActive - 1
	if index < 0 || index >= len(m.workspaceSessions) {
		return nil
	}
	return m.workspaceSessions[index]
}

func (m *tuiModel) workspaceSessionCount(alias string) int {
	count := 0
	for _, session := range m.workspaceSessions {
		if session == nil || session.Alias() != alias {
			continue
		}
		if exited, _ := session.Exited(); !exited {
			count++
		}
	}
	return count
}

func (m *tuiModel) workspaceIndexByAlias(alias string) int {
	for index, session := range m.workspaceSessions {
		if session == nil || session.Alias() != alias {
			continue
		}
		exited, _ := session.Exited()
		if !exited {
			return index
		}
	}
	return -1
}

func (m *tuiModel) openWorkspaceSession(serverAlias string) (tea.Model, tea.Cmd) {
	if index := m.workspaceIndexByAlias(serverAlias); index >= 0 {
		m.workspaceActive = index + 1
		if session := m.activeWorkspaceSession(); session != nil {
			width, height := m.workspaceTerminalSize()
			_ = session.Resize(width, height)
		}
		return m, workspaceTickCmd()
	}
	if OpenWorkspaceSession == nil {
		return m, func() tea.Msg {
			return workspaceOpenedMsg{err: fmt.Errorf("%s", i18n.T("workspace sessions are unavailable", "сессии рабочего пространства недоступны"))}
		}
	}
	width, height := m.workspaceTerminalSize()
	return m, func() tea.Msg {
		session, err := OpenWorkspaceSession(serverAlias, width, height)
		return workspaceOpenedMsg{session: session, err: err}
	}
}

func (m *tuiModel) handleWorkspaceOpened(msg workspaceOpenedMsg) tea.Cmd {
	if msg.err != nil {
		m.err = msg.err
		m.workspaceActive = 0
		return nil
	}
	if msg.session == nil {
		m.err = fmt.Errorf("%s", i18n.T("session launcher returned no session", "запуск сессии не вернул сессию"))
		m.workspaceActive = 0
		return nil
	}
	m.workspaceSessions = append(m.workspaceSessions, msg.session)
	m.workspaceActive = len(m.workspaceSessions)
	m.err = nil
	return workspaceTickCmd()
}

func (m *tuiModel) closeActiveWorkspaceSession() {
	index := m.workspaceActive - 1
	if index < 0 || index >= len(m.workspaceSessions) {
		m.workspaceActive = 0
		return
	}
	session := m.workspaceSessions[index]
	if session != nil {
		_ = session.Close()
	}
	m.workspaceSessions = append(m.workspaceSessions[:index], m.workspaceSessions[index+1:]...)
	if len(m.workspaceSessions) == 0 {
		m.workspaceActive = 0
		return
	}
	if index >= len(m.workspaceSessions) {
		index = len(m.workspaceSessions) - 1
	}
	m.workspaceActive = index + 1
}

func (m *tuiModel) closeWorkspaceSessions() {
	for _, session := range m.workspaceSessions {
		if session != nil {
			_ = session.Close()
		}
	}
	m.workspaceSessions = nil
	m.workspaceActive = 0
}

// CloseWorkspaceSessions terminates all embedded SSH sessions. It is called on
// a real application exit; temporary TUI restarts preserve the sessions in
// State instead.
func (m *tuiModel) CloseWorkspaceSessions() {
	m.closeWorkspaceSessions()
}

func (m *tuiModel) workspaceNext(delta int) {
	total := len(m.workspaceSessions) + 1 // dashboard + sessions
	if total <= 1 {
		m.workspaceActive = 0
		return
	}
	next := m.workspaceActive + delta
	for next < 0 {
		next += total
	}
	next %= total
	m.workspaceActive = next
	if session := m.activeWorkspaceSession(); session != nil {
		width, height := m.workspaceTerminalSize()
		_ = session.Resize(width, height)
	}
}

func (m *tuiModel) workspaceShortcut(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyCtrlPgUp:
		m.workspaceNext(-1)
		return true
	case tea.KeyCtrlPgDown:
		m.workspaceNext(1)
		return true
	}
	if msg.Type == tea.KeyRunes && msg.Alt && len(msg.Runes) == 1 {
		switch r := msg.Runes[0]; {
		case r == '0':
			m.workspaceActive = 0
			return true
		case r >= '1' && r <= '9':
			index := int(r - '1')
			if index < len(m.workspaceSessions) {
				m.workspaceActive = index + 1
				if session := m.activeWorkspaceSession(); session != nil {
					width, height := m.workspaceTerminalSize()
					_ = session.Resize(width, height)
				}
			}
			return true
		case r == 'w' || r == 'W':
			if m.workspaceActive > 0 {
				m.closeActiveWorkspaceSession()
			}
			return true
		}
	}
	return false
}

func (m *tuiModel) updateWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.workspaceShortcut(msg) {
		return m, workspaceTickCmd()
	}
	session := m.activeWorkspaceSession()
	if session == nil {
		m.workspaceActive = 0
		return m, nil
	}
	if msg.Paste && msg.Type == tea.KeyRunes {
		session.Paste(string(msg.Runes))
		return m, workspaceTickCmd()
	}
	if msg.Type == tea.KeyRunes {
		mod := vt.KeyMod(0)
		if msg.Alt {
			mod |= vt.ModAlt
		}
		for _, r := range msg.Runes {
			session.SendKey(vt.KeyPressEvent{Code: r, Mod: mod})
		}
		return m, workspaceTickCmd()
	}
	if event, ok := workspaceVTKey(msg); ok {
		session.SendKey(event)
	}
	return m, workspaceTickCmd()
}

func workspaceVTKey(msg tea.KeyMsg) (vt.KeyPressEvent, bool) {
	mod := vt.KeyMod(0)
	if msg.Alt {
		mod |= vt.ModAlt
	}
	plain := func(code rune) (vt.KeyPressEvent, bool) {
		return vt.KeyPressEvent{Code: code, Mod: mod}, true
	}
	ctrl := func(code rune) (vt.KeyPressEvent, bool) {
		return vt.KeyPressEvent{Code: code, Mod: mod | vt.ModCtrl}, true
	}

	if msg.Type >= tea.KeyCtrlA && msg.Type <= tea.KeyCtrlZ {
		return ctrl(rune('a' + int(msg.Type-tea.KeyCtrlA)))
	}
	switch msg.Type {
	case tea.KeyCtrlAt:
		return ctrl(vt.KeySpace)
	case tea.KeyCtrlBackslash:
		return ctrl('\\')
	case tea.KeyCtrlCloseBracket:
		return ctrl(']')
	case tea.KeyCtrlCaret:
		return ctrl('^')
	case tea.KeyCtrlUnderscore:
		return ctrl('_')
	case tea.KeyEnter:
		return plain(vt.KeyEnter)
	case tea.KeyTab:
		return plain(vt.KeyTab)
	case tea.KeyShiftTab:
		return vt.KeyPressEvent{Code: vt.KeyTab, Mod: mod | vt.ModShift}, true
	case tea.KeyBackspace:
		return plain(vt.KeyBackspace)
	case tea.KeyEsc:
		return plain(vt.KeyEscape)
	case tea.KeySpace:
		return plain(vt.KeySpace)
	case tea.KeyUp:
		return plain(vt.KeyUp)
	case tea.KeyDown:
		return plain(vt.KeyDown)
	case tea.KeyLeft:
		return plain(vt.KeyLeft)
	case tea.KeyRight:
		return plain(vt.KeyRight)
	case tea.KeyHome:
		return plain(vt.KeyHome)
	case tea.KeyEnd:
		return plain(vt.KeyEnd)
	case tea.KeyPgUp:
		return plain(vt.KeyPgUp)
	case tea.KeyPgDown:
		return plain(vt.KeyPgDown)
	case tea.KeyInsert:
		return plain(vt.KeyInsert)
	case tea.KeyDelete:
		return plain(vt.KeyDelete)
	case tea.KeyCtrlUp:
		return vt.KeyPressEvent{Code: vt.KeyUp, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlDown:
		return vt.KeyPressEvent{Code: vt.KeyDown, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlLeft:
		return vt.KeyPressEvent{Code: vt.KeyLeft, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlRight:
		return vt.KeyPressEvent{Code: vt.KeyRight, Mod: mod | vt.ModCtrl}, true
	case tea.KeyShiftUp:
		return vt.KeyPressEvent{Code: vt.KeyUp, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftDown:
		return vt.KeyPressEvent{Code: vt.KeyDown, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftLeft:
		return vt.KeyPressEvent{Code: vt.KeyLeft, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftRight:
		return vt.KeyPressEvent{Code: vt.KeyRight, Mod: mod | vt.ModShift}, true
	}
	if msg.Type >= tea.KeyF1 && msg.Type <= tea.KeyF20 {
		code := vt.KeyF1 + rune(msg.Type-tea.KeyF1)
		return plain(code)
	}
	return vt.KeyPressEvent{}, false
}

func (m *tuiModel) workspaceTabPlainLabels() []string {
	labels := []string{i18n.T("Servers", "Серверы")}
	for _, session := range m.workspaceSessions {
		if session == nil {
			continue
		}
		label := session.Alias()
		if exited, _ := session.Exited(); exited {
			label += " ×"
		} else {
			label += " ●"
		}
		labels = append(labels, label)
	}
	return labels
}

func (m *tuiModel) renderWorkspaceTabs(width int) string {
	labels := m.workspaceTabPlainLabels()
	parts := make([]string, 0, len(labels))
	for index, label := range labels {
		text := "[" + label + "]"
		if index == m.workspaceActive {
			parts = append(parts, brandStyle.Copy().Bold(true).Render(text))
		} else {
			parts = append(parts, mutedStyle.Render(text))
		}
	}
	return fitLine(strings.Join(parts, " "), width)
}

func (m *tuiModel) workspaceTabAtX(x int) int {
	if x < 0 {
		return -1
	}
	pos := 0
	for index, label := range m.workspaceTabPlainLabels() {
		width := lineWidth("[" + label + "]")
		if x >= pos && x < pos+width {
			return index
		}
		pos += width + 1
	}
	return -1
}

func (m *tuiModel) viewWorkspaceSession() string {
	session := m.activeWorkspaceSession()
	if session == nil {
		m.workspaceActive = 0
		return m.viewServerList()
	}
	width := max(1, m.width-1)
	height := max(1, m.height)
	tabs := m.renderWorkspaceTabs(width)

	exited, exitErr := session.Exited()
	status := i18n.Tf("%s · running %s", "%s · работает %s", session.Alias(), time.Since(session.StartedAt()).Round(time.Second))
	if exited {
		status = i18n.Tf("%s · session ended", "%s · сессия завершена", session.Alias())
		if exitErr != nil {
			status += ": " + firstLine(exitErr.Error())
		}
	}
	help := renderHelpLine([]helpItem{
		{Key: "Ctrl+PgUp/PgDn", Action: i18n.T("switch tab", "сменить вкладку")},
		{Key: "Alt+0", Action: i18n.T("servers", "серверы")},
		{Key: "Alt+W", Action: i18n.T("close session", "закрыть сессию")},
	})
	statusLine := fitLine(status+"  "+help, width)

	bodyHeight := max(1, height-2)
	_ = session.Resize(width, bodyHeight)
	body := session.Render()
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return tabs + "\n" + strings.Join(lines, "\n") + "\n" + statusLine
}

func (m *tuiModel) updateWorkspaceMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == 0 {
		if tab := m.workspaceTabAtX(event.X); tab >= 0 {
			m.workspaceActive = tab
			return m, workspaceTickCmd()
		}
	}
	return m, nil
}

func (m *tuiModel) dashboardBodyTop(width int) int {
	header := m.renderDashboardHeader(width)
	if len(m.workspaceSessions) > 0 {
		header = m.renderWorkspaceTabs(width) + "\n" + header
	}
	return displayLineCount(header) + displayLineCount(m.renderDashboardNotification(width))
}

func (m *tuiModel) updateDashboardMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	if m.screen != screenList && m.screen != screenSearch {
		return m, nil
	}
	if event.Y == 0 && event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft {
		if tab := m.workspaceTabAtX(event.X); tab >= 0 {
			m.workspaceActive = tab
			return m, workspaceTickCmd()
		}
	}
	switch event.Button {
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
		return m, nil
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if classifyTerminal(m.width, m.height) == sizeWide {
		contentWidth := max(1, m.width-1)
		leftWidth := contentWidth * 62 / 100
		rightWidth := contentWidth - leftWidth - 1
		if event.X > leftWidth {
			server := m.selectedServer()
			if server == nil {
				return m, nil
			}
			m.dashboardFocus = 1
			m.detailAction = 0

			// The right panel starts at dashboardBodyTop; its first inner row is
			// one line below the panel title. Clicking an action executes it
			// immediately, while other clicks simply focus the actions panel.
			line := event.Y - m.dashboardBodyTop(contentWidth) - 1
			innerWidth := max(1, rightWidth-2)
			if actionIndex := m.detailActionIndexAtLine(server, innerWidth, line); actionIndex >= 0 {
				actions := m.profileActions(server)
				m.detailAction = actionIndex
				return m.runDetailAction(server, actions[actionIndex])
			}
			return m, nil
		}
	}
	// Map a click in the visible server rows back to the current row. The
	// dashboard header, optional notice, panel border, optional filter prompt,
	// and column header sit above the first selectable row.
	headerRows := 1
	if len(m.workspaceSessions) > 0 {
		headerRows++
	}
	if m.err != nil || m.warning != "" || m.success != "" {
		headerRows++
	}
	rowY := headerRows + 2
	if m.filtering() {
		rowY++
	}
	if event.Y < rowY {
		return m, nil
	}

	bodyHeight := m.height - headerRows - displayLineCount(m.renderListHelp(len(m.selectedServers()), len(m.bgResults) > 0))
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	innerHeight := max(1, bodyHeight-2)
	prefixLines := 1
	if m.filtering() {
		prefixLines++
	}
	rowCapacity := max(0, innerHeight-prefixLines)
	showRange := len(m.rows) > rowCapacity
	if showRange {
		rowCapacity = max(1, rowCapacity-1)
	}
	start, end := visibleServerRange(len(m.rows), m.cursor, rowCapacity)
	index := start + (event.Y - rowY)
	if index >= start && index < end {
		m.cursor = index
	}
	return m, nil
}
