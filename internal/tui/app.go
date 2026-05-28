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

	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).MarginLeft(2)
	hotkeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	helpTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

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

type templatesLoadedMsg struct {
	templates []*model.CommandTemplate
	err       error
}

type tagsLoadedMsg struct {
	tags []string
	err  error
}

type backgroundRunDoneMsg struct {
	results []templateRunResult
}

// connectRequestMsg — TUI requests a connect action to be handled outside
type connectRequestMsg struct {
	server *model.Server
}

type templateRunRequestMsg struct {
	servers      []*model.Server
	templateName string
	command      string
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

type templateItem struct {
	template *model.CommandTemplate
}

func (i templateItem) Title() string       { return i.template.Name }
func (i templateItem) Description() string { return i.template.Command }
func (i templateItem) FilterValue() string {
	return i.template.Name + " " + i.template.Command + " " + i.template.Description
}

type templateRunResult struct {
	Alias  string
	Output string
	Err    string
}

type helpItem struct {
	Key    string
	Action string
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
	ListTags                   func() ([]string, error)
	RenameTag                  func(oldName, newName string) error
	DeleteTag                  func(name string) error
	SetServerTags              func(server *model.Server, tags []string) error
	ListCommandTemplates       func() ([]*model.CommandTemplate, error)
	SaveCommandTemplate        func(oldName string, template *model.CommandTemplate) error
	DeleteCommandTemplate      func(name string) error
	RunTemplateBackground      func(server *model.Server, command string) (string, error)
)

// --- Screen type ---

type screen int

const (
	screenList screen = iota
	screenForm
	screenSearch
	screenTags
	screenTagInput
	screenTemplates
	screenTemplateForm
	screenTemplatePicker
	screenTemplateMode
	screenBackgroundResults
)

// --- Result type — returned from TUI to caller ---

type TUIResult struct {
	Server       *model.Server
	Servers      []*model.Server
	Action       string // "connect" or "run_template_foreground"
	Command      string
	TemplateName string
}

// --- Main TUI model ---

type tuiModel struct {
	screen          screen
	list            list.Model
	servers         []*model.Server
	searchInput     textinput.Model
	form            *formModel
	templateForm    *templateFormModel
	templates       []*model.CommandTemplate
	templateList    list.Model
	pendingTemplate *model.CommandTemplate
	tagList         list.Model
	tags            []string
	tagInput        textinput.Model
	tagMode         string
	tagOldName      string
	selected        map[string]bool
	bgResults       []templateRunResult
	err             error
	success         string
	width           int
	height          int
	result          *TUIResult
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

	tagInput := textinput.New()
	tagInput.Placeholder = "tag"
	tagInput.CharLimit = 64
	templateList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	templateList.SetShowStatusBar(false)
	templateList.SetFilteringEnabled(false)
	templateList.SetShowHelp(false)
	tagList := newStringList(nil, "Tags", 0, 0)

	return &tuiModel{
		screen:       screenList,
		list:         l,
		servers:      servers,
		searchInput:  search,
		selected:     map[string]bool{},
		tagInput:     tagInput,
		templateList: templateList,
		tagList:      tagList,
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
		m.templateList.SetSize(msg.Width, managerListHeight(msg.Height))
		m.tagList.SetSize(msg.Width, managerListHeight(msg.Height))
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

	case templatesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.setTemplates(msg.templates)
		return m, nil

	case tagsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.setTags(msg.tags)
		return m, nil

	case connectRequestMsg:
		// Store result and quit TUI — caller will handle the connect
		m.result = &TUIResult{
			Server: msg.server,
			Action: "connect",
		}
		return m, tea.Quit

	case templateRunRequestMsg:
		m.result = &TUIResult{
			Servers:      msg.servers,
			Action:       "run_template_foreground",
			Command:      msg.command,
			TemplateName: msg.templateName,
		}
		if len(msg.servers) == 1 {
			m.result.Server = msg.servers[0]
		}
		return m, tea.Quit

	case backgroundRunDoneMsg:
		m.bgResults = msg.results
		m.screen = screenList
		m.pendingTemplate = nil
		return m, nil

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
		if m.templateForm != nil {
			if msg.err != nil {
				m.templateForm.err = msg.err
				m.templateForm.saved = false
			} else {
				m.templateForm.saved = true
			}
			if m.templateForm.saved {
				m.screen = screenTemplates
				m.templateForm = nil
				return m, m.loadTemplatesCmd()
			}
			return m, nil
		}
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
		case screenTags:
			return m.updateTags(msg)
		case screenTagInput:
			return m.updateTagInput(msg)
		case screenTemplates:
			return m.updateTemplates(msg)
		case screenTemplateForm:
			return m.updateTemplateForm(msg)
		case screenTemplatePicker:
			return m.updateTemplatePicker(msg)
		case screenTemplateMode:
			return m.updateTemplateMode(msg)
		case screenBackgroundResults:
			return m.updateBackgroundResults(msg)
		}
	}

	return m, nil
}

