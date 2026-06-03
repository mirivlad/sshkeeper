package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Forward type selector items ---

type forwardTypeItem struct {
	value       model.ForwardType
	label       string
	description string
}

func (i forwardTypeItem) Title() string       { return i.label }
func (i forwardTypeItem) Description() string { return i.description }
func (i forwardTypeItem) FilterValue() string { return i.label }

// --- Forward list screen model ---

type forwardScreenModel struct {
	serverID    int64
	serverAlias string
	list        list.Model
	forwards    []*model.Forward
	width       int
	height      int
	err         error
}

func newForwardScreenModel(serverID int64, serverAlias string, w, h int) *forwardScreenModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), w, h-6)
	l.Title = "Port Forwards — " + serverAlias
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return &forwardScreenModel{
		serverID:    serverID,
		serverAlias: serverAlias,
		list:        l,
		width:       w,
		height:      h,
	}
}

type forwardListItem struct {
	forward *model.Forward
}

func (i forwardListItem) Title() string {
	name := i.forward.Name
	if name == "" {
		name = i.forward.ForwardListen()
	}
	return fmt.Sprintf("%-20s  %-8s  %-20s  %-20s  %s",
		truncate(name, 20),
		i.forward.Type,
		truncate(i.forward.ForwardListen(), 20),
		truncate(i.forward.ForwardTarget(), 20),
		map[bool]string{true: "yes", false: "no"}[i.forward.Enabled],
	)
}
func (i forwardListItem) Description() string {
	return i.forward.ForwardHumanExplanation("")
}
func (i forwardListItem) FilterValue() string {
	return i.forward.Name + " " + string(i.forward.Type) + " " + i.forward.ForwardListen() + " " + i.forward.ForwardTarget()
}

func (m *forwardScreenModel) SetServerAlias(alias string) {
	m.serverAlias = alias
	m.list.Title = "Port Forwards — " + alias
}

func (m *forwardScreenModel) loadForwards() tea.Cmd {
	return func() tea.Msg {
		if ListForwards == nil {
			return forwardsLoadedMsg{err: fmt.Errorf("forward storage is unavailable")}
		}
		forwards, err := ListForwards(m.serverID)
		return forwardsLoadedMsg{forwards: forwards, err: err}
	}
}

func (m *forwardScreenModel) rebuildList() {
	items := make([]list.Item, len(m.forwards))
	for i, f := range m.forwards {
		items[i] = forwardListItem{forward: f}
	}
	m.list.SetItems(items)
}

func (m *forwardScreenModel) deleteSelected() tea.Cmd {
	if item, ok := m.list.SelectedItem().(forwardListItem); ok && DeleteForward != nil {
		f := item.forward
		return func() tea.Msg {
			return forwardDeletedMsg{id: f.ID, err: DeleteForward(f.ID)}
		}
	}
	return nil
}

func (m *forwardScreenModel) editSelected() tea.Cmd {
	// Return signal to open edit form
	if _, ok := m.list.SelectedItem().(forwardListItem); ok {
		return func() tea.Msg {
			return forwardEditSignal{}
		}
	}
	return nil
}

func (m *forwardScreenModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(renderHelp([]helpItem{
		{Key: "Ctrl+A (a)", Action: "add"},
		{Key: "Ctrl+E/Enter", Action: "edit"},
		{Key: "Ctrl+D (d)", Action: "delete"},
		{Key: "Esc", Action: "back"},
	}, m.width))
	return b.String()
}

// --- Forward form screen model ---

type forwardFormModel struct {
	serverID    int64
	editMode    bool
	editID      int64
	inputs      []textinput.Model
	labels      []string
	focusIdx    int
	err         error
	saved       bool
	typeList    list.Model
	showList    bool
	currentType model.ForwardType
	nameInput   textinput.Model
	descInput   textinput.Model
	width       int
	height      int
}

