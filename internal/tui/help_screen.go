package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
)

// --- Help screen (?) ---

type helpScreenModel struct {
	list  list.Model
	width int
}

func newHelpScreenModel(w, h int) *helpScreenModel {
	items := []list.Item{
		helpScreenItem{key: "Enter", action: "Connect / Confirm", section: "Actions"},
		helpScreenItem{key: "Esc", action: "Back / Cancel", section: "Actions"},
		helpScreenItem{key: "Tab/↓", action: "Next field", section: "Forms"},
		helpScreenItem{key: "Shift+Tab/↑", action: "Previous field", section: "Forms"},
		helpScreenItem{key: "/", action: "Open dropdown picker", section: "Forms"},
		helpScreenItem{key: "Ctrl+A", action: "Add server", section: "Server list"},
		helpScreenItem{key: "Ctrl+E", action: "Edit server", section: "Server list"},
		helpScreenItem{key: "Ctrl+F", action: "Search", section: "Server list"},
		helpScreenItem{key: "Ctrl+X", action: "Action menu", section: "Server list"},
		helpScreenItem{key: "Ins", action: "Select / deselect", section: "Server list"},
		helpScreenItem{key: "Ctrl+W", action: "Manage port forwards", section: "Forwards"},
		helpScreenItem{key: "?", action: "This quick help", section: "Other"},
		helpScreenItem{key: "F1", action: "Full documentation", section: "Other"},
		helpScreenItem{key: "Ctrl+Q", action: "Quit", section: "Other"},
	}

	l := list.New(items, helpScreenDelegate{}, w, h-4)
	l.Title = "sshkeeper — Quick Help"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return &helpScreenModel{list: l, width: w}
}

type helpScreenItem struct {
	key     string
	action  string
	section string
}

func (i helpScreenItem) Title() string       { return i.key }
func (i helpScreenItem) Description() string { return i.action }
func (i helpScreenItem) FilterValue() string { return i.key + " " + i.action }

type helpScreenDelegate struct{}

func (d helpScreenDelegate) Height() int                               { return 2 }
func (d helpScreenDelegate) Spacing() int                              { return 0 }
func (d helpScreenDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d helpScreenDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(helpScreenItem)
	if !ok {
		return
	}
	style := normalStyle
	if index == m.Index() {
		style = selectedRowStyle
	}
	keyStr := fmt.Sprintf("%-12s", i.key)
	actionStr := i.action
	line := hotkeyStyle.Render(keyStr) + helpTextStyle.Render(actionStr)
	w.Write([]byte(style.Render("  " + line + "\n")))
}

func (m *helpScreenModel) Init() tea.Cmd {
	return nil
}

func (m *helpScreenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *helpScreenModel) View() string {
	return m.list.View()
}

// --- Full help (F1) ---

type fullHelpModel struct {
	width  int
	height int
	offset int
}

func newFullHelpModel(w, h int) *fullHelpModel {
	return &fullHelpModel{width: w, height: h}
}

func (m *fullHelpModel) Init() tea.Cmd { return nil }

