package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func (m *tuiModel) renderServerDashboard() string {
	width, height := m.width, m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}
	sizeClass := classifyTerminal(width, height)
	width = max(1, width-1)

	header := m.renderDashboardHeader(width)
	notification := m.renderDashboardNotification(width)
	footer := m.renderListHelp(len(m.selectedServers()), len(m.bgResults) > 0)
	headerHeight := displayLineCount(header)
	notificationHeight := displayLineCount(notification)
	footerHeight := displayLineCount(footer)
	bodyHeight := height - headerHeight - notificationHeight - footerHeight
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	var body string
	if len(m.servers) == 0 {
		body = m.renderWelcome(width, bodyHeight)
	} else {
		body = m.renderDashboardBody(sizeClass, width, bodyHeight)
	}

	view := header + notification + body
	padding := height - displayLineCount(view) - footerHeight
	if padding > 0 {
		view += strings.Repeat("\n", padding)
	}
	return view + "\n" + footer
}

func (m *tuiModel) renderDashboardBody(sizeClass terminalSizeClass, width, bodyHeight int) string {
	switch sizeClass {
	case sizeWide:
		leftWidth := width * 62 / 100
		rightWidth := width - leftWidth - 1
		left := m.renderServerPanel(leftWidth, bodyHeight, true)
		right := m.renderSelectedPanel(rightWidth, bodyHeight)
		return joinPanelColumns(left, leftWidth, right, rightWidth)
	case sizeMedium:
		detailsHeight := 4
		listHeight := max(5, bodyHeight-detailsHeight)
		body := m.renderServerPanel(width, listHeight, true)
		if listHeight+detailsHeight <= bodyHeight {
			body += "\n" + m.renderCompactSelected(width, detailsHeight)
		}
		return body
	default:
		return m.renderServerPanel(width, bodyHeight, false)
	}
}

func (m *tuiModel) renderDashboardNotification(width int) string {
	if m.err != nil {
		return fitLine(errorStyle.Render(i18n.T("Error: ", "Ошибка: ")+m.err.Error()), width) + "\n"
	}
	if m.warning != "" {
		return fitLine(warningStyle.Render(m.warning), width) + "\n"
	}
	if m.success != "" {
		return fitLine(successStyle.Render(m.success), width) + "\n"
	}
	return ""
}

func (m *tuiModel) renderDashboardHeader(width int) string {
	detail := i18n.Tf("%d profiles", "профилей: %d", len(m.servers))
	if selected := len(m.selectedServers()); selected > 0 {
		detail += i18n.Tf(" · %d selected", " · выбрано: %d", selected)
	}
	if testing := len(m.testing); testing > 0 {
		detail += i18n.Tf(" · testing %d…", " · проверка: %d…", testing)
	}
	status := shellStatus(m.vaultUnlocked, detail)
	if badge := m.syncBadge(); badge != "" {
		status = badge + mutedStyle.Render(" "+glyphs.dot+" ") + status
	}
	return renderAppHeader(width, i18n.T("Servers", "Серверы"), status) + "\n"
}

// syncBadge shows sync state in the dashboard header once this device syncs:
// "⇅ 2m", "⇅ syncing…", or "⇅ sync failed".
func (m *tuiModel) syncBadge() string {
	info := m.syncInfo
	if !info.Joined || info.Settings.Mode == SyncModeOff {
		return ""
	}
	switch {
	case m.syncing:
		return stateTestingStyle.Render(glyphs.sync + " " + i18n.T("syncing…", "синхр…"))
	case info.LastError != "":
		return testFailStyle.Render(glyphs.sync + " " + i18n.T("sync failed", "ошибка синхр."))
	case !info.LastSync.IsZero():
		return mutedStyle.Render(glyphs.sync + " " + relativeAge(&info.LastSync))
	}
	return mutedStyle.Render(glyphs.sync)
}

