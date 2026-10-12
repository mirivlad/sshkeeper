package workspace

import (
	"strings"
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
)

type selectionPoint struct{ x, row int }
type selectionUnit uint8

const (
	selectionCharacter selectionUnit = iota
	selectionWord
	selectionLine
)

// Work in physical terminal cells, including scrollback, rather than
// ANSI-rendered strings. This keeps graphemes, tabs and styled output aligned.
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

// Classify a terminal grapheme as word text, whitespace or punctuation.
func selectionClass(c uv.Cell) int {
	if c.IsZero() || c.Content == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(c.Content)
	if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' {
		return 1
	}
	if unicode.IsSpace(r) {
		return 0
	}
	return 2
}

func (s *Session) selectionSpanLocked(p selectionPoint, mode selectionUnit) (selectionPoint, selectionPoint) {
	first, last := p, p
	if mode == selectionCharacter {
		return first, last
	}
	width := s.emu.Width()
	if mode == selectionLine {
		first.x = 0
		last.x = width - 1
		return first, last
	}
	line := s.selectionLineLocked(p.row, s.emu.ScrollbackLen())
	if len(line) == 0 {
		return first, last
	}
	x := p.x
	// A double-width glyph has an empty continuation cell.
	for x > 0 && line[x].Width == 0 {
		x--
	}
	class := selectionClass(line[x])
	left, right := x, x
	for left > 0 {
		prev := left - 1
		for prev > 0 && line[prev].Width == 0 {
			prev--
		}
		if selectionClass(line[prev]) != class {
			break
		}
		left = prev
	}
	for right+1 < len(line) {
		next := right + max(1, line[right].Width)
		if next >= len(line) || selectionClass(line[next]) != class {
			break
		}
		right = next
	}
	first.x = left
	last.x = min(len(line)-1, right+max(1, line[right].Width)-1)
	return first, last
}

// StartSelectionWithClicks implements traditional terminal mouse gestures:
// one click selects by character on drag, two by word, three by line.
func (s *Session) StartSelectionWithClicks(x, y, clicks int) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	p := s.selectionPointLocked(x, y)
	mode := selectionCharacter
	if clicks == 2 {
		mode = selectionWord
	} else if clicks >= 3 {
		mode = selectionLine
	}
	s.selectionUnit = mode
	s.selectionAnchorStart, s.selectionAnchorEnd = s.selectionSpanLocked(p, mode)
	s.selectionStart, s.selectionEnd = s.selectionAnchorStart, s.selectionAnchorEnd
	s.selectionSet = true
	s.selectionDrag = true
}
func (s *Session) StartSelection(x, y int) { s.StartSelectionWithClicks(x, y, 1) }
func (s *Session) moveSelectionLocked(x, y int) {
	p := s.selectionPointLocked(x, y)
	if s.selectionUnit == selectionCharacter {
		s.selectionEnd = p
		return
	}
	first, last := s.selectionSpanLocked(p, s.selectionUnit)
	switch {
	case lessSelection(last, s.selectionAnchorStart):
		s.selectionStart = first
		s.selectionEnd = s.selectionAnchorEnd
	case lessSelection(s.selectionAnchorEnd, first):
		s.selectionStart = s.selectionAnchorStart
		s.selectionEnd = last
	default:
		s.selectionStart = s.selectionAnchorStart
		s.selectionEnd = s.selectionAnchorEnd
	}
}
func lessSelection(a, b selectionPoint) bool {
	return a.row < b.row || (a.row == b.row && a.x < b.x)
}
func (s *Session) MoveSelection(x, y int) {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	if s.selectionDrag {
		s.moveSelectionLocked(x, y)
	}
}
func (s *Session) FinishSelection(x, y int) string {
	s.screenMu.Lock()
	defer s.screenMu.Unlock()
	if !s.selectionDrag {
		return ""
	}
	s.moveSelectionLocked(x, y)
	s.selectionDrag = false
	if s.selectionUnit == selectionCharacter && s.selectionStart == s.selectionEnd {
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
	if !s.selectionSet || (s.selectionUnit == selectionCharacter && s.selectionStart == s.selectionEnd) {
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
	if !s.selectionSet || (s.selectionUnit == selectionCharacter && s.selectionStart == s.selectionEnd) {
		return ""
	}
	start, end := orderedSelection(s.selectionStart, s.selectionEnd)
	sb := s.emu.ScrollbackLen()
	height := s.emu.Height()
	lines := make([]string, 0, max(0, end.row-start.row+1))
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
			}
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
