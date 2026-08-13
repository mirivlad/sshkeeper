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
	footer := renderHelp([]helpItem{
		{Key: "Ctrl+A (a)", Action: "add"},
		{Key: "Ctrl+E/Enter", Action: "edit"},
		{Key: "Ctrl+D (d)", Action: "delete"},
		{Key: "Esc", Action: "back"},
	}, m.width)
	lines := []string{titleStyle.Copy().MarginLeft(0).Render(fitLine("Port Forwards — "+m.serverAlias, m.width))}
	if m.err != nil {
		lines = append(lines, fitLine(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)), m.width))
	}

	footerRows := displayLineCount(footer)
	detailRows := 0
	if len(m.list) > 0 && m.height-footerRows >= 7 {
		detailRows = 3
	}
	rowCapacity := max(1, m.height-len(lines)-footerRows-detailRows-1)
	if len(m.list) == 0 {
		lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(fitLine("No port forwards configured. Ctrl+A adds one.", m.width)))
	} else {
		lines = append(lines, m.renderForwardRow(nil, false))
		rowCapacity--
		start, end := visibleServerRange(len(m.list), m.selected, rowCapacity)
		for index := start; index < end; index++ {
			lines = append(lines, m.renderForwardRow(m.list[index], index == m.selected))
		}
		if end < len(m.list) || start > 0 {
			lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.list))))
		}
		if detailRows > 0 && m.selected >= 0 && m.selected < len(m.list) {
			forward := m.list[m.selected]
			lines = append(lines,
				sectionStyle.Copy().MarginTop(0).Render("Selected"),
				fitLine(forward.ForwardHumanExplanation(m.serverAlias), m.width),
				fitLine("ssh "+strings.Join(forward.ForwardSSHArgs(), " "), m.width),
			)
		}
	}
	lines = append(lines, strings.Split(footer, "\n")...)
	if len(lines) > m.height && m.height > 0 {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m *forwardScreenModel) renderForwardRow(forward *model.Forward, selected bool) string {
	marker, name, kind, listen, target, enabled := " ", "NAME", "TYPE", "LISTEN", "TARGET", "ON"
	if forward != nil {
		if selected {
			marker = ">"
		}
		name = forward.Name
		if name == "" {
			name = forward.ForwardListen()
		}
		kind = string(forward.Type)
		listen = forward.ForwardListen()
		target = forward.ForwardTarget()
		enabled = "yes"
		if !forward.Enabled {
			enabled = "no"
		}
	}
	wide := m.width >= 70
	typeWidth, enabledWidth := 8, 3
	if wide {
		nameWidth := max(12, (m.width-typeWidth-enabledWidth-6)*30/100)
		listenWidth := max(14, (m.width-typeWidth-enabledWidth-nameWidth-6)/2)
		targetWidth := m.width - nameWidth - typeWidth - listenWidth - enabledWidth - 5
		line := marker + " " + padCells(name, nameWidth) + " " + padCells(kind, typeWidth) + " " + padCells(listen, listenWidth) + " " + padCells(target, targetWidth) + " " + padCells(enabled, enabledWidth)
		if forward == nil {
			return listHeaderStyle.Render(fitLine(line, m.width))
		}
		if selected {
			return selectedRowStyle.Render(fitLine(line, m.width))
		}
		return fitLine(line, m.width)
	}
	nameWidth := max(12, m.width-typeWidth-enabledWidth-4)
	line := marker + " " + padCells(name, nameWidth) + " " + padCells(kind, typeWidth) + " " + padCells(enabled, enabledWidth)
	if forward == nil {
		return listHeaderStyle.Render(fitLine(line, m.width))
	}
	if selected {
		return selectedRowStyle.Render(fitLine(line, m.width))
	}
	return fitLine(line, m.width)
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
	initial     forwardFormSnapshot
}

