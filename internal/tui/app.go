package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("4")).
			Bold(true)

	normalStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	selectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4"))
	listHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	sectionStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).MarginTop(1)

	testOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	testFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).MarginLeft(2)

	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
)

// --- Messages ---

type serversLoadedMsg struct {
	servers []*model.Server
	err     error
}

type testDoneMsg struct {
	ok  bool
	err string
}

type saveDoneMsg struct {
	err error
}

// connectRequestMsg — TUI requests a connect action to be handled outside
type connectRequestMsg struct {
	server *model.Server
}

// --- Server list item ---

type serverItem struct {
	server *model.Server
}

func (i serverItem) Title() string { return i.server.Alias }
func (i serverItem) Description() string {
	return fmt.Sprintf("%s@%s:%d  %s", i.server.User, i.server.Host, i.server.Port, i.server.AuthMethod)
}
func (i serverItem) FilterValue() string {
	return i.server.Alias + " " + i.server.DisplayName + " " + i.server.Host + " " + i.server.User
}

// --- External callbacks ---

var (
	ListServers    func() ([]*model.Server, error)
	SearchServers  func(query string) ([]*model.Server, error)
	DeleteServer   func(alias string) error
	TestConnection func(server *model.Server) (bool, string)
	// TestConnectionWithPassword tests with explicit password (for form test before save)
	TestConnectionWithPassword func(server *model.Server, password string) (bool, string)
	SaveServer                 func(server *model.Server, password string, oldAlias string) error
	UpdateTestResult           func(alias string, status model.TestStatus, testErr string) error
	HasSecret                  func(alias string, secretType string) bool
	GetGroups                  func() ([]string, error)
	RenameGroup                func(oldName, newName string) error
	DeleteGroup                func(name string) error
)

// --- Screen type ---

type screen int

const (
	screenList screen = iota
	screenForm
	screenSearch
)

// --- Result type — returned from TUI to caller ---

type TUIResult struct {
	Server *model.Server
	Action string // "connect"
}

// --- Main TUI model ---

type tuiModel struct {
	screen      screen
	list        list.Model
	servers     []*model.Server
	searchInput textinput.Model
	form        *formModel
	err         error
	success     string
	width       int
	height      int
	result      *TUIResult
}

func New(servers []*model.Server) *tuiModel {
	items := make([]list.Item, len(servers))
	for i, s := range servers {
		items[i] = serverItem{server: s}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "sshkeeper"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	search := textinput.New()
	search.Placeholder = "Search..."
	search.CharLimit = 64

	return &tuiModel{
		screen:      screenList,
		list:        l,
		servers:     servers,
		searchInput: search,
	}
}

func (m *tuiModel) Result() *TUIResult {
	return m.result
}

func (m *tuiModel) Init() tea.Cmd {
	return nil
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		if m.form != nil {
			m.form.width = msg.Width
			m.form.height = msg.Height
		}
		return m, nil

	case serversLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.servers = msg.servers
			items := make([]list.Item, len(msg.servers))
			for i, s := range msg.servers {
				items[i] = serverItem{server: s}
			}
			m.list.SetItems(items)
		}
		return m, nil

	case connectRequestMsg:
		// Store result and quit TUI — caller will handle the connect
		m.result = &TUIResult{
			Server: msg.server,
			Action: "connect",
		}
		return m, tea.Quit

	case testDoneMsg:
		if m.form != nil {
			m.form.testing = false
			if msg.ok {
				m.form.testResult = "Connection OK."
				m.form.testOK = true
			} else {
				m.form.testResult = fmt.Sprintf("Connection failed:\n%s", msg.err)
				m.form.testOK = false
			}
			m.form.testResultTime = time.Now()
			m.form.err = nil
			return m, nil
		}
		// Update test status in DB and reload list
		if item, ok := m.list.SelectedItem().(serverItem); ok && UpdateTestResult != nil {
			status := model.TestUnknown
			if msg.ok {
				status = model.TestOK
			} else if msg.err != "" {
				status = model.TestFailed
			}
			UpdateTestResult(item.server.Alias, status, msg.err)
		}
		return m, func() tea.Msg {
			servers, err := ListServers()
			return serversLoadedMsg{servers: servers, err: err}
		}

	case saveDoneMsg:
		if m.form != nil {
			m.form.saving = false
			if msg.err != nil {
				m.form.err = msg.err
				m.form.saved = false
			} else {
				m.form.saved = true
				m.form.savedTime = time.Now()
				m.form.err = nil
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenList:
			return m.updateList(msg)
		case screenForm:
			return m.updateForm(msg)
		case screenSearch:
			return m.updateSearch(msg)
		}
	}

	return m, nil
}

