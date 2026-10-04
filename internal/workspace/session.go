package workspace

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

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

	screenMu sync.Mutex
	stateMu  sync.RWMutex
	exited   bool
	exitErr  error
	width    int
	height   int
	done     chan struct{}
	once     sync.Once
}

func New(alias string, proc Process, width, height int) *Session {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	s := &Session{
		id:      fmt.Sprintf("session-%d", nextID.Add(1)),
		alias:   alias,
		started: time.Now(),
		proc:    proc,
		emu:     vt.NewEmulator(width, height),
		width:   width,
		height:  height,
		done:    make(chan struct{}),
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

func (s *Session) Render() string {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	return s.emu.Render()
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
	s.screenMu.Unlock()
	return s.proc.Resize(width, height)
}

func (s *Session) SendKey(key vt.KeyPressEvent) {
	s.screenMu.Lock()
	s.emu.SendKey(key)
	s.screenMu.Unlock()
}

func (s *Session) SendText(text string) {
	s.screenMu.Lock()
	s.emu.SendText(text)
	s.screenMu.Unlock()
}

func (s *Session) Paste(text string) {
	s.screenMu.Lock()
	s.emu.Paste(text)
	s.screenMu.Unlock()
}

func (s *Session) Exited() (bool, error) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.exited, s.exitErr
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
	buf := make([]byte, 16384)
	for {
		n, err := s.proc.Read(buf)
		if n > 0 {
			s.screenMu.Lock()
			_, _ = s.emu.Write(buf[:n])
			s.screenMu.Unlock()
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

func (s *Session) waitProcess() {
	err := s.proc.Wait()
	s.stateMu.Lock()
	s.exited = true
	s.exitErr = err
	s.stateMu.Unlock()
	_ = s.proc.Close()
	s.screenMu.Lock()
	_ = s.emu.Close()
	s.screenMu.Unlock()
	s.once.Do(func() { close(s.done) })
}
