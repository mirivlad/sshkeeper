package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

var passwordPromptRe = regexp.MustCompile(`(?i)(password|passphrase).*:\s*$`)

// ConnectWithPassword runs SSH through a PTY, detects the password prompt,
// sends the password, and then bridges the user terminal to the SSH session.
func ConnectWithPassword(sshBinary string, args []string, password string) error {
	// Start SSH with PTY
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

	// Save terminal state and set to raw
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("set raw terminal: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Channel to signal when password has been sent
	passwordSent := make(chan bool, 1)
	done := make(chan error, 1)

	// Read from PTY, detect password prompt
	go func() {
		buf := make([]byte, 4096)
		var accumulated strings.Builder

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				data := buf[:n]
				accumulated.Write(data)

				// Write to stdout
				os.Stdout.Write(data)

				// Check for password prompt
				if !<-passwordSent {
					text := accumulated.String()
					if passwordPromptRe.MatchString(text) {
						passwordSent <- true
						time.Sleep(100 * time.Millisecond)
						ptmx.Write([]byte(password + "\r"))
						continue
					}
				}

				// Reset accumulated buffer periodically to avoid unbounded growth
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

	// Copy stdin to PTY
	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	// Wait for command completion
	err = cmd.Wait()
	passwordSent <- false // signal to stop

	return err
}
