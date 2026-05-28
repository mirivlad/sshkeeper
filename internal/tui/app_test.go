package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestServerListViewUsesDashboardLayout(t *testing.T) {
	now := time.Date(2026, 5, 28, 1, 50, 0, 0, time.UTC)
	m := New([]*model.Server{
		{
			Alias:          "mail.kp",
			DisplayName:    "Mail",
			Host:           "mail.example.org",
			Port:           222,
			User:           "mirivlad",
			AuthMethod:     model.AuthPassword,
			GroupName:      "KP",
			LastTestStatus: model.TestOK,
			LastTestAt:     &now,
		},
		{
			Alias:          "mirv.top",
			Host:           "mirv.top",
			Port:           22,
			User:           "root",
			AuthMethod:     model.AuthKey,
			LastTestStatus: model.TestUnknown,
		},
	})
	m.width = 100
	m.height = 30
	m.list.SetSize(100, 24)

	view := m.View()
	for _, want := range []string{
		"sshkeeper",
		"2 servers",
		"Vault",
		"NAME",
		"TARGET",
		"AUTH",
		"GROUP",
		"STATUS",
		"Mail",
		"mail.kp",
		"mirivlad@mail.example.org:222",
		"KP",
		"OK",
		"Selected",
		"Host: mail.example.org",
		"Alias: mail.kp",
		"Display Name: Mail",
		"Port: 222",
		"Enter connect",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected list view to contain %q\nview:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Profiles managed locally") {
		t.Fatalf("expected compact status header instead of README text\nview:\n%s", view)
	}
}

func TestServerListViewKeepsDetailsVisibleWithManyServers(t *testing.T) {
	servers := make([]*model.Server, 45)
	for i := range servers {
		servers[i] = &model.Server{
			Alias:          fmt.Sprintf("server-%02d", i+1),
			DisplayName:    fmt.Sprintf("Server %02d", i+1),
			Host:           fmt.Sprintf("host-%02d.example.org", i+1),
			Port:           22,
			User:           "mirivlad",
			AuthMethod:     model.AuthKey,
			LastTestStatus: model.TestUnknown,
		}
	}

	m := New(servers)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	model := updated.(*tuiModel)

	view := model.View()
	if !strings.Contains(view, "Server 01") {
		t.Fatalf("expected first selected server to be visible:\n%s", view)
	}
	if !strings.Contains(view, "Selected") {
		t.Fatalf("expected selected server details to remain visible:\n%s", view)
	}
	if !strings.Contains(view, "Enter connect") {
		t.Fatalf("expected footer to remain visible:\n%s", view)
	}
	if count := strings.Count(view, "server-"); count >= len(servers) {
		t.Fatalf("expected bounded row rendering, rendered %d server aliases", count)
	}
}

