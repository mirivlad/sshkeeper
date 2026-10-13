package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atotto/clipboard"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	workspacepkg "github.com/mirivlad/sshkeeper/internal/workspace"
)

type workspaceOpenedMsg struct {
	session *workspacepkg.Session
	err     error
}

type workspaceSessionUpdateMsg struct {
	id   string
	done bool
}

type workspaceCopiedMsg struct{ err error }
type workspacePastedMsg struct {
	id   string
	text string
	err  error
}

// Mouse selection is captured by sshkeeper, not by the enclosing terminal.
// Publish it to PRIMARY as well as CLIPBOARD so middle-click works on X11 and
// Wayland desktops. PRIMARY is optional (e.g. on Windows), so failures here
// must never make an otherwise successful copy fail.
func workspaceWritePrimary(text string) {
	var name string
	var args []string
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "":
		name, args = "wl-copy", []string{"--primary"}
	case os.Getenv("DISPLAY") != "":
		if _, err := exec.LookPath("xclip"); err == nil {
			name, args = "xclip", []string{"-in", "-selection", "primary"}
		} else {
			name, args = "xsel", []string{"--input", "--primary"}
		}
	default:
		return
	}
	if _, err := exec.LookPath(name); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

func workspaceReadTool(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

// Middle-click prefers PRIMARY and falls back to the normal clipboard on
// systems without a PRIMARY selection (including native Windows consoles).
var workspaceClipboardRead = func(primary bool) (string, error) {
	if primary {
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-paste"); err == nil {
				if value, err := workspaceReadTool("wl-paste", "--primary", "--no-newline"); err == nil {
					return value, nil
				}
			}
		}
		if os.Getenv("DISPLAY") != "" {
			if _, err := exec.LookPath("xclip"); err == nil {
				if value, err := workspaceReadTool("xclip", "-out", "-selection", "primary"); err == nil {
					return value, nil
				}
			}
			if _, err := exec.LookPath("xsel"); err == nil {
				if value, err := workspaceReadTool("xsel", "--output", "--primary"); err == nil {
					return value, nil
				}
			}
		}
	}
	return clipboard.ReadAll()
}

var workspaceClipboardWrite = func(text string) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			if err := exec.Command("wl-copy", "--", text).Run(); err == nil {
				workspaceWritePrimary(text)
				return nil
			}
		}
	}
	if err := clipboard.WriteAll(text); err == nil {
		workspaceWritePrimary(text)
		return nil
	} else if tty, openErr := os.OpenFile("/dev/tty", os.O_WRONLY, 0); openErr == nil {
		defer tty.Close()
		// OSC 52 is supported by many terminal emulators, and works even
		// without xclip/xsel. Keep the payload bounded for terminal safety.
		if len(text) <= 100*1024 {
			_, writeErr := fmt.Fprintf(tty, "\x1b]52;c;%s\a", base64.StdEncoding.EncodeToString([]byte(text)))
			if writeErr == nil {
				workspaceWritePrimary(text)
			}
			return writeErr
		}
		return err
	} else {
		return err
	}
}

func workspaceCopyCmd(text string) tea.Cmd {
	return func() tea.Msg { return workspaceCopiedMsg{err: workspaceClipboardWrite(text)} }
}

func workspacePasteCmd(session *workspacepkg.Session, primary bool) tea.Cmd {
	id := session.ID()
	return func() tea.Msg {
		value, err := workspaceClipboardRead(primary)
		return workspacePastedMsg{id: id, text: value, err: err}
	}
}

func workspaceSessionUpdateCmd(session *workspacepkg.Session) tea.Cmd {
	if session == nil {
		return nil
	}
	// Capture the current update generation before Bubble Tea schedules the
	// command. Otherwise output arriving between Update() and command execution
	// could advance the generation and leave the screen stale until later data.
	updates := session.Updates()
	done := session.Done()
	return func() tea.Msg {
		select {
		case <-updates:
			exited, _ := session.Exited()
			return workspaceSessionUpdateMsg{id: session.ID(), done: exited}
		case <-done:
			return workspaceSessionUpdateMsg{id: session.ID(), done: true}
		}
	}
}

