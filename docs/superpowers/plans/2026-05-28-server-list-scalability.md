# Server List Scalability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the server list usable when the saved server count is larger than the terminal height.

**Architecture:** The list screen should render a bounded table viewport instead of rendering every server row. Selection remains owned by the existing `bubbles/list.Model`, while `viewServerList` derives a visible row range around the selected item and keeps the selected-server detail panel and footer visible.

**Tech Stack:** Go, Bubble Tea, Bubbles list, Lip Gloss, existing TUI tests in `internal/tui/app_test.go`.

---

### Task 1: Add Regression Coverage For Long Server Lists

**Files:**
- Modify: `internal/tui/app_test.go`

- [ ] **Step 1: Add a test for a constrained terminal height**

Add a test that creates more servers than can fit on screen, sets a small terminal size, renders the list, and verifies the selected details and footer are still visible.

```go
func TestServerListViewKeepsDetailsVisibleWithManyServers(t *testing.T) {
	servers := make([]*model.Server, 45)
	for i := range servers {
		servers[i] = &model.Server{
			Alias:          fmt.Sprintf("server-%02d", i+1),
			DisplayName:    fmt.Sprintf("Server %02d", i+1),
			Host:           fmt.Sprintf("host-%02d.example.org", i+1),
			Port:           22,
			User:           "mirivlad",
			AuthMethod:     model.AuthKey,
			LastTestStatus: model.TestUnknown,
		}
	}

	m := New(servers)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	model := updated.(*tuiModel)

	view := model.View()
	if !strings.Contains(view, "Server 01") {
		t.Fatalf("expected first selected server to be visible:\n%s", view)
	}
	if !strings.Contains(view, "Selected") {
		t.Fatalf("expected selected server details to remain visible:\n%s", view)
	}
	if !strings.Contains(view, "Enter connect") {
		t.Fatalf("expected footer to remain visible:\n%s", view)
	}
	if count := strings.Count(view, "server-"); count >= len(servers) {
		t.Fatalf("expected bounded row rendering, rendered %d server aliases", count)
	}
}
```

- [ ] **Step 2: Run the focused test and confirm it fails**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui -run TestServerListViewKeepsDetailsVisibleWithManyServers -count=1
```

Expected: FAIL because the current table renders every server and can push the detail panel/footer below the visible terminal area.

### Task 2: Compute A Visible Row Window

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/app_test.go`

- [ ] **Step 1: Add focused tests for visible range calculation**

Add tests for a helper that computes the inclusive start and exclusive end indexes for rendered rows.

```go
func TestVisibleServerRangeKeepsSelectionInsideWindow(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		selected  int
		available int
		wantStart int
		wantEnd   int
	}{
		{name: "first page", total: 40, selected: 0, available: 10, wantStart: 0, wantEnd: 10},
		{name: "middle page", total: 40, selected: 20, available: 10, wantStart: 11, wantEnd: 21},
		{name: "last page", total: 40, selected: 39, available: 10, wantStart: 30, wantEnd: 40},
		{name: "all fit", total: 5, selected: 3, available: 10, wantStart: 0, wantEnd: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visibleServerRange(tt.total, tt.selected, tt.available)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("visibleServerRange() = %d, %d; want %d, %d", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
```

- [ ] **Step 2: Implement `visibleServerRange`**

Add a small helper near `selectedServer`.

```go
func visibleServerRange(total, selected, available int) (int, int) {
	if total <= 0 || available <= 0 {
		return 0, 0
	}
	if available >= total {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	start := selected - available + 1
	if start < 0 {
		start = 0
	}
	end := start + available
	if end > total {
		end = total
		start = end - available
	}
	return start, end
}
```

- [ ] **Step 3: Run helper tests**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui -run TestVisibleServerRangeKeepsSelectionInsideWindow -count=1
```

Expected: PASS.

### Task 3: Render Only Rows That Fit

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/app_test.go`

- [ ] **Step 1: Reserve terminal space for fixed UI blocks**

Add a helper that decides how many server rows may be rendered while keeping the selected details and footer visible.

```go
func (m *tuiModel) visibleServerRows() int {
	if m.height <= 0 {
		return len(m.servers)
	}

	const fixedRows = 16
	rows := m.height - fixedRows
	if rows < 3 {
		return 3
	}
	return rows
}
```

- [ ] **Step 2: Use the visible range in `viewServerList`**

In `viewServerList`, replace the loop over all servers with a bounded loop:

```go
selectedIndex := m.list.Index()
start, end := visibleServerRange(len(m.servers), selectedIndex, m.visibleServerRows())
for _, server := range m.servers[start:end] {
	// existing row rendering body stays unchanged
}
```

Then render a compact range hint when rows are hidden:

```go
if len(m.servers) > end-start {
	b.WriteString(helpStyle.Render(fmt.Sprintf("  Showing %d-%d of %d", start+1, end, len(m.servers))))
	b.WriteString("\n")
}
```

- [ ] **Step 3: Run long-list regression test**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui -run TestServerListViewKeepsDetailsVisibleWithManyServers -count=1
```

Expected: PASS.

### Task 4: Verify Navigation Still Works

**Files:**
- Modify: `internal/tui/app_test.go`

- [ ] **Step 1: Add a test for moving selection beyond the first window**

Use the existing `m.list.Update` path by sending `tea.KeyDown` messages and confirm the rendered window follows the selected server.

```go
func TestServerListViewScrollsWithSelection(t *testing.T) {
	servers := make([]*model.Server, 45)
	for i := range servers {
		servers[i] = &model.Server{
			Alias:       fmt.Sprintf("server-%02d", i+1),
			DisplayName: fmt.Sprintf("Server %02d", i+1),
			Host:        fmt.Sprintf("host-%02d.example.org", i+1),
			Port:        22,
			User:        "mirivlad",
			AuthMethod:  model.AuthKey,
		}
	}

	m := New(servers)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	model := updated.(*tuiModel)
	for i := 0; i < 20; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(*tuiModel)
	}

	view := model.View()
	if !strings.Contains(view, "Server 21") {
		t.Fatalf("expected selected server to be visible after navigation:\n%s", view)
	}
	if !strings.Contains(view, "Showing") {
		t.Fatalf("expected range hint for long server list:\n%s", view)
	}
}
```

- [ ] **Step 2: Run TUI tests**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui -count=1
```

Expected: PASS.

### Task 5: Final Verification And Build

**Files:**
- No source edits expected.

- [ ] **Step 1: Run the full test suite**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Rebuild the project binary**

Run:

```bash
env GOCACHE=/tmp/sshkeeper-go-cache go build -o bin/sshkeeper .
```

Expected: exit code 0 and updated `bin/sshkeeper`.

- [ ] **Step 3: Commit the implementation**

Run:

```bash
git add internal/tui/app.go internal/tui/app_test.go bin/sshkeeper
git commit -m "fix: keep server list usable with many servers"
```

Expected: commit succeeds.

---

## Self-Review

- Spec coverage: the plan covers the known failure mode for 40+ servers, keeps selected details visible, keeps the footer visible, and preserves existing `bubbles/list` navigation.
- Placeholder scan: no `TBD`, `TODO`, or open-ended implementation placeholders remain.
- Type consistency: helper names and files match the current TUI code shape.
