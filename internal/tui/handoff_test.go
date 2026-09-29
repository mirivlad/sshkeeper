package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestHandoffLineNamesTargetAndRoute(t *testing.T) {
	server := &model.Server{Alias: "web", DisplayName: "Production web", Host: "web01", Port: 22, User: "ops",
		Route: model.Route{Hops: []model.RouteHop{{ServerID: 1, Alias: "bastion", IsProfile: true}}}}
	if got := ansi.Strip(HandoffLine(server)); got != "○━┳┳ → Production web · ops@web01:22 via bastion" {
		t.Fatalf("handoff line = %q", got)
	}
}

func TestReturnAndErrorNotices(t *testing.T) {
	server := &model.Server{Alias: "web"}
	cases := map[time.Duration]string{
		12 * time.Second:                "← Back from web · 12s",
		7*time.Minute + 30*time.Second:  "← Back from web · 7m",
		65*time.Minute + 59*time.Second: "← Back from web · 1h 05m",
	}
	for elapsed, want := range cases {
		if got := ReturnNotice(server, elapsed); got != want {
			t.Errorf("ReturnNotice(%v) = %q, want %q", elapsed, got, want)
		}
	}
	if got := ErrorNotice(server, errors.New("exit status 255")); !strings.Contains(got, "Connection to web failed: exit status 255") {
		t.Errorf("ErrorNotice = %q", got)
	}
}
