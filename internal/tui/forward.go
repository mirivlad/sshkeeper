package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
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
	notice      string
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
			return forwardsLoadedMsg{err: fmt.Errorf("%s", i18n.T("forward storage is unavailable", "Хранилище пробросов портов недоступно"))}
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
	notification := ""
	if m.err != nil {
		notification = errorStyle.Render(i18n.Tf("Error: %v", "Ошибка: %v", m.err))
	} else if m.notice != "" {
		notification = successStyle.Render(m.notice)
	}
	body := func(width, height int) string {
		switch classifyShellContent(width) {
		case sizeWide:
			leftWidth := width * 70 / 100
			rightWidth := width - leftWidth - 1
			return joinPanelColumns(
				renderPaddedPanel(leftWidth, height, m.forwardListLines(leftWidth-4, height-2, false)), leftWidth,
				renderPaddedPanel(rightWidth, height, m.forwardDetailLines(rightWidth-4, false)), rightWidth,
			)
		case sizeMedium:
			detailHeight := min(7, max(4, height/3))
			listHeight := max(3, height-detailHeight-1)
			listPanel := renderPaddedPanel(width, listHeight, m.forwardListLines(width-4, listHeight-2, false))
			detailPanel := renderPaddedPanel(width, detailHeight, m.forwardDetailLines(width-4, true))
			return listPanel + "\n" + detailPanel
		default:
			return renderPaddedPanel(width, height, m.forwardListLines(width-4, height-2, true))
		}
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Port Forwards / ", "Пробросы портов / ") + m.serverAlias,
		status:       i18n.Tf("%d rules", "Правил: %d", len(m.list)),
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "Ctrl+B (b)", Action: i18n.T("start in background", "запустить в фоне")},
			{Key: "Ctrl+X", Action: i18n.T("start modes", "режимы запуска")},
			{Key: "Ctrl+A (a)", Action: i18n.T("add", "добавить")},
			{Key: "Ctrl+E/Enter", Action: i18n.T("edit", "изменить")},
			{Key: "Space", Action: i18n.T("enable/disable", "вкл./выкл.")},
			{Key: "Ctrl+D (d)", Action: i18n.T("delete", "удалить")},
			{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}

func (m *forwardScreenModel) forwardListLines(width, capacity int, compact bool) []string {
	if len(m.list) == 0 {
		return []string{helpStyle.Copy().MarginLeft(0).Render(i18n.T("No port forwards configured. Ctrl+A adds one before starting a tunnel.", "Пробросы портов не настроены. Нажмите Ctrl+A, чтобы добавить правило перед запуском туннеля."))}
	}
	lines := []string{m.renderForwardRow(nil, false, width, compact)}
	rowCapacity := max(1, capacity-1)
	showRange := len(m.list) > rowCapacity
	if showRange {
		rowCapacity = max(1, rowCapacity-1)
	}
	start, end := visibleServerRange(len(m.list), m.selected, rowCapacity)
	for index := start; index < end; index++ {
		lines = append(lines, m.renderForwardRow(m.list[index], index == m.selected, width, compact))
	}
	if showRange {
		lines = append(lines, dashboardHelp(i18n.Tf("Showing %d-%d of %d", "Показано %d–%d из %d", start+1, end, len(m.list))))
	}
	return lines
}

func (m *forwardScreenModel) forwardDetailLines(width int, compact bool) []string {
	lines := []string{dashboardSection(i18n.T("Selected rule", "Выбранное правило"))}
	if m.selected < 0 || m.selected >= len(m.list) {
		return append(lines, "", dashboardHelp(i18n.T("No rule selected.", "Правило не выбрано.")))
	}
	forward := m.list[m.selected]
	name := forward.Name
	if name == "" {
		name = forward.ForwardListen()
	}
	if compact {
		lines = append(lines, name+" · "+string(forward.Type))
	} else {
		lines = append(lines, "", name, string(forward.Type))
	}
	lines = append(lines, wrapCells(forward.ForwardHumanExplanation(m.serverAlias), max(1, width))...)
	if !compact {
		lines = append(lines, "")
	}
	lines = append(lines, dashboardHelp("ssh "+strings.Join(forward.ForwardSSHArgs(), " ")))
	return lines
}

