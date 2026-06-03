package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

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

type forwardItem struct {
	forward *model.Forward
}

func (i forwardItem) Title() string {
	return fmt.Sprintf("[%s] %s", i.forward.Type, forwardSummary(i.forward))
}
func (i forwardItem) Description() string {
	return forwardPreview(i.forward)
}
func (i forwardItem) FilterValue() string {
	return string(i.forward.Type) + " " + fmt.Sprintf("%d", i.forward.LocalPort) + " " + i.forward.RemoteAddr
}

func forwardSummary(f *model.Forward) string {
	switch f.Type {
	case model.ForwardLocal:
		return fmt.Sprintf("%s:%d → %s:%d", f.LocalAddr, f.LocalPort, f.RemoteAddr, f.RemotePort)
	case model.ForwardRemote:
		return fmt.Sprintf("%s:%d ← %s:%d", f.RemoteAddr, f.RemotePort, f.LocalAddr, f.LocalPort)
	case model.ForwardDynamic:
		return fmt.Sprintf("SOCKS %s:%d", f.LocalAddr, f.LocalPort)
	default:
		return fmt.Sprintf("%s %s:%d", f.Type, f.LocalAddr, f.LocalPort)
	}
}

func forwardPreview(f *model.Forward) string {
	switch f.Type {
	case model.ForwardLocal:
		return fmt.Sprintf("-L %s:%d:%s:%d", f.LocalAddr, f.LocalPort, f.RemoteAddr, f.RemotePort)
	case model.ForwardRemote:
		return fmt.Sprintf("-R %s:%d:%s:%d", f.RemoteAddr, f.RemotePort, f.LocalAddr, f.LocalPort)
	case model.ForwardDynamic:
		return fmt.Sprintf("-D %s:%d", f.LocalAddr, f.LocalPort)
	default:
		return string(f.Type)
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

func (m *forwardScreenModel) rebuildList() {
	items := make([]list.Item, len(m.forwards))
	for i, f := range m.forwards {
		items[i] = forwardItem{forward: f}
	}
	m.list.SetItems(items)
}

func (m *forwardScreenModel) deleteSelected() tea.Cmd {
	if item, ok := m.list.SelectedItem().(forwardItem); ok && DeleteForward != nil {
		f := item.forward
		return func() tea.Msg {
			return forwardDeletedMsg{id: f.ID, err: DeleteForward(f.ID)}
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
		{Key: "Ctrl+D (d)", Action: "delete"},
		{Key: "Esc", Action: "back"},
	}, m.width))
	return b.String()
}

// --- Forward form screen model ---

type forwardFormModel struct {
	serverID int64
	inputs   []textinput.Model
	labels   []string
	focusIdx int
	err      error
	saved    bool
	typeList list.Model
	showList bool
	width    int
	height   int
}

func newForwardFormModel(serverID int64, w, h int) *forwardFormModel {
	labels := []string{"Type (local/remote/dynamic)", "Listen Addr", "Listen Port", "Target Addr", "Target Port"}
	inputs := make([]textinput.Model, len(labels))
	for i, label := range labels {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = forwardPlaceholder(label)
		inputs[i].CharLimit = 128
	}
	inputs[0].Focus()

	typeList := newStringList([]string{"local", "remote", "dynamic"}, "Select type", 30, 8)

	return &forwardFormModel{
		serverID: serverID,
		inputs:   inputs,
		labels:   labels,
		focusIdx: 0,
		typeList: typeList,
		width:    w,
		height:   h,
	}
}

func forwardPlaceholder(label string) string {
	switch label {
	case "Type (local/remote/dynamic)":
		return "local"
	case "Listen Addr":
		return "0.0.0.0"
	case "Listen Port":
		return "8080"
	case "Target Addr":
		return "internal.web"
	case "Target Port":
		return "80"
	default:
		return label
	}
}

func (fm *forwardFormModel) Init() tea.Cmd {
	return nil
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
				if item, ok := fm.typeList.SelectedItem().(groupItem); ok {
					fm.inputs[0].SetValue(item.name)
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
			total := len(fm.inputs) + 1
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyShiftTab:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := len(fm.inputs) + 1
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
			if fm.focusIdx == len(fm.inputs) {
				return fm, fm.runSave()
			}
			fm.focusIdx++
			fm.updateFocus()
			return fm, nil
		case tea.KeyEsc:
			return fm, nil
		case tea.KeyDown:
			fm.focusIdx++
			total := len(fm.inputs) + 1
			if fm.focusIdx >= total {
				fm.focusIdx = 0
			}
			fm.updateFocus()
			return fm, nil
		case tea.KeyUp:
			fm.focusIdx--
			if fm.focusIdx < 0 {
				total := len(fm.inputs) + 1
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

	return fm, nil
}

func (fm *forwardFormModel) updateFocus() {
	for i := range fm.inputs {
		fm.inputs[i].Blur()
		fm.inputs[i].Prompt = blurredStyle.Render(fm.labels[i] + ": ")
	}
	if fm.focusIdx < len(fm.inputs) {
		fm.inputs[fm.focusIdx].Focus()
		fm.inputs[fm.focusIdx].Prompt = focusedStyle.Render(fm.labels[fm.focusIdx] + "> ")
	}
}

func (fm *forwardFormModel) runSave() tea.Cmd {
	return func() tea.Msg {
		fwdType := model.ForwardType(strings.TrimSpace(fm.inputs[0].Value()))
		if fwdType == "" {
			fwdType = model.ForwardLocal
		}
		localPort := 0
		fmt.Sscanf(fm.inputs[2].Value(), "%d", &localPort)
		remotePort := 0
		fmt.Sscanf(fm.inputs[4].Value(), "%d", &remotePort)

		fwd := &model.Forward{
			ServerID:   fm.serverID,
			Type:       fwdType,
			LocalAddr:  fm.inputs[1].Value(),
			LocalPort:  localPort,
			RemoteAddr: fm.inputs[3].Value(),
			RemotePort: remotePort,
		}

		if SaveForward == nil {
			return saveDoneMsg{err: fmt.Errorf("forward storage is unavailable")}
		}
		if err := SaveForward(fwd); err != nil {
			return saveDoneMsg{err: err}
		}
		return saveDoneMsg{}
	}
}

func (fm *forwardFormModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add Port Forward"))
	b.WriteString("\n\n")

	for i := range fm.inputs {
		b.WriteString(fm.inputs[i].View())
		b.WriteString("\n")
		if i == 0 && fm.showList {
			b.WriteString("\n" + renderDropdown(fm.typeList) + "\n")
			b.WriteString(renderHelp([]helpItem{{Key: "Enter", Action: "select"}, {Key: "Esc", Action: "cancel"}}, fm.width))
			return b.String()
		}
	}

	// Preview
	fwdType := strings.TrimSpace(fm.inputs[0].Value())
	localAddr := fm.inputs[1].Value()
	localPort := fm.inputs[2].Value()
	remoteAddr := fm.inputs[3].Value()
	remotePort := fm.inputs[4].Value()

	if fwdType != "" && localPort != "" {
		b.WriteString("\n" + sectionStyle.Render("Preview") + "\n")
		switch fwdType {
		case "local":
			b.WriteString(fmt.Sprintf("  -L %s:%s:%s:%s\n", localAddr, localPort, remoteAddr, remotePort))
		case "remote":
			b.WriteString(fmt.Sprintf("  -R %s:%s:%s:%s\n", remoteAddr, remotePort, localAddr, localPort))
		case "dynamic":
			b.WriteString(fmt.Sprintf("  -D %s:%s\n", localAddr, localPort))
		}
		b.WriteString("  -o ExitOnForwardFailure=yes\n")
	}

	button := "\n[ Save ]"
	if fm.focusIdx == len(fm.inputs) {
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
		{Key: "/", Action: "pick type"},
		{Key: "Enter", Action: "save"},
		{Key: "Esc", Action: "back"},
	}, fm.width))

	return b.String()
}
