package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
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
	labels := []string{i18n.T("Name", "Имя"), i18n.T("Command", "Команда"), i18n.T("Description", "Описание")}
	inputs := make([]textinput.Model, len(labels))
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 512
	}
	inputs[0].Placeholder = "uptime"
	inputs[1].Placeholder = "uptime"
	inputs[2].Placeholder = i18n.T("optional", "необязательно")
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
			return saveDoneMsg{err: fmt.Errorf("%s", i18n.T("template storage is unavailable", "Хранилище шаблонов недоступно"))}
		}
		t := &model.CommandTemplate{
			Name:        strings.TrimSpace(tf.inputs[0].Value()),
			Command:     strings.TrimSpace(tf.inputs[1].Value()),
			Description: strings.TrimSpace(tf.inputs[2].Value()),
		}
		if t.Name == "" {
			return saveDoneMsg{err: fmt.Errorf("%s", i18n.T("name is required", "укажите имя"))}
		}
		if t.Command == "" {
			return saveDoneMsg{err: fmt.Errorf("%s", i18n.T("command is required", "укажите команду"))}
		}
		if err := SaveCommandTemplate(tf.oldName, t); err != nil {
			return saveDoneMsg{err: err}
		}
		return saveDoneMsg{}
	}
}

func (tf *templateFormModel) View() string {
	title := i18n.T("Add Template", "Добавить шаблон")
	if tf.edit {
		title = i18n.T("Edit Template", "Изменить шаблон")
	}
	notification := ""
	if tf.err != nil {
		notification = errorStyle.Render(tf.err.Error())
	} else if tf.saved {
		notification = successStyle.Render(i18n.T("✓ Saved.", "✓ Сохранено."))
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Command Templates / ", "Шаблоны команд / ") + title,
		status:       i18n.T("Template editor", "Редактор шаблона"),
		notification: notification,
		width:        tf.width,
		height:       tf.height,
		body: func(width, height int) string {
			lines := make([]string, 0, len(tf.inputs)+3)
			for i := range tf.inputs {
				lines = append(lines, tf.inputs[i].View())
			}
			button := i18n.T("  [ Save ]", "  [ Сохранить ]")
			if tf.focusIdx == len(tf.inputs) {
				button = selectedStyle.Render(i18n.T("> [ Save ]", "> [ Сохранить ]"))
			}
			lines = append(lines, "", button)
			return renderPaddedPanel(width, height, lines)
		},
		footer: []helpItem{
			{Key: "Tab/↓", Action: i18n.T("next", "далее")},
			{Key: "↑", Action: i18n.T("prev", "назад")},
			{Key: "Enter", Action: i18n.T("select", "выбрать")},
			{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}
