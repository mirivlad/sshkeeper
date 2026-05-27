package db

import (
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestUpdateServerByAliasCanRenameAlias(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	server := &model.Server{
		Alias:      "old",
		Host:       "old.example",
		Port:       22,
		User:       "root",
		AuthMethod: model.AuthPassword,
	}
	if err := db.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}

	server.Alias = "new"
	server.Host = "new.example"
	server.AuthMethod = model.AuthKey
	if err := db.UpdateServerByAlias("old", server); err != nil {
		t.Fatalf("update server by old alias: %v", err)
	}

	if _, err := db.GetServer("old"); err == nil {
		t.Fatal("expected old alias to be gone")
	}
	got, err := db.GetServer("new")
	if err != nil {
		t.Fatalf("get new alias: %v", err)
	}
	if got.ID != server.ID {
		t.Fatalf("expected ID to stay %d, got %d", server.ID, got.ID)
	}
	if got.Host != "new.example" || got.AuthMethod != model.AuthKey {
		t.Fatalf("unexpected updated server: %#v", got)
	}
}
