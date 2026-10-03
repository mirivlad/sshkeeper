//go:build windows

package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const (
	conPTYDefaultWidth  = 80
	conPTYDefaultHeight = 24
	// Non-interactive runs use a wide pseudo console so command output is
	// not hard-wrapped at 80 columns.
	conPTYReadWidth = 512
	// conPTYModeReset turns off the win32-input and focus-event modes the
	// pseudo console may request from the real terminal, so they do not
	// outlive the ssh session.
	conPTYModeReset = "\x1b[?9001l\x1b[?1004l"
)

// conPTY is ssh.exe attached to a Windows pseudo console (ConPTY): the
// Windows counterpart of the Unix PTY used to type a stored password.
type conPTY struct {
	console windows.Handle
	input   windows.Handle // keystrokes for ssh
	output  windows.Handle // everything ssh draws
	process windows.Handle
}

func startConPTY(sshBinary string, args []string, width, height int) (*conPTY, error) {
	path, err := exec.LookPath(sshBinary)
	if err != nil {
		return nil, fmt.Errorf("find ssh: %w", err)
	}
	appName, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{path}, args...)))
	if err != nil {
		return nil, err
	}

	var inputRead, inputWrite, outputRead, outputWrite windows.Handle
	if err := windows.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		windows.CloseHandle(inputRead)
		windows.CloseHandle(inputWrite)
		return nil, fmt.Errorf("create output pipe: %w", err)
	}
	p := &conPTY{input: inputWrite, output: outputRead}

	var console windows.Handle
	err = windows.CreatePseudoConsole(windows.Coord{X: int16(width), Y: int16(height)}, inputRead, outputWrite, 0, &console)
	// The pseudo console keeps its own duplicates of its pipe ends.
	windows.CloseHandle(inputRead)
	windows.CloseHandle(outputWrite)
	if err != nil {
		p.close()
		return nil, fmt.Errorf("create pseudo console: %w", err)
	}
	p.console = console

	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		p.close()
		return nil, err
	}
	defer attributes.Delete()
	// The attribute value is the pseudo console handle itself.
	if err := attributes.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&console)), unsafe.Sizeof(console)); err != nil {
		p.close()
		return nil, fmt.Errorf("attach pseudo console: %w", err)
	}

	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var info windows.ProcessInformation
	if err := windows.CreateProcess(appName, commandLine, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT, nil, nil, &startup.StartupInfo, &info); err != nil {
		p.close()
		return nil, fmt.Errorf("start %s: %w", path, err)
	}
	windows.CloseHandle(info.Thread)
	p.process = info.Process
	return p, nil
}

func (p *conPTY) Read(b []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(p.output, b, &n, nil)
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) {
		return int(n), io.EOF
	}
	return int(n), err
}

func (p *conPTY) Write(b []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(p.input, b, &n, nil)
	return int(n), err
}

func (p *conPTY) resize(width, height int) error {
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(width), Y: int16(height)})
}

