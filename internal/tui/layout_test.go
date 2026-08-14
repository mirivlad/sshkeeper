package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestTruncateCellsHandlesUnicodeDisplayWidth(t *testing.T) {
	tests := []string{
		"production-сервер",
		"数据库服务器",
		"e\u0301-combining",
		"🔐 gateway",
	}
	for _, value := range tests {
		got := truncateCells(value, 8)
		if width := ansi.StringWidth(got); width > 8 {
			t.Fatalf("truncateCells(%q) width=%d result=%q", value, width, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Fatalf("truncated value has no indicator: %q", got)
		}
	}
}

func TestDashboardFitsSupportedTerminalSizes(t *testing.T) {
	servers := []*model.Server{
		{
			Alias:       "staging-数据库-bastion",
			DisplayName: "Production сервер 🔐 with a very long display name",
			Host:        "bastion.staging.example.net",
			Port:        2222,
			User:        "operations",
			AuthMethod:  model.AuthAgent,
			GroupName:   "STAGING-LONG",
			Tags:        []string{"stage", "bastion", "кириллица"},
		},
		{Alias: "db", DisplayName: "Database", Host: "db.internal", Port: 22, User: "postgres", AuthMethod: model.AuthKeyPassphrase},
	}

	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		t.Run(strings.Join([]string{itoa(size.width), "x", itoa(size.height)}, ""), func(t *testing.T) {
			m := New(servers)
			m.width, m.height = size.width, size.height
			assertViewFits(t, m.View(), size.width, size.height)
			assertRightMargin(t, m.View(), size.width)
			for _, want := range []string{"sshkeeper", "Servers", "Vault", "Enter", "Ctrl+Q"} {
				if !strings.Contains(m.View(), want) {
					t.Fatalf("dashboard at %dx%d missing %q:\n%s", size.width, size.height, want, m.View())
				}
			}
		})
	}
}

func TestDashboardNotificationFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		m := New([]*model.Server{{Alias: "prod", Host: "prod.example", Port: 22, User: "ops", AuthMethod: model.AuthAgent}})
		m.width, m.height = size.width, size.height
		m.err = errText("vault reload failed and this message must remain visible")
		view := m.View()
		assertViewFits(t, view, size.width, size.height)
		if !strings.Contains(view, "vault reload failed") {
			t.Fatalf("dashboard at %dx%d lost notification:\n%s", size.width, size.height, view)
		}
	}
}

func TestScreensBelowSupportedFloorShowSizeMessage(t *testing.T) {
	m := New(nil)
	m.width, m.height = 59, 15
	view := m.View()
	if !strings.Contains(view, "60x16") {
		t.Fatalf("missing supported-size message:\n%s", view)
	}
	assertViewFits(t, view, 59, 15)
}

func TestServerFormFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		t.Run(itoa(size.width), func(t *testing.T) {
			fm := newFormModel(size.width, size.height)
			fm.inputs[0].SetValue("prod")
			fm.inputs[2].SetValue("prod.example")
			fm.inputs[3].SetValue("not-a-port")
			fm.err = errText("Port must be a number from 1 to 65535")
			fm.focusIdx = 3
			fm.updateFocus()
			view := fm.View()
			assertViewFits(t, view, size.width, size.height)
			assertUnifiedScreen(t, view, size.width, size.height)
			for _, want := range []string{"Server", "Port *", "not-a-port", "Port must be", "Save", "Esc"} {
				if !strings.Contains(view, want) {
					t.Fatalf("form at %dx%d missing %q:\n%s", size.width, size.height, want, view)
				}
			}
		})
	}
}

func TestForwardFormFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		fm := newForwardFormModel(1, size.width, size.height)
		fm.nameInput.SetValue("Local PostgreSQL")
		fm.inputs[0].SetValue("127.0.0.1")
		fm.inputs[1].SetValue("15432")
		fm.inputs[2].SetValue("database.internal.example")
		fm.inputs[3].SetValue("5432")
		view := fm.View()
		assertViewFits(t, view, size.width, size.height)
		assertUnifiedScreen(t, view, size.width, size.height)
	}
}

func TestForwardListFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		fm := newForwardScreenModel(1, "production-数据库-bastion", size.width, size.height)
		fm.list = []*model.Forward{
			{Name: "Local PostgreSQL with a very long name", Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "database.internal.example", RemotePort: 5432, Enabled: true},
			{Name: "SOCKS proxy", Type: model.ForwardDynamic, LocalAddr: "127.0.0.1", LocalPort: 1080, Enabled: false},
		}
		view := fm.View()
		assertViewFits(t, view, size.width, size.height)
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"Port Forwards", "Local PostgreSQL", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("forward list at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestActionMenuFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		menu := newActionMenuModel(size.width, size.height)
		view := menu.View()
		assertViewFits(t, view, size.width, size.height)
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"Actions", "Connect", "Manage port forwards", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("action menu at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestConfirmationFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		m := New(nil)
		m.width, m.height = size.width, size.height
		m.beginConfirm(confirmState{
			title:       "Delete port forward?",
			target:      `"Очень длинный Local PostgreSQL 数据库 forward" · 127.0.0.1:15432 → database.internal.example:5432`,
			consequence: "This removes the saved forwarding rule. Active tunnels are not stopped.",
			verb:        "Delete",
			parent:      screenForwardList,
		})
		view := m.View()
		assertViewFits(t, view, size.width, size.height)
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"Local PostgreSQL", "not stopped.", "> [ Cancel ]", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("confirmation at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestConfirmationKeepsActionsVisibleWithLongContent(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		m := New(nil)
		m.width, m.height = size.width, size.height
		m.beginConfirm(confirmState{
			title:       "Delete port forward?",
			target:      strings.Repeat("非常に長い-очень-длинный-🔐 ", 20),
			consequence: strings.Repeat("Active connections can be interrupted. ", 20),
			verb:        "Delete",
			parent:      screenForwardList,
		})
		view := m.View()
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"[ Cancel ]", "[ Delete ]"} {
			if !strings.Contains(view, want) {
				t.Fatalf("confirmation at %dx%d clipped %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestHelpScreensUseUnifiedShell(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		for name, view := range map[string]string{
			"quick": newHelpScreenModel(size.width, size.height).View(),
			"full":  newFullHelpModel(size.width, size.height).View(),
		} {
			t.Run(name+itoa(size.width), func(t *testing.T) {
				assertUnifiedScreen(t, view, size.width, size.height)
				if strings.Contains(view, "F1") || !strings.Contains(view, "Ctrl+H") {
					t.Fatalf("help exposes the wrong global binding:\n%s", view)
				}
			})
		}
	}
}

func TestManagerScreensUseUnifiedShell(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		m := New([]*model.Server{{Alias: "prod", Host: "prod.example", Port: 22, User: "ops"}})
		m.width, m.height = size.width, size.height
		template := &model.CommandTemplate{Name: "Disk usage", Command: "df -h", Description: "Show mounted filesystems"}
		m.setTemplates([]*model.CommandTemplate{template})
		m.setTags([]string{"production"})
		m.pendingTemplate = template
		m.bgResults = []templateRunResult{{Alias: "prod", Output: "ok\n数据库 ready"}}

		screens := []struct {
			name   string
			screen screen
		}{
			{"search", screenSearch},
			{"tags", screenTags},
			{"tag-input", screenTagInput},
			{"templates", screenTemplates},
			{"template-picker", screenTemplatePicker},
			{"template-mode", screenTemplateMode},
			{"background-results", screenBackgroundResults},
		}
		for _, tt := range screens {
			m.screen = tt.screen
			t.Run(tt.name+itoa(size.width), func(t *testing.T) {
				assertUnifiedScreen(t, m.View(), size.width, size.height)
			})
		}

		tunnelScreen := newTunnelScreenModel(size.width, size.height)
		assertUnifiedScreen(t, tunnelScreen.View(), size.width, size.height)
	}
}

func TestLayoutMatrixInventoriesEveryScreen(t *testing.T) {
	covered := map[screen]string{
		screenList:              "dashboard",
		screenForm:              "server form",
		screenSearch:            "manager matrix",
		screenTags:              "manager matrix",
		screenTagInput:          "manager matrix",
		screenTemplates:         "manager matrix",
		screenTemplateForm:      "template form",
		screenTemplatePicker:    "manager matrix",
		screenTemplateMode:      "manager matrix",
		screenBackgroundResults: "manager matrix",
		screenHelp:              "help matrix",
		screenActionMenu:        "action matrix",
		screenForwardList:       "forward matrix",
		screenForwardForm:       "forward form matrix",
		screenTunnelManager:     "manager matrix",
		screenConfirm:           "confirmation matrix",
		screenFullHelp:          "help matrix",
	}
	for value := screenList; value <= screenFullHelp; value++ {
		if _, ok := covered[value]; !ok {
			t.Fatalf("screen %d is missing from the layout matrix", value)
		}
	}
}

func TestShellBreakpointsUseTerminalWidth(t *testing.T) {
	for _, tt := range []struct {
		contentWidth int
		want         terminalSizeClass
	}{{68, sizeNarrow}, {69, sizeMedium}, {98, sizeMedium}, {99, sizeWide}} {
		if got := classifyShellContent(tt.contentWidth); got != tt.want {
			t.Fatalf("content width %d classified as %v, want %v", tt.contentWidth, got, tt.want)
		}
	}
}

func TestTunnelErrorUsesUnifiedShellRows(t *testing.T) {
	tunnelModel := newTunnelScreenModel(60, 16)
	tunnelModel.tunnels = []*model.TunnelState{{Name: "prod tunnel", ServerAlias: "prod", LastError: "connection lost\nretry failed"}}
	tunnelModel.rebuildList()
	view := tunnelModel.View()
	assertUnifiedScreen(t, view, 60, 16)
	if !strings.Contains(view, "connection lost") || !strings.Contains(view, "retry failed") {
		t.Fatalf("tunnel error was lost:\n%s", view)
	}
}

func TestTemplateViewportKeepsSelectedDescribedItemVisible(t *testing.T) {
	m := New(nil)
	m.width, m.height = 60, 16
	templates := make([]*model.CommandTemplate, 20)
	for index := range templates {
		templates[index] = &model.CommandTemplate{Name: fmt.Sprintf("template-%02d", index), Command: "echo ok", Description: "description"}
	}
	m.setTemplates(templates)
	m.templateList.Select(len(templates) - 1)
	m.screen = screenTemplates
	view := m.View()
	assertUnifiedScreen(t, view, 60, 16)
	if !strings.Contains(view, "> template-19") {
		t.Fatalf("selected template is outside viewport:\n%s", view)
	}
}

func TestTemplateFormFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		form := newTemplateFormModel(nil, size.width, size.height)
		form.inputs[0].SetValue("проверка-数据库")
		form.inputs[1].SetValue("printf 'a very long command that remains editable'")
		view := form.View()
		assertViewFits(t, view, size.width, size.height)
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"Template", "Name *", "Save", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("template form at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestServerFormDropdownUsesUnifiedShell(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		form := newFormModel(size.width, size.height)
		form.focusIdx = 5
		form.showAuthList = true
		view := form.View()
		assertUnifiedScreen(t, view, size.width, size.height)
		for _, want := range []string{"Select auth method", "password", "agent", "Enter", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("dropdown at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("view has %d lines, terminal height is %d:\n%s", len(lines), height, view)
	}
	for index, line := range lines {
		if lineWidth := ansi.StringWidth(line); lineWidth > width {
			t.Fatalf("line %d has width %d, terminal width is %d: %q", index+1, lineWidth, width, ansi.Strip(line))
		}
	}
}

func assertUnifiedScreen(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Fatalf("unified screen has %d lines, want %d:\n%s", len(lines), height, view)
	}
	if !strings.HasPrefix(ansi.Strip(lines[0]), "sshkeeper / ") {
		t.Fatalf("unified screen has no breadcrumb header: %q", ansi.Strip(lines[0]))
	}
	if !strings.Contains(ansi.Strip(view), "┌") || !strings.Contains(ansi.Strip(view), "┘") {
		t.Fatalf("unified screen has no framed content:\n%s", view)
	}
	if strings.TrimSpace(ansi.Strip(lines[height-1])) == "" {
		t.Fatalf("unified screen footer is not on last row:\n%s", view)
	}
	for index, line := range lines {
		if got := ansi.StringWidth(line); got > width-1 {
			t.Fatalf("line %d uses unsafe last terminal column: width=%d terminal=%d", index+1, got, width)
		}
	}
}

func assertRightMargin(t *testing.T, view string, width int) {
	t.Helper()
	for index, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > width-1 {
			t.Fatalf("line %d uses unsafe last terminal column: width=%d terminal=%d", index+1, got, width)
		}
	}
}

type errText string

func (e errText) Error() string { return string(e) }

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var out [20]byte
	index := len(out)
	for value > 0 {
		index--
		out[index] = digits[value%10]
		value /= 10
	}
	return string(out[index:])
}
