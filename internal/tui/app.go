package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
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
	templates   []*model.CommandTemplate
	deleted     bool
	deletedName string
	err         error
}

type tagsLoadedMsg struct {
	tags        []string
	deleted     bool
	deletedName string
	err         error
}

type backgroundRunDoneMsg struct {
	results []templateRunResult
}

type connectRequestMsg struct {
	server *model.Server
}

type templateRunRequestMsg struct {
	servers      []*model.Server
	templateName string
	command      string
}

type forwardsLoadedMsg struct {
	forwards []*model.Forward
	err      error
}

type forwardDeletedMsg struct {
	id  int64
	err error
}

type serverDeletedMsg struct {
	alias   string
	servers []*model.Server
	deleted bool
	err     error
}

type quitAfterDiscardMsg struct{}

type discardFormMsg struct {
	origin screen
}

type importDoneMsg struct {
	servers []*model.Server
	count   int
	err     error
}

// --- List items ---

type serverItem struct {
	server *model.Server
}

func (i serverItem) Title() string { return i.server.Alias }
func (i serverItem) Description() string {
	target := fmt.Sprintf("%s@%s:%d", i.server.User, i.server.Host, i.server.Port)
	routeStr := i.server.Route.DisplaySummary(target)
	return fmt.Sprintf("%s  %s", routeStr, i.server.AuthMethod)
}
func (i serverItem) FilterValue() string {
	parts := []string{
		i.server.Alias,
		i.server.DisplayName,
		i.server.Host,
		i.server.User,
		i.server.GroupName,
		i.server.Notes,
		i.server.ProxyJump,
		strings.Join(i.server.Tags, " "),
	}
	for _, h := range i.server.Route.Hops {
		parts = append(parts, h.Alias, h.Raw)
	}
	return strings.Join(parts, " ")
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
	ListForwards               func(serverID int64) ([]*model.Forward, error)
	SaveForward                func(fwd *model.Forward) error
	UpdateForward              func(fwd *model.Forward) error
	DeleteForward              func(forwardID int64) error
	ImportServers              func() (int, error)
	LockVault                  func() error
	VaultUnlocked              func() bool
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
	screenHelp
	screenActionMenu
	screenForwardList
	screenForwardForm
	screenTunnelManager
	screenConfirm
	screenFullHelp
)

type confirmChoice int

const (
	confirmCancel confirmChoice = iota
	confirmAccept
)

