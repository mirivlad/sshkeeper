package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// groupItem implements list.Item for dropdowns (groups, auth methods, etc.)
type groupItem struct {
	name string
}

func (i groupItem) Title() string       { return i.name }
func (i groupItem) Description() string { return "" }
func (i groupItem) FilterValue() string { return i.name }

type routeProfileItem struct {
	server *model.Server
}

func (i routeProfileItem) Title() string { return i.server.Alias }
func (i routeProfileItem) Description() string {
	target := i.server.Host
	if i.server.User != "" {
		target = i.server.User + "@" + i.server.Host
	}
	return fmt.Sprintf("%s:%d", target, i.server.Port)
}
func (i routeProfileItem) FilterValue() string {
	return strings.Join([]string{i.server.Alias, i.server.DisplayName, i.server.Host, i.server.User, i.server.GroupName}, " ")
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

func (fm *formModel) setRouteProfiles(servers []*model.Server) {
	fm.routeProfiles = nil
	items := []list.Item{}
	for _, server := range servers {
		if server == nil {
			continue
		}
		if fm.server != nil && ((fm.server.ID > 0 && server.ID == fm.server.ID) || server.Alias == fm.server.Alias) {
			continue
		}
		fm.routeProfiles = append(fm.routeProfiles, server)
		items = append(items, routeProfileItem{server: server})
	}
	l := list.New(items, list.NewDefaultDelegate(), 44, 14)
	l.Title = "Available server profiles"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	fm.routeList = l
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
	groups         []string
	groupList      list.Model
	showGroupList  bool
	authList       list.Model
	showAuthList   bool
	routeProfiles  []*model.Server
	routeList      list.Model
	showRouteList  bool
	routePane      int // 0=current route, 1=available profiles
	routeCursor    int
	initial        formSnapshot
}

type formSnapshot struct {
	values   []string
	password string
}

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
		"Route (direct / ordered bastions)",
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
	inputs[3].SetValue("22")

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

	if GetGroups != nil {
		if groups, err := GetGroups(); err == nil && len(groups) > 0 {
			fm.groups = groups
			fm.groupList = newStringList(groups, "Select group", 30, 8)
		}
	}
	fm.updateFocus()
	fm.initial = fm.snapshot()
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
	case "Route (direct / ordered bastions)":
		return "profile:bastion, raw:user@gw.example"
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

	// Store an unambiguous route spec in the editable field.
	if len(s.Route.Hops) > 0 {
		fm.inputs[7].SetValue(model.FormatRouteSpec(s.Route))
	} else if s.ProxyJump != "" {
		fm.inputs[7].SetValue(s.ProxyJump)
	}

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
	fm.initial = fm.snapshot()
	return fm
}

func (fm *formModel) snapshot() formSnapshot {
	values := make([]string, len(fm.inputs))
	for i := range fm.inputs {
		values[i] = fm.inputs[i].Value()
	}
	return formSnapshot{values: values, password: fm.password.Value()}
}

func (fm *formModel) Dirty() bool {
	current := fm.snapshot()
	if current.password != fm.initial.password || len(current.values) != len(fm.initial.values) {
		return true
	}
	for i := range current.values {
		if current.values[i] != fm.initial.values[i] {
			return true
		}
	}
	return false
}

func (fm *formModel) resolveRouteAlias(alias string) (int64, bool) {
	alias = strings.TrimSpace(alias)
	for _, server := range fm.routeProfiles {
		if server != nil && server.Alias == alias && server.ID > 0 {
			return server.ID, true
		}
	}
	if ResolveRouteAlias != nil {
		return ResolveRouteAlias(alias)
	}
	return 0, false
}

func (fm *formModel) parseRouteInput() (model.Route, error) {
	return model.ParseRouteSpec(fm.inputs[7].Value(), fm.resolveRouteAlias)
}

func (fm *formModel) currentRoute() model.Route {
	route, err := fm.parseRouteInput()
	if err != nil {
		return model.Route{}
	}
	return route
}

