package cmd

import (
	"strconv"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// restoreFlags returns every flag touched during the test back to its default.
// The cobra commands are package-level singletons, so parsing argv into one
// leaks state into whatever test runs next.
func restoreFlags(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	t.Cleanup(func() {
		cmd.Flags().Visit(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	})
}

func TestForwardEditUpdatesEnabledFlag(t *testing.T) {
	testDB, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer testDB.Close()

	previousDB := appDB
	appDB = testDB
	t.Cleanup(func() { appDB = previousDB })

	server := &model.Server{Alias: "web", Host: "web.example.org", Port: 22, User: "root", AuthMethod: model.AuthKey}
	if err := appDB.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}
	forwardID, err := appDB.AddForward(&model.Forward{
		ServerID:  server.ID,
		Name:      "SOCKS",
		Type:      model.ForwardDynamic,
		LocalAddr: "127.0.0.1",
		LocalPort: 1080,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("add forward: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().Bool("enabled", true, "Enable/disable forward")
	if err := cmd.Flags().Set("enabled", "false"); err != nil {
		t.Fatalf("set enabled flag: %v", err)
	}

	if err := forwardEditCmd.RunE(cmd, []string{strconv.FormatInt(forwardID, 10)}); err != nil {
		t.Fatalf("edit forward: %v", err)
	}

	forwards, err := appDB.GetForwards(server.ID)
	if err != nil {
		t.Fatalf("get forwards: %v", err)
	}
	if len(forwards) != 1 {
		t.Fatalf("expected one forward, got %d", len(forwards))
	}
	if forwards[0].Enabled {
		t.Fatal("expected forward to be disabled")
	}
}

func TestForwardAddStoresNameAndDescription(t *testing.T) {
	testDB, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer testDB.Close()

	previousDB := appDB
	appDB = testDB
	t.Cleanup(func() { appDB = previousDB })

	server := &model.Server{Alias: "web", Host: "web.example.org", Port: 22, User: "root", AuthMethod: model.AuthKey}
	if err := appDB.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("type", "local", "")
	cmd.Flags().String("local-addr", "127.0.0.1", "")
	cmd.Flags().Int("local-port", 0, "")
	cmd.Flags().String("remote-addr", "", "")
	cmd.Flags().Int("remote-port", 0, "")
	for flag, value := range map[string]string{
		"name":        "Local PostgreSQL",
		"description": "DB access",
		"type":        "local",
		"local-port":  "15432",
		"remote-addr": "127.0.0.1",
		"remote-port": "5432",
	} {
		if err := cmd.Flags().Set(flag, value); err != nil {
			t.Fatalf("set %s: %v", flag, err)
		}
	}

	if err := forwardAddCmd.RunE(cmd, []string{"web"}); err != nil {
		t.Fatalf("add forward: %v", err)
	}

	forwards, err := appDB.GetForwards(server.ID)
	if err != nil {
		t.Fatalf("get forwards: %v", err)
	}
	if len(forwards) != 1 {
		t.Fatalf("expected one forward, got %d", len(forwards))
	}
	if forwards[0].Name != "Local PostgreSQL" || forwards[0].Description != "DB access" {
		t.Fatalf("unexpected forward metadata: %#v", forwards[0])
	}
}

// TestForwardAddParsesItsOwnFlags drives the real forwardAddCmd flag set the
// way the CLI does, instead of handing RunE a command built by the test.
//
// Regression: RunE read --local-port and init() marked it required, but the
// flag was never registered on forwardAddCmd. Cobra silently ignores
// MarkFlagRequired for an unknown flag and GetInt returns 0 for one, so every
// real invocation died on "invalid local port 0" while the sibling tests --
// which registered the flag on a throwaway command themselves -- kept passing.
func TestForwardAddParsesItsOwnFlags(t *testing.T) {
	testDB, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer testDB.Close()

	previousDB := appDB
	appDB = testDB
	t.Cleanup(func() { appDB = previousDB })

	server := &model.Server{Alias: "web", Host: "web.example.org", Port: 22, User: "root", AuthMethod: model.AuthKey}
	if err := appDB.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}

	restoreFlags(t, forwardAddCmd)
	if err := forwardAddCmd.Flags().Parse([]string{
		"--name", "Local PostgreSQL",
		"--type", "local",
		"--local-port", "15432",
		"--remote-addr", "db01.internal.example.com",
		"--remote-port", "5432",
	}); err != nil {
		t.Fatalf("parse forward add flags: %v", err)
	}

	if err := forwardAddCmd.RunE(forwardAddCmd, []string{"web"}); err != nil {
		t.Fatalf("add forward: %v", err)
	}

	forwards, err := appDB.GetForwards(server.ID)
	if err != nil {
		t.Fatalf("get forwards: %v", err)
	}
	if len(forwards) != 1 {
		t.Fatalf("expected one forward, got %d", len(forwards))
	}
	got := forwards[0]
	if got.LocalPort != 15432 {
		t.Fatalf("local port not carried through: got %d, want 15432", got.LocalPort)
	}
	if got.Type != model.ForwardLocal || got.RemoteAddr != "db01.internal.example.com" || got.RemotePort != 5432 {
		t.Fatalf("unexpected forward: %#v", got)
	}
}

// TestForwardAddRequiresLocalPort pins the flag's registration and its required
// annotation, which is what MarkFlagRequired silently failed to attach.
func TestForwardAddRequiresLocalPort(t *testing.T) {
	flag := forwardAddCmd.Flags().Lookup("local-port")
	if flag == nil {
		t.Fatal("forward add does not register --local-port")
	}
	if _, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Fatal("--local-port is registered but not marked required")
	}
}
