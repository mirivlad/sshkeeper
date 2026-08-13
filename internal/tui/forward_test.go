package tui

import (
	"fmt"
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