func (m *forwardScreenModel) renderForwardRow(forward *model.Forward, selected bool, width int, compact bool) string {
	marker, name, kind, listen, target, enabled := " ", i18n.T("NAME", "ИМЯ"), i18n.T("TYPE", "ТИП"), i18n.T("LISTEN", "СЛУШАЕТ"), i18n.T("TARGET", "ЦЕЛЬ"), i18n.T("ON", "ВКЛ")
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
		enabled = i18n.T("yes", "да")
		if !forward.Enabled {
			enabled = i18n.T("no", "нет")
		}
	}
	typeWidth, enabledWidth := 8, 3
	if !compact && width >= 58 {
		flexible := max(3, width-typeWidth-enabledWidth-7)
		nameWidth := max(1, flexible*30/100)
		listenWidth := max(1, flexible*32/100)
		targetWidth := max(1, flexible-nameWidth-listenWidth)
		line := padCells(marker, 2) + " " + padCells(name, nameWidth) + " " + padCells(kind, typeWidth) + " " + padCells(listen, listenWidth) + " " + padCells(target, targetWidth) + " " + padCells(enabled, enabledWidth)
		if forward == nil {
			return listHeaderStyle.Render(fitLine(line, width))
		}
		if selected {
			return selectedRowStyle.Render(fitLine(line, width))
		}
		return fitLine(line, width)
	}
	nameWidth := max(1, width-typeWidth-enabledWidth-5)
	line := padCells(marker, 2) + " " + padCells(name, nameWidth) + " " + padCells(kind, typeWidth) + " " + padCells(enabled, enabledWidth)
	if forward == nil {
		return listHeaderStyle.Render(fitLine(line, width))
	}
	if selected {
		return selectedRowStyle.Render(fitLine(line, width))
	}
	return fitLine(line, width)
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
	enabled     bool
	width       int
	height      int
	initial     forwardFormSnapshot
}

type forwardFormSnapshot struct {
	name        string
	description string
	values      []string
	forwardType model.ForwardType
	enabled     bool
}

var forwardTypes = []forwardTypeItem{
	{value: model.ForwardLocal, label: "Local", description: "port on my machine → service on SSH server"},
	{value: model.ForwardRemote, label: "Remote", description: "port on SSH server → service on my machine"},
	{value: model.ForwardDynamic, label: "SOCKS", description: "local dynamic SOCKS proxy through SSH"},
}

func newForwardFormModel(serverID int64, w, h int) *forwardFormModel {
	nameInput := textinput.New()
	nameInput.Placeholder = i18n.T("Local PostgreSQL", "Локальный PostgreSQL")
	nameInput.CharLimit = 128

	descInput := textinput.New()
	descInput.Placeholder = i18n.T("optional", "необязательно")
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
		enabled:     true,
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
	fm.enabled = fwd.Enabled
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
		enabled:     fm.enabled,
	}
}