func (m *tuiModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if len(m.bgResults) > 0 {
			m.bgResults = nil
			return m, nil
		}

	case tea.KeyEnter:
		// Connect to selected server
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				return connectRequestMsg{server: item.server}
			}
		}

	case tea.KeyInsert:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			if m.selected[item.server.Alias] {
				delete(m.selected, item.server.Alias)
			} else {
				m.selected[item.server.Alias] = true
			}
			if m.list.Index() < len(m.servers)-1 {
				m.list.Select(m.list.Index() + 1)
			}
		}
		return m, nil

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

	case tea.KeyCtrlG:
		m.screen = screenTags
		return m, m.loadTagsCmd()

	case tea.KeyCtrlP:
		m.screen = screenTemplates
		return m, m.loadTemplatesCmd()

	case tea.KeyCtrlR:
		return m.openTemplatePicker()

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

func (m *tuiModel) updateTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		return m, m.reloadServersCmd()
	case tea.KeyCtrlA:
		m.tagMode = "add"
		m.tagOldName = ""
		m.tagInput.SetValue("")
		m.tagInput.Focus()
		m.screen = screenTagInput
		return m, nil
	case tea.KeyCtrlE:
		if item, ok := m.tagList.SelectedItem().(groupItem); ok {
			m.tagMode = "rename"
			m.tagOldName = item.name
			m.tagInput.SetValue(item.name)
			m.tagInput.Focus()
			m.screen = screenTagInput
		}
		return m, nil
	case tea.KeyCtrlD:
		if item, ok := m.tagList.SelectedItem().(groupItem); ok && DeleteTag != nil {
			name := item.name
			return m, func() tea.Msg {
				if err := DeleteTag(name); err != nil {
					return tagsLoadedMsg{err: err}
				}
				tags, err := ListTags()
				return tagsLoadedMsg{tags: tags, err: err}
			}
		}
	case tea.KeyEnter:
		if item, ok := m.tagList.SelectedItem().(groupItem); ok && SetServerTags != nil {
			servers := m.selectedServers()
			if len(servers) == 0 {
				if selected := m.selectedServer(); selected != nil {
					servers = []*model.Server{selected}
				}
			}
			tag := item.name
			return m, func() tea.Msg {
				for _, server := range servers {
					tags := toggleString(server.Tags, tag)
					if err := SetServerTags(server, tags); err != nil {
						return tagsLoadedMsg{err: err}
					}
				}
				loaded, err := ListTags()
				return tagsLoadedMsg{tags: loaded, err: err}
			}
		}
	}
	var cmd tea.Cmd
	m.tagList, cmd = m.tagList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateTagInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenTags
		return m, nil
	case tea.KeyEnter:
		value := strings.TrimSpace(m.tagInput.Value())
		if value == "" {
			m.screen = screenTags
			return m, nil
		}
		mode := m.tagMode
		oldName := m.tagOldName
		return m, func() tea.Msg {
			if mode == "rename" && RenameTag != nil {
				if err := RenameTag(oldName, value); err != nil {
					return tagsLoadedMsg{err: err}
				}
			} else if SetServerTags != nil {
				servers := m.selectedServers()
				if len(servers) == 0 {
					if selected := m.selectedServer(); selected != nil {
						servers = []*model.Server{selected}
					}
				}
				for _, server := range servers {
					tags := append(splitCSV(strings.Join(server.Tags, ",")), value)
					if err := SetServerTags(server, tags); err != nil {
						return tagsLoadedMsg{err: err}
					}
				}
			}
			tags, err := ListTags()
			return tagsLoadedMsg{tags: tags, err: err}
		}
	}
	var cmd tea.Cmd
	m.tagInput, cmd = m.tagInput.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateTemplates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		return m, nil
	case tea.KeyCtrlA:
		m.templateForm = newTemplateFormModel(nil, m.width, m.height)
		m.screen = screenTemplateForm
		return m, nil
	case tea.KeyCtrlE:
		if item, ok := m.templateList.SelectedItem().(templateItem); ok {
			m.templateForm = newTemplateFormModel(item.template, m.width, m.height)
			m.screen = screenTemplateForm
		}
		return m, nil
	case tea.KeyCtrlD:
		if item, ok := m.templateList.SelectedItem().(templateItem); ok && DeleteCommandTemplate != nil {
			name := item.template.Name
			return m, func() tea.Msg {
				if err := DeleteCommandTemplate(name); err != nil {
					return templatesLoadedMsg{err: err}
				}
				templates, err := ListCommandTemplates()
				return templatesLoadedMsg{templates: templates, err: err}
			}
		}
	}
	var cmd tea.Cmd
	m.templateList, cmd = m.templateList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateTemplateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.screen = screenTemplates
		m.templateForm = nil
		return m, nil
	}
	updated, cmd := m.templateForm.Update(msg)
	if tf, ok := updated.(*templateFormModel); ok {
		m.templateForm = tf
		if tf.saved {
			m.screen = screenTemplates
			m.templateForm = nil
			return m, m.loadTemplatesCmd()
		}
	}
	return m, cmd
}

