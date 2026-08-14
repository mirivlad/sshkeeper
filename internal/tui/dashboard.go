package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	switch sizeClass {
	case sizeWide:
		leftWidth := width * 62 / 100
		rightWidth := width - leftWidth - 1
		left := m.renderServerPanel(leftWidth, bodyHeight, true)
		right := m.renderSelectedPanel(rightWidth, bodyHeight)
		body = joinPanelColumns(left, leftWidth, right, rightWidth)
	case sizeMedium:
		detailsHeight := 6
		listHeight := bodyHeight - detailsHeight
		if listHeight < 5 {
			listHeight = 5
		}
		body = m.renderServerPanel(width, listHeight, false)
		if selected := m.selectedServer(); selected != nil && listHeight+detailsHeight <= bodyHeight {
			body += "\n" + m.renderCompactSelected(selected, width, detailsHeight-1)
		}
	default:
		body = m.renderServerPanel(width, bodyHeight, false)
	}

	view := header + notification + body
	padding := height - displayLineCount(view) - footerHeight
	if padding > 0 {
		view += strings.Repeat("\n", padding)
	}
	return view + "\n" + footer
}

func (m *tuiModel) renderDashboardNotification(width int) string {
	if m.err != nil {
		return fitLine(errorStyle.Render("Error: "+m.err.Error()), width) + "\n"
	}
	if m.success != "" {
		return fitLine(successStyle.Render(m.success), width) + "\n"
	}
	return ""
}

func (m *tuiModel) renderDashboardHeader(width int) string {
	left := "sshkeeper / Servers"
	vault := "Vault locked"
	if m.vaultUnlocked {
		vault = "Vault unlocked"
	}
	right := fmt.Sprintf("%s · %d profiles", vault, len(m.servers))
	if selected := len(m.selectedServers()); selected > 0 {
		right += fmt.Sprintf(" · %d selected", selected)
	}
	line := left + " " + right
	if lipgloss.Width(left)+lipgloss.Width(right)+1 <= width {
		line = left + strings.Repeat(" ", width-lipgloss.Width(left)-lipgloss.Width(right)) + right
	}
	headerStyle := titleStyle.Copy().MarginLeft(0)
	separatorStyle := helpStyle.Copy().MarginLeft(0)
	return headerStyle.Render(fitLine(line, width)) + "\n" + separatorStyle.Render(strings.Repeat("─", width)) + "\n"
}

func (m *tuiModel) renderServerPanel(width, height int, showTarget bool) string {
	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	lines := make([]string, 0, innerHeight)
	lines = append(lines, listHeaderStyle.Render(fitLine(fmt.Sprintf("%d servers", len(m.servers)), innerWidth)))
	lines = append(lines, m.renderServerColumns(innerWidth, showTarget, nil, true))

	rowCapacity := max(0, innerHeight-len(lines))
	showRange := len(m.servers) > rowCapacity
	if showRange {
		rowCapacity = max(1, rowCapacity-1)
	}
	if len(m.servers) == 0 {
		lines = append(lines, helpStyle.Render(fitLine("No servers yet. Ctrl+A adds the first profile.", innerWidth)))
	} else if rowCapacity > 0 {
		start, end := visibleServerRange(len(m.servers), m.list.Index(), rowCapacity)
		selected := m.selectedServer()
		for _, server := range m.servers[start:end] {
			lines = append(lines, m.renderServerColumns(innerWidth, showTarget, server, selected != nil && server.Alias == selected.Alias))
		}
		if showRange {
			lines = append(lines, dashboardHelp(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.servers))))
		}
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	return renderPanel(width, height, lines)
}

