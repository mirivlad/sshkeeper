# sshkeeper TUI UX Redesign

## Goal

Make destructive actions safe, keep security and asynchronous status truthful,
make forms predictable, and preserve primary tasks from 120x40 down to 60x16.

## Evidence and approved direction

The design is based on a source audit, the existing documentation/screenshots,
and runtime checks in isolated XDG directories at 120x40, 80x24, and 60x16.
The approved mockups use:

- a two-pane server dashboard at wide sizes;
- a compact single-pane server table at narrow sizes;
- persistent breadcrumbs and security status;
- explicit non-color focus/selection markers;
- field-adjacent validation with preserved input;
- confirmation dialogs that name the target and default to Cancel.

## Interaction contract

Input ownership is:

`confirmation or help overlay -> active picker -> active text input -> focused component -> screen -> global`

- Printable runes always belong to a focused text input.
- `Enter` activates the focused row, button, or confirmation choice.
- `Esc` closes the most local state and returns to its recorded parent.
- `Tab` and `Shift+Tab` traverse controls in forms and dialogs.
- `?` opens contextual shortcut help outside text inputs.
- `F1` opens full help from every non-editor screen and every form.
- `Ctrl+Q` quits only from a clean state; dirty forms require discard confirmation.

## Destructive actions

Server, forward, tag, template, and running-tunnel deletion/stop operations use
one confirmation state containing:

- exact target and consequence;
- parent screen and return selection;
- safe Cancel choice as the initial focus;
- pending state that ignores repeated activation;
- success and error transitions back to the parent screen.

Deleting a server must identify that its saved forwards and vault secrets are
also removed. Deleting a saved forward must return to the forward list.

## Status and notifications

The root model receives the actual vault lock state instead of rendering
`unlocked` unconditionally. Notifications are durable model state. Rendering is
pure: `View` never clears errors or success messages. A later explicit user
event or replacement notification clears them.

Loading, pending, success, and error are distinct. Actions that may take time
show their target and disable duplicate execution.

## Forms

Server and forward ports use strict decimal parsing and the range 1 through
65535. Invalid text remains in the field and produces an actionable error.
Required fields are marked with `*`.

Every editable form stores an initial snapshot. `Esc` returns immediately when
unchanged; otherwise it opens a discard confirmation and returns to the correct
parent only after confirmation.

The server and forward forms use a viewport-like visible window centered on
the focused control. Header, validation/status area, action row, and footer
remain visible. Type selector shortcuts do not consume digits while an editor
owns input.

## Responsive layout

### 100 columns and wider

The server screen shows a list pane and a details pane. Columns are Name,
Target/Route, Auth, and Status. The details pane repeats the exact selected
target and exposes the two primary actions.

### 70 through 99 columns

The details pane moves below the list. Secondary fields are omitted from the
table, not clipped.

### 60 through 69 columns

The table contains Name, Auth, and Status. The focused row and footer remain
visible. Details are available through the action menu/help rather than taking
vertical space.

The declared supported floor is 60x16. Below it, the TUI renders a minimum-size
message rather than a misleading clipped form.

## Accessibility and compatibility

- Selection, focus, status, and severity never depend on color alone.
- Layout measurements use terminal display width, not byte length.
- Decorative color honors `NO_COLOR`; textual markers remain.
- Unicode content is truncated by display cells without splitting runes.
- Mouse remains optional; every primary task is keyboard accessible.

## Verification contract

Automated state-transition tests cover confirmation yes/no/error/repeat,
printable input ownership, form validation and dirty return paths, help return
context, vault status, resize propagation, and Unicode truncation.

Runtime checks use an isolated XDG profile and real Bubble Tea execution at
120x40, 80x24, and 60x16. Each size is captured and visually inspected for
clipping, missing focus, missing actions, and false status.
