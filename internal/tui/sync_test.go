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
	joined   string
	password string
}

func installFakeSync(t *testing.T, info SyncInfo) *fakeSync {
	t.Helper()
	fake := &fakeSync{info: info}
	previous := []any{GetSyncInfo, SaveSyncSettings, RunSync, CreateSyncSpace, JoinSyncSpace, SyncRecoveryKey, LeaveSync, EnableSyncPassword}
	t.Cleanup(func() {
		GetSyncInfo = previous[0].(func() SyncInfo)
		SaveSyncSettings = previous[1].(func(SyncSettings) error)
		RunSync = previous[2].(func() (SyncResult, error))
		CreateSyncSpace = previous[3].(func(string) (SyncResult, error))
		JoinSyncSpace = previous[4].(func(string) (SyncResult, error))
		SyncRecoveryKey = previous[5].(func() (string, error))
		LeaveSync = previous[6].(func() error)
		EnableSyncPassword = previous[7].(func(string) error)
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
	CreateSyncSpace = func(password string) (SyncResult, error) {
		fake.password = password
		fake.info.Joined = true
		return SyncResult{RecoveryKey: "AAAA-BBBB"}, nil
	}
	JoinSyncSpace = func(password string) (SyncResult, error) {
		fake.joined = password
		if password != "right" {
			return SyncResult{}, errors.New("Wrong synchronization master password.")
		}
		fake.info.Joined = true
		return SyncResult{Received: 5}, nil
	}
	EnableSyncPassword = func(password string) error {
		fake.password = password
		return nil
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

func TestSyncRestoreByMasterPasswordOnly(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}})
	m := openSync(t)
	focusAction(t, m, "join")
	m, _ = press(m, key(tea.KeyEnter))
	if m.syncScreen.view != syncViewJoin {
		t.Fatal("restore view did not open")
	}
	if strings.Contains(m.View(), "Code") {
		t.Fatal("restore must not request a pairing code")
	}
	m = typeKeys(m, "wrong")
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if m.syncScreen.err == nil || fake.joined != "wrong" {
		t.Fatalf("wrong password: %v %q", m.syncScreen.err, fake.joined)
	}
	if m.syncScreen.view != syncViewJoin || m.syncScreen.password.Value() != "" {
		t.Fatal("password should be cleared for retry")
	}
	m = typeKeys(m, "right")
	m, cmd = press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if m.syncScreen.err != nil || !m.syncInfo.Joined {
		t.Fatalf("restore failed: %v", m.syncScreen.err)
	}
	if strings.Contains(m.View(), "right") {
		t.Fatal("password echoed")
	}
}

func TestSyncJoinedUsesPasswordRecoveryNotDevicePairing(t *testing.T) {
	fake := installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeGit, GitURL: "git@x:y.git", Auto: true}, Joined: true})
	m := openSync(t)
	for _, control := range m.syncScreen.controls() {
		if control.action == "pair" || control.action == "pair-offline" {
			t.Fatalf("device pairing must not appear: %s", control.action)
		}
	}
	focusAction(t, m, "enable-password")
	m, _ = press(m, key(tea.KeyEnter))
	if m.syncScreen.view != syncViewPassword {
		t.Fatal("password prompt missing")
	}
	m = typeKeys(m, "vault-secret")
	m, cmd := press(m, key(tea.KeyEnter))
	m = runCmd(m, cmd)
	if fake.password != "vault-secret" || m.syncScreen.err != nil || m.syncScreen.view != syncViewForm {
		t.Fatalf("password recovery: %q %v", fake.password, m.syncScreen.err)
	}
}

func TestSyncCreateShowsRecoveryKey(t *testing.T) {
	installFakeSync(t, SyncInfo{Settings: SyncSettings{Mode: SyncModeFolder, Folder: "/sync", Auto: true}})
	m := openSync(t)
	focusAction(t, m, "create")
	m, _ = press(m, key(tea.KeyEnter))
	if m.syncScreen.view != syncViewPassword {
		t.Fatal("create must ask for vault password")
	}
	m = typeKeys(m, "vault-secret")
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
		for _, view := range []syncView{syncViewForm, syncViewPassword, syncViewJoin, syncViewRecovery} {
			m := New(nil)
			m.width, m.height = size.width, size.height
			m.openSettingsMenu("sync")
			m, _ = press(m, key(tea.KeyEnter))
			m.syncScreen.view = view
			m.syncScreen.recovery = "MQB5-MJZ5-I6WP-ISER-KUEC-YXQC-RYJI-FRO2-I3ZN-4WTT-ZOPW-BIKA-L5QQ"
			assertUnifiedScreen(t, m.View(), size.width, size.height)
		}
	}
}
