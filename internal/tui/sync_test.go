package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// fakeSync installs sync callbacks backed by in-memory state.
type fakeSync struct {
	info     SyncInfo
	saved    []SyncSettings
	syncs    int
	joined   [2]string
	password string
}

func installFakeSync(t *testing.T, info SyncInfo) *fakeSync {
	t.Helper()
	fake := &fakeSync{info: info}
	previous := []any{GetSyncInfo, SaveSyncSettings, RunSync, CreateSyncSpace, PairSyncDevice, PairSyncDeviceOffline, JoinSyncSpace, SyncRecoveryKey, LeaveSync}
	t.Cleanup(func() {
		GetSyncInfo = previous[0].(func() SyncInfo)
		SaveSyncSettings = previous[1].(func(SyncSettings) error)
		RunSync = previous[2].(func() (SyncResult, error))
		CreateSyncSpace = previous[3].(func() (SyncResult, error))
		PairSyncDevice = previous[4].(func(string) (string, time.Time, error))
		PairSyncDeviceOffline = previous[5].(func() (string, time.Time, error))
		JoinSyncSpace = previous[6].(func(string, string) (SyncResult, error))
		SyncRecoveryKey = previous[7].(func() (string, error))
		LeaveSync = previous[8].(func() error)
	})
	GetSyncInfo = func() SyncInfo { return fake.info }
	SaveSyncSettings = func(settings SyncSettings) error {
		fake.saved = append(fake.saved, settings)
		fake.info.Settings = settings
		return nil
	}
	RunSync = func() (SyncResult, error) {
		fake.syncs++
		fake.info.LastSync = time.Now()
		return SyncResult{Received: 2, Sent: 1}, nil
	}
	CreateSyncSpace = func() (SyncResult, error) {
		fake.info.Joined = true
		return SyncResult{RecoveryKey: "AAAA-BBBB"}, nil
	}
	PairSyncDevice = func(password string) (string, time.Time, error) {
		fake.password = password
		return "123456", time.Now().Add(10 * time.Minute), nil
	}
	PairSyncDeviceOffline = func() (string, time.Time, error) {
		return "SKP1-ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZ", time.Now().Add(24 * time.Hour), nil
	}
	JoinSyncSpace = func(secret, password string) (SyncResult, error) {
		fake.joined = [2]string{secret, password}
		if password != "right" {
			return SyncResult{}, errors.New("Wrong pairing code or master password.")
		}
		fake.info.Joined = true
		return SyncResult{Received: 5}, nil
	}
	SyncRecoveryKey = func() (string, error) { return "RECO-VERY", nil }
	LeaveSync = func() error { fake.info.Joined = false; return nil }
	return fake
}

func key(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func press(m *tuiModel, msgs ...tea.Msg) (*tuiModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var updated tea.Model
		updated, cmd = m.Update(msg)
		m = updated.(*tuiModel)
	}
	return m, cmd
}

// runCmd delivers a command's messages, skipping timers.
func runCmd(m *tuiModel, cmd tea.Cmd) *tuiModel {
	for _, msg := range runBatch(cmd) {
		switch msg.(type) {
		case noticeExpiredMsg, syncTickMsg, runtimeTickMsg:
			continue
		}
		m, _ = press(m, msg)
	}
	return m
}

func openSync(t *testing.T) *tuiModel {
	t.Helper()
	m := New([]*model.Server{{Alias: "a", Host: "a", Port: 22, User: "u"}})
	m.width, m.height = 100, 30
	m.openSettingsMenu("sync")
	m, _ = press(m, key(tea.KeyEnter))
	if m.screen != screenSync {
		t.Fatalf("sync screen did not open: %v", m.screen)
	}
	return m
}

func focusAction(t *testing.T, m *tuiModel, action string) {
	t.Helper()
	for index, control := range m.syncScreen.controls() {
		if control.action == action {
			m.syncScreen.focus = index
			m.syncScreen.syncFocus()
			return
		}
	}
	t.Fatalf("no %q button in %+v", action, m.syncScreen.controls())
}

