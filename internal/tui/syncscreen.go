package tui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// Sync storage modes, matching config sync.mode.
const (
	SyncModeOff    = "off"
	SyncModeFolder = "folder"
	SyncModeGit    = "git"
)

var syncModes = []string{SyncModeOff, SyncModeFolder, SyncModeGit}

// SyncSettings is where the encrypted bundle is stored.
type SyncSettings struct {
	Mode      string
	Folder    string
	GitURL    string
	GitBranch string
	Auto      bool
}

// SyncInfo is the sync state of this device.
type SyncInfo struct {
	Settings  SyncSettings
	Joined    bool
	LastSync  time.Time
	LastError string
	Records   int
}

// SyncResult is the outcome of a sync operation.
type SyncResult struct {
	Received    int
	Sent        int
	Warnings    []string
	RecoveryKey string
}

// Sync callbacks, provided by the command layer.
var (
	GetSyncInfo      func() SyncInfo
	SaveSyncSettings func(SyncSettings) error
	RunSync          func() (SyncResult, error)
	CreateSyncSpace  func() (SyncResult, error)
	PairSyncDevice   func(password string) (string, time.Time, error)
	JoinSyncSpace    func(secret, password string) (SyncResult, error)
	SyncRecoveryKey  func() (string, error)
	LeaveSync        func() error
)

// autoSyncDelay debounces automatic syncs after local changes.
var autoSyncDelay = 3 * time.Second

type syncDoneMsg struct {
	kind   string
	result SyncResult
	err    error
}

type syncPairDoneMsg struct {
	code    string
	expires time.Time
	err     error
}

type syncTickMsg struct{ generation int }

type syncView int

const (
	syncViewForm syncView = iota
	syncViewPassword
	syncViewCode
	syncViewJoin
	syncViewRecovery
)

// syncControl is one focusable row of the sync form.
type syncControl struct {
	kind   string // mode, folder, url, branch, auto, button
	action string
	label  string
}

type syncScreenModel struct {
	width, height int
	info          SyncInfo
	saved         SyncSettings

	mode      int
	folder    textinput.Model
	gitURL    textinput.Model
	gitBranch textinput.Model
	auto      bool
	focus     int

	view        syncView
	password    textinput.Model
	secret      textinput.Model
	joinFocus   int
	pairCode    string
	pairExpires time.Time
	recovery    string

	busy     string
	notice   string
	err      error
	warnings []string
}

func newSyncInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.CharLimit = 512
	return input
}

func newSyncScreenModel(width, height int, info SyncInfo) *syncScreenModel {
	m := &syncScreenModel{
		width: width, height: height,
		folder:    newSyncInput("~/Sync/sshkeeper"),
		gitURL:    newSyncInput("git@git.example.org:me/sshkeeper-sync.git"),
		gitBranch: newSyncInput("sshkeeper"),
		password:  newSyncInput(""),
		secret:    newSyncInput("123 456"),
	}
	m.password.EchoMode = textinput.EchoPassword
	m.applyInfo(info)
	m.syncFocus()
	return m
}

func (m *syncScreenModel) applyInfo(info SyncInfo) {
	m.info = info
	m.saved = info.Settings
	m.mode = 0
	for index, mode := range syncModes {
		if mode == info.Settings.Mode {
			m.mode = index
		}
	}
	m.folder.SetValue(info.Settings.Folder)
	m.gitURL.SetValue(info.Settings.GitURL)
	m.gitBranch.SetValue(info.Settings.GitBranch)
	m.auto = info.Settings.Auto
}

func (m *syncScreenModel) settings() SyncSettings {
	return SyncSettings{
		Mode:      syncModes[m.mode],
		Folder:    strings.TrimSpace(m.folder.Value()),
		GitURL:    strings.TrimSpace(m.gitURL.Value()),
		GitBranch: strings.TrimSpace(m.gitBranch.Value()),
		Auto:      m.auto,
	}
}

func (m *syncScreenModel) dirty() bool {
	return m.settings() != m.saved
}