type confirmState struct {
	title       string
	target      string
	consequence string
	verb        string
	parent      screen
	complete    screen
	completeSet bool
	focus       confirmChoice
	pending     bool
	action      func() tea.Cmd
}

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
	tunnelScreen    *tunnelScreenModel
	bgResults       []templateRunResult
	err             error
	success         string
	width           int
	height          int
	result          *TUIResult
	helpScreen      *helpScreenModel
	actionMenu      *actionMenuModel
	forwardScreen   *forwardScreenModel
	forwardForm     *forwardFormModel
	confirm         *confirmState
	fullHelp        *fullHelpModel
	helpParent      screen
	vaultUnlocked   bool
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

	vaultIsUnlocked := true
	if VaultUnlocked != nil {
		vaultIsUnlocked = VaultUnlocked()
	}

	return &tuiModel{
		screen:        screenList,
		list:          l,
		servers:       servers,
		searchInput:   search,
		selected:      map[string]bool{},
		tagInput:      tagInput,
		templateList:  templateList,
		tagList:       tagList,
		vaultUnlocked: vaultIsUnlocked,
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
		if m.templateForm != nil {
			m.templateForm.width = msg.Width
			m.templateForm.height = msg.Height
		}
		if m.forwardScreen != nil {
			m.forwardScreen.width = msg.Width
			m.forwardScreen.height = msg.Height
		}
		if m.forwardForm != nil {
			m.forwardForm.width = msg.Width
			m.forwardForm.height = msg.Height
		}
		if m.tunnelScreen != nil {
			m.tunnelScreen.width = msg.Width
			m.tunnelScreen.height = msg.Height
			m.tunnelScreen.list.SetSize(msg.Width, managerListHeight(msg.Height))
		}
		if m.helpScreen != nil {
			updated, _ := m.helpScreen.Update(msg)
			if help, ok := updated.(*helpScreenModel); ok {
				m.helpScreen = help
			}
		}
		if m.fullHelp != nil {
			m.fullHelp.width = msg.Width
			m.fullHelp.height = msg.Height
		}
		if m.actionMenu != nil {
			m.actionMenu.width = msg.Width
			m.actionMenu.height = msg.Height
			m.actionMenu.list.SetSize(msg.Width, managerListHeight(msg.Height))
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
		if m.confirm != nil && m.confirm.pending && m.confirm.parent == screenTemplates {
			m.finishConfirm()
		}
		if msg.err != nil {
			if msg.deleted {
				m.removeTemplate(msg.deletedName)
				m.err = nil
				m.success = fmt.Sprintf("Deleted %q; refresh failed: %v", msg.deletedName, msg.err)
			} else {
				m.err = msg.err
			}
			return m, nil
		}
		m.setTemplates(msg.templates)
		return m, nil

	case tagsLoadedMsg:
		if m.confirm != nil && m.confirm.pending && m.confirm.parent == screenTags {
			m.finishConfirm()
		}
		if msg.err != nil {
			if msg.deleted {
				m.removeTag(msg.deletedName)
				m.err = nil
				m.success = fmt.Sprintf("Deleted %q; refresh failed: %v", msg.deletedName, msg.err)
			} else {
				m.err = msg.err
			}
			return m, nil
		}
		m.setTags(msg.tags)
		return m, nil

	case connectRequestMsg:
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

	case importDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.servers = msg.servers
		items := make([]list.Item, len(msg.servers))
		for i, s := range msg.servers {
			items[i] = serverItem{server: s}
		}
		m.list.SetItems(items)
		m.success = fmt.Sprintf("Imported %d server(s).", msg.count)
		return m, nil

	case forwardsLoadedMsg:
		if m.forwardScreen != nil {
			if msg.err != nil {
				m.forwardScreen.err = msg.err
			} else {
				m.forwardScreen.list = msg.forwards
				if len(msg.forwards) > 0 && m.forwardScreen.selected < 0 {
					m.forwardScreen.selected = 0
				}
			}
		}
		return m, nil

	case forwardDeletedMsg:
		m.finishConfirm()
		if m.forwardScreen != nil {
			if msg.err != nil {
				m.forwardScreen.err = msg.err
				return m, nil
			}
			m.forwardScreen.err = nil
			forwards := make([]*model.Forward, 0, len(m.forwardScreen.list))
			for _, forward := range m.forwardScreen.list {
				if forward.ID != msg.id {
					forwards = append(forwards, forward)
				}
			}
			m.forwardScreen.list = forwards
			if m.forwardScreen.selected >= len(forwards) {
				m.forwardScreen.selected = max(0, len(forwards)-1)
			}
			return m, m.forwardScreen.loadForwards()
		}
		return m, nil

	case serverDeletedMsg:
		m.finishConfirm()
		if msg.deleted {
			m.removeServer(msg.alias)
		}
		if msg.err != nil {
			if msg.deleted {
				m.err = nil
				m.success = fmt.Sprintf("Deleted %q; refresh failed: %v", msg.alias, msg.err)
			} else {
				m.err = msg.err
			}
			return m, nil
		}
		m.servers = msg.servers
		items := make([]list.Item, len(msg.servers))
		for i, server := range msg.servers {
			items[i] = serverItem{server: server}
		}
		m.list.SetItems(items)
		delete(m.selected, msg.alias)
		return m, nil

	case quitAfterDiscardMsg:
		m.confirm = nil
		m.form = nil
		m.forwardForm = nil
		m.templateForm = nil
		return m, tea.Quit

	case discardFormMsg:
		m.finishConfirm()
		switch msg.origin {
		case screenForm:
			m.form = nil
		case screenForwardForm:
			m.forwardForm = nil
		case screenTemplateForm:
			m.templateForm = nil
		}
		return m, nil

	case forwardDeleteConfirmMsg:
		m.beginConfirm(confirmState{
			title:       "Delete port forward?",
			target:      msg.name,
			consequence: "This removes the saved forwarding rule.",
			verb:        "Delete",
			parent:      screenForwardList,
			action: func() tea.Cmd {
				return func() tea.Msg {
					if DeleteForward == nil {
						return forwardDeletedMsg{id: msg.id, err: fmt.Errorf("forward deletion is unavailable")}
					}
					return forwardDeletedMsg{id: msg.id, err: DeleteForward(msg.id)}
				}
			},
		})
		return m, nil

	case forwardEditSignal:
		if m.forwardScreen != nil {
			if m.forwardScreen.selected >= 0 && m.forwardScreen.selected < len(m.forwardScreen.list) {
				fwd := m.forwardScreen.list[m.forwardScreen.selected]
				m.forwardForm = newForwardEditModel(m.forwardScreen.serverID, fwd, m.width, m.height)
				m.screen = screenForwardForm
			}
		}
		return m, nil

	case tunnelsLoadedMsg:
		if m.tunnelScreen != nil {
			m.tunnelScreen.tunnels = nil
			for _, item := range msg.items {
				if ti, ok := item.(tunnelItem); ok {
					m.tunnelScreen.tunnels = append(m.tunnelScreen.tunnels, ti.state)
				}
			}
			m.tunnelScreen.rebuildList()
		}
		return m, nil

	case tunnelStoppedMsg:
		m.finishConfirm()
		if m.tunnelScreen != nil {
			if msg.err != nil {
				m.tunnelScreen.err = msg.err
				return m, nil
			}
			m.tunnelScreen.err = nil
			return m, m.tunnelScreen.loadTunnels()
		}
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
		if m.forwardForm != nil {
			if msg.err != nil {
				m.forwardForm.applySaveError(msg.err)
				m.forwardForm.saved = false
				// Stay on screenForwardForm to show error
				return m, nil
			}
			m.forwardForm.saved = true
			// Return to forward list and reload
			m.forwardForm = nil
			m.screen = screenForwardList
			if m.forwardScreen != nil {
				return m, m.forwardScreen.loadForwards()
			}
			return m, nil
		}
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
				m.form.applySaveError(msg.err)
				m.form.saved = false
			} else {
				m.form.saved = true
				m.form.savedTime = time.Now()
				m.form.err = nil
				m.form.password.SetValue("")
				m.form.initial = m.form.snapshot()
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.err != nil || m.success != "" {
			m.err = nil
			m.success = ""
		}
		if msg.Type == tea.KeyCtrlH && m.screen != screenHelp && m.screen != screenFullHelp && m.screen != screenConfirm {
			m.helpParent = m.screen
			m.fullHelp = newFullHelpModel(m.width, m.height)
			m.screen = screenFullHelp
			return m, nil
		}
		if msg.Type == tea.KeyCtrlQ && m.screen != screenConfirm {
			return m.requestQuit()
		}
		if msg.Type == tea.KeyRunes && msg.String() == "?" && !m.screenOwnsPrintableInput() && m.screen != screenHelp && m.screen != screenFullHelp && m.screen != screenConfirm {
			m.helpParent = m.screen
			m.helpScreen = newHelpScreenModel(m.width, m.height)
			m.screen = screenHelp
			return m, nil
		}
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
		case screenHelp:
			return m.updateHelp(msg)
		case screenActionMenu:
			return m.updateActionMenu(msg)
		case screenForwardList:
			return m.updateForwardList(msg)
		case screenForwardForm:
			return m.updateForwardForm(msg)
		case screenTunnelManager:
			return m.updateTunnelManager(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenFullHelp:
			return m.updateFullHelp(msg)
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
		m.form = newFormModel(m.width, m.height)
		m.screen = screenForm
		return m, nil

	case tea.KeyCtrlE:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			m.form = newEditFormModel(item.server, m.width, m.height)
			m.screen = screenForm
		}
		return m, nil

	case tea.KeyCtrlD:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			m.confirmServerDelete(item.server)
			return m, nil
		}

	case tea.KeyCtrlT:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				ok, testErr := TestConnection(item.server)
				return testDoneMsg{ok: ok, err: testErr}
			}
		}

	case tea.KeyCtrlF, tea.KeyCtrlS:
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

	case tea.KeyRunes:
		if msg.String() == "?" {
			m.helpParent = m.screen
			m.helpScreen = newHelpScreenModel(m.width, m.height)
			m.screen = screenHelp
			return m, nil
		}

	case tea.KeyCtrlW:
		// Open forward manager for selected server
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			m.forwardScreen = newForwardScreenModel(item.server.ID, item.server.Alias, m.width, m.height)
			m.screen = screenForwardList
			return m, m.forwardScreen.loadForwards()
		}

	case tea.KeyCtrlX:
		m.actionMenu = newActionMenuModel(m.width, m.height)
		m.screen = screenActionMenu
		return m, nil

	default:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *tuiModel) requestQuit() (tea.Model, tea.Cmd) {
	dirty := false
	switch m.screen {
	case screenForm:
		dirty = m.form != nil && m.form.Dirty()
	case screenForwardForm:
		dirty = m.forwardForm != nil && m.forwardForm.Dirty()
	case screenTemplateForm:
		dirty = m.templateForm != nil && m.templateForm.Dirty()
	}
	if !dirty {
		return m, tea.Quit
	}
	origin := m.screen
	m.beginConfirm(confirmState{
		title:       "Discard changes and quit?",
		target:      "Unsaved form changes",
		consequence: "Your edits will be lost before sshkeeper exits.",
		verb:        "Quit",
		parent:      origin,
		action: func() tea.Cmd {
			return func() tea.Msg { return quitAfterDiscardMsg{} }
		},
	})
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
		if item, ok := m.tagList.SelectedItem().(groupItem); ok {
			name := item.name
			m.beginConfirm(confirmState{
				title:       "Delete tag?",
				target:      fmt.Sprintf("%q", name),
				consequence: "This removes the tag from every server profile.",
				verb:        "Delete",
				parent:      screenTags,
				action: func() tea.Cmd {
					return func() tea.Msg {
						if DeleteTag == nil {
							return tagsLoadedMsg{err: fmt.Errorf("tag deletion is unavailable")}
						}
						if err := DeleteTag(name); err != nil {
							return tagsLoadedMsg{err: err}
						}
						if ListTags == nil {
							return tagsLoadedMsg{deleted: true, deletedName: name, err: fmt.Errorf("tag reload is unavailable")}
						}
						tags, err := ListTags()
						return tagsLoadedMsg{tags: tags, deleted: true, deletedName: name, err: err}
					}
				},
			})
			return m, nil
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
		if item, ok := m.templateList.SelectedItem().(templateItem); ok {
			name := item.template.Name
			m.beginConfirm(confirmState{
				title:       "Delete command template?",
				target:      fmt.Sprintf("%q", name),
				consequence: "This removes the saved command template.",
				verb:        "Delete",
				parent:      screenTemplates,
				action: func() tea.Cmd {
					return func() tea.Msg {
						if DeleteCommandTemplate == nil {
							return templatesLoadedMsg{err: fmt.Errorf("template deletion is unavailable")}
						}
						if err := DeleteCommandTemplate(name); err != nil {
							return templatesLoadedMsg{err: err}
						}
						if ListCommandTemplates == nil {
							return templatesLoadedMsg{deleted: true, deletedName: name, err: fmt.Errorf("template reload is unavailable")}
						}
						templates, err := ListCommandTemplates()
						return templatesLoadedMsg{templates: templates, deleted: true, deletedName: name, err: err}
					}
				},
			})
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.templateList, cmd = m.templateList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateTemplateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.templateForm != nil && m.templateForm.Dirty() {
			m.confirmDiscard("Command template", screenTemplateForm, screenTemplates)
			return m, nil
		}
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
		default:
			if msg.Type == tea.KeyEsc {
				m.screen = screenTemplatePicker
				return m, nil
			}
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
		if m.form != nil && m.form.Dirty() {
			m.confirmDiscard("Server profile", screenForm, screenList)
			return m, nil
		}

		m.screen = screenList
		m.form = nil
		m.err = nil
		m.success = ""
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
	if classifyTerminal(m.width, m.height) == sizeBelowFloor {
		return minimumSizeView(m.width)
	}
	var b strings.Builder

	switch m.screen {
	case screenList:
		b.WriteString(m.viewServerList())

	case screenSearch:
		b.WriteString(m.viewSearch())

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

	case screenHelp:
		if m.helpScreen != nil {
			b.WriteString(m.helpScreen.View())
		}

	case screenFullHelp:
		if m.fullHelp != nil {
			b.WriteString(m.fullHelp.View())
		}

	case screenActionMenu:
		if m.actionMenu != nil {
			b.WriteString(m.actionMenu.View())
		}

	case screenForwardList:
		if m.forwardScreen != nil {
			b.WriteString(m.forwardScreen.View())
		}

	case screenForwardForm:
		if m.forwardForm != nil {
			b.WriteString(m.forwardForm.View())
		}

	case screenTunnelManager:
		if m.tunnelScreen != nil {
			b.WriteString(m.tunnelScreen.View())
		}

	case screenConfirm:
		b.WriteString(m.viewConfirm())
	}

	return b.String()
}

func (m *tuiModel) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, cmd := m.helpScreen.Update(msg)
	if hs, ok := updated.(*helpScreenModel); ok {
		m.helpScreen = hs
	}
	// Esc or Enter closes help
	if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || (msg.Type == tea.KeyRunes && (msg.String() == "q" || msg.String() == "Q" || msg.String() == "?")) {
		m.screen = m.helpParent
		m.helpScreen = nil
		return m, nil
	}
	return m, cmd
}

func (m *tuiModel) updateActionMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, action := m.actionMenu.Update(msg)
	m.actionMenu = updated

	if msg.Type == tea.KeyEsc {
		m.screen = screenList
		m.actionMenu = nil
		return m, nil
	}

	if action != nil {
		switch *action {
		case "connect":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.screen = screenList
				m.actionMenu = nil
				return m, func() tea.Msg {
					return connectRequestMsg{server: item.server}
				}
			}
		case "tunnel":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.actionMenu = nil
				m.result = &TUIResult{
					Server:  item.server,
					Action:  "tunnel",
					Servers: []*model.Server{item.server},
				}
				return m, tea.Quit
			}
		case "tunnel_n":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.actionMenu = nil
				m.result = &TUIResult{
					Server:  item.server,
					Action:  "tunnel_n",
					Servers: []*model.Server{item.server},
				}
				return m, tea.Quit
			}
		case "tunnel_bg":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.actionMenu = nil
				m.result = &TUIResult{
					Server:  item.server,
					Action:  "tunnel_bg",
					Servers: []*model.Server{item.server},
				}
				return m, tea.Quit
			}
		case "forwards":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.forwardScreen = newForwardScreenModel(item.server.ID, item.server.Alias, m.width, m.height)
				m.screen = screenForwardList
				m.actionMenu = nil
				return m, m.forwardScreen.loadForwards()
			}
		case "tunnels":
			m.tunnelScreen = newTunnelScreenModel(m.width, m.height)
			m.screen = screenTunnelManager
			m.actionMenu = nil
			return m, m.tunnelScreen.loadTunnels()
		case "route":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.form = newEditFormModel(item.server, m.width, m.height)
				m.form.focusIdx = 7
				m.form.updateFocus()
				m.screen = screenForm
				m.actionMenu = nil
			}
		case "test":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.screen = screenList
				m.actionMenu = nil
				return m, func() tea.Msg {
					ok, testErr := TestConnection(item.server)
					return testDoneMsg{ok: ok, err: testErr}
				}
			}
		case "edit":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.form = newEditFormModel(item.server, m.width, m.height)
				m.screen = screenForm
				m.actionMenu = nil
				return m, nil
			}
		case "delete":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.actionMenu = nil
				m.confirmServerDelete(item.server)
				return m, nil
			}
		case "import":
			m.screen = screenList
			m.actionMenu = nil
			return m, func() tea.Msg {
				if ImportServers == nil {
					return importDoneMsg{err: fmt.Errorf("import is unavailable")}
				}
				count, err := ImportServers()
				if err != nil {
					return importDoneMsg{err: err}
				}
				servers, err := ListServers()
				return importDoneMsg{servers: servers, count: count, err: err}
			}
		case "export":
			m.actionMenu = nil
			m.result = &TUIResult{Action: "export"}
			return m, tea.Quit
		case "vault_lock":
			m.screen = screenList
			m.actionMenu = nil
			if LockVault == nil {
				m.err = fmt.Errorf("vault lock is unavailable")
			} else if err := LockVault(); err != nil {
				m.err = err
			} else {
				m.vaultUnlocked = false
				m.success = "Vault locked."
			}
		case "vault_change_pw":
			m.actionMenu = nil
			m.result = &TUIResult{Action: "vault_change_pw"}
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

