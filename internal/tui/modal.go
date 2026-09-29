package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mirivlad/sshkeeper/internal/i18n"
)

// viewConfirm draws the confirmation as a centered dialog over a dimmed copy
// of the screen it interrupts, so the user still sees what they act on.
func (m *tuiModel) viewConfirm() string {
	if m.confirm == nil {
		return ""
	}
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	canvasWidth := max(1, width-1)

	background := m.confirmBackground(height)
	footer := renderHelp([]helpItem{
		{Key: "Tab", Action: i18n.T("choose", "выбрать")},
		{Key: "Enter", Action: i18n.T("activate", "подтвердить")},
		{Key: "Esc", Action: i18n.T("cancel", "отмена")},
	}, canvasWidth)
	footerLines := splitBlock(footer)
	lines := make([]string, height)
	for index := range lines {
		plain := ""
		if index < len(background) {
			plain = ansi.Strip(background[index])
		}
		lines[index] = padCells(plain, canvasWidth)
	}
	// The header stays readable; everything below it is dimmed.
	top := min(2, height)
	bottom := max(top, height-len(footerLines))
	modalWidth := min(64, max(24, canvasWidth-8))
	modal := m.renderConfirmModal(modalWidth, max(5, bottom-top-2))
	modalLines := splitBlock(modal)
	startRow := top + max(0, (bottom-top-len(modalLines))/2)
	startCol := max(0, (canvasWidth-modalWidth)/2)

	for index := range lines {
		switch {
		case index < top:
			lines[index] = fitLine(background[index], canvasWidth)
		case index >= bottom:
			lines[index] = fitLine(footerLines[index-bottom], canvasWidth)
		case index >= startRow && index < startRow+len(modalLines):
			plain := lines[index]
			left := ansi.Cut(plain, 0, startCol)
			right := ansi.Cut(plain, startCol+modalWidth, canvasWidth)
			lines[index] = dimStyle.Render(left) + modalLines[index-startRow] + dimStyle.Render(right)
		default:
			lines[index] = dimStyle.Render(lines[index])
		}
	}
	return strings.Join(lines, "\n")
}

// confirmBackground renders the interrupted screen as rows.
func (m *tuiModel) confirmBackground(height int) []string {
	screen := m.screen
	m.screen = m.confirm.parent
	view := m.View()
	m.screen = screen
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		// The parent could not render; keep a header so the dialog has context.
		lines = splitBlock(renderAppHeader(max(1, m.width-1), i18n.T("Confirm", "Подтверждение"), shellStatus(m.vaultUnlocked, "")))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// renderConfirmModal draws the dialog box. Long targets and consequences are
// clipped so the buttons always stay visible.
func (m *tuiModel) renderConfirmModal(width, maxHeight int) string {
	inner := max(1, width-4)
	message := []string{}
	for _, line := range wrapCells(m.confirm.target, inner) {
		message = append(message, normalStyle.Copy().Bold(true).Render(line))
	}
	if m.confirm.consequence != "" {
		message = append(message, "")
		for _, line := range wrapCells(m.confirm.consequence, inner) {
			message = append(message, mutedStyle.Render(line))
		}
	}
	// Two border rows, a blank row before the buttons, and the buttons.
	if room := maxHeight - 4; len(message) > room {
		message = message[:max(0, room)]
		if len(message) > 0 {
			message[len(message)-1] = truncateCells(strings.TrimSpace(ansi.Strip(message[len(message)-1]))+" …", inner)
		}
	}

	cancel := i18n.T("[ Cancel ]", "[ Отмена ]")
	accept := "[ " + m.confirm.verb + " ]"
	if m.confirm.focus == confirmCancel {
		cancel = selectedStyle.Render("> "+cancel) + " "
		accept = mutedStyle.Render(accept)
	} else {
		cancel = mutedStyle.Render(cancel) + " "
		accept = dangerStyle.Render("> " + accept)
	}
	action := cancel + " " + accept
	if m.confirm.pending {
		action = stateTestingStyle.Render(i18n.Tf("%s in progress…", "%s: выполняется…", m.confirm.verb))
	}

	lines := make([]string, 0, len(message)+2)
	for _, line := range message {
		lines = append(lines, " "+padCells(line, inner)+" ")
	}
	lines = append(lines, "", " "+action)
	return renderTitledPanel(width, len(lines)+2, m.confirm.title, "", lines)
}