// controls lists the form rows. The storage fields depend on the chosen mode
// and the actions on whether this device has joined a sync space.
func (m *syncScreenModel) controls() []syncControl {
	controls := []syncControl{{kind: "mode"}}
	switch syncModes[m.mode] {
	case SyncModeFolder:
		controls = append(controls, syncControl{kind: "folder"})
	case SyncModeGit:
		controls = append(controls, syncControl{kind: "url"}, syncControl{kind: "branch"})
	}
	if syncModes[m.mode] != SyncModeOff {
		controls = append(controls, syncControl{kind: "auto"})
	}
	controls = append(controls, syncControl{kind: "button", action: "save", label: i18n.T("Save", "Сохранить")})
	if syncModes[m.mode] == SyncModeOff {
		if m.info.Joined {
			controls = append(controls, syncControl{kind: "button", action: "leave", label: i18n.T("Leave sync", "Отключить устройство")})
		}
		return controls
	}
	if m.info.Joined {
		controls = append(controls,
			syncControl{kind: "button", action: "sync", label: i18n.T("Sync now", "Синхронизировать")},
			syncControl{kind: "button", action: "pair", label: i18n.T("Add device", "Добавить устройство")},
			syncControl{kind: "button", action: "recovery", label: i18n.T("Recovery key", "Ключ восстановления")},
			syncControl{kind: "button", action: "leave", label: i18n.T("Leave sync", "Отключить устройство")},
		)
	} else {
		controls = append(controls,
			syncControl{kind: "button", action: "create", label: i18n.T("Create sync space", "Создать синхронизацию")},
			syncControl{kind: "button", action: "join", label: i18n.T("Join with code", "Подключиться по коду")},
		)
	}
	return controls
}

func (m *syncScreenModel) focused() syncControl {
	controls := m.controls()
	m.focus = min(max(0, m.focus), len(controls)-1)
	return controls[m.focus]
}

// syncFocus focuses the text input under the cursor.
func (m *syncScreenModel) syncFocus() {
	m.folder.Blur()
	m.gitURL.Blur()
	m.gitBranch.Blur()
	m.password.Blur()
	m.secret.Blur()
	switch m.view {
	case syncViewPassword:
		m.password.Focus()
		return
	case syncViewJoin:
		if m.joinFocus == 0 {
			m.secret.Focus()
		} else {
			m.password.Focus()
		}
		return
	}
	switch m.focused().kind {
	case "folder":
		m.folder.Focus()
	case "url":
		m.gitURL.Focus()
	case "branch":
		m.gitBranch.Focus()
	}
}

func (m *syncScreenModel) validate() error {
	settings := m.settings()
	switch settings.Mode {
	case SyncModeFolder:
		if settings.Folder == "" {
			return fmt.Errorf("%s", i18n.T("Choose a folder for the sync file.", "Укажите папку для файла синхронизации."))
		}
	case SyncModeGit:
		if settings.GitURL == "" {
			return fmt.Errorf("%s", i18n.T("Enter the git repository URL.", "Укажите адрес git-репозитория."))
		}
	}
	return nil
}

// --- tuiModel integration ---

func (m *tuiModel) openSettingsMenu(action string) {
	items := []list.Item{
		actionMenuItem{label: i18n.T("Language", "Язык"), action: "language", description: i18n.T("Choose the interface language.", "Выбрать язык интерфейса.")},
		actionMenuItem{label: i18n.T("Synchronization", "Синхронизация"), action: "sync", description: i18n.T("Keep profiles, secrets, and keys in step across your devices through an encrypted file in a folder or a git repository.", "Синхронизировать профили, секреты и ключи между устройствами через зашифрованный файл в папке или git-репозитории.")},
	}
	m.settingsMenu = newMenuModel(i18n.T("Settings", "Настройки"), items, m.width, m.height)
	for index, item := range items {
		if item.(actionMenuItem).action == action {
			m.settingsMenu.list.Select(index)
		}
	}
	m.screen = screenSettingsMenu
}

