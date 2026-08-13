# Unified sshkeeper TUI Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the approved dashboard-style full-screen shell to every sshkeeper TUI screen and replace F1 help with Ctrl+H.

**Architecture:** Add one pure screen-shell renderer that owns header, notification, bordered content height, inner width, and bottom footer. Refactor each child screen to supply bounded body panels and contextual help rather than free-form terminal strings; retain existing Bubble Tea state and callback boundaries.

**Tech Stack:** Go 1.25, Bubble Tea v1.3.10, Bubbles v1.0.0, Lip Gloss v1.1.0, charmbracelet/x/ansi v0.11.6, tmux/xterm/Xvfb runtime capture.

## Global Constraints

- The existing server dashboard is the visual reference.
- Supported terminal floor is exactly `60x16`.
- Breakpoints are narrow 60-69, medium 70-99, and wide 100+ columns.
- Every screen has a header, framed bounded content, and footer anchored to the bottom.
- No rendered row directly consumes the last terminal column.
- `Ctrl+H` is global full help; remove `F1` from runtime and documentation.
- Printable input ownership, destructive safety, validation, dirty state, and callback boundaries remain intact.
- No release publication or release-metadata changes.
- Commit and push after every task.

---

### Task 1: Shared shell and help binding

**Files:**
- Create: `internal/tui/shell.go`
- Create: `internal/tui/shell_test.go`
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/help.go`
- Modify: `internal/tui/help_screen.go`
- Modify: `internal/tui/status_help_test.go`

**Interfaces:**
- Produces: `screenShell`, `renderScreenShell(screenShell) string`, `renderBodyPanel(width, height int, lines []string) string`, and `footerAtBottom(body, footer string, height int) string`.
- Consumes: existing `renderPanel`, display-cell helpers, root vault/notification state, and child body strings.

- [ ] Add failing tests that render representative action/help/confirm states at 120x40, 80x24, and 60x16 and assert header, border, exact height, last-row footer, one-cell right safety margin, and no `F1` text.
- [ ] Add failing root-event tests proving `tea.KeyCtrlH` opens full help from list, manager, and form; `tea.KeyBackspace` still reaches a focused text input; and closing help restores its parent.
- [ ] Run `go test ./internal/tui -run 'Test(ScreenShell|CtrlH|Backspace|NoF1)' -count=1` and confirm failures identify missing shell/binding behavior.
- [ ] Implement the shell with a spec carrying `breadcrumb`, `status`, `notification`, `body`, `footer`, `width`, and `height`; calculate body height after wrapped footer rows and render content inside a bounded panel.
- [ ] Replace the root F1 branch with `tea.KeyCtrlH`, remove F1 entries from quick/full help content, and preserve active confirmation ownership.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Commit as `feat: add unified tui screen shell` and push.

### Task 2: Actions, help, search, and confirmations

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/help_screen.go`
- Modify: `internal/tui/layout_test.go`
- Modify: `internal/tui/status_help_test.go`

**Interfaces:**
- Consumes: `renderScreenShell` and `renderBodyPanel` from Task 1.
- Produces: shell-backed action menu, quick/full help, search, tag input, and confirmation views.

- [ ] Add failing render tests for action selection at every breakpoint, long help rows, search input, tag input, and long Unicode confirmation content; assert selection/focus markers and bottom footer.
- [ ] Run `go test ./internal/tui -run 'Test(ActionShell|HelpShell|InputShell|ConfirmationShell)' -count=1` and confirm current free-form views fail.
- [ ] Render actions as a bounded list with a wide description panel and stacked medium description; render help as a scrolling framed body; render search/tag input inside a form panel; render confirmation as a bounded dialog within the shell.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Capture action, help, and confirmation in real xterm at 120x40, 80x24, and 60x16; inspect header, border, focus, footer, and right margin.
- [ ] Commit as `feat: unify tui actions and help screens` and push.

### Task 3: Port forward manager and form

**Files:**
- Modify: `internal/tui/forward.go`
- Modify: `internal/tui/forward_test.go`
- Modify: `internal/tui/layout_test.go`

**Interfaces:**
- Consumes: shared shell and panel primitives.
- Produces: responsive framed forward table/details and framed forward editor.

