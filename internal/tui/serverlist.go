package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// serverRow is one line of the dashboard list. The list is a flat sequence of
// rows so the cursor, scrolling, and rendering share one index space.
// In group order, a header row precedes each group; its server is nil.
type serverRow struct {
	server *model.Server
	// group is the group name for header rows; "" is the ungrouped header.
	group string
	// count is the number of profiles under a header row.
	count int
}

func (r serverRow) isHeader() bool { return r.server == nil }

// serverMatch records why a server matched the live filter: the ranking score
// and the rune positions of the matched characters in its display label.
type serverMatch struct {
	score int
	label []int
}

// Server list orders. The values match config ui.sort.
const (
	sortByName   = "name"
	sortByRecent = "recent"
	sortByGroup  = "group"
)

var sortModes = []string{sortByName, sortByRecent, sortByGroup}

// now is replaced in tests that render relative times.
var now = time.Now

// serverLabel is the name shown in the list: the display name when set,
// otherwise the alias.
func serverLabel(server *model.Server) string {
	if server.DisplayName != "" {
		return server.DisplayName
	}
	return server.Alias
}

// setServers replaces the full profile set and rebuilds the visible rows while
// keeping the cursor on the same profile when it still exists.
func (m *tuiModel) setServers(servers []*model.Server) {
	prefer := ""
	if selected := m.selectedServer(); selected != nil {
		prefer = selected.Alias
	}
	m.servers = servers
	for alias := range m.selected {
		if m.serverByAlias(alias) == nil {
			delete(m.selected, alias)
		}
	}
	m.rebuildServerRows(prefer)
}

func (m *tuiModel) serverByAlias(alias string) *model.Server {
	for _, server := range m.servers {
		if server.Alias == alias {
			return server
		}
	}
	return nil
}

// filterQuery is the live filter text, trimmed.
func (m *tuiModel) filterQuery() string {
	return strings.TrimSpace(m.searchInput.Value())
}

// rebuildServerRows applies the live filter to the profile set and moves the
// cursor to prefer, or to the best match when prefer is not visible.
func (m *tuiModel) rebuildServerRows(prefer string) {
	query := m.filterQuery()
	visible := make([]*model.Server, 0, len(m.servers))
	m.matches = map[string]serverMatch{}
	visible = append(visible, m.servers...)
	sort.SliceStable(visible, func(i, j int) bool {
		return m.serverLess(visible[i], visible[j])
	})
	if query != "" {
		all := visible
		visible = make([]*model.Server, 0, len(all))
		for _, server := range all {
			if match, ok := matchServer(server, query, m.forwardSearchText(server.ID)); ok {
				m.matches[server.Alias] = match
				visible = append(visible, server)
			}
		}
		sort.SliceStable(visible, func(i, j int) bool {
			return m.matches[visible[i].Alias].score > m.matches[visible[j].Alias].score
		})
	}

	rows := make([]serverRow, 0, len(visible))
	if query == "" && m.sortMode == sortByGroup {
		rows = m.groupedRows(visible)
	} else {
		for _, server := range visible {
			rows = append(rows, serverRow{server: server})
		}
	}
	m.rows = rows

	m.cursor = 0
	for index, row := range rows {
		if row.server != nil && row.server.Alias == prefer {
			m.cursor = index
			return
		}
	}
	// A profile hidden in a collapsed group leaves the cursor on its header.
	if server := m.serverByAlias(prefer); server != nil && m.collapsed[server.GroupName] {
		if index := m.headerRow(server.GroupName); index >= 0 {
			m.cursor = index
			return
		}
	}
	m.cursor = m.firstServerRow()
}

// groupedRows inserts a header before each group of the already group-sorted
// profiles and omits the members of collapsed groups.
func (m *tuiModel) groupedRows(servers []*model.Server) []serverRow {
	rows := make([]serverRow, 0, len(servers)+8)
	for start := 0; start < len(servers); {
		group := servers[start].GroupName
		end := start
		for end < len(servers) && strings.EqualFold(servers[end].GroupName, group) {
			end++
		}
		rows = append(rows, serverRow{group: group, count: end - start})
		if !m.collapsed[group] {
			for _, server := range servers[start:end] {
				rows = append(rows, serverRow{server: server})
			}
		}
		start = end
	}
	return rows
}

func (m *tuiModel) headerRow(group string) int {
	for index, row := range m.rows {
		if row.isHeader() && row.group == group {
			return index
		}
	}
	return -1
}