func (m *tuiModel) updateSettingsMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsMenu == nil {
		m.openSettingsMenu("language")
	}
	if msg.Type == tea.KeyEsc {
		m.settingsMenu = nil
		m.openManageMenu("settings")
		return m, nil
	}
	updated, action := m.settingsMenu.Update(msg)
	m.settingsMenu = updated
	if action == nil {
		return m, nil
	}
	switch *action {
	case "language":
		preference := "auto"
		if GetLanguagePreference != nil {
			preference = GetLanguagePreference()
		}
		m.settingsScreen = newSettingsModel(m.width, m.height, preference)
		m.screen = screenSettings
	case "sync":
		m.syncScreen = newSyncScreenModel(m.width, m.height, m.loadSyncInfo())
		m.screen = screenSync
	}
	return m, nil
}

func (m *tuiModel) loadSyncInfo() SyncInfo {
	if GetSyncInfo == nil {
		return SyncInfo{Settings: SyncSettings{Mode: SyncModeOff, Auto: true}}
	}
	m.syncInfo = GetSyncInfo()
	return m.syncInfo
}

func (m *tuiModel) autoSyncEnabled() bool {
	return RunSync != nil && m.syncInfo.Joined && m.syncInfo.Settings.Mode != SyncModeOff && m.syncInfo.Settings.Auto
}

// startSync runs one sync in the background.
func (m *tuiModel) startSync(kind string) tea.Cmd {
	if RunSync == nil || m.syncing {
		return nil
	}
	m.syncing = true
	return func() tea.Msg {
		result, err := RunSync()
		return syncDoneMsg{kind: kind, result: result, err: err}
	}
}

// scheduleAutoSync syncs a few seconds after the last local change.
func (m *tuiModel) scheduleAutoSync() tea.Cmd {
	if !m.autoSyncEnabled() {
		return nil
	}
	m.syncGeneration++
	generation := m.syncGeneration
	return tea.Tick(autoSyncDelay, func(time.Time) tea.Msg { return syncTickMsg{generation: generation} })
}

func (m *tuiModel) handleSyncTick(msg syncTickMsg) tea.Cmd {
	if msg.generation != m.syncGeneration {
		return nil
	}
	if m.syncing {
		// A sync is running; try again once it is done.
		return m.scheduleAutoSync()
	}
	return m.startSync("auto")
}

func (m *tuiModel) handleSyncDone(msg syncDoneMsg) tea.Cmd {
	m.syncing = false
	m.loadSyncInfo()
	var cmds []tea.Cmd
	if msg.err == nil && msg.result.Received > 0 {
		cmds = append(cmds, m.reloadServersCmd(), m.loadForwardIndexCmd())
	}
	if screen := m.syncScreen; screen != nil {
		screen.busy = ""
		screen.applyInfo(m.syncInfo)
		screen.err = msg.err
		screen.warnings = msg.result.Warnings
		if msg.err == nil {
			switch msg.kind {
			case "create":
				screen.recovery = msg.result.RecoveryKey
				screen.view = syncViewRecovery
				screen.notice = i18n.T("Sync space created. Save the recovery key below.", "Синхронизация создана. Сохраните ключ восстановления.")
			case "join":
				screen.view = syncViewForm
				screen.notice = i18n.Tf("This device joined the sync: %d change(s) received.", "Устройство подключено: получено изменений — %d.", msg.result.Received)
			case "leave":
				screen.notice = i18n.T("This device no longer syncs. Its data is unchanged.", "Устройство больше не синхронизируется. Его данные не изменены.")
			case "sync":
				screen.notice = i18n.Tf("Synced: %d received, %d sent.", "Синхронизировано: получено — %d, отправлено — %d.", msg.result.Received, msg.result.Sent)
			}
		}
		screen.syncFocus()
	}
	if msg.err == nil && msg.kind == "auto" && msg.result.Received > 0 && m.screen == screenList {
		m.success = i18n.Tf("⇅ %d change(s) arrived from your other devices.", "⇅ Получено изменений с других устройств: %d.", msg.result.Received)
	}
	return tea.Batch(cmds...)
}

