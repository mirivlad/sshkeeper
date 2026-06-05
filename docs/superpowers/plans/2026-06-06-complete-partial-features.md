# Complete Partial Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish sshkeeper features that are currently stubbed or partially implemented, then align documentation and screenshots with the actual product.

**Architecture:** Keep changes surgical and local to existing command, TUI, database, SSH, and tunnel modules. Prefer existing callbacks and model types over new abstractions. Each task must end with focused tests, a commit, and a push to `origin/main`.

**Tech Stack:** Go 1.25, Cobra, Bubble Tea, SQLite via modernc.org/sqlite, OpenSSH command execution.

---

## Files

- Modify `cmd/forward.go`: make `forward edit` update stored forwards instead of printing a fake success.
- Modify `cmd/tunnel.go`: add `--background`, `list`, `stop`, and `stop-all` tunnel CLI support.
- Modify `internal/tunnel/manager.go`: improve process status checks, cleanup, and background tunnel startup validation.
- Modify `internal/ssh/command.go` and/or `internal/ssh/pty.go`: support safe non-interactive background tunnel behavior for secret auth or reject unsupported modes clearly.
- Modify `internal/db/servers.go`: search route JSON, tags, and forward fields.
- Modify `internal/tui/forward.go`: fix SOCKS/dynamic focus panic.
- Modify `internal/tui/app.go`, `internal/tui/help_screen.go`, `cmd/tui.go`: remove or implement action-menu stubs.
- Modify `cmd/import.go`, `cmd/extra.go` as needed if TUI import/export should delegate to existing CLI behavior.
- Modify `README.md`, `docs/guide.md`, `docs/roadmap/v0.2.0.md`: bring docs into line with implemented behavior.
- Replace `docs/screenshots/screen_*.png`: clean screenshots for main, action menu, routes, forwards, tunnels.
- Add/modify tests in `cmd/*_test.go`, `internal/db/*_test.go`, `internal/tui/*_test.go`, `internal/tunnel/*_test.go`, `internal/ssh/*_test.go`.

## Task 1: Small Partial Features and Search

- [ ] Step 1: Add failing tests for SOCKS forward form focus.
  - Test file: `internal/tui/app_test.go`.
  - Behavior: selecting dynamic/SOCKS and rendering/updating focus must not panic.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui -run 'TestForward.*Dynamic|TestForward.*SOCKS'`.
  - Expected before implementation: fail or panic.

- [ ] Step 2: Fix the forward form label/focus logic.
  - File: `internal/tui/forward.go`.
  - Keep hidden inputs from asking for labels that do not exist for dynamic forwards.

- [ ] Step 3: Add failing CLI tests for `forward edit`.
  - Test file: `cmd/forward_test.go` or existing cmd tests.
  - Behavior: editing `--enabled=false` persists the enabled state for the selected forward.
  - Run focused cmd tests.

- [ ] Step 4: Implement `forward edit`.
  - File: `cmd/forward.go`.
  - Load the forward, validate it exists, apply only changed flags, persist with `appDB.UpdateForward`.

- [ ] Step 5: Add failing search tests.
  - Test file: `internal/db/servers_test.go`.
  - Behavior: search finds servers by tags, route hops, and forward ports.

- [ ] Step 6: Implement expanded search.
  - File: `internal/db/servers.go`.
  - Use SQL `EXISTS` subqueries for tags and forwards, include `route_hops`.

- [ ] Step 7: Verify and commit.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui ./internal/db ./cmd`.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go vet ./...`.
  - Commit: `fix: complete forwards and search behavior`.
  - Push: `git push origin main`.

## Task 2: Tunnel CLI and Background State

- [ ] Step 1: Add failing tests for tunnel process status.
  - Test file: `internal/tunnel/manager_test.go`.
  - Behavior: `IsRunning` returns false for missing/stopped PIDs and uses signal 0 semantics.

- [ ] Step 2: Fix process status and cleanup.
  - File: `internal/tunnel/manager.go`.
  - Use `syscall.Signal(0)`, remove stopped states on list/refresh where appropriate, and report save errors when state persistence fails.

- [ ] Step 3: Add failing command tests for tunnel subcommands.
  - Test file: `cmd/tunnel_test.go`.
  - Behavior: `tunnel --help` exposes `--background`; `tunnel list`, `tunnel stop <id>`, and `tunnel stop-all` are registered.

- [ ] Step 4: Implement tunnel CLI subcommands.
  - File: `cmd/tunnel.go`.
  - `--background` starts `tunnelpkg.Start`; `list` prints saved states and status; `stop` and `stop-all` delegate to manager.

- [ ] Step 5: Handle secret auth background tunnels safely.
  - File: `cmd/tunnel.go` or `internal/tunnel/manager.go`.
  - If password/key-passphrase background tunneling cannot be supported safely without interactive PTY, reject with a clear error. Do not create a broken saved state.

- [ ] Step 6: Verify and commit.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tunnel ./cmd ./internal/ssh`.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go vet ./...`.
  - Commit: `feat: add tunnel background cli management`.
  - Push: `git push origin main`.

## Task 3: TUI Stub Actions

- [ ] Step 1: Route action.
  - Implement a non-stub route action in TUI. Minimal acceptable behavior: open an edit flow for route hops or delegate to server edit where route/proxy jump can be changed, with clear user-facing text.
  - Add tests that action no longer sets `not yet implemented`.

- [ ] Step 2: Import/export actions.
  - Implement TUI import by calling existing import logic and refreshing the server list.
  - Implement TUI export if a real export command exists; otherwise remove the TUI action and documentation entry so no fake capability is shown.

- [ ] Step 3: Vault actions.
  - Implement TUI vault lock by locking the current process vault.
  - Implement TUI vault change password only if it can safely exit to a CLI prompt; otherwise remove the menu entry and keep the CLI command documented.

- [ ] Step 4: Verify and commit.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./internal/tui ./cmd`.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go vet ./...`.
  - Commit: `feat: replace tui action stubs`.
  - Push: `git push origin main`.

## Task 4: Documentation and Screenshots

- [ ] Step 1: Update README and guide.
  - Ensure every documented command exists and every TUI action described is implemented or intentionally absent.
  - Clarify background tunnels with secret auth limitations if unsupported.

- [ ] Step 2: Update roadmap status.
  - Mark fulfilled features accurately and note remaining future work only if it is not shown as available.

- [ ] Step 3: Rebuild and capture screenshots.
  - Build binary with `env GOCACHE=/tmp/sshkeeper-go-cache go build -o /tmp/sshkeeper-screens .`.
  - Use isolated `XDG_CONFIG_HOME` and `XDG_DATA_HOME` under `/tmp`.
  - Capture clean terminal screenshots for main list, action menu, route/forward/tunnel screens.

- [ ] Step 4: Verify and commit.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./...`.
  - Run: `env GOCACHE=/tmp/sshkeeper-go-cache go vet ./...`.
  - Commit: `docs: align guide and screenshots`.
  - Push: `git push origin main`.

## Final Verification

- [ ] Run full checks: `env GOCACHE=/tmp/sshkeeper-go-cache go test ./...`.
- [ ] Run full vet: `env GOCACHE=/tmp/sshkeeper-go-cache go vet ./...`.
- [ ] Confirm `git status --short --branch` is clean and `main` is pushed.
