package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Forward type items ---

type forwardTypeItem struct {
	value       model.ForwardType
	label       string
	description string
}

// --- Forward list screen model ---

type forwardScreenModel struct {
	serverID    int64
	serverAlias string
	list        []*model.Forward
	width       int
	height      int
	err         error
	selected    int
}

func newForwardScreenModel(serverID int64, serverAlias string, w, h int) *forwardScreenModel {
	return &forwardScreenModel{
		serverID:    serverID,
		serverAlias: serverAlias,
		width:       w,
		height:      h,
	}
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

func (m *forwardScreenModel) deleteSelected() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.list) {
		return nil
	}
	f := m.list[m.selected]
	return func() tea.Msg {
		return forwardDeleteConfirmMsg{id: f.ID, name: f.Name}
	}
}

func (m *forwardScreenModel) confirmDelete() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.list) {
		return nil
	}
	f := m.list[m.selected]
	return func() tea.Msg {
		return forwardDeletedMsg{id: f.ID, err: DeleteForward(f.ID)}
	}
}

func (m *forwardScreenModel) editSelected() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.list) {
		return nil
	}
	return func() tea.Msg {
		return forwardEditSignal{}
	}
}

func (m *forwardScreenModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Port Forwards — " + m.serverAlias))
	b.WriteString("\n\n")

	if len(m.list) == 0 {
		b.WriteString(helpStyle.Render("  No port forwards configured. Press Ctrl+A to add one."))
		b.WriteString("\n")
	} else {
		// Column header
		b.WriteString(listHeaderStyle.Render(fmt.Sprintf("  %-22s %-8s %-20s %-20s %s",
			"NAME", "TYPE", "LISTEN", "TARGET", "ON")))
		b.WriteString("\n")

		for i, f := range m.list {
			name := f.Name
			if name == "" {
				name = f.ForwardListen()
			}
			enabled := "yes"
			if !f.Enabled {
				enabled = "no"
			}
			line := fmt.Sprintf("  %-22s %-8s %-20s %-20s %s",
				truncate(name, 22),
				f.Type,
				truncate(f.ForwardListen(), 20),
				truncate(f.ForwardTarget(), 20),
				enabled,
			)
			style := normalStyle
			if i == m.selected {
				style = selectedRowStyle
			}
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}

		// Details for selected
		if m.selected >= 0 && m.selected < len(m.list) {
			f := m.list[m.selected]
			b.WriteString("\n")
			b.WriteString(sectionStyle.Render("Selected"))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  %s\n", f.ForwardHumanExplanation(m.serverAlias)))
			for _, arg := range f.ForwardSSHArgs() {
				b.WriteString(fmt.Sprintf("  %s\n", arg))
			}
		}
	}

	b.WriteString("\n")
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
	currentType model.ForwardType
	nameInput   textinput.Model
	descInput   textinput.Model
	typeIdx     int // 0=local, 1=remote, 2=socks
	width       int
	height      int
}

var forwardTypes = []forwardTypeItem{
	{value: model.ForwardLocal, label: "Local", description: "port on my machine → service on SSH server"},
	{value: model.ForwardRemote, label: "Remote", description: "port on SSH server → service on my machine"},
	{value: model.ForwardDynamic, label: "SOCKS", description: "local dynamic SOCKS proxy through SSH"},
}

func newForwardFormModel(serverID int64, w, h int) *forwardFormModel {
	nameInput := textinput.New()
	nameInput.Placeholder = "Local PostgreSQL"
	nameInput.CharLimit = 128

	descInput := textinput.New()
	descInput.Placeholder = "optional"
	descInput.CharLimit = 256

	inputs := make([]textinput.Model, 4)
	placeholders := []string{"127.0.0.1", "15432", "127.0.0.1", "5432"}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholders[i]
		inputs[i].CharLimit = 128
	}

	return &forwardFormModel{
		serverID:    serverID,
		inputs:      inputs,
		focusIdx:    0,
		currentType: model.ForwardLocal,
		typeIdx:     0,
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
	fm.typeIdx = typeIndex(fwd.Type)
	fm.inputs[0].SetValue(fwd.LocalAddr)
	fm.inputs[1].SetValue(strconv.Itoa(fwd.LocalPort))
	fm.inputs[2].SetValue(fwd.RemoteAddr)
	fm.inputs[3].SetValue(strconv.Itoa(fwd.RemotePort))
	return fm
}

func typeIndex(t model.ForwardType) int {
	switch t {
	case model.ForwardLocal:
		return 0
	case model.ForwardRemote:
		return 1
	case model.ForwardDynamic:
		return 2
	}
	return 0
}

func (fm *forwardFormModel) Init() tea.Cmd {
	return nil
}

func (fm *forwardFormModel) visibleFields() []int {
	switch fm.currentType {
	case model.ForwardLocal:
		return []int{0, 1, 2, 3}
	case model.ForwardRemote:
		return []int{0, 1, 2, 3}
	case model.ForwardDynamic:
		return []int{0, 1}
	default:
		return []int{0, 1, 2, 3}
	}
}

