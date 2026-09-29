package tui

import (
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// runBatch executes cmd and every nested batch command, returning the leaf
// messages. It keeps tests independent of goroutine scheduling.
func runBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		msgs []tea.Msg
	)
	for _, child := range batch {
		wg.Add(1)
		go func(child tea.Cmd) {
			defer wg.Done()
			result := runBatch(child)
			mu.Lock()
			msgs = append(msgs, result...)
			mu.Unlock()
		}(child)
	}
	wg.Wait()
	return msgs
}

func TestTestAllChecksVisibleServersAndSummarizes(t *testing.T) {
	previousTest, previousUpdate := TestConnection, UpdateTestResult
	t.Cleanup(func() { TestConnection, UpdateTestResult = previousTest, previousUpdate })
	TestConnection = func(server *model.Server) (bool, string) {
		if server.Alias == "down" {
			return false, "timeout"
		}
		return true, ""
	}
	recorded := map[string]model.TestStatus{}
	var mu sync.Mutex
	UpdateTestResult = func(alias string, status model.TestStatus, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		recorded[alias] = status
		return nil
	}

	m := New([]*model.Server{
		{Alias: "up", Host: "a", Port: 22, User: "u"},
		{Alias: "down", Host: "b", Port: 22, User: "u"},
		{Alias: "other", Host: "c", Port: 22, User: "u"},
	})
	m.width, m.height = 120, 30
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m = updated.(*tuiModel)
	if len(m.testing) != 3 || !strings.Contains(m.View(), "testing 3") {
		t.Fatalf("expected three tests in progress, got %v", m.testing)
	}
	if !strings.Contains(m.View(), glyphs.testing) {
		t.Fatal("rows under test should show the testing glyph")
	}
	for _, msg := range runBatch(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(*tuiModel)
	}
	if len(m.testing) != 0 {
		t.Fatalf("tests still pending: %v", m.testing)
	}
	if recorded["up"] != model.TestOK || recorded["down"] != model.TestFailed {
		t.Fatalf("results not persisted: %v", recorded)
	}
	if m.serverByAlias("down").LastTestStatus != model.TestFailed {
		t.Fatal("in-memory status not updated")
	}
	if m.err != nil || !strings.Contains(m.warning, "2 OK, 1 failed") || !strings.Contains(m.View(), "2 OK, 1 failed") {
		t.Fatalf("expected batch summary warning, got err=%v warning=%q", m.err, m.warning)
	}
}

func TestSingleTestTargetsProfileEvenAfterCursorMoves(t *testing.T) {
	previousTest, previousUpdate := TestConnection, UpdateTestResult
	t.Cleanup(func() { TestConnection, UpdateTestResult = previousTest, previousUpdate })
	TestConnection = func(*model.Server) (bool, string) { return true, "" }
	UpdateTestResult = func(string, model.TestStatus, string) error { return nil }

	m := New([]*model.Server{
		{Alias: "first", Host: "a", Port: 22, User: "u"},
		{Alias: "second", Host: "b", Port: 22, User: "u"},
	})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(*tuiModel)
	m.moveCursor(1)
	for _, msg := range runBatch(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(*tuiModel)
	}
	if m.serverByAlias("first").LastTestStatus != model.TestOK || m.serverByAlias("second").LastTestStatus == model.TestOK {
		t.Fatal("test result landed on the wrong profile")
	}
	if !strings.Contains(m.success, "first: connection OK") {
		t.Fatalf("single test summary = %q", m.success)
	}
}

func TestStateColumnShowsTunnelAndSessionIndicators(t *testing.T) {
	m := New([]*model.Server{
		{Alias: "db", Host: "a", Port: 22, User: "u", LastTestStatus: model.TestOK},
		{Alias: "web", Host: "b", Port: 22, User: "u"},
	})
	m.width, m.height = 120, 30
	updated, _ := m.Update(runtimeLoadedMsg{status: runtimeStatus{
		tunnels:  map[string]int{"db": 1},
		sessions: map[string]int{"db": 2},
	}})
	m = updated.(*tuiModel)
	var dbLine, webLine string
	for _, line := range strings.Split(m.renderServerPanel(80, 8, true), "\n") {
		// Take the first hit: the list row comes before the details panel.
		switch {
		case dbLine == "" && strings.Contains(line, "u@a:22"):
			dbLine = line
		case webLine == "" && strings.Contains(line, "u@b:22"):
			webLine = line
		}
	}
	want := glyphs.ok + " " + glyphs.tunnel + " " + glyphs.session
	if !strings.Contains(dbLine, want) {
		t.Fatalf("db row should show %q:\n%s", want, dbLine)
	}
	if strings.Contains(webLine, glyphs.tunnel) || !strings.Contains(webLine, glyphs.unknown) {
		t.Fatalf("web row should show only the unknown status:\n%s", webLine)
	}
}

func TestStateGlyphsKeepTheirColorInsideRows(t *testing.T) {
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	styled := layer(selected, testFailStyle)
	if styled.GetForeground() != testFailStyle.GetForeground() {
		t.Fatalf("glyph lost its color: %v", styled.GetForeground())
	}
	if styled.GetBackground() != selected.GetBackground() {
		t.Fatalf("glyph lost the row background: %v", styled.GetBackground())
	}
}