func (m *tuiModel) updateForwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		m.forwardScreen = nil
		return m, nil
	case tea.KeyCtrlA:
		// Add forward
		if m.forwardScreen != nil {
			m.forwardForm = newForwardFormModel(m.forwardScreen.serverID, m.width, m.height)
			m.screen = screenForwardForm
			return m, nil
		}
	case tea.KeyCtrlD:
		if m.forwardScreen != nil && m.forwardScreen.selected >= 0 && m.forwardScreen.selected < len(m.forwardScreen.list) {
			fwd := m.forwardScreen.list[m.forwardScreen.selected]
			m.confirmForwardDelete(fwd)
			return m, nil
		}
	case tea.KeyCtrlE, tea.KeyEnter:
		if m.forwardScreen != nil {
			return m, m.forwardScreen.editSelected()
		}
	case tea.KeyRunes:
		switch msg.String() {
		case "a", "A":
			if m.forwardScreen != nil {
				m.forwardForm = newForwardFormModel(m.forwardScreen.serverID, m.width, m.height)
				m.screen = screenForwardForm
				return m, nil
			}
		case "d", "D":
			if m.forwardScreen != nil && m.forwardScreen.selected >= 0 && m.forwardScreen.selected < len(m.forwardScreen.list) {
				fwd := m.forwardScreen.list[m.forwardScreen.selected]
				m.confirmForwardDelete(fwd)
				return m, nil
			}
		}
	case tea.KeyDown:
		if m.forwardScreen != nil && m.forwardScreen.selected < len(m.forwardScreen.list)-1 {
			m.forwardScreen.selected++
		}
		return m, nil
	case tea.KeyUp:
		if m.forwardScreen != nil && m.forwardScreen.selected > 0 {
			m.forwardScreen.selected--
		}
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) updateTunnelManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		m.tunnelScreen = nil
		return m, nil
	case tea.KeyCtrlD:
		m.confirmTunnelStop()
		return m, nil
	case tea.KeyCtrlR:
		if m.tunnelScreen != nil {
			return m, m.tunnelScreen.loadTunnels()
		}
	case tea.KeyRunes:
		switch msg.String() {
		case "d", "D", "s", "S":
			m.confirmTunnelStop()
			return m, nil
		case "r", "R":
			if m.tunnelScreen != nil {
				return m, m.tunnelScreen.loadTunnels()
			}
		}
	}
	var cmd tea.Cmd
	m.tunnelScreen.list, cmd = m.tunnelScreen.list.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm == nil {
		m.screen = screenList
		return m, nil
	}
	if m.confirm.pending {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		return m.cancelConfirm()
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyLeft, tea.KeyRight:
		if m.confirm.focus == confirmCancel {
			m.confirm.focus = confirmAccept
		} else {
			m.confirm.focus = confirmCancel
		}
		return m, nil
	case tea.KeyEnter:
		if m.confirm.focus == confirmCancel {
			return m.cancelConfirm()
		}
		return m.acceptConfirm()
	case tea.KeyRunes:
		switch msg.String() {
		case "y", "Y":
			return m.acceptConfirm()
		case "n", "N":
			return m.cancelConfirm()
		}
	}
	return m, nil
}