func (m *fullHelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			return m, nil
		case tea.KeyRunes:
			switch msg.String() {
			case "q", "Q":
				return m, nil
			case "j", "J":
				m.offset++
			case "k", "K":
				if m.offset > 0 {
					m.offset--
				}
			}
		case tea.KeyDown:
			m.offset++
		case tea.KeyUp:
			if m.offset > 0 {
				m.offset--
			}
		case tea.KeyHome:
			m.offset = 0
		case tea.KeyEnd:
			m.offset = 100
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m *fullHelpModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("sshkeeper — Full Help"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		rows  [][2]string
	}{
		{"What is sshkeeper", [][2]string{
			{"", "sshkeeper is a console SSH connection manager."},
			{"", "Linux and macOS are primary release targets; Windows is experimental."},
			{"", "It stores server profiles, secrets, and launches the system ssh client."},
			{"", ""},
		}},
		{"Navigation", [][2]string{
			{"↑/↓", "Move through list"},
			{"Tab/↓", "Next field"},
			{"Shift+Tab/↑", "Previous field"},
			{"/", "Open dropdown picker"},
		}},
		{"Global actions", [][2]string{
			{"Enter", "Select / Confirm / Open"},
			{"Esc", "Back / Cancel / Close"},
			{"?", "Quick help (hotkeys)"},
			{"F1", "Full documentation"},
			{"Ctrl+Q", "Quit"},
		}},
		{"Server list", [][2]string{
			{"Enter", "Connect to server"},
			{"Ctrl+A", "Add server"},
			{"Ctrl+E", "Edit server"},
			{"Ctrl+F", "Search"},
			{"Ctrl+X", "Action menu"},
			{"Ins", "Select / deselect"},
		}},
		{"Action menu (Ctrl+X)", [][2]string{
			{"Connect", "Standard SSH session"},
			{"Connect with tunnels", "SSH + all enabled forwards"},
			{"Start tunnels only", "Forwards without shell"},
			{"Start tunnels in bg", "Background tunnel process"},
			{"Manage port forwards", "Add / edit / delete forwards"},
			{"Manage tunnels", "View and stop running tunnels"},
			{"Manage route", "Configure ProxyJump / bastions"},
			{"Test connection", "Check if server is reachable"},
			{"Edit", "Edit server profile"},
			{"Delete", "Remove server profile"},
		}},
		{"Routes / ProxyJump", [][2]string{
			{"", "Routes define how to reach a server through jump hosts."},
			{"● direct", "No jump host"},
			{"→ via", "One bastion"},
			{"⇒ chain", "Multiple bastions"},
			{"", ""},
			{"CLI:", "sshkeeper route set <alias> --jumps bastion"},
		}},
		{"Port forwarding", [][2]string{
			{"", "A forward is a saved rule — just configuration."},
			{"Local", "Local port → remote service"},
			{"Remote", "Remote port → local service"},
			{"SOCKS", "Dynamic SOCKS proxy through SSH"},
			{"", ""},
			{"Ctrl+A", "Add forward"},
			{"Enter/Ctrl+E", "Edit forward"},
			{"Ctrl+D", "Delete forward (with confirmation)"},
		}},
		{"Tunnels", [][2]string{
			{"", "A tunnel is a running SSH process that activates forwards."},
			{"", ""},
			{"CLI:", "sshkeeper tunnel <alias>"},
			{"CLI:", "sshkeeper tunnel <alias> --forward-only"},
			{"CLI:", "sshkeeper tunnel <alias> --background"},
		}},
	}

	for _, sec := range sections {
		b.WriteString(sectionStyle.Render(sec.title))
		b.WriteString("\n")
		for _, row := range sec.rows {
			if row[0] == "" {
				b.WriteString(fmt.Sprintf("  %s\n", row[1]))
			} else {
				b.WriteString(fmt.Sprintf("  %-16s %s\n", row[0], row[1]))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  ↑/↓ scroll — q/Esc/Enter close"))

	// Simple scroll
	lines := strings.Split(b.String(), "\n")
	maxLines := m.height - 1
	if maxLines < 5 {
		maxLines = 5
	}
	start := m.offset
	if start > len(lines)-maxLines {
		start = len(lines) - maxLines
	}
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// --- Action menu ---

type actionMenuItem struct {
	label  string
	action string
}

func (i actionMenuItem) Title() string       { return i.label }
func (i actionMenuItem) Description() string { return "" }
func (i actionMenuItem) FilterValue() string { return i.label }

type actionMenuModel struct {
	list  list.Model
	width int
}

func newActionMenuModel(w, h int) *actionMenuModel {
	items := []list.Item{
		actionMenuItem{label: "Connect", action: "connect"},
		actionMenuItem{label: "Connect with tunnels", action: "tunnel"},
		actionMenuItem{label: "Start tunnels only", action: "tunnel_n"},
		actionMenuItem{label: "Start tunnels in background", action: "tunnel_bg"},
		actionMenuItem{label: "Manage port forwards", action: "forwards"},
		actionMenuItem{label: "Manage tunnels", action: "tunnels"},
		actionMenuItem{label: "Manage route", action: "route"},
		actionMenuItem{label: "Test connection", action: "test"},
		actionMenuItem{label: "Edit", action: "edit"},
		actionMenuItem{label: "Delete", action: "delete"},
		actionMenuItem{label: "Import", action: "import"},
		actionMenuItem{label: "Export", action: "export"},
		actionMenuItem{label: "Vault: lock", action: "vault_lock"},
		actionMenuItem{label: "Vault: change password", action: "vault_change_pw"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, len(items)+2)
	l.Title = "Actions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle

	return &actionMenuModel{list: l, width: w}
}

func (m *actionMenuModel) Update(msg tea.Msg) (*actionMenuModel, *string) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, nil
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(actionMenuItem); ok {
				return m, &item.action
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	_ = cmd
	return m, nil
}

func (m *actionMenuModel) View() string {
	return m.list.View()
}