func TestSyncFormFieldsFollowTheChosenStorage(t *testing.T) {
	installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeOff, Auto: true}})
	m := openSync(t)
	kinds := func() string {
		var parts []string
		for _, control := range m.syncScreen.controls() {
			parts = append(parts, control.kind+":"+control.action)
		}
		return strings.Join(parts, " ")
	}
	if got := kinds(); got != "mode: button:save" {
		t.Fatalf("off controls = %s", got)
	}
	m, _ = press(m, key(tea.KeyRight))
	if got := kinds(); got != "mode: folder: auto: button:save button:create button:join" {
		t.Fatalf("folder controls = %s", got)
	}
	m, _ = press(m, key(tea.KeyRight))
	if got := kinds(); got != "mode: url: branch: auto: button:save button:create button:join" {
		t.Fatalf("git controls = %s", got)
	}
	if view := m.View(); !strings.Contains(view, "Repository") || !strings.Contains(view, "own branch") {
		t.Fatalf("git form should explain itself:\n%s", view)
	}
}

func TestSyncSaveValidatesAndPersists(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeOff, Auto: true}})
	m := openSync(t)
	m, _ = press(m, key(tea.KeyRight), key(tea.KeyCtrlS))
	if m.syncScreen.err == nil || len(fake.saved) != 0 {
		t.Fatal("an empty folder must not be saved")
	}
	m, _ = press(m, key(tea.KeyDown))
	m = typeKeys(m, "~/Sync/keys")
	m, _ = press(m, key(tea.KeyCtrlS))
	if len(fake.saved) != 1 || fake.saved[0].Mode != SyncModeFolder || fake.saved[0].Folder != "~/Sync/keys" || !fake.saved[0].Auto {
		t.Fatalf("saved = %+v", fake.saved)
	}
	if m.syncScreen.dirty() {
		t.Fatal("form should be clean after saving")
	}
}

func TestSyncJoinWithCodeAndPassword(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}})
	m := openSync(t)
	focusAction(t, m, "join")
	m, _ = press(m, key(tea.KeyEnter))
	if m.syncScreen.view != syncViewJoin {
		t.Fatal("join view did not open")
	}
	m = typeKeys(m, "123 456")
	m, _ = press(m, key(tea.KeyEnter))
	m = typeKeys(m, "wrong")
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if m.syncScreen.err == nil || fake.joined != [2]string{"123 456", "wrong"} {
		t.Fatalf("wrong password should fail: err=%v joined=%v", m.syncScreen.err, fake.joined)
	}

	// A failed attempt keeps the code and asks for the password again.
	if m.syncScreen.view != syncViewJoin || m.syncScreen.secret.Value() != "123 456" || m.syncScreen.password.Value() != "" {
		t.Fatalf("retry state: view=%v secret=%q", m.syncScreen.view, m.syncScreen.secret.Value())
	}
	m = typeKeys(m, "right")
	m, cmd = press(m, key(tea.KeyEnter))
	if !strings.Contains(m.View(), "Joining") {
		t.Fatal("join should show progress")
	}
	m = runCmd(m, cmd)
	if m.syncScreen.err != nil || !strings.Contains(m.syncScreen.notice, "5 change") {
		t.Fatalf("join result: err=%v notice=%q", m.syncScreen.err, m.syncScreen.notice)
	}
	if !m.syncInfo.Joined {
		t.Fatal("sync info should be refreshed")
	}
	if strings.Contains(m.View(), "right") {
		t.Fatal("the password must never be displayed")
	}
}

func TestSyncJoinWithRecoveryKeyHidesPassword(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}})
	m := openSync(t)
	focusAction(t, m, "join")
	m, _ = press(m, key(tea.KeyEnter))
	m = typeKeys(m, "ABCD-EFGH-IJKL-MNOP")
	if strings.Contains(m.View(), "Master password") {
		t.Fatal("a recovery key needs no password field")
	}
	_, cmd := press(m, key(tea.KeyEnter))
	runCmd(m, cmd)
	if fake.joined[0] != "ABCD-EFGH-IJKL-MNOP" || fake.joined[1] != "" {
		t.Fatalf("joined = %v", fake.joined)
	}
}

func TestSyncAddDeviceShowsCode(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeGit, GitURL: "git@x:y.git", Auto: true}, Joined: true})
	m := openSync(t)
	focusAction(t, m, "pair")
	m, _ = press(m, key(tea.KeyEnter))
	m = typeKeys(m, "pw")
	if strings.Contains(m.View(), "pw\n") {
		t.Fatal("password echoed")
	}
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if fake.password != "pw" || m.syncScreen.view != syncViewCode || !strings.Contains(m.View(), "123 456") {
		t.Fatalf("code view:\n%s", m.View())
	}
	m, _ = press(m, key(tea.KeyEnter))
	if m.syncScreen.view != syncViewForm {
		t.Fatal("Enter should close the code")
	}
}