func (m *tuiModel) viewConfirm() string {
	if m.confirm == nil {
		return ""
	}
	body := func(width, height int) string {
		innerWidth := max(1, width-4)
		lines := []string{dashboardSection(m.confirm.title), ""}
		lines = append(lines, wrapCells(m.confirm.target, innerWidth)...)
		if m.confirm.consequence != "" {
			lines = append(lines, "")
			lines = append(lines, wrapCells(m.confirm.consequence, innerWidth)...)
		}
		lines = append(lines, "")
		cancel := "[ Cancel ]"
		accept := "[ " + m.confirm.verb + " ]"
		if m.confirm.focus == confirmCancel {
			cancel = selectedStyle.Render("> " + cancel)
		} else {
			accept = errorStyle.Render("> " + accept)
		}
		if m.confirm.pending {
			lines = append(lines, m.confirm.verb+" in progress…")
		} else {
			lines = append(lines, cancel+"  "+accept)
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb: "Confirm",
		status:     shellStatus(m.vaultUnlocked, "Action required"),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "Tab", Action: "choose"},
			{Key: "Enter", Action: "activate"},
			{Key: "Esc", Action: "cancel"},
		},
	})
}

func (m *tuiModel) beginConfirm(state confirmState) {
	state.focus = confirmCancel
	m.confirm = &state
	m.screen = screenConfirm
}