func (m *tuiModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Connect to selected server
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				return connectRequestMsg{server: item.server}
			}
		}

	case tea.KeyCtrlC, tea.KeyCtrlQ:
		return m, tea.Quit

	case tea.KeyCtrlA:
		// Add server
		m.form = newFormModel(m.width, m.height)
		m.screen = screenForm
		return m, nil

	case tea.KeyCtrlE:
		// Edit selected server
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			m.form = newEditFormModel(item.server, m.width, m.height)
			m.screen = screenForm
		}
		return m, nil

	case tea.KeyCtrlD:
		// Delete selected server
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				err := DeleteServer(item.server.Alias)
				if err != nil {
					return saveDoneMsg{err: err}
				}
				servers, err := ListServers()
				return serversLoadedMsg{servers: servers, err: err}
			}
		}

	case tea.KeyCtrlT:
		// Test connection
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				ok, testErr := TestConnection(item.server)
				return testDoneMsg{ok: ok, err: testErr}
			}
		}

	case tea.KeyCtrlF, tea.KeyCtrlS:
		// Search
		m.screen = screenSearch
		m.searchInput.Focus()
		return m, nil

	default:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *tuiModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil

	case tea.KeyEnter:
		m.screen = screenList
		m.searchInput.Blur()
		query := m.searchInput.Value()
		if query != "" {
			return m, func() tea.Msg {
				servers, err := SearchServers(query)
				return serversLoadedMsg{servers: servers, err: err}
			}
		}
		return m, func() tea.Msg {
			servers, err := ListServers()
			return serversLoadedMsg{servers: servers, err: err}
		}

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
}

func (m *tuiModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.form != nil && (m.form.showGroupList || m.form.showAuthList) {
			updated, cmd := m.form.Update(msg)
			if fm, ok := updated.(*formModel); ok {
				m.form = fm
			}
			return m, cmd
		}

		m.screen = screenList
		m.form = nil
		m.err = nil
		m.success = ""
		// Reload server list after form close
		return m, func() tea.Msg {
			servers, err := ListServers()
			return serversLoadedMsg{servers: servers, err: err}
		}
	}

	updated, cmd := m.form.Update(msg)
	if fm, ok := updated.(*formModel); ok {
		m.form = fm
	}
	return m, cmd
}

func (m *tuiModel) View() string {
	var b strings.Builder

	switch m.screen {
	case screenList:
		b.WriteString(m.viewServerList())

	case screenSearch:
		b.WriteString("Search: " + m.searchInput.View() + "\n")
		b.WriteString(helpStyle.Render("Type to search | Enter confirm | Esc cancel"))

	case screenForm:
		b.WriteString(m.form.View())
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		m.err = nil
	}
	if m.success != "" {
		b.WriteString("\n" + successStyle.Render(m.success))
		m.success = ""
	}

	return b.String()
}

