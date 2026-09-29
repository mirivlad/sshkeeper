package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// detailLabelWidth aligns the label column of the details panel.
const detailLabelWidth = 9

// renderSelectedPanel is the right-hand details panel of the wide dashboard.
// It shows what the list cannot: how the connection travels, what forwards
// exist and whether they run, when the profile was last used, and notes.
func (m *tuiModel) renderSelectedPanel(width, height int) string {
	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	title, info := i18n.T("Selected profile", "Выбранный профиль"), ""
	var lines []string
	if header, ok := m.selectedHeader(); ok {
		title = groupTitle(header.group)
		lines = m.groupDetailLines(header, innerWidth)
	} else if server := m.selectedServer(); server != nil {
		title, info = serverLabel(server), authLabel(server.AuthMethod)
		lines = m.serverDetailLines(server, innerWidth)
		lines = append(lines, m.backgroundPanelLines(server.Alias, innerWidth)...)
	} else {
		lines = []string{dashboardHelp(i18n.T("No profile selected.", "Профиль не выбран."))}
	}
	padded := make([]string, 0, innerHeight)
	for _, line := range lines {
		padded = append(padded, " "+fitLine(line, innerWidth-2))
	}
	for len(padded) < innerHeight {
		padded = append(padded, "")
	}
	if len(padded) > innerHeight {
		padded = padded[:innerHeight]
	}
	return renderTitledPanel(width, height, title, info, padded)
}

func (m *tuiModel) serverDetailLines(server *model.Server, width int) []string {
	lines := []string{
		normalStyle.Copy().Bold(true).Render(serverTarget(server)),
		m.routeChain(server),
		"",
		detailRow(i18n.T("Alias", "Псевдоним"), server.Alias),
	}
	if server.GroupName != "" {
		lines = append(lines, detailRow(i18n.T("Group", "Группа"), server.GroupName))
	}
	if len(server.Tags) > 0 {
		lines = append(lines, detailRow(i18n.T("Tags", "Теги"), strings.Join(server.Tags, ", ")))
	}
	lines = append(lines,
		detailRow(i18n.T("Last in", "Вход"), ageLong(server.LastConnectedAt)),
		detailRow(i18n.T("Test", "Проверка"), m.testSummaryLine(server)),
	)
	if count := m.runtime.sessions[server.Alias]; count > 0 {
		lines = append(lines, detailRow(i18n.T("Sessions", "Сессии"), stateSessionStyle.Render(glyphs.session)+" "+i18n.Tf("%d open in tmux", "открыто в tmux: %d", count)))
	}

	if forwards := m.forwardIndex[server.ID]; len(forwards) > 0 {
		heading := i18n.T("Forwards", "Пробросы")
		if m.runtime.tunnels[server.Alias] > 0 {
			heading += "  " + stateTunnelStyle.Render(glyphs.tunnel+" "+i18n.T("tunnel running", "туннель работает"))
		}
		lines = append(lines, "", dashboardSection(heading))
		for _, fwd := range forwards {
			lines = append(lines, forwardLine(fwd))
		}
	}
	if notes := strings.TrimSpace(server.Notes); notes != "" {
		lines = append(lines, "", dashboardSection(i18n.T("Notes", "Заметки")))
		for _, line := range wrapCells(notes, max(1, width-2)) {
			lines = append(lines, mutedStyle.Render(line))
		}
	}
	lines = append(lines, "", renderHelpLine([]helpItem{
		{Key: "Enter", Action: i18n.T("connect", "подключиться")},
		{Key: "x", Action: i18n.T("actions", "действия")},
	}))
	return lines
}

func (m *tuiModel) groupDetailLines(header serverRow, width int) []string {
	ok, failed, tunnels := 0, 0, 0
	for _, server := range m.servers {
		if !strings.EqualFold(server.GroupName, header.group) {
			continue
		}
		switch server.LastTestStatus {
		case model.TestOK:
			ok++
		case model.TestFailed:
			failed++
		}
		tunnels += m.runtime.tunnels[server.Alias]
	}
	lines := []string{
		i18n.Tf("%d profiles", "Профилей: %d", header.count),
		"",
		detailRow(i18n.T("Test OK", "Доступны"), testOKStyle.Render(glyphs.ok)+" "+strconv.Itoa(ok)),
		detailRow(i18n.T("Failed", "Ошибки"), testFailStyle.Render(glyphs.fail)+" "+strconv.Itoa(failed)),
	}
	if tunnels > 0 {
		lines = append(lines, detailRow(i18n.T("Tunnels", "Туннели"), stateTunnelStyle.Render(glyphs.tunnel)+" "+strconv.Itoa(tunnels)))
	}
	fold := i18n.T("fold", "свернуть")
	if m.collapsed[header.group] {
		fold = i18n.T("unfold", "развернуть")
	}
	lines = append(lines, "", renderHelpLine([]helpItem{{Key: "Enter", Action: fold}, {Key: "T", Action: i18n.T("test all", "проверить все")}}))
	return lines
}

func groupTitle(group string) string {
	if group == "" {
		return i18n.T("No group", "Без группы")
	}
	return group
}