func (m *tuiModel) cancelConfirm() (tea.Model, tea.Cmd) {
	parent := m.confirm.parent
	m.confirm = nil
	m.screen = parent
	return m, nil
}

func (m *tuiModel) acceptConfirm() (tea.Model, tea.Cmd) {
	if m.confirm == nil || m.confirm.pending || m.confirm.action == nil {
		return m, nil
	}
	m.confirm.pending = true
	return m, m.confirm.action()
}

func (m *tuiModel) finishConfirm() {
	if m.confirm == nil {
		return
	}
	destination := m.confirm.parent
	if m.confirm.completeSet {
		destination = m.confirm.complete
	}
	m.screen = destination
	m.confirm = nil
}

func (m *tuiModel) confirmDiscard(title string, origin, destination screen) {
	m.beginConfirm(confirmState{
		title:       "Discard unsaved changes?",
		target:      title,
		consequence: "Your edits on this form will be lost.",
		verb:        "Discard",
		parent:      origin,
		complete:    destination,
		completeSet: true,
		action: func() tea.Cmd {
			return func() tea.Msg { return discardFormMsg{origin: origin} }
		},
	})
}

func (m *tuiModel) confirmServerDelete(server *model.Server) {
	alias := server.Alias
	m.beginConfirm(confirmState{
		title:       "Delete server profile?",
		target:      fmt.Sprintf("%q", alias),
		consequence: "This also removes its saved port forwards and vault secrets.",
		verb:        "Delete",
		parent:      screenList,
		action: func() tea.Cmd {
			return func() tea.Msg {
				if DeleteServer == nil {
					return serverDeletedMsg{alias: alias, err: fmt.Errorf("server deletion is unavailable")}
				}
				if err := DeleteServer(alias); err != nil {
					return serverDeletedMsg{alias: alias, err: err}
				}
				if ListServers == nil {
					return serverDeletedMsg{alias: alias, deleted: true, err: fmt.Errorf("server reload is unavailable")}
				}
				servers, err := ListServers()
				return serverDeletedMsg{alias: alias, servers: servers, deleted: true, err: err}
			}
		},
	})
}

