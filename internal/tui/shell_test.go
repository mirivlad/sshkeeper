package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestScreenShellFitsAndAnchorsFooter(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {60, 16}} {
		t.Run(itoa(size.width)+"x"+itoa(size.height), func(t *testing.T) {
			view := renderScreenShell(screenShell{
				breadcrumb: "Actions / production-数据库",
				status:     "Vault unlocked",
				width:      size.width,
				height:     size.height,
				body: func(width, height int) string {
					return renderPaddedPanel(width, height, []string{"Actions", "> Connect", "  Manage port forwards"})
				},
				footer: []helpItem{{Key: "Enter", Action: "select"}, {Key: "Ctrl+H", Action: "help"}, {Key: "Esc", Action: "back"}},
			})

			lines := strings.Split(view, "\n")
			if len(lines) != size.height {
				t.Fatalf("shell has %d lines, want %d:\n%s", len(lines), size.height, view)
			}
			if !strings.Contains(ansi.Strip(lines[0]), "sshkeeper / Actions") {
				t.Fatalf("missing breadcrumb header: %q", ansi.Strip(lines[0]))
			}
			if !strings.HasPrefix(ansi.Strip(lines[2]), "┌") || !strings.HasSuffix(ansi.Strip(lines[size.height-2]), "┘") {
				t.Fatalf("body panel does not fill shell:\n%s", view)
			}
			if !strings.Contains(ansi.Strip(lines[size.height-1]), "Ctrl+H") {
				t.Fatalf("footer is not anchored to last row: %q", ansi.Strip(lines[size.height-1]))
			}
			for index, line := range lines {
				if got := ansi.StringWidth(line); got > size.width-1 {
					t.Fatalf("line %d uses unsafe last terminal column: width=%d, terminal=%d", index+1, got, size.width)
				}
			}
		})
	}
}

func TestScreenShellShowsNotificationWithoutMovingFooter(t *testing.T) {
	view := renderScreenShell(screenShell{
		breadcrumb:   "Port Forwards / prod",
		status:       "2 rules",
		notification: "Forward saved",
		width:        60,
		height:       16,
		body: func(width, height int) string {
			return renderPaddedPanel(width, height, []string{"Local PostgreSQL"})
		},
		footer: []helpItem{{Key: "Esc", Action: "back"}},
	})
	lines := strings.Split(view, "\n")
	if len(lines) != 16 || !strings.Contains(view, "Forward saved") || !strings.Contains(ansi.Strip(lines[15]), "Esc") {
		t.Fatalf("notification broke shell geometry:\n%s", view)
	}
}