func (m *tuiModel) updateSync(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.syncScreen
	if s == nil {
		m.openSettingsMenu("sync")
		return m, nil
	}
	if s.busy != "" {
		return m, nil
	}
	s.notice = ""
	switch s.view {
	case syncViewPassword:
		return m.updateSyncPassword(msg)
	case syncViewJoin:
		return m.updateSyncJoin(msg)
	case syncViewCode, syncViewRecovery:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
			s.view = syncViewForm
			s.recovery = ""
			s.syncFocus()
		}
		return m, nil
	}

	control := s.focused()
	switch msg.Type {
	case tea.KeyEsc:
		if s.dirty() {
			m.confirmDiscard(i18n.T("Sync settings", "Настройки синхронизации"), screenSync, screenSettingsMenu)
			return m, nil
		}
		m.syncScreen = nil
		m.openSettingsMenu("sync")
		return m, nil
	case tea.KeyUp, tea.KeyShiftTab:
		s.focus = max(0, s.focus-1)
		s.syncFocus()
		return m, nil
	case tea.KeyDown, tea.KeyTab:
		s.focus = min(len(s.controls())-1, s.focus+1)
		s.syncFocus()
		return m, nil
	case tea.KeyCtrlS:
		return m, m.saveSyncSettings()
	}

	switch control.kind {
	case "mode":
		switch msg.Type {
		case tea.KeyLeft:
			s.mode = (s.mode + len(syncModes) - 1) % len(syncModes)
		case tea.KeyRight, tea.KeySpace:
			s.mode = (s.mode + 1) % len(syncModes)
		}
		s.err = nil
		return m, nil
	case "auto":
		if msg.Type == tea.KeySpace || msg.Type == tea.KeyEnter || msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight {
			s.auto = !s.auto
		}
		return m, nil
	case "folder", "url", "branch":
		if msg.Type == tea.KeyEnter {
			s.focus++
			s.syncFocus()
			return m, nil
		}
		var cmd tea.Cmd
		switch control.kind {
		case "folder":
			s.folder, cmd = s.folder.Update(msg)
		case "url":
			s.gitURL, cmd = s.gitURL.Update(msg)
		default:
			s.gitBranch, cmd = s.gitBranch.Update(msg)
		}
		return m, cmd
	case "button":
		if msg.Type != tea.KeyEnter {
			return m, nil
		}
		return m.runSyncAction(control.action)
	}
	return m, nil
}

// saveSyncSettings persists the form. Actions save first, so what the user
// sees is what they act on.
func (m *tuiModel) saveSyncSettings() tea.Cmd {
	s := m.syncScreen
	if err := s.validate(); err != nil {
		s.err = err
		return nil
	}
	if SaveSyncSettings == nil {
		s.err = fmt.Errorf("%s", i18n.T("sync settings are unavailable", "настройки синхронизации недоступны"))
		return nil
	}
	if err := SaveSyncSettings(s.settings()); err != nil {
		s.err = err
		return nil
	}
	s.err = nil
	s.applyInfo(m.loadSyncInfo())
	s.notice = i18n.T("Sync settings saved.", "Настройки синхронизации сохранены.")
	return nil
}