func newForwardFormModel(serverID int64, w, h int) *forwardFormModel {
	nameInput := textinput.New()
	nameInput.Placeholder = "Local PostgreSQL"
	nameInput.CharLimit = 128

	descInput := textinput.New()
	descInput.Placeholder = "optional"
	descInput.CharLimit = 256

	inputs := make([]textinput.Model, 4)
	labels := []string{"Listen Address", "Listen Port", "Target Host", "Target Port"}
	placeholders := []string{"127.0.0.1", "15432", "127.0.0.1", "5432"}
	for i := range labels {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholders[i]
		inputs[i].CharLimit = 128
	}

	typeItems := []list.Item{
		forwardTypeItem{value: model.ForwardLocal, label: "Local", description: "port on my machine → service reachable from SSH server"},
		forwardTypeItem{value: model.ForwardRemote, label: "Remote", description: "port on SSH server → service on my machine"},
		forwardTypeItem{value: model.ForwardDynamic, label: "SOCKS", description: "local dynamic SOCKS proxy through SSH"},
	}

	typeList := list.New(typeItems, list.NewDefaultDelegate(), 50, 6)
	typeList.Title = "Select forward type"
	typeList.SetShowStatusBar(false)
	typeList.SetShowHelp(false)
	typeList.SetFilteringEnabled(false)
	typeList.Styles.Title = titleStyle

	return &forwardFormModel{
		serverID:    serverID,
		inputs:      inputs,
		labels:      labels,
		focusIdx:    0,
		typeList:    typeList,
		currentType: model.ForwardLocal,
		nameInput:   nameInput,
		descInput:   descInput,
		width:       w,
		height:      h,
	}
}

func newForwardEditModel(serverID int64, fwd *model.Forward, w, h int) *forwardFormModel {
	fm := newForwardFormModel(serverID, w, h)
	fm.editMode = true
	fm.editID = fwd.ID
	fm.nameInput.SetValue(fwd.Name)
	fm.descInput.SetValue(fwd.Description)
	fm.currentType = fwd.Type

	switch fwd.Type {
	case model.ForwardLocal:
		fm.typeList.Select(0)
	case model.ForwardRemote:
		fm.typeList.Select(1)
	case model.ForwardDynamic:
		fm.typeList.Select(2)
	}

	fm.inputs[0].SetValue(fwd.LocalAddr)
	fm.inputs[1].SetValue(strconv.Itoa(fwd.LocalPort))
	fm.inputs[2].SetValue(fwd.RemoteAddr)
	fm.inputs[3].SetValue(strconv.Itoa(fwd.RemotePort))

	return fm
}

func (fm *forwardFormModel) Init() tea.Cmd {
	return nil
}

func (fm *forwardFormModel) visibleFields() []int {
	switch fm.currentType {
	case model.ForwardLocal:
		return []int{0, 1, 2, 3} // listen addr/port, target host/port
	case model.ForwardRemote:
		return []int{0, 1, 2, 3} // remote listen addr/port, local target host/port
	case model.ForwardDynamic:
		return []int{0, 1} // listen addr/port only
	default:
		return []int{0, 1, 2, 3}
	}
}

func (fm *forwardFormModel) labelForField(idx int) string {
	switch fm.currentType {
	case model.ForwardLocal:
		return fm.labels[idx]
	case model.ForwardRemote:
		labels := []string{"Remote Listen Addr", "Remote Listen Port", "Local Target Host", "Local Target Port"}
		return labels[idx]
	case model.ForwardDynamic:
		return fm.labels[idx]
	default:
		return fm.labels[idx]
	}
}