func (m *tuiModel) renderServerPanel(width, height int, showTarget bool) string {
	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	lines := make([]string, 0, innerHeight)
	title := i18n.Tf("%d servers", "Серверов: %d", len(m.servers))
	info := i18n.Tf("s sort: %s", "s сортировка: %s", sortModeLabel(m.sortMode))
	if m.filtering() {
		info = i18n.Tf("%d of %d", "%d из %d", m.visibleServerCount(), len(m.servers))
		lines = append(lines, m.renderFilterPrompt(innerWidth))
	}
	lines = append(lines, m.renderServerColumns(innerWidth, showTarget, nil, true))

	rowCapacity := max(0, innerHeight-len(lines))
	showRange := len(m.rows) > rowCapacity
	if showRange {
		rowCapacity = max(1, rowCapacity-1)
	}
	switch {
	case len(m.servers) == 0:
		lines = append(lines, helpStyle.Render(fitLine(i18n.T("No servers yet. Ctrl+A adds the first profile.", "Серверов пока нет. Ctrl+A добавит первый профиль."), innerWidth)))
	case len(m.rows) == 0:
		lines = append(lines, helpStyle.Render(fitLine(i18n.T("Nothing matches. Esc clears the filter.", "Ничего не найдено. Esc сбрасывает фильтр."), innerWidth)))
	case rowCapacity > 0:
		start, end := visibleServerRange(len(m.rows), m.cursor, rowCapacity)
		for index, row := range m.rows[start:end] {
			if row.isHeader() {
				lines = append(lines, m.renderGroupHeader(innerWidth, row, start+index == m.cursor))
				continue
			}
			lines = append(lines, m.renderServerColumns(innerWidth, showTarget, row.server, start+index == m.cursor))
		}
		if showRange {
			lines = append(lines, dashboardHelp(i18n.Tf("Showing %d-%d of %d", "Показаны %d–%d из %d", start+1, end, len(m.rows))))
		}
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	return renderTitledPanel(width, height, title, info, lines)
}

// filtering reports whether the live filter is being typed or is kept.
func (m *tuiModel) filtering() bool {
	return m.screen == screenSearch || m.filterQuery() != ""
}

// renderGroupHeader draws a foldable group heading: "▾ Production  3".
func (m *tuiModel) renderGroupHeader(width int, row serverRow, selected bool) string {
	fold := glyphs.expanded
	if m.collapsed[row.group] {
		fold = glyphs.collapsed
	}
	name := row.group
	if name == "" {
		name = i18n.T("No group", "Без группы")
	}
	style, marker := groupHeaderStyle, " "
	if selected {
		style, marker = selectedRowStyle, cursorStyle.Render(glyphs.cursor)
	}
	count := layer(style, mutedStyle).Render(fmt.Sprint(row.count))
	text := style.Render(" "+fold+" "+name+"  ") + count
	return marker + text + style.Render(strings.Repeat(" ", max(0, width-1-lipgloss.Width(text))))
}

// renderFilterPrompt is the live filter input line: "/ prod▏".
func (m *tuiModel) renderFilterPrompt(width int) string {
	prompt := "/ " + m.searchInput.Value()
	if m.screen == screenSearch {
		prompt += glyphs.caret
	}
	return brandStyle.Render(fitLine(prompt, width))
}

func (m *tuiModel) renderServerColumns(width int, showTarget bool, server *model.Server, selected bool) string {
	marker, name, target, group, seen, status := "  ", i18n.T("NAME", "ИМЯ"), i18n.T("TARGET", "ЦЕЛЬ"), i18n.T("GROUP", "ГРУППА"), i18n.T("SEEN", "ВХОД"), i18n.T("STATE", "СОСТ")
	style := normalStyle
	var hits []int
	if server != nil {
		if selected {
			style = selectedRowStyle
		}
		name = serverLabel(server)
		hits = m.matches[server.Alias].label
		target = fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
		if len(server.Route.Hops) > 0 {
			target = server.Route.DisplaySummary(target)
		}
		group = server.GroupName
		if group == "" {
			group = "-"
		}
		seen = relativeAge(server.LastConnectedAt)
	}

	// The auth method lives in the details panel; the list keeps the name
	// wide enough to scan.
	markerWidth, groupWidth, seenWidth, statusWidth := 2, 10, 4, 5
	nameWidth := width - markerWidth - groupWidth - seenWidth - statusWidth - 4
	targetWidth := 0
	if showTarget {
		available := nameWidth - 1
		nameWidth = min(24, max(12, available*2/5))
		targetWidth = available - nameWidth
	}
	tail := padCells(group, groupWidth) + " " + padCells(seen, seenWidth) + " "
	if server == nil {
		line := padCells(marker, markerWidth) + " " + padCells(name, nameWidth) + " "
		if showTarget {
			line += padCells(target, targetWidth) + " "
		}
		line += tail + padCells(status, statusWidth)
		return listHeaderStyle.Render(fitLine(line, width))
	}
	parts := []string{
		m.renderRowMarker(server, selected, style),
		highlightCells(name, nameWidth, hits, style, layer(style, matchStyle)),
		style.Render(" "),
	}
	if showTarget {
		parts = append(parts, style.Render(padCells(target, targetWidth)+" "))
	}
	parts = append(parts, style.Render(tail), m.renderServerState(server, style))
	return fitLine(strings.Join(parts, ""), width)
}

// renderRowMarker draws the two leading cells of a row: the accent cursor bar
// and a check for profiles marked with Space, followed by a gap.
func (m *tuiModel) renderRowMarker(server *model.Server, selected bool, style lipgloss.Style) string {
	cursor := style.Render(" ")
	if selected {
		cursor = cursorStyle.Render(glyphs.cursor)
	}
	mark := style.Render(" ")
	if m.selected[server.Alias] {
		mark = layer(style, markStyle).Render(glyphs.marked)
	}
	return cursor + mark + style.Render(" ")
}

// renderServerState draws the five-cell STATE column: the last test result
// (or a test in progress), a running tunnel, and an open tmux session.
func (m *tuiModel) renderServerState(server *model.Server, base lipgloss.Style) string {
	status := layer(base, stateUnknownStyle).Render(glyphs.unknown)
	switch {
	case m.testing[server.Alias]:
		status = layer(base, stateTestingStyle).Render(glyphs.testing)
	case server.LastTestStatus == model.TestOK:
		status = layer(base, testOKStyle).Render(glyphs.ok)
	case server.LastTestStatus == model.TestFailed:
		status = layer(base, testFailStyle).Render(glyphs.fail)
	}
	tunnel := base.Render(" ")
	if m.runtime.tunnels[server.Alias] > 0 {
		tunnel = layer(base, stateTunnelStyle).Render(glyphs.tunnel)
	}
	session := base.Render(" ")
	if m.runtime.sessions[server.Alias] > 0 {
		session = layer(base, stateSessionStyle).Render(glyphs.session)
	}
	gap := base.Render(" ")
	return status + gap + tunnel + gap + session
}

// highlightCells pads value to width and renders the runes at hits with hl.
// Truncation happens on the plain text first so rune positions stay aligned.
func highlightCells(value string, width int, hits []int, base, hl lipgloss.Style) string {
	plain := padCells(value, width)
	if len(hits) == 0 {
		return base.Render(plain)
	}
	marked := make(map[int]bool, len(hits))
	for _, hit := range hits {
		marked[hit] = true
	}
	truncated := []rune(value)
	if lipgloss.Width(value) > width {
		// Keep the "…" that padCells added unhighlighted.
		truncated = []rune(truncateCells(value, width))
		truncated = truncated[:max(0, len(truncated)-1)]
	}
	var b strings.Builder
	var run []rune
	runHL := false
	flush := func() {
		if len(run) == 0 {
			return
		}
		if runHL {
			b.WriteString(hl.Render(string(run)))
		} else {
			b.WriteString(base.Render(string(run)))
		}
		run = run[:0]
	}
	plainRunes := []rune(plain)
	for index, r := range plainRunes {
		isHit := index < len(truncated) && marked[index]
		if isHit != runHL {
			flush()
			runHL = isHit
		}
		run = append(run, r)
	}
	flush()
	return b.String()
}

func (m *tuiModel) backgroundPanelLines(alias string, width int) []string {
	if len(m.bgResults) == 0 {
		return nil
	}
	lines := []string{"", dashboardSection(i18n.T("Last Background Run", "Последний фоновый запуск"))}
	for _, result := range m.bgResults {
		status := "OK"
		if result.Err != "" {
			status = "FAIL"
		}
		lines = append(lines, fitLine(result.Alias+"  "+status, width))
	}
	result := m.backgroundResultForAlias(alias)
	if result == nil && len(m.bgResults) == 1 {
		result = &m.bgResults[0]
	}
	if result != nil {
		output := strings.TrimSpace(result.Output)
		if output == "" {
			output = result.Err
		}
		if output != "" {
			lines = append(lines, dashboardHelp(i18n.T("Output: ", "Вывод: ")+result.Alias))
			for _, line := range strings.Split(output, "\n") {
				lines = append(lines, fitLine(strings.ReplaceAll(line, "\t", "    "), width))
			}
		}
	}
	return lines
}

func dashboardSection(value string) string {
	return sectionStyle.Copy().MarginTop(0).Render(value)
}

func dashboardHelp(value string) string {
	return helpStyle.Copy().MarginLeft(0).Render(value)
}

func renderPanel(width, height int, lines []string) string {
	return renderTitledPanel(width, height, "", "", lines)
}

// renderTitledPanel draws a rounded box. A title sits in the top border on
// the left and optional info on the right: "╭─ Servers ───── sort: name ─╮".
func renderTitledPanel(width, height int, title, info string, lines []string) string {
	if width < 2 || height < 2 {
		return ""
	}
	innerWidth := width - 2
	var b strings.Builder
	b.WriteString(panelTopBorder(innerWidth, title, info) + "\n")
	side := borderStyle.Render(glyphs.vertical)
	for row := 0; row < height-2; row++ {
		line := ""
		if row < len(lines) {
			line = lines[row]
		}
		b.WriteString(side + padCells(line, innerWidth) + side + "\n")
	}
	b.WriteString(borderStyle.Render(glyphs.bottomLeft + strings.Repeat(glyphs.horizontal, innerWidth) + glyphs.bottomRight))
	return b.String()
}

func panelTopBorder(innerWidth int, title, info string) string {
	h := glyphs.horizontal
	if title == "" && info == "" {
		return borderStyle.Render(glyphs.topLeft + strings.Repeat(h, innerWidth) + glyphs.topRight)
	}
	// "─ title " on the left and " info ─" on the right; each needs at least
	// one border cell around it.
	left, right := "", ""
	leftWidth, rightWidth := 0, 0
	if title != "" {
		title = truncateCells(title, max(1, innerWidth-4))
		left = borderStyle.Render(h+" ") + panelTitle.Render(title) + " "
		leftWidth = 3 + lipgloss.Width(title)
	}
	if info != "" && innerWidth-leftWidth-lipgloss.Width(info)-4 >= 1 {
		right = " " + mutedStyle.Render(info) + borderStyle.Render(" "+h)
		rightWidth = 3 + lipgloss.Width(info)
	}
	fill := max(0, innerWidth-leftWidth-rightWidth)
	return borderStyle.Render(glyphs.topLeft) + left + borderStyle.Render(strings.Repeat(h, fill)) + right + borderStyle.Render(glyphs.topRight)
}

func joinPanelColumns(left string, leftWidth int, right string, rightWidth int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	rows := max(len(leftLines), len(rightLines))
	joined := make([]string, rows)
	for row := 0; row < rows; row++ {
		leftLine, rightLine := "", ""
		if row < len(leftLines) {
			leftLine = leftLines[row]
		}
		if row < len(rightLines) {
			rightLine = rightLines[row]
		}
		joined[row] = padCells(leftLine, leftWidth) + " " + padCells(rightLine, rightWidth)
	}
	return strings.Join(joined, "\n")
}
