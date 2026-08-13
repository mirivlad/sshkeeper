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
		"Route hops (comma-separated, or pick from profiles)",
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
	case "Route hops (comma-separated, or pick from profiles)":
		return "bastion, dmz-gw"
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

	// Populate Route hops
	if len(s.Route.Hops) > 0 {
		hopStrs := make([]string, len(s.Route.Hops))
		for i, h := range s.Route.Hops {
			if h.IsProfile {
				hopStrs[i] = h.Alias
			} else {
				hopStrs[i] = h.Raw
			}
		}
		fm.inputs[7].SetValue(strings.Join(hopStrs, ", "))
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
			fm.err = msg.err
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
	s := fm.buildServer()
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

// parseRouteHops parses the route hops input string into a model.Route.
// Format: comma-separated list of aliases or raw addresses.
func parseRouteHops(input string) model.Route {
	input = strings.TrimSpace(input)
	if input == "" {
		return model.Route{}
	}
	parts := strings.Split(input, ",")
	hops := make([]model.RouteHop, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Heuristic: if it contains @ or :, treat as raw address
		if strings.Contains(p, "@") || strings.Contains(p, ":") {
			hops = append(hops, model.RouteHop{Raw: p, IsProfile: false})
		} else {
			// Treat as profile alias
			hops = append(hops, model.RouteHop{Alias: p, IsProfile: true})
		}
	}
	return model.Route{Hops: hops}
}

func (fm *formModel) buildServer() *model.Server {
	port, _ := parsePort(fm.inputs[3].Value())
	authMethod := model.AuthMethod(fm.inputs[5].Value())
	if authMethod == "" {
		authMethod = model.AuthKey
	}

	route := parseRouteHops(fm.inputs[7].Value())

	return &model.Server{
		Alias:          fm.inputs[0].Value(),
		DisplayName:    fm.inputs[1].Value(),
		Host:           fm.inputs[2].Value(),
		Port:           port,
		User:           fm.inputs[4].Value(),
		AuthMethod:     authMethod,
		IdentityFile:   fm.inputs[6].Value(),
		ProxyJump:      route.ProxyJumpString(),
		Route:          route,
		GroupName:      fm.inputs[8].Value(),
		Notes:          fm.inputs[9].Value(),
		StartupCommand: fm.inputs[10].Value(),
		Tags:           splitCSV(fm.inputs[11].Value()),
	}
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
	var b strings.Builder

	title := "Add Server"
	if fm.edit {
		title = "Edit Server: " + fm.server.Alias
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	reserved := 9
	available := fm.height - reserved
	if available < 4 {
		available = 4
	}

	numInputs := len(fm.inputs)
	startIdx := 0
	endIdx := numInputs

	if numInputs > available {
		focusInput := fm.focusIdx
		if focusInput >= numInputs {
			focusInput = numInputs - 1
		}
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
	}
	if fm.err != nil {
		b.WriteString("\n" + errorStyle.Render(fmt.Sprintf("✗ Error: %v", fm.err)) + "\n")
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