// selectedHeader returns the header row under the cursor.
func (m *tuiModel) selectedHeader() (serverRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) || !m.rows[m.cursor].isHeader() {
		return serverRow{}, false
	}
	return m.rows[m.cursor], true
}

// setGroupCollapsed folds or unfolds a group and keeps the cursor on its
// header.
func (m *tuiModel) setGroupCollapsed(group string, collapsed bool) {
	if m.collapsed == nil {
		m.collapsed = map[string]bool{}
	}
	if collapsed {
		m.collapsed[group] = true
	} else {
		delete(m.collapsed, group)
	}
	m.rebuildServerRows("")
	if index := m.headerRow(group); index >= 0 {
		m.cursor = index
	}
}

// updateGroupFold handles folding keys in group order: Enter or Space toggle
// a header, Left folds the current group, Right unfolds it.
func (m *tuiModel) updateGroupFold(key string) bool {
	if m.sortMode != sortByGroup || m.filterQuery() != "" {
		return false
	}
	header, onHeader := m.selectedHeader()
	switch key {
	case "enter", " ":
		if onHeader {
			m.setGroupCollapsed(header.group, !m.collapsed[header.group])
			return true
		}
	case "left", "h":
		if onHeader {
			m.setGroupCollapsed(header.group, true)
			return true
		}
		if server := m.selectedServer(); server != nil {
			m.setGroupCollapsed(server.GroupName, true)
			return true
		}
	case "right", "l":
		if onHeader {
			m.setGroupCollapsed(header.group, false)
			return true
		}
	}
	return false
}

// serverLess orders profiles by the active sort mode. Ties fall back to the
// label so the order is stable and predictable.
func (m *tuiModel) serverLess(a, b *model.Server) bool {
	switch m.sortMode {
	case sortByRecent:
		switch {
		case a.LastConnectedAt != nil && b.LastConnectedAt != nil:
			if !a.LastConnectedAt.Equal(*b.LastConnectedAt) {
				return a.LastConnectedAt.After(*b.LastConnectedAt)
			}
		case a.LastConnectedAt != nil:
			return true
		case b.LastConnectedAt != nil:
			return false
		}
	case sortByGroup:
		ga, gb := strings.ToLower(a.GroupName), strings.ToLower(b.GroupName)
		if ga != gb {
			// Ungrouped profiles go last.
			if ga == "" || gb == "" {
				return gb == ""
			}
			return ga < gb
		}
	}
	la, lb := strings.ToLower(serverLabel(a)), strings.ToLower(serverLabel(b))
	if la != lb {
		return la < lb
	}
	return a.Alias < b.Alias
}

// cycleSort switches to the next sort mode and returns it.
func (m *tuiModel) cycleSort() string {
	next := sortModes[0]
	for index, mode := range sortModes {
		if mode == m.sortMode {
			next = sortModes[(index+1)%len(sortModes)]
			break
		}
	}
	m.sortMode = next
	m.refreshServerRows()
	return next
}

func sortModeLabel(mode string) string {
	switch mode {
	case sortByRecent:
		return i18n.T("recent", "недавние")
	case sortByGroup:
		return i18n.T("group", "группы")
	default:
		return i18n.T("name", "имя")
	}
}

// relativeAge renders how long ago t was in at most four cells: "now", "5m",
// "3h", "2d", "6w", "1y", or "—" when it never happened.
func relativeAge(t *time.Time) string {
	if t == nil || t.IsZero() {
		return glyphs.none
	}
	age := now().Sub(*t)
	switch {
	case age < time.Minute:
		return i18n.T("now", "<1м")
	case age < time.Hour:
		return i18n.Tf("%dm", "%dм", int(age/time.Minute))
	case age < 24*time.Hour:
		return i18n.Tf("%dh", "%dч", int(age/time.Hour))
	case age < 14*24*time.Hour:
		return i18n.Tf("%dd", "%dд", int(age/(24*time.Hour)))
	case age < 365*24*time.Hour:
		return i18n.Tf("%dw", "%dн", int(age/(7*24*time.Hour)))
	default:
		return i18n.Tf("%dy", "%dг", int(age/(365*24*time.Hour)))
	}
}

func (m *tuiModel) firstServerRow() int {
	for index, row := range m.rows {
		if row.server != nil {
			return index
		}
	}
	return 0
}

