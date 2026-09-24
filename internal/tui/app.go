package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
	sessionpkg "github.com/mirivlad/sshkeeper/internal/session"
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

type groupsLoadedMsg struct {
	groups      []*model.Group
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

type actionForwardsLoadedMsg struct {
	serverID int64
	count    int
	err      error
}

type backgroundTunnelStartedMsg struct {
	alias  string
	origin screen
	state  *model.TunnelState
	err    error
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

type groupManagerItem struct {
	group *model.Group
}

func (i groupManagerItem) Title() string { return i.group.Name }
func (i groupManagerItem) Description() string {
	return i18n.Tf("%d servers", "%d серверов", i.group.ServerCount)
}
func (i groupManagerItem) FilterValue() string { return i.group.Name }

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
	ListIdentityFiles          func() ([]string, error)
	GetGroups                  func() ([]string, error)
	ListGroups                 func() ([]*model.Group, error)
	CreateGroup                func(name string) error
	ResolveRouteAlias          func(alias string) (int64, bool)
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
	StartBackgroundTunnel      func(alias string) (*model.TunnelState, error)
	ImportServers              func() (int, error)
	LockVault                  func() error
	VaultUnlocked              func() bool
	GetLanguagePreference      func() string
	SetLanguagePreference      func(string) error
)

// --- Screen type ---

type screen int

const (
	screenList screen = iota
	screenForm
	screenSearch
	screenTags
	screenTagInput
	screenGroups
	screenGroupInput
	screenTemplates
	screenTemplateForm
	screenTemplatePicker
	screenTemplateMode
	screenBackgroundResults
	screenHelp
	screenActionMenu
	screenManageMenu
	screenForwardList
	screenForwardForm
	screenSessionManager
	screenTunnelManager
	screenConfirm
	screenFullHelp
	screenSettings
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
	SessionID    string
}

// --- Main TUI model ---

type tuiModel struct {
	screen            screen
	list              list.Model
	servers           []*model.Server
	searchInput       textinput.Model
	form              *formModel
	templateForm      *templateFormModel
	templates         []*model.CommandTemplate
	templateList      list.Model
	pendingTemplate   *model.CommandTemplate
	tagList           list.Model
	tags              []string
	tagInput          textinput.Model
	tagMode           string
	tagOldName        string
	groups            []*model.Group
	groupList         list.Model
	groupInput        textinput.Model
	groupMode         string
	groupOldName      string
	selected          map[string]bool
	sessionsAvailable bool
	sessionScreen     *sessionScreenModel
	tunnelScreen      *tunnelScreenModel
	bgResults         []templateRunResult
	err               error
	success           string
	width             int
	height            int
	result            *TUIResult
	helpScreen        *helpScreenModel
	actionMenu        *actionMenuModel
	manageMenu        *actionMenuModel
	forwardScreen     *forwardScreenModel
	forwardForm       *forwardFormModel
	actionMenuParent  screen
	tunnelStarting    bool
	confirm           *confirmState
	fullHelp          *fullHelpModel
	settingsScreen    *settingsModel
	helpParent        screen
	vaultUnlocked     bool
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
	search.Placeholder = i18n.T("Search...", "Поиск...")
	search.CharLimit = 64

	tagInput := textinput.New()
	tagInput.Placeholder = "tag"
	tagInput.CharLimit = 64

	groupInput := textinput.New()
	groupInput.Placeholder = "group"
	groupInput.CharLimit = 64
	templateList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	templateList.SetShowStatusBar(false)
	templateList.SetFilteringEnabled(false)
	templateList.SetShowHelp(false)
	tagList := newStringList(nil, i18n.T("Tags", "Теги"), 0, 0)
	groupList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	groupList.SetShowStatusBar(false)
	groupList.SetFilteringEnabled(false)
	groupList.SetShowHelp(false)

	vaultIsUnlocked := true
	if VaultUnlocked != nil {
		vaultIsUnlocked = VaultUnlocked()
	}

	return &tuiModel{
		screen:            screenList,
		list:              l,
		servers:           servers,
		searchInput:       search,
		selected:          map[string]bool{},
		sessionsAvailable: sessionpkg.Available(),
		tagInput:          tagInput,
		groupInput:        groupInput,
		templateList:      templateList,
		tagList:           tagList,
		groupList:         groupList,
		vaultUnlocked:     vaultIsUnlocked,
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
		if m.sessionScreen != nil {
			m.sessionScreen.width = msg.Width
			m.sessionScreen.height = msg.Height
			m.sessionScreen.list.SetSize(msg.Width, managerListHeight(msg.Height))
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
		if m.settingsScreen != nil {
			m.settingsScreen.width = msg.Width
			m.settingsScreen.height = msg.Height
		}
		if m.actionMenu != nil {
			m.actionMenu.width = msg.Width
			m.actionMenu.height = msg.Height
			m.actionMenu.list.SetSize(msg.Width, managerListHeight(msg.Height))
		}
		m.templateList.SetSize(msg.Width, managerListHeight(msg.Height))
		m.tagList.SetSize(msg.Width, managerListHeight(msg.Height))
		m.groupList.SetSize(msg.Width, managerListHeight(msg.Height))
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
				m.success = i18n.Tf("Deleted %q; refresh failed: %v", "Удалено %q; обновить список не удалось: %v", msg.deletedName, msg.err)
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
				m.success = i18n.Tf("Deleted %q; refresh failed: %v", "Удалено %q; обновить список не удалось: %v", msg.deletedName, msg.err)
			} else {
				m.err = msg.err
			}
			return m, nil
		}
		m.setTags(msg.tags)
		return m, nil

	case groupsLoadedMsg:
		if m.confirm != nil && m.confirm.pending && m.confirm.parent == screenGroups {
			m.finishConfirm()
		}
		if msg.err != nil {
			if msg.deleted {
				m.removeGroup(msg.deletedName)
				m.err = nil
				m.success = i18n.Tf("Deleted %q; refresh failed: %v", "Удалено %q; обновить список не удалось: %v", msg.deletedName, msg.err)
			} else {
				m.err = msg.err
			}
			return m, nil
		}
		m.setGroups(msg.groups)
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
		m.success = i18n.Tf("Imported %d server(s).", "Импортировано серверов: %d.", msg.count)
		return m, nil

	case forwardsLoadedMsg:
		if m.forwardScreen != nil {
			if msg.err != nil {
				m.forwardScreen.err = msg.err
			} else {
				m.forwardScreen.err = nil
				m.forwardScreen.list = msg.forwards
				if len(msg.forwards) > 0 && m.forwardScreen.selected < 0 {
					m.forwardScreen.selected = 0
				}
			}
		}
		return m, nil

	case actionForwardsLoadedMsg:
		if m.actionMenu != nil && m.actionMenu.serverID == msg.serverID {
			m.actionMenu.loadingForwards = false
			m.actionMenu.enabledForwards = msg.count
			m.actionMenu.loadErr = msg.err
		}
		return m, nil

	case backgroundTunnelStartedMsg:
		m.tunnelStarting = false
		if msg.err == nil && msg.state == nil {
			msg.err = fmt.Errorf("%s", i18n.T("tunnel launcher returned no process", "запуск туннеля не вернул процесс"))
		}
		if msg.err != nil {
			if msg.origin == screenForwardList && m.forwardScreen != nil && m.forwardScreen.serverAlias == msg.alias {
				m.forwardScreen.err = msg.err
				m.forwardScreen.notice = ""
			} else {
				m.err = fmt.Errorf("%s: %w", i18n.Tf("Start tunnel for %s", "Запуск туннеля для %s", msg.alias), msg.err)
				m.success = ""
			}
			return m, nil
		}
		notice := i18n.Tf("Tunnel process launched for %s (PID %d). Check Running tunnels for status.", "Процесс туннеля запущен для %s (PID %d). Проверьте состояние в разделе работающих туннелей.", msg.alias, msg.state.PID)
		if msg.origin == screenForwardList && m.forwardScreen != nil && m.forwardScreen.serverAlias == msg.alias {
			m.forwardScreen.err = nil
			m.forwardScreen.notice = notice
		} else {
			m.err = nil
			m.success = notice
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
			m.forwardScreen.notice = ""
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
				m.success = i18n.Tf("Deleted %q; refresh failed: %v", "Удалено %q; обновить список не удалось: %v", msg.alias, msg.err)
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
			title:       i18n.T("Delete port forward?", "Удалить правило проброса порта?"),
			target:      msg.name,
			consequence: i18n.T("This removes the saved forwarding rule.", "Сохранённое правило проброса будет удалено."),
			verb:        i18n.T("Delete", "Удалить"),
			parent:      screenForwardList,
			action: func() tea.Cmd {
				return func() tea.Msg {
					if DeleteForward == nil {
						return forwardDeletedMsg{id: msg.id, err: fmt.Errorf("%s", i18n.T("forward deletion is unavailable", "удаление правила проброса недоступно"))}
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

	case sessionsLoadedMsg:
		if msg.closed && m.confirm != nil && m.confirm.pending && m.confirm.parent == screenSessionManager {
			m.finishConfirm()
		}
		if m.sessionScreen != nil {
			m.sessionScreen.err = msg.err
			if msg.err == nil {
				m.sessionScreen.setSessions(msg.sessions)
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
				m.form.testResult = i18n.T("Connection OK.", "Соединение установлено.")
				m.form.testOK = true
			} else {
				m.form.testResult = i18n.Tf("Connection failed:\n%s", "Ошибка соединения:\n%s", msg.err)
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
				m.forwardScreen.notice = i18n.T("Rule saved. Ctrl+B starts a background tunnel; Ctrl+X shows all start modes.", "Правило сохранено. Ctrl+B запускает фоновый туннель; Ctrl+X показывает все режимы запуска.")
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
		case screenGroups:
			return m.updateGroups(msg)
		case screenGroupInput:
			return m.updateGroupInput(msg)
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
		case screenManageMenu:
			return m.updateManageMenu(msg)
		case screenForwardList:
			return m.updateForwardList(msg)
		case screenForwardForm:
			return m.updateForwardForm(msg)
		case screenSessionManager:
			return m.updateSessionManager(msg)
		case screenTunnelManager:
			return m.updateTunnelManager(msg)
		case screenConfirm:
			return m.updateConfirm(msg)
		case screenFullHelp:
			return m.updateFullHelp(msg)
		case screenSettings:
			return m.updateSettings(msg)
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
		m.form.setRouteProfiles(m.servers)
		m.screen = screenForm
		return m, nil

	case tea.KeyCtrlE:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			m.form = newEditFormModel(item.server, m.width, m.height)
			m.form.setRouteProfiles(m.servers)
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
		if msg.String() == "m" || msg.String() == "M" {
			m.manageMenu = newManageMenuModel(m.width, m.height, m.sessionsAvailable)
			m.screen = screenManageMenu
			return m, nil
		}
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
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m.openServerActions(item.server, screenList, nil)
		}
		m.err = fmt.Errorf("%s", i18n.T("select a server before opening server actions", "выберите сервер перед открытием действий"))
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
		title:       i18n.T("Discard changes and quit?", "Отменить изменения и выйти?"),
		target:      i18n.T("Unsaved form changes", "Несохранённые изменения формы"),
		consequence: i18n.T("Your edits will be lost before sshkeeper exits.", "Изменения будут потеряны при выходе из sshkeeper."),
		verb:        i18n.T("Quit", "Выйти"),
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
				title:       i18n.T("Delete tag?", "Удалить тег?"),
				target:      fmt.Sprintf("%q", name),
				consequence: i18n.T("This removes the tag from every server profile.", "Тег будет удалён из всех профилей серверов."),
				verb:        i18n.T("Delete", "Удалить"),
				parent:      screenTags,
				action: func() tea.Cmd {
					return func() tea.Msg {
						if DeleteTag == nil {
							return tagsLoadedMsg{err: fmt.Errorf("%s", i18n.T("tag deletion is unavailable", "удаление тегов недоступно"))}
						}
						if err := DeleteTag(name); err != nil {
							return tagsLoadedMsg{err: err}
						}
						if ListTags == nil {
							return tagsLoadedMsg{deleted: true, deletedName: name, err: fmt.Errorf("%s", i18n.T("tag reload is unavailable", "обновление тегов недоступно"))}
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

func (m *tuiModel) updateGroups(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		return m, m.reloadServersCmd()
	case tea.KeyCtrlA:
		m.groupMode = "add"
		m.groupOldName = ""
		m.groupInput.SetValue("")
		m.groupInput.Focus()
		m.screen = screenGroupInput
		return m, nil
	case tea.KeyCtrlE:
		if item, ok := m.groupList.SelectedItem().(groupManagerItem); ok && item.group != nil {
			m.groupMode = "rename"
			m.groupOldName = item.group.Name
			m.groupInput.SetValue(item.group.Name)
			m.groupInput.Focus()
			m.screen = screenGroupInput
		}
		return m, nil
	case tea.KeyCtrlD:
		if item, ok := m.groupList.SelectedItem().(groupManagerItem); ok && item.group != nil {
			name := item.group.Name
			count := item.group.ServerCount
			m.beginConfirm(confirmState{
				title:       i18n.T("Delete group?", "Удалить группу?"),
				target:      fmt.Sprintf("%q", name),
				consequence: i18n.Tf("The group is removed; %d server profile(s) become ungrouped.", "Группа будет удалена; профилей без группы станет больше на %d.", count),
				verb:        i18n.T("Delete", "Удалить"),
				parent:      screenGroups,
				action: func() tea.Cmd {
					return func() tea.Msg {
						if DeleteGroup == nil {
							return groupsLoadedMsg{err: fmt.Errorf("%s", i18n.T("group deletion is unavailable", "удаление групп недоступно"))}
						}
						if err := DeleteGroup(name); err != nil {
							return groupsLoadedMsg{err: err}
						}
						if ListGroups == nil {
							return groupsLoadedMsg{deleted: true, deletedName: name, err: fmt.Errorf("%s", i18n.T("group reload is unavailable", "обновление групп недоступно"))}
						}
						groups, err := ListGroups()
						return groupsLoadedMsg{groups: groups, deleted: true, deletedName: name, err: err}
					}
				},
			})
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.groupList, cmd = m.groupList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenGroups
		m.groupInput.Blur()
		return m, nil
	case tea.KeyEnter:
		value := strings.TrimSpace(m.groupInput.Value())
		if value == "" {
			m.screen = screenGroups
			return m, nil
		}
		mode := m.groupMode
		oldName := m.groupOldName
		return m, func() tea.Msg {
			switch mode {
			case "rename":
				if RenameGroup == nil {
					return groupsLoadedMsg{err: fmt.Errorf("%s", i18n.T("group rename is unavailable", "переименование группы недоступно"))}
				}
				if err := RenameGroup(oldName, value); err != nil {
					return groupsLoadedMsg{err: err}
				}
			default:
				if CreateGroup == nil {
					return groupsLoadedMsg{err: fmt.Errorf("%s", i18n.T("group creation is unavailable", "создание группы недоступно"))}
				}
				if err := CreateGroup(value); err != nil {
					return groupsLoadedMsg{err: err}
				}
			}
			groups, err := ListGroups()
			return groupsLoadedMsg{groups: groups, err: err}
		}
	}
	var cmd tea.Cmd
	m.groupInput, cmd = m.groupInput.Update(msg)
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
				title:       i18n.T("Delete command template?", "Удалить шаблон команды?"),
				target:      fmt.Sprintf("%q", name),
				consequence: i18n.T("This removes the saved command template.", "Сохранённый шаблон команды будет удалён."),
				verb:        i18n.T("Delete", "Удалить"),
				parent:      screenTemplates,
				action: func() tea.Cmd {
					return func() tea.Msg {
						if DeleteCommandTemplate == nil {
							return templatesLoadedMsg{err: fmt.Errorf("%s", i18n.T("template deletion is unavailable", "удаление шаблонов недоступно"))}
						}
						if err := DeleteCommandTemplate(name); err != nil {
							return templatesLoadedMsg{err: err}
						}
						if ListCommandTemplates == nil {
							return templatesLoadedMsg{deleted: true, deletedName: name, err: fmt.Errorf("%s", i18n.T("template reload is unavailable", "обновление шаблонов недоступно"))}
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
			m.confirmDiscard(i18n.T("Command template", "Шаблон команды"), screenTemplateForm, screenTemplates)
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
			m.confirmDiscard(i18n.T("Server profile", "Профиль сервера"), screenForm, screenList)
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
	case screenGroups:
		b.WriteString(m.viewGroups())
	case screenGroupInput:
		b.WriteString(m.viewGroupInput())

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
	case screenManageMenu:
		if m.manageMenu != nil {
			b.WriteString(m.manageMenu.View())
		}

	case screenForwardList:
		if m.forwardScreen != nil {
			b.WriteString(m.forwardScreen.View())
		}

	case screenForwardForm:
		if m.forwardForm != nil {
			b.WriteString(m.forwardForm.View())
		}

	case screenSessionManager:
		if m.sessionScreen != nil {
			b.WriteString(m.sessionScreen.View())
		}

	case screenTunnelManager:
		if m.tunnelScreen != nil {
			b.WriteString(m.tunnelScreen.View())
		}

	case screenConfirm:
		b.WriteString(m.viewConfirm())
	case screenSettings:
		if m.settingsScreen != nil {
			b.WriteString(m.settingsScreen.View())
		}
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

func enabledForwardCount(forwards []*model.Forward) int {
	count := 0
	for _, forward := range forwards {
		if forward != nil && forward.Enabled {
			count++
		}
	}
	return count
}

func (m *tuiModel) serverByID(id int64) *model.Server {
	for _, server := range m.servers {
		if server.ID == id {
			return server
		}
	}
	return nil
}

func (m *tuiModel) openServerActions(server *model.Server, parent screen, forwards []*model.Forward) (tea.Model, tea.Cmd) {
	m.actionMenu = newActionMenuModel(m.width, m.height, m.sessionsAvailable)
	m.actionMenuParent = parent
	m.screen = screenActionMenu
	if parent == screenForwardList {
		m.actionMenu.setServer(server, enabledForwardCount(forwards), false)
		return m, nil
	}
	m.actionMenu.setServer(server, 0, true)
	return m, func() tea.Msg {
		if ListForwards == nil {
			return actionForwardsLoadedMsg{serverID: server.ID, err: fmt.Errorf("%s", i18n.T("forward storage is unavailable", "хранилище правил проброса недоступно"))}
		}
		items, err := ListForwards(server.ID)
		return actionForwardsLoadedMsg{serverID: server.ID, count: enabledForwardCount(items), err: err}
	}
}

func (m *tuiModel) beginBackgroundTunnel(server *model.Server, origin screen) (tea.Model, tea.Cmd) {
	if m.tunnelStarting {
		return m, nil
	}
	m.tunnelStarting = true
	if origin == screenForwardList && m.forwardScreen != nil {
		m.forwardScreen.err = nil
		m.forwardScreen.notice = i18n.Tf("Starting background tunnel for %s...", "Запуск фонового туннеля для %s...", server.Alias)
	} else {
		m.err = nil
		m.success = i18n.Tf("Starting background tunnel for %s...", "Запуск фонового туннеля для %s...", server.Alias)
	}
	return m, func() tea.Msg {
		if StartBackgroundTunnel == nil {
			return backgroundTunnelStartedMsg{alias: server.Alias, origin: origin, err: fmt.Errorf("%s", i18n.T("background tunnel startup is unavailable", "запуск фонового туннеля недоступен"))}
		}
		state, err := StartBackgroundTunnel(server.Alias)
		return backgroundTunnelStartedMsg{alias: server.Alias, origin: origin, state: state, err: err}
	}
}

func (m *tuiModel) updateActionMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, action := m.actionMenu.Update(msg)
	m.actionMenu = updated

	if msg.Type == tea.KeyEsc {
		m.screen = m.actionMenuParent
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
		case "session_open":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.actionMenu = nil
				m.result = &TUIResult{Server: item.server, Action: "session_open"}
				return m, tea.Quit
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
				origin := m.actionMenuParent
				m.actionMenu = nil
				m.screen = origin
				return m.beginBackgroundTunnel(item.server, origin)
			}
		case "forwards":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				if m.actionMenuParent == screenForwardList && m.forwardScreen != nil {
					m.screen = screenForwardList
					m.actionMenu = nil
					return m, nil
				}
				m.forwardScreen = newForwardScreenModel(item.server.ID, item.server.Alias, m.width, m.height)
				m.screen = screenForwardList
				m.actionMenu = nil
				return m, m.forwardScreen.loadForwards()
			}
		case "route":
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.form = newEditFormModel(item.server, m.width, m.height)
				m.form.setRouteProfiles(m.servers)
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
				m.form.setRouteProfiles(m.servers)
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
		}
		return m, nil
	}

	return m, nil
}

func (m *tuiModel) updateManageMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, action := m.manageMenu.Update(msg)
	m.manageMenu = updated
	if msg.Type == tea.KeyEsc {
		m.screen = screenList
		m.manageMenu = nil
		return m, nil
	}
	if action == nil {
		return m, nil
	}
	m.manageMenu = nil
	switch *action {
	case "groups":
		m.screen = screenGroups
		return m, m.loadGroupsCmd()
	case "tags":
		m.screen = screenTags
		return m, m.loadTagsCmd()
	case "templates":
		m.screen = screenTemplates
		return m, m.loadTemplatesCmd()
	case "settings":
		preference := "auto"
		if GetLanguagePreference != nil {
			preference = GetLanguagePreference()
		}
		m.settingsScreen = newSettingsModel(m.width, m.height, preference)
		m.screen = screenSettings
		return m, nil
	case "sessions":
		m.sessionScreen = newSessionScreenModel(m.width, m.height)
		m.screen = screenSessionManager
		return m, m.sessionScreen.loadSessions()
	case "tunnels":
		m.tunnelScreen = newTunnelScreenModel(m.width, m.height)
		m.screen = screenTunnelManager
		return m, m.tunnelScreen.loadTunnels()
	case "import":
		m.screen = screenList
		return m, func() tea.Msg {
			if ImportServers == nil {
				return importDoneMsg{err: fmt.Errorf("%s", i18n.T("import is unavailable", "импорт недоступен"))}
			}
			count, err := ImportServers()
			if err != nil {
				return importDoneMsg{err: err}
			}
			servers, err := ListServers()
			return importDoneMsg{servers: servers, count: count, err: err}
		}
	case "export":
		m.result = &TUIResult{Action: "export"}
		return m, tea.Quit
	case "vault_lock":
		m.screen = screenList
		if LockVault == nil {
			m.err = fmt.Errorf("%s", i18n.T("vault lock is unavailable", "блокировка хранилища недоступна"))
		} else if err := LockVault(); err != nil {
			m.err = err
		} else {
			m.vaultUnlocked = false
			m.success = i18n.T("Vault locked.", "Хранилище заблокировано.")
		}
	case "vault_change_pw":
		m.result = &TUIResult{Action: "vault_change_pw"}
		return m, tea.Quit
	}
	return m, nil
}

func (m *tuiModel) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsScreen == nil {
		m.screen = screenManageMenu
		m.manageMenu = newManageMenuModel(m.width, m.height, m.sessionsAvailable)
		return m, nil
	}
	selected, changed, back := m.settingsScreen.Update(msg)
	if changed {
		if SetLanguagePreference == nil {
			m.settingsScreen.err = fmt.Errorf("%s", i18n.T("Language settings are unavailable", "Настройки языка недоступны"))
			return m, nil
		}
		if err := SetLanguagePreference(selected); err != nil {
			m.settingsScreen.err = err
			return m, nil
		}
		if err := i18n.SetPreference(selected); err != nil {
			m.settingsScreen.err = err
			return m, nil
		}
		m.settingsScreen.applyPreference(selected)
		m.searchInput.Placeholder = i18n.T("Search...", "Поиск...")
		m.tagList = newStringList(m.tags, i18n.T("Tags", "Теги"), m.width, managerListHeight(m.height))
		m.groupList.Title = i18n.T("Groups", "Группы")
		m.templateList.Title = i18n.T("Command Templates", "Шаблоны команд")
		m.settingsScreen.err = nil
	}
	if back {
		m.settingsScreen = nil
		m.manageMenu = newManageMenuModel(m.width, m.height, m.sessionsAvailable)
		m.screen = screenManageMenu
	}
	return m, nil
}

func (m *tuiModel) updateForwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		m.forwardScreen = nil
		return m, nil
	case tea.KeyCtrlX:
		if m.forwardScreen != nil {
			if server := m.serverByID(m.forwardScreen.serverID); server != nil {
				return m.openServerActions(server, screenForwardList, m.forwardScreen.list)
			}
			m.forwardScreen.err = fmt.Errorf("%s", i18n.T("server profile is no longer available", "профиль сервера больше недоступен"))
		}
		return m, nil
	case tea.KeyCtrlB:
		return m.startForwardListTunnel()
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
	case tea.KeySpace:
		if m.forwardScreen != nil && m.forwardScreen.selected >= 0 && m.forwardScreen.selected < len(m.forwardScreen.list) {
			m.forwardScreen.notice = ""
			selected := *m.forwardScreen.list[m.forwardScreen.selected]
			selected.Enabled = !selected.Enabled
			return m, func() tea.Msg {
				if UpdateForward == nil {
					return forwardsLoadedMsg{err: fmt.Errorf("%s", i18n.T("forward update is unavailable", "изменение правила проброса недоступно"))}
				}
				if err := UpdateForward(&selected); err != nil {
					return forwardsLoadedMsg{err: err}
				}
				forwards, err := ListForwards(m.forwardScreen.serverID)
				return forwardsLoadedMsg{forwards: forwards, err: err}
			}
		}
	case tea.KeyRunes:
		switch msg.String() {
		case "b", "B":
			return m.startForwardListTunnel()
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

func (m *tuiModel) startForwardListTunnel() (tea.Model, tea.Cmd) {
	if m.forwardScreen == nil || m.tunnelStarting {
		return m, nil
	}
	server := m.serverByID(m.forwardScreen.serverID)
	if server == nil {
		m.forwardScreen.err = fmt.Errorf("%s", i18n.T("server profile is no longer available", "профиль сервера больше недоступен"))
		return m, nil
	}
	if enabledForwardCount(m.forwardScreen.list) == 0 {
		m.forwardScreen.err = fmt.Errorf("%s", i18n.T("no enabled port forwards; add or enable a rule before starting a tunnel", "нет включённых правил проброса; добавьте или включите правило перед запуском туннеля"))
		return m, nil
	}
	if server.AuthMethod == model.AuthPassword || server.AuthMethod == model.AuthKeyPassphrase {
		m.forwardScreen.err = fmt.Errorf("%s", i18n.T("background mode needs key or agent authentication; use Ctrl+X for a foreground mode", "для фонового режима нужен ключ или SSH-агент; Ctrl+X открывает активный режим"))
		return m, nil
	}
	return m.beginBackgroundTunnel(server, screenForwardList)
}

func (m *tuiModel) updateSessionManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenList
		m.sessionScreen = nil
		return m, nil
	case tea.KeyEnter:
		if m.sessionScreen != nil {
			if selected := m.sessionScreen.selected(); selected != nil {
				m.result = &TUIResult{Action: "session_attach", SessionID: selected.ID}
				return m, tea.Quit
			}
		}
	case tea.KeyCtrlD:
		m.confirmSessionClose()
		return m, nil
	case tea.KeyCtrlR:
		if m.sessionScreen != nil {
			return m, m.sessionScreen.loadSessions()
		}
	case tea.KeyRunes:
		switch msg.String() {
		case "d", "D":
			m.confirmSessionClose()
			return m, nil
		case "r", "R":
			if m.sessionScreen != nil {
				return m, m.sessionScreen.loadSessions()
			}
		}
	}
	if m.sessionScreen != nil {
		var cmd tea.Cmd
		m.sessionScreen.list, cmd = m.sessionScreen.list.Update(msg)
		return m, cmd
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
		innerHeight := max(1, height-2)
		message := wrapCells(m.confirm.target, innerWidth)
		if m.confirm.consequence != "" {
			message = append(message, "")
			message = append(message, wrapCells(m.confirm.consequence, innerWidth)...)
		}
		cancel := i18n.T("[ Cancel ]", "[ Отмена ]")
		accept := "[ " + m.confirm.verb + " ]"
		if m.confirm.focus == confirmCancel {
			cancel = selectedStyle.Render("> " + cancel)
		} else {
			accept = errorStyle.Render("> " + accept)
		}
		action := cancel + "  " + accept
		if m.confirm.pending {
			action = i18n.Tf("%s in progress…", "%s: выполняется…", m.confirm.verb)
		}
		messageRows := max(0, innerHeight-2)
		if len(message) > messageRows {
			message = message[:messageRows]
			if len(message) > 0 {
				message[len(message)-1] = truncateCells(strings.TrimSpace(message[len(message)-1])+" …", innerWidth)
			}
		}
		lines := []string{dashboardSection(m.confirm.title)}
		lines = append(lines, message...)
		for len(lines) < innerHeight-1 {
			lines = append(lines, "")
		}
		lines = append(lines, action)
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb: i18n.T("Confirm", "Подтверждение"),
		status:     shellStatus(m.vaultUnlocked, i18n.T("Action required", "Требуется действие")),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "Tab", Action: i18n.T("choose", "выбрать")},
			{Key: "Enter", Action: i18n.T("activate", "подтвердить")},
			{Key: "Esc", Action: i18n.T("cancel", "отмена")},
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
		title:       i18n.T("Discard unsaved changes?", "Отменить несохранённые изменения?"),
		target:      title,
		consequence: i18n.T("Your edits on this form will be lost.", "Изменения в форме будут потеряны."),
		verb:        i18n.T("Discard", "Отменить изменения"),
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
		title:       i18n.T("Delete server profile?", "Удалить профиль сервера?"),
		target:      fmt.Sprintf("%q", alias),
		consequence: i18n.T("This also removes its saved port forwards and vault secrets.", "Также удалятся сохранённые правила проброса и секреты хранилища."),
		verb:        i18n.T("Delete", "Удалить"),
		parent:      screenList,
		action: func() tea.Cmd {
			return func() tea.Msg {
				if DeleteServer == nil {
					return serverDeletedMsg{alias: alias, err: fmt.Errorf("%s", i18n.T("server deletion is unavailable", "удаление сервера недоступно"))}
				}
				if err := DeleteServer(alias); err != nil {
					return serverDeletedMsg{alias: alias, err: err}
				}
				if ListServers == nil {
					return serverDeletedMsg{alias: alias, deleted: true, err: fmt.Errorf("%s", i18n.T("server reload is unavailable", "обновление серверов недоступно"))}
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
		title:       i18n.T("Delete port forward?", "Удалить правило проброса порта?"),
		target:      fmt.Sprintf("%q · %s → %s", name, fwd.ForwardListen(), fwd.ForwardTarget()),
		consequence: i18n.T("This removes the saved forwarding rule. Active tunnels are not stopped.", "Сохранённое правило будет удалено. Работающие туннели не остановятся."),
		verb:        i18n.T("Delete", "Удалить"),
		parent:      screenForwardList,
		action: func() tea.Cmd {
			return func() tea.Msg {
				if DeleteForward == nil {
					return forwardDeletedMsg{id: id, err: fmt.Errorf("%s", i18n.T("forward deletion is unavailable", "удаление правила проброса недоступно"))}
				}
				return forwardDeletedMsg{id: id, err: DeleteForward(id)}
			}
		},
	})
}

func (m *tuiModel) confirmSessionClose() {
	if m.sessionScreen == nil {
		return
	}
	selected := m.sessionScreen.selected()
	if selected == nil {
		return
	}
	m.beginConfirm(confirmState{
		title:       i18n.T("Close SSH session?", "Закрыть SSH-сессию?"),
		target:      fmt.Sprintf("%q · tmux %s", selected.ServerAlias, selected.ID),
		consequence: i18n.T("The interactive SSH process in this tmux window will be terminated.", "Интерактивный процесс SSH в этом окне tmux будет завершён."),
		verb:        i18n.T("Close", "Закрыть"),
		parent:      screenSessionManager,
		action: func() tea.Cmd {
			return m.sessionScreen.closeSelected()
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
		title:       i18n.T("Stop running tunnel?", "Остановить работающий туннель?"),
		target:      fmt.Sprintf("%q · PID %d · %s", state.Name, state.PID, state.ServerAlias),
		consequence: i18n.T("Active forwarded connections through this process will close.", "Активные соединения через этот процесс закроются."),
		verb:        i18n.T("Stop", "Остановить"),
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
	case screenForm, screenSearch, screenTagInput, screenGroupInput, screenTemplateForm, screenForwardForm:
		return true
	default:
		return false
	}
}

func (m *tuiModel) updateForwardForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.forwardForm != nil && m.forwardForm.Dirty() {
			m.confirmDiscard(i18n.T("Port forward", "Правило проброса"), screenForwardForm, screenForwardList)
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
		return errorStyle.Render(i18n.T("Error: ", "Ошибка: ") + m.err.Error())
	}
	if m.success != "" {
		return successStyle.Render(m.success)
	}
	return ""
}

func (m *tuiModel) viewSearch() string {
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Search", "Поиск"),
		status:       shellStatus(m.vaultUnlocked, i18n.Tf("%d profiles", "Профилей: %d", len(m.servers))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{
				dashboardSection(i18n.T("Find server", "Найти сервер")),
				"",
				m.searchInput.View(),
				"",
				dashboardHelp(i18n.T("Search alias, host, display name, group, tags, notes, and route.", "Поиск по псевдониму, хосту, имени, группе, тегам, заметкам и маршруту.")),
			})
		},
		footer: []helpItem{{Key: i18n.T("Type", "Ввод"), Action: i18n.T("search", "поиск")}, {Key: "Enter", Action: i18n.T("confirm", "подтвердить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("cancel", "отмена")}},
	})
}

func (m *tuiModel) viewInlineBackgroundResults() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render(i18n.T("Last Background Run", "Последний фоновый запуск")))
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
			b.WriteString(helpStyle.Render(i18n.T("  Output: ", "  Вывод: ") + result.Alias))
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
	b.WriteString(sectionStyle.Render(i18n.T("Selected", "Выбрано")))
	b.WriteString("\n")
	b.WriteString(i18n.Tf("  Alias: %s\n", "  Псевдоним: %s\n", server.Alias))
	b.WriteString(i18n.Tf("  Display Name: %s\n", "  Имя: %s\n", displayName))
	b.WriteString(i18n.Tf("  Host: %s\n", "  Хост: %s\n", server.Host))
	b.WriteString(i18n.Tf("  Port: %d\n", "  Порт: %d\n", server.Port))
	b.WriteString(i18n.Tf("  User: %s\n", "  Пользователь: %s\n", server.User))
	target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
	b.WriteString(i18n.Tf("  Target: %s\n", "  Цель: %s\n", target))
	b.WriteString(i18n.Tf("  Auth: %s\n", "  Авторизация: %s\n", authLabel(server.AuthMethod)))
	if len(server.Route.Hops) > 0 {
		b.WriteString(i18n.Tf("  Route: %s\n", "  Маршрут: %s\n", server.Route.DisplaySummary(target)))
	} else if server.ProxyJump != "" {
		b.WriteString(fmt.Sprintf("  ProxyJump: %s\n", server.ProxyJump))
	}
	b.WriteString(i18n.Tf("  Group: %s\n", "  Группа: %s\n", group))
	if len(server.Tags) > 0 {
		b.WriteString(i18n.Tf("  Tags: %s\n", "  Теги: %s\n", strings.Join(server.Tags, ", ")))
	}
	if server.StartupCommand != "" {
		b.WriteString(i18n.Tf("  Startup: %s\n", "  Команда запуска: %s\n", server.StartupCommand))
	}
	b.WriteString(i18n.Tf("  Status: %s\n", "  Статус: %s\n", testStatusLabel(server)))
	return b.String()
}

func (m *tuiModel) viewTags() string {
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Tags", "Теги"),
		status:       shellStatus(m.vaultUnlocked, i18n.Tf("%d tags", "Тегов: %d", len(m.tags))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			if len(m.tags) == 0 {
				return renderPaddedPanel(width, height, []string{dashboardHelp(i18n.T("No tags yet. Ctrl+A adds one to the selected servers.", "Тегов пока нет. Ctrl+A добавит тег выбранным серверам."))})
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
		footer: []helpItem{{Key: "Enter", Action: i18n.T("toggle", "вкл/выкл")}, {Key: "Ctrl+A", Action: i18n.T("add", "добавить")}, {Key: "Ctrl+E", Action: i18n.T("rename", "переименовать")}, {Key: "Ctrl+D", Action: i18n.T("delete", "удалить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("back", "назад")}},
	})
}

func (m *tuiModel) viewTagInput() string {
	title := i18n.T("Add Tag", "Добавить тег")
	if m.tagMode == "rename" {
		title = i18n.T("Rename Tag", "Переименовать тег")
	}
	return renderScreenShell(screenShell{
		breadcrumb: title,
		status:     shellStatus(m.vaultUnlocked, i18n.T("Tag editor", "Редактор тегов")),
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{dashboardSection(title), "", m.tagInput.View()})
		},
		footer: []helpItem{{Key: "Enter", Action: i18n.T("save", "сохранить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("cancel", "отмена")}},
	})
}

func (m *tuiModel) viewGroups() string {
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Groups", "Группы"),
		status:       shellStatus(m.vaultUnlocked, i18n.Tf("%d groups", "Групп: %d", len(m.groups))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			if len(m.groups) == 0 {
				return renderPaddedPanel(width, height, []string{dashboardHelp(i18n.T("No groups yet. Ctrl+A creates one.", "Групп пока нет. Ctrl+A создаст группу."))})
			}
			capacity := max(1, height-2)
			start, end := visibleServerRange(len(m.groupList.Items()), m.groupList.Index(), capacity)
			lines := make([]string, 0, capacity)
			for index := start; index < end; index++ {
				item, ok := m.groupList.Items()[index].(groupManagerItem)
				if !ok || item.group == nil {
					continue
				}
				marker := "  "
				if index == m.groupList.Index() {
					marker = "> "
				}
				lines = append(lines, i18n.Tf("%s%-28s %d server(s)", "%s%-28s серверов: %d", marker, item.group.Name, item.group.ServerCount))
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Ctrl+A", Action: i18n.T("add", "добавить")}, {Key: "Ctrl+E", Action: i18n.T("rename", "переименовать")}, {Key: "Ctrl+D", Action: i18n.T("delete", "удалить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("back", "назад")}},
	})
}

func (m *tuiModel) viewGroupInput() string {
	title := i18n.T("Add Group", "Добавить группу")
	if m.groupMode == "rename" {
		title = i18n.T("Rename Group", "Переименовать группу")
	}
	return renderScreenShell(screenShell{
		breadcrumb: title,
		status:     shellStatus(m.vaultUnlocked, i18n.T("Group editor", "Редактор группы")),
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{dashboardSection(title), "", m.groupInput.View()})
		},
		footer: []helpItem{{Key: "Enter", Action: i18n.T("save", "сохранить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("cancel", "отмена")}},
	})
}

func (m *tuiModel) viewTemplates() string {
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Command Templates", "Шаблоны команд"),
		status:       shellStatus(m.vaultUnlocked, i18n.Tf("%d templates", "Шаблонов: %d", len(m.templates))),
		notification: m.rootNotification(),
		width:        m.width,
		height:       m.height,
		body: func(width, height int) string {
			if len(m.templates) == 0 {
				return renderPaddedPanel(width, height, []string{dashboardHelp(i18n.T("No command templates yet. Ctrl+A adds one.", "Шаблонов команд пока нет. Ctrl+A добавит шаблон."))})
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
				line := marker + tpl.template.Name + "  " + tpl.template.Command
				if tpl.template.Description != "" && classifyShellContent(width) != sizeNarrow {
					line += "  — " + tpl.template.Description
				}
				lines = append(lines, line)
			}
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{{Key: "Ctrl+A", Action: i18n.T("add", "добавить")}, {Key: "Ctrl+E", Action: i18n.T("edit", "изменить")}, {Key: "Ctrl+D", Action: i18n.T("delete", "удалить")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("back", "назад")}},
	})
}

func (m *tuiModel) viewTemplatePicker() string {
	targets := strings.Join(serverAliases(m.targetServers()), ", ")
	return renderScreenShell(screenShell{
		breadcrumb: i18n.T("Run Template", "Запуск шаблона"),
		status:     i18n.T("Targets: ", "Серверы: ") + targets,
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			lines := []string{dashboardSection(i18n.T("Choose template", "Выберите шаблон")), dashboardHelp(i18n.T("Targets: ", "Серверы: ") + targets), ""}
			if len(m.templates) == 0 {
				lines = append(lines, dashboardHelp(i18n.T("No command templates. Press Esc, then Ctrl+P to add one.", "Шаблонов нет. Нажмите Esc, затем Ctrl+P для добавления.")))
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
		footer: []helpItem{{Key: "Enter", Action: i18n.T("choose", "выбрать")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("back", "назад")}},
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
		breadcrumb: i18n.T("Run Template / Mode", "Запуск шаблона / Режим"),
		status:     i18n.T("Targets: ", "Серверы: ") + targets,
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{dashboardSection(i18n.T("Execution", "Выполнение")), "", i18n.T("Template: ", "Шаблон: ") + name, i18n.T("Command: ", "Команда: ") + command, i18n.T("Targets: ", "Серверы: ") + targets, "", i18n.T("Choose foreground for an interactive run or background to keep using sshkeeper.", "Выберите активный запуск для интерактивной работы или фоновый, чтобы продолжить использовать sshkeeper.")})
		},
		footer: []helpItem{{Key: "Ctrl+F (Enter)", Action: i18n.T("Foreground", "Активный")}, {Key: "Ctrl+B", Action: i18n.T("Background", "Фоновый")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}, {Key: "Esc", Action: i18n.T("back", "назад")}},
	})
}

func (m *tuiModel) viewBackgroundResults() string {
	return renderScreenShell(screenShell{
		breadcrumb: i18n.T("Background Results", "Результаты фонового запуска"),
		status:     i18n.Tf("%d results", "Результатов: %d", len(m.bgResults)),
		width:      m.width,
		height:     m.height,
		body: func(width, height int) string {
			lines := make([]string, 0)
			for _, result := range m.bgResults {
				status := "OK"
				if result.Err != "" {
					status = i18n.T("FAIL: ", "ОШИБКА: ") + result.Err
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
		footer: []helpItem{{Key: "Enter/Esc", Action: i18n.T("back", "назад")}, {Key: "Ctrl+H", Action: i18n.T("help", "справка")}},
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

func (m *tuiModel) loadGroupsCmd() tea.Cmd {
	return func() tea.Msg {
		if ListGroups == nil {
			return groupsLoadedMsg{err: fmt.Errorf("%s", i18n.T("group storage is unavailable", "хранилище групп недоступно"))}
		}
		groups, err := ListGroups()
		return groupsLoadedMsg{groups: groups, err: err}
	}
}

func (m *tuiModel) setGroups(groups []*model.Group) {
	m.groups = groups
	items := make([]list.Item, len(groups))
	for i, group := range groups {
		items[i] = groupManagerItem{group: group}
	}
	l := list.New(items, list.NewDefaultDelegate(), m.width, managerListHeight(m.height))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Title = i18n.T("Groups", "Группы")
	l.Styles.Title = titleStyle
	m.groupList = l
}

func (m *tuiModel) removeGroup(name string) {
	groups := make([]*model.Group, 0, len(m.groups))
	for _, group := range m.groups {
		if group != nil && group.Name != name {
			groups = append(groups, group)
		}
	}
	m.setGroups(groups)
}

func (m *tuiModel) loadTemplatesCmd() tea.Cmd {
	return func() tea.Msg {
		if ListCommandTemplates == nil {
			return templatesLoadedMsg{err: fmt.Errorf("%s", i18n.T("template storage is unavailable", "хранилище шаблонов недоступно"))}
		}
		templates, err := ListCommandTemplates()
		return templatesLoadedMsg{templates: templates, err: err}
	}
}

func (m *tuiModel) loadTagsCmd() tea.Cmd {
	return func() tea.Msg {
		if ListTags == nil {
			return tagsLoadedMsg{err: fmt.Errorf("%s", i18n.T("tag storage is unavailable", "хранилище тегов недоступно"))}
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
	l.Title = i18n.T("Command Templates", "Шаблоны команд")
	l.Styles.Title = titleStyle
	m.templateList = l
}

func (m *tuiModel) setTags(tags []string) {
	m.tags = tags
	m.tagList = newStringList(tags, i18n.T("Tags", "Теги"), m.width, managerListHeight(m.height))
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
	width := m.width - 3
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
	insAction := i18n.T("select", "выбрать")
	if selectedCount > 0 {
		insAction = i18n.Tf("select (%d selected)", "выбрать (отмечено: %d)", selectedCount)
	}
	var items []helpItem
	if hasBackgroundResult {
		items = append(items, helpItem{Key: "Esc", Action: i18n.T("clear result", "убрать результат")})
	}
	items = append(items,
		helpItem{Key: "Enter", Action: i18n.T("connect", "подключиться")},
		helpItem{Key: "Ctrl+X", Action: i18n.T("server actions", "действия с сервером")},
		helpItem{Key: "Ctrl+W", Action: i18n.T("forward rules", "правила проброса")},
		helpItem{Key: "m", Action: i18n.T("manage", "управление")},
		helpItem{Key: "Ctrl+A", Action: i18n.T("add", "добавить")},
		helpItem{Key: "Ctrl+E", Action: i18n.T("edit", "изменить")},
		helpItem{Key: "Ctrl+F", Action: i18n.T("search", "поиск")},
		helpItem{Key: "Ins", Action: insAction},
		helpItem{Key: "?", Action: i18n.T("hotkeys", "клавиши")},
		helpItem{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
		helpItem{Key: "Ctrl+Q", Action: i18n.T("quit", "выход")},
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