func (m *tuiModel) viewServerList() string {
	var b strings.Builder
	selectedAlias := ""
	if item, ok := m.list.SelectedItem().(serverItem); ok && item.server != nil {
		selectedAlias = item.server.Alias
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("sshkeeper  %d servers", len(m.servers))))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("Vault unlocked | %s", testSummary(m.servers))))
	b.WriteString("\n\n")
	b.WriteString(listHeaderStyle.Render(fmt.Sprintf("  %-20s %-20s %-34s %-12s %-10s %s", "NAME", "ALIAS", "TARGET", "AUTH", "GROUP", "STATUS")))
	b.WriteString("\n")

	if len(m.servers) == 0 {
		b.WriteString(helpStyle.Render("  No servers yet. Press Ctrl+A to add one."))
		b.WriteString("\n")
	} else {
		for _, server := range m.servers {
			marker := " "
			rowStyle := normalStyle
			if server.Alias == selectedAlias {
				marker = ">"
				rowStyle = selectedRowStyle
			}
			name := server.DisplayName
			if name == "" {
				name = server.Alias
			}
			target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
			group := server.GroupName
			if group == "" {
				group = "-"
			}
			row := fmt.Sprintf("%s %-20s %-20s %-34s %-12s %-10s %s",
				marker,
				truncate(name, 20),
				truncate(server.Alias, 20),
				truncate(target, 34),
				authLabel(server.AuthMethod),
				truncate(group, 10),
				testStatusLabel(server),
			)
			b.WriteString(rowStyle.Render(row))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if selectedAlias != "" {
		if selected := m.selectedServer(); selected != nil {
			b.WriteString(m.viewSelectedServer(selected))
			b.WriteString("\n")
		}
	}
	b.WriteString(helpStyle.Render("Enter connect | Ctrl+A add | Ctrl+E edit | Ctrl+D del | Ctrl+T test | Ctrl+F search | Ctrl+Q quit"))
	return b.String()
}

func (m *tuiModel) selectedServer() *model.Server {
	if item, ok := m.list.SelectedItem().(serverItem); ok && item.server != nil {
		return item.server
	}
	return nil
}

func (m *tuiModel) viewSelectedServer(server *model.Server) string {
	displayName := server.DisplayName
	if displayName == "" {
		displayName = "-"
	}
	group := server.GroupName
	if group == "" {
		group = "-"
	}

	var b strings.Builder
	b.WriteString(sectionStyle.Render("Selected"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Alias: %s\n", server.Alias))
	b.WriteString(fmt.Sprintf("  Display Name: %s\n", displayName))
	b.WriteString(fmt.Sprintf("  Host: %s\n", server.Host))
	b.WriteString(fmt.Sprintf("  Port: %d\n", server.Port))
	b.WriteString(fmt.Sprintf("  User: %s\n", server.User))
	b.WriteString(fmt.Sprintf("  Auth: %s\n", authLabel(server.AuthMethod)))
	b.WriteString(fmt.Sprintf("  Group: %s\n", group))
	b.WriteString(fmt.Sprintf("  Status: %s\n", testStatusLabel(server)))
	return b.String()
}

func testSummary(servers []*model.Server) string {
	okCount := 0
	failedCount := 0
	for _, server := range servers {
		switch server.LastTestStatus {
		case model.TestOK:
			okCount++
		case model.TestFailed:
			failedCount++
		}
	}
	return fmt.Sprintf("%d OK | %d FAIL", okCount, failedCount)
}

func authLabel(auth model.AuthMethod) string {
	switch auth {
	case model.AuthPassword:
		return "password"
	case model.AuthKey:
		return "key"
	case model.AuthKeyPassphrase:
		return "key+phrase"
	case model.AuthAgent:
		return "agent"
	default:
		return string(auth)
	}
}

func testStatusLabel(server *model.Server) string {
	switch server.LastTestStatus {
	case model.TestOK:
		return "OK"
	case model.TestFailed:
		if server.LastTestError != "" {
			return "FAIL"
		}
		return "FAIL"
	default:
		return "?"
	}
}

// --- Form model ---

type formModel struct {
	edit           bool
	server         *model.Server
	inputs         []textinput.Model
	labels         []string
	password       textinput.Model
	passwordLabel  string
	focusIdx       int
	testResult     string
	testOK         bool
	testResultTime time.Time
	testing        bool
	saving         bool
	saved          bool
	savedTime      time.Time
	err            error
	spinner        spinner.Model
	width          int
	height         int
	groups         []string   // existing group names
	groupList      list.Model // dropdown list for groups
	showGroupList  bool       // whether group dropdown is visible
	authList       list.Model
	showAuthList   bool
}

// groupItem implements list.Item for the group dropdown
type groupItem struct {
	name string
}

func (i groupItem) Title() string       { return i.name }
func (i groupItem) Description() string { return "" }
func (i groupItem) FilterValue() string { return i.name }

func newFormModel(w, h int) *formModel {
	inputs := make([]textinput.Model, 10)
	labels := []string{
		"Alias",
		"Display Name",
		"Host",
		"Port",
		"User",
		"Auth Method (password/key/key_passphrase/agent)",
		"Identity File",
		"ProxyJump",
		"Group (type new or pick from list)",
		"Notes",
	}
	for i, label := range labels {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholderForLabel(label)
		inputs[i].CharLimit = 128
	}

	pw := textinput.New()
	pw.Placeholder = "optional"
	pw.CharLimit = 256
	pw.EchoMode = textinput.EchoPassword

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	inputs[0].Focus()

	fm := &formModel{
		inputs:        inputs,
		labels:        labels,
		password:      pw,
		passwordLabel: "Password / Passphrase",
		focusIdx:      0,
		spinner:       s,
		width:         w,
		height:        h,
	}
	fm.authList = newStringList([]string{
		string(model.AuthPassword),
		string(model.AuthKey),
		string(model.AuthKeyPassphrase),
		string(model.AuthAgent),
	}, "Select auth method", 34, 16)

	// Load existing groups
	if GetGroups != nil {
		if groups, err := GetGroups(); err == nil && len(groups) > 0 {
			fm.groups = groups
			fm.groupList = newStringList(groups, "Select group", 30, 8)
		}
	}

	fm.updateFocus()
	return fm
}

func placeholderForLabel(label string) string {
	switch label {
	case "Alias":
		return "mail.kp"
	case "Display Name":
		return "Production mail"
	case "Host":
		return "mail.example.org"
	case "Port":
		return "22"
	case "User":
		return "root"
	case "Auth Method (password/key/key_passphrase/agent)":
		return "key"
	case "Identity File":
		return "~/.ssh/id_ed25519"
	case "ProxyJump":
		return "optional"
	case "Group (type new or pick from list)":
		return "KP"
	case "Notes":
		return "optional"
	default:
		return label
	}
}

func newStringList(values []string, title string, width, height int) list.Model {
	items := make([]list.Item, len(values))
	for i, value := range values {
		items[i] = groupItem{name: value}
	}
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Title = title
	l.Styles.Title = titleStyle
	return l
}

func newEditFormModel(s *model.Server, w, h int) *formModel {
	fm := newFormModel(w, h)
	fm.edit = true
	fm.server = s
	fm.inputs[0].SetValue(s.Alias)
	fm.inputs[1].SetValue(s.DisplayName)
	fm.inputs[2].SetValue(s.Host)
	fm.inputs[3].SetValue(fmt.Sprintf("%d", s.Port))
	fm.inputs[4].SetValue(s.User)
	fm.inputs[5].SetValue(string(s.AuthMethod))
	fm.inputs[6].SetValue(s.IdentityFile)
	fm.inputs[7].SetValue(s.ProxyJump)
	fm.inputs[8].SetValue(s.GroupName)
	fm.inputs[9].SetValue(s.Notes)
	if HasSecret != nil {
		switch s.AuthMethod {
		case model.AuthPassword:
			if HasSecret(s.Alias, "ssh_password") {
				fm.passwordLabel = "Password (secret saved; leave blank to keep)"
				fm.password.Placeholder = ""
			}
		case model.AuthKeyPassphrase:
			if HasSecret(s.Alias, "key_passphrase") {
				fm.passwordLabel = "Key passphrase (secret saved; leave blank to keep)"
				fm.password.Placeholder = ""
			}
		}
	}
	fm.updateFocus()
	return fm
}

func (fm *formModel) Init() tea.Cmd {
	return nil
}

func (fm *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle test/save completion
	switch msg := msg.(type) {
	case testDoneMsg:
		fm.testing = false
		if msg.ok {
			fm.testResult = "Connection OK."
			fm.testOK = true
		} else {
			fm.testResult = fmt.Sprintf("Connection failed:\n%s", msg.err)
			fm.testOK = false
		}
		fm.testResultTime = time.Now()
		fm.err = nil
		return fm, nil
	case saveDoneMsg:
		fm.saving = false
		if msg.err != nil {
			fm.err = msg.err
			fm.saved = false
		} else {
			fm.saved = true
			fm.savedTime = time.Now()
			fm.err = nil
		}
		return fm, nil
	}

	// Handle spinner tick while testing/saving
	if fm.testing || fm.saving {
		var cmd tea.Cmd
		fm.spinner, cmd = fm.spinner.Update(msg)
		if _, ok := msg.(tea.KeyMsg); ok {
			return fm, cmd
		}
		return fm, cmd
	}

	// Handle group dropdown
	if fm.showGroupList {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				fm.showGroupList = false
				return fm, nil
			case tea.KeyEnter:
				if item, ok := fm.groupList.SelectedItem().(groupItem); ok {
					fm.inputs[8].SetValue(item.name)
				}
				fm.showGroupList = false
				return fm, nil
			}
		}
		// Pass other keys to the list
		var cmd tea.Cmd
		fm.groupList, cmd = fm.groupList.Update(msg)
		return fm, cmd
	}

	if fm.showAuthList {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				fm.showAuthList = false
				return fm, nil
			case tea.KeyEnter:
				if item, ok := fm.authList.SelectedItem().(groupItem); ok {
					fm.inputs[5].SetValue(item.name)
				}
				fm.showAuthList = false
				return fm, nil
			}
		}
		var cmd tea.Cmd
		fm.authList, cmd = fm.authList.Update(msg)
		return fm, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			fm.focusIdx++
			total := len(fm.inputs) + 3
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil

		case tea.KeyShiftTab:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := len(fm.inputs) + 3
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil

		case tea.KeyRunes:
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' && !msg.Alt && fm.focusIdx == 5 {
				fm.showAuthList = true
				return fm, nil
			}
			// '/' on Group field opens group dropdown
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' && !msg.Alt && fm.focusIdx == 8 && len(fm.groups) > 0 {
				fm.showGroupList = true
				return fm, nil
			}

		case tea.KeyEnter:
			switch {
			case fm.focusIdx == len(fm.inputs)+1:
				return fm, fm.runTest()
			case fm.focusIdx == len(fm.inputs)+2:
				return fm, fm.runSave()
			default:
				fm.focusIdx++
				total := len(fm.inputs) + 3
				if fm.focusIdx >= total {
					fm.focusIdx = 0
				}
				fm.updateFocus()
				return fm, nil
			}

		case tea.KeyEsc:
			return fm, nil

		case tea.KeyDown:
			fm.focusIdx++
			total := len(fm.inputs) + 3
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil

		case tea.KeyUp:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := len(fm.inputs) + 3
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		}
	}

	if fm.focusIdx < len(fm.inputs) {
		var cmd tea.Cmd
		fm.inputs[fm.focusIdx], cmd = fm.inputs[fm.focusIdx].Update(msg)
		return fm, cmd
	}

	if fm.focusIdx == len(fm.inputs) {
		var cmd tea.Cmd
		fm.password, cmd = fm.password.Update(msg)
		return fm, cmd
	}

	return fm, nil
}

