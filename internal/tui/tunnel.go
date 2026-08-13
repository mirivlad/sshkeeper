package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
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
	status := "stopped"
	if tunnel.IsRunning(i.state.ID) {
		status = "running"
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
	l.Title = "Tunnel Manager"
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

func (m *tunnelScreenModel) View() string {
	notification := ""
	if m.err != nil {
		notification = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	body := func(width, height int) string {
		if len(m.tunnels) == 0 {
			return renderPaddedPanel(width, height, []string{dashboardHelp("No running tunnels.")})
		}
		capacity := max(1, height-2)
		start, end := visibleServerRange(len(m.tunnels), m.list.Index(), max(1, capacity/2))
		lines := make([]string, 0, capacity)
		for index := start; index < end; index++ {
			item := tunnelItem{state: m.tunnels[index]}
			marker := "  "
			if index == m.list.Index() {
				marker = "> "
			}
			lines = append(lines, marker+item.Title(), "    "+item.Description())
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   "Tunnel Manager",
		status:       fmt.Sprintf("%d running", len(m.tunnels)),
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "Ctrl+D (s)", Action: "stop tunnel"},
			{Key: "Ctrl+R (r)", Action: "refresh"},
			{Key: "Ctrl+H", Action: "help"},
			{Key: "Esc", Action: "back"},
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