func (fm *forwardFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case saveDoneMsg:
		fm.saved = (msg.err == nil)
		fm.err = msg.err
		return fm, nil
	}

	if fm.showList {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				fm.showList = false
				return fm, nil
			case tea.KeyEnter:
				if item, ok := fm.typeList.SelectedItem().(forwardTypeItem); ok {
					fm.currentType = item.value
				}
				fm.showList = false
				return fm, nil
			}
		}
		var cmd tea.Cmd
		fm.typeList, cmd = fm.typeList.Update(msg)
		return fm, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			fm.focusIdx++
			total := 2 + len(fm.visibleFields()) + 1 // name + desc + fields + save btn
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyShiftTab:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + len(fm.visibleFields()) + 1
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' && !msg.Alt && fm.focusIdx == 0 {
				fm.showList = true
				return fm, nil
			}
		case tea.KeyEnter:
			if fm.focusIdx == 2+len(fm.visibleFields()) {
				return fm, fm.runSave()
			}
			fm.focusIdx++
			fm.updateFocus()
			return fm, nil
		case tea.KeyEsc:
			return fm, nil
		case tea.KeyDown:
			fm.focusIdx++
			total := 2 + len(fm.visibleFields()) + 1
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyUp:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + len(fm.visibleFields()) + 1
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		}
	}

	// Route to focused input
	if fm.focusIdx == 0 {
		var cmd tea.Cmd
		fm.nameInput, cmd = fm.nameInput.Update(msg)
		return fm, cmd
	}
	if fm.focusIdx == 1 {
		var cmd tea.Cmd
		fm.descInput, cmd = fm.descInput.Update(msg)
		return fm, cmd
	}
	visible := fm.visibleFields()
	if fm.focusIdx >= 2 && fm.focusIdx < 2+len(visible) {
		fieldIdx := visible[fm.focusIdx-2]
		var cmd tea.Cmd
		fm.inputs[fieldIdx], cmd = fm.inputs[fieldIdx].Update(msg)
		return fm, cmd
	}

	return fm, nil
}

func (fm *forwardFormModel) updateFocus() {
	fm.nameInput.Blur()
	fm.nameInput.Prompt = blurredStyle.Render("Name: ")
	fm.descInput.Blur()
	fm.descInput.Prompt = blurredStyle.Render("Description: ")
	for i := range fm.inputs {
		fm.inputs[i].Blur()
		fm.inputs[i].Prompt = blurredStyle.Render(fm.labelForField(i) + ": ")
	}

	total := 2 + len(fm.visibleFields()) + 1
	switch {
	case fm.focusIdx == 0:
		fm.nameInput.Focus()
		fm.nameInput.Prompt = focusedStyle.Render("Name> ")
	case fm.focusIdx == 1:
		fm.descInput.Focus()
		fm.descInput.Prompt = focusedStyle.Render("Description> ")
	case fm.focusIdx >= 2 && fm.focusIdx < total-1:
		visible := fm.visibleFields()
		fieldIdx := visible[fm.focusIdx-2]
		fm.inputs[fieldIdx].Focus()
		fm.inputs[fieldIdx].Prompt = focusedStyle.Render(fm.labelForField(fieldIdx) + "> ")
	}
}

func (fm *forwardFormModel) runSave() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(fm.nameInput.Value())
		desc := strings.TrimSpace(fm.descInput.Value())

		localPort := 0
		fmt.Sscanf(fm.inputs[1].Value(), "%d", &localPort)
		remotePort := 0
		fmt.Sscanf(fm.inputs[3].Value(), "%d", &remotePort)

		localAddr := strings.TrimSpace(fm.inputs[0].Value())
		remoteAddr := strings.TrimSpace(fm.inputs[2].Value())

		if name == "" {
			return saveDoneMsg{err: fmt.Errorf("name is required")}
		}
		if localPort < 1 || localPort > 65535 {
			return saveDoneMsg{err: fmt.Errorf("invalid listen port %d: must be 1-65535", localPort)}
		}

		switch fm.currentType {
		case model.ForwardLocal:
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
			if remoteAddr == "" {
				return saveDoneMsg{err: fmt.Errorf("target host is required for local forward")}
			}
			if remotePort < 1 || remotePort > 65535 {
				return saveDoneMsg{err: fmt.Errorf("invalid target port %d: must be 1-65535", remotePort)}
			}
		case model.ForwardRemote:
			if remoteAddr == "" {
				return saveDoneMsg{err: fmt.Errorf("remote listen address is required")}
			}
			if remotePort < 1 || remotePort > 65535 {
				return saveDoneMsg{err: fmt.Errorf("invalid remote port %d: must be 1-65535", remotePort)}
			}
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
		case model.ForwardDynamic:
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
			remoteAddr = ""
			remotePort = 0
		}

		fwd := &model.Forward{
			ServerID:    fm.serverID,
			Name:        name,
			Description: desc,
			Type:        fm.currentType,
			LocalAddr:   localAddr,
			LocalPort:   localPort,
			RemoteAddr:  remoteAddr,
			RemotePort:  remotePort,
			Enabled:     true,
		}

		if fm.editMode {
			fwd.ID = fm.editID
			if UpdateForward == nil {
				return saveDoneMsg{err: fmt.Errorf("update not available")}
			}
			return saveDoneMsg{err: UpdateForward(fwd)}
		}

		if SaveForward == nil {
			return saveDoneMsg{err: fmt.Errorf("forward storage is unavailable")}
		}
		err := SaveForward(fwd)
		return saveDoneMsg{err: err}
	}
}