func (m *tuiModel) removeServer(alias string) {
	servers := make([]*model.Server, 0, len(m.servers))
	for _, server := range m.servers {
		if server.Alias != alias {
			servers = append(servers, server)
		}
	}
	m.servers = servers
	items := make([]list.Item, len(servers))
	for index, server := range servers {
		items[index] = serverItem{server: server}
	}
	m.list.SetItems(items)
	delete(m.selected, alias)
}

func (m *tuiModel) confirmForwardDelete(fwd *model.Forward) {
	name := fwd.Name
	if strings.TrimSpace(name) == "" {
		name = fwd.ForwardListen()
	}
	id := fwd.ID
	m.beginConfirm(confirmState{
		title:       "Delete port forward?",
		target:      fmt.Sprintf("%q · %s → %s", name, fwd.ForwardListen(), fwd.ForwardTarget()),
		consequence: "This removes the saved forwarding rule. Active tunnels are not stopped.",
		verb:        "Delete",
		parent:      screenForwardList,
		action: func() tea.Cmd {
			return func() tea.Msg {
				if DeleteForward == nil {
					return forwardDeletedMsg{id: id, err: fmt.Errorf("forward deletion is unavailable")}
				}
				return forwardDeletedMsg{id: id, err: DeleteForward(id)}
			}
		},
	})
}

