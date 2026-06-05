package cmd

import (
	"strings"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestTunnelCommandExposesBackgroundAndManagementCommands(t *testing.T) {
	if flag := tunnelCmd.Flags().Lookup("background"); flag == nil {
		t.Fatal("expected tunnel --background flag")
	}

	for _, name := range []string{"list", "stop", "stop-all"} {
		if cmd, _, err := tunnelCmd.Find([]string{name}); err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("expected tunnel subcommand %q, got cmd=%v err=%v", name, cmd, err)
		}
	}
}

func TestValidateBackgroundTunnelRejectsSecretAuth(t *testing.T) {
	server := &model.Server{Alias: "db", AuthMethod: model.AuthPassword}
	forwards := []*model.Forward{{Enabled: true, Type: model.ForwardLocal, LocalPort: 15432}}

	err := validateBackgroundTunnel(server, forwards)
	if err == nil {
		t.Fatal("expected background tunnel to reject password auth")
	}
	if !strings.Contains(err.Error(), "key or agent auth") {
		t.Fatalf("expected auth guidance, got %v", err)
	}
}

func TestValidateBackgroundTunnelRequiresEnabledForward(t *testing.T) {
	server := &model.Server{Alias: "db", AuthMethod: model.AuthKey}
	forwards := []*model.Forward{{Enabled: false, Type: model.ForwardLocal, LocalPort: 15432}}

	err := validateBackgroundTunnel(server, forwards)
	if err == nil {
		t.Fatal("expected background tunnel to require enabled forwards")
	}
	if !strings.Contains(err.Error(), "no enabled forwards") {
		t.Fatalf("expected enabled forward error, got %v", err)
	}
}