// wait blocks until ssh exits and reports a non-zero exit code as an error,
// like exec.Cmd.Wait.
func (p *conPTY) wait() error {
	if _, err := windows.WaitForSingleObject(p.process, windows.INFINITE); err != nil {
		return fmt.Errorf("wait for ssh: %w", err)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(p.process, &code); err != nil {
		return fmt.Errorf("ssh exit code: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("exit status %d", code)
	}
	return nil
}

func (p *conPTY) kill() {
	_ = windows.TerminateProcess(p.process, 1)
}

// closeConsole ends the pseudo console. Its output pipe reports EOF only
// after the last frame is drained, so a reader must still be running.
func (p *conPTY) closeConsole() {
	if p.console != 0 {
		windows.ClosePseudoConsole(p.console)
		p.console = 0
	}
}

func (p *conPTY) close() {
	p.closeConsole()
	for _, handle := range []*windows.Handle{&p.input, &p.output, &p.process} {
		if *handle != 0 {
			windows.CloseHandle(*handle)
			*handle = 0
		}
	}
}

func ConnectWithPassword(sshBinary string, args []string, password string) error {
	width, height := consoleSize()
	p, err := startConPTY(sshBinary, args, width, height)
	if err != nil {
		return fmt.Errorf("start ssh with pseudo console: %w", err)
	}
	defer p.close()

	restoreConsole, err := prepareConsole()
	if err != nil {
		p.kill()
		_ = p.wait()
		return err
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		var accumulated strings.Builder
		passwordSent := false

		for {
			n, err := p.Read(buf)
			if n > 0 {
				data := buf[:n]
				accumulated.Write(data)
				os.Stdout.Write(data)

				if !passwordSent && passwordPromptRe.MatchString(ansi.Strip(accumulated.String())) {
					passwordSent = true
					time.Sleep(100 * time.Millisecond)
					p.Write([]byte(password + "\r"))
				}

				if accumulated.Len() > 8192 {
					s := accumulated.String()
					accumulated.Reset()
					accumulated.WriteString(s[len(s)-2048:])
				}
			}
			if err != nil {
				return
			}
		}
	}()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		forwardConsoleInput(p, stop)
	}()
	go func() {
		defer wg.Done()
		followConsoleSize(p, width, height, stop)
	}()

	err = p.wait()
	close(stop)
	wg.Wait()
	p.closeConsole()
	<-readDone

	if restoreErr := restoreConsole(); err == nil && restoreErr != nil {
		err = restoreErr
	}
	return err
}

// connectWithPasswordAndRead runs SSH through a pseudo console, sends the
// password, collects all output, and returns it. Used for non-interactive
// testing. Returns (true, output) on success, (false, error) on failure.
func connectWithPasswordAndRead(sshBinary string, args []string, password string, timeoutSec int) (bool, string) {
	p, err := startConPTY(sshBinary, args, conPTYReadWidth, conPTYDefaultHeight)
	if err != nil {
		return false, fmt.Sprintf("start ssh with pseudo console: %v", err)
	}
	defer p.close()

	output := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		var accumulated strings.Builder
		passwordSent := false

		for {
			n, err := p.Read(buf)
			if n > 0 {
				accumulated.Write(buf[:n])

				if !passwordSent && passwordPromptRe.MatchString(ansi.Strip(accumulated.String())) {
					passwordSent = true
					time.Sleep(100 * time.Millisecond)
					p.Write([]byte(password + "\r"))
					continue
				}

				if accumulated.Len() > 16384 {
					s := accumulated.String()
					accumulated.Reset()
					accumulated.WriteString(s[len(s)-4096:])
				}
			}
			if err != nil {
				// The pseudo console redraws with escape sequences; callers
				// expect plain text, as a Unix PTY gives.
				output <- ansi.Strip(accumulated.String())
				return
			}
		}
	}()

	exited := make(chan struct{})
	go func() {
		_ = p.wait()
		close(exited)
	}()

	timedOut := false
	select {
	case <-exited:
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		p.kill()
		<-exited
		timedOut = true
	}
	// Unlike a Unix PTY, the pseudo console keeps its output open after ssh
	// exits; closing it lets the reader finish.
	p.closeConsole()
	text := <-output
	if timedOut {
		return false, "connection timeout"
	}
	return true, text
}

func consoleSize() (int, int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 || height <= 0 {
		return conPTYDefaultWidth, conPTYDefaultHeight
	}
	return width, height
}