func (fm *forwardFormModel) Dirty() bool {
	current := fm.snapshot()
	if current.name != fm.initial.name || current.description != fm.initial.description || current.forwardType != fm.initial.forwardType || current.enabled != fm.initial.enabled || len(current.values) != len(fm.initial.values) {
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
		labels = []string{i18n.T("Listen Address", "Адрес прослушивания"), i18n.T("Listen Port", "Порт прослушивания"), i18n.T("Target Host", "Целевой хост"), i18n.T("Target Port", "Целевой порт")}
	case model.ForwardRemote:
		labels = []string{i18n.T("Remote Listen Addr", "Адрес на сервере"), i18n.T("Remote Listen Port", "Порт на сервере"), i18n.T("Local Target Host", "Локальный целевой хост"), i18n.T("Local Target Port", "Локальный целевой порт")}
	case model.ForwardDynamic:
		labels = []string{i18n.T("Listen Address", "Адрес прослушивания"), i18n.T("Listen Port", "Порт прослушивания")}
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
			total := 2 + 3 + 1 + len(fm.visibleFields()) + 1 // name + desc + type(3) + fields + save
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyShiftTab:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + 3 + 1 + len(fm.visibleFields()) + 1
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
			if fm.focusIdx == 2+3 {
				fm.enabled = !fm.enabled
				return fm, nil
			}
			if fm.focusIdx == 2+3+1+len(fm.visibleFields()) {
				return fm, fm.runSave()
			}
			fm.focusIdx++
			fm.updateFocus()
			return fm, nil
		case tea.KeyEsc:
			return fm, nil
		case tea.KeyDown:
			fm.focusIdx++
			total := 2 + 3 + 1 + len(fm.visibleFields()) + 1
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyUp:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := 2 + 3 + 1 + len(fm.visibleFields()) + 1
				fm.focusIdx = total - 1
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyRunes:
			if fm.focusIdx == 2+3 && msg.String() == " " {
				fm.enabled = !fm.enabled
				return fm, nil
			}
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
	if fm.focusIdx >= 2+3+1 && fm.focusIdx < 2+3+1+len(visible) {
		fieldIdx := visible[fm.focusIdx-(2+3+1)]
		var cmd tea.Cmd
		fm.inputs[fieldIdx], cmd = fm.inputs[fieldIdx].Update(msg)
		return fm, cmd
	}

	return fm, nil
}

func (fm *forwardFormModel) updateFocus() {
	fm.nameInput.Blur()
	fm.nameInput.Prompt = blurredStyle.Render(i18n.T("Name *: ", "Имя *: "))
	fm.descInput.Blur()
	fm.descInput.Prompt = blurredStyle.Render(i18n.T("Description: ", "Описание: "))
	for i := range fm.inputs {
		fm.inputs[i].Blur()
		fm.inputs[i].Prompt = blurredStyle.Render(fm.labelForField(i) + ": ")
	}

	total := 2 + 3 + 1 + len(fm.visibleFields()) + 1
	switch {
	case fm.focusIdx == 0:
		fm.nameInput.Focus()
		fm.nameInput.Prompt = focusedStyle.Render(i18n.T("Name *> ", "Имя *> "))
	case fm.focusIdx == 1:
		fm.descInput.Focus()
		fm.descInput.Prompt = focusedStyle.Render(i18n.T("Description> ", "Описание> "))
	case fm.focusIdx >= 2 && fm.focusIdx < 2+3:
		// Type selector focused — no input to focus
	case fm.focusIdx == 2+3:
		// Enabled toggle focused.
	case fm.focusIdx >= 2+3+1 && fm.focusIdx < total-1:
		visible := fm.visibleFields()
		fieldIdx := visible[fm.focusIdx-(2+3+1)]
		fm.inputs[fieldIdx].Focus()
		fm.inputs[fieldIdx].Prompt = focusedStyle.Render(fm.labelForField(fieldIdx) + "> ")
	}
}

func (fm *forwardFormModel) buildForwardFromForm() (*model.Forward, error) {
	name := strings.TrimSpace(fm.nameInput.Value())
	if name == "" {
		return nil, fmt.Errorf("%s", i18n.T("name is required", "укажите имя"))
	}
	forward := &model.Forward{
		ID:          fm.editID,
		ServerID:    fm.serverID,
		Name:        name,
		Description: strings.TrimSpace(fm.descInput.Value()),
		Type:        fm.currentType,
		Enabled:     fm.enabled,
	}
	var err error
	switch fm.currentType {
	case model.ForwardLocal:
		forward.LocalAddr = strings.TrimSpace(fm.inputs[0].Value())
		if forward.LocalAddr == "" {
			forward.LocalAddr = "127.0.0.1"
		}
		forward.LocalPort, err = parseNamedPort(i18n.T("Listen port", "порт прослушивания"), fm.inputs[1].Value())
		if err != nil {
			return nil, err
		}
		forward.RemoteAddr = strings.TrimSpace(fm.inputs[2].Value())
		if forward.RemoteAddr == "" {
			return nil, fmt.Errorf("%s", i18n.T("target host is required for local forward", "укажите целевой хост для локального проброса"))
		}
		forward.RemotePort, err = parseNamedPort(i18n.T("Target port", "целевой порт"), fm.inputs[3].Value())
		if err != nil {
			return nil, err
		}
	case model.ForwardRemote:
		forward.RemoteAddr = strings.TrimSpace(fm.inputs[0].Value())
		if forward.RemoteAddr == "" {
			return nil, fmt.Errorf("%s", i18n.T("remote listen address is required", "укажите адрес прослушивания на сервере"))
		}
		forward.RemotePort, err = parseNamedPort(i18n.T("Remote listen port", "порт прослушивания на сервере"), fm.inputs[1].Value())
		if err != nil {
			return nil, err
		}
		forward.LocalAddr = strings.TrimSpace(fm.inputs[2].Value())
		if forward.LocalAddr == "" {
			forward.LocalAddr = "127.0.0.1"
		}
		forward.LocalPort, err = parseNamedPort(i18n.T("Local target port", "локальный целевой порт"), fm.inputs[3].Value())
		if err != nil {
			return nil, err
		}
	case model.ForwardDynamic:
		forward.LocalAddr = strings.TrimSpace(fm.inputs[0].Value())
		if forward.LocalAddr == "" {
			forward.LocalAddr = "127.0.0.1"
		}
		forward.LocalPort, err = parseNamedPort(i18n.T("Listen port", "порт прослушивания"), fm.inputs[1].Value())
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%s", i18n.Tf("unsupported forward type: %s", "Неподдерживаемый тип проброса: %s", fm.currentType))
	}
	return forward, nil
}

func (fm *forwardFormModel) runSave() tea.Cmd {
	return func() tea.Msg {
		forward, err := fm.buildForwardFromForm()
		if err != nil {
			return saveDoneMsg{err: err}
		}
		if fm.editMode {
			if UpdateForward == nil {
				return saveDoneMsg{err: fmt.Errorf("%s", i18n.T("update not available", "Обновление недоступно"))}
			}
			return saveDoneMsg{err: UpdateForward(forward)}
		}
		if SaveForward == nil {
			return saveDoneMsg{err: fmt.Errorf("%s", i18n.T("forward storage is unavailable", "Хранилище пробросов портов недоступно"))}
		}
		return saveDoneMsg{err: SaveForward(forward)}
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
	case strings.Contains(message, "name is required"), strings.Contains(message, "укажите имя"):
		fm.focusIdx = 0
		fm.updateFocus()
		return
	case strings.Contains(message, "listen address"), strings.Contains(message, "адрес прослушивания"):
		fieldIndex = 0
	case strings.Contains(message, "listen port"), strings.Contains(message, "порт прослушивания"):
		fieldIndex = 1
	case strings.Contains(message, "target host"), strings.Contains(message, "целевой хост"):
		fieldIndex = 2
	case strings.Contains(message, "target port"), strings.Contains(message, "целевой порт"):
		fieldIndex = 3
	}
	if fieldIndex >= 0 {
		fm.focusIdx = 2 + len(forwardTypes) + 1 + fieldIndex
		fm.updateFocus()
	}
}

func parseNamedPort(label, value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s", i18n.Tf("%s must be a number from 1 to 65535", "%s должен быть числом от 1 до 65535", label))
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s", i18n.Tf("%s must be between 1 and 65535", "%s должен быть от 1 до 65535", label))
	}
	return port, nil
}