func (m *tuiModel) runSyncAction(action string) (tea.Model, tea.Cmd) {
	s := m.syncScreen
	if action == "save" {
		return m, m.saveSyncSettings()
	}
	if s.dirty() {
		m.saveSyncSettings()
		if s.err != nil {
			return m, nil
		}
	}
	s.err, s.warnings = nil, nil
	switch action {
	case "sync":
		if cmd := m.startSync("sync"); cmd != nil {
			s.busy = i18n.T("Syncing…", "Синхронизация…")
			return m, cmd
		}
	case "create":
		if CreateSyncSpace == nil {
			return m, nil
		}
		s.busy = i18n.T("Creating the sync space…", "Создание синхронизации…")
		m.syncing = true
		return m, func() tea.Msg {
			result, err := CreateSyncSpace()
			return syncDoneMsg{kind: "create", result: result, err: err}
		}
	case "join":
		s.view = syncViewJoin
		s.joinFocus = 0
		s.secret.SetValue("")
		s.password.SetValue("")
		s.syncFocus()
	case "pair":
		s.view = syncViewPassword
		s.password.SetValue("")
		s.syncFocus()
	case "recovery":
		if SyncRecoveryKey == nil {
			return m, nil
		}
		key, err := SyncRecoveryKey()
		if err != nil {
			s.err = err
			return m, nil
		}
		s.recovery = key
		s.view = syncViewRecovery
	case "leave":
		m.beginConfirm(confirmState{
			title:       i18n.T("Stop syncing this device?", "Отключить это устройство от синхронизации?"),
			target:      i18n.T("Sync on this device", "Синхронизация на этом устройстве"),
			consequence: i18n.T("Profiles, secrets, and keys stay here. Other devices keep syncing. You can join again later with a code.", "Профили, секреты и ключи останутся здесь. Остальные устройства продолжат синхронизацию. Подключиться снова можно по коду."),
			verb:        i18n.T("Leave", "Отключить"),
			parent:      screenSync,
			action: func() tea.Cmd {
				return func() tea.Msg {
					if LeaveSync == nil {
						return syncDoneMsg{kind: "leave", err: fmt.Errorf("%s", i18n.T("sync is unavailable", "синхронизация недоступна"))}
					}
					return syncDoneMsg{kind: "leave", err: LeaveSync()}
				}
			},
		})
	}
	return m, nil
}

func (m *tuiModel) updateSyncPassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.syncScreen
	switch msg.Type {
	case tea.KeyEsc:
		s.view = syncViewForm
		s.password.SetValue("")
		s.syncFocus()
		return m, nil
	case tea.KeyEnter:
		password := s.password.Value()
		s.password.SetValue("")
		if password == "" || PairSyncDevice == nil {
			return m, nil
		}
		s.busy = i18n.T("Creating a pairing code…", "Создание кода…")
		return m, func() tea.Msg {
			code, expires, err := PairSyncDevice(password)
			return syncPairDoneMsg{code: code, expires: expires, err: err}
		}
	}
	var cmd tea.Cmd
	s.password, cmd = s.password.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateSyncJoin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.syncScreen
	recovery := looksLikeRecoveryKey(s.secret.Value())
	switch msg.Type {
	case tea.KeyEsc:
		s.view = syncViewForm
		s.secret.SetValue("")
		s.password.SetValue("")
		s.syncFocus()
		return m, nil
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		if !recovery {
			s.joinFocus = 1 - s.joinFocus
			s.syncFocus()
		}
		return m, nil
	case tea.KeyEnter:
		if s.joinFocus == 0 && !recovery {
			s.joinFocus = 1
			s.syncFocus()
			return m, nil
		}
		secret, password := s.secret.Value(), s.password.Value()
		s.password.SetValue("")
		if strings.TrimSpace(secret) == "" || JoinSyncSpace == nil {
			return m, nil
		}
		s.busy = i18n.T("Joining the sync…", "Подключение к синхронизации…")
		m.syncing = true
		return m, func() tea.Msg {
			result, err := JoinSyncSpace(secret, password)
			return syncDoneMsg{kind: "join", result: result, err: err}
		}
	}
	var cmd tea.Cmd
	if s.joinFocus == 0 {
		s.secret, cmd = s.secret.Update(msg)
	} else {
		s.password, cmd = s.password.Update(msg)
	}
	return m, cmd
}

// looksLikeRecoveryKey mirrors syncer.LooksLikeRecoveryKey: a pairing code
// has six digits, a recovery key is much longer.
func looksLikeRecoveryKey(text string) bool {
	return len(strings.TrimSpace(text)) > 12
}

func (m *tuiModel) handleSyncPairDone(msg syncPairDoneMsg) {
	s := m.syncScreen
	if s == nil {
		return
	}
	s.busy = ""
	if msg.err != nil {
		s.err = msg.err
		s.view = syncViewForm
		s.syncFocus()
		return
	}
	s.pairCode, s.pairExpires = msg.code, msg.expires
	s.view = syncViewCode
}

