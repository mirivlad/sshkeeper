package ssh

import (
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestBuildRouteArgs_Direct(t *testing.T) {
	route := model.Route{Hops: []model.RouteHop{}}
	args := BuildRouteArgs(route)
	if len(args) != 0 {
		t.Fatalf("expected no args for direct route, got %v", args)
	}
}

func TestBuildRouteArgs_Via(t *testing.T) {
	route := model.Route{Hops: []model.RouteHop{
		{Raw: "bastion.example.com", IsProfile: false},
	}}
	args := BuildRouteArgs(route)
	if len(args) != 2 || args[0] != "-J" || args[1] != "bastion.example.com" {
		t.Fatalf("expected [-J bastion.example.com], got %v", args)
	}
}

func TestBuildRouteArgs_Chain(t *testing.T) {
	route := model.Route{Hops: []model.RouteHop{
		{Alias: "bastion", IsProfile: true},
		{Raw: "dmz-gw.internal", IsProfile: false},
	}}
	args := BuildRouteArgs(route)
	if len(args) != 2 || args[0] != "-J" || args[1] != "bastion,dmz-gw.internal" {
		t.Fatalf("expected [-J bastion,dmz-gw.internal], got %v", args)
	}
}

func TestBuildRouteArgs_ProfileHop(t *testing.T) {
	route := model.Route{Hops: []model.RouteHop{
		{Alias: "my-bastion", IsProfile: true},
	}}
	args := BuildRouteArgs(route)
	if len(args) != 2 || args[1] != "my-bastion" {
		t.Fatalf("expected profile alias in -J, got %v", args)
	}
}

func TestRouteProxyJumpString(t *testing.T) {
	route := model.Route{Hops: []model.RouteHop{
		{Alias: "bastion", IsProfile: true},
		{Raw: "10.0.0.1", IsProfile: false},
	}}
	got := route.ProxyJumpString()
	if got != "bastion,10.0.0.1" {
		t.Fatalf("expected 'bastion,10.0.0.1', got %q", got)
	}
}

func TestRouteDisplaySummary(t *testing.T) {
	tests := []struct {
		route  model.Route
		target string
		want   string
	}{
		{model.Route{}, "target", "direct → target"},
		{model.Route{Hops: []model.RouteHop{{Alias: "bastion", IsProfile: true}}}, "target", "bastion → target"},
		{model.Route{Hops: []model.RouteHop{
			{Alias: "bastion", IsProfile: true},
			{Raw: "dmz-gw", IsProfile: false},
		}}, "target", "bastion → dmz-gw → target"},
	}
	for _, tt := range tests {
		got := tt.route.DisplaySummary(tt.target)
		if got != tt.want {
			t.Fatalf("DisplaySummary() = %q, want %q", got, tt.want)
		}
	}
}

func TestRouteMode(t *testing.T) {
	tests := []struct {
		hops int
		want string
	}{
		{0, "direct"},
		{1, "via"},
		{2, "chain"},
		{3, "chain"},
	}
	for _, tt := range tests {
		route := model.Route{Hops: make([]model.RouteHop, tt.hops)}
		got := route.RouteMode()
		if got != tt.want {
			t.Fatalf("RouteMode() = %q, want %q", got, tt.want)
		}
	}
}

func TestBuildSSHArgs_WithRoute(t *testing.T) {
	server := &model.Server{
		Host: "target.internal",
		Port: 22,
		User: "root",
		Route: model.Route{Hops: []model.RouteHop{
			{Alias: "bastion", IsProfile: true},
		}},
	}
	args := BuildSSHArgs(server)
	// Should contain -J bastion
	found := false
	for i, a := range args {
		if a == "-J" && i+1 < len(args) && args[i+1] == "bastion" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -J bastion in args, got %v", args)
	}
}

func TestBuildSSHArgs_FallbackToProxyJump(t *testing.T) {
	server := &model.Server{
		Host:      "target.internal",
		Port:      22,
		User:      "root",
		ProxyJump: "old-bastion",
	}
	args := BuildSSHArgs(server)
	found := false
	for i, a := range args {
		if a == "-J" && i+1 < len(args) && args[i+1] == "old-bastion" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected -J old-bastion in args, got %v", args)
	}
}
