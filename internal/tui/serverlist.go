package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mirivlad/sshkeeper/internal/model"
)

// serverRow is one line of the dashboard list. The list is a flat sequence of
// rows so the cursor, scrolling, and rendering share one index space.
type serverRow struct {
	server *model.Server
}

// serverMatch records why a server matched the live filter: the ranking score
// and the rune positions of the matched characters in its display label.
type serverMatch struct {
	score int
	label []int
}

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
	if query == "" {
		visible = append(visible, m.servers...)
	} else {
		for _, server := range m.servers {
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
	for _, server := range visible {
		rows = append(rows, serverRow{server: server})
	}
	m.rows = rows

	m.cursor = 0
	for index, row := range rows {
		if row.server != nil && row.server.Alias == prefer {
			m.cursor = index
			return
		}
	}
	m.cursor = m.firstServerRow()
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
