package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Template form model ---

type templateFormModel struct {
	edit     bool
	oldName  string
	inputs   []textinput.Model
	labels   []string
	focusIdx int
	err      error
	saved    bool
	width    int
	height   int
	initial  []string
}

func newTemplateFormModel(t *model.CommandTemplate, w, h int) *templateFormModel {
	labels := []string{"Name", "Command", "Description"}
	inputs := make([]textinput.Model, len(labels))
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 512
	}
	inputs[0].Placeholder = "uptime"
	inputs[1].Placeholder = "uptime"
	inputs[2].Placeholder = "optional"
	inputs[0].Focus()

	tf := &templateFormModel{inputs: inputs, labels: labels, width: w, height: h}
	if t != nil {
		tf.edit = true
		tf.oldName = t.Name
		inputs[0].SetValue(t.Name)
		inputs[1].SetValue(t.Command)
		inputs[2].SetValue(t.Description)
	}
	tf.updateFocus()
	tf.initial = tf.snapshot()
	return tf
}

func (tf *templateFormModel) snapshot() []string {
	values := make([]string, len(tf.inputs))
	for i := range tf.inputs {
		values[i] = tf.inputs[i].Value()
	}
	return values
}

func (tf *templateFormModel) Dirty() bool {
	current := tf.snapshot()
	if len(current) != len(tf.initial) {
		return true
	}
	for i := range current {
		if current[i] != tf.initial[i] {
			return true
		}
	}
	return false
}

func (tf *templateFormModel) Init() tea.Cmd {
	return nil
}

func (tf *templateFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab, tea.KeyDown:
			tf.focusIdx++
			if tf.focusIdx > len(tf.inputs) {
				tf.focusIdx = 0
			}
			tf.updateFocus()
			return tf, nil
		case tea.KeyShiftTab, tea.KeyUp:
			tf.focusIdx--
			if tf.focusIdx < 0 {
				tf.focusIdx = len(tf.inputs)
			}
			tf.updateFocus()
			return tf, nil
		case tea.KeyEnter:
			if tf.focusIdx == len(tf.inputs) {
				return tf, tf.save()
			}
			tf.focusIdx++
			tf.updateFocus()
			return tf, nil
		}
	}
	if tf.focusIdx < len(tf.inputs) {
		var cmd tea.Cmd
		tf.inputs[tf.focusIdx], cmd = tf.inputs[tf.focusIdx].Update(msg)
		return tf, cmd
	}
	return tf, nil
}

func (tf *templateFormModel) updateFocus() {
	for i := range tf.inputs {
		tf.inputs[i].Blur()
		tf.inputs[i].Prompt = blurredStyle.Render(tf.labelAt(i) + ": ")
	}
	if tf.focusIdx < len(tf.inputs) {
		tf.inputs[tf.focusIdx].Focus()
		tf.inputs[tf.focusIdx].Prompt = focusedStyle.Render(tf.labelAt(tf.focusIdx) + "> ")
	}
}

func (tf *templateFormModel) labelAt(index int) string {
	if index == 0 || index == 1 {
		return tf.labels[index] + " *"
	}
	return tf.labels[index]
}

func (tf *templateFormModel) save() tea.Cmd {
	return func() tea.Msg {
		if SaveCommandTemplate == nil {
			return saveDoneMsg{err: fmt.Errorf("template storage is unavailable")}
		}
		t := &model.CommandTemplate{
			Name:        strings.TrimSpace(tf.inputs[0].Value()),
			Command:     strings.TrimSpace(tf.inputs[1].Value()),
			Description: strings.TrimSpace(tf.inputs[2].Value()),
		}
		if t.Name == "" {
			return saveDoneMsg{err: fmt.Errorf("name is required")}
		}
		if t.Command == "" {
			return saveDoneMsg{err: fmt.Errorf("command is required")}
		}
		if err := SaveCommandTemplate(tf.oldName, t); err != nil {
			return saveDoneMsg{err: err}
		}
		return saveDoneMsg{}
	}
}

func (tf *templateFormModel) View() string {
	var b strings.Builder
	title := "Add Template"
	if tf.edit {
		title = "Edit Template"
	}
	b.WriteString(titleStyle.Copy().MarginLeft(0).Render(fitLine(title, tf.width)))
	b.WriteString("\n\n")
	for i := range tf.inputs {
		b.WriteString(fitLine(tf.inputs[i].View(), tf.width))
		b.WriteString("\n")
	}
	button := "  [ Save ]"
	if tf.focusIdx == len(tf.inputs) {
		button = selectedStyle.Render("> [ Save ]")
	}
	b.WriteString("\n" + button + "\n\n")
	if tf.err != nil {
		b.WriteString(errorStyle.Render(tf.err.Error()))
		b.WriteString("\n")
	}
	b.WriteString(renderHelp([]helpItem{
		{Key: "Tab/↓", Action: "next"},
		{Key: "↑", Action: "prev"},
		{Key: "Enter", Action: "select"},
		{Key: "Esc", Action: "back"},
	}, tf.width))
	return b.String()
}
