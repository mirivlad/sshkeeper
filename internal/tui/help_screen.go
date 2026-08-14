package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
)

// --- Help screen (?) ---

type helpScreenModel struct {
	list   list.Model
	width  int
	height int
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
		helpScreenItem{key: "Ctrl+H", action: "Full documentation", section: "Other"},
		helpScreenItem{key: "Ctrl+Q", action: "Quit", section: "Other"},
	}

	l := list.New(items, helpScreenDelegate{}, w, h-4)
	l.Title = "sshkeeper — Quick Help"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return &helpScreenModel{list: l, width: w, height: h}
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
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *helpScreenModel) View() string {
	items := m.list.Items()
	body := func(width, height int) string {
		innerRows := max(1, height-2)
		start, end := visibleServerRange(len(items), m.list.Index(), innerRows)
		lines := make([]string, 0, innerRows)
		for index := start; index < end; index++ {
			item, ok := items[index].(helpScreenItem)
			if !ok {
				continue
			}
			marker := "  "
			if index == m.list.Index() {
				marker = "> "
			}
			lines = append(lines, marker+padCells(item.key, 12)+" "+item.action)
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb: "Quick Help",
		status:     "Keyboard reference",
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "↑/↓", Action: "move"},
			{Key: "Ctrl+H", Action: "full help"},
			{Key: "Esc", Action: "back"},
		},
	})
}

// --- Full help (Ctrl+H) ---

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
			{"Ctrl+H", "Full documentation"},
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

	var lines []string
	for _, sec := range sections {
		lines = append(lines, sectionStyle.Copy().MarginTop(0).Render(sec.title))
		for _, row := range sec.rows {
			if row[0] == "" {
				lines = append(lines, "  "+row[1])
			} else {
				lines = append(lines, fmt.Sprintf("  %-16s %s", row[0], row[1]))
			}
		}
		lines = append(lines, "")
	}

	body := func(width, height int) string {
		capacity := max(1, height-2)
		start := min(m.offset, max(0, len(lines)-capacity))
		end := min(len(lines), start+capacity)
		return renderPaddedPanel(width, height, lines[start:end])
	}
	return renderScreenShell(screenShell{
		breadcrumb: "Full Help",
		status:     fmt.Sprintf("line %d/%d", min(m.offset+1, len(lines)), len(lines)),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "↑/↓", Action: "scroll"},
			{Key: "Ctrl+H", Action: "full help"},
			{Key: "Esc/Enter", Action: "close"},
		},
	})
}

// --- Action menu ---

type actionMenuItem struct {
	label       string
	action      string
	description string
}

func (i actionMenuItem) Title() string       { return i.label }
func (i actionMenuItem) Description() string { return "" }
func (i actionMenuItem) FilterValue() string { return i.label }

type actionMenuModel struct {
	list   list.Model
	width  int
	height int
}

func newActionMenuModel(w, h int) *actionMenuModel {
	items := []list.Item{
		actionMenuItem{label: "Connect", action: "connect", description: "Open an interactive SSH session."},
		actionMenuItem{label: "Connect with tunnels", action: "tunnel", description: "Open SSH and activate enabled port forwards."},
		actionMenuItem{label: "Start tunnels only", action: "tunnel_n", description: "Activate enabled forwards without a shell."},
		actionMenuItem{label: "Start tunnels in background", action: "tunnel_bg", description: "Run enabled forwards as a background process."},
		actionMenuItem{label: "Manage port forwards", action: "forwards", description: "Add, edit, enable, or remove forwarding rules."},
		actionMenuItem{label: "Manage tunnels", action: "tunnels", description: "Inspect and stop running tunnel processes."},
		actionMenuItem{label: "Manage route", action: "route", description: "Configure direct or ProxyJump routing."},
		actionMenuItem{label: "Test connection", action: "test", description: "Check SSH reachability for this profile."},
		actionMenuItem{label: "Edit", action: "edit", description: "Change this server profile."},
		actionMenuItem{label: "Delete", action: "delete", description: "Permanently remove this server profile."},
		actionMenuItem{label: "Import", action: "import", description: "Import profiles from a supported source."},
		actionMenuItem{label: "Export", action: "export", description: "Export selected server profiles."},
		actionMenuItem{label: "Vault: lock", action: "vault_lock", description: "Lock secrets for the current session."},
		actionMenuItem{label: "Vault: change password", action: "vault_change_pw", description: "Change the password protecting stored secrets."},
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, len(items)+2)
	l.Title = "Actions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle

	return &actionMenuModel{list: l, width: w, height: h}
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
	body := func(width, height int) string {
		listLines := m.actionLines(max(1, height-2))
		if classifyShellContent(width) == sizeWide {
			leftWidth := width * 48 / 100
			rightWidth := width - leftWidth - 1
			selected, _ := m.list.SelectedItem().(actionMenuItem)
			detail := []string{dashboardSection("Selected action"), "", selected.label, ""}
			detail = append(detail, wrapCells(selected.description, max(1, rightWidth-4))...)
			return joinPanelColumns(
				renderPaddedPanel(leftWidth, height, listLines), leftWidth,
				renderPaddedPanel(rightWidth, height, detail), rightWidth,
			)
		}
		if classifyShellContent(width) == sizeMedium {
			if selected, ok := m.list.SelectedItem().(actionMenuItem); ok && len(listLines) < height-4 {
				listLines = append(listLines, "", dashboardSection("Selected"), selected.description)
			}
		}
		return renderPaddedPanel(width, height, listLines)
	}
	return renderScreenShell(screenShell{
		breadcrumb: "Actions",
		status:     fmt.Sprintf("%d actions", len(m.list.Items())),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "↑/↓", Action: "move"},
			{Key: "Enter", Action: "select"},
			{Key: "Ctrl+H", Action: "help"},
			{Key: "Esc", Action: "back"},
		},
	})
}

func (m *actionMenuModel) actionLines(capacity int) []string {
	start, end := visibleServerRange(len(m.list.Items()), m.list.Index(), capacity)
	lines := make([]string, 0, capacity)
	for index := start; index < end; index++ {
		item, ok := m.list.Items()[index].(actionMenuItem)
		if !ok {
			continue
		}
		marker := "  "
		if index == m.list.Index() {
			marker = "> "
		}
		lines = append(lines, marker+item.label)
	}
	return lines
}