func (fm *formModel) updateFocus() {
	for i := range fm.inputs {
		fm.inputs[i].Blur()
		fm.inputs[i].Prompt = blurredStyle.Render(fm.labelAt(i) + ": ")
	}
	fm.password.Blur()
	fm.password.Prompt = blurredStyle.Render(fm.passwordLabel + ": ")

	if fm.focusIdx < len(fm.inputs) {
		fm.inputs[fm.focusIdx].Focus()
		fm.inputs[fm.focusIdx].Prompt = focusedStyle.Render(fm.labelAt(fm.focusIdx) + "> ")
	} else if fm.focusIdx == len(fm.inputs) {
		fm.password.Focus()
		fm.password.Prompt = focusedStyle.Render(fm.passwordLabel + "> ")
	}
}

func (fm *formModel) labelAt(index int) string {
	if index >= 0 && index < len(fm.labels) {
		if index == 5 {
			return "Auth Method (/ pick)"
		}
		if index == 8 {
			if len(fm.groups) > 0 {
				return "Group (/ pick)"
			}
			return "Group"
		}
		return fm.labels[index]
	}
	return ""
}

func (fm *formModel) runTest() tea.Cmd {
	fm.testing = true
	fm.testResult = ""
	fm.err = nil
	fm.saved = false

	s := fm.buildServer()
	pw := fm.password.Value()

	return tea.Batch(
		fm.spinner.Tick,
		func() tea.Msg {
			// Use direct password test if available (for form test before save)
			if TestConnectionWithPassword != nil {
				ok, testErr := TestConnectionWithPassword(s, pw)
				return testDoneMsg{ok: ok, err: testErr}
			}
			// Fallback to vault-based test
			if s.AuthMethod == model.AuthPassword && pw == "" {
				return testDoneMsg{ok: false, err: "Password is required for password auth."}
			}
			ok, testErr := TestConnection(s)
			return testDoneMsg{ok: ok, err: testErr}
		},
	)
}