func (m *tuiModel) updateTemplatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		return m, nil
	case tea.KeyEnter:
		if item, ok := m.templateList.SelectedItem().(templateItem); ok {
			m.pendingTemplate = item.template
			m.screen = screenTemplateMode
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.templateList, cmd = m.templateList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateTemplateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingTemplate == nil {
		m.screen = screenList
		return m, nil
	}
	switch msg.Type {
	case tea.KeyCtrlB:
		servers := m.targetServers()
		tpl := m.pendingTemplate
		return m, func() tea.Msg {
			results := make([]templateRunResult, 0, len(servers))
			for _, server := range servers {
				output, err := RunTemplateBackground(server, tpl.Command)
				result := templateRunResult{Alias: server.Alias, Output: strings.TrimSpace(output)}
				if err != nil {
					result.Err = err.Error()
				}
				results = append(results, result)
			}
			return backgroundRunDoneMsg{results: results}
		}
	case tea.KeyCtrlF, tea.KeyEnter:
		servers := m.targetServers()
		tpl := m.pendingTemplate
		return m, func() tea.Msg {
			return templateRunRequestMsg{servers: servers, templateName: tpl.Name, command: tpl.Command}
		}
	case tea.KeyEsc:
		m.screen = screenTemplatePicker
		return m, nil
	case tea.KeyRunes:
		switch msg.String() {
		case "b", "B":
			servers := m.targetServers()
			tpl := m.pendingTemplate
			return m, func() tea.Msg {
				results := make([]templateRunResult, 0, len(servers))
				for _, server := range servers {
					output, err := RunTemplateBackground(server, tpl.Command)
					result := templateRunResult{Alias: server.Alias, Output: strings.TrimSpace(output)}
					if err != nil {
						result.Err = err.Error()
					}
					results = append(results, result)
				}
				return backgroundRunDoneMsg{results: results}
			}
		case "f", "F":
			servers := m.targetServers()
			tpl := m.pendingTemplate
			return m, func() tea.Msg {
				return templateRunRequestMsg{servers: servers, templateName: tpl.Name, command: tpl.Command}
			}
		}
	default:
		if msg.Type == tea.KeyEsc {
			m.screen = screenTemplatePicker
			return m, nil
		}
	}
	return m, nil
}