// prepareConsole puts the real console into raw VT mode for the session:
// keystrokes reach ssh as VT input, and the pseudo console's VT output is
// rendered rather than printed.
func prepareConsole() (func() error, error) {
	stdin := int(os.Stdin.Fd())
	stdout := windows.Handle(os.Stdout.Fd())

	var inputState *term.State
	if term.IsTerminal(stdin) {
		state, err := term.MakeRaw(stdin)
		if err != nil {
			return nil, fmt.Errorf("set raw terminal: %w", err)
		}
		inputState = state
	}

	var outputMode uint32
	outputIsConsole := windows.GetConsoleMode(stdout, &outputMode) == nil
	if outputIsConsole {
		_ = windows.SetConsoleMode(stdout, outputMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}

	return func() error {
		var err error
		if outputIsConsole {
			os.Stdout.WriteString(conPTYModeReset)
			if setErr := windows.SetConsoleMode(stdout, outputMode); setErr != nil {
				err = fmt.Errorf("restore console output mode: %w", setErr)
			}
		}
		if inputState != nil {
			if restoreErr := term.Restore(stdin, inputState); err == nil && restoreErr != nil {
				err = restoreErr
			}
		}
		return err
	}, nil
}

func followConsoleSize(p *conPTY, width, height int, stop <-chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if w, h := consoleSize(); w != width || h != height {
			if p.resize(w, h) == nil {
				width, height = w, h
			}
		}
	}
}

// forwardConsoleInput copies keystrokes from the real console to ssh until
// stop is closed. It reads only when text is waiting, so no read is left
// pending to swallow a keystroke meant for the TUI after ssh exits.
func forwardConsoleInput(dst io.Writer, stop <-chan struct{}) {
	stdin := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if windows.GetConsoleMode(stdin, &mode) != nil {
		// Redirected stdin cannot be polled; copy it until it ends.
		go io.Copy(dst, os.Stdin)
		return
	}

	units := make([]uint16, 1024)
	var pending []uint16 // a high surrogate still waiting for its pair
	for {
		select {
		case <-stop:
			return
		default:
		}
		event, err := windows.WaitForSingleObject(stdin, 10)
		if err != nil {
			return
		}
		if event != windows.WAIT_OBJECT_0 {
			continue
		}
		ready, err := consoleHasText(stdin)
		if err != nil {
			return
		}
		if !ready {
			continue
		}

		var n uint32
		if err := windows.ReadConsole(stdin, &units[0], uint32(len(units)), &n, nil); err != nil {
			return
		}
		text := append(pending, units[:n]...)
		pending = nil
		if last := len(text) - 1; last >= 0 && text[last] >= 0xD800 && text[last] < 0xDC00 {
			pending = []uint16{text[last]}
			text = text[:last]
		}
		if len(text) > 0 {
			if _, err := dst.Write([]byte(string(utf16.Decode(text)))); err != nil {
				return
			}
		}
	}
}

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procPeekConsoleInputW = kernel32.NewProc("PeekConsoleInputW")
	procReadConsoleInputW = kernel32.NewProc("ReadConsoleInputW")
)

const keyEvent = 0x0001

// inputRecord is INPUT_RECORD with its union read as KEY_EVENT_RECORD.
type inputRecord struct {
	eventType   uint16
	_           uint16
	keyDown     int32
	repeatCount uint16
	virtualKey  uint16
	scanCode    uint16
	char        uint16
	controlKeys uint32
}

// consoleHasText reports whether ReadConsole would return at once. In VT
// input mode the console stores every keystroke, arrows included, as key-down
// records carrying characters. Focus, mouse, resize, key-up and bare modifier
// records also wake the input handle but produce no text; they are dropped so
// ReadConsole never blocks past the end of the session.
func consoleHasText(stdin windows.Handle) (bool, error) {
	var records [32]inputRecord
	var count uint32
	if ok, _, err := procPeekConsoleInputW.Call(uintptr(stdin), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&count))); ok == 0 {
		return false, err
	}
	for _, record := range records[:count] {
		if record.eventType == keyEvent && record.keyDown != 0 && record.char != 0 {
			return true, nil
		}
	}
	if count > 0 {
		if ok, _, err := procReadConsoleInputW.Call(uintptr(stdin), uintptr(unsafe.Pointer(&records[0])), uintptr(count), uintptr(unsafe.Pointer(&count))); ok == 0 {
			return false, err
		}
	}
	return false, nil
}
