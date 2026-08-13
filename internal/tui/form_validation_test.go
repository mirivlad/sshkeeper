package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestParsePortRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "abc", "0", "65536", "22x"} {
		t.Run(value, func(t *testing.T) {
			if _, err := parsePort(value); err == nil {
				t.Fatalf("parsePort(%q) succeeded", value)
			}
		})
	}
	if port, err := parsePort("22"); err != nil || port != 22 {
		t.Fatalf("parsePort(22) = %d, %v", port, err)
	}
}

func TestServerFormPreservesInvalidPortAndDoesNotSave(t *testing.T) {
	fm := newFormModel(80, 24)
	fm.inputs[0].SetValue("prod")
	fm.inputs[2].SetValue("prod.example")
	fm.inputs[3].SetValue("abc")
	oldSave := SaveServer
	t.Cleanup(func() { SaveServer = oldSave })
	saves := 0
	SaveServer = func(*model.Server, string, string) error {
		saves++
		return nil
	}

	cmd := fm.runSave()
	if cmd == nil {
		t.Fatal("expected validation result command")
	}
	updated, _ := fm.Update(cmd())
	fm = updated.(*formModel)
	if saves != 0 || fm.inputs[3].Value() != "abc" {
		t.Fatalf("invalid input was lost or saved: saves=%d value=%q", saves, fm.inputs[3].Value())
	}
	if fm.err == nil || !strings.Contains(fm.err.Error(), "Port") {
		t.Fatalf("missing actionable port error: %v", fm.err)
	}
	if view := fm.View(); !strings.Contains(view, "Port must be a number") {
		t.Fatalf("validation error is not rendered:\n%s", view)
	}
}

func TestDirtyServerFormRequiresDiscardConfirmation(t *testing.T) {
	oldList := ListServers
	t.Cleanup(func() { ListServers = oldList })
	ListServers = func() ([]*model.Server, error) { return nil, nil }

	m := New(nil)
	m.screen = screenForm
	m.form = newFormModel(80, 24)
	m.form.inputs[0].SetValue("prod")

	updated, cmd := m.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	if cmd != nil || m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenForm {
		t.Fatalf("dirty form did not open discard confirmation: screen=%v confirm=%#v", m.screen, m.confirm)
	}
	updated, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if m.screen != screenForm || m.form == nil || m.form.inputs[0].Value() != "prod" {
		t.Fatalf("Cancel did not preserve form: screen=%v form=%#v", m.screen, m.form)
	}

	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)
	updated, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*tuiModel)
	updated, cmd = m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*tuiModel)
	if cmd == nil {
		t.Fatal("expected discard command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(*tuiModel)
	if m.screen != screenList || m.form != nil || m.confirm != nil {
		t.Fatalf("discard did not return to list: screen=%v form=%v confirm=%v", m.screen, m.form, m.confirm)
	}
}

func TestCleanFormsExitWithoutConfirmation(t *testing.T) {
	oldList := ListServers
	t.Cleanup(func() { ListServers = oldList })
	ListServers = func() ([]*model.Server, error) { return nil, nil }

	tests := []struct {
		name   string
		screen screen
		setup  func(*tuiModel)
		exit   func(*tuiModel) (tea.Model, tea.Cmd)
		want   screen
	}{
		{
			name:   "server",
			screen: screenForm,
			setup:  func(m *tuiModel) { m.form = newFormModel(80, 24) },
			exit:   func(m *tuiModel) (tea.Model, tea.Cmd) { return m.updateForm(tea.KeyMsg{Type: tea.KeyEsc}) },
			want:   screenList,
		},
		{
			name:   "forward",
			screen: screenForwardForm,
			setup: func(m *tuiModel) {
				m.forwardScreen = newForwardScreenModel(1, "prod", 80, 24)
				m.forwardForm = newForwardFormModel(1, 80, 24)
			},
			exit: func(m *tuiModel) (tea.Model, tea.Cmd) {
				return m.updateForwardForm(tea.KeyMsg{Type: tea.KeyEsc})
			},
			want: screenForwardList,
		},
		{
			name:   "template",
			screen: screenTemplateForm,
			setup:  func(m *tuiModel) { m.templateForm = newTemplateFormModel(nil, 80, 24) },
			exit: func(m *tuiModel) (tea.Model, tea.Cmd) {
				return m.updateTemplateForm(tea.KeyMsg{Type: tea.KeyEsc})
			},
			want: screenTemplates,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil)
			m.screen = tt.screen
			tt.setup(m)
			updated, _ := tt.exit(m)
			m = updated.(*tuiModel)
			if m.screen != tt.want || m.confirm != nil {
				t.Fatalf("clean exit: screen=%v confirm=%v", m.screen, m.confirm)
			}
		})
	}
}

func TestDirtyForwardAndTemplateFormsRequireConfirmation(t *testing.T) {
	t.Run("forward", func(t *testing.T) {
		m := New(nil)
		m.screen = screenForwardForm
		m.forwardScreen = newForwardScreenModel(1, "prod", 80, 24)
		m.forwardForm = newForwardFormModel(1, 80, 24)
		m.forwardForm.nameInput.SetValue("postgres")
		updated, _ := m.updateForwardForm(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(*tuiModel)
		if m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenForwardForm {
			t.Fatalf("dirty forward form did not confirm: screen=%v confirm=%#v", m.screen, m.confirm)
		}
	})

	t.Run("template", func(t *testing.T) {
		m := New(nil)
		m.screen = screenTemplateForm
		m.templateForm = newTemplateFormModel(nil, 80, 24)
		m.templateForm.inputs[0].SetValue("uptime")
		updated, _ := m.updateTemplateForm(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(*tuiModel)
		if m.screen != screenConfirm || m.confirm == nil || m.confirm.parent != screenTemplateForm {
			t.Fatalf("dirty template form did not confirm: screen=%v confirm=%#v", m.screen, m.confirm)
		}
	})
}

func TestRequiredFieldsAreMarked(t *testing.T) {
	serverView := newFormModel(100, 30).View()
	for _, want := range []string{"Alias *", "Host *", "Port *"} {
		if !strings.Contains(serverView, want) {
			t.Fatalf("server form missing %q:\n%s", want, serverView)
		}
	}

	forwardView := newForwardFormModel(1, 100, 30).View()
	for _, want := range []string{"Name *", "Listen Port *", "Target Host *", "Target Port *"} {
		if !strings.Contains(forwardView, want) {
			t.Fatalf("forward form missing %q:\n%s", want, forwardView)
		}
	}

	templateView := newTemplateFormModel(nil, 100, 30).View()
	for _, want := range []string{"Name *", "Command *"} {
		if !strings.Contains(templateView, want) {
			t.Fatalf("template form missing %q:\n%s", want, templateView)
		}
	}
}
