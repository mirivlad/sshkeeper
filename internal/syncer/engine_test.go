package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/vault"
)

const testPassword = "correct horse battery staple"

type device struct {
	engine *Engine
	db     *db.DB
	vault  *vault.Vault
	home   string
}

var clock = time.Unix(1_800_000_000, 0)

func tick() time.Time {
	clock = clock.Add(time.Second)
	return clock
}

func newDevice(t *testing.T, transport Transport) *device {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	vaultPath := filepath.Join(dir, "vault.bin")
	if err := vault.Create(vaultPath, testPassword); err != nil {
		t.Fatal(err)
	}
	v := vault.New(vaultPath)
	if err := v.Unlock(testPassword); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(dir, "home")
	return &device{
		engine: &Engine{DB: database, Vault: v, Transport: transport, Home: home, Now: tick},
		db:     database, vault: v, home: home,
	}
}

func mustSync(t *testing.T, d *device) Report {
	t.Helper()
	report, err := d.engine.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return report
}

func seedDeviceA(t *testing.T, a *device) {
	t.Helper()
	bastion := &model.Server{Alias: "bastion", Host: "bastion.example", Port: 22, User: "jump", AuthMethod: model.AuthAgent, GroupName: "Edge"}
	if err := a.db.CreateServer(bastion); err != nil {
		t.Fatal(err)
	}
	web := &model.Server{
		Alias: "web", DisplayName: "Production web", Host: "web01.internal", Port: 2222, User: "ops",
		AuthMethod: model.AuthKeyPassphrase, IdentityFile: "~/.ssh/id_web", GroupName: "Production", Notes: "primary",
		Route: model.Route{Hops: []model.RouteHop{{ServerID: bastion.ID, IsProfile: true}, {Raw: "gw.example"}}},
	}
	if err := a.db.CreateServer(web); err != nil {
		t.Fatal(err)
	}
	if err := a.db.SetServerTags(web.ID, []string{"prod", "web"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.AddForward(&model.Forward{ServerID: web.ID, Name: "pg", Type: model.ForwardLocal, LocalAddr: "127.0.0.1", LocalPort: 15432, RemoteAddr: "127.0.0.1", RemotePort: 5432, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.db.CreateCommandTemplate(&model.CommandTemplate{Name: "uptime", Command: "uptime"}); err != nil {
		t.Fatal(err)
	}
	if err := a.db.CreateGroup("Empty group"); err != nil {
		t.Fatal(err)
	}
	if err := a.vault.Put(stableSecretID(web.ID, "key_passphrase"), "key_passphrase", []byte("s3cret")); err != nil {
		t.Fatal(err)
	}
	if err := a.vault.Save(); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(a.home, ".ssh", "id_web")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nweb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 AAAA web"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncMakesAbsoluteIdentityPathPortableAcrossHomes(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)

	keyPath := filepath.Join(a.home, ".ssh", "id_cross_os")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	private := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ncross-os\n")
	if err := os.WriteFile(keyPath, private, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 AAAA cross-os"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := &model.Server{
		Alias: "cross-os", Host: "cross-os.example", Port: 22, User: "ops",
		AuthMethod: model.AuthKey, IdentityFile: keyPath,
	}
	if err := a.db.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	recovery, _, err := a.engine.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.engine.Join(recovery, ""); err != nil {
		t.Fatal(err)
	}

	got, err := b.db.GetServer("cross-os")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(b.home, ".ssh", "id_cross_os")
	if got.IdentityFile != wantPath {
		t.Fatalf("received identity path = %q, want %q", got.IdentityFile, wantPath)
	}
	if gotKey, err := os.ReadFile(wantPath); err != nil || string(gotKey) != string(private) {
		t.Fatalf("received private key = %q, %v", gotKey, err)
	}

	// The next export must stay portable rather than leaking B's local home
	// back into the shared bundle.
	local := &Local{DB: b.db, Vault: b.vault, Home: b.home}
	records, warnings, err := local.Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("export warnings: %v", warnings)
	}
	var serverPath, keyPathInBundle string
	for _, record := range records {
		switch record.Kind {
		case KindServer:
			var data ServerData
			if err := decode(record, &data); err == nil && data.Alias == "cross-os" {
				serverPath = data.IdentityFile
			}
		case KindKey:
			var data KeyData
			if err := decode(record, &data); err == nil {
				keyPathInBundle = data.Path
			}
		}
	}
	if serverPath != "~/.ssh/id_cross_os" || keyPathInBundle != "~/.ssh/id_cross_os" {
		t.Fatalf("portable paths: server=%q key=%q", serverPath, keyPathInBundle)
	}
}

func TestUnreadableLegacyKeyProtectsOldAndPortableRecordIDs(t *testing.T) {
	d := newDevice(t, &Folder{Dir: t.TempDir()})
	legacyPath := "/home/alice/.ssh/id_missing_from_legacy"
	server := &model.Server{
		Alias: "legacy-key", Host: "legacy.example", Port: 22, User: "ops",
		AuthMethod: model.AuthKey, IdentityFile: legacyPath,
	}
	if err := d.db.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	local := &Local{DB: d.db, Vault: d.vault, Home: d.home}
	if _, _, err := local.Export(); err != nil {
		t.Fatal(err)
	}
	legacyID := KindKey + ":" + legacyPath
	portableID := KindKey + ":~/.ssh/id_missing_from_legacy"
	if !local.unreadable[legacyID] || !local.unreadable[portableID] {
		t.Fatalf("unreadable IDs = %#v; want legacy %q and portable %q", local.unreadable, legacyID, portableID)
	}

	remoteCopy := Record{ID: legacyID, Kind: KindKey, Hash: "remote", Updated: 10}
	stamped := []Record{{ID: legacyID, Kind: KindKey, Deleted: true, Updated: 20}}
	kept := keepUnreadable(stamped, local.unreadable, map[string]Record{legacyID: remoteCopy})
	if len(kept) != 1 || kept[0].Deleted || kept[0].Hash != remoteCopy.Hash {
		t.Fatalf("legacy unreadable key was not preserved: %#v", kept)
	}
}

func TestPortableKeyPathUnderstandsForeignWindowsAndLinuxHomes(t *testing.T) {
	local := &Local{Home: filepath.Join(t.TempDir(), "home")}
	cases := map[string]string{
		"/home/alice/.ssh/id_ed25519":               "~/.ssh/id_ed25519",
		"C:\\Users\\Alice\\.ssh\\id_ed25519":        "~/.ssh/id_ed25519",
		"C:\\Users\\Alice\\.ssh\\company\\prod_key": "~/.ssh/company/prod_key",
		"~/.ssh/id_rsa":                             "~/.ssh/id_rsa",
	}
	for input, want := range cases {
		if got := local.portableKeyPath(input); got != want {
			t.Errorf("portableKeyPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTwoDevicesPairAndStayInStep(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)
	seedDeviceA(t, a)

	recovery, _, err := a.engine.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if strings.Count(recovery, "-") != 12 {
		t.Fatalf("recovery key = %q", recovery)
	}
	code, expires, err := a.engine.Pair(testPassword)
	if err != nil || len(code) != 6 || !expires.After(clock) {
		t.Fatalf("pair = %q %v %v", code, expires, err)
	}
	if _, err := b.engine.Join(code, "wrong password"); !errors.Is(err, ErrPairingInvalid) {
		t.Fatalf("join with wrong password = %v", err)
	}
	report, err := b.engine.Join(code, testPassword)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if report.Received == 0 {
		t.Fatal("join should bring A's data")
	}
	if blob, _ := storage.ReadPairing(); blob != nil {
		t.Fatal("the pairing code must be removed after use")
	}

	// Everything arrived on B.
	web, err := b.db.GetServer("web")
	if err != nil {
		t.Fatalf("web missing on B: %v", err)
	}
	if web.DisplayName != "Production web" || web.Port != 2222 || web.GroupName != "Production" || strings.Join(web.Tags, ",") != "prod,web" {
		t.Fatalf("web on B = %+v", web)
	}
	if len(web.Route.Hops) != 2 || web.Route.Hops[0].Alias != "bastion" || !web.Route.Hops[0].Profile() || web.Route.Hops[1].Raw != "gw.example" {
		t.Fatalf("route on B = %+v", web.Route)
	}
	secret, err := b.vault.Get(stableSecretID(web.ID, "key_passphrase"))
	if err != nil || string(secret) != "s3cret" {
		t.Fatalf("secret on B = %q, %v", secret, err)
	}
	forwards, _ := b.db.GetForwards(web.ID)
	if len(forwards) != 1 || forwards[0].LocalPort != 15432 || !forwards[0].Enabled {
		t.Fatalf("forwards on B = %+v", forwards)
	}
	if _, err := b.db.GetCommandTemplate("uptime"); err != nil {
		t.Fatal("template missing on B")
	}
	if groups, _ := b.db.GetGroups(); !strings.Contains(strings.Join(groups, ","), "Empty group") {
		t.Fatalf("empty group missing on B: %v", groups)
	}
	keyPath := filepath.Join(b.home, ".ssh", "id_web")
	info, err := os.Stat(keyPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key on B: %v %v", info, err)
	}
	if public, _ := os.ReadFile(keyPath + ".pub"); string(public) != "ssh-ed25519 AAAA web" {
		t.Fatal("public key not written")
	}

	// An edit on B reaches A.
	web.Notes = "edited on B"
	if err := b.db.UpdateServerByAlias("web", web); err != nil {
		t.Fatal(err)
	}
	mustSync(t, b)
	mustSync(t, a)
	onA, _ := a.db.GetServer("web")
	if onA.Notes != "edited on B" {
		t.Fatalf("edit did not reach A: %q", onA.Notes)
	}

	// A deletion on A reaches B, and edits to different profiles merge.
	aForwards, _ := a.db.GetForwards(onA.ID)
	if err := a.db.DeleteForward(aForwards[0].ID); err != nil {
		t.Fatal(err)
	}
	bastionB, _ := b.db.GetServer("bastion")
	bastionB.User = "admin"
	if err := b.db.UpdateServerByAlias("bastion", bastionB); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a)
	mustSync(t, b)
	mustSync(t, a)
	if forwards, _ := b.db.GetForwards(web.ID); len(forwards) != 0 {
		t.Fatal("deleted forward survived on B")
	}
	if bastionA, _ := a.db.GetServer("bastion"); bastionA.User != "admin" {
		t.Fatal("B's edit did not reach A")
	}

	// A quiet second sync writes nothing.
	if report := mustSync(t, a); report.Stored || report.Received != 0 {
		t.Fatalf("idle sync did work: %+v", report)
	}
}

func TestOfflinePairingJoinsWithoutOldMasterPassword(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)
	seedDeviceA(t, a)

	if _, _, err := a.engine.Create(); err != nil {
		t.Fatal(err)
	}
	code, expires, err := a.engine.PairOffline()
	if err != nil {
		t.Fatal(err)
	}
	if !LooksLikeOfflinePairingCode(code) || expires.Sub(clock) < 23*time.Hour {
		t.Fatalf("offline pair = %q, expires %v", code, expires)
	}
	report, err := b.engine.Join(code, "definitely not the old master password")
	if err != nil {
		t.Fatalf("offline join: %v", err)
	}
	if report.Received == 0 {
		t.Fatal("offline join should bring the synced data")
	}
	if _, err := b.db.GetServer("web"); err != nil {
		t.Fatal("web missing after offline join")
	}
	if blob, _ := storage.ReadPairing(); blob != nil {
		t.Fatal("offline code must be removed after use")
	}
}

func TestJoinAdoptsSameAliasInsteadOfDuplicating(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)
	seedDeviceA(t, a)
	if err := b.db.CreateServer(&model.Server{Alias: "web", Host: "old-address", Port: 22, AuthMethod: model.AuthKey}); err != nil {
		t.Fatal(err)
	}
	recovery, _, err := a.engine.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.engine.Join(recovery, ""); err != nil {
		t.Fatalf("join with recovery key: %v", err)
	}
	servers, _ := b.db.ListServers()
	if len(servers) != 2 {
		t.Fatalf("B should have bastion and one web, got %d", len(servers))
	}
	web, _ := b.db.GetServer("web")
	if web.Host != "web01.internal" {
		t.Fatalf("joining should adopt the space's version, got host %q", web.Host)
	}
}

func TestExistingDifferentKeyFileIsNeverOverwritten(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)
	seedDeviceA(t, a)
	keyPath := filepath.Join(b.home, ".ssh", "id_web")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("B's own key"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovery, _, _ := a.engine.Create()
	report, err := b.engine.Join(recovery, "")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(keyPath); string(data) != "B's own key" {
		t.Fatal("a local key file was overwritten")
	}
	found := false
	for _, warning := range report.Warnings {
		found = found || strings.Contains(warning, "local key was kept")
	}
	if !found {
		t.Fatalf("expected a warning about the kept key: %v", report.Warnings)
	}
}

func TestSyncRefusesLockedVaultAndForeignSpaces(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	if _, _, err := a.engine.Create(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.engine.Create(); !errors.Is(err, ErrSpaceExists) {
		t.Fatalf("second create = %v", err)
	}
	a.vault.Lock()
	if _, err := a.engine.Sync(); !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("locked sync = %v", err)
	}

	other := newDevice(t, storage)
	foreign, _ := NewKey()
	if _, err := other.engine.Join(RecoveryKey(foreign), ""); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("join with a foreign key = %v", err)
	}
	if _, err := other.engine.Join("123456", testPassword); !errors.Is(err, ErrNoPairing) {
		t.Fatalf("join without a pending code = %v", err)
	}
}

func TestGitStorageEndToEnd(t *testing.T) {
	remote := requireGit(t)
	a := newDevice(t, &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "a")})
	b := newDevice(t, &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "b")})
	seedDeviceA(t, a)
	if _, _, err := a.engine.Create(); err != nil {
		t.Fatal(err)
	}
	code, _, err := a.engine.Pair(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.engine.Join(code, testPassword); err != nil {
		t.Fatalf("join over git: %v", err)
	}
	if _, err := b.db.GetServer("web"); err != nil {
		t.Fatal("web missing on B")
	}
}