func (m *tuiModel) confirmTunnelStop() {
	if m.tunnelScreen == nil {
		return
	}
	item, ok := m.tunnelScreen.list.SelectedItem().(tunnelItem)
	if !ok || item.state == nil {
		return
	}
	state := item.state
	m.beginConfirm(confirmState{
		title:       "Stop running tunnel?",
		target:      fmt.Sprintf("%q · PID %d · %s", state.Name, state.PID, state.ServerAlias),
		consequence: "Active forwarded connections through this process will close.",
		verb:        "Stop",
		parent:      screenTunnelManager,
		action: func() tea.Cmd {
			return m.tunnelScreen.stopSelected()
		},
	})
}

func (m *tuiModel) updateFullHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, _ := m.fullHelp.Update(msg)
	if fh, ok := updated.(*fullHelpModel); ok {
		m.fullHelp = fh
	}
	if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || (msg.Type == tea.KeyRunes && (msg.String() == "q" || msg.String() == "Q")) {
		m.screen = m.helpParent
		m.fullHelp = nil
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) screenOwnsPrintableInput() bool {
	switch m.screen {
	case screenForm, screenSearch, screenTagInput, screenTemplateForm, screenForwardForm:
		return true
	default:
		return false
	}
}

func (m *tuiModel) updateForwardForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.forwardForm != nil && m.forwardForm.Dirty() {
			m.confirmDiscard("Port forward", screenForwardForm, screenForwardList)
			return m, nil
		}
		m.screen = screenForwardList
		m.forwardForm = nil
		return m, nil
	}
	updated, cmd := m.forwardForm.Update(msg)
	if fm, ok := updated.(*forwardFormModel); ok {
		m.forwardForm = fm
		if fm.saved {
			m.screen = screenForwardList
			m.forwardForm = nil
			// Reload forward list
			if m.forwardScreen != nil {
				return m, m.forwardScreen.loadForwards()
			}
		}
	}
	return m, cmd
}

func (m *tuiModel) viewServerList() string {
	return m.renderServerDashboard()
}

func (m *tuiModel) rootNotification() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.success != "" {
		return successStyle.Render(m.success)
	}
	return ""
}

func (m *tuiModel) viewSearch() string {
	return renderScreenShell(screenShell{
		breadcrumb:   "Search",
		status:       shellStatus(m.vaultUnlocked, fmt.Sprintf("%d profiles", len(m.servers))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{
				dashboardSection("Find server"),
				"",
				m.searchInput.View(),
				"",
				dashboardHelp("Search alias, host, display name, group, tags, notes, and route."),
			})
		},
		footer: []helpItem{{Key: "Type", Action: "search"}, {Key: "Enter", Action: "confirm"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "cancel"}},
	})
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
	target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
	b.WriteString(fmt.Sprintf("  Target: %s\n", target))
	b.WriteString(fmt.Sprintf("  Auth: %s\n", authLabel(server.AuthMethod)))
	if len(server.Route.Hops) > 0 {
		b.WriteString(fmt.Sprintf("  Route: %s\n", server.Route.DisplaySummary(target)))
	} else if server.ProxyJump != "" {
		b.WriteString(fmt.Sprintf("  ProxyJump: %s\n", server.ProxyJump))
	}
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
	return renderScreenShell(screenShell{
		breadcrumb:   "Tags",
		status:       shellStatus(m.vaultUnlocked, fmt.Sprintf("%d tags", len(m.tags))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			if len(m.tags) == 0 {
				return renderPaddedPanel(width, height, []string{dashboardHelp("No tags yet. Ctrl+A adds one to the selected servers.")})
			}
			capacity := max(1, height-2)
			start, end := visibleServerRange(len(m.tagList.Items()), m.tagList.Index(), capacity)
			lines := make([]string, 0, capacity)
			for index := start; index < end; index++ {
				tag, ok := m.tagList.Items()[index].(groupItem)
				if !ok {
					continue
				}
				marker := "  "
				if index == m.tagList.Index() {
					marker = "> "
				}
				lines = append(lines, marker+tag.name)
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Enter", Action: "toggle"}, {Key: "Ctrl+A", Action: "add"}, {Key: "Ctrl+E", Action: "rename"}, {Key: "Ctrl+D", Action: "delete"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "back"}},
	})
}

func (m *tuiModel) viewTagInput() string {
	title := "Add Tag"
	if m.tagMode == "rename" {
		title = "Rename Tag"
	}
	return renderScreenShell(screenShell{
		breadcrumb: title,
		status:     shellStatus(m.vaultUnlocked, "Tag editor"),
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{dashboardSection(title), "", m.tagInput.View()})
		},
		footer: []helpItem{{Key: "Enter", Action: "save"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "cancel"}},
	})
}