- [ ] Add failing tests with long ASCII/Cyrillic/CJK/emoji names and endpoints at all three sizes. Assert every ANSI-stripped line is at most `width-1`, both table and footer stay within height, and footer occupies the final rows.
- [ ] Add table-driven tests for wide two-panel, medium stacked, and narrow compact column sets.
- [ ] Run `go test ./internal/tui -run 'Test(ForwardManagerShell|ForwardFormShell|ForwardColumns)' -count=1` and confirm overflow/frame/footer failures.
- [ ] Derive every table width from panel inner width with a one-cell safety margin; add framed table/details layouts per breakpoint and move form content into the shared framed shell.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Capture forward list and form at all three sizes in xterm, inspect for wrapping/overflow, and add a failing regression test before correcting any observed defect.
- [ ] Commit as `feat: redesign tui port forward screens` and push.

### Task 4: Tags, templates, results, and tunnels

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/tunnel.go`
- Modify: `internal/tui/template_form.go`
- Modify: `internal/tui/layout_test.go`

**Interfaces:**
- Consumes: shared shell and bounded list/panel primitives.
- Produces: shell-backed tag manager, template manager/form/picker/mode/results, and tunnel manager.

- [ ] Add a screen-inventory render test covering normal, empty, error, and selected states for every manager at 120x40, 80x24, and 60x16.
- [ ] Run `go test ./internal/tui -run 'Test(ManagerScreenInventory|TunnelShell|TemplateShell|TagShell)' -count=1` and confirm missing frames/footer anchoring.
- [ ] Replace default Bubbles list rendering and free-form strings with bounded viewport rows inside framed panels. Keep exact selection, scroll index, and contextual actions.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Capture every manager family at 80x24 and its densest/longest state at 60x16; inspect borders, focus, footer, empty/error copy, and truncation.
- [ ] Commit as `feat: unify tui manager screens` and push.

### Task 5: Server form and complete screen matrix

**Files:**
- Modify: `internal/tui/form.go`
- Modify: `internal/tui/template_form.go`
- Modify: `internal/tui/layout_test.go`
- Modify: `internal/tui/form_validation_test.go`

**Interfaces:**
- Consumes: shared shell and existing focus-centered form viewport.
- Produces: framed server/template forms plus a complete enum-state layout matrix.

- [ ] Add failing tests proving server/template form breadcrumbs, panel borders, focused field visibility, validation visibility, action row, and bottom footer at all sizes.
- [ ] Add one exhaustive table listing every `screen` enum value with a representative model builder; assert all non-below-floor screens satisfy shell invariants.
- [ ] Run `go test ./internal/tui -run 'Test(FormShell|AllScreensUseShell)' -count=1` and confirm remaining non-shell states fail.
- [ ] Move form viewport content into the shared shell without changing navigation or save behavior; close all inventory gaps.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Commit as `feat: finish unified tui screen coverage` and push.

### Task 6: Runtime verification, documentation, and binary

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`
- Replace after runtime verification: `docs/screenshots/screen_1.png` through `docs/screenshots/screen_5.png`

**Interfaces:**
- Consumes: completed TUI and isolated XDG audit profile.
- Produces: verified screenshots, synchronized documentation, and `bin/sshkeeper`.

- [ ] Build `/tmp/sshkeeper-unified-audit` and run it in tmux/xterm with the isolated audit profile.
- [ ] Capture dashboard, actions, forwards, forward form, tunnels, server form, tags/templates, confirmation, quick help, and full help at 120x40, 80x24, and 60x16.
- [ ] Inspect every capture for border continuity, right-column overflow, focus, notification truth, bottom footer, and correct Ctrl+H copy. For any defect, add a failing automated test before editing code.
- [ ] Verify real xterm sends Backspace as `KeyBackspace` and Ctrl+H as `KeyCtrlH`; document the terminal mapping constraint without restoring F1.
- [ ] Update README and guide only after runtime behavior is verified; refresh repository screenshots from the verified binary.
- [ ] Run `gofmt`, `git diff --check`, `go vet ./...`, `go test ./... -count=1`, and `go build ./...`.
- [ ] Commit as `docs: refresh unified tui screenshots` and push.
- [ ] Run `./build.sh`, report the absolute binary path, version, SHA-256, feature-branch HEAD, remote synchronization, and clean worktree. Do not publish a release.