func (m *tuiModel) workspaceTerminalSize() (int, int) {
	width := max(20, m.width)
	// One row for tabs and one for sshkeeper's workspace help/status line.
	height := max(5, m.height-2)
	return width, height
}

func (m *tuiModel) activeWorkspaceSession() *workspacepkg.Session {
	index := m.workspaceActive - 1
	if index < 0 || index >= len(m.workspaceSessions) {
		return nil
	}
	return m.workspaceSessions[index]
}

func (m *tuiModel) workspaceSessionByID(id string) *workspacepkg.Session {
	for _, session := range m.workspaceSessions {
		if session != nil && session.ID() == id {
			return session
		}
	}
	return nil
}

func (m *tuiModel) workspaceWatchCmds() []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.workspaceSessions))
	for _, session := range m.workspaceSessions {
		if session == nil {
			continue
		}
		if exited, _ := session.Exited(); exited {
			continue
		}
		cmds = append(cmds, workspaceSessionUpdateCmd(session))
	}
	return cmds
}

func (m *tuiModel) workspaceSessionCount(alias string) int {
	count := 0
	for _, session := range m.workspaceSessions {
		if session == nil || session.Alias() != alias {
			continue
		}
		if exited, _ := session.Exited(); !exited {
			count++
		}
	}
	return count
}

func (m *tuiModel) workspaceIndexByAlias(alias string) int {
	for index, session := range m.workspaceSessions {
		if session == nil || session.Alias() != alias {
			continue
		}
		exited, _ := session.Exited()
		if !exited {
			return index
		}
	}
	return -1
}

func (m *tuiModel) openWorkspaceSession(serverAlias string) (tea.Model, tea.Cmd) {
	if index := m.workspaceIndexByAlias(serverAlias); index >= 0 {
		m.workspaceActive = index + 1
		if session := m.activeWorkspaceSession(); session != nil {
			width, height := m.workspaceTerminalSize()
			_ = session.Resize(width, height)
		}
		return m, nil
	}
	if OpenWorkspaceSession == nil {
		return m, func() tea.Msg {
			return workspaceOpenedMsg{err: fmt.Errorf("%s", i18n.T("workspace sessions are unavailable", "сессии рабочего пространства недоступны"))}
		}
	}
	width, height := m.workspaceTerminalSize()
	return m, func() tea.Msg {
		session, err := OpenWorkspaceSession(serverAlias, width, height)
		return workspaceOpenedMsg{session: session, err: err}
	}
}

func (m *tuiModel) handleWorkspaceOpened(msg workspaceOpenedMsg) tea.Cmd {
	if msg.err != nil {
		m.err = msg.err
		m.workspaceActive = 0
		return nil
	}
	if msg.session == nil {
		m.err = fmt.Errorf("%s", i18n.T("session launcher returned no session", "запуск сессии не вернул сессию"))
		m.workspaceActive = 0
		return nil
	}
	m.workspaceSessions = append(m.workspaceSessions, msg.session)
	m.workspaceActive = len(m.workspaceSessions)
	m.err = nil
	return workspaceSessionUpdateCmd(msg.session)
}

func (m *tuiModel) closeActiveWorkspaceSession() {
	index := m.workspaceActive - 1
	m.closeWorkspaceSessionAt(index)
}

func (m *tuiModel) closeWorkspaceSessionByID(id string) {
	for i, session := range m.workspaceSessions {
		if session != nil && session.ID() == id {
			m.closeWorkspaceSessionAt(i)
			return
		}
	}
}

