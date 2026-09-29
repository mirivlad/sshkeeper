package tui

// State is what the dashboard carries across a TUI restart. The caller leaves
// the TUI to run ssh and starts a new program afterwards; restoring State puts
// the user back where they were instead of at the top of the list.
type State struct {
	// SelectedAlias is the profile under the cursor.
	SelectedAlias string
	// Notice is shown once on the dashboard, e.g. a summary of the session
	// that just ended.
	Notice string
	// NoticeIsError renders Notice as an error.
	NoticeIsError bool
	// Collapsed lists the folded groups in group order.
	Collapsed []string
}

// State returns the dashboard state to restore after the next restart.
func (m *tuiModel) State() State {
	state := State{}
	if selected := m.selectedServer(); selected != nil {
		state.SelectedAlias = selected.Alias
	}
	for group := range m.collapsed {
		state.Collapsed = append(state.Collapsed, group)
	}
	return state
}

// Restore applies a State saved from a previous TUI run.
func (m *tuiModel) Restore(state State) {
	if len(state.Collapsed) > 0 {
		m.collapsed = map[string]bool{}
		for _, group := range state.Collapsed {
			m.collapsed[group] = true
		}
	}
	m.rebuildServerRows(state.SelectedAlias)
	if state.Notice != "" {
		if state.NoticeIsError {
			m.err = errorNotice(state.Notice)
		} else {
			m.success = state.Notice
		}
	}
}

type errorNotice string

func (e errorNotice) Error() string { return string(e) }
