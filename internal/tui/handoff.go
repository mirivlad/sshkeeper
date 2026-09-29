package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// HandoffLine is printed when the TUI hands the terminal to ssh:
// "○━┳┳ → Production web · ops@web01:22 via bastion".
func HandoffLine(server *model.Server) string {
	line := brandStyle.Render(glyphs.logo+" "+glyphs.arrow) + " " + normalStyle.Copy().Bold(true).Render(serverLabel(server)) +
		mutedStyle.Render(" "+glyphs.dot+" "+serverTarget(server))
	if len(server.Route.Hops) > 0 {
		hops := make([]string, len(server.Route.Hops))
		for index, hop := range server.Route.Hops {
			hops[index] = hop.DisplayName()
		}
		line += mutedStyle.Render(i18n.T(" via ", " через ") + strings.Join(hops, ", "))
	}
	return asciiOnly(line)
}

// ReturnNotice is the dashboard notice after an ssh session ends cleanly:
// "← Back from Production web · 42m".
func ReturnNotice(server *model.Server, elapsed time.Duration) string {
	return i18n.Tf("← Back from %s · %s", "← Возврат из %s · %s", serverLabel(server), formatElapsed(elapsed))
}

// formatElapsed renders a session length: "12s", "7m", "1h 05m".
func formatElapsed(elapsed time.Duration) string {
	switch {
	case elapsed < time.Minute:
		return i18n.Tf("%ds", "%d с", int(elapsed/time.Second))
	case elapsed < time.Hour:
		return i18n.Tf("%dm", "%d мин", int(elapsed/time.Minute))
	default:
		hours := int(elapsed / time.Hour)
		minutes := int((elapsed % time.Hour) / time.Minute)
		return i18n.Tf("%dh %02dm", "%d ч %02d мин", hours, minutes)
	}
}

// ErrorNotice is the dashboard notice after an ssh session fails.
func ErrorNotice(server *model.Server, err error) string {
	return fmt.Sprintf("%s: %v", i18n.Tf("Connection to %s failed", "Подключение к %s не удалось", serverLabel(server)), err)
}