// --- view ---

func (s *syncScreenModel) View(vaultUnlocked bool) string {
	notification := ""
	switch {
	case s.busy != "":
		notification = stateTestingStyle.Render(s.busy)
	case s.err != nil:
		notification = errorStyle.Render(s.err.Error())
	case s.notice != "":
		notification = successStyle.Render(s.notice)
	}
	footer := []helpItem{
		{Key: "↑/↓", Action: i18n.T("move", "перемещение")},
		{Key: "←/→", Action: i18n.T("choose", "выбор")},
		{Key: "Enter", Action: i18n.T("activate", "выполнить")},
		{Key: "Ctrl+S", Action: i18n.T("save", "сохранить")},
		{Key: "Esc", Action: i18n.T("back", "назад")},
	}
	var body func(width, height int) string
	switch s.view {
	case syncViewPassword:
		footer = []helpItem{{Key: "Enter", Action: i18n.T("continue", "продолжить")}, {Key: "Esc", Action: i18n.T("cancel", "отмена")}}
		body = func(width, height int) string {
			return renderPaddedPanel(width, height, append([]string{
				dashboardSection(i18n.T("Add a device", "Добавить устройство")), "",
			}, append(wrapCells(i18n.T("Confirm the master password of this device. The new device will need it together with a six-digit code.", "Подтвердите мастер-пароль этого устройства. Новому устройству понадобятся он и шестизначный код."), max(1, width-6)),
				"", focusedStyle.Render(i18n.T("Master password> ", "Мастер-пароль> "))+s.password.View())...))
		}
	case syncViewCode:
		footer = []helpItem{{Key: "Enter", Action: i18n.T("done", "готово")}}
		body = func(width, height int) string {
			code := s.pairCode
			if len(code) == 6 {
				code = code[:3] + " " + code[3:]
			}
			lines := []string{dashboardSection(i18n.T("Pairing code", "Код сопряжения")), "", brandStyle.Render("    " + code), ""}
			lines = append(lines, wrapCells(i18n.Tf("On the new device open Settings → Synchronization, choose the same storage, then \"Join with code\" and enter this code with the master password of this device. Valid until %s.", "На новом устройстве откройте Настройки → Синхронизация, выберите то же хранилище, затем «Подключиться по коду» и введите этот код и мастер-пароль этого устройства. Код действует до %s.", s.pairExpires.Format("15:04")), max(1, width-6))...)
			return renderPaddedPanel(width, height, lines)
		}
	case syncViewJoin:
		footer = []helpItem{{Key: "Tab", Action: i18n.T("next field", "следующее поле")}, {Key: "Enter", Action: i18n.T("join", "подключиться")}, {Key: "Esc", Action: i18n.T("cancel", "отмена")}}
		body = func(width, height int) string {
			lines := []string{dashboardSection(i18n.T("Join with code", "Подключиться по коду")), ""}
			lines = append(lines, wrapCells(i18n.T("Enter the six-digit code shown on a device that already syncs, and that device's master password. A recovery key works too.", "Введите шестизначный код с уже подключённого устройства и мастер-пароль того устройства. Подойдёт и ключ восстановления."), max(1, width-6))...)
			label := func(text string, focused bool) string {
				if focused {
					return focusedStyle.Render(text + "> ")
				}
				return blurredStyle.Render(text + ": ")
			}
			lines = append(lines, "", label(i18n.T("Code", "Код"), s.joinFocus == 0)+s.secret.View())
			if !looksLikeRecoveryKey(s.secret.Value()) {
				lines = append(lines, label(i18n.T("Master password of that device", "Мастер-пароль того устройства"), s.joinFocus == 1)+s.password.View())
			}
			return renderPaddedPanel(width, height, lines)
		}
	case syncViewRecovery:
		footer = []helpItem{{Key: "Enter", Action: i18n.T("done", "готово")}}
		body = func(width, height int) string {
			lines := []string{dashboardSection(i18n.T("Recovery key", "Ключ восстановления")), ""}
			lines = append(lines, wrapCells(s.recovery, max(1, width-6))...)
			lines = append(lines, "")
			lines = append(lines, wrapCells(i18n.T("Keep it offline, for example printed or in a password manager. It restores sync if every device is lost. Anyone with it and the sync file can read your data.", "Храните его офлайн: на бумаге или в менеджере паролей. Он восстановит синхронизацию, если потеряны все устройства. Любой, у кого есть ключ и файл синхронизации, сможет прочитать данные."), max(1, width-6))...)
			return renderPaddedPanel(width, height, lines)
		}
	default:
		body = s.formBody
	}
	return renderScreenShell(screenShell{
		breadcrumb:   i18n.T("Settings / Synchronization", "Настройки / Синхронизация"),
		status:       shellStatus(vaultUnlocked, s.statusText()),
		notification: notification,
		width:        s.width,
		height:       s.height,
		body:         body,
		footer:       footer,
	})
}

