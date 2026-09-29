package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestASCIIModeDrawsOnlyASCIIOnTheDashboard(t *testing.T) {
	UseASCII(true)
	t.Cleanup(func() { UseASCII(false) })

	for _, servers := range [][]*model.Server{
		nil,
		{
			{ID: 1, Alias: "db", Host: "db01", Port: 22, User: "postgres", GroupName: "Prod", LastTestStatus: model.TestFailed,
				Route: model.Route{Hops: []model.RouteHop{{ServerID: 2, Alias: "bastion", IsProfile: true}}}},
			{ID: 2, Alias: "bastion", Host: "bastion", Port: 22, User: "jump", LastTestStatus: model.TestOK},
		},
	} {
		m := New(servers)
		m.width, m.height = 120, 30
		m.selected["db"] = true
		m.runtime = runtimeStatus{tunnels: map[string]int{"db": 1}, sessions: map[string]int{"db": 1}}
		view := ansi.Strip(m.View())
		for _, r := range view {
			if r > 127 {
				t.Fatalf("non-ASCII rune %q in ASCII mode:\n%s", r, view)
			}
		}
		if len(servers) > 0 && !strings.Contains(view, "x = #") {
			t.Fatalf("state column should use ASCII symbols:\n%s", view)
		}
	}
}