func (m *tuiModel) closeWorkspaceSessionAt(index int) {
	if index < 0 || index >= len(m.workspaceSessions) {
		return
	}
	if m.workspaceActive == index+1 {
		m.workspaceSelecting = false
	}
	session := m.workspaceSessions[index]
	if session != nil {
		_ = session.Close()
	}
	m.workspaceSessions = append(m.workspaceSessions[:index], m.workspaceSessions[index+1:]...)
	// Keep focus stable if a background tab exits.
	if m.workspaceActive > index+1 {
		m.workspaceActive--
	}
	if len(m.workspaceSessions) == 0 {
		m.workspaceActive = 0
		return
	}
	if m.workspaceActive > len(m.workspaceSessions) {
		m.workspaceActive = len(m.workspaceSessions)
	}
}

func (m *tuiModel) closeWorkspaceSessions() {
	for _, session := range m.workspaceSessions {
		if session != nil {
			_ = session.Close()
		}
	}
	m.workspaceSessions = nil
	m.workspaceActive = 0
}

// CloseWorkspaceSessions terminates all embedded SSH sessions. It is called on
// a real application exit; temporary TUI restarts preserve the sessions in
// State instead.
func (m *tuiModel) CloseWorkspaceSessions() {
	m.closeWorkspaceSessions()
}

func (m *tuiModel) workspaceNext(delta int) {
	total := len(m.workspaceSessions) + 1 // dashboard + sessions
	if total <= 1 {
		m.workspaceActive = 0
		return
	}
	next := m.workspaceActive + delta
	for next < 0 {
		next += total
	}
	next %= total
	m.workspaceActive = next
	if session := m.activeWorkspaceSession(); session != nil {
		width, height := m.workspaceTerminalSize()
		_ = session.Resize(width, height)
	}
}

func (m *tuiModel) workspaceShortcut(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyCtrlPgUp:
		m.workspaceNext(-1)
		return true
	case tea.KeyCtrlPgDown:
		m.workspaceNext(1)
		return true
	}
	// Bubble Tea 1.3.x loses the Ctrl modifier for PageUp/PageDown when
	// reading native Windows console input. Alt+Left/Right survives that path,
	// so keep it as a cross-platform workspace-cycle shortcut as well.
	if msg.Alt {
		switch msg.Type {
		case tea.KeyLeft:
			m.workspaceNext(-1)
			return true
		case tea.KeyRight:
			m.workspaceNext(1)
			return true
		}
	}
	if msg.Type == tea.KeyRunes && msg.Alt && len(msg.Runes) == 1 {
		switch r := msg.Runes[0]; {
		case r == '0':
			m.workspaceActive = 0
			return true
		case r >= '1' && r <= '9':
			index := int(r - '1')
			if index < len(m.workspaceSessions) {
				m.workspaceActive = index + 1
				if session := m.activeWorkspaceSession(); session != nil {
					width, height := m.workspaceTerminalSize()
					_ = session.Resize(width, height)
				}
			}
			return true
		case r == 'w' || r == 'W':
			if m.workspaceActive > 0 {
				m.closeActiveWorkspaceSession()
			}
			return true
		}
	}
	return false
}

// Bubble Tea 1.x wraps ANSI input in go-localereader on Windows. For a
// bracketed paste sent as UTF-8 by a modern terminal, that decoder can map
// each UTF-8 byte to an individual Latin-1 rune. Recover the original bytes
// only when they form valid multi-byte UTF-8. Native Windows console key
// events (and legitimate single-byte characters) are left untouched.
func workspaceWindowsPasteText(runes []rune) string {
	text := string(runes)
	bytes := make([]byte, len(runes))
	for i, r := range runes {
		if r > 0xff {
			return text
		}
		bytes[i] = byte(r)
	}
	if utf8.Valid(bytes) && utf8.RuneCount(bytes) < len(bytes) {
		return string(bytes)
	}
	return text
}