func (s *syncScreenModel) statusText() string {
	switch {
	case !s.info.Joined:
		return i18n.T("not set up", "не настроена")
	case s.info.LastSync.IsZero():
		return i18n.T("set up", "настроена")
	default:
		return i18n.Tf("synced %s", "синхр. %s", ageLong(&s.info.LastSync))
	}
}

func (s *syncScreenModel) formBody(width, height int) string {
	controls := s.controls()
	s.focus = min(max(0, s.focus), len(controls)-1)
	inner := max(1, width-6)
	label := func(text string, index int) string {
		if index == s.focus {
			return focusedStyle.Render(padCells(text, 12) + "> ")
		}
		return blurredStyle.Render(padCells(text, 12) + ": ")
	}
	lines := []string{}
	var buttons []string
	for index, control := range controls {
		switch control.kind {
		case "mode":
			names := []string{i18n.T("off", "выкл."), i18n.T("folder", "папка"), "git"}
			parts := make([]string, len(names))
			for option, name := range names {
				switch {
				case option == s.mode && index == s.focus:
					parts[option] = selectedStyle.Render(glyphs.pickLeft + name + glyphs.pickRight)
				case option == s.mode:
					parts[option] = normalStyle.Copy().Bold(true).Render(glyphs.pickLeft + name + glyphs.pickRight)
				default:
					parts[option] = mutedStyle.Render(name)
				}
			}
			lines = append(lines, label(i18n.T("Storage", "Хранилище"), index)+strings.Join(parts, " "))
		case "folder":
			lines = append(lines, label(i18n.T("Folder", "Папка"), index)+s.folder.View())
		case "url":
			lines = append(lines, label(i18n.T("Repository", "Репозиторий"), index)+s.gitURL.View())
		case "branch":
			lines = append(lines, label(i18n.T("Branch", "Ветка"), index)+s.gitBranch.View())
		case "auto":
			mark := "[ ]"
			if s.auto {
				mark = "[x]"
			}
			lines = append(lines, label(i18n.T("Auto sync", "Автосинхр."), index)+mark+" "+mutedStyle.Render(i18n.T("on start and after changes", "при запуске и после изменений")))
		case "button":
			text := "[ " + control.label + " ]"
			if index == s.focus {
				text = selectedStyle.Render("> " + text)
			}
			buttons = append(buttons, text)
		}
	}
	lines = append(lines, "")
	for _, line := range wrapCells(s.modeHint(), inner) {
		lines = append(lines, mutedStyle.Render(line))
	}
	lines = append(lines, "")
	// Buttons wrap to the panel width.
	row := ""
	for _, button := range buttons {
		if row != "" && lineWidth(row)+2+lineWidth(button) > inner {
			lines = append(lines, row)
			row = ""
		}
		if row != "" {
			row += "  "
		}
		row += button
	}
	if row != "" {
		lines = append(lines, row)
	}
	lines = append(lines, "")
	lines = append(lines, s.statusLines(inner)...)
	for _, warning := range s.warnings {
		lines = append(lines, stateTestingStyle.Render("! "+warning))
	}
	return renderPaddedPanel(width, height, lines)
}

