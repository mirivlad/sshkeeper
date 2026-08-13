# sshkeeper TUI UX Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved safe, truthful, responsive sshkeeper TUI and verify it through automated state transitions and real terminal screenshots.

**Architecture:** Keep the existing Bubble Tea v1 root model and screen enum. Add explicit overlay return/pending state and small pure layout helpers; route keys from overlays and editors outward; keep persistence and process callbacks at the existing command boundary.

**Tech Stack:** Go 1.25, Bubble Tea v1.3.10, Bubbles v1.0.0, Lip Gloss v1.1.0, tmux-based runtime capture.

## Global Constraints

- Do not migrate Bubble Tea or add dependencies.
- Preserve CLI behavior and callback boundaries.
- Supported terminal floor is 60x16.
- Rendering must not mutate model state.
- Printable runes belong to focused editors.
- Every behavior change follows red-green TDD.
- Each task is committed and pushed to `codex/tui-ux-redesign` after focused and full tests.

---

### Task 1: Safe destructive action state machine

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/forward.go`
- Modify: `internal/tui/app_test.go`
- Create: `internal/tui/confirm_test.go`

**Interfaces:**
- Produces: `confirmState` with parent screen, message, consequence, focus, pending, action, and cancel/success return states.
- Consumes: existing delete/stop callbacks and result messages.

- [ ] Add failing tests proving server deletion opens confirmation, Cancel is the default, Esc returns to the recorded parent, Enter cannot execute while Cancel is focused, deletion executes once, forward deletion returns to its list, and errors remain visible in that list.
- [ ] Run `go test ./internal/tui -run 'Test(ServerDelete|Confirm|ForwardDelete)' -count=1` and verify the new tests fail for the missing state transitions.
- [ ] Replace `confirmMsg`/`confirmAction` with explicit confirmation state and route all server/forward/tag/template/tunnel destructive actions through it.
- [ ] Run the focused tests, then `go test ./... -count=1`.
- [ ] Commit as `fix: make tui destructive actions safe` and push the feature branch.

### Task 2: Truthful status, durable notifications, and help return context

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/help_screen.go`
- Modify: `cmd/tui.go`
- Modify: `internal/tui/app_test.go`

**Interfaces:**
- Produces: `VaultUnlocked func() bool`, durable `notification` state, and explicit help parent screen.
- Consumes: current vault callback setup and existing help models.

- [ ] Add failing tests proving lock changes the dashboard label, repeated `View()` retains notifications, F1 opens from manager screens, and closing help returns to the originating screen.
- [ ] Run `go test ./internal/tui -run 'Test(Vault|Notification|Help)' -count=1` and verify expected failures.
- [ ] Wire real vault state, move notification clearing to explicit update events, and store/restore help parent context.
- [ ] Propagate resize messages to active help, action menu, forward, form, template, and tunnel children.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Commit as `fix: keep tui status and help context truthful` and push.

### Task 3: Strict validation and dirty form exits

**Files:**
- Modify: `internal/tui/form.go`
- Modify: `internal/tui/forward.go`
- Modify: `internal/tui/template_form.go`
- Modify: `internal/tui/app.go`
- Create: `internal/tui/form_validation_test.go`

**Interfaces:**
- Produces: strict `parsePort(value string) (int, error)` and `Dirty() bool` methods for all editable forms.
- Consumes: the confirmation state from Task 1.

- [ ] Add failing tests for non-numeric, zero, and 65536 server ports; preserved invalid input; clean Esc; dirty Esc cancel/discard; and return to server/forward/template parent.
- [ ] Run `go test ./internal/tui -run 'Test(ServerPort|Dirty|Discard)' -count=1` and verify failure reasons.
- [ ] Implement strict parsing, persistent validation error state, form snapshots, and discard confirmation through the common overlay.
- [ ] Add required markers without changing stored field names.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Commit as `fix: validate tui forms and protect edits` and push.

### Task 4: Responsive dashboard and form layout

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/form.go`
- Modify: `internal/tui/forward.go`
- Modify: `internal/tui/template_form.go`
- Create: `internal/tui/layout.go`
- Create: `internal/tui/layout_test.go`

**Interfaces:**
- Produces: pure display-cell truncation, size-class, bounded-row, and pane-layout helpers.
- Consumes: model width/height and existing view data.

- [ ] Add failing render tests at 120x40, 80x24, 60x16 and below-floor size, including long Cyrillic, CJK, combining, and emoji values.
- [ ] Run `go test ./internal/tui -run 'Test(Layout|Dashboard|FormRender|DisplayWidth)' -count=1` and verify layout/width failures.
- [ ] Implement the wide two-pane, medium stacked, and narrow compact dashboard shown in the approved mockup.
- [ ] Keep form header, focused field window, inline status, action row, and footer within the height budget; render a minimum-size message below 60x16.
- [ ] Honor `NO_COLOR` when constructing styles while retaining textual markers.
- [ ] Run focused tests and `go test ./... -count=1`.
- [ ] Commit as `feat: add responsive tui layouts` and push.

### Task 5: Runtime visual verification and documentation alignment

**Files:**
- Modify: `docs/guide.md`
- Replace as needed: `docs/screenshots/screen_1.png` through `docs/screenshots/screen_5.png`

**Interfaces:**
- Consumes: completed TUI behavior.
- Produces: current screenshots and user-facing key/confirmation/responsive documentation.

- [ ] Build with `go build -o /tmp/sshkeeper-tui-audit .` and run with fresh isolated `XDG_CONFIG_HOME` and `XDG_DATA_HOME`.
- [ ] Capture the main dashboard, server form, forward form, safe confirmation, and manager states at 120x40; capture responsive dashboard/form states at 80x24 and 60x16.
- [ ] Inspect every PNG for clipping, cursor/focus visibility, status truthfulness, target naming, and footer visibility; fix defects through a new failing render/state test before code changes.
- [ ] Update `docs/guide.md` so shortcuts, confirmation behavior, minimum size, and screenshots agree with implementation.
- [ ] Run `gofmt -w` on changed Go files, `go vet ./...`, `go test ./... -count=1`, and a clean `go build ./...`.
- [ ] Commit as `docs: refresh tui guide and screenshots` and push.

### Task 6: Review and integration

**Files:**
- No planned product file changes unless review finds a defect.

**Interfaces:**
- Consumes: all prior commits.
- Produces: reviewed commit range ready for `main`.

- [ ] Request an independent code review against this design and plan.
- [ ] Resolve every Critical or Important issue using a failing regression test first.
- [ ] Re-run `go vet ./...`, `go test ./... -count=1`, `go build ./...`, and the three-size runtime capture.
- [ ] Verify feature branch is clean and synchronized, fast-forward `main`, push `main` to `origin` and `github`, and verify all three refs resolve to the same SHA.

