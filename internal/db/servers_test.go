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

func TestServerPersistsStartupCommandAndTags(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	server := &model.Server{
		Alias:          "prod",
		DisplayName:    "Production",
		Host:           "prod.example",
		Port:           2222,
		User:           "deploy",
		AuthMethod:     model.AuthKey,
		StartupCommand: "tmux attach -t ops",
		Tags:           []string{"prod", "db"},
	}
	if err := db.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}
	if err := db.SetServerTags(server.ID, server.Tags); err != nil {
		t.Fatalf("set tags: %v", err)
	}

	got, err := db.GetServer("prod")
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.StartupCommand != "tmux attach -t ops" {
		t.Fatalf("startup command = %q", got.StartupCommand)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "db" || got.Tags[1] != "prod" {
		t.Fatalf("unexpected tags: %#v", got.Tags)
	}

	got.StartupCommand = "uptime"
	got.Tags = []string{"web"}
	if err := db.UpdateServerByAlias("prod", got); err != nil {
		t.Fatalf("update server: %v", err)
	}
	if err := db.SetServerTags(got.ID, got.Tags); err != nil {
		t.Fatalf("replace tags: %v", err)
	}

	reopened, err := db.GetServer("prod")
	if err != nil {
		t.Fatalf("get updated server: %v", err)
	}
	if reopened.StartupCommand != "uptime" {
		t.Fatalf("updated startup command = %q", reopened.StartupCommand)
	}
	if len(reopened.Tags) != 1 || reopened.Tags[0] != "web" {
		t.Fatalf("updated tags: %#v", reopened.Tags)
	}
}

func TestGlobalCommandTemplateCRUD(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	tpl := &model.CommandTemplate{Name: "uptime", Command: "uptime"}
	if err := db.CreateCommandTemplate(tpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if tpl.ID == 0 {
		t.Fatal("expected template ID")
	}

	got, err := db.GetCommandTemplate("uptime")
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Command != "uptime" {
		t.Fatalf("template command = %q", got.Command)
	}

	got.Name = "load"
	got.Command = "cat /proc/loadavg"
	if err := db.UpdateCommandTemplate("uptime", got); err != nil {
		t.Fatalf("update template: %v", err)
	}
	if _, err := db.GetCommandTemplate("uptime"); err == nil {
		t.Fatal("expected old template name to be gone")
	}
	updated, err := db.GetCommandTemplate("load")
	if err != nil {
		t.Fatalf("get renamed template: %v", err)
	}
	if updated.Command != "cat /proc/loadavg" {
		t.Fatalf("updated command = %q", updated.Command)
	}

	templates, err := db.ListCommandTemplates()
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "load" {
		t.Fatalf("unexpected templates: %#v", templates)
	}

	if err := db.DeleteCommandTemplate("load"); err != nil {
		t.Fatalf("delete template: %v", err)
	}
	if templates, err := db.ListCommandTemplates(); err != nil || len(templates) != 0 {
		t.Fatalf("expected no templates, got %#v err %v", templates, err)
	}
}

func TestLegacyCommandTemplatesAreCopiedToGlobalTemplates(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	server := &model.Server{Alias: "prod", Host: "prod.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	if err := db.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}
	if _, err := db.conn.Exec(
		"INSERT INTO command_templates (server_id, name, command) VALUES (?, ?, ?)",
		server.ID, "legacy-uptime", "uptime",
	); err != nil {
		t.Fatalf("insert legacy template: %v", err)
	}
	if err := db.ensureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	got, err := db.GetCommandTemplate("legacy-uptime")
	if err != nil {
		t.Fatalf("get copied template: %v", err)
	}
	if got.Command != "uptime" {
		t.Fatalf("copied command = %q", got.Command)
	}
}

func TestTagManagementCRUD(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	server := &model.Server{Alias: "prod", Host: "prod.example", Port: 22, User: "root", AuthMethod: model.AuthKey}
	if err := db.CreateServer(server); err != nil {
		t.Fatalf("create server: %v", err)
	}
	if err := db.SetServerTags(server.ID, []string{"prod", "web"}); err != nil {
		t.Fatalf("set tags: %v", err)
	}

	tags, err := db.ListTags()
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "prod" || tags[1] != "web" {
		t.Fatalf("unexpected tags: %#v", tags)
	}

	if err := db.RenameTag("web", "frontend"); err != nil {
		t.Fatalf("rename tag: %v", err)
	}
	got, err := db.GetServer("prod")
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "frontend" || got.Tags[1] != "prod" {
		t.Fatalf("renamed tags: %#v", got.Tags)
	}

	if err := db.DeleteTag("prod"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	got, err = db.GetServer("prod")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "frontend" {
		t.Fatalf("tags after delete: %#v", got.Tags)
	}
}
