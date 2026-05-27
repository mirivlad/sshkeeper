package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var passwordPromptRe = regexp.MustCompile(`(?i)(password|passphrase).*:\s*$`)

func ConnectWithPassword(sshBinary string, args []string, password string) error {
	cmd := exec.Command(sshBinary, args...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start ssh with pty: %w", err)
	}
	defer ptmx.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("set raw terminal: %w", err)
	}

	passwordSent := new(atomic.Bool)
	done := make(chan error, 1)

	go func() {
		buf := make([]byte, 4096)
		var accumulated strings.Builder

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				data := buf[:n]
				accumulated.Write(data)
				os.Stdout.Write(data)

				if !passwordSent.Load() {
					text := accumulated.String()
					if passwordPromptRe.MatchString(text) {
						passwordSent.Store(true)
						time.Sleep(100 * time.Millisecond)
						ptmx.Write([]byte(password + "\r"))
					}
				}

				if accumulated.Len() > 8192 {
					s := accumulated.String()
					accumulated.Reset()
					accumulated.WriteString(s[len(s)-2048:])
				}
			}
			if err != nil {
				if err != io.EOF {
					done <- err
				} else {
					done <- nil
				}
				return
			}
		}
	}()

	stopInput, waitInput, restoreInput, err := forwardInputToPTY(ptmx)
	if err != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		return err
	}

	err = cmd.Wait()
	close(stopInput)
	waitInput()

	if restoreErr := restoreInput(); err == nil && restoreErr != nil {
		err = restoreErr
	}
	if restoreErr := term.Restore(int(os.Stdin.Fd()), oldState); err == nil && restoreErr != nil {
		err = restoreErr
	}
	return err
}

func forwardInputToPTY(ptmx *os.File) (chan struct{}, func(), func() error, error) {
	stdinFD := int(os.Stdin.Fd())
	flags, err := unix.FcntlInt(uintptr(stdinFD), unix.F_GETFL, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get stdin flags: %w", err)
	}
	if err := unix.SetNonblock(stdinFD, true); err != nil {
		return nil, nil, nil, fmt.Errorf("set stdin nonblocking: %w", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-stop:
				return
			default:
			}

			n, readErr := unix.Read(stdinFD, buf)
			if n > 0 {
				if _, writeErr := ptmx.Write(buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr == nil {
				continue
			}
			if readErr == unix.EAGAIN || readErr == unix.EWOULDBLOCK {
				select {
				case <-stop:
					return
				case <-time.After(10 * time.Millisecond):
					continue
				}
			}
			return
		}
	}()

	restore := func() error {
		_, err := unix.FcntlInt(uintptr(stdinFD), unix.F_SETFL, flags)
		if err != nil {
			return fmt.Errorf("restore stdin flags: %w", err)
		}
		return nil
	}

	return stop, wg.Wait, restore, nil
}

// connectWithPasswordAndRead runs SSH through a PTY, sends the password,
// collects all output, and returns it. Used for non-interactive testing.
// Returns (true, output) on success, (false, error) on failure.
func connectWithPasswordAndRead(sshBinary string, args []string, password string, timeoutSec int) (bool, string) {
	cmd := exec.Command(sshBinary, args...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return false, fmt.Sprintf("start ssh with pty: %v", err)
	}
	defer ptmx.Close()

	passwordSent := false
	done := make(chan string, 1)

	// Read from PTY, detect password prompt, collect output
	go func() {
		buf := make([]byte, 4096)
		var accumulated strings.Builder

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				data := buf[:n]
				accumulated.Write(data)

				// Check for password prompt
				if !passwordSent {
					text := accumulated.String()
					if passwordPromptRe.MatchString(text) {
						passwordSent = true
						time.Sleep(100 * time.Millisecond)
						ptmx.Write([]byte(password + "\r"))
						continue
					}
				}

				// Reset accumulated buffer periodically
				if accumulated.Len() > 16384 {
					s := accumulated.String()
					accumulated.Reset()
					accumulated.WriteString(s[len(s)-4096:])
				}
			}
			if err != nil {
				done <- accumulated.String()
				return
			}
		}
	}()

	// Wait for command completion or timeout
	select {
	case output := <-done:
		return true, output
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		cmd.Process.Kill()
		return false, "connection timeout"
	}
}