func (m *tuiModel) updateWorkspaceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.workspaceShortcut(msg) {
		return m, nil
	}
	session := m.activeWorkspaceSession()
	if session == nil {
		m.workspaceActive = 0
		return m, nil
	}
	// The enclosing terminal owns Ctrl+Shift+C/V. Text selected with the
	// mouse is copied on release; Ctrl+Shift+V normally arrives as bracketed
	// paste below. Never steal Alt+C/V from programs in the remote PTY.
	if !msg.Paste && msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		session.ClearSelection()
	}
	if msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown {
		_, pageHeight := m.workspaceTerminalSize()
		delta := max(1, pageHeight-1)
		if msg.Type == tea.KeyPgDown {
			delta = -delta
		}
		if session.Scroll(delta) {
			return m, nil
		}
	}
	if msg.Paste && msg.Type == tea.KeyRunes {
		session.ClearSelection()
		text := string(msg.Runes)
		if runtime.GOOS == "windows" {
			text = workspaceWindowsPasteText(msg.Runes)
		}
		session.Paste(text)
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		mod := vt.KeyMod(0)
		if msg.Alt {
			mod |= vt.ModAlt
		}
		for _, r := range msg.Runes {
			session.SendKey(vt.KeyPressEvent{Code: r, Mod: mod})
		}
		return m, nil
	}
	if event, ok := workspaceVTKey(msg); ok {
		session.SendKey(event)
	}
	return m, nil
}

