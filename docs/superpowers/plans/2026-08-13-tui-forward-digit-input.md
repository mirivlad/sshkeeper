# TUI Port Forward Digit Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `1`, `2`, and `3` to be entered in port-forward text fields without removing the existing type-selection shortcuts.

**Architecture:** Keep the current event-routing structure and make the digit shortcut context-sensitive. The shortcut block will run only while one of the three type selector rows has focus; otherwise the existing focused `textinput` receives the key event.

**Tech Stack:** Go 1.25+, Bubble Tea, Bubbles `textinput`, standard Go testing.

## Global Constraints

- Touch only the port-forward form and its focused regression tests.
- Preserve `1/2/3` type selection on the type selector.
- Preserve all existing navigation, validation, persistence, and help text.
- Add no dependencies or abstractions.

---

### Task 1: Context-sensitive digit shortcuts

**Files:**
- Create: `internal/tui/forward_test.go`
- Modify: `internal/tui/forward.go:320-340`
- Verify: `internal/tui/form.go`, `internal/tui/template_form.go`, `internal/tui/app.go`

**Interfaces:**
- Consumes: `forwardFormModel.Update(tea.Msg) (tea.Model, tea.Cmd)`, `forwardTypes`, and the existing focus index layout where indices `2` through `2+len(forwardTypes)-1` are type selector rows.
- Produces: unchanged public and package interfaces; only event precedence changes inside `forwardFormModel.Update`.

- [ ] **Step 1: Add the failing digit-entry regression test**

Create `internal/tui/forward_test.go` with a test that focuses the listen-port
input and sends real Bubble Tea key messages one at a time:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestForwardFormDigitsReachFocusedInput(t *testing.T) {
	fm := newForwardFormModel(1, 100, 30)
	fm.focusIdx = 2 + len(forwardTypes) + 1
	fm.updateFocus()

	for _, digit := range []rune{'1', '2', '3'} {
		updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{digit}})
		fm = updated.(*forwardFormModel)
	}

	if got := fm.inputs[1].Value(); got != "123" {
		t.Fatalf("listen port = %q, want %q", got, "123")
	}
	if fm.currentType != model.ForwardLocal {
		t.Fatalf("forward type = %q, want %q", fm.currentType, model.ForwardLocal)
	}
}
```

- [ ] **Step 2: Run the regression test and confirm the existing bug**

Run:

```bash
go test ./internal/tui -run '^TestForwardFormDigitsReachFocusedInput$' -count=1
```

Expected result: FAIL because the listen-port value is empty; `1`, `2`, and
`3` were consumed by the global type shortcut block.

- [ ] **Step 3: Gate digit shortcuts by selector focus**

Change the existing `tea.KeyRunes` condition in `internal/tui/forward.go` to:

```go
case tea.KeyRunes:
	// Direct number keys select a type only while the type selector has focus.
	if fm.focusIdx >= 2 && fm.focusIdx < 2+len(forwardTypes) && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case '1':
			fm.typeIdx = 0
			fm.currentType = model.ForwardLocal
			fm.updateFocus()
			return fm, nil
		case '2':
			fm.typeIdx = 1
			fm.currentType = model.ForwardRemote
			fm.updateFocus()
			return fm, nil
		case '3':
			fm.typeIdx = 2
			fm.currentType = model.ForwardDynamic
			fm.updateFocus()
			return fm, nil
		}
	}
```

- [ ] **Step 4: Confirm the digit-entry regression is fixed**

Run:

```bash
go test ./internal/tui -run '^TestForwardFormDigitsReachFocusedInput$' -count=1
```

Expected result: PASS.

- [ ] **Step 5: Add shortcut-preservation coverage**

Add `"fmt"` to the existing import block, then append this independent test to
`internal/tui/forward_test.go`:

```go
import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)
```

The resulting import block must replace the original block. Append:

```go
func TestForwardFormDigitShortcutsWorkOnTypeSelector(t *testing.T) {
	tests := []struct {
		digit rune
		want  model.ForwardType
		idx   int
	}{
		{digit: '1', want: model.ForwardLocal, idx: 0},
		{digit: '2', want: model.ForwardRemote, idx: 1},
		{digit: '3', want: model.ForwardDynamic, idx: 2},
	}

	for focusIdx := 2; focusIdx < 2+len(forwardTypes); focusIdx++ {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("focus_%d_digit_%c", focusIdx, tt.digit), func(t *testing.T) {
				fm := newForwardFormModel(1, 100, 30)
				fm.currentType = model.ForwardDynamic
				fm.typeIdx = typeIndex(model.ForwardDynamic)
				if tt.want == model.ForwardDynamic {
					fm.currentType = model.ForwardLocal
					fm.typeIdx = typeIndex(model.ForwardLocal)
				}
				fm.focusIdx = focusIdx
				fm.updateFocus()

				updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.digit}})
				fm = updated.(*forwardFormModel)

				if fm.currentType != tt.want {
					t.Fatalf("forward type = %q, want %q", fm.currentType, tt.want)
				}
				if fm.typeIdx != tt.idx {
					t.Fatalf("type index = %d, want %d", fm.typeIdx, tt.idx)
				}
			})
		}
	}
}
```

- [ ] **Step 6: Run focused and full verification**

Run:

```bash
gofmt -w internal/tui/forward.go internal/tui/forward_test.go
go test ./internal/tui -run '^TestForwardFormDigit' -count=1
go test ./...
go vet ./...
git diff --check
```

Expected result: both focused tests pass, all repository tests pass, `go vet`
exits successfully, and `git diff --check` prints no errors.

- [ ] **Step 7: Commit the reviewed fix**

Run:

```bash
git add docs/superpowers/specs/2026-08-13-tui-forward-digit-input-design.md \
  docs/superpowers/plans/2026-08-13-tui-forward-digit-input.md \
  internal/tui/forward.go internal/tui/forward_test.go
git commit -m "fix: allow digits in port forward fields"
```

Expected result: one commit containing only the design, plan, regression tests,
and minimal production fix.

- [ ] **Step 8: Build and inspect the user-testable binary**

Run:

```bash
./build.sh
file /home/mirivlad/git/sshkeeper/bin/sshkeeper
sha256sum /home/mirivlad/git/sshkeeper/bin/sshkeeper
git status --short --branch
```

Expected result: `./build.sh` exits successfully, the file is a native
executable at `/home/mirivlad/git/sshkeeper/bin/sshkeeper`, a checksum is
printed, and the tracked working tree is clean.
