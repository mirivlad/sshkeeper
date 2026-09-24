package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
)

var languageOptions = []string{i18n.Auto, i18n.Russian, i18n.English}

type settingsModel struct {
	width      int
	height     int
	preference string
	selected   int
	err        error
}

func newSettingsModel(width, height int, preference string) *settingsModel {
	model := &settingsModel{width: width, height: height, preference: preference}
	for index, option := range languageOptions {
		if option == preference {
			model.selected = index
			break
		}
	}
	return model
}

// Update returns a requested preference change or a request to go back. The
// caller persists the change before updating the active preference.
func (m *settingsModel) Update(msg tea.KeyMsg) (selected string, changed, back bool) {
	m.err = nil
	switch msg.Type {
	case tea.KeyEsc:
		return "", false, true
	case tea.KeyUp:
		m.selected = (m.selected + len(languageOptions) - 1) % len(languageOptions)
	case tea.KeyDown, tea.KeyTab:
		m.selected = (m.selected + 1) % len(languageOptions)
	case tea.KeyEnter:
		option := languageOptions[m.selected]
		return option, option != m.preference, false
	}
	return "", false, false
}

func (m *settingsModel) applyPreference(preference string) {
	m.preference = preference
	m.err = nil
}

func (m *settingsModel) View() string {
	notification := ""
	if m.err != nil {
		notification = i18n.Tf("Could not save language: %v", "Не удалось сохранить язык: %v", m.err)
	}
	body := func(width, height int) string {
		lines := []string{
			dashboardSection(i18n.T("Language", "Язык")),
			"",
		}
		for index, option := range languageOptions {
			marker := "  "
			if index == m.selected {
				marker = "> "
			}
			current := ""
			if option == m.preference {
				current = "  " + i18n.T("(current)", "(текущий)")
			}
			label := option
			switch option {
			case i18n.Auto:
				language := "English"
				if i18n.SystemLanguage() == i18n.Russian {
					language = "Русский"
				}
				label = fmt.Sprintf("%s (%s)", i18n.T("System", "Системный"), language)
			case i18n.Russian:
				label = "Русский"
			case i18n.English:
				label = "English"
			}
			lines = append(lines, marker+label+current)
		}
		lines = append(lines, "", i18n.T("Enter applies immediately. Esc returns to Manage.", "Enter применяет сразу. Esc возвращает в меню."))
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Settings", "Настройки"),
		status:       i18n.T("Language", "Язык"),
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "↑/↓", Action: i18n.T("choose", "выбрать")},
			{Key: "Enter", Action: i18n.T("apply", "применить")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}
