package session

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var dedicatedWorkspace = "sshkeeper"

type Window struct {
	ID          string
	Index       int
	Name        string
	ServerAlias string
	Active      bool
	StartedAt   time.Time
}

func Available() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	_, err := exec.LookPath("tmux")
	return err == nil
}
func workspaceTarget() (string, bool, error) {
	if !Available() {
		return "", false, fmt.Errorf("tmux is unavailable")
	}
	if os.Getenv("TMUX") == "" {
		return dedicatedWorkspace, false, nil
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", true, fmt.Errorf("resolve current tmux session: %w", err)
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", true, fmt.Errorf("current tmux session has no name")
	}
	return name, true, nil
}

func sessionExists(target string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", target)
	return cmd.Run() == nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
func Open(serverAlias string) (string, bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("resolve sshkeeper executable: %w", err)
	}
	command := shellQuote(executable) + " __session-connect " + shellQuote(serverAlias)
	return openWindow(serverAlias, command)
}

func openWindow(serverAlias, command string) (string, bool, error) {
	target, insideTmux, err := workspaceTarget()
	if err != nil {
		return "", insideTmux, err
	}
	name := sanitizeWindowName(serverAlias)
	var args []string
	if !insideTmux && !sessionExists(target) {
		args = []string{"new-session", "-d", "-P", "-F", "#{window_id}", "-s", target, "-n", name, command}
	} else {
		args = []string{"new-window", "-d", "-P", "-F", "#{window_id}", "-t", target, "-n", name, command}
	}
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return "", insideTmux, fmt.Errorf("create tmux session window: %s: %w", strings.TrimSpace(string(out)), err)
	}
	windowID := strings.TrimSpace(string(out))
	if windowID == "" {
		return "", insideTmux, fmt.Errorf("tmux did not return a window id")
	}
	if err := setWindowMetadata(windowID, serverAlias, time.Now()); err != nil {
		return "", insideTmux, err
	}
	return windowID, insideTmux, nil
}

func sanitizeWindowName(alias string) string {
	name := strings.TrimSpace(alias)
	if name == "" {
		return "ssh"
	}
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, " ", "-")
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

func setWindowMetadata(windowID, alias string, started time.Time) error {
	pairs := [][2]string{
		{"@sshkeeper_server", alias},
		{"@sshkeeper_started", strconv.FormatInt(started.Unix(), 10)},
	}
	for _, pair := range pairs {
		out, err := exec.Command("tmux", "set-window-option", "-t", windowID, pair[0], pair[1]).CombinedOutput()
		if err != nil {
			return fmt.Errorf("set tmux metadata %s: %s: %w", pair[0], strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func List() ([]Window, error) {
	if !Available() {
		return nil, nil
	}
	target, _, err := workspaceTarget()
	if err != nil {
		return nil, err
	}
	if !sessionExists(target) {
		return nil, nil
	}
	format := "#{window_id}\t#{window_index}\t#{window_name}\t#{window_active}\t#{@sshkeeper_server}\t#{@sshkeeper_started}"
	out, err := exec.Command("tmux", "list-windows", "-t", target, "-F", format).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list tmux windows: %s: %w", strings.TrimSpace(string(out)), err)
	}
	var result []Window
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 || strings.TrimSpace(parts[4]) == "" {
			continue
		}
		index, _ := strconv.Atoi(parts[1])
		startedUnix, _ := strconv.ParseInt(parts[5], 10, 64)
		window := Window{ID: parts[0], Index: index, Name: parts[2], Active: parts[3] == "1", ServerAlias: parts[4]}
		if startedUnix > 0 {
			window.StartedAt = time.Unix(startedUnix, 0)
		}
		result = append(result, window)
	}
	return result, nil
}
func Attach(windowID string) error {
	if !Available() {
		return fmt.Errorf("tmux is unavailable")
	}
	if strings.TrimSpace(windowID) == "" {
		return fmt.Errorf("tmux window id is required")
	}
	if os.Getenv("TMUX") != "" {
		out, err := exec.Command("tmux", "select-window", "-t", windowID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("select tmux window: %s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	target, _, err := workspaceTarget()
	if err != nil {
		return err
	}
	if out, err := exec.Command("tmux", "select-window", "-t", windowID).CombinedOutput(); err != nil {
		return fmt.Errorf("select tmux window: %s: %w", strings.TrimSpace(string(out)), err)
	}
	cmd := exec.Command("tmux", "attach-session", "-t", target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("attach tmux session: %w", err)
	}
	return nil
}

func Close(windowID string) error {
	if !Available() {
		return fmt.Errorf("tmux is unavailable")
	}
	out, err := exec.Command("tmux", "kill-window", "-t", windowID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("close tmux window: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
