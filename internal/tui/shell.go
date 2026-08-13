package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	headerLeft := "sshkeeper"
	if shell.breadcrumb != "" {
		headerLeft += " / " + shell.breadcrumb
	}
	header := headerLeft
	if shell.status != "" {
		if gap := canvasWidth - lipgloss.Width(headerLeft) - lipgloss.Width(shell.status); gap > 0 {
			header = headerLeft + strings.Repeat(" ", gap) + shell.status
		} else {
			header = headerLeft + " " + shell.status
		}
	}

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
	lines = append(lines,
		titleStyle.Copy().MarginLeft(0).Render(fitLine(header, canvasWidth)),
		helpStyle.Copy().MarginLeft(0).Render(strings.Repeat("─", canvasWidth)),
	)
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

func shellStatus(vaultUnlocked bool, detail string) string {
	vault := "Vault locked"
	if vaultUnlocked {
		vault = "Vault unlocked"
	}
	if detail == "" {
		return vault
	}
	return fmt.Sprintf("%s · %s", vault, detail)
}