func TestSyncOfflinePairingShowsLongLivedCodeAndNeedsNoPassword(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeGit, GitURL: "git@x:y.git", Auto: true}, Joined: true})
	m := openSync(t)
	focusAction(t, m, "pair-offline")
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if m.syncScreen.view != syncViewCode || !m.syncScreen.pairOffline || !strings.Contains(m.View(), "SKP1-") {
		t.Fatalf("offline code view:\n%s", m.View())
	}
	if strings.Contains(m.View(), "master password of this device") {
		t.Fatal("offline pairing must not ask for the old device master password")
	}

	fake.info.Joined = false
	m.syncScreen.info.Joined = false
	m.syncScreen.view = syncViewForm
	focusAction(t, m, "join")
	m, _ = press(m, key(tea.KeyEnter))
	m = typeKeys(m, "SKP1-ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZ")
	if strings.Contains(m.View(), "Master password") {
		t.Fatal("offline code must hide the master-password field")
	}
	_, cmd = press(m, key(tea.KeyEnter))
	runCmd(m, cmd)
	if fake.joined[0] == "" || fake.joined[1] != "" {
		t.Fatalf("offline joined = %v", fake.joined)
	}
}

func TestSyncCreateShowsRecoveryKey(t *testing.T) {
	installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}})
	m := openSync(t)
	focusAction(t, m, "create")
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if m.syncScreen.view != syncViewRecovery || !strings.Contains(m.View(), "AAAA-BBBB") {
		t.Fatalf("recovery key not shown:\n%s", m.View())
	}
}

func TestSyncEscWithUnsavedChangesAsks(t *testing.T) {
	installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeOff, Auto: true}})
	m := openSync(t)
	m, _ = press(m, key(tea.KeyRight), key(tea.KeyEsc))
	if m.screen != screenConfirm {
		t.Fatal("unsaved sync settings should ask before leaving")
	}
	m, _ = press(m, key(tea.KeyEsc))
	m, _ = press(m, key(tea.KeyLeft), key(tea.KeyEsc))
	if m.screen != screenSettingsMenu {
		t.Fatalf("clean form should return to Settings, got %v", m.screen)
	}
}

func TestLocalChangesScheduleAutoSync(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}, Joined: true})
	previousSave := SaveServer
	t.Cleanup(func() { SaveServer = previousSave })
	SaveServer = func(*model.Server, string, string) error { return nil }
	TrackLocalChanges()

	m := New([]*model.Server{{Alias: "a", Host: "a", Port: 22, User: "u"}})
	m.width, m.height = 100, 30
	if err := SaveServer(&model.Server{}, "", ""); err != nil {
		t.Fatal(err)
	}
	m, cmd := press(m, key(tea.KeyDown))
	var tick syncTickMsg
	for _, msg := range runBatch(cmd) {
		if found, ok := msg.(syncTickMsg); ok {
			tick = found
		}
	}
	if tick.generation == 0 {
		t.Fatal("a local change should schedule an automatic sync")
	}
	m, cmd = press(m, tick)
	if !m.syncing || !strings.Contains(m.View(), "syncing") {
		t.Fatal("the tick should start a sync shown in the header")
	}
	m = runCmd(m, cmd)
	if fake.syncs != 1 || m.syncing {
		t.Fatalf("sync ran %d times, syncing=%v", fake.syncs, m.syncing)
	}
	if !strings.Contains(m.View(), "⇅") {
		t.Fatalf("header should show the sync badge:\n%s", m.View())
	}
}

func TestSyncScreenFitsSupportedSizes(t *testing.T) {
	installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeGit, GitURL: "git@git.example.org:someone/a-rather-long-repository-name.git", Auto: true}, Joined: true, LastSync: time.Now()})
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		for _, view := range []syncView{syncViewForm, syncViewPassword, syncViewCode, syncViewJoin, syncViewRecovery} {
			m := New(nil)
			m.width, m.height = size.width, size.height
			m.openSettingsMenu("sync")
			m, _ = press(m, key(tea.KeyEnter))
			m.syncScreen.view = view
			m.syncScreen.pairCode = "123456"
			m.syncScreen.recovery = "MQB5-MJZ5-I6WP-ISER-KUEC-YXQC-RYJI-FRO2-I3ZN-4WTT-ZOPW-BIKA-L5QQ"
			assertUnifiedScreen(t, m.View(), size.width, size.height)
		}
	}
}