// moveCursor moves by delta rows, clamped to the list bounds.
func (m *tuiModel) moveCursor(delta int) {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = min(max(0, m.cursor+delta), len(m.rows)-1)
}

func (m *tuiModel) selectedServer() *model.Server {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor].server
}

func (m *tuiModel) visibleServerCount() int {
	count := 0
	for _, row := range m.rows {
		if row.server != nil {
			count++
		}
	}
	return count
}

// forwardSearchText returns the searchable text of a profile's saved forwards:
// names, descriptions, addresses, and ports.
func (m *tuiModel) forwardSearchText(serverID int64) string {
	forwards := m.forwardIndex[serverID]
	if len(forwards) == 0 {
		return ""
	}
	parts := make([]string, 0, len(forwards)*6)
	for _, fwd := range forwards {
		parts = append(parts, fwd.Name, fwd.Description, fwd.LocalAddr, fwd.RemoteAddr)
		if fwd.LocalPort > 0 {
			parts = append(parts, strconv.Itoa(fwd.LocalPort))
		}
		if fwd.RemotePort > 0 {
			parts = append(parts, strconv.Itoa(fwd.RemotePort))
		}
	}
	return strings.Join(parts, " ")
}

func (m *tuiModel) setForwardIndex(forwards []*model.Forward) {
	index := map[int64][]*model.Forward{}
	for _, fwd := range forwards {
		index[fwd.ServerID] = append(index[fwd.ServerID], fwd)
	}
	m.forwardIndex = index
}

// matchServer checks every whitespace-separated token of query against the
// profile. A token must match somewhere for the profile to match. Matches in
// the visible label rank highest, then the alias, then a fuzzy subsequence of
// the label (three or more characters), then any other metadata: host, user,
// group, tags, notes, route, and forwards.
func matchServer(server *model.Server, query, extra string) (serverMatch, bool) {
	label := []rune(serverLabel(server))
	lowerLabel := lowerRunes(label)
	alias := strings.ToLower(server.Alias)
	haystack := strings.ToLower(strings.Join([]string{
		serverItem{server: server}.FilterValue(),
		fmt.Sprintf("%s@%s:%d", server.User, server.Host, server.Port),
		extra,
	}, " "))

	result := serverMatch{}
	hits := map[int]bool{}
	for _, token := range strings.Fields(strings.ToLower(query)) {
		tokenRunes := []rune(token)
		if at := indexRunes(lowerLabel, tokenRunes); at >= 0 {
			result.score += 100
			if at == 0 {
				result.score += 50
			}
			for offset := range tokenRunes {
				hits[at+offset] = true
			}
			continue
		}
		if at := strings.Index(alias, token); at >= 0 {
			result.score += 80
			if at == 0 {
				result.score += 40
			}
			continue
		}
		// Short tokens scatter across almost any name, so fuzzy matching
		// starts at three characters.
		if positions, ok := subsequenceRunes(lowerLabel, tokenRunes); ok && len(tokenRunes) >= 3 {
			result.score += 40 - min(30, spread(positions))
			for _, position := range positions {
				hits[position] = true
			}
			continue
		}
		if strings.Contains(haystack, token) {
			result.score += 20
			continue
		}
		return serverMatch{}, false
	}
	for position := range hits {
		result.label = append(result.label, position)
	}
	sort.Ints(result.label)
	return result, true
}

func lowerRunes(value []rune) []rune {
	lower := make([]rune, len(value))
	for index, r := range value {
		lower[index] = unicode.ToLower(r)
	}
	return lower
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
outer:
	for start := 0; start+len(needle) <= len(haystack); start++ {
		for offset, r := range needle {
			if haystack[start+offset] != r {
				continue outer
			}
		}
		return start
	}
	return -1
}

// subsequenceRunes finds needle as an in-order, possibly gapped subsequence.
func subsequenceRunes(haystack, needle []rune) ([]int, bool) {
	positions := make([]int, 0, len(needle))
	next := 0
	for index, r := range haystack {
		if next < len(needle) && r == needle[next] {
			positions = append(positions, index)
			next++
		}
	}
	return positions, len(needle) > 0 && next == len(needle)
}

// spread is the number of skipped characters inside a subsequence match; tight
// matches rank above scattered ones.
func spread(positions []int) int {
	if len(positions) < 2 {
		return 0
	}
	return positions[len(positions)-1] - positions[0] + 1 - len(positions)
}