func (m *tuiModel) renderServerColumns(width int, showTarget bool, server *model.Server, selected bool) string {
	marker, name, target, auth, group, status := "", "NAME", "TARGET / ROUTE", "AUTH", "GROUP", "STATUS"
	style := normalStyle
	if server != nil {
		marker = " "
		if selected {
			marker = ">"
			style = selectedRowStyle
		}
		if m.selected[server.Alias] {
			marker = "*"
			if selected {
				marker = ">*"
			}
		}
		name = server.DisplayName
		if name == "" {
			name = server.Alias
		}
		target = fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
		if len(server.Route.Hops) > 0 {
			target = server.Route.DisplaySummary(target)
		}
		auth = authLabel(server.AuthMethod)
		group = server.GroupName
		if group == "" {
			group = "-"
		}
		status = testStatusLabel(server)
	}

	markerWidth, authWidth, groupWidth, statusWidth := 2, 10, 10, 7
	nameWidth := width - markerWidth - authWidth - groupWidth - statusWidth - 4
	if showTarget {
		nameWidth = min(18, max(10, nameWidth/3))
		targetWidth := width - markerWidth - nameWidth - authWidth - groupWidth - statusWidth - 5
		line := padCells(marker, markerWidth) + " " + padCells(name, nameWidth) + " " + padCells(target, targetWidth) + " " + padCells(auth, authWidth) + " " + padCells(group, groupWidth) + " " + padCells(status, statusWidth)
		if server == nil {
			return listHeaderStyle.Render(fitLine(line, width))
		}
		return style.Render(fitLine(line, width))
	}
	line := padCells(marker, markerWidth) + " " + padCells(name, nameWidth) + " " + padCells(auth, authWidth) + " " + padCells(group, groupWidth) + " " + padCells(status, statusWidth)
	if server == nil {
		return listHeaderStyle.Render(fitLine(line, width))
	}
	return style.Render(fitLine(line, width))
}

func (m *tuiModel) renderSelectedPanel(width, height int) string {
	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	lines := make([]string, 0, innerHeight)
	selected := m.selectedServer()
	if selected == nil {
		lines = append(lines, dashboardSection("Selected profile"), dashboardHelp("No profile selected."))
	} else {
		target := fmt.Sprintf("%s@%s:%d", selected.User, selected.Host, selected.Port)
		route := "direct"
		if len(selected.Route.Hops) > 0 {
			route = selected.Route.DisplaySummary(target)
		}
		group := selected.GroupName
		if group == "" {
			group = "-"
		}
		lines = append(lines,
			dashboardSection("Selected profile"),
			fitLine("Alias: "+selected.Alias, innerWidth),
			fitLine("Display Name: "+selected.DisplayName, innerWidth),
			fitLine("Host: "+selected.Host, innerWidth),
			fitLine(fmt.Sprintf("Port: %d  User: %s", selected.Port, selected.User), innerWidth),
			fitLine(target, innerWidth),
			fitLine("Route      "+route, innerWidth),
			fitLine("Group      "+group, innerWidth),
			fitLine("Tags       "+strings.Join(selected.Tags, ", "), innerWidth),
			fitLine("Last test  "+testStatusLabel(selected), innerWidth),
			"",
			dashboardSection("Primary actions"),
			"Enter      Connect",
			"Ctrl+X     More actions…",
		)
		lines = append(lines, m.backgroundPanelLines(selected.Alias, innerWidth)...)
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	return renderPanel(width, height, lines)
}

func (m *tuiModel) backgroundPanelLines(alias string, width int) []string {
	if len(m.bgResults) == 0 {
		return nil
	}
	lines := []string{"", dashboardSection("Last Background Run")}
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
			lines = append(lines, dashboardHelp("Output: "+result.Alias))
			for _, line := range strings.Split(output, "\n") {
				lines = append(lines, fitLine(strings.ReplaceAll(line, "\t", "    "), width))
			}
		}
	}
	return lines
}

func (m *tuiModel) renderCompactSelected(server *model.Server, width, height int) string {
	target := fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
	group := server.GroupName
	if group == "" {
		group = "-"
	}
	lines := []string{
		dashboardSection("Selected profile"),
		fitLine("Alias: "+server.Alias+"  Target: "+target, width),
		fitLine("Auth: "+authLabel(server.AuthMethod)+"  Group: "+group+"  Status: "+testStatusLabel(server), width),
		fitLine("Enter: Connect  Ctrl+X: More actions…", width),
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func dashboardSection(value string) string {
	return sectionStyle.Copy().MarginTop(0).Render(value)
}

func dashboardHelp(value string) string {
	return helpStyle.Copy().MarginLeft(0).Render(value)
}

func renderPanel(width, height int, lines []string) string {
	if width < 2 || height < 2 {
		return ""
	}
	innerWidth := width - 2
	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", innerWidth) + "┐\n")
	for row := 0; row < height-2; row++ {
		line := ""
		if row < len(lines) {
			line = lines[row]
		}
		b.WriteString("│" + padCells(line, innerWidth) + "│\n")
	}
	b.WriteString("└" + strings.Repeat("─", innerWidth) + "┘")
	return b.String()
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
