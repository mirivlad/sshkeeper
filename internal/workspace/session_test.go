package workspace

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type fakeProcess struct {
	outR *io.PipeReader
	outW *io.PipeWriter

	mu          sync.Mutex
	input       bytes.Buffer
	width       int
	height      int
	resizeCount int
	done        chan struct{}
	once        sync.Once
}

func newFakeProcess() *fakeProcess {
	r, w := io.Pipe()
	return &fakeProcess{outR: r, outW: w, done: make(chan struct{})}
}

func (p *fakeProcess) Read(b []byte) (int, error) { return p.outR.Read(b) }
func (p *fakeProcess) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.Write(b)
}
func (p *fakeProcess) Resize(width, height int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width, p.height = width, height
	p.resizeCount++
	return nil
}
func (p *fakeProcess) Wait() error {
	<-p.done
	return nil
}
func (p *fakeProcess) Kill() error {
	p.once.Do(func() {
		close(p.done)
		_ = p.outW.Close()
	})
	return nil
}
func (p *fakeProcess) Close() error {
	p.once.Do(func() { close(p.done) })
	_ = p.outW.Close()
	return p.outR.Close()
}
func (p *fakeProcess) feed(text string) {
	_, _ = io.WriteString(p.outW, text)
}
func (p *fakeProcess) inputString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.String()
}
func (p *fakeProcess) resizeCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.resizeCount
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func TestSessionKeepsTerminalStateWhileDetachedFromView(t *testing.T) {
	proc := newFakeProcess()
	session := New("web01", proc, 40, 10)
	t.Cleanup(func() { _ = session.Close() })

	proc.feed("\x1b[31mhello workspace\x1b[0m\r\n")
	waitUntil(t, func() bool {
		return strings.Contains(ansi.Strip(session.Render()), "hello workspace")
	})

	proc.feed("still alive\r\n")
	waitUntil(t, func() bool {
		text := ansi.Strip(session.Render())
		return strings.Contains(text, "hello workspace") && strings.Contains(text, "still alive")
	})
}

func TestSessionForwardsTerminalInputAndAvoidsRedundantResize(t *testing.T) {
	proc := newFakeProcess()
	session := New("db01", proc, 80, 24)
	t.Cleanup(func() { _ = session.Close() })

	session.SendText("printf test")
	waitUntil(t, func() bool { return strings.Contains(proc.inputString(), "printf test") })

	if err := session.Resize(80, 24); err != nil {
		t.Fatal(err)
	}
	if got := proc.resizeCalls(); got != 0 {
		t.Fatalf("unchanged size triggered %d process resize(s)", got)
	}
	if err := session.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if got := proc.resizeCalls(); got != 1 {
		t.Fatalf("changed size triggered %d process resize(s), want 1", got)
	}
}

func TestSessionCloseTerminatesProcessAndSignalsDone(t *testing.T) {
	proc := newFakeProcess()
	session := New("router", proc, 40, 10)

	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("session did not report completion")
	}
	if exited, err := session.Exited(); !exited || err != nil {
		t.Fatalf("Exited = %v, %v; want true, nil", exited, err)
	}
}

func TestSessionKeepsFinalScreenAfterProcessExit(t *testing.T) {
	proc := newFakeProcess()
	session := New("final", proc, 40, 10)

	proc.feed("final screen stays\r\n")
	waitUntil(t, func() bool {
		return strings.Contains(ansi.Strip(session.Render()), "final screen stays")
	})
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("session did not exit")
	}
	if got := ansi.Strip(session.Render()); !strings.Contains(got, "final screen stays") {
		t.Fatalf("final terminal screen was lost after exit: %q", got)
	}
}
