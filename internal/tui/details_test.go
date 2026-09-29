package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestDetailsPanelShowsRouteForwardsAndNotes(t *testing.T) {
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	withNow(t, base)
	connected := base.Add(-2 * time.Hour)
	server := &model.Server{
		ID: 7, Alias: "db", DisplayName: "DB Master", Host: "db01.internal", Port: 22, User: "postgres",
		AuthMethod: model.AuthKey, GroupName: "Production", Notes: "Primary PostgreSQL",
		LastConnectedAt: &connected, LastTestStatus: model.TestFailed, LastTestError: "timeout\nmore",
		Route: model.Route{Hops: []model.RouteHop{{ServerID: 1, Alias: "bastion", IsProfile: true}, {Raw: "gw.example"}}},
	}
	m := New([]*model.Server{server})
	m.setForwardIndex([]*model.Forward{
		{ServerID: 7, Name: "pg", Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432, Enabled: true},
		{ServerID: 7, Type: model.ForwardDynamic, LocalAddr: "0.0.0.0", LocalPort: 1080},
	})
	m.runtime = runtimeStatus{tunnels: map[string]int{"db": 1}, sessions: map[string]int{"db": 1}}

	panel := ansi.Strip(m.renderSelectedPanel(60, 24))
	for _, want := range []string{
		"DB Master", "key",
		"postgres@db01.internal:22",
		"you → bastion → gw.example → db01.internal",
		"Last in   2h ago",
		"✗ failed: timeout",
		"1 open in tmux",
		"⇄ tunnel running",
		"● L :15432 → 127.0.0.1:5432  pg",
		"○ D 0.0.0.0:1080 SOCKS",
		"Primary PostgreSQL",
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("details missing %q:\n%s", want, panel)
		}
	}
	if strings.Contains(panel, "more") {
		t.Error("only the first line of a test error belongs in the panel")
	}
}

func TestDetailsPanelSummarizesGroupHeader(t *testing.T) {
	m := groupTestModel()
	m.serverByAlias("web").LastTestStatus = model.TestOK
	m.serverByAlias("db").LastTestStatus = model.TestFailed
	m.cursor = m.headerRow("Production")
	panel := ansi.Strip(m.renderSelectedPanel(50, 16))
	for _, want := range []string{"Production", "2 profiles", "Test OK   ● 1", "Failed    ✗ 1", "Enter fold"} {
		if !strings.Contains(panel, want) {
			t.Errorf("group details missing %q:\n%s", want, panel)
		}
	}
}

func TestForwardLineFormatsRemoteRule(t *testing.T) {
	line := ansi.Strip(forwardLine(&model.Forward{Type: model.ForwardRemote, RemoteAddr: "0.0.0.0", RemotePort: 8080, LocalAddr: "127.0.0.1", LocalPort: 3000, Enabled: true}))
	if line != "● R 0.0.0.0:8080 → 127.0.0.1:3000" {
		t.Fatalf("remote forward line = %q", line)
	}
}
