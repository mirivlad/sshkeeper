package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/i18n"
)

type screenShell struct {
	breadcrumb   string
	status       string
	notification string
	width        int
	height       int
	body         func(width, height int) string
	footer       []helpItem
}

func renderScreenShell(shell screenShell) string {
	width, height := shell.width, shell.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	canvasWidth := max(1, width-1)

	footer := renderHelp(shell.footer, canvasWidth)
	footerLines := splitBlock(footer)
	if len(footerLines) == 0 {
		footerLines = []string{""}
	}
	fixedRows := 2 + len(footerLines)
	notificationLines := []string(nil)
	if shell.notification != "" {
		notificationLines = []string{fitLine(shell.notification, canvasWidth)}
		fixedRows++
	}
	bodyHeight := max(1, height-fixedRows)
	body := ""
	if shell.body != nil {
		body = shell.body(canvasWidth, bodyHeight)
	}
	bodyLines := fitBlock(body, canvasWidth, bodyHeight)

	lines := make([]string, 0, height)
	lines = append(lines, splitBlock(renderAppHeader(canvasWidth, shell.breadcrumb, shell.status))...)
	lines = append(lines, notificationLines...)
	lines = append(lines, bodyLines...)
	for _, line := range footerLines {
		lines = append(lines, fitLine(line, canvasWidth))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderPaddedPanel(width, height int, lines []string) string {
	if width < 4 || height < 2 {
		return ""
	}
	contentWidth := width - 4
	padded := make([]string, 0, len(lines))
	for _, line := range lines {
		padded = append(padded, " "+padCells(line, contentWidth)+" ")
	}
	return renderPanel(width, height, padded)
}

func fitBlock(block string, width, height int) []string {
	lines := splitBlock(block)
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = fitLine(lines[index], width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return lines
}

func splitBlock(block string) []string {
	if block == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(block, "\n"), "\n")
}

func classifyShellContent(contentWidth int) terminalSizeClass {
	terminalWidth := contentWidth + 1
	if terminalWidth >= 100 {
		return sizeWide
	}
	if terminalWidth >= 70 {
		return sizeMedium
	}
	return sizeNarrow
}

// renderAppHeader draws the two header rows shared by every screen: the logo,
// app name, and breadcrumb on the left, status on the right, and a rule.
func renderAppHeader(width int, breadcrumb, status string) string {
	left := brandStyle.Render(glyphs.logo) + " " + brandStyle.Render("sshkeeper")
	if breadcrumb != "" {
		crumb := mutedStyle.Render(" " + glyphs.crumb + " ")
		// Screens build nested breadcrumbs as "Actions / alias".
		parts := strings.Split(breadcrumb, " / ")
		for index, part := range parts {
			left += crumb
			if index == len(parts)-1 {
				left += normalStyle.Copy().Bold(true).Render(part)
			} else {
				left += normalStyle.Render(part)
			}
		}
	}
	line := left
	if status != "" {
		if gap := width - lipgloss.Width(left) - lipgloss.Width(status); gap > 0 {
			line = left + strings.Repeat(" ", gap) + status
		} else {
			line = left + " " + status
		}
	}
	return fitLine(line, width) + "\n" + borderStyle.Render(strings.Repeat(glyphs.horizontal, width))
}

// vaultBadge is the colored vault state shown in every header.
func vaultBadge(unlocked bool) string {
	if unlocked {
		return vaultOpen.Render(glyphs.vaultOpen + " " + i18n.T("Vault unlocked", "Хранилище открыто"))
	}
	return vaultLocked.Render(glyphs.vaultShut + " " + i18n.T("Vault locked", "Хранилище заблокировано"))
}

func shellStatus(vaultUnlocked bool, detail string) string {
	vault := vaultBadge(vaultUnlocked)
	if detail == "" {
		return vault
	}
	return vault + mutedStyle.Render(fmt.Sprintf(" %s %s", glyphs.dot, detail))
}