func (m *tuiModel) viewTemplates() string {
	return renderScreenShell(screenShell{
		breadcrumb:   "Command Templates",
		status:       shellStatus(m.vaultUnlocked, fmt.Sprintf("%d templates", len(m.templates))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			if len(m.templates) == 0 {
				return renderPaddedPanel(width, height, []string{dashboardHelp("No command templates yet. Ctrl+A adds one.")})
			}
			capacity := max(1, height-2)
			start, end := visibleServerRange(len(m.templateList.Items()), m.templateList.Index(), capacity)
			lines := make([]string, 0, capacity)
			for index := start; index < end; index++ {
				tpl, ok := m.templateList.Items()[index].(templateItem)
				if !ok {
					continue
				}
				marker := "  "
				if index == m.templateList.Index() {
					marker = "> "
				}
				lines = append(lines, marker+tpl.template.Name+"  "+tpl.template.Command)
				if tpl.template.Description != "" && classifyTerminal(width, height) != sizeNarrow {
					lines = append(lines, "    "+dashboardHelp(tpl.template.Description))
				}
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Ctrl+A", Action: "add"}, {Key: "Ctrl+E", Action: "edit"}, {Key: "Ctrl+D", Action: "delete"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "back"}},
	})
}

func (m *tuiModel) viewTemplatePicker() string {
	targets := strings.Join(serverAliases(m.targetServers()), ", ")
	return renderScreenShell(screenShell{
		breadcrumb: "Run Template",
		status:     "Targets: " + targets,
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			lines := []string{dashboardSection("Choose template"), dashboardHelp("Targets: " + targets), ""}
			if len(m.templates) == 0 {
				lines = append(lines, dashboardHelp("No command templates. Press Esc, then Ctrl+P to add one."))
			} else {
				capacity := max(1, height-len(lines)-2)
				start, end := visibleServerRange(len(m.templateList.Items()), m.templateList.Index(), capacity)
				for index := start; index < end; index++ {
					tpl, ok := m.templateList.Items()[index].(templateItem)
					if !ok {
						continue
					}
					marker := "  "
					if index == m.templateList.Index() {
						marker = "> "
					}
					lines = append(lines, marker+tpl.template.Name+"  "+tpl.template.Command)
				}
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Enter", Action: "choose"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "back"}},
	})
}

func (m *tuiModel) viewTemplateMode() string {
	name := ""
	command := ""
	if m.pendingTemplate != nil {
		name = m.pendingTemplate.Name
		command = m.pendingTemplate.Command
	}
	targets := strings.Join(serverAliases(m.targetServers()), ", ")
	return renderScreenShell(screenShell{
		breadcrumb: "Run Template / Mode",
		status:     "Targets: " + targets,
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{dashboardSection("Execution"), "", "Template: " + name, "Command: " + command, "Targets: " + targets, "", "Choose foreground for an interactive run or background to keep using sshkeeper."})
		},
		footer: []helpItem{{Key: "Ctrl+F (Enter)", Action: "Foreground"}, {Key: "Ctrl+B", Action: "Background"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "back"}},
	})
}

func (m *tuiModel) viewBackgroundResults() string {
	return renderScreenShell(screenShell{
		breadcrumb: "Background Results",
		status:     fmt.Sprintf("%d results", len(m.bgResults)),
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			lines := make([]string, 0)
			for _, result := range m.bgResults {
				status := "OK"
				if result.Err != "" {
					status = "FAIL: " + result.Err
				}
				lines = append(lines, dashboardSection(result.Alias+"  "+status))
				if result.Output != "" {
					for _, line := range strings.Split(result.Output, "\n") {
						lines = append(lines, strings.ReplaceAll(line, "\t", "    "))
					}
				}
				lines = append(lines, "")
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Enter/Esc", Action: "back"}, {Key: "Ctrl+H", Action: "help"}},
	})
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

func (m *tuiModel) removeTemplate(name string) {
	templates := make([]*model.CommandTemplate, 0, len(m.templates))
	for _, template := range m.templates {
		if template.Name != name {
			templates = append(templates, template)
		}
	}
	m.setTemplates(templates)
}

func (m *tuiModel) removeTag(name string) {
	tags := make([]string, 0, len(m.tags))
	for _, tag := range m.tags {
		if tag != name {
			tags = append(tags, tag)
		}
	}
	m.setTags(tags)
}

// --- Server list footer ---

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
	var items []helpItem
	if hasBackgroundResult {
		items = append(items, helpItem{Key: "Esc", Action: "clear result"})
	}
	items = append(items,
		helpItem{Key: "Enter", Action: "connect"},
		helpItem{Key: "Ctrl+X", Action: "actions"},
		helpItem{Key: "Ctrl+A", Action: "add"},
		helpItem{Key: "Ctrl+E", Action: "edit"},
		helpItem{Key: "Ctrl+F", Action: "search"},
		helpItem{Key: "Ins", Action: insAction},
		helpItem{Key: "?", Action: "hotkeys"},
		helpItem{Key: "Ctrl+H", Action: "help"},
		helpItem{Key: "Ctrl+Q", Action: "quit"},
	)
	return items
}

// --- Utility functions ---

func managerListHeight(height int) int {
	if height <= 8 {
		return 3
	}
	return height - 6
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
		return "FAIL"
	default:
		return "?"
	}
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

func truncate(s string, maxLen int) string {
	return truncateCells(s, maxLen)
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
