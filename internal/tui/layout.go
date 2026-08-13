package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	minimumTUIWidth  = 60
	minimumTUIHeight = 16
)

type terminalSizeClass int

const (
	sizeBelowFloor terminalSizeClass = iota
	sizeNarrow
	sizeMedium
	sizeWide
)

func classifyTerminal(width, height int) terminalSizeClass {
	if width > 0 && height > 0 && (width < minimumTUIWidth || height < minimumTUIHeight) {
		return sizeBelowFloor
	}
	if width >= 100 {
		return sizeWide
	}
	if width >= 70 {
		return sizeMedium
	}
	return sizeNarrow
}

func truncateCells(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(value, width, "…")
}

func padCells(value string, width int) string {
	value = truncateCells(value, width)
	missing := width - ansi.StringWidth(value)
	if missing > 0 {
		value += strings.Repeat(" ", missing)
	}
	return value
}

func fitLine(value string, width int) string {
	return truncateCells(value, width)
}

func wrapCells(value string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	return strings.Split(ansi.Wrap(value, width, ""), "\n")
}

func minimumSizeView(width int) string {
	message := "sshkeeper needs at least 60x16"
	if width <= 0 {
		return message
	}
	return truncateCells(message, width)
}