func (m *tuiModel) updateBackgroundResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.screen = screenList
		m.pendingTemplate = nil
		return m, m.reloadServersCmd()
	}
	return m, nil
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
		b.WriteString(renderHelp([]helpItem{{Key: "Type", Action: "search"}, {Key: "Enter", Action: "confirm"}, {Key: "Esc", Action: "cancel"}}, m.width))

	case screenForm:
		b.WriteString(m.form.View())

	case screenTags:
		b.WriteString(m.viewTags())

	case screenTagInput:
		b.WriteString(m.viewTagInput())

	case screenTemplates:
		b.WriteString(m.viewTemplates())

	case screenTemplateForm:
		b.WriteString(m.templateForm.View())

	case screenTemplatePicker:
		b.WriteString(m.viewTemplatePicker())

	case screenTemplateMode:
		b.WriteString(m.viewTemplateMode())

	case screenBackgroundResults:
		b.WriteString(m.viewBackgroundResults())
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
		selectedIndex := m.list.Index()
		start, end := visibleServerRange(len(m.servers), selectedIndex, m.visibleServerRows())
		for _, server := range m.servers[start:end] {
			marker := " "
			rowStyle := normalStyle
			if server.Alias == selectedAlias {
				marker = ">"
				rowStyle = selectedRowStyle
			}
			if m.selected[server.Alias] {
				marker = "*"
				if server.Alias == selectedAlias {
					marker = ">*"
				}
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
		if len(m.servers) > end-start {
			b.WriteString(helpStyle.Render(fmt.Sprintf("  Showing %d-%d of %d", start+1, end, len(m.servers))))
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
	if len(m.bgResults) > 0 {
		b.WriteString(m.viewInlineBackgroundResults())
		b.WriteString("\n")
	}
	selectedCount := len(m.selectedServers())
	footer := m.renderListHelp(selectedCount, len(m.bgResults) > 0)
	b.WriteString(strings.Repeat("\n", bottomPaddingLines(b.String(), footer, m.height)))
	b.WriteString(footer)
	return b.String()
}

func (m *tuiModel) viewInlineBackgroundResults() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Last Background Run"))
	b.WriteString("\n")
	for _, result := range m.bgResults {
		status := "OK"
		if result.Err != "" {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("  %-20s %s", result.Alias, status))
		if result.Err != "" {
			b.WriteString("  " + result.Err)
		}
		b.WriteString("\n")
	}

	selectedAlias := ""
	if selected := m.selectedServer(); selected != nil {
		selectedAlias = selected.Alias
	}
	result := m.backgroundResultForAlias(selectedAlias)
	if result == nil && len(m.bgResults) == 1 {
		result = &m.bgResults[0]
	}
	if result != nil {
		output := strings.TrimSpace(result.Output)
		if output == "" && result.Err != "" {
			output = result.Err
		}
		if output != "" {
			b.WriteString(helpStyle.Render("  Output: " + result.Alias))
			b.WriteString("\n")
			for _, line := range strings.Split(output, "\n") {
				b.WriteString(m.renderBackgroundOutputLine(line))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func (m *tuiModel) backgroundResultForAlias(alias string) *templateRunResult {
	for i := range m.bgResults {
		if m.bgResults[i].Alias == alias {
			return &m.bgResults[i]
		}
	}
	return nil
}

func (m *tuiModel) renderListHelp(selectedCount int, hasBackgroundResult bool) string {
	width := m.width - 2
	if width <= 0 {
		width = 80
	}
	lines := wrapHelpItems(m.listHelpItems(selectedCount, hasBackgroundResult), width)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = "  " + renderHelpLine(line)
	}
	return strings.Join(rendered, "\n")
}

func (m *tuiModel) listHelpItems(selectedCount int, hasBackgroundResult bool) []helpItem {
	insAction := "select"
	if selectedCount > 0 {
		insAction = fmt.Sprintf("select (%d selected)", selectedCount)
	}
	items := []helpItem{
		{Key: "Enter", Action: "connect"},
		{Key: "Ctrl+R", Action: "run tpl"},
		{Key: "Ins", Action: insAction},
	}
	if hasBackgroundResult {
		items = append(items, helpItem{Key: "Esc", Action: "clear result"})
	}
	return append(items,
		helpItem{Key: "Ctrl+P", Action: "tpl mgr"},
		helpItem{Key: "Ctrl+G", Action: "tags"},
		helpItem{Key: "Ctrl+A", Action: "add"},
		helpItem{Key: "Ctrl+E", Action: "edit"},
		helpItem{Key: "Ctrl+D", Action: "del"},
		helpItem{Key: "Ctrl+T", Action: "test"},
		helpItem{Key: "Ctrl+F", Action: "search"},
		helpItem{Key: "Ctrl+Q", Action: "quit"},
	)
}

func wrapHelpItems(items []helpItem, width int) [][]helpItem {
	if width <= 0 {
		return [][]helpItem{items}
	}
	var lines [][]helpItem
	var current []helpItem
	currentWidth := 0
	for _, item := range items {
		itemWidth := len(plainHelpItem(item))
		if len(current) == 0 {
			current = []helpItem{item}
			currentWidth = itemWidth
			continue
		}
		nextWidth := currentWidth + len(" | ") + itemWidth
		if nextWidth > width {
			lines = append(lines, current)
			current = []helpItem{item}
			currentWidth = itemWidth
			continue
		}
		current = append(current, item)
		currentWidth = nextWidth
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	return lines
}

func renderHelpLine(items []helpItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = hotkeyStyle.Render(item.Key) + helpTextStyle.Render(": "+item.Action)
	}
	return strings.Join(parts, helpTextStyle.Render(" | "))
}

func renderHelp(items []helpItem, width int) string {
	if width <= 0 {
		width = 80
	}
	lines := wrapHelpItems(items, width-2)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = "  " + renderHelpLine(line)
	}
	return strings.Join(rendered, "\n")
}

func plainHelpItem(item helpItem) string {
	return item.Key + ": " + item.Action
}

func plainHelpLine(items []helpItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = plainHelpItem(item)
	}
	return strings.Join(parts, " | ")
}

func bottomPaddingLines(content string, footer string, height int) int {
	if height <= 0 {
		return 0
	}
	used := strings.Count(content, "\n") + displayLineCount(footer)
	if used >= height {
		return 0
	}
	return height - used
}

func displayLineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m *tuiModel) renderBackgroundOutputLine(line string) string {
	line = strings.ReplaceAll(strings.TrimRight(line, "\r"), "\t", "    ")
	line = "    " + line
	width := m.width
	if width <= 0 {
		return line
	}
	if len(line) > width {
		line = truncate(line, width)
	}
	return line + strings.Repeat(" ", width-len(line))
}

func (m *tuiModel) selectedServer() *model.Server {
	if item, ok := m.list.SelectedItem().(serverItem); ok && item.server != nil {
		return item.server
	}
	return nil
}

func (m *tuiModel) visibleServerRows() int {
	if m.height <= 0 {
		return len(m.servers)
	}

	const fixedRows = 16
	rows := m.height - fixedRows
	if rows < 3 {
		return 3
	}
	return rows
}

func visibleServerRange(total, selected, available int) (int, int) {
	if total <= 0 || available <= 0 {
		return 0, 0
	}
	if available >= total {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	start := selected - available + 1
	if start < 0 {
		start = 0
	}
	end := start + available
	if end > total {
		end = total
		start = end - available
	}
	return start, end
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
	if len(server.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(server.Tags, ", ")))
	}
	if server.StartupCommand != "" {
		b.WriteString(fmt.Sprintf("  Startup: %s\n", server.StartupCommand))
	}
	b.WriteString(fmt.Sprintf("  Status: %s\n", testStatusLabel(server)))
	return b.String()
}