func TestVisibleServerRangeKeepsSelectionInsideWindow(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		selected  int
		available int
		wantStart int
		wantEnd   int
	}{
		{name: "first page", total: 40, selected: 0, available: 10, wantStart: 0, wantEnd: 10},
		{name: "middle page", total: 40, selected: 20, available: 10, wantStart: 11, wantEnd: 21},
		{name: "last page", total: 40, selected: 39, available: 10, wantStart: 30, wantEnd: 40},
		{name: "all fit", total: 5, selected: 3, available: 10, wantStart: 0, wantEnd: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visibleServerRange(tt.total, tt.selected, tt.available)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("visibleServerRange() = %d, %d; want %d, %d", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestServerListViewScrollsWithSelection(t *testing.T) {
	servers := make([]*model.Server, 45)
	for i := range servers {
		servers[i] = &model.Server{
			Alias:       fmt.Sprintf("server-%02d", i+1),
			DisplayName: fmt.Sprintf("Server %02d", i+1),
			Host:        fmt.Sprintf("host-%02d.example.org", i+1),
			Port:        22,
			User:        "mirivlad",
			AuthMethod:  model.AuthKey,
		}
	}

	m := New(servers)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	model := updated.(*tuiModel)
	for i := 0; i < 20; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(*tuiModel)
	}

	view := model.View()
	if !strings.Contains(view, "Server 21") {
		t.Fatalf("expected selected server to be visible after navigation:\n%s", view)
	}
	if !strings.Contains(view, "Showing") {
		t.Fatalf("expected range hint for long server list:\n%s", view)
	}
}

func TestEscClosesGroupListBeforeLeavingForm(t *testing.T) {
	oldGetGroups := GetGroups
	GetGroups = func() ([]string, error) {
		return []string{"prod", "stage"}, nil
	}
	defer func() { GetGroups = oldGetGroups }()

	m := &tuiModel{
		screen: screenForm,
		form:   newFormModel(80, 24),
	}
	m.form.focusIdx = 8
	m.form.updateFocus()

	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(*tuiModel)
	if !m.form.showGroupList {
		t.Fatal("expected / on the group field to open the group list")
	}

	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)

	if m.screen != screenForm {
		t.Fatalf("expected Esc to keep the user in the form, got screen %v", m.screen)
	}
	if m.form == nil {
		t.Fatal("expected form to remain open")
	}
	if m.form.showGroupList {
		t.Fatal("expected Esc to close only the group list")
	}
}

func TestEscClosesAuthMethodListBeforeLeavingForm(t *testing.T) {
	m := &tuiModel{
		screen: screenForm,
		form:   newFormModel(80, 24),
	}
	m.form.focusIdx = 5
	m.form.updateFocus()

	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(*tuiModel)
	if !m.form.showAuthList {
		t.Fatal("expected / on the auth method field to open the auth method list")
	}

	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tuiModel)

	if m.screen != screenForm {
		t.Fatalf("expected Esc to keep the user in the form, got screen %v", m.screen)
	}
	if m.form == nil {
		t.Fatal("expected form to remain open")
	}
	if m.form.showAuthList {
		t.Fatal("expected Esc to close only the auth method list")
	}
}

func TestAuthMethodListSelectsValue(t *testing.T) {
	fm := newFormModel(80, 24)
	fm.focusIdx = 5
	fm.updateFocus()

	updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	fm = updated.(*formModel)
	if !fm.showAuthList {
		t.Fatal("expected / on the auth method field to open the auth method list")
	}

	fm.authList.Select(2)
	updated, _ = fm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	fm = updated.(*formModel)

	if fm.showAuthList {
		t.Fatal("expected Enter to close auth method list")
	}
	if got := fm.inputs[5].Value(); got != string(model.AuthKeyPassphrase) {
		t.Fatalf("expected auth method %q, got %q", model.AuthKeyPassphrase, got)
	}
}

func TestAuthMethodListViewShowsAllOptions(t *testing.T) {
	fm := newFormModel(80, 12)
	fm.focusIdx = 5
	fm.updateFocus()

	updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	fm = updated.(*formModel)

	view := fm.View()
	authPos := strings.Index(view, "Auth Method")
	listPos := strings.Index(view, "Select auth method")
	if authPos < 0 || listPos < 0 {
		t.Fatalf("expected auth field and auth list title in view\nview:\n%s", view)
	}
	if listPos < authPos {
		t.Fatalf("expected auth method list after auth field\nview:\n%s", view)
	}
	if between := view[authPos:listPos]; strings.Contains(between, "Identity File") {
		t.Fatalf("expected auth method list to render directly under auth field\nview:\n%s", view)
	}
	if strings.Contains(view, "│") {
		t.Fatalf("expected compact auth method dropdown without default list border\nview:\n%s", view)
	}
	for _, method := range []model.AuthMethod{
		model.AuthPassword,
		model.AuthKey,
		model.AuthKeyPassphrase,
		model.AuthAgent,
	} {
		if !strings.Contains(view, string(method)) {
			t.Fatalf("expected auth method list view to contain %q\nview:\n%s", method, view)
		}
	}
}