func (fm *formModel) setCurrentRoute(route model.Route) {
	fm.inputs[7].SetValue(model.FormatRouteSpec(route))
	if len(route.Hops) == 0 {
		fm.routeCursor = 0
	} else if fm.routeCursor >= len(route.Hops) {
		fm.routeCursor = len(route.Hops) - 1
	}
}

func (fm *formModel) routeContainsProfile(route model.Route, serverID int64) bool {
	for _, hop := range route.Hops {
		if hop.Profile() && hop.ServerID == serverID {
			return true
		}
	}
	return false
}

func (fm *formModel) updateRouteEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if fm.routePane == 1 {
			var cmd tea.Cmd
			fm.routeList, cmd = fm.routeList.Update(msg)
			return fm, cmd
		}
		return fm, nil
	}
	if key.Type == tea.KeyEsc {
		fm.showRouteList = false
		fm.inputs[7].Focus()
		return fm, nil
	}
	if key.Type == tea.KeyTab || key.Type == tea.KeyShiftTab {
		if fm.routePane == 0 {
			fm.routePane = 1
		} else {
			fm.routePane = 0
		}
		return fm, nil
	}

	route := fm.currentRoute()
	if fm.routePane == 0 {
		switch key.Type {
		case tea.KeyUp:
			if fm.routeCursor > 0 {
				fm.routeCursor--
			}
			return fm, nil
		case tea.KeyDown:
			if fm.routeCursor+1 < len(route.Hops) {
				fm.routeCursor++
			}
			return fm, nil
		case tea.KeyBackspace, tea.KeyDelete:
			if len(route.Hops) > 0 && fm.routeCursor < len(route.Hops) {
				route.Hops = append(route.Hops[:fm.routeCursor], route.Hops[fm.routeCursor+1:]...)
				fm.setCurrentRoute(route)
			}
			return fm, nil
		case tea.KeyRunes:
			switch key.String() {
			case "x", "X":
				if len(route.Hops) > 0 && fm.routeCursor < len(route.Hops) {
					route.Hops = append(route.Hops[:fm.routeCursor], route.Hops[fm.routeCursor+1:]...)
					fm.setCurrentRoute(route)
				}
				return fm, nil
			case "[":
				if fm.routeCursor > 0 && fm.routeCursor < len(route.Hops) {
					i := fm.routeCursor
					route.Hops[i-1], route.Hops[i] = route.Hops[i], route.Hops[i-1]
					fm.routeCursor--
					fm.setCurrentRoute(route)
				}
				return fm, nil
			case "]":
				if fm.routeCursor >= 0 && fm.routeCursor+1 < len(route.Hops) {
					i := fm.routeCursor
					route.Hops[i], route.Hops[i+1] = route.Hops[i+1], route.Hops[i]
					fm.routeCursor++
					fm.setCurrentRoute(route)
				}
				return fm, nil
			}
		}
		return fm, nil
	}

	if key.Type == tea.KeyEnter {
		if item, ok := fm.routeList.SelectedItem().(routeProfileItem); ok && item.server != nil {
			if !fm.routeContainsProfile(route, item.server.ID) {
				route.Hops = append(route.Hops, model.RouteHop{ServerID: item.server.ID, Alias: item.server.Alias, IsProfile: true})
				fm.routeCursor = len(route.Hops) - 1
				fm.setCurrentRoute(route)
			}
			return fm, nil
		}
	}
	var cmd tea.Cmd
	fm.routeList, cmd = fm.routeList.Update(msg)
	return fm, cmd
}

func (fm *formModel) Init() tea.Cmd {
	return nil
}

func (fm *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			fm.applySaveError(msg.err)
			fm.saved = false
		} else {
			fm.saved = true
			fm.savedTime = time.Now()
			fm.err = nil
		}
		return fm, nil
	}

	if fm.testing || fm.saving {
		var cmd tea.Cmd
		fm.spinner, cmd = fm.spinner.Update(msg)
		if _, ok := msg.(tea.KeyMsg); ok {
			return fm, cmd
		}
		return fm, cmd
	}

	if fm.showRouteList {
		return fm.updateRouteEditor(msg)
	}

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
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' && !msg.Alt && fm.focusIdx == 7 {
				fm.showRouteList = true
				fm.routePane = 1
				route := fm.currentRoute()
				if len(route.Hops) > 0 {
					fm.routeCursor = len(route.Hops) - 1
				}
				return fm, nil
			}
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