func (s *syncScreenModel) modeHint() string {
	switch syncModes[s.mode] {
	case SyncModeFolder:
		return i18n.T("Any folder your devices share: Syncthing, Nextcloud, Dropbox, or a network drive. Only an encrypted file is written there.", "Любая папка, общая для устройств: Syncthing, Nextcloud, Dropbox или сетевой диск. Туда пишется только зашифрованный файл.")
	case SyncModeGit:
		return i18n.T("A private repository you can push to with your own git credentials. The encrypted file lives on its own branch.", "Приватный репозиторий, в который вы можете делать push своими учётными данными git. Зашифрованный файл хранится в отдельной ветке.")
	default:
		return i18n.T("Sync is off. Choose a folder or a git repository to keep your devices in step.", "Синхронизация выключена. Выберите папку или git-репозиторий, чтобы устройства оставались в согласии.")
	}
}

func (s *syncScreenModel) statusLines(width int) []string {
	lines := []string{dashboardSection(i18n.T("This device", "Это устройство"))}
	if !s.info.Joined {
		hint := i18n.T("Not set up. On the first device choose \"Create sync space\"; on the others choose \"Join with code\".", "Не настроено. На первом устройстве выберите «Создать синхронизацию», на остальных — «Подключиться по коду».")
		for _, line := range wrapCells(hint, width) {
			lines = append(lines, mutedStyle.Render(line))
		}
		return lines
	}
	last := i18n.T("never", "никогда")
	if !s.info.LastSync.IsZero() {
		last = ageLong(&s.info.LastSync)
	}
	lines = append(lines, detailRow(i18n.T("Last sync", "Последняя"), last), detailRow(i18n.T("Items", "Объектов"), fmt.Sprint(s.info.Records)))
	if s.info.LastError != "" {
		lines = append(lines, detailRow(i18n.T("Last error", "Ошибка"), errorStyle.Render(firstLine(s.info.LastError))))
	}
	return lines
}

func lineWidth(text string) int {
	return lipgloss.Width(text)
}

// localChanges counts successful data changes made through the TUI
// callbacks. Update compares it with the last seen value to schedule an
// automatic sync; changes applied by sync itself do not go through the
// callbacks, so they never trigger another sync.
var localChanges atomic.Int64

// TrackLocalChanges wraps every data-changing callback so a successful
// change is counted. Call it once, after the callbacks are set.
func TrackLocalChanges() {
	count := func(err error) error {
		if err == nil {
			localChanges.Add(1)
		}
		return err
	}
	if f := SaveServer; f != nil {
		SaveServer = func(server *model.Server, password, oldAlias string) error {
			return count(f(server, password, oldAlias))
		}
	}
	if f := DeleteServer; f != nil {
		DeleteServer = func(alias string) error { return count(f(alias)) }
	}
	if f := SetServerTags; f != nil {
		SetServerTags = func(server *model.Server, tags []string) error { return count(f(server, tags)) }
	}
	for _, target := range []*func(string) error{&CreateGroup, &DeleteGroup, &DeleteTag, &DeleteCommandTemplate} {
		if f := *target; f != nil {
			*target = func(name string) error { return count(f(name)) }
		}
	}
	for _, target := range []*func(string, string) error{&RenameGroup, &RenameTag} {
		if f := *target; f != nil {
			*target = func(oldName, newName string) error { return count(f(oldName, newName)) }
		}
	}
	if f := SaveCommandTemplate; f != nil {
		SaveCommandTemplate = func(oldName string, template *model.CommandTemplate) error { return count(f(oldName, template)) }
	}
	for _, target := range []*func(*model.Forward) error{&SaveForward, &UpdateForward} {
		if f := *target; f != nil {
			*target = func(fwd *model.Forward) error { return count(f(fwd)) }
		}
	}
	if f := DeleteForward; f != nil {
		DeleteForward = func(id int64) error { return count(f(id)) }
	}
	if f := ImportServers; f != nil {
		ImportServers = func() (int, error) {
			imported, err := f()
			if imported > 0 {
				count(err)
			}
			return imported, err
		}
	}
}
