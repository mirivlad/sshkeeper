package syncer

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestPasswordAccessEnvelope(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := SealAccess(key, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte(testPassword)) || bytes.Contains(blob, key) {
		t.Fatal("remote access file contains cleartext key or password")
	}
	actual, err := OpenAccess(blob, testPassword)
	if err != nil || !bytes.Equal(actual, key) {
		t.Fatalf("recover: %v", err)
	}
	if _, err := OpenAccess(blob, "incorrect"); !errors.Is(err, ErrAccessPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := OpenAccess([]byte("{}"), testPassword); err == nil {
		t.Fatal("corrupt access file accepted")
	}
}

func TestPasswordOnlyRestoreRecoversProfilesSecretsAndSSHKeys(t *testing.T) {
	folder := &Folder{Dir: t.TempDir()}
	a := newDevice(t, folder)
	seedDeviceA(t, a)

	_, _, err := a.engine.CreatePassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	firstBundle, err := os.ReadFile(filepath.Join(folder.Dir, BundleFile))
	if err != nil {
		t.Fatal(err)
	}
	access, err := os.ReadFile(filepath.Join(folder.Dir, AccessFile))
	if err != nil || len(access) == 0 {
		t.Fatalf("missing access file: %v", err)
	}
	if bytes.Contains(firstBundle, []byte("web01.internal")) || bytes.Contains(access, []byte("web01.internal")) {
		t.Fatal("remote storage exposes server names")
	}

	b := newDevice(t, folder)
	if _, err := b.engine.JoinPassword("incorrect"); !errors.Is(err, ErrAccessPassword) {
		t.Fatalf("wrong password must fail: %v", err)
	}
	if b.engine.HasKey() {
		t.Fatal("failed restore changed the new vault")
	}
	if _, err := b.db.GetServer("web"); err == nil {
		t.Fatal("failed restore imported a server")
	}

	report, err := b.engine.JoinPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if report.Received == 0 {
		t.Fatal("no records restored")
	}
	srv, err := b.db.GetServer("web")
	if err != nil {
		t.Fatalf("server not restored: %v", err)
	}
	if srv.Host != "web01.internal" || srv.User != "ops" {
		t.Fatalf("bad restored server: %+v", srv)
	}
	wantedKeyPath := filepath.Join(b.home, ".ssh", "id_web")
	if srv.IdentityFile != wantedKeyPath {
		t.Fatalf("non-portable path: %q", srv.IdentityFile)
	}
	priv, err := os.ReadFile(wantedKeyPath)
	if err != nil || !bytes.Contains(priv, []byte("web")) {
		t.Fatalf("SSH private key not restored: %v", err)
	}
	passphrase, err := b.vault.Get(stableSecretID(srv.ID, "key_passphrase"))
	if err != nil || string(passphrase) != "s3cret" {
		t.Fatalf("passphrase not restored: %v", err)
	}
	// Updating the bundle must leave password recovery readable.
	if _, err := a.engine.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAccess(access, testPassword); err != nil {
		t.Fatal(err)
	}
}

func TestLegacySyncCanEnablePasswordRestoreWithoutRecreatingSpace(t *testing.T) {
	folder := &Folder{Dir: t.TempDir()}
	a := newDevice(t, folder)
	if err := a.db.CreateServer(&model.Server{Alias: "legacy", Host: "old.example", Port: 22, User: "sysop", AuthMethod: model.AuthAgent}); err != nil {
		t.Fatal(err)
	}
	recovery, _, err := a.engine.Create() // Old version setup.
	if err != nil {
		t.Fatal(err)
	}
	oldBundle, err := os.ReadFile(filepath.Join(folder.Dir, BundleFile))
	if err != nil {
		t.Fatal(err)
	}
	b := newDevice(t, folder)
	if _, err := b.engine.JoinPassword(testPassword); !errors.Is(err, ErrAccessMissing) {
		t.Fatalf("old space should require migration: %v", err)
	}
	if err := a.engine.EnablePassword(testPassword); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(folder.Dir, BundleFile))
	if err != nil || !bytes.Equal(oldBundle, after) {
		t.Fatal("migration modified encrypted bundle")
	}
	if _, err := b.engine.JoinPassword(testPassword); err != nil {
		t.Fatalf("password migration: %v", err)
	}
	key, err := b.engine.RecoveryKey()
	if err != nil || key != recovery {
		t.Fatalf("random key was replaced: %v", err)
	}
}

func TestPasswordRecoveryCannotJoinMissingOrWrongSpace(t *testing.T) {
	folder := &Folder{Dir: t.TempDir()}
	a := newDevice(t, folder)
	if _, _, err := a.engine.CreatePassword(testPassword); err != nil {
		t.Fatal(err)
	}
	b := newDevice(t, folder)
	if err := os.Remove(filepath.Join(folder.Dir, BundleFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.engine.JoinPassword(testPassword); err == nil {
		t.Fatal("joined without bundle")
	}
	if b.engine.HasKey() {
		t.Fatal("unexpected key after empty restore")
	}
	if _, _, err := b.engine.CreatePassword(testPassword); !errors.Is(err, ErrSpaceExists) {
		t.Fatalf("orphan access file must prevent overwrite: %v", err)
	}
}

func TestGitAccessBranchSurvivesBundleUpdates(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "bare.git")
	if out, err := exec.Command("git", "init", "--bare", "--quiet", remote).CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v (%s)", err, out)
	}
	first := &Git{URL: remote, Cache: filepath.Join(root, "cache-first")}
	a := newDevice(t, first)
	seedDeviceA(t, a)
	if _, _, err := a.engine.CreatePassword(testPassword); err != nil {
		t.Fatal(err)
	}
	accessBefore, err := first.ReadAccess()
	if err != nil || len(accessBefore) == 0 {
		t.Fatalf("git access branch: %v", err)
	}
	second := &Git{URL: remote, Cache: filepath.Join(root, "cache-second")}
	b := newDevice(t, second)
	if _, err := b.engine.JoinPassword(testPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.GetServer("web"); err != nil {
		t.Fatalf("git did not restore profile: %v", err)
	}
	if _, err := a.engine.Sync(); err != nil {
		t.Fatal(err)
	}
	accessAfter, err := second.ReadAccess()
	if err != nil || !bytes.Equal(accessBefore, accessAfter) {
		t.Fatal("the access branch was replaced by a bundle sync")
	}
}
