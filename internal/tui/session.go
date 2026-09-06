package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	sessionpkg "github.com/mirivlad/sshkeeper/internal/session"
)

type sessionScreenModel struct {
	list     list.Model
	sessions []sessionpkg.Window
	width    int
	height   int
	err      error
}

type sessionItem struct {
	window sessionpkg.Window
}

func (i sessionItem) Title() string {
	active := ""
	if i.window.Active {
		active = " active"
	}
	return fmt.Sprintf("%-28s  #%d%s", truncate(i.window.ServerAlias, 28), i.window.Index, active)
}
func (i sessionItem) Description() string {
	if i.window.StartedAt.IsZero() {
		return "tmux window " + i.window.ID
	}
	return fmt.Sprintf("running %s · tmux %s", time.Since(i.window.StartedAt).Round(time.Second), i.window.ID)
}

func (i sessionItem) FilterValue() string {
	return i.window.ServerAlias + " " + i.window.Name
}

func newSessionScreenModel(w, h int) *sessionScreenModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), w, managerListHeight(h))
	l.Title = "Sessions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle
	return &sessionScreenModel{list: l, width: w, height: h}
}

func (m *sessionScreenModel) loadSessions() tea.Cmd {
	return func() tea.Msg {
		windows, err := sessionpkg.List()
		return sessionsLoadedMsg{sessions: windows, err: err, closed: true}
	}
}
func (m *sessionScreenModel) setSessions(windows []sessionpkg.Window) {
	m.sessions = windows
	items := make([]list.Item, len(windows))
	for index, window := range windows {
		items[index] = sessionItem{window: window}
	}
	m.list.SetItems(items)
}

func (m *sessionScreenModel) selected() *sessionpkg.Window {
	item, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return nil
	}
	window := item.window
	return &window
}

func (m *sessionScreenModel) closeSelected() tea.Cmd {
	selected := m.selected()
	if selected == nil {
		return nil
	}
	id := selected.ID
	return func() tea.Msg {
		err := sessionpkg.Close(id)
		windows, listErr := sessionpkg.List()
		if err == nil {
			err = listErr
		}
		return sessionsLoadedMsg{sessions: windows, err: err, closed: true}
	}
}
func (m *sessionScreenModel) View() string {
	notification := ""
	if m.err != nil {
		notification = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	body := func(width, height int) string {
		if len(m.sessions) == 0 {
			return renderPaddedPanel(width, height, []string{dashboardHelp("No active SSH sessions.")})
		}
		capacity := max(1, height-2)
		start, end := visibleServerRange(len(m.sessions), m.list.Index(), max(1, capacity/2))
		lines := make([]string, 0, capacity)
		for index := start; index < end; index++ {
			item := sessionItem{window: m.sessions[index]}
			marker := "  "
			if index == m.list.Index() {
				marker = "> "
			}
			lines = append(lines, marker+item.Title(), "    "+item.Description())
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   "Sessions",
		status:       fmt.Sprintf("%d active · tmux", len(m.sessions)),
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "Enter", Action: "attach"},
			{Key: "Ctrl+D (d)", Action: "close"},
			{Key: "Ctrl+R (r)", Action: "refresh"},
			{Key: "Ctrl+H", Action: "help"},
			{Key: "Esc", Action: "back"},
		},
	})
}

type sessionsLoadedMsg struct {
	sessions []sessionpkg.Window
	err      error
	closed   bool
}