func (fm *formModel) runSave() tea.Cmd {
	fm.saving = true
	fm.err = nil
	fm.saved = false
	fm.testResult = ""

	s := fm.buildServer()
	pw := fm.password.Value()

	return tea.Batch(
		fm.spinner.Tick,
		func() tea.Msg {
			if s.Alias == "" {
				return saveDoneMsg{err: fmt.Errorf("alias is required")}
			}
			if s.Host == "" {
				return saveDoneMsg{err: fmt.Errorf("host is required")}
			}
			oldAlias := ""
			if fm.edit && fm.server != nil {
				oldAlias = fm.server.Alias
			}
			err := SaveServer(s, pw, oldAlias)
			return saveDoneMsg{err: err}
		},
	)
}

func (fm *formModel) buildServer() *model.Server {
	port := 22
	fmt.Sscanf(fm.inputs[3].Value(), "%d", &port)
	authMethod := model.AuthMethod(fm.inputs[5].Value())
	if authMethod == "" {
		authMethod = model.AuthKey
	}
	return &model.Server{
		Alias:        fm.inputs[0].Value(),
		DisplayName:  fm.inputs[1].Value(),
		Host:         fm.inputs[2].Value(),
		Port:         port,
		User:         fm.inputs[4].Value(),
		AuthMethod:   authMethod,
		IdentityFile: fm.inputs[6].Value(),
		ProxyJump:    fm.inputs[7].Value(),
		GroupName:    fm.inputs[8].Value(),
		Notes:        fm.inputs[9].Value(),
	}
}