func (fm *forwardFormModel) View() string {
	var b strings.Builder
	title := "Add Port Forward"
	if fm.editMode {
		title = "Edit Port Forward"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Type selector
	typeLabel := fmt.Sprintf("Type: %s (/ to change)", fm.currentType)
	if fm.focusIdx == 0 {
		typeLabel = focusedStyle.Render("Type> " + fmt.Sprintf("%s (/ to change)", fm.currentType))
	}
	b.WriteString(typeLabel)
	b.WriteString("\n")

	if fm.showList {
		b.WriteString("\n" + fm.typeList.View() + "\n")
		b.WriteString(renderHelp([]helpItem{{Key: "Enter", Action: "select"}, {Key: "Esc", Action: "cancel"}}, fm.width))
		return b.String()
	}

	// Name
	b.WriteString(fm.nameInput.View())
	b.WriteString("\n")

	// Description
	b.WriteString(fm.descInput.View())
	b.WriteString("\n")

	// Dynamic fields based on type
	visible := fm.visibleFields()
	for _, idx := range visible {
		b.WriteString(fm.inputs[idx].View())
		b.WriteString("\n")
	}

	// Warning for 0.0.0.0
	if localAddr := strings.TrimSpace(fm.inputs[0].Value()); localAddr == "0.0.0.0" {
		b.WriteString(helpStyle.Render("  ⚠ This port will be accessible from the network.\n"))
	}

	// Preview
	if fm.currentType != "" && fm.inputs[1].Value() != "" {
		b.WriteString("\n" + sectionStyle.Render("Preview") + "\n")
		fwd := &model.Forward{
			Type:       fm.currentType,
			LocalAddr:  fm.inputs[0].Value(),
			LocalPort:  0,
			RemoteAddr: fm.inputs[2].Value(),
			RemotePort: 0,
		}
		fmt.Sscanf(fm.inputs[1].Value(), "%d", &fwd.LocalPort)
		fmt.Sscanf(fm.inputs[3].Value(), "%d", &fwd.RemotePort)
		for _, arg := range fwd.ForwardSSHArgs() {
			b.WriteString("  " + arg + "\n")
		}
		b.WriteString("  -o ExitOnForwardFailure=yes\n")
	}

	// Save button
	total := 2 + len(visible) + 1
	button := "\n[ Save ]"
	if fm.focusIdx == total-1 {
		button = selectedStyle.Render(button)
	}
	b.WriteString(button)
	b.WriteString("\n\n")

	if fm.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ Error: %v", fm.err)) + "\n\n")
	}
	if fm.saved {
		b.WriteString(successStyle.Render("✓ Saved.") + "\n\n")
	}

	b.WriteString(renderHelp([]helpItem{
		{Key: "Tab/↓", Action: "next"},
		{Key: "↑", Action: "prev"},
		{Key: "/", Action: "change type"},
		{Key: "Enter", Action: "save"},
		{Key: "Esc", Action: "back"},
	}, fm.width))

	return b.String()
}

// forwardEditSignal is sent when user wants to edit a forward
type forwardEditSignal struct{}