func (m *tuiModel) viewTags() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tags"))
	b.WriteString("\n\n")
	if len(m.tags) == 0 {
		b.WriteString(helpStyle.Render("  No tags yet. Press Ctrl+A to add one to the selected servers."))
		b.WriteString("\n")
	} else {
		for i, item := range m.tagList.Items() {
			tag, ok := item.(groupItem)
			if !ok {
				continue
			}
			marker := "  "
			style := normalStyle
			if i == m.tagList.Index() {
				marker = "> "
				style = selectedRowStyle
			}
			b.WriteString(style.Render(marker + tag.name))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(renderHelp([]helpItem{
		{Key: "Enter", Action: "toggle for selected/current"},
		{Key: "Ctrl+A", Action: "add"},
		{Key: "Ctrl+E", Action: "rename"},
		{Key: "Ctrl+D", Action: "delete"},
		{Key: "Esc", Action: "back"},
	}, m.width))
	return b.String()
}

func (m *tuiModel) viewTagInput() string {
	title := "Add Tag"
	if m.tagMode == "rename" {
		title = "Rename Tag"
	}
	return titleStyle.Render(title) + "\n\n" + m.tagInput.View() + "\n\n" + renderHelp([]helpItem{{Key: "Enter", Action: "save"}, {Key: "Esc", Action: "cancel"}}, m.width)
}

func (m *tuiModel) viewTemplates() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Command Templates"))
	b.WriteString("\n\n")
	if len(m.templates) == 0 {
		b.WriteString(helpStyle.Render("  No command templates yet. Press Ctrl+A to add one."))
		b.WriteString("\n")
	} else {
		for i, item := range m.templateList.Items() {
			tpl, ok := item.(templateItem)
			if !ok {
				continue
			}
			marker := "  "
			style := normalStyle
			if i == m.templateList.Index() {
				marker = "> "
				style = selectedRowStyle
			}
			line := fmt.Sprintf("%s%-24s %s", marker, truncate(tpl.template.Name, 24), tpl.template.Command)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
			if tpl.template.Description != "" {
				b.WriteString(helpStyle.Render("    " + tpl.template.Description))
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(renderHelp([]helpItem{
		{Key: "Ctrl+A", Action: "add"},
		{Key: "Ctrl+E", Action: "edit"},
		{Key: "Ctrl+D", Action: "delete"},
		{Key: "Esc", Action: "back"},
	}, m.width))
	return b.String()
}

func (m *tuiModel) viewTemplatePicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Run Template"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("Targets: %s", strings.Join(serverAliases(m.targetServers()), ", "))))
	b.WriteString("\n\n")
	if len(m.templates) == 0 {
		b.WriteString(helpStyle.Render("  No command templates. Press Esc, then Ctrl+P to add one."))
	} else {
		for i, item := range m.templateList.Items() {
			tpl, ok := item.(templateItem)
			if !ok {
				continue
			}
			marker := "  "
			style := normalStyle
			if i == m.templateList.Index() {
				marker = "> "
				style = selectedRowStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%-24s %s", marker, truncate(tpl.template.Name, 24), tpl.template.Command)))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(renderHelp([]helpItem{{Key: "Enter", Action: "choose"}, {Key: "Esc", Action: "back"}}, m.width))
	return b.String()
}

func (m *tuiModel) viewTemplateMode() string {
	name := ""
	command := ""
	if m.pendingTemplate != nil {
		name = m.pendingTemplate.Name
		command = m.pendingTemplate.Command
	}
	return titleStyle.Render("Run Mode") + "\n\n" +
		fmt.Sprintf("Template: %s\nCommand: %s\nTargets: %s\n\n", name, command, strings.Join(serverAliases(m.targetServers()), ", ")) +
		renderHelp([]helpItem{{Key: "Ctrl+F (Enter)", Action: "Foreground"}, {Key: "Ctrl+B", Action: "Background"}, {Key: "Esc", Action: "back"}}, m.width)
}

func (m *tuiModel) viewBackgroundResults() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Background Results"))
	b.WriteString("\n\n")
	for _, result := range m.bgResults {
		status := "OK"
		if result.Err != "" {
			status = "FAIL: " + result.Err
		}
		b.WriteString(sectionStyle.Render(result.Alias + "  " + status))
		b.WriteString("\n")
		if result.Output != "" {
			b.WriteString(result.Output)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(renderHelp([]helpItem{{Key: "Enter/Esc", Action: "back"}}, m.width))
	return b.String()
}

func (m *tuiModel) selectedServers() []*model.Server {
	if len(m.selected) == 0 {
		return nil
	}
	servers := make([]*model.Server, 0, len(m.selected))
	for _, server := range m.servers {
		if m.selected[server.Alias] {
			servers = append(servers, server)
		}
	}
	return servers
}

func (m *tuiModel) targetServers() []*model.Server {
	servers := m.selectedServers()
	if len(servers) > 0 {
		return servers
	}
	if selected := m.selectedServer(); selected != nil {
		return []*model.Server{selected}
	}
	return nil
}

func (m *tuiModel) openTemplatePicker() (tea.Model, tea.Cmd) {
	if m.selectedServer() == nil && len(m.selectedServers()) == 0 {
		return m, nil
	}
	m.screen = screenTemplatePicker
	return m, m.loadTemplatesCmd()
}

func (m *tuiModel) reloadServersCmd() tea.Cmd {
	return func() tea.Msg {
		servers, err := ListServers()
		return serversLoadedMsg{servers: servers, err: err}
	}
}

func (m *tuiModel) loadTemplatesCmd() tea.Cmd {
	return func() tea.Msg {
		if ListCommandTemplates == nil {
			return templatesLoadedMsg{err: fmt.Errorf("template storage is unavailable")}
		}
		templates, err := ListCommandTemplates()
		return templatesLoadedMsg{templates: templates, err: err}
	}
}

func (m *tuiModel) loadTagsCmd() tea.Cmd {
	return func() tea.Msg {
		if ListTags == nil {
			return tagsLoadedMsg{err: fmt.Errorf("tag storage is unavailable")}
		}
		tags, err := ListTags()
		return tagsLoadedMsg{tags: tags, err: err}
	}
}

func (m *tuiModel) setTemplates(templates []*model.CommandTemplate) {
	m.templates = templates
	items := make([]list.Item, len(templates))
	for i, template := range templates {
		items[i] = templateItem{template: template}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, managerListHeight(m.height))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Title = "Command Templates"
	l.Styles.Title = titleStyle
	m.templateList = l
}

func (m *tuiModel) setTags(tags []string) {
	m.tags = tags
	m.tagList = newStringList(tags, "Tags", m.width, managerListHeight(m.height))
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
	inputs := make([]textinput.Model, 12)
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
		"Startup Command",
		"Tags (comma-separated)",
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
	case "Startup Command":
		return "optional"
	case "Tags (comma-separated)":
		return "prod, web"
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
	fm.inputs[10].SetValue(s.StartupCommand)
	fm.inputs[11].SetValue(strings.Join(s.Tags, ", "))
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
		Alias:          fm.inputs[0].Value(),
		DisplayName:    fm.inputs[1].Value(),
		Host:           fm.inputs[2].Value(),
		Port:           port,
		User:           fm.inputs[4].Value(),
		AuthMethod:     authMethod,
		IdentityFile:   fm.inputs[6].Value(),
		ProxyJump:      fm.inputs[7].Value(),
		GroupName:      fm.inputs[8].Value(),
		Notes:          fm.inputs[9].Value(),
		StartupCommand: fm.inputs[10].Value(),
		Tags:           splitCSV(fm.inputs[11].Value()),
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
			b.WriteString(renderHelp([]helpItem{{Key: "Enter", Action: "select"}, {Key: "Esc", Action: "cancel"}}, fm.width))
			return b.String()
		}
		if i == 8 && fm.showGroupList {
			b.WriteString("\n" + renderDropdown(fm.groupList) + "\n")
			b.WriteString(renderHelp([]helpItem{{Key: "Enter", Action: "select"}, {Key: "Esc", Action: "cancel"}}, fm.width))
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
	b.WriteString(renderHelp([]helpItem{
		{Key: "Tab/↓", Action: "next"},
		{Key: "↑", Action: "prev"},
		{Key: "/", Action: "pick list"},
		{Key: "Enter", Action: "select"},
		{Key: "Esc", Action: "back"},
	}, fm.width))

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

type templateFormModel struct {
	edit     bool
	oldName  string
	inputs   []textinput.Model
	labels   []string
	focusIdx int
	err      error
	saved    bool
	width    int
	height   int
}

func newTemplateFormModel(t *model.CommandTemplate, w, h int) *templateFormModel {
	labels := []string{"Name", "Command", "Description"}
	inputs := make([]textinput.Model, len(labels))
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 512
	}
	inputs[0].Placeholder = "uptime"
	inputs[1].Placeholder = "uptime"
	inputs[2].Placeholder = "optional"
	inputs[0].Focus()

	tf := &templateFormModel{inputs: inputs, labels: labels, width: w, height: h}
	if t != nil {
		tf.edit = true
		tf.oldName = t.Name
		inputs[0].SetValue(t.Name)
		inputs[1].SetValue(t.Command)
		inputs[2].SetValue(t.Description)
	}
	tf.updateFocus()
	return tf
}

func (tf *templateFormModel) Init() tea.Cmd {
	return nil
}

func (tf *templateFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab, tea.KeyDown:
			tf.focusIdx++
			if tf.focusIdx > len(tf.inputs) {
				tf.focusIdx = 0
			}
			tf.updateFocus()
			return tf, nil
		case tea.KeyShiftTab, tea.KeyUp:
			tf.focusIdx--
			if tf.focusIdx < 0 {
				tf.focusIdx = len(tf.inputs)
			}
			tf.updateFocus()
			return tf, nil
		case tea.KeyEnter:
			if tf.focusIdx == len(tf.inputs) {
				return tf, tf.save()
			}
			tf.focusIdx++
			tf.updateFocus()
			return tf, nil
		}
	}
	if tf.focusIdx < len(tf.inputs) {
		var cmd tea.Cmd
		tf.inputs[tf.focusIdx], cmd = tf.inputs[tf.focusIdx].Update(msg)
		return tf, cmd
	}
	return tf, nil
}

func (tf *templateFormModel) updateFocus() {
	for i := range tf.inputs {
		tf.inputs[i].Blur()
		tf.inputs[i].Prompt = blurredStyle.Render(tf.labels[i] + ": ")
	}
	if tf.focusIdx < len(tf.inputs) {
		tf.inputs[tf.focusIdx].Focus()
		tf.inputs[tf.focusIdx].Prompt = focusedStyle.Render(tf.labels[tf.focusIdx] + "> ")
	}
}

func (tf *templateFormModel) save() tea.Cmd {
	return func() tea.Msg {
		if SaveCommandTemplate == nil {
			return saveDoneMsg{err: fmt.Errorf("template storage is unavailable")}
		}
		t := &model.CommandTemplate{
			Name:        strings.TrimSpace(tf.inputs[0].Value()),
			Command:     strings.TrimSpace(tf.inputs[1].Value()),
			Description: strings.TrimSpace(tf.inputs[2].Value()),
		}
		if t.Name == "" {
			return saveDoneMsg{err: fmt.Errorf("name is required")}
		}
		if t.Command == "" {
			return saveDoneMsg{err: fmt.Errorf("command is required")}
		}
		if err := SaveCommandTemplate(tf.oldName, t); err != nil {
			return saveDoneMsg{err: err}
		}
		return saveDoneMsg{}
	}
}

func (tf *templateFormModel) View() string {
	var b strings.Builder
	title := "Add Template"
	if tf.edit {
		title = "Edit Template"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	for i := range tf.inputs {
		b.WriteString(tf.inputs[i].View())
		b.WriteString("\n")
	}
	button := "[ Save ]"
	if tf.focusIdx == len(tf.inputs) {
		button = selectedStyle.Render(button)
	}
	b.WriteString("\n" + button + "\n\n")
	if tf.err != nil {
		b.WriteString(errorStyle.Render(tf.err.Error()))
		b.WriteString("\n")
	}
	b.WriteString(renderHelp([]helpItem{
		{Key: "Tab/↓", Action: "next"},
		{Key: "↑", Action: "prev"},
		{Key: "Enter", Action: "select"},
		{Key: "Esc", Action: "back"},
	}, tf.width))
	return b.String()
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

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func toggleString(values []string, value string) []string {
	clean := splitCSV(strings.Join(values, ","))
	for i, item := range clean {
		if item == value {
			return append(clean[:i], clean[i+1:]...)
		}
	}
	return append(clean, value)
}

func serverAliases(servers []*model.Server) []string {
	aliases := make([]string, 0, len(servers))
	for _, server := range servers {
		aliases = append(aliases, server.Alias)
	}
	return aliases
}

func managerListHeight(height int) int {
	if height <= 8 {
		return 3
	}
	return height - 6
}
