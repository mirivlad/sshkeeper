package workspace

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

var nextID atomic.Uint64

// Process is the PTY/ConPTY transport owned by an embedded workspace session.
type Process interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(width, height int) error
	Wait() error
	Kill() error
	Close() error
}

// Session is one live interactive terminal owned by sshkeeper. The SSH process
// stays alive while another workspace tab is selected.
type Session struct {
	id      string
	alias   string
	started time.Time

	proc Process
	emu  *vt.Emulator

	screenMu             sync.Mutex
	stateMu              sync.RWMutex
	updateMu             sync.Mutex
	exited               bool
	exitErr              error
	width                int
	height               int
	scrollOffset         int
	done                 chan struct{}
	outputDone           chan struct{}
	updates              chan struct{}
	once                 sync.Once
	inputCloseOnce       sync.Once
	selectionStart       selectionPoint // Protected by screenMu.
	selectionEnd         selectionPoint
	selectionSet         bool
	selectionDrag        bool
	selectionUnit        selectionUnit
	selectionAnchorStart selectionPoint
	selectionAnchorEnd   selectionPoint
}

func New(alias string, proc Process, width, height int) *Session {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	s := &Session{
		id:         fmt.Sprintf("session-%d", nextID.Add(1)),
		alias:      alias,
		started:    time.Now(),
		proc:       proc,
		emu:        vt.NewEmulator(width, height),
		width:      width,
		height:     height,
		done:       make(chan struct{}),
		outputDone: make(chan struct{}),
		updates:    make(chan struct{}),
	}
	go s.forwardTerminalInput()
	go s.readProcessOutput()
	go s.waitProcess()
	return s
}

func (s *Session) ID() string            { return s.id }
func (s *Session) Alias() string         { return s.alias }
func (s *Session) StartedAt() time.Time  { return s.started }
func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Updates() <-chan struct{} {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	return s.updates
}

func (s *Session) Render() string {
	s.stateMu.RLock()
	offset := s.scrollOffset
	s.stateMu.RUnlock()

	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	return s.renderLocked(offset)
}