func TestGroupListViewRendersDirectlyUnderGroupField(t *testing.T) {
	oldGetGroups := GetGroups
	GetGroups = func() ([]string, error) {
		return []string{"KP", "MY"}, nil
	}
	defer func() { GetGroups = oldGetGroups }()

	fm := newFormModel(80, 24)
	fm.focusIdx = 8
	fm.updateFocus()

	updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	fm = updated.(*formModel)

	view := fm.View()
	groupPos := strings.Index(view, "Group")
	listPos := strings.Index(view, "Select group")
	if groupPos < 0 || listPos < 0 {
		t.Fatalf("expected group field and group list title in view\nview:\n%s", view)
	}
	if listPos < groupPos {
		t.Fatalf("expected group list after group field\nview:\n%s", view)
	}
	if between := view[groupPos:listPos]; strings.Contains(between, "Password") {
		t.Fatalf("expected group dropdown to render before password field\nview:\n%s", view)
	}
	if strings.Contains(view, "│") {
		t.Fatalf("expected compact group dropdown without default list border\nview:\n%s", view)
	}
}

func TestSelectableFieldHintsAreVisible(t *testing.T) {
	oldGetGroups := GetGroups
	GetGroups = func() ([]string, error) {
		return []string{"KP"}, nil
	}
	defer func() { GetGroups = oldGetGroups }()

	fm := newFormModel(80, 24)
	view := fm.View()
	for _, want := range []string{"Auth Method (/ pick)", "Group (/ pick)", "/ pick list"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected form view to contain selectable-field hint %q\nview:\n%s", want, view)
		}
	}
}

func TestEditFormShowsSavedSecretMarker(t *testing.T) {
	oldHasSecret := HasSecret
	HasSecret = func(alias string, secretType string) bool {
		return alias == "prod" && secretType == "ssh_password"
	}
	defer func() { HasSecret = oldHasSecret }()

	fm := newEditFormModel(&model.Server{
		Alias:      "prod",
		Host:       "example.org",
		Port:       22,
		User:       "root",
		AuthMethod: model.AuthPassword,
	}, 80, 24)

	view := fm.View()
	if !strings.Contains(view, "secret saved") {
		t.Fatalf("expected edit form to show saved secret marker\nview:\n%s", view)
	}
	if !strings.Contains(view, "leave blank to keep") {
		t.Fatalf("expected edit form to explain blank password keeps saved secret\nview:\n%s", view)
	}
	if strings.Count(view, "secret saved") != 1 {
		t.Fatalf("expected saved secret marker to appear once\nview:\n%s", view)
	}
}

func TestFormViewUsesSectionsAndStableLabels(t *testing.T) {
	fm := newFormModel(100, 30)
	view := fm.View()

	for _, want := range []string{
		"Identity",
		"Connection",
		"Authentication",
		"Metadata",
		"Actions",
		"Alias",
		"Display Name",
		"Auth Method",
		"Password / Passphrase",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected form view to contain %q\nview:\n%s", want, view)
		}
	}
}

func TestFormTestResultDoesNotUpdateSelectedListServer(t *testing.T) {
	oldUpdateTestResult := UpdateTestResult
	oldListServers := ListServers
	defer func() {
		UpdateTestResult = oldUpdateTestResult
		ListServers = oldListServers
	}()

	updateCalled := false
	UpdateTestResult = func(alias string, status model.TestStatus, testErr string) error {
		updateCalled = true
		return nil
	}
	ListServers = func() ([]*model.Server, error) {
		t.Fatal("form test result should not reload server list")
		return nil, nil
	}

	selected := &model.Server{Alias: "selected", Host: "example.org", Port: 22, User: "root"}
	m := New([]*model.Server{selected})
	m.screen = screenForm
	m.form = newFormModel(80, 24)
	m.list = list.New([]list.Item{serverItem{server: selected}}, list.NewDefaultDelegate(), 80, 20)

	updated, cmd := m.Update(testDoneMsg{ok: true})
	m = updated.(*tuiModel)

	if cmd != nil {
		t.Fatal("form test result should not return a reload command")
	}
	if updateCalled {
		t.Fatal("form test result should not update the selected list server")
	}
	if m.form == nil || m.form.testResult != "Connection OK." {
		t.Fatal("expected form to keep its test result")
	}
}
