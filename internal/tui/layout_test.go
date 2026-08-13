package tui

import (
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
		assertViewFits(t, fm.View(), size.width, size.height)
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
		for _, want := range []string{"Actions", "Connect", "Manage port forwards", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("action menu at %dx%d missing %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}

func TestTemplateFormFitsSupportedTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		form := newTemplateFormModel(nil, size.width, size.height)
		form.inputs[0].SetValue("проверка-数据库")
		form.inputs[1].SetValue("printf 'a very long command that remains editable'")
		view := form.View()
		assertViewFits(t, view, size.width, size.height)
		for _, want := range []string{"Template", "Name *", "Save", "Esc"} {
			if !strings.Contains(view, want) {
				t.Fatalf("template form at %dx%d missing %q:\n%s", size.width, size.height, want, view)
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