func (fm *forwardFormModel) View() string {
	title := i18n.T("Add Port Forward", "Добавить проброс порта")
	if fm.editMode {
		title = i18n.T("Edit Port Forward", "Изменить проброс порта")
	}
	notification := ""
	if fm.err != nil {
		notification = errorStyle.Render(i18n.Tf("✗ Error: %v", "✗ Ошибка: %v", fm.err))
	} else if fm.saved {
		notification = successStyle.Render(i18n.T("✓ Saved.", "✓ Сохранено."))
	}
	body := func(width, height int) string {
		contentWidth := max(1, width-4)
		lines := []string{fm.nameInput.View(), fm.descInput.View()}
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
			label := forwardType.label
			switch forwardType.value {
			case model.ForwardLocal:
				label = i18n.T("Local", "Локальный")
			case model.ForwardRemote:
				label = i18n.T("Remote", "Удалённый")
			case model.ForwardDynamic:
				label = i18n.T("SOCKS", "SOCKS")
			}
			typeParts[i] = fmt.Sprintf("%s%s %d %s", focus, selected, i+1, label)
		}
		lines = append(lines, i18n.T("Type  ", "Тип  ")+strings.Join(typeParts, "   "))
		if width >= 100 {
			description := forwardTypes[fm.typeIdx].description
			switch fm.currentType {
			case model.ForwardLocal:
				description = i18n.T("port on my machine → service on SSH server", "порт на моём компьютере → служба на SSH-сервере")
			case model.ForwardRemote:
				description = i18n.T("port on SSH server → service on my machine", "порт на SSH-сервере → служба на моём компьютере")
			case model.ForwardDynamic:
				description = i18n.T("local dynamic SOCKS proxy through SSH", "локальный динамический SOCKS-прокси через SSH")
			}
			lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(description))
		}
		enabledMark := "[ ]"
		if fm.enabled {
			enabledMark = "[x]"
		}
		enabledLine := i18n.T("  Enabled ", "  Включено ") + enabledMark
		if fm.focusIdx == 2+3 {
			enabledLine = selectedStyle.Render(i18n.T("> Enabled ", "> Включено ") + enabledMark + i18n.T("  Enter/Space toggles", "  Enter/Space переключает"))
		}
		lines = append(lines, enabledLine)
		visible := fm.visibleFields()
		for _, idx := range visible {
			lines = append(lines, fm.inputs[idx].View())
		}
		if strings.TrimSpace(fm.inputs[0].Value()) == "0.0.0.0" {
			lines = append(lines, helpStyle.Copy().MarginLeft(0).Render(i18n.T("⚠ This port will be accessible from the network.", "⚠ Этот порт будет доступен из сети.")))
		}
		if width >= 70 && fm.currentType != "" && fm.inputs[1].Value() != "" {
			if fwd, err := fm.buildForwardFromForm(); err == nil {
				preview := i18n.T("Preview  ssh ", "Предпросмотр  ssh ") + strings.Join(fwd.ForwardSSHArgs(), " ") + " -o ExitOnForwardFailure=yes"
				lines = append(lines, wrapCells(preview, contentWidth)...)
			}
		}
		total := 2 + 3 + 1 + len(visible) + 1
		button := i18n.T("  [ Save ]", "  [ Сохранить ]")
		if fm.focusIdx == total-1 {
			button = selectedStyle.Render(i18n.T("> [ Save ]", "> [ Сохранить ]"))
		}
		lines = append(lines, "", button)
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Port Forwards / ", "Пробросы портов / ") + title,
		status:       string(fm.currentType),
		notification: notification,
		width:        fm.width,
		height:       fm.height,
		body:         body,
		footer: []helpItem{
			{Key: "Tab/↓", Action: i18n.T("next", "далее")},
			{Key: "↑", Action: i18n.T("prev", "назад")},
			{Key: "1/2/3", Action: i18n.T("select type", "выбрать тип")},
			{Key: "Enter/Space", Action: i18n.T("toggle/save", "переключить/сохранить")},
			{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}

// forwardEditSignal is sent when user wants to edit a forward
type forwardEditSignal struct{}

// forwardDeleteConfirmMsg asks for confirmation before deleting
type forwardDeleteConfirmMsg struct {
	id   int64
	name string
}
