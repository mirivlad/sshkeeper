# TUI Tags And Global Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full TUI support for tags and global command templates, including multi-server template execution.

**Architecture:** Move command templates to global entities while preserving legacy server-scoped rows as importable data. Add server `startup_command`, richer tag CRUD helpers, and TUI screens for tag/template management. Template execution uses existing OpenSSH command construction, with foreground execution returning control to the terminal and background execution collecting per-server results in a TUI results screen.

**Tech Stack:** Go, Bubble Tea, Bubbles list/textinput, SQLite, existing Cobra CLI and TUI package.

---

### Task 1: Data Model And CLI

- [x] Add `StartupCommand` to `model.Server`.
- [x] Add DB schema migration helpers for `servers.startup_command` and global `global_command_templates`.
- [x] Add DB CRUD for global templates and tag management.
- [x] Update CLI `template` commands to manage global templates.
- [x] Update `run-template` to run a global template on a server.
- [x] Add focused DB and CLI tests.

### Task 2: TUI Tags

- [x] Add tags to server form as comma-separated input.
- [x] Persist tags on add/edit.
- [x] Show tags in selected-server details.
- [x] Add a tag management screen with list, add, rename, delete, assign/remove selected servers.
- [x] Add tests for tag rendering and callbacks.

### Task 3: TUI Templates And Selection

- [x] Add multi-selection state toggled by Insert.
- [x] Show selected markers and selected count in list footer.
- [x] Add global template picker opened by Shift+Enter.
- [x] Add template manager screen with list/add/edit/delete.
- [x] Add startup command field to server form.
- [x] Add tests for selection and template screen state.

### Task 4: Template Execution

- [x] Add foreground template execution result path that exits TUI and lets caller run SSH.
- [x] Add background execution callback and results screen.
- [x] Run background template on selected servers, collecting stdout/stderr/status.
- [x] Add tests for run mode selection and result rendering.

### Task 5: Verification

- [x] Run `env GOCACHE=/tmp/sshkeeper-go-cache go test ./...`.
- [x] Run `env GOCACHE=/tmp/sshkeeper-go-cache go build -o bin/sshkeeper .`.
- [ ] Smoke-test CLI template CRUD on temporary XDG paths.
- [ ] Commit final implementation.
