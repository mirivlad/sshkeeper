package tui

import "github.com/charmbracelet/lipgloss"

// glyphSet holds the symbols the dashboard draws for server state.
type glyphSet struct {
	ok        string
	fail      string
	unknown   string
	testing   string
	tunnel    string
	session   string
	expanded  string
	collapsed string
}

var unicodeGlyphs = glyphSet{
	ok:      "●",
	fail:    "●",
	unknown: "·",
	testing: "◌",
	tunnel:  "⇄",
	session: "▣",

	expanded:  "▾",
	collapsed: "▸",
}

var glyphs = unicodeGlyphs

var (
	stateUnknownStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stateTestingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stateTunnelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	stateSessionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	groupHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)