func (fm *formModel) View() string {
	var b strings.Builder

	title := "Add Server"
	if fm.edit {
		title = "Edit Server: " + fm.server.Alias
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Calculate visible range based on terminal height
	// Reserve lines for: title (2) + password (1) + buttons (3) + help (1) + padding (2) = ~9
	reserved := 9
	available := fm.height - reserved
	if available < 4 {
		available = 4
	}

	numInputs := len(fm.inputs)
	startIdx := 0
	endIdx := numInputs

	// Scroll: keep focused field visible
	if numInputs > available {
		focusInput := fm.focusIdx
		if focusInput >= numInputs {
			focusInput = numInputs - 1
		}
		// Try to show `available` fields centered on focus
		startIdx = focusInput - available/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + available
		if endIdx > numInputs {
			endIdx = numInputs
			startIdx = endIdx - available
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	// Show scroll indicator if needed
	if startIdx > 0 {
		b.WriteString(helpStyle.Render("  ↑ more fields above\n"))
	}

	for i := startIdx; i < endIdx; i++ {
		if section := formSectionTitle(i); section != "" {
			b.WriteString(sectionStyle.Render(section))
			b.WriteString("\n")
		}
		if i == 5 {
			fm.inputs[i].Placeholder = "password/key/key_passphrase/agent"
		}
		// Show group hint inline in placeholder for Group field
		if i == 8 && len(fm.groups) > 0 && !fm.showGroupList {
			fm.inputs[i].Placeholder = truncate(strings.Join(fm.groups, ", "), 25)
		}
		b.WriteString(fm.inputs[i].View())
		b.WriteString("\n")
		if i == 5 && fm.showAuthList {
			b.WriteString("\n" + renderDropdown(fm.authList) + "\n")
			b.WriteString(helpStyle.Render("Enter select | Esc cancel"))
			return b.String()
		}
		if i == 8 && fm.showGroupList {
			b.WriteString("\n" + renderDropdown(fm.groupList) + "\n")
			b.WriteString(helpStyle.Render("Enter select | Esc cancel"))
			return b.String()
		}
	}

	if endIdx < numInputs {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↓ more fields below (%d-%d of %d)\n", startIdx+1, endIdx, numInputs)))
	}

	b.WriteString(fm.password.View())
	b.WriteString("\n")

	showResults := time.Since(fm.testResultTime) < 10*time.Second || time.Since(fm.savedTime) < 10*time.Second

	if fm.testing {
		b.WriteString("\n" + fm.spinner.View() + " Testing connection...\n")
	} else if fm.saving {
		b.WriteString("\n" + fm.spinner.View() + " Saving...\n")
	} else if showResults {
		if fm.testResult != "" {
			b.WriteString("\n")
			if fm.testOK {
				b.WriteString(testOKStyle.Render("✓ " + fm.testResult))
			} else {
				b.WriteString(testFailStyle.Render("✗ " + fm.testResult))
			}
			b.WriteString("\n")
		}
		if fm.saved {
			b.WriteString("\n" + successStyle.Render("✓ Saved.") + "\n")
		}
		if fm.err != nil {
			b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("✗ Error: %v", fm.err)) + "\n")
		}
	}

	testBtn := "[ Test ]"
	saveBtn := "[ Save ]"

	if fm.focusIdx == len(fm.inputs)+1 {
		testBtn = selectedStyle.Render(testBtn)
	} else {
		testBtn = normalStyle.Render(testBtn)
	}

	if fm.focusIdx == len(fm.inputs)+2 {
		saveBtn = selectedStyle.Render(saveBtn)
	} else {
		saveBtn = normalStyle.Render(saveBtn)
	}

	b.WriteString("\n" + sectionStyle.Render("Actions") + "\n")
	b.WriteString(testBtn + "  " + saveBtn + "\n\n")
	b.WriteString(helpStyle.Render("Tab/↓ next | ↑ prev | / pick list | Enter select | Esc back"))

	return b.String()
}

func renderDropdown(l list.Model) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render(l.Title))
	b.WriteString("\n")
	for i, item := range l.Items() {
		group, ok := item.(groupItem)
		if !ok {
			continue
		}
		prefix := "  "
		style := normalStyle
		if i == l.Index() {
			prefix = "> "
			style = selectedRowStyle
		}
		b.WriteString(style.Render(prefix + group.name))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formSectionTitle(index int) string {
	switch index {
	case 0:
		return "Identity"
	case 2:
		return "Connection"
	case 5:
		return "Authentication"
	case 8:
		return "Metadata"
	default:
		return ""
	}
}

// truncate limits a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
