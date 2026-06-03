package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
)

// --- Help screen ---

type helpScreenModel struct {
	list  list.Model
	width int
}

func newHelpScreenModel(w, h int) *helpScreenModel {
	items := []list.Item{
		helpScreenItem{key: "Enter", action: "Connect to server", section: "Navigation"},
		helpScreenItem{key: "↑/↓", action: "Navigate list", section: "Navigation"},
		helpScreenItem{key: "Tab/↓", action: "Next field", section: "Forms"},
		helpScreenItem{key: "Shift+Tab/↑", action: "Previous field", section: "Forms"},
		helpScreenItem{key: "/", action: "Open dropdown picker", section: "Forms"},
		helpScreenItem{key: "Esc", action: "Back / Cancel", section: "Navigation"},
		helpScreenItem{key: "Ctrl+A", action: "Add server", section: "Actions"},
		helpScreenItem{key: "Ctrl+E", action: "Edit server", section: "Actions"},
		helpScreenItem{key: "Ctrl+W", action: "Manage port forwards", section: "Actions"},
		helpScreenItem{key: "Ctrl+X", action: "Action menu (delete, test, tags, tunnel)", section: "Actions"},
		helpScreenItem{key: "Ctrl+D", action: "Delete server", section: "Actions"},
		helpScreenItem{key: "Ctrl+T", action: "Test connection", section: "Actions"},
		helpScreenItem{key: "Ctrl+F", action: "Search", section: "Actions"},
		helpScreenItem{key: "Ctrl+G", action: "Tags manager", section: "Actions"},
		helpScreenItem{key: "Ctrl+P", action: "Templates manager", section: "Actions"},
		helpScreenItem{key: "Ctrl+R", action: "Run template", section: "Templates"},
		helpScreenItem{key: "Ctrl+B", action: "Run in background", section: "Templates"},
		helpScreenItem{key: "Ins", action: "Select / deselect", section: "Selection"},
		helpScreenItem{key: "Ctrl+X", action: "Action menu", section: "Actions"},
		helpScreenItem{key: "?", action: "This help screen", section: "Navigation"},
		helpScreenItem{key: "Ctrl+Q", action: "Quit", section: "Navigation"},
	}

	l := list.New(items, helpScreenDelegate{}, w, h-4)
	l.Title = "sshkeeper — Help"
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
			return m, nil // caller checks screen transition
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
		actionMenuItem{label: "Start tunnel", action: "tunnel"},
		actionMenuItem{label: "Tunnel mode (-N)", action: "tunnel_n"},
		actionMenuItem{label: "Delete", action: "delete"},
		actionMenuItem{label: "Test connection", action: "test"},
		actionMenuItem{label: "Tags", action: "tags"},
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