func (fm *forwardFormModel) labelForField(idx int) string {
	var labels []string
	switch fm.currentType {
	case model.ForwardLocal:
		labels = []string{"Listen Address", "Listen Port", "Target Host", "Target Port"}
	case model.ForwardRemote:
		labels = []string{"Remote Listen Addr", "Remote Listen Port", "Local Target Host", "Local Target Port"}
	case model.ForwardDynamic:
		labels = []string{"Listen Address", "Listen Port"}
	default:
		return ""
	}
	if idx < 0 || idx >= len(labels) {
		return ""
	}
	return labels[idx]
}

func (fm *forwardFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case saveDoneMsg:
		fm.saved = (msg.err == nil)
		fm.err = msg.err
		return fm, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			fm.focusIdx++
			total := 2 + 3 + len(fm.visibleFields()) + 1 // name + desc + type(3) + fields + save
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyShiftTab:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + 3 + len(fm.visibleFields()) + 1
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyEnter:
			// Check if on type selector
			if fm.focusIdx >= 2 && fm.focusIdx < 2+3 {
				fm.typeIdx = fm.focusIdx - 2
				fm.currentType = forwardTypes[fm.typeIdx].value
				fm.focusIdx++
				fm.updateFocus()
				return fm, nil
			}
			if fm.focusIdx == 2+3+len(fm.visibleFields()) {
				return fm, fm.runSave()
			}
			fm.focusIdx++
			fm.updateFocus()
			return fm, nil
		case tea.KeyEsc:
			return fm, nil
		case tea.KeyDown:
			fm.focusIdx++
			total := 2 + 3 + len(fm.visibleFields()) + 1
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyUp:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + 3 + len(fm.visibleFields()) + 1
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyRunes:
			// Direct number key to select type
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case '1':
					fm.typeIdx = 0
					fm.currentType = model.ForwardLocal
					fm.updateFocus()
					return fm, nil
				case '2':
					fm.typeIdx = 1
					fm.currentType = model.ForwardRemote
					fm.updateFocus()
					return fm, nil
				case '3':
					fm.typeIdx = 2
					fm.currentType = model.ForwardDynamic
					fm.updateFocus()
					return fm, nil
				}
			}
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
	if fm.focusIdx >= 2+3 && fm.focusIdx < 2+3+len(visible) {
		fieldIdx := visible[fm.focusIdx-(2+3)]
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

	total := 2 + 3 + len(fm.visibleFields()) + 1
	switch {
	case fm.focusIdx == 0:
		fm.nameInput.Focus()
		fm.nameInput.Prompt = focusedStyle.Render("Name> ")
	case fm.focusIdx == 1:
		fm.descInput.Focus()
		fm.descInput.Prompt = focusedStyle.Render("Description> ")
	case fm.focusIdx >= 2 && fm.focusIdx < 2+3:
		// Type selector focused — no input to focus
	case fm.focusIdx >= 2+3 && fm.focusIdx < total-1:
		visible := fm.visibleFields()
		fieldIdx := visible[fm.focusIdx-(2+3)]
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

	// Name
	b.WriteString(fm.nameInput.View())
	b.WriteString("\n")

	// Description
	b.WriteString(fm.descInput.View())
	b.WriteString("\n\n")

	// Type selector — visible radio items with descriptions
	b.WriteString(sectionStyle.Render("Type"))
	b.WriteString("\n")
	for i, t := range forwardTypes {
		prefix := "  "
		style := normalStyle
		if i == fm.typeIdx {
			prefix = "▸ "
			style = selectedRowStyle
		}
		line := fmt.Sprintf("%s%d. %-8s  %s", prefix, i+1, t.label, t.description)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	// Show human-readable explanation for selected type
	if fm.typeIdx >= 0 && fm.typeIdx < len(forwardTypes) {
		explanations := map[model.ForwardType]string{
			model.ForwardLocal:   "Opens a local port on this machine and forwards it through SSH to the target address.",
			model.ForwardRemote:  "Opens a port on the remote SSH server and forwards it back to this machine.",
			model.ForwardDynamic: "Creates a local SOCKS proxy that routes all traffic through the SSH server.",
		}
		if exp, ok := explanations[forwardTypes[fm.typeIdx].value]; ok {
			b.WriteString(helpStyle.Render(fmt.Sprintf("  %s\n", exp)))
		}
	}
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
	total := 2 + 3 + len(visible) + 1
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
		{Key: "1/2/3", Action: "select type"},
		{Key: "Enter", Action: "save"},
		{Key: "Esc", Action: "back"},
	}, fm.width))

	return b.String()
}

// forwardEditSignal is sent when user wants to edit a forward
type forwardEditSignal struct{}

// forwardDeleteConfirmMsg asks for confirmation before deleting
type forwardDeleteConfirmMsg struct {
	id   int64
	name string
}
