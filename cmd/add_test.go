package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestPromptServerForAddCollectsInteractiveFields(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		"prod",
		"Production",
		"prod.example.org",
		"2222",
		"deploy",
		"key",
		"~/.ssh/id_prod",
		"bastion",
		"prod",
		"critical host",
		"",
	}, "\n"))
	var output bytes.Buffer

	server, err := promptServerForAdd(input, &output)
	if err != nil {
		t.Fatalf("prompt server: %v", err)
	}

	if server.Alias != "prod" ||
		server.DisplayName != "Production" ||
		server.Host != "prod.example.org" ||
		server.Port != 2222 ||
		server.User != "deploy" ||
		server.AuthMethod != model.AuthKey ||
		server.IdentityFile != "~/.ssh/id_prod" ||
		server.ProxyJump != "bastion" ||
		server.GroupName != "prod" ||
		server.Notes != "critical host" {
		t.Fatalf("unexpected server: %#v", server)
	}
	if strings.Contains(output.String(), "not yet implemented") {
		t.Fatalf("interactive add should not report unimplemented:\n%s", output.String())
	}
}

func TestPromptServerForAddAppliesDefaults(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		"prod",
		"",
		"prod.example.org",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	}, "\n"))
	var output bytes.Buffer

	server, err := promptServerForAdd(input, &output)
	if err != nil {
		t.Fatalf("prompt server: %v", err)
	}

	if server.DisplayName != "prod" {
		t.Fatalf("display name default = %q; want alias", server.DisplayName)
	}
	if server.Port != 22 {
		t.Fatalf("port default = %d; want 22", server.Port)
	}
	if server.User != "root" {
		t.Fatalf("user default = %q; want root", server.User)
	}
	if server.AuthMethod != model.AuthKey {
		t.Fatalf("auth default = %q; want key", server.AuthMethod)
	}
}
