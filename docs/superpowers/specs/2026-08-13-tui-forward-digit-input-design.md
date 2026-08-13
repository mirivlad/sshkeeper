# TUI Port Forward Digit Input Design

## Problem

The port-forward form displays `1`, `2`, and `3` as shortcuts for Local,
Remote, and SOCKS. Its `Update` method currently handles those runes before it
routes the event to the focused `textinput`, regardless of which control has
focus. As a result, typing any of those digits into a name, description,
address, or port field changes the forward type and drops the character.

## Scope

- Preserve the existing `1/2/3` type-selection shortcuts while focus is on any
  of the three type selector rows.
- Treat `1`, `2`, and `3` as ordinary input everywhere else in the form.
- Add focused regression coverage for digit entry and shortcut preservation.
- Do not change form navigation, validation, persistence, or unrelated TUI
  behavior.

## Design

Gate the existing digit shortcut block on the type selector focus range:
`focusIdx >= 2 && focusIdx < 2+len(forwardTypes)`. When that condition is
false, processing falls through to the existing focused-input routing. No new
state, helper, or dependency is needed.

The type selector itself remains keyboard-accessible through Tab/arrow
navigation, Enter, and direct `1/2/3` selection. The footer remains accurate.

## Regression Coverage

1. Focus the listen-port input, send separate Bubble Tea key messages for
   `1`, `2`, and `3`, and assert that the field contains `123` and the forward
   type remains Local.
2. For every type selector row, assert that each direct digit still selects
   the corresponding Local, Remote, or SOCKS type and index, starting from a
   deliberately different type so no case can pass from the default state.

## Similar-Error Audit

All `tea.KeyRunes` branches and all `textinput.Model.Update` call sites under
`internal/tui` were inspected. The server form's `/` shortcut is explicitly
limited to the Auth Method and Group selector fields. Search, tag, and template
forms do not intercept printable shortcut keys before their text inputs. The
remaining rune shortcuts are confined to non-input screens (lists, help,
confirmation, and mode selection). No second instance of this bug class was
found.

## Verification

- Demonstrate that the new regression test fails against the current code.
- Apply the one-condition production fix and demonstrate that the focused
  tests pass.
- Run `go test ./...`, `go vet ./...`, and `./build.sh`.
- Confirm the final binary exists at
  `/home/mirivlad/git/sshkeeper/bin/sshkeeper` and report its metadata.
