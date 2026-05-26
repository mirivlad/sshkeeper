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

	normalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

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

func (i serverItem) Title() string       { return i.server.Alias }
func (i serverItem) Description() string { return fmt.Sprintf("%s@%s:%d  %s", i.server.User, i.server.Host, i.server.Port, i.server.AuthMethod) }
func (i serverItem) FilterValue() string { return i.server.Alias + " " + i.server.DisplayName + " " + i.server.Host + " " + i.server.User }

// --- External callbacks ---

var (
	ListServers    func() ([]*model.Server, error)
	SearchServers  func(query string) ([]*model.Server, error)
	DeleteServer   func(alias string) error
	TestConnection func(server *model.Server) (bool, string)
	SaveServer     func(server *model.Server, password string) error
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
	Server  *model.Server
	Action  string // "connect"
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
		}
		return m, nil

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

func matchKeys(msg tea.KeyMsg, en, ru string) bool {
	if len(msg.Runes) != 1 {
		return false
	}
	r := msg.Runes[0]
	return r == []rune(en)[0] || r == []rune(ru)[0]
}

func (m *tuiModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check key by runes (layout-independent)
	if msg.Type == tea.KeyRunes {
		switch {
		case matchKeys(msg, "q", "й"):
			return m, tea.Quit
		case matchKeys(msg, "/", "?"):
			m.screen = screenSearch
			m.searchInput.Focus()
			return m, nil
		case matchKeys(msg, "a", "ф"):
			m.form = newFormModel(m.width, m.height)
			m.screen = screenForm
			return m, nil
		case matchKeys(msg, "e", "у"):
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				m.form = newEditFormModel(item.server, m.width, m.height)
				m.screen = screenForm
			}
			return m, nil
		case matchKeys(msg, "d", "в"):
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
		case matchKeys(msg, "t", "е"):
			if item, ok := m.list.SelectedItem().(serverItem); ok {
				return m, func() tea.Msg {
					ok, testErr := TestConnection(item.server)
					return testDoneMsg{ok: ok, err: testErr}
				}
			}
		}
	}

	switch msg.Type {
	case tea.KeyEnter:
		if item, ok := m.list.SelectedItem().(serverItem); ok {
			return m, func() tea.Msg {
				return connectRequestMsg{server: item.server}
			}
		}
	case tea.KeyCtrlC:
		return m, tea.Quit
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
		m.screen = screenList
		m.form = nil
		m.err = nil
		m.success = ""
		return m, nil
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
		b.WriteString(m.list.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Enter connect | a add | e edit | d delete | t test | / search | q quit"))

	case screenSearch:
		b.WriteString("Search: " + m.searchInput.View() + "\n")
		b.WriteString(helpStyle.Render("Enter search | Esc cancel"))

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

// --- Form model ---

type formModel struct {
	edit           bool
	server         *model.Server
	inputs         []textinput.Model
	password       textinput.Model
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
}

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
		"Group",
		"Notes",
	}
	for i, label := range labels {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = label
		inputs[i].CharLimit = 128
	}

	pw := textinput.New()
	pw.Placeholder = "Password / Passphrase (stored in vault)"
	pw.CharLimit = 256
	pw.EchoMode = textinput.EchoPassword

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	inputs[0].Focus()

	return &formModel{
		inputs:   inputs,
		password: pw,
		focusIdx: 0,
		spinner:  s,
		width:    w,
		height:   h,
	}
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
		fm.inputs[i].Prompt = blurredStyle.Render(fm.inputs[i].Placeholder + ": ")
	}
	fm.password.Blur()
	fm.password.Prompt = blurredStyle.Render(fm.password.Placeholder + ": ")

	if fm.focusIdx < len(fm.inputs) {
		fm.inputs[fm.focusIdx].Focus()
		fm.inputs[fm.focusIdx].Prompt = focusedStyle.Render(fm.inputs[fm.focusIdx].Placeholder + "> ")
	} else if fm.focusIdx == len(fm.inputs) {
		fm.password.Focus()
		fm.password.Prompt = focusedStyle.Render(fm.password.Placeholder + "> ")
	}
}

func (fm *formModel) runTest() tea.Cmd {
	fm.testing = true
	fm.testResult = ""
	fm.err = nil
	fm.saved = false

	s := fm.buildServer()
	return tea.Batch(
		fm.spinner.Tick,
		func() tea.Msg {
			if s.AuthMethod == model.AuthPassword && fm.password.Value() == "" {
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
			err := SaveServer(s, pw)
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

	for i := range fm.inputs {
		b.WriteString(fm.inputs[i].View())
		b.WriteString("\n")
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

	b.WriteString("\n" + testBtn + "  " + saveBtn + "\n\n")
	b.WriteString(helpStyle.Render("Tab/↓ next | ↑ prev | Enter select | Esc back"))

	return b.String()
}
