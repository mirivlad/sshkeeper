# Unified sshkeeper TUI Shell

## Goal

Bring every sshkeeper TUI screen to the same visual and behavioral standard as
the approved server dashboard. Every screen must have a stable header, bounded
framed content, and a contextual footer anchored to the bottom of the terminal.

## User-approved direction

The existing server dashboard is the visual reference. Do not introduce a new
visual language or browser mockups. Apply its hierarchy, border treatment,
selection markers, spacing, colors, and responsive degradation to all child
screens.

`Ctrl+H` replaces `F1` as the global full-help binding. `?` remains contextual
quick help outside text editors. Remove `F1` from runtime help, footers, README,
and the user guide.

## Root cause

The dashboard owns a height-aware renderer with a header, panels, and footer.
Most child screens still render independent free-form strings or a default
Bubbles list. They therefore do not share inner-width budgeting, borders,
viewport height, or bottom-footer placement. The correction is a shared shell,
not per-screen blank-line padding.

## Shared screen shell

Every full-screen state uses this vertical contract:

1. Header: `sshkeeper / <breadcrumb>` on the left and truthful vault/context
   status on the right.
2. Separator: one display-cell-bounded horizontal line.
3. Content: one or two bordered panels filling all available rows.
4. Footer: only the current screen's primary shortcuts, wrapped by display
   width and anchored to the last terminal row.

The shell computes content height as terminal height minus header, separator,
notification, and wrapped-footer rows. A panel owns a one-cell border and at
least one-cell inner horizontal padding. No content row may consume the
terminal's last column directly.

Errors, success messages, pending state, and partial success appear in a
dedicated notification row below the separator. Rendering remains pure.

## Screen families

### Actions

Actions use a framed selectable list. At 100 columns and wider, a second panel
describes the selected action and its target. At 70-99 columns, the description
appears below the list when height permits. At 60-69 columns, only the framed
list remains. The footer is always at the bottom.

### Port forwards

At 100 columns and wider, forwards use a framed table and a framed selected-rule
panel. At 70-99 columns, the selected-rule panel is stacked below the table. At
60-69 columns, the table contains Name, Type, and On; details stay available
through the selected-rule panel only when vertical space permits.

All column widths are derived from the panel's inner width. The table must
leave an inner right margin, so an 80-column terminal never renders a row at 80
display cells. Long names, endpoints, explanations, and SSH arguments truncate
by display cells with an ellipsis.

The forward form uses the shared shell, a framed form panel, radio markers for
type, and an action row. Focused fields and validation remain visible.

### Managers and pickers

Tags, command templates, template picker/mode/results, and tunnel manager use a
framed list or result panel. Selection uses `>` as well as color. Empty and
error states remain inside the panel. Lists viewport around the selected item
and never rely on the default Bubbles frame or footer.

### Forms and text entry

Server, template, tag, search, and forward editors use a framed form panel.
Their title moves into the common breadcrumb. Required markers, validation,
dirty confirmation, and input ownership do not change. The focused control,
status/error, and action row remain in the panel's visible window.

### Confirmations

Confirmations use the same application header and a centered or width-bounded
framed dialog panel. Exact target, consequence, Cancel-first action row, and
footer remain visible at 60x16. Long Unicode text wraps by display cells.

### Help

`Ctrl+H` opens full help from every state except a confirmation overlay. `?`
opens contextual quick help only when a text editor does not own printable
input. `F1` has no documented or runtime binding.

Bubble Tea v1 distinguishes `KeyCtrlH` (ASCII BS) from `KeyBackspace` (DEL).
Automated and PTY checks must prove that xterm Backspace edits text while
`Ctrl+H` opens help. Terminals configured to emit BS for Backspace cannot
distinguish the two; the supported runtime check uses xterm's default DEL
Backspace mapping.

Both help screens use the shared shell and a framed, scrolling body. Their
footer stays at the bottom and states how to close/scroll.

## Responsive contract

- Supported floor: `60x16`.
- Wide: `width >= 100`, two panels where useful.
- Medium: `70 <= width < 100`, stacked panels.
- Narrow: `60 <= width < 70`, compact single panel.
- Below the floor: render only the minimum-size message.

Every screen family is tested at 120x40, 80x24, and 60x16 with long ASCII,
Cyrillic, CJK, combining, and emoji content. Assertions measure ANSI-aware
display cells and exact maximum height.

## Verification

Automated coverage must inventory every `screen` enum value and prove that its
normal representative state:

- fits terminal width and height;
- contains a top header and at least one border;
- keeps the contextual footer on the final rendered row(s);
- retains a non-color selection/focus marker where applicable;
- does not expose `F1` and does expose `Ctrl+H` help where applicable.

Runtime verification uses the freshly built binary in an isolated XDG profile.
Capture and inspect dashboard, actions, forwards, forward form, tunnel manager,
server form, template/tag managers, confirmation, and help at all three sizes.

## Delivery boundary

Commit and push each implementation stage. Finish with a fresh binary under
`bin/sshkeeper` and report its absolute path and checksum. Do not publish a
release or update release metadata until the user explicitly approves the
binary.