type forwardFormSnapshot struct {
	name        string
	description string
	values      []string
	forwardType model.ForwardType
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

	fm := &forwardFormModel{
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
	fm.updateFocus()
	fm.initial = fm.snapshot()
	return fm
}

func newForwardEditModel(serverID int64, fwd *model.Forward, w, h int) *forwardFormModel {
	fm := newForwardFormModel(serverID, w, h)
	fm.editMode = true
	fm.editID = fwd.ID
	fm.nameInput.SetValue(fwd.Name)
	fm.descInput.SetValue(fwd.Description)
	fm.currentType = fwd.Type
	fm.typeIdx = typeIndex(fwd.Type)
	if fwd.Type == model.ForwardRemote {
		fm.inputs[0].SetValue(fwd.RemoteAddr)
		fm.inputs[1].SetValue(strconv.Itoa(fwd.RemotePort))
		fm.inputs[2].SetValue(fwd.LocalAddr)
		fm.inputs[3].SetValue(strconv.Itoa(fwd.LocalPort))
	} else {
		fm.inputs[0].SetValue(fwd.LocalAddr)
		fm.inputs[1].SetValue(strconv.Itoa(fwd.LocalPort))
		fm.inputs[2].SetValue(fwd.RemoteAddr)
		fm.inputs[3].SetValue(strconv.Itoa(fwd.RemotePort))
	}
	fm.updateFocus()
	fm.initial = fm.snapshot()
	return fm
}

func (fm *forwardFormModel) snapshot() forwardFormSnapshot {
	values := make([]string, len(fm.inputs))
	for i := range fm.inputs {
		values[i] = fm.inputs[i].Value()
	}
	return forwardFormSnapshot{
		name:        fm.nameInput.Value(),
		description: fm.descInput.Value(),
		values:      values,
		forwardType: fm.currentType,
	}
}

func (fm *forwardFormModel) Dirty() bool {
	current := fm.snapshot()
	if current.name != fm.initial.name || current.description != fm.initial.description || current.forwardType != fm.initial.forwardType || len(current.values) != len(fm.initial.values) {
		return true
	}
	for i := range current.values {
		if current.values[i] != fm.initial.values[i] {
			return true
		}
	}
	return false
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
	return labels[idx] + " *"
}

func (fm *forwardFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case saveDoneMsg:
		fm.saved = (msg.err == nil)
		fm.applySaveError(msg.err)
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
			// Direct number keys select a type only while the type selector has focus.
			if fm.focusIdx >= 2 && fm.focusIdx < 2+len(forwardTypes) && len(msg.Runes) == 1 {
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
	fm.nameInput.Prompt = blurredStyle.Render("Name *: ")
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
		fm.nameInput.Prompt = focusedStyle.Render("Name *> ")
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
		localAddr, remoteAddr := "", ""
		localPort, remotePort := 0, 0
		var err error

		if name == "" {
			return saveDoneMsg{err: fmt.Errorf("name is required")}
		}
		switch fm.currentType {
		case model.ForwardLocal:
			localAddr = strings.TrimSpace(fm.inputs[0].Value())
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
			localPort, err = parseNamedPort("Listen port", fm.inputs[1].Value())
			if err != nil {
				return saveDoneMsg{err: err}
			}
			remoteAddr = strings.TrimSpace(fm.inputs[2].Value())
			if remoteAddr == "" {
				return saveDoneMsg{err: fmt.Errorf("target host is required for local forward")}
			}
			remotePort, err = parseNamedPort("Target port", fm.inputs[3].Value())
			if err != nil {
				return saveDoneMsg{err: err}
			}
		case model.ForwardRemote:
			remoteAddr = strings.TrimSpace(fm.inputs[0].Value())
			if remoteAddr == "" {
				return saveDoneMsg{err: fmt.Errorf("remote listen address is required")}
			}
			remotePort, err = parseNamedPort("Remote listen port", fm.inputs[1].Value())
			if err != nil {
				return saveDoneMsg{err: err}
			}
			localAddr = strings.TrimSpace(fm.inputs[2].Value())
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
			localPort, err = parseNamedPort("Local target port", fm.inputs[3].Value())
			if err != nil {
				return saveDoneMsg{err: err}
			}
		case model.ForwardDynamic:
			localAddr = strings.TrimSpace(fm.inputs[0].Value())
			if localAddr == "" {
				localAddr = "127.0.0.1"
			}
			localPort, err = parseNamedPort("Listen port", fm.inputs[1].Value())
			if err != nil {
				return saveDoneMsg{err: err}
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
		err = SaveForward(fwd)
		return saveDoneMsg{err: err}
	}
}

func (fm *forwardFormModel) applySaveError(err error) {
	fm.err = err
	if err == nil {
		return
	}
	message := strings.ToLower(err.Error())
	fieldIndex := -1
	switch {
	case strings.Contains(message, "name is required"):
		fm.focusIdx = 0
		fm.updateFocus()
		return
	case strings.Contains(message, "listen address"):
		fieldIndex = 0
	case strings.Contains(message, "listen port"):
		fieldIndex = 1
	case strings.Contains(message, "target host"):
		fieldIndex = 2
	case strings.Contains(message, "target port"):
		fieldIndex = 3
	}
	if fieldIndex >= 0 {
		fm.focusIdx = 2 + len(forwardTypes) + fieldIndex
		fm.updateFocus()
	}
}

func parseNamedPort(label, value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be a number from 1 to 65535", label)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", label)
	}
	return port, nil
}

func (fm *forwardFormModel) View() string {
	title := "Add Port Forward"
	if fm.editMode {
		title = "Edit Port Forward"
	}
	lines := []string{titleStyle.Copy().MarginLeft(0).Render(fitLine(title, fm.width))}
	lines = append(lines,
		fitLine(fm.nameInput.View(), fm.width),
		fitLine(fm.descInput.View(), fm.width),
	)

	typeParts := make([]string, len(forwardTypes))
	for i, forwardType := range forwardTypes {
		selected := "○"
		if i == fm.typeIdx {
			selected = "●"
		}
		focus := " "
		if fm.focusIdx == 2+i {
			focus = ">"
		}
		typeParts[i] = fmt.Sprintf("%s%s %d %s", focus, selected, i+1, forwardType.label)
	}
	lines = append(lines, fitLine("Type  "+strings.Join(typeParts, "   "), fm.width))
	if fm.width >= 100 {
		lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(fitLine(forwardTypes[fm.typeIdx].description, fm.width)))
	}

	visible := fm.visibleFields()
	for _, idx := range visible {
		lines = append(lines, fitLine(fm.inputs[idx].View(), fm.width))
	}

	if localAddr := strings.TrimSpace(fm.inputs[0].Value()); localAddr == "0.0.0.0" {
		lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(fitLine("⚠ This port will be accessible from the network.", fm.width)))
	}

	if fm.width >= 70 && fm.currentType != "" && fm.inputs[1].Value() != "" {
		fwd := &model.Forward{
			Type:       fm.currentType,
			LocalAddr:  fm.inputs[0].Value(),
			LocalPort:  0,
			RemoteAddr: fm.inputs[2].Value(),
			RemotePort: 0,
		}
		fmt.Sscanf(fm.inputs[1].Value(), "%d", &fwd.LocalPort)
		fmt.Sscanf(fm.inputs[3].Value(), "%d", &fwd.RemotePort)
		preview := strings.Join(fwd.ForwardSSHArgs(), " ") + " -o ExitOnForwardFailure=yes"
		lines = append(lines, fitLine("Preview  ssh "+preview, fm.width))
	}

	total := 2 + 3 + len(visible) + 1
	button := "  [ Save ]"
	if fm.focusIdx == total-1 {
		button = selectedStyle.Render("> [ Save ]")
	}
	if fm.err != nil {
		lines = append(lines, fitLine(errorStyle.Render(fmt.Sprintf("✗ Error: %v", fm.err)), fm.width))
	}
	if fm.saved {
		lines = append(lines, successStyle.Render("✓ Saved."))
	}
	lines = append(lines, button)
	footer := renderHelp([]helpItem{
		{Key: "Tab/↓", Action: "next"},
		{Key: "↑", Action: "prev"},
		{Key: "1/2/3", Action: "select type"},
		{Key: "Enter", Action: "save"},
		{Key: "Esc", Action: "back"},
	}, fm.width)
	lines = append(lines, strings.Split(footer, "\n")...)
	if len(lines) > fm.height && fm.height > 0 {
		lines = lines[:fm.height]
	}
	return strings.Join(lines, "\n")
}

// forwardEditSignal is sent when user wants to edit a forward
type forwardEditSignal struct{}

// forwardDeleteConfirmMsg asks for confirmation before deleting
type forwardDeleteConfirmMsg struct {
	id   int64
	name string
}