func (fm *formModel) applySaveError(err error) {
	fm.err = err
	if err == nil {
		return
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "alias is required"):
		fm.focusIdx = 0
	case strings.Contains(message, "host is required"):
		fm.focusIdx = 2
	case strings.Contains(message, "port"):
		fm.focusIdx = 3
	case strings.Contains(message, "route"):
		fm.focusIdx = 7
	default:
		return
	}
	fm.updateFocus()
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
		switch index {
		case 0:
			return "Alias *"
		case 2:
			return "Host *"
		case 3:
			return "Port *"
		}
		if index == 5 {
			return "Auth Method (/ pick)"
		}
		if index == 7 {
			return "Route (/ edit)"
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

	if _, err := parsePort(fm.inputs[3].Value()); err != nil {
		fm.testing = false
		return func() tea.Msg { return testDoneMsg{ok: false, err: err.Error()} }
	}
	s, err := fm.buildServerValidated()
	if err != nil {
		fm.testing = false
		return func() tea.Msg { return testDoneMsg{ok: false, err: err.Error()} }
	}
	pw := fm.password.Value()

	return tea.Batch(
		fm.spinner.Tick,
		func() tea.Msg {
			if TestConnectionWithPassword != nil {
				ok, testErr := TestConnectionWithPassword(s, pw)
				return testDoneMsg{ok: ok, err: testErr}
			}
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

	if _, err := parsePort(fm.inputs[3].Value()); err != nil {
		return func() tea.Msg { return saveDoneMsg{err: err} }
	}
	s, err := fm.buildServerValidated()
	if err != nil {
		return func() tea.Msg { return saveDoneMsg{err: err} }
	}
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

// parseRouteHops parses an explicit route spec. Exact known aliases become
// stable profile references; unknown unprefixed values remain raw OpenSSH targets.
func parseRouteHops(input string) (model.Route, error) {
	return model.ParseRouteSpec(input, ResolveRouteAlias)
}

func (fm *formModel) buildServerValidated() (*model.Server, error) {
	port, err := parsePort(fm.inputs[3].Value())
	if err != nil {
		return nil, err
	}
	authMethod := model.AuthMethod(strings.TrimSpace(fm.inputs[5].Value()))
	if authMethod == "" {
		authMethod = model.AuthKey
	}
	route, err := fm.parseRouteInput()
	if err != nil {
		return nil, fmt.Errorf("route: %w", err)
	}
	server := &model.Server{
		Alias:          strings.TrimSpace(fm.inputs[0].Value()),
		DisplayName:    strings.TrimSpace(fm.inputs[1].Value()),
		Host:           strings.TrimSpace(fm.inputs[2].Value()),
		Port:           port,
		User:           strings.TrimSpace(fm.inputs[4].Value()),
		AuthMethod:     authMethod,
		IdentityFile:   strings.TrimSpace(fm.inputs[6].Value()),
		ProxyJump:      route.ProxyJumpString(),
		Route:          route,
		GroupName:      strings.TrimSpace(fm.inputs[8].Value()),
		Notes:          fm.inputs[9].Value(),
		StartupCommand: fm.inputs[10].Value(),
		Tags:           splitCSV(fm.inputs[11].Value()),
	}
	if fm.edit && fm.server != nil {
		server.ID = fm.server.ID
	}
	if err := model.ValidateServerBasics(server); err != nil {
		return nil, err
	}
	return server, nil
}

// buildServer is kept as a convenience for view/tests that already populate
// valid fields. Save/Test paths use buildServerValidated and surface errors.
func (fm *formModel) buildServer() *model.Server {
	server, _ := fm.buildServerValidated()
	return server
}

func parsePort(value string) (int, error) {
	value = strings.TrimSpace(value)
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("Port must be a number from 1 to 65535")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("Port must be between 1 and 65535")
	}
	return port, nil
}

func (fm *formModel) View() string {
	title := "Add Server"
	if fm.edit {
		title = "Edit Server: " + fm.server.Alias
	}
	if fm.showRouteList {
		return fm.routeEditorView(title)
	}
	if fm.showAuthList || fm.showGroupList {
		var dropdown list.Model
		fieldIndex := 8
		if fm.showAuthList {
			dropdown = fm.authList
			fieldIndex = 5
		} else {
			dropdown = fm.groupList
		}
		return renderScreenShell(screenShell{
			breadcrumb: title + " / Picker",
			status:     "Choose a value",
			width:      fm.width,
			height:     fm.height,
			body: func(width, height int) string {
				lines := []string{fm.inputs[fieldIndex].View(), ""}
				lines = append(lines, splitBlock(renderDropdown(dropdown))...)
				return renderPaddedPanel(width, height, lines)
			},
			footer: []helpItem{{Key: "↑/↓", Action: "move"}, {Key: "Enter", Action: "select"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "cancel"}},
		})
	}

	status := fm.formStatusLine()
	testBtn, saveBtn := "  [ Test ]", "  [ Save ]"
	if fm.focusIdx == len(fm.inputs)+1 {
		testBtn = selectedStyle.Render("> [ Test ]")
	}
	if fm.focusIdx == len(fm.inputs)+2 {
		saveBtn = selectedStyle.Render("> [ Save ]")
	}
	actions := testBtn + "  " + saveBtn

	body := func(width, height int) string {
		richLayout := width >= 90 && height >= 20
		allFields := make([]string, 0, len(fm.inputs)+5)
		focusRows := make([]int, len(fm.inputs)+1)
		for i := range fm.inputs {
			if richLayout {
				if section := formSectionTitle(i); section != "" {
					allFields = append(allFields, sectionStyle.Copy().MarginTop(0).Render(section))
				}
			}
			if i == 5 {
				fm.inputs[i].Placeholder = "password/key/key_passphrase/agent"
			}
			if i == 8 && len(fm.groups) > 0 {
				fm.inputs[i].Placeholder = truncateCells(strings.Join(fm.groups, ", "), 25)
			}
			focusRows[i] = len(allFields)
			allFields = append(allFields, fm.inputs[i].View())
		}
		focusRows[len(fm.inputs)] = len(allFields)
		allFields = append(allFields, fm.password.View())
		focusField := len(allFields) - 1
		if fm.focusIdx <= len(fm.inputs) {
			focusField = focusRows[fm.focusIdx]
		}
		actionRows := 1
		if richLayout {
			actionRows = 2
		}
		fieldRows := max(1, height-2-actionRows)
		start, end := visibleServerRange(len(allFields), focusField, fieldRows)
		visible := append([]string(nil), allFields[start:end]...)
		if start > 0 && len(visible) > 0 {
			visible[0] = "↑ more fields · " + visible[0]
		}
		if end < len(allFields) && len(visible) > 0 {
			visible[len(visible)-1] += " · more ↓"
		}
		if richLayout {
			visible = append(visible, sectionStyle.Copy().MarginTop(0).Render("Actions"))
		}
		visible = append(visible, actions)
		return renderPaddedPanel(width, height, visible)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   title,
		status:       "Server profile",
		notification: status,
		width:        fm.width,
		height:       fm.height,
		body:         body,
		footer: []helpItem{
			{Key: "Tab/↓", Action: "next"},
			{Key: "↑", Action: "prev"},
			{Key: "/", Action: "pick list"},
			{Key: "Enter", Action: "select"},
			{Key: "Ctrl+H", Action: "help"},
			{Key: "Esc", Action: "back"},
		},
	})
}

func (fm *formModel) routeEditorView(title string) string {
	route := fm.currentRoute()
	body := func(width, height int) string {
		lines := []string{dashboardSection("Current route")}
		if len(route.Hops) == 0 {
			line := "  Direct connection"
			if fm.routePane == 0 {
				line = selectedRowStyle.Render("> Direct connection")
			}
			lines = append(lines, line)
		} else {
			for index, hop := range route.Hops {
				kind := "raw"
				name := hop.Raw
				if hop.Profile() {
					kind = "profile"
					name = hop.Alias
				}
				prefix := "  "
				line := fmt.Sprintf("%s%d. %-24s [%s]", prefix, index+1, name, kind)
				if fm.routePane == 0 && index == fm.routeCursor {
					line = selectedRowStyle.Render(fmt.Sprintf("> %d. %-24s [%s]", index+1, name, kind))
				}
				lines = append(lines, line)
			}
		}
		target := strings.TrimSpace(fm.inputs[2].Value())
		if target == "" {
			target = "target"
		}
		lines = append(lines, dashboardHelp("Preview: "+route.DisplaySummary(target)), "", dashboardSection("Available server profiles"))

		if len(fm.routeList.Items()) == 0 {
			lines = append(lines, dashboardHelp("No other server profiles are available."))
		} else {
			capacity := max(1, height-len(lines)-4)
			start, end := visibleServerRange(len(fm.routeList.Items()), fm.routeList.Index(), capacity)
			for index := start; index < end; index++ {
				item, ok := fm.routeList.Items()[index].(routeProfileItem)
				if !ok || item.server == nil {
					continue
				}
				prefix := "  "
				mark := " "
				if fm.routeContainsProfile(route, item.server.ID) {
					mark = "✓"
				}
				target := item.server.Host
				if item.server.User != "" {
					target = item.server.User + "@" + item.server.Host
				}
				line := fmt.Sprintf("%s%s %-20s %s:%d", prefix, mark, item.server.Alias, target, item.server.Port)
				if fm.routePane == 1 && index == fm.routeList.Index() {
					line = selectedRowStyle.Render(fmt.Sprintf("> %s %-20s %s:%d", mark, item.server.Alias, target, item.server.Port))
				}
				lines = append(lines, fitLine(line, max(1, width-4)))
			}
		}
		lines = append(lines, "", dashboardHelp("Need a host that is not a sshkeeper profile? Esc and type raw:<user@host:port> in the Route field."))
		return renderPaddedPanel(width, height, lines)
	}
	pane := "profiles"
	if fm.routePane == 0 {
		pane = "current route"
	}
	return renderScreenShell(screenShell{
		breadcrumb: title + " / Route Editor",
		status:     "Editing " + pane,
		width:      fm.width,
		height:     fm.height,
		body:       body,
		footer: []helpItem{
			{Key: "Tab", Action: "switch pane"},
			{Key: "↑/↓", Action: "move"},
			{Key: "Enter", Action: "add profile"},
			{Key: "x/Del", Action: "remove hop"},
			{Key: "[/]", Action: "reorder hop"},
			{Key: "/", Action: "filter profiles"},
			{Key: "Esc", Action: "done"},
		},
	})
}

func (fm *formModel) formStatusLine() string {
	if fm.err != nil {
		return errorStyle.Render(fmt.Sprintf("✗ Error: %v", fm.err))
	}
	if fm.testing {
		return fm.spinner.View() + " Testing connection..."
	}
	if fm.saving {
		return fm.spinner.View() + " Saving..."
	}
	showResults := time.Since(fm.testResultTime) < 10*time.Second || time.Since(fm.savedTime) < 10*time.Second
	if showResults && fm.testResult != "" {
		if fm.testOK {
			return testOKStyle.Render("✓ " + fm.testResult)
		}
		return testFailStyle.Render("✗ " + strings.ReplaceAll(fm.testResult, "\n", " "))
	}
	if showResults && fm.saved {
		return successStyle.Render("✓ Saved.")
	}
	return ""
}

func renderDropdown(l list.Model) string {
	var b strings.Builder
	b.WriteString(dashboardSection(l.Title))
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
