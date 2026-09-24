package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/tunnel"
)

// --- Tunnel manager screen ---

type tunnelScreenModel struct {
	list    list.Model
	tunnels []*model.TunnelState
	width   int
	height  int
	err     error
}

type tunnelItem struct {
	state *model.TunnelState
}

func (i tunnelItem) Title() string {
	status := i18n.T("stopped", "остановлен")
	if tunnel.IsRunning(i.state.ID) {
		status = i18n.T("running", "работает")
	}
	duration := time.Since(i.state.StartedAt).Round(time.Second)
	return fmt.Sprintf("%-30s  PID %-8d  %-8s  %s",
		truncate(i.state.Name, 30),
		i.state.PID,
		status,
		duration,
	)
}

func (i tunnelItem) Description() string {
	preview := fmt.Sprintf("ssh -N ... → %s", i.state.ServerAlias)
	if i.state.LastError != "" {
		return fmt.Sprintf("  %s\n  ✗ %s", preview, i.state.LastError)
	}
	return "  " + preview
}

func (i tunnelItem) FilterValue() string {
	return i.state.Name + " " + i.state.ServerAlias
}

func newTunnelScreenModel(w, h int) *tunnelScreenModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), w, h-6)
	l.Title = i18n.T("Tunnel Manager", "Управление туннелями")
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return &tunnelScreenModel{
		list:   l,
		width:  w,
		height: h,
	}
}

func (m *tunnelScreenModel) loadTunnels() tea.Cmd {
	return func() tea.Msg {
		states := tunnel.List()
		items := make([]list.Item, len(states))
		for i, s := range states {
			items[i] = tunnelItem{state: s}
		}
		return tunnelsLoadedMsg{items: items}
	}
}

func (m *tunnelScreenModel) rebuildList() {
	items := make([]list.Item, len(m.tunnels))
	for i, s := range m.tunnels {
		items[i] = tunnelItem{state: s}
	}
	m.list.SetItems(items)
}

func (m *tunnelScreenModel) stopSelected() tea.Cmd {
	if item, ok := m.list.SelectedItem().(tunnelItem); ok {
		return func() tea.Msg {
			return tunnelStoppedMsg{id: item.state.ID, err: tunnel.Stop(item.state.ID)}
		}
	}
	return nil
}

func (m *tunnelScreenModel) runningCount() int {
	count := 0
	for _, state := range m.tunnels {
		if state != nil && tunnel.IsRunning(state.ID) {
			count++
		}
	}
	return count
}

func (m *tunnelScreenModel) View() string {
	notification := ""
	if m.err != nil {
		notification = errorStyle.Render(i18n.Tf("Error: %v", "Ошибка: %v", m.err))
	}
	body := func(width, height int) string {
		if len(m.tunnels) == 0 {
			return renderPaddedPanel(width, height, []string{
				dashboardHelp(i18n.T("No tracked tunnels.", "Нет отслеживаемых туннелей.")),
				dashboardHelp(i18n.T("Select a server on the dashboard.", "Выберите сервер на главном экране.")),
				dashboardHelp(i18n.T("Ctrl+W: Port-forward rules; Ctrl+B: start in background.", "Ctrl+W: правила проброса; Ctrl+B: запуск в фоне.")),
			})
		}
		capacity := max(1, height-2)
		start, end := visibleServerRange(len(m.tunnels), m.list.Index(), max(1, capacity/3))
		lines := make([]string, 0, capacity)
		for index := start; index < end; index++ {
			item := tunnelItem{state: m.tunnels[index]}
			marker := "  "
			if index == m.list.Index() {
				marker = "> "
			}
			lines = append(lines, marker+item.Title())
			for _, description := range strings.Split(item.Description(), "\n") {
				lines = append(lines, "    "+strings.TrimSpace(description))
			}
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Tunnel Manager", "Управление туннелями"),
		status:       i18n.Tf("%d running · %d tracked", "Работают: %d · отслеживаются: %d", m.runningCount(), len(m.tunnels)),
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "Ctrl+D (s)", Action: i18n.T("stop tunnel", "остановить туннель")},
			{Key: "Ctrl+R (r)", Action: i18n.T("refresh", "обновить")},
			{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}

type tunnelsLoadedMsg struct {
	items []list.Item
}

type tunnelStoppedMsg struct {
	id  int64
	err error
}
