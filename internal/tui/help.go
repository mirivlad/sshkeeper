package tui

import (
	"strings"
)

// --- Help rendering utilities ---

func renderHelp(items []helpItem, width int) string {
	if width <= 0 {
		width = 80
	}
	lines := wrapHelpItems(items, width-2)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = "  " + renderHelpLine(line)
	}
	return strings.Join(rendered, "\n")
}

func renderHelpLine(items []helpItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = hotkeyStyle.Render(item.Key) + helpTextStyle.Render(": "+item.Action)
	}
	return strings.Join(parts, helpTextStyle.Render(" | "))
}

func wrapHelpItems(items []helpItem, width int) [][]helpItem {
	if width <= 0 {
		return [][]helpItem{items}
	}
	var lines [][]helpItem
	var current []helpItem
	currentWidth := 0
	for _, item := range items {
		itemWidth := len(plainHelpItem(item))
		if len(current) == 0 {
			current = []helpItem{item}
			currentWidth = itemWidth
			continue
		}
		nextWidth := currentWidth + len(" | ") + itemWidth
		if nextWidth > width {
			lines = append(lines, current)
			current = []helpItem{item}
			currentWidth = itemWidth
			continue
		}
		current = append(current, item)
		currentWidth = nextWidth
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	return lines
}

func plainHelpItem(item helpItem) string {
	return item.Key + ": " + item.Action
}

func plainHelpLine(items []helpItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = plainHelpItem(item)
	}
	return strings.Join(parts, " | ")
}