func workspaceVTKey(msg tea.KeyMsg) (vt.KeyPressEvent, bool) {
	mod := vt.KeyMod(0)
	if msg.Alt {
		mod |= vt.ModAlt
	}
	plain := func(code rune) (vt.KeyPressEvent, bool) {
		return vt.KeyPressEvent{Code: code, Mod: mod}, true
	}
	ctrl := func(code rune) (vt.KeyPressEvent, bool) {
		return vt.KeyPressEvent{Code: code, Mod: mod | vt.ModCtrl}, true
	}

	if msg.Type >= tea.KeyCtrlA && msg.Type <= tea.KeyCtrlZ {
		return ctrl(rune('a' + int(msg.Type-tea.KeyCtrlA)))
	}
	switch msg.Type {
	case tea.KeyCtrlAt:
		return ctrl(vt.KeySpace)
	case tea.KeyCtrlBackslash:
		return ctrl('\\')
	case tea.KeyCtrlCloseBracket:
		return ctrl(']')
	case tea.KeyCtrlCaret:
		return ctrl('^')
	case tea.KeyCtrlUnderscore:
		return ctrl('_')
	case tea.KeyEnter:
		return plain(vt.KeyEnter)
	case tea.KeyTab:
		return plain(vt.KeyTab)
	case tea.KeyShiftTab:
		return vt.KeyPressEvent{Code: vt.KeyTab, Mod: mod | vt.ModShift}, true
	case tea.KeyBackspace:
		return plain(vt.KeyBackspace)
	case tea.KeyEsc:
		return plain(vt.KeyEscape)
	case tea.KeySpace:
		return plain(vt.KeySpace)
	case tea.KeyUp:
		return plain(vt.KeyUp)
	case tea.KeyDown:
		return plain(vt.KeyDown)
	case tea.KeyLeft:
		return plain(vt.KeyLeft)
	case tea.KeyRight:
		return plain(vt.KeyRight)
	case tea.KeyHome:
		return plain(vt.KeyHome)
	case tea.KeyEnd:
		return plain(vt.KeyEnd)
	case tea.KeyPgUp:
		return plain(vt.KeyPgUp)
	case tea.KeyPgDown:
		return plain(vt.KeyPgDown)
	case tea.KeyInsert:
		return plain(vt.KeyInsert)
	case tea.KeyDelete:
		return plain(vt.KeyDelete)
	case tea.KeyCtrlUp:
		return vt.KeyPressEvent{Code: vt.KeyUp, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlDown:
		return vt.KeyPressEvent{Code: vt.KeyDown, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlLeft:
		return vt.KeyPressEvent{Code: vt.KeyLeft, Mod: mod | vt.ModCtrl}, true
	case tea.KeyCtrlRight:
		return vt.KeyPressEvent{Code: vt.KeyRight, Mod: mod | vt.ModCtrl}, true
	case tea.KeyShiftUp:
		return vt.KeyPressEvent{Code: vt.KeyUp, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftDown:
		return vt.KeyPressEvent{Code: vt.KeyDown, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftLeft:
		return vt.KeyPressEvent{Code: vt.KeyLeft, Mod: mod | vt.ModShift}, true
	case tea.KeyShiftRight:
		return vt.KeyPressEvent{Code: vt.KeyRight, Mod: mod | vt.ModShift}, true
	}
	if msg.Type >= tea.KeyF1 && msg.Type <= tea.KeyF20 {
		code := vt.KeyF1 + rune(msg.Type-tea.KeyF1)
		return plain(code)
	}
	return vt.KeyPressEvent{}, false
}

func (m *tuiModel) workspaceTabPlainLabels() []string {
	labels := []string{i18n.T("Servers", "Серверы")}
	for _, session := range m.workspaceSessions {
		if session == nil {
			continue
		}
		label := session.Alias()
		if exited, _ := session.Exited(); exited {
			label += " ×"
		} else {
			label += " ●"
		}
		labels = append(labels, label)
	}
	return labels
}

func (m *tuiModel) renderWorkspaceTabs(width int) string {
	labels := m.workspaceTabPlainLabels()
	parts := make([]string, 0, len(labels))
	for index, label := range labels {
		text := "[" + label + "]"
		if index == m.workspaceActive {
			parts = append(parts, brandStyle.Copy().Bold(true).Render(text))
		} else {
			parts = append(parts, mutedStyle.Render(text))
		}
	}
	return fitLine(strings.Join(parts, " "), width)
}

func (m *tuiModel) workspaceTabAtX(x int) int {
	if x < 0 {
		return -1
	}
	pos := 0
	for index, label := range m.workspaceTabPlainLabels() {
		width := lineWidth("[" + label + "]")
		if x >= pos && x < pos+width {
			return index
		}
		pos += width + 1
	}
	return -1
}

func (m *tuiModel) viewWorkspaceSession() string {
	session := m.activeWorkspaceSession()
	if session == nil {
		m.workspaceActive = 0
		return m.viewServerList()
	}
	width := max(1, m.width-1)
	height := max(1, m.height)
	tabs := m.renderWorkspaceTabs(width)

	exited, exitErr := session.Exited()
	status := i18n.Tf("%s · connected", "%s · подключено", session.Alias())
	if offset := session.ScrollOffset(); offset > 0 {
		status = i18n.Tf("%s · scrollback: %d lines above live", "%s · история: на %d строк выше", session.Alias(), offset)
	}
	if exited {
		status = i18n.Tf("%s · session ended", "%s · сессия завершена", session.Alias())
		if exitErr != nil {
			status += ": " + firstLine(exitErr.Error())
		}
	}
	if m.workspaceCopyNotice != "" {
		status = m.workspaceCopyNotice
	}
	help := renderHelpLine([]helpItem{
		{Key: "Wheel · PgUp/PgDn", Action: i18n.T("scroll output", "листать вывод")},
		{Key: "Ctrl+PgUp/PgDn · Alt+←/→", Action: i18n.T("switch tab", "сменить вкладку")},
		{Key: "Alt+0", Action: i18n.T("servers", "серверы")},
		{Key: "Drag / Ctrl+Shift+V", Action: i18n.T("copy / paste", "копировать / вставить")},
		{Key: "Alt+W", Action: i18n.T("close session", "закрыть сессию")},
	})
	statusLine := fitLine(status+"  "+help, width)

	bodyHeight := max(1, height-2)
	_ = session.Resize(width, bodyHeight)
	body := session.Render()
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	return tabs + "\n" + strings.Join(lines, "\n") + "\n" + statusLine
}

// A repeating event is scheduled only while the pointer is being dragged
// beyond the SSH viewport. Generation tokens invalidate stale timer events.
type workspaceAutoScrollMsg struct {
	id    string
	epoch uint64
}

var workspaceMouseNow = time.Now

func (m *tuiModel) cancelWorkspaceAutoScroll() {
	m.workspaceScrollEpoch++
	m.workspaceScrollDir = 0
}
func (m *tuiModel) workspaceScrollTick(session *workspacepkg.Session) tea.Cmd {
	epoch := m.workspaceScrollEpoch
	id := session.ID()
	return tea.Tick(65*time.Millisecond, func(time.Time) tea.Msg {
		return workspaceAutoScrollMsg{id: id, epoch: epoch}
	})
}
func (m *tuiModel) workspaceEdgeDirection(y int) int {
	if y <= 0 {
		return 1
	}
	if y >= m.height-1 {
		return -1
	}
	return 0
}
func (m *tuiModel) workspaceDragTo(session *workspacepkg.Session, x, y int) tea.Cmd {
	m.workspaceDragMoved = m.workspaceDragMoved || x != m.workspaceStartX || y != m.workspaceStartY
	m.workspaceDragX, m.workspaceDragY = x, y
	dir := m.workspaceEdgeDirection(y)
	session.MoveSelection(x, y-1)
	if dir != m.workspaceScrollDir {
		m.cancelWorkspaceAutoScroll()
		m.workspaceScrollDir = dir
		if dir != 0 {
			session.Scroll(dir)
			session.MoveSelection(x, y-1)
			return m.workspaceScrollTick(session)
		}
	} else if dir != 0 {
		if session.Scroll(dir) {
			session.MoveSelection(x, y-1)
		}
	}
	return nil
}
func (m *tuiModel) handleWorkspaceAutoScroll(msg workspaceAutoScrollMsg) (tea.Model, tea.Cmd) {
	session := m.activeWorkspaceSession()
	if session == nil || session.ID() != msg.id || !m.workspaceSelecting ||
		m.workspaceScrollDir == 0 || m.workspaceScrollEpoch != msg.epoch {
		return m, nil
	}
	if !session.Scroll(m.workspaceScrollDir) {
		return m, nil
	}
	session.MoveSelection(m.workspaceDragX, m.workspaceDragY-1)
	return m, m.workspaceScrollTick(session)
}

func (m *tuiModel) updateWorkspaceMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	session := m.activeWorkspaceSession()
	if session != nil && m.workspaceSelecting {
		switch event.Action {
		case tea.MouseActionMotion:
			return m, m.workspaceDragTo(session, event.X, event.Y)
		case tea.MouseActionRelease:
			m.workspaceSelecting = false
			m.cancelWorkspaceAutoScroll()
			// Wheel events do not release the selection.
			text := session.FinishSelection(event.X, event.Y-1)
			if m.workspaceDragMoved {
				m.workspaceClickCount = 0
			}
			if text != "" {
				return m, workspaceCopyCmd(text)
			}
			return m, nil
		}
	}
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y == 0 {
		if tab := m.workspaceTabAtX(event.X); tab >= 0 {
			m.workspaceActive = tab
			m.cancelWorkspaceAutoScroll()
			m.workspaceSelecting = false
			m.workspaceClickCount = 0
			return m, nil
		}
	}
	if session == nil {
		return m, nil
	}
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonMiddle && event.Y >= 1 && event.Y < m.height-1 {
		return m, workspacePasteCmd(session, true)
	}
	if event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft && event.Y >= 1 && event.Y < m.height-1 {
		now := workspaceMouseNow()
		if !now.Before(m.workspaceLastClick) && now.Sub(m.workspaceLastClick) <= 450*time.Millisecond &&
			m.workspaceLastClickID == session.ID() &&
			event.X >= m.workspaceLastClickX-1 && event.X <= m.workspaceLastClickX+1 && event.Y == m.workspaceLastClickY {
			m.workspaceClickCount++
			if m.workspaceClickCount > 3 {
				m.workspaceClickCount = 1
			}
		} else {
			m.workspaceClickCount = 1
		}
		m.workspaceLastClick = now
		m.workspaceLastClickX, m.workspaceLastClickY = event.X, event.Y
		m.workspaceLastClickID = session.ID()
		m.workspaceStartX, m.workspaceStartY = event.X, event.Y
		m.workspaceDragX, m.workspaceDragY = event.X, event.Y
		m.workspaceDragMoved = false
		m.cancelWorkspaceAutoScroll()
		session.StartSelectionWithClicks(event.X, event.Y-1, m.workspaceClickCount)
		m.workspaceSelecting = true
		m.workspaceCopyNotice = ""
		return m, nil
	}
	switch event.Button {
	case tea.MouseButtonWheelUp:
		session.Scroll(3)
		if m.workspaceSelecting {
			session.MoveSelection(m.workspaceDragX, m.workspaceDragY-1)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		session.Scroll(-3)
		if m.workspaceSelecting {
			session.MoveSelection(m.workspaceDragX, m.workspaceDragY-1)
		}
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) dashboardBodyTop(width int) int {
	header := m.renderDashboardHeader(width)
	if len(m.workspaceSessions) > 0 {
		header = m.renderWorkspaceTabs(width) + "\n" + header
	}
	return displayLineCount(header) + displayLineCount(m.renderDashboardNotification(width))
}

func (m *tuiModel) updateDashboardMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	if m.screen != screenList && m.screen != screenSearch {
		return m, nil
	}
	if event.Y == 0 && event.Action == tea.MouseActionPress && event.Button == tea.MouseButtonLeft {
		if tab := m.workspaceTabAtX(event.X); tab >= 0 {
			m.workspaceActive = tab
			return m, nil
		}
	}
	switch event.Button {
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
		return m, nil
	}
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if classifyTerminal(m.width, m.height) == sizeWide {
		contentWidth := max(1, m.width-1)
		leftWidth := contentWidth * 62 / 100
		rightWidth := contentWidth - leftWidth - 1
		if event.X > leftWidth {
			server := m.selectedServer()
			if server == nil {
				return m, nil
			}
			m.dashboardFocus = 1
			m.detailAction = 0

			// The right panel starts at dashboardBodyTop; its first inner row is
			// one line below the panel title. Clicking an action executes it
			// immediately, while other clicks simply focus the actions panel.
			line := event.Y - m.dashboardBodyTop(contentWidth) - 1
			innerWidth := max(1, rightWidth-2)
			if actionIndex := m.detailActionIndexAtLine(server, innerWidth, line); actionIndex >= 0 {
				actions := m.profileActions(server)
				m.detailAction = actionIndex
				return m.runDetailAction(server, actions[actionIndex])
			}
			return m, nil
		}
	}
	// Map a click in the visible server rows back to the current row. Derive
	// the body start from the renderer itself: renderAppHeader is two terminal
	// rows (title + separator), which the old hand-written counter treated as
	// one and therefore selected the row below the click.
	contentWidth := max(1, m.width-1)
	bodyTop := m.dashboardBodyTop(contentWidth)
	rowY := bodyTop + 2 // panel top border + column header
	if m.filtering() {
		rowY++
	}
	if event.Y < rowY {
		return m, nil
	}

	bodyHeight := m.height - bodyTop - displayLineCount(m.renderListHelp(len(m.selectedServers()), len(m.bgResults) > 0))
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	innerHeight := max(1, bodyHeight-2)
	prefixLines := 1
	if m.filtering() {
		prefixLines++
	}
	rowCapacity := max(0, innerHeight-prefixLines)
	showRange := len(m.rows) > rowCapacity
	if showRange {
		rowCapacity = max(1, rowCapacity-1)
	}
	start, end := visibleServerRange(len(m.rows), m.cursor, rowCapacity)
	index := start + (event.Y - rowY)
	if index >= start && index < end {
		m.cursor = index
	}
	return m, nil
}