func (s *Session) renderLocked(offset int) string {
	height := s.emu.Height()
	if height < 1 {
		return ""
	}
	if s.emu.IsAltScreen() {
		offset = 0
	}
	sbLen := s.emu.ScrollbackLen()
	if offset < 0 {
		offset = 0
	}
	if offset > sbLen {
		offset = sbLen
	}

	end := sbLen + height - offset
	start := max(0, end-height)
	lines := make([]string, 0, height)
	cursor := s.emu.CursorPosition()
	for row := start; row < end; row++ {
		line := s.selectionLineLocked(row, sbLen)
		y := row - sbLen
		// The VT emulator tracks the remote cursor but Render() only returns
		// cells. Overlay a reverse-video cell while viewing the live screen so
		// full-screen applications such as nano/vim have a visible cursor.
		if row >= sbLen && offset == 0 && y == cursor.Y && cursor.X >= 0 && cursor.X < len(line) {
			if line[cursor.X].IsZero() {
				line[cursor.X] = uv.EmptyCell
			}
			line[cursor.X].Style.Attrs ^= uv.AttrReverse
		}
		s.selectionHighlightLocked(line, row)
		lines = append(lines, line.Render())
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (s *Session) IsAltScreen() bool {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	return s.emu.IsAltScreen()
}

func (s *Session) ScrollOffset() int {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.scrollOffset
}

// Scroll moves the local viewport through the main-screen scrollback. Positive
// delta moves toward older output, negative delta moves back toward live output.
// Alternate-screen applications own PageUp/PageDown themselves and are never
// intercepted here.
func (s *Session) Scroll(delta int) bool {
	if delta == 0 {
		return false
	}
	s.screenMu.Lock()
	if s.emu.IsAltScreen() {
		s.screenMu.Unlock()
		return false
	}
	maxOffset := s.emu.ScrollbackLen()
	s.screenMu.Unlock()
	if maxOffset == 0 {
		return false
	}

	s.stateMu.Lock()
	before := s.scrollOffset
	s.scrollOffset = min(maxOffset, max(0, s.scrollOffset+delta))
	changed := before != s.scrollOffset
	s.stateMu.Unlock()
	return changed
}

func (s *Session) ScrollToBottom() {
	s.stateMu.Lock()
	s.scrollOffset = 0
	s.stateMu.Unlock()
}

func (s *Session) Resize(width, height int) error {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	s.stateMu.Lock()
	if s.width == width && s.height == height {
		s.stateMu.Unlock()
		return nil
	}
	s.width, s.height = width, height
	s.stateMu.Unlock()
	s.screenMu.Lock()
	s.emu.Resize(width, height)
	maxOffset := s.emu.ScrollbackLen()
	s.screenMu.Unlock()
	s.stateMu.Lock()
	if s.scrollOffset > maxOffset {
		s.scrollOffset = maxOffset
	}
	s.stateMu.Unlock()
	return s.proc.Resize(width, height)
}

func (s *Session) SendKey(key vt.KeyPressEvent) {
	s.ScrollToBottom()
	s.screenMu.Lock()
	s.emu.SendKey(key)
	s.screenMu.Unlock()
}

func (s *Session) SendText(text string) {
	s.ScrollToBottom()
	s.screenMu.Lock()
	s.emu.SendText(text)
	s.screenMu.Unlock()
}

func (s *Session) Paste(text string) {
	s.ScrollToBottom()
	s.screenMu.Lock()
	s.emu.Paste(text)
	s.screenMu.Unlock()
}

func (s *Session) Exited() (bool, error) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.exited, s.exitErr
}

func (s *Session) signalUpdate() {
	// Closing the current generation broadcasts the update to every waiter.
	// This is robust across temporary Bubble Tea restarts: an old waiter cannot
	// steal the only notification from the new program. Bursts are naturally
	// coalesced because the next View reads the emulator's latest state.
	s.updateMu.Lock()
	close(s.updates)
	s.updates = make(chan struct{})
	s.updateMu.Unlock()
}

func (s *Session) Close() error {
	exited, _ := s.Exited()
	if !exited {
		_ = s.proc.Kill()
	}
	return nil
}

func (s *Session) forwardTerminalInput() {
	buf := make([]byte, 4096)
	for {
		n, err := s.emu.Read(buf)
		if n > 0 {
			if _, writeErr := s.proc.Write(buf[:n]); writeErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) readProcessOutput() {
	defer close(s.outputDone)
	buf := make([]byte, 16384)
	for {
		n, err := s.proc.Read(buf)
		if n > 0 {
			s.screenMu.Lock()
			beforeScrollback := s.emu.ScrollbackLen()
			_, _ = s.emu.Write(buf[:n])
			afterScrollback := s.emu.ScrollbackLen()
			s.screenMu.Unlock()
			if delta := afterScrollback - beforeScrollback; delta > 0 {
				s.stateMu.Lock()
				if s.scrollOffset > 0 {
					s.scrollOffset = min(afterScrollback, s.scrollOffset+delta)
				}
				s.stateMu.Unlock()
			}
			s.signalUpdate()
		}
		if err != nil {
			if err != io.EOF {
				// Wait() owns the user-visible exit status. PTYs commonly
				// return EIO/BROKEN_PIPE while a process is shutting down.
			}
			return
		}
	}
}

func (s *Session) closeTerminalInput() {
	s.inputCloseOnce.Do(func() {
		if closer, ok := s.emu.InputPipe().(io.Closer); ok {
			_ = closer.Close()
		}
	})
}

func (s *Session) waitProcess() {
	err := s.proc.Wait()
	s.stateMu.Lock()
	s.exited = true
	s.exitErr = err
	s.stateMu.Unlock()
	s.signalUpdate()
	// Stop accepting terminal input now that the child is gone, but keep the
	// emulator screen itself alive for the exited tab.
	s.closeTerminalInput()

	// Unix PTYs normally report EOF/EIO as soon as the child exits, so let the
	// reader drain the final frame before closing the descriptor. ConPTY keeps
	// its output pipe open until the pseudo console is closed; cap the drain
	// window there (and for unusual Unix descendants that inherit the PTY).
	select {
	case <-s.outputDone:
	case <-time.After(150 * time.Millisecond):
	}
	_ = s.proc.Close()
	select {
	case <-s.outputDone:
	case <-time.After(150 * time.Millisecond):
	}

	// Do not call Emulator.Close here: x/vt's Read and Close touch its closed
	// flag without synchronization. The closed input pipe already wakes
	// forwardTerminalInput, while the final terminal screen remains renderable.
	s.once.Do(func() { close(s.done) })
}