func TestKeptLocalKeyNeverTravelsToOtherDevices(t *testing.T) {
	storage := &Folder{Dir: t.TempDir()}
	a := newDevice(t, storage)
	b := newDevice(t, storage)
	seedDeviceA(t, a)
	original, _ := os.ReadFile(filepath.Join(a.home, ".ssh", "id_web"))
	originalPublic, _ := os.ReadFile(filepath.Join(a.home, ".ssh", "id_web.pub"))
	remote, _ := NewRecord(KindKey, "~/.ssh/id_web", KeyData{Path: "~/.ssh/id_web", Private: original, Public: originalPublic})

	// B's own key must lose even when its content hash would win a tie.
	content := ""
	for index := 0; ; index++ {
		candidate := fmt.Sprintf("B key %d", index)
		local, _ := NewRecord(KindKey, "~/.ssh/id_web", KeyData{Path: "~/.ssh/id_web", Private: []byte(candidate)})
		if local.Hash > remote.Hash {
			content = candidate
			break
		}
	}
	keyPath := filepath.Join(b.home, ".ssh", "id_web")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	recovery, _, err := a.engine.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.engine.Join(recovery, ""); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 3; round++ {
		mustSync(t, b)
		mustSync(t, a)
	}
	key, _ := a.engine.Key()
	bundle, _ := os.ReadFile(filepath.Join(storage.Dir, BundleFile))
	records, err := Open(key, bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Kind == KindKey && record.Hash != remote.Hash {
			t.Fatal("B's private key reached the bundle")
		}
	}
	if data, _ := os.ReadFile(filepath.Join(a.home, ".ssh", "id_web")); string(data) != string(original) {
		t.Fatal("A's key changed")
	}
	if data, _ := os.ReadFile(keyPath); string(data) != content {
		t.Fatal("B's key changed")
	}
}