func detailRow(label, value string) string {
	return mutedStyle.Render(padCells(label, detailLabelWidth)) + " " + value
}

func serverTarget(server *model.Server) string {
	return fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port)
}

// routeChain draws how the connection travels: "you → bastion → web01".
func (m *tuiModel) routeChain(server *model.Server) string {
	arrow := mutedStyle.Render(" " + glyphs.arrow + " ")
	parts := []string{mutedStyle.Render(i18n.T("you", "вы"))}
	for _, hop := range server.Route.Hops {
		name := hop.DisplayName()
		if hop.Profile() {
			parts = append(parts, normalStyle.Render(name))
		} else {
			parts = append(parts, mutedStyle.Render(name))
		}
	}
	if len(server.Route.Hops) == 0 && server.ProxyJump != "" {
		parts = append(parts, mutedStyle.Render(server.ProxyJump))
	}
	parts = append(parts, brandStyle.Render(server.Host))
	return strings.Join(parts, arrow)
}

func (m *tuiModel) testSummaryLine(server *model.Server) string {
	if m.testing[server.Alias] {
		return stateTestingStyle.Render(glyphs.testing + " " + i18n.T("testing…", "проверка…"))
	}
	when := ""
	if server.LastTestAt != nil {
		when = mutedStyle.Render(" " + glyphs.dot + " " + ageLong(server.LastTestAt))
	}
	switch server.LastTestStatus {
	case model.TestOK:
		return testOKStyle.Render(glyphs.ok+" OK") + when
	case model.TestFailed:
		line := testFailStyle.Render(glyphs.fail+" "+i18n.T("failed", "ошибка")) + when
		if reason := firstLine(server.LastTestError); reason != "" {
			line += mutedStyle.Render(": " + reason)
		}
		return line
	default:
		return mutedStyle.Render(glyphs.unknown + " " + i18n.T("not tested · t tests", "не проверялся · t проверит"))
	}
}

// forwardLine renders one saved forward compactly, e.g.
// "● L :15432 → 127.0.0.1:5432  Local PostgreSQL".
func forwardLine(fwd *model.Forward) string {
	state := stateUnknownStyle.Render(glyphs.off)
	if fwd.Enabled {
		state = testOKStyle.Render(glyphs.on)
	}
	listen := listenAddr(fwd.LocalAddr, fwd.LocalPort)
	var rule string
	switch fwd.Type {
	case model.ForwardRemote:
		rule = "R " + listenAddr(fwd.RemoteAddr, fwd.RemotePort) + " " + glyphs.arrow + " " + fmt.Sprintf("%s:%d", fwd.LocalAddr, fwd.LocalPort)
	case model.ForwardDynamic:
		rule = "D " + listen + " SOCKS"
	default:
		rule = "L " + listen + " " + glyphs.arrow + " " + fmt.Sprintf("%s:%d", fwd.RemoteAddr, fwd.RemotePort)
	}
	line := state + " " + normalStyle.Render(rule)
	if fwd.Name != "" {
		line += mutedStyle.Render("  " + fwd.Name)
	}
	return line
}

// listenAddr hides the default loopback bind address: ":15432".
func listenAddr(addr string, port int) string {
	if addr == "" || addr == "127.0.0.1" || addr == "localhost" {
		return fmt.Sprintf(":%d", port)
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

// ageLong renders a past time for the details panel: "2h ago", "never".
func ageLong(t *time.Time) string {
	if t == nil || t.IsZero() {
		return i18n.T("never", "никогда")
	}
	short := relativeAge(t)
	if short == i18n.T("now", "<1м") {
		return i18n.T("just now", "только что")
	}
	return i18n.Tf("%s ago", "%s назад", short)
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	return value
}

// renderCompactSelected is the details strip under the list on medium
// terminals: target and route, key facts, and the main hints.
func (m *tuiModel) renderCompactSelected(width, height int) string {
	innerWidth := max(1, width-2)
	title, info := i18n.T("Selected profile", "Выбранный профиль"), ""
	var lines []string
	if header, ok := m.selectedHeader(); ok {
		title = groupTitle(header.group)
		lines = []string{i18n.Tf("%d profiles", "Профилей: %d", header.count)}
	} else if server := m.selectedServer(); server != nil {
		title, info = serverLabel(server), authLabel(server.AuthMethod)
		// The test line goes last: a failure reason can be long.
		facts := []string{mutedStyle.Render(i18n.T("last in ", "вход ") + ageLong(server.LastConnectedAt))}
		if forwards := m.forwardIndex[server.ID]; len(forwards) > 0 {
			facts = append(facts, mutedStyle.Render(i18n.Tf("%d forwards", "пробросов: %d", len(forwards))))
		}
		facts = append(facts, m.testSummaryLine(server))
		lines = []string{
			normalStyle.Copy().Bold(true).Render(serverTarget(server)) + "  " + m.routeChain(server),
			strings.Join(facts, mutedStyle.Render("  "+glyphs.dot+"  ")),
		}
	}
	padded := make([]string, 0, height)
	for _, line := range lines {
		padded = append(padded, " "+fitLine(line, innerWidth-2))
	}
	return renderTitledPanel(width, height, title, info, padded)
}
