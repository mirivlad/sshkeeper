package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/i18n"
)

// welcomeArt is a key drawn with box-drawing characters for the first-run
// screen. It uses only glyphs every terminal font carries.
var welcomeArt = []string{
	"╭───╮",
	"│ ○ ├━━━━━━┳━┳━╸",
	"╰───╯      ╹ ╹",
}

// renderWelcome replaces the empty server list on the first run with the
// logo, what the app does, and the three ways to get started.
func (m *tuiModel) renderWelcome(width, height int) string {
	inner := max(1, width-2)
	steps := []helpItem{
		{Key: "i", Action: i18n.T("import hosts from ~/.ssh/config", "импортировать хосты из ~/.ssh/config")},
		{Key: "a", Action: i18n.T("add a server by hand", "добавить сервер вручную")},
		{Key: "?", Action: i18n.T("see every key · Ctrl+H opens the guide", "все клавиши · Ctrl+H — руководство")},
	}
	block := []string{}
	if height >= 14 {
		artWidth := 0
		for _, line := range welcomeArt {
			artWidth = max(artWidth, lipgloss.Width(line))
		}
		// Equal widths keep the art's rows aligned when each is centered.
		for _, line := range welcomeArt {
			block = append(block, brandStyle.Render(padCells(line, artWidth)))
		}
		block = append(block, "")
	}
	block = append(block,
		brandStyle.Render("sshkeeper"),
		mutedStyle.Render(i18n.T("Your SSH profiles, secrets, and tunnels in one place.", "SSH-профили, секреты и туннели в одном месте.")),
		"",
	)
	stepWidth := 0
	for _, step := range steps {
		stepWidth = max(stepWidth, lipgloss.Width(step.Key)+3+lipgloss.Width(step.Action))
	}
	for _, step := range steps {
		line := hotkeyStyle.Render(padCells(step.Key, 1)) + "   " + normalStyle.Render(step.Action)
		block = append(block, line+strings.Repeat(" ", max(0, stepWidth-lipgloss.Width(line))))
	}

	// Center the block: each line by its own width for the art and text,
	// the steps as one left-aligned column.
	lines := make([]string, 0, height-2)
	top := max(0, (height-2-len(block))/2)
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	for _, line := range block {
		pad := max(0, (inner-lipgloss.Width(line))/2)
		lines = append(lines, strings.Repeat(" ", pad)+fitLine(line, inner-pad))
	}
	return renderTitledPanel(width, height, i18n.T("Welcome", "Добро пожаловать"), "", lines)
}
