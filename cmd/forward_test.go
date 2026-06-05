package cmd

import (
	"strconv"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

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
