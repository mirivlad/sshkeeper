package workspace

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

type selectionPoint struct{ x, row int }

// selectionLineLocked returns a copy so styling selection cannot mutate VT history.
func (s *Session) selectionLineLocked(row, scrollbackLength int) uv.Line {
	if row < scrollbackLength {
		line := s.emu.Scrollback().Line(row)
		if line == nil {
			return uv.NewLine(s.emu.Width())
		}
		return append(uv.Line(nil), line...)
	}
	y := row - scrollbackLength
	line := uv.NewLine(s.emu.Width())
	for x := range line {
		if cell := s.emu.CellAt(x, y); cell != nil {
			line[x] = *cell.Clone()
		}
	}
	return line
}

func orderedSelection(a, b selectionPoint) (selectionPoint, selectionPoint) {
	if a.row > b.row || (a.row == b.row && a.x > b.x) {
		return b, a
	}
	return a, b
}

func (s *Session) selectionPointLocked(x, y int) selectionPoint {
	w, h := s.emu.Width(), s.emu.Height()
	x = max(0, min(w-1, x))
	y = max(0, min(h-1, y))
	s.stateMu.RLock()
	offset := s.scrollOffset
	s.stateMu.RUnlock()
	sb := s.emu.ScrollbackLen()
	if s.emu.IsAltScreen() {
		offset = 0
	}
	offset = min(offset, sb)
	return selectionPoint{x: x, row: sb - offset + y}
}

// StartSelection begins a normal left-button selection within the SSH viewport.
func (s *Session) StartSelection(x, y int) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	s.selectionStart = s.selectionPointLocked(x, y)
	s.selectionEnd = s.selectionStart
	s.selectionSet = true
	s.selectionDrag = true
}
func (s *Session) MoveSelection(x, y int) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	if s.selectionDrag {
		s.selectionEnd = s.selectionPointLocked(x, y)
	}
}
func (s *Session) FinishSelection(x, y int) string {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	if !s.selectionDrag {
		return ""
	}
	s.selectionEnd = s.selectionPointLocked(x, y)
	s.selectionDrag = false
	if s.selectionStart == s.selectionEnd {
		s.selectionSet = false
		return ""
	}
	return s.selectedTextLocked()
}
func (s *Session) ClearSelection() {
	s.screenMu.Lock()
	s.selectionDrag = false
	s.selectionSet = false
	s.screenMu.Unlock()
}
func (s *Session) selectionHighlightLocked(line uv.Line, row int) {
	if !s.selectionSet || s.selectionStart == s.selectionEnd {
		return
	}
	start, end := orderedSelection(s.selectionStart, s.selectionEnd)
	if row < start.row || row > end.row {
		return
	}
	from, to := 0, len(line)-1
	if row == start.row {
		from = start.x
	}
	if row == end.row {
		to = end.x
	}
	for x := from; x <= to && x < len(line); x++ {
		if x < 0 {
			continue
		}
		if line[x].IsZero() {
			line[x] = uv.EmptyCell
		}
		line[x].Style.Attrs |= uv.AttrReverse
	}
}
func (s *Session) selectedTextLocked() string {
	if !s.selectionSet || s.selectionStart == s.selectionEnd {
		return ""
	}
	start, end := orderedSelection(s.selectionStart, s.selectionEnd)
	sb := s.emu.ScrollbackLen()
	height := s.emu.Height()
	lines := make([]string, 0, end.row-start.row+1)
	for row := start.row; row <= end.row; row++ {
		if row < 0 || row >= sb+height {
			continue
		}
		cells := s.selectionLineLocked(row, sb)
		first, last := 0, len(cells)-1
		if row == start.row {
			first = start.x
		}
		if row == end.row {
			last = end.x
		}
		var b strings.Builder
		for x := max(0, first); x <= last && x < len(cells); x++ {
			c := cells[x]
			if c.Width == 0 {
				continue
			} // Continuation of a wide grapheme.
			if c.IsZero() || c.Content == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Content)
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return strings.Join(lines, "\n")
}
