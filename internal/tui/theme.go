package tui

import "github.com/charmbracelet/lipgloss"

// The palette is a small set of adaptive tokens. Each token has a light and a
// dark terminal value from the 256-color table; lipgloss downsamples them on
// 16-color terminals and drops them entirely under NO_COLOR.
var (
	// colorAccent is the brand color: an amber "key" tone used for the logo,
	// hotkeys, the selection bar, and section titles.
	colorAccent = lipgloss.AdaptiveColor{Light: "130", Dark: "214"}
	colorText   = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "243", Dark: "246"}
	colorFaint  = lipgloss.AdaptiveColor{Light: "250", Dark: "239"}
	// colorSelection is the background of the row under the cursor.
	colorSelection = lipgloss.AdaptiveColor{Light: "254", Dark: "236"}
	colorOK        = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colorFail      = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorWarn      = lipgloss.AdaptiveColor{Light: "136", Dark: "221"}
	colorTunnel    = lipgloss.AdaptiveColor{Light: "30", Dark: "80"}
	colorSession   = lipgloss.AdaptiveColor{Light: "127", Dark: "176"}
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).MarginLeft(2)

	// selectedStyle marks the focused entry in menus and pickers.
	selectedStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSelection).Bold(true)

	normalStyle      = lipgloss.NewStyle().Foreground(colorText)
	matchStyle       = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Underline(true)
	selectedRowStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)
	listHeaderStyle  = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	sectionStyle     = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).MarginTop(1)

	testOKStyle   = lipgloss.NewStyle().Foreground(colorOK).Bold(true)
	testFailStyle = lipgloss.NewStyle().Foreground(colorFail).Bold(true)

	helpStyle     = lipgloss.NewStyle().Foreground(colorMuted).MarginLeft(2)
	hotkeyStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	helpTextStyle = lipgloss.NewStyle().Foreground(colorMuted)

	errorStyle   = lipgloss.NewStyle().Foreground(colorFail).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(colorOK).Bold(true)

	focusedStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	blurredStyle = lipgloss.NewStyle().Foreground(colorMuted)

	brandStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	borderStyle  = lipgloss.NewStyle().Foreground(colorFaint)
	panelTitle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(colorAccent).Background(colorSelection).Bold(true)
	markStyle    = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	vaultOpen    = lipgloss.NewStyle().Foreground(colorOK)
	vaultLocked  = lipgloss.NewStyle().Foreground(colorWarn)
	mutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	spinnerStyle = lipgloss.NewStyle().Foreground(colorAccent)

	stateUnknownStyle = lipgloss.NewStyle().Foreground(colorFaint)
	stateTestingStyle = lipgloss.NewStyle().Foreground(colorWarn)
	stateTunnelStyle  = lipgloss.NewStyle().Foreground(colorTunnel).Bold(true)
	stateSessionStyle = lipgloss.NewStyle().Foreground(colorSession).Bold(true)
	groupHeaderStyle  = lipgloss.NewStyle().Foreground(colorText).Bold(true)
)

// glyphSet holds the symbols the interface draws.
type glyphSet struct {
	logo      string
	ok        string
	fail      string
	unknown   string
	testing   string
	tunnel    string
	session   string
	expanded  string
	collapsed string
	cursor    string
	marked    string
	vaultOpen string
	vaultShut string
	crumb     string
	dot       string
	arrow     string
	ellipsis  string

	topLeft     string
	topRight    string
	bottomLeft  string
	bottomRight string
	horizontal  string
	vertical    string
}

var unicodeGlyphs = glyphSet{
	// The logo is a key drawn with geometric and box-drawing characters,
	// which every terminal font carries.
	logo:      "○━┳┳",
	ok:        "●",
	fail:      "●",
	unknown:   "·",
	testing:   "◌",
	tunnel:    "⇄",
	session:   "▣",
	expanded:  "▾",
	collapsed: "▸",
	cursor:    "▌",
	marked:    "✓",
	vaultOpen: "●",
	vaultShut: "○",
	crumb:     "›",
	dot:       "·",
	arrow:     "→",
	ellipsis:  "…",

	topLeft:     "╭",
	topRight:    "╮",
	bottomLeft:  "╰",
	bottomRight: "╯",
	horizontal:  "─",
	vertical:    "│",
}

var glyphs = unicodeGlyphs
